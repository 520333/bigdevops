package view_server

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// autoExecuteOrderRobotTask 处理工单流转到 auto_order_robot 时的自动化流水线创建任务
func autoExecuteOrderRobotTask(sc *config.ServerConfig, jc *cache.JenkinsCache, orderID int) {
	dbObj, err := models.GetWorkOrderInstanceById(orderID)
	if err != nil || dbObj == nil {
		sc.Logger.Error("autoExecuteOrderRobotTask 未找到工单实例", zap.Int("orderID", orderID), zap.Error(err))
		return
	}

	formDataStr := dbObj.ActualApiJsonData
	if formDataStr == "" {
		recordRobotExecutionFailure(sc, dbObj, "工单未包含任何动态表单数据")
		return
	}

	var form map[string]interface{}
	if err := json.Unmarshal([]byte(formDataStr), &form); err != nil {
		recordRobotExecutionFailure(sc, dbObj, fmt.Sprintf("解析工单表单 JSON 失败: %v", err))
		return
	}

	// 1. 提取 Git 仓库
	gitRepo := strings.TrimSpace(getFormString(form, "git_repo"))
	if gitRepo == "" {
		recordRobotExecutionFailure(sc, dbObj, "表单缺少必填参数: git_repo (GIT仓库)")
		return
	}
	gitBranch := strings.TrimSpace(getFormString(form, "git_branch"))
	if gitBranch == "" {
		gitBranch = "main"
	}

	// 2. 提取部署环境 (dev/test/stage/uat/prod)
	deployEnv := strings.TrimSpace(getFormString(form, "deploy_env"))
	if deployEnv == "" {
		deployEnv = strings.TrimSpace(getFormString(form, "use_config"))
	}
	if deployEnv == "" {
		deployEnv = "test"
	}

	// 3. 提取部署类型 (binary/docker/k8s -> 映射为 bin/docker/k8s)
	deployType := strings.TrimSpace(getFormString(form, "deploy_type"))
	if deployType == "binary" || deployType == "" {
		deployType = "bin"
	}

	// 4. 提取 Jenkins 实例
	jenkinsInstance := getFormUint(form, "jenkins_instance")
	if jenkinsInstance == 0 {
		var firstInst models.JenkinsInstance
		if err := models.Db.First(&firstInst).Error; err == nil {
			jenkinsInstance = firstInst.ID
		}
	}
	if jenkinsInstance == 0 {
		recordRobotExecutionFailure(sc, dbObj, "未找到任何可用的 Jenkins 实例配置")
		return
	}

	// 5. 提取流水线模板
	pipelineTemplateId := getFormUint(form, "pipeline_template")

	// 6. 提取目标主机与目标集群
	targetHosts := getFormStringSlice(form, "host")
	k8sClusters := getFormStringSlice(form, "k8s")

	// 7. 技术栈推导
	projectType := strings.TrimSpace(getFormString(form, "project_type"))
	lang := "Java"
	if projectType == "frontend" {
		lang = "Vue/TS"
	}

	// 8. 智能推导 JobName 与 Folder (严格遵循规范: {repo_name}-{deploy_env})
	var folder string
	var repoName string

	var dbRepo models.CodeGitRepo
	if err := models.Db.Where(
		"clone_url_ssh = ? OR clone_url_http = ? OR clone_url_ssh LIKE ? OR clone_url_http LIKE ?",
		gitRepo, gitRepo, "%"+gitRepo+"%", "%"+gitRepo+"%",
	).First(&dbRepo).Error; err == nil && dbRepo.Name != "" {
		repoName = dbRepo.Name
		folder = dbRepo.NamespacePath
		if folder == "" && strings.Contains(dbRepo.FullName, "/") {
			folder = strings.Split(dbRepo.FullName, "/")[0]
		}
	}

	if repoName == "" || folder == "" {
		cleanPath := parseGitUrlToPath(gitRepo)
		parts := strings.Split(cleanPath, "/")
		if len(parts) >= 2 {
			folder = parts[0]
			repoName = parts[len(parts)-1]
		} else if len(parts) == 1 {
			repoName = parts[0]
			folder = parts[0]
		}
	}

	if folder == "" {
		folder = "default"
	}
	if repoName == "" {
		repoName = "app"
	}

	jobName := fmt.Sprintf("%s-%s", repoName, deployEnv)

	// 9. 获取流水线脚本模板并注入变量渲染
	var pipelineScript string
	if pipelineTemplateId > 0 {
		tpl, err := models.GetJenkinsPipelineConfigById(pipelineTemplateId)
		if err == nil && tpl != nil {
			pipelineScript = tpl.PipelineScript
			if tpl.Lang != "" {
				lang = tpl.Lang
			}
		}
	}

	if pipelineScript == "" {
		var defaultTpl models.JenkinsPipelineConfig
		if err := models.Db.Where("lang = ?", lang).First(&defaultTpl).Error; err == nil {
			pipelineScript = defaultTpl.PipelineScript
		} else {
			_ = models.Db.First(&defaultTpl).Error
			pipelineScript = defaultTpl.PipelineScript
		}
	}

	if pipelineScript == "" {
		recordRobotExecutionFailure(sc, dbObj, "未找到可匹配的流水线基础模板")
		return
	}

	renderedScript := renderPipelineScript(pipelineScript, gitRepo, gitBranch, deployType, deployEnv, targetHosts, k8sClusters)

	// 10. 联线 Jenkins 实例执行远端创建
	client := jc.GetJenkinsClientById(jenkinsInstance)
	if client == nil {
		recordRobotExecutionFailure(sc, dbObj, fmt.Sprintf("Jenkins 实例 (ID: %d) 离线或无法连线", jenkinsInstance))
		return
	}

	req := createOrUpdateJobReq{
		InstanceID:     jenkinsInstance,
		JobName:        jobName,
		Folder:         folder,
		ProjectName:    folder,
		GitRepo:        gitRepo,
		GitBranch:      gitBranch,
		DeployType:     deployType,
		DeployEnv:      deployEnv,
		Lang:           lang,
		PipelineScript: renderedScript,
		CreateUserName: "auto_order_robot",
		EnableDelete:   2,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	createdJob, err := doCreateJenkinsJobCore(ctx, sc, client, req)
	if err != nil {
		recordRobotExecutionFailure(sc, dbObj, fmt.Sprintf("调用 Jenkins 引擎创建作业过程报错: %v", err))
		return
	}

	// 11. 标记执行成功，写回工单流转记录
	recordRobotExecutionSuccess(sc, dbObj, createdJob)
}

// renderPipelineScript 将工单表单参数替换进流水线 Groovy 模板中
func renderPipelineScript(script, gitRepo, branch, deployType, deployEnv string, targetHosts, k8sCluster []string) string {
	if script == "" {
		return script
	}

	envNameMap := map[string]string{
		"dev":   "开发环境",
		"test":  "测试环境",
		"stage": "预发布环境",
		"uat":   "UAT环境",
		"pre":   "灰度环境",
		"prod":  "生产环境",
	}
	envText := envNameMap[deployEnv]
	if envText == "" {
		envText = "测试环境"
	}

	// 1. 同步 Git 仓库
	if gitRepo != "" {
		reGit := regexp.MustCompile(`(?i)string\s+defaultValue:\s*['"][^'"]*['"],\s*name:\s*['"](GIT仓库|gitRepo|git_repo)['"]`)
		script = reGit.ReplaceAllString(script, fmt.Sprintf("string defaultValue: '%s', name: '$1'", gitRepo))

		reUrl := regexp.MustCompile(`url:\s*['"][^'"]*['"]`)
		script = reUrl.ReplaceAllString(script, fmt.Sprintf("url: '%s'", gitRepo))

		reLsRemote := regexp.MustCompile(`git ls-remote -t -h [^\s"'\\]+`)
		script = reLsRemote.ReplaceAllString(script, fmt.Sprintf("git ls-remote -t -h %s", gitRepo))
	}

	// 2. 同步分支
	if branch != "" {
		reBranch := regexp.MustCompile(`(?i)string\s+defaultValue:\s*['"][^'"]*['"],\s*name:\s*['"](分支名|GIT分支|gitBranch|branch)['"]`)
		script = reBranch.ReplaceAllString(script, fmt.Sprintf("string defaultValue: '%s', name: '$1'", branch))

		reBranchKey := regexp.MustCompile(`branch:\s*['"][^'"]*['"]`)
		script = reBranchKey.ReplaceAllString(script, fmt.Sprintf("branch: '%s'", branch))
	}

	// 3. 同步目标主机
	if (deployType == "bin" || deployType == "binary" || deployType == "docker") && len(targetHosts) > 0 {
		hostsCsv := strings.Join(targetHosts, ",")
		hostParamLine := fmt.Sprintf("string defaultValue: '%s', description: '''%s   ---%s''', name: '目标主机'", hostsCsv, hostsCsv, envText)

		lines := strings.Split(script, "\n")
		hostLineIndex := -1
		for i, l := range lines {
			if (strings.Contains(l, "choice") || strings.Contains(l, "string")) &&
				(strings.Contains(l, "'目标主机'") || strings.Contains(l, "\"目标主机\"") || strings.Contains(l, "TARGET_HOST")) {
				hostLineIndex = i
				break
			}
		}

		if hostLineIndex != -1 {
			lines[hostLineIndex] = "        " + hostParamLine
			script = strings.Join(lines, "\n")
		} else {
			reParam := regexp.MustCompile(`(?i)parameters\s*\{`)
			if reParam.MatchString(script) {
				script = reParam.ReplaceAllString(script, fmt.Sprintf("parameters {\n        %s", hostParamLine))
			}
		}
	}

	// 4. 同步 K8s 集群
	if deployType == "k8s" && len(k8sCluster) > 0 {
		var choices []string
		for _, c := range k8sCluster {
			choices = append(choices, fmt.Sprintf("'%s'", c))
		}
		clusterParamLine := fmt.Sprintf("choice choices: [%s], name: 'CLUSTER'", strings.Join(choices, ", "))
		lines := strings.Split(script, "\n")
		clusterLineIndex := -1
		for i, l := range lines {
			if strings.Contains(l, "choice") &&
				(strings.Contains(l, "'CLUSTER'") || strings.Contains(l, "\"CLUSTER\"") || strings.Contains(l, "目标集群")) {
				clusterLineIndex = i
				break
			}
		}

		if clusterLineIndex != -1 {
			lines[clusterLineIndex] = "        " + clusterParamLine
			script = strings.Join(lines, "\n")
		} else {
			reParam := regexp.MustCompile(`(?i)parameters\s*\{`)
			if reParam.MatchString(script) {
				script = reParam.ReplaceAllString(script, fmt.Sprintf("parameters {\n        %s", clusterParamLine))
			}
		}
	}

	return script
}

func recordRobotExecutionSuccess(sc *config.ServerConfig, dbObj *models.WorkOrderInstance, createdJob *models.JenkinsJob) {
	var flowNodes []models.WorkOrderFlowNode
	_ = json.Unmarshal([]byte(dbObj.ActualFlowData), &flowNodes)

	for i := range flowNodes {
		if flowNodes[i].DefineUserOrGroup == "auto_order_robot" && flowNodes[i].ActualUser == "" {
			flowNodes[i].ActualUser = "auto_order_robot"
			flowNodes[i].EndTime = time.Now().Format("2006-01-02 15:04:05")
			flowNodes[i].IsPassOrIsSuccess = true
			flowNodes[i].OutPut = fmt.Sprintf("✅ 流水线任务开通成功！\n任务名称: %s\n所属目录: %s\nJenkins链接: %s\n服务基线已同步落库归档。",
				createdJob.Name, createdJob.ProjectName, createdJob.URL)
			break
		}
	}

	flownodeStr, _ := json.Marshal(flowNodes)
	dbObj.ActualFlowData = string(flownodeStr)
	dbObj.Status = common.WORKORDER_INSTANCE_FINISHED
	dbObj.CurrentFlowNode = ""

	_ = models.Db.Model(dbObj).Where("id = ?", dbObj.ID).Updates(map[string]interface{}{
		"status":            dbObj.Status,
		"current_flow_node": dbObj.CurrentFlowNode,
		"actual_flow_data":  dbObj.ActualFlowData,
		"final_run_data":    fmt.Sprintf("Jenkins Job: %s", createdJob.URL),
	}).Error

	sc.Logger.Info("工单机器人执行流水线开通成功", zap.Uint("orderId", dbObj.ID), zap.String("job", createdJob.Name))
}

func recordRobotExecutionFailure(sc *config.ServerConfig, dbObj *models.WorkOrderInstance, reason string) {
	var flowNodes []models.WorkOrderFlowNode
	_ = json.Unmarshal([]byte(dbObj.ActualFlowData), &flowNodes)

	for i := range flowNodes {
		if flowNodes[i].DefineUserOrGroup == "auto_order_robot" && flowNodes[i].ActualUser == "" {
			flowNodes[i].ActualUser = "auto_order_robot"
			flowNodes[i].EndTime = time.Now().Format("2006-01-02 15:04:05")
			flowNodes[i].IsPassOrIsSuccess = false
			flowNodes[i].OutPut = fmt.Sprintf("❌ 流水线自动开通失败: %s", reason)
			break
		}
	}

	flownodeStr, _ := json.Marshal(flowNodes)
	dbObj.ActualFlowData = string(flownodeStr)
	dbObj.Status = common.WORKORDER_INSTANCE_APPROVAL_REJECT
	dbObj.CurrentFlowNode = ""

	_ = models.Db.Model(dbObj).Where("id = ?", dbObj.ID).Updates(map[string]interface{}{
		"status":            dbObj.Status,
		"current_flow_node": dbObj.CurrentFlowNode,
		"actual_flow_data":  dbObj.ActualFlowData,
	}).Error

	sc.Logger.Error("工单机器人执行流水线开通失败", zap.Uint("orderId", dbObj.ID), zap.String("reason", reason))
}

func getFormString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok && val != nil {
		return fmt.Sprintf("%v", val)
	}
	return ""
}

func getFormUint(m map[string]interface{}, key string) uint {
	if val, ok := m[key]; ok && val != nil {
		switch v := val.(type) {
		case float64:
			return uint(v)
		case int:
			return uint(v)
		case string:
			parsed, _ := strconv.Atoi(v)
			return uint(parsed)
		}
	}
	return 0
}

func getFormStringSlice(m map[string]interface{}, key string) []string {
	res := make([]string, 0)
	val, ok := m[key]
	if !ok || val == nil {
		return res
	}
	switch v := val.(type) {
	case []interface{}:
		for _, item := range v {
			if item != nil {
				res = append(res, fmt.Sprintf("%v", item))
			}
		}
	case []string:
		return v
	case string:
		if v != "" {
			for _, part := range strings.Split(v, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					res = append(res, part)
				}
			}
		}
	}
	return res
}

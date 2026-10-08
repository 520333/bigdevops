package view_server

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bndr/gojenkins"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	jenkinsAnnotationRegex = regexp.MustCompile(`(ha|a):////[A-Za-z0-9+/=]+`)
	ansiConcealRegex       = regexp.MustCompile(`\x1b\[8m[^\x1b]*\x1b\[0m|\x1b\[8m[^\x1b]*|\x1b\[8m|\x1b\[0m\x1b\[8m`)
	unhandledAnsiRegex     = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b[()][A-B0-2]|\x1b[=><]|\x1b`)
)

// getJenkinsJobHelper 自动识别并拆解带有文件夹的层级结构以精确定位 Jenkins Job，杜绝返回 HTML 404 导致的 JSON '<' 解析报错
func getJenkinsJobHelper(ctx context.Context, client *gojenkins.Jenkins, jobName string, folder ...string) (*gojenkins.Job, error) {
	jobName = strings.Trim(strings.TrimSpace(jobName), "/")
	parent := ""
	for _, f := range folder {
		if strings.TrimSpace(f) != "" {
			parent = strings.Trim(strings.TrimSpace(f), "/")
			break
		}
	}

	// 如果 parent 为空且 jobName 不含斜杠，尝试查库补充获取其 ProjectName / Folder
	if parent == "" && !strings.Contains(jobName, "/") {
		var dbJob models.JenkinsJob
		if err := models.Db.Where("name = ?", jobName).First(&dbJob).Error; err == nil && dbJob.ProjectName != "" {
			parent = dbJob.ProjectName
		}
	}

	fullPath := jobName
	if parent != "" && !strings.HasPrefix(jobName, parent+"/") && jobName != parent {
		fullPath = parent + "/" + jobName
	}

	var job *gojenkins.Job
	var err error
	if !strings.Contains(fullPath, "/") {
		job, err = client.GetJob(ctx, fullPath)
	} else {
		parts := strings.Split(fullPath, "/")
		realName := parts[len(parts)-1]
		parentIDs := parts[:len(parts)-1]
		job, err = client.GetJob(ctx, realName, parentIDs...)
	}

	if (err != nil || job == nil) && !strings.Contains(jobName, "/") {
		topJobs, tErr := client.GetAllJobs(ctx)
		if tErr == nil {
			allJobs := getAllJobsRecursive(ctx, topJobs, "")
			for _, j := range allJobs {
				parts := strings.Split(j.Raw.Name, "/")
				shortName := parts[len(parts)-1]
				if shortName == jobName || j.Raw.Name == jobName {
					return j, nil
				}
			}
		}
	}

	return job, err
}

// deleteJenkinsJobHelper 自动理顺含文件夹路径的 Job，完美向Jenkins云间同步发起移除，不再踩坑 404 HTML 返回
func deleteJenkinsJobHelper(ctx context.Context, client *gojenkins.Jenkins, jobName string, folder ...string) error {
	job, err := getJenkinsJobHelper(ctx, client, jobName, folder...)
	if err == nil && job != nil {
		_, errDel := job.Delete(ctx)
		if errDel == nil || strings.Contains(errDel.Error(), "404") || strings.Contains(errDel.Error(), "invalid character") {
			return nil
		}
		return errDel
	}

	jobName = strings.Trim(strings.TrimSpace(jobName), "/")
	parent := ""
	if len(folder) > 0 && strings.TrimSpace(folder[0]) != "" {
		parent = strings.Trim(strings.TrimSpace(folder[0]), "/")
	}
	fullPath := jobName
	if parent != "" && !strings.HasPrefix(jobName, parent+"/") && jobName != parent {
		fullPath = parent + "/" + jobName
	}

	_, err = client.DeleteJob(ctx, fullPath)
	if err != nil && fullPath != jobName {
		_, err = client.DeleteJob(ctx, jobName)
	}
	if err != nil && (strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "invalid character")) {
		return nil
	}
	return err
}

func parseFolderAndJobName(folderInput, jobNameInput string) (string, string, string) {
	folderInput = strings.Trim(strings.TrimSpace(folderInput), "/")
	jobNameInput = strings.Trim(strings.TrimSpace(jobNameInput), "/")

	if folderInput != "" {
		if strings.HasPrefix(jobNameInput, folderInput+"/") {
			jobNameInput = strings.TrimPrefix(jobNameInput, folderInput+"/")
		}
		if strings.Contains(jobNameInput, "/") {
			parts := strings.Split(jobNameInput, "/")
			folderInput = folderInput + "/" + strings.Join(parts[:len(parts)-1], "/")
			jobNameInput = parts[len(parts)-1]
		}
		return folderInput, jobNameInput, folderInput + "/" + jobNameInput
	}

	if strings.Contains(jobNameInput, "/") {
		parts := strings.Split(jobNameInput, "/")
		folder := strings.Join(parts[:len(parts)-1], "/")
		realJobName := parts[len(parts)-1]
		return folder, realJobName, jobNameInput
	}

	return "", jobNameInput, jobNameInput
}

func getJenkinsClientFromCtx(c *gin.Context) (*gojenkins.Jenkins, uint, bool) {
	instanceIdStr := c.DefaultQuery("instanceId", c.DefaultQuery("instance_id", ""))
	if instanceIdStr == "" {
		common.ReqBadFailWithMessage("缺少 instanceId 参数", c)
		return nil, 0, false
	}
	id, _ := strconv.Atoi(instanceIdStr)

	jcVal, ok := c.Get(common.GIN_CTX_JENKINS_CACHE)
	if !ok {
		common.ReqBadFailWithMessage("JenkinsCache 未挂载", c)
		return nil, 0, false
	}

	jenkinsCache := jcVal.(*cache.JenkinsCache)
	client := jenkinsCache.GetJenkinsClientById(uint(id))
	if client == nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("对应实例 (ID: %d) 未找到或连接探活失败", id), c)
		return nil, uint(id), false
	}

	return client, uint(id), true
}

func mapJenkinsColorToStatus(color string) string {
	if strings.HasSuffix(color, "_anime") || color == "building" {
		return "BUILDING"
	}
	switch color {
	case "blue":
		return "SUCCESS"
	case "red":
		return "FAILURE"
	case "disabled", "aborted":
		return "ABORTED"
	case "yellow":
		return "UNSTABLE"
	case "notbuilt":
		return "NOT_BUILT"
	default:
		return "SUCCESS"
	}
}

func isJenkinsFolder(rawClass string) bool {
	cls := strings.ToLower(rawClass)
	return strings.Contains(cls, "folder")
}

func getAllJobsRecursive(ctx context.Context, jobs []*gojenkins.Job, parentPath string) []*gojenkins.Job {
	var list []*gojenkins.Job
	for _, j := range jobs {
		if j == nil {
			continue
		}
		if isJenkinsFolder(j.Raw.Class) {
			innerJobs, err := j.GetInnerJobs(ctx)
			if err == nil && len(innerJobs) > 0 {
				folderName := j.Raw.Name
				if parentPath != "" {
					folderName = parentPath + "/" + folderName
				}
				subList := getAllJobsRecursive(ctx, innerJobs, folderName)
				list = append(list, subList...)
			}
		} else {
			if parentPath != "" && !strings.Contains(j.Raw.Name, "/") {
				j.Raw.Name = parentPath + "/" + j.Raw.Name
			}
			list = append(list, j)
		}
	}
	return list
}

// SyncJenkinsJobsToDB 全量拉取同步 Jenkins 真实 Job 状态至持久库 (Pipeline脚本均不在 DB 下存储)
func SyncJenkinsJobsToDB(ctx context.Context, instanceId uint, client *gojenkins.Jenkins) error {
	topJobs, err := client.GetAllJobs(ctx)
	if err != nil {
		return err
	}
	allJobs := getAllJobsRecursive(ctx, topJobs, "")
	// 记录远端当前真实存在的所有 Job 唯一标识: "folder|jobName"
	remoteKeys := make(map[string]bool)

	for _, j := range allJobs {
		var count int64 = j.Raw.LastBuild.Number
		var lastBuildTime *time.Time
		if lb, lbErr := j.GetLastBuild(ctx); lbErr == nil && lb != nil {
			t := lb.GetTimestamp()
			lastBuildTime = &t
			if count == 0 {
				count = lb.GetBuildNumber()
			}
		}
		folder, shortJobName, _ := parseFolderAndJobName("", j.Raw.Name)
		remoteKeys[fmt.Sprintf("%s|%s", folder, shortJobName)] = true

		var existing models.JenkinsJob
		q := models.Db.Where("instance_id = ? AND name = ?", instanceId, shortJobName)
		if folder != "" {
			q = q.Where("project_name = ?", folder)
		} else {
			q = q.Where("project_name = '' OR project_name IS NULL")
		}
		_ = q.First(&existing).Error

		jobObj := &models.JenkinsJob{
			InstanceID:     instanceId,
			Name:           shortJobName,
			ProjectName:    folder, // Group名称 / 项目名称与文件夹呼应
			Count:          count,
			Status:         mapJenkinsColorToStatus(j.Raw.Color),
			URL:            j.Raw.URL,
			DeployType:     existing.DeployType,
			DeployEnv:      existing.DeployEnv,
			GitRepo:        existing.GitRepo,
			GitBranch:      existing.GitBranch,
			Lang:           existing.Lang,
			CreateUserName: existing.CreateUserName,
			EnableDelete: func() int {
				if existing.EnableDelete == 1 {
					return 1
				}
				return 2
			}(),
			LastBuildTime: func() *time.Time {
				if lastBuildTime != nil {
					return lastBuildTime
				}
				return existing.LastBuildTime
			}(),
		}
		if existing.GitRepo != "" {
			jobObj.GitRepo = existing.GitRepo
		}
		if existing.GitBranch != "" {
			jobObj.GitBranch = existing.GitBranch
		}
		if jobObj.Lang == "" {
			jobObj.Lang = "Java"
		}
		if jobObj.GitBranch == "" {
			jobObj.GitBranch = "main"
		}
		if existing.ProjectName != "" {
			jobObj.ProjectName = existing.ProjectName
		}
		_ = models.SaveOrUpdateJenkinsJob(jobObj)
	}

	// 自动差量清理：如果在 Jenkins 远端已被删除（包括整个 Folder 或单个 Job 被删），则同步清除本地 DB 中的孤儿记录
	var currentDbJobs []models.JenkinsJob
	if err := models.Db.Where("instance_id = ?", instanceId).Find(&currentDbJobs).Error; err == nil {
		for _, dbJob := range currentDbJobs {
			key := fmt.Sprintf("%s|%s", dbJob.ProjectName, dbJob.Name)
			if !remoteKeys[key] {
				_ = models.Db.Delete(&dbJob).Error
			}
		}
	}

	return nil
}

// getJenkinsJobList 获取 Job 列表接口 (基于全功能模糊及特定条件查询)
func getJenkinsJobList(c *gin.Context) {
	client, instanceId, ok := getJenkinsClientFromCtx(c)
	if !ok {
		return
	}

	var param models.JenkinsJobQueryParam
	_ = c.ShouldBindQuery(&param)
	if param.InstanceID == 0 && instanceId > 0 {
		param.InstanceID = instanceId
	}

	// 获取当前登录用户及系统配置
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var currentUser *models.SystemUser
	if userNameVal, ok := c.Get(common.GIN_CTX_JWT_USER_NAME); ok {
		userName := fmt.Sprintf("%v", userNameVal)
		currentUser, _ = models.GetUserByUsername(userName)
	}

	isSuper := models.IsUserSuperRole(currentUser, sc.SuperRoleName)
	forceSync := (c.Query("sync") == "true" || c.Query("refresh") == "true") && isSuper

	// 1. 如果请求显式要求同步且具备管理员权限，先从 Jenkins 远端拉取并同步
	if forceSync {
		_ = SyncJenkinsJobsToDB(c.Request.Context(), instanceId, client)
	}

	// 2. 从数据库提取条件结果 (严格应用角色行级物理隔离，无权限的作业在数据库层直接过滤无法看见)
	dbJobs, err := models.GetJenkinsJobListByParamWithUser(&param, currentUser, sc.SuperRoleName)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("获取 Job 列表失败: %v", err), c)
		return
	}

	if len(dbJobs) == 0 && !forceSync && isSuper {
		_ = SyncJenkinsJobsToDB(c.Request.Context(), instanceId, client)
		dbJobs, _ = models.GetJenkinsJobListByParamWithUser(&param, currentUser, sc.SuperRoleName)
	} else if !forceSync && isSuper {
		go func(instId uint, cli *gojenkins.Jenkins) {
			_ = SyncJenkinsJobsToDB(context.Background(), instId, cli)
		}(instanceId, client)
	}

	for _, job := range dbJobs {
		if isSuper {
			job.CanBuild = true
		} else {
			job.CanBuild = models.CheckJobOperationPermission(currentUser, sc.SuperRoleName, job.ProjectName, job.DeployEnv, job.Name, true)
		}
	}

	common.OkWithDetailed(gin.H{
		"items": dbJobs,
		"total": len(dbJobs),
	}, "获取成功", c)
}

// verifyJenkinsScriptSyntax 内部调用 Linter 检测管道语法，把挡在保存与远端通信的前期
func verifyJenkinsScriptSyntax(instanceId uint, script string) (bool, string) {
	var targetInst models.JenkinsInstance
	err := models.Db.Where("id = ?", instanceId).First(&targetInst).Error
	if err != nil || targetInst.URL == "" {
		return false, "查询Jenkins实例参数失败或未配置有效URL"
	}

	apiURL := fmt.Sprintf("%s/pipeline-model-converter/validate", strings.TrimRight(targetInst.URL, "/"))
	formData := url.Values{}
	formData.Set("jenkinsfile", script)

	httpReq, err := http.NewRequest("POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return false, fmt.Sprintf("创建 Linter 校验网络请求异常: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if targetInst.Username != "" && targetInst.ApiToken != "" {
		httpReq.SetBasicAuth(targetInst.Username, targetInst.ApiToken)
	}

	httpClient := &http.Client{Timeout: 8 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return false, fmt.Sprintf("调用Jenkins Linter 官方语法接口通信中断: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	respText := string(bodyBytes)
	if strings.Contains(respText, "Jenkinsfile successfully validated") {
		return true, "验证通过"
	}
	return false, respText
}

type createOrUpdateJobReq struct {
	RawID          interface{} `json:"id"`
	RawInstanceID  interface{} `json:"instanceId"`
	ID             uint        `json:"-"`
	InstanceID     uint        `json:"-"`
	DeployType     string      `json:"deployType"`
	DeployEnv      string      `json:"deployEnv"`
	Name           string      `json:"name"`
	JobName        string      `json:"jobName"`
	ProjectName    string      `json:"projectName"` // Git仓库group组名/文件夹名
	Folder         string      `json:"folder"`      // 支持可选入参 不入库
	GitRepo        string      `json:"gitRepo"`     // 仓库全息链接
	GitBranch      string      `json:"gitBranch"`   // 编译部署选定分支
	Lang           string      `json:"lang"`
	PipelineScript string      `json:"pipelineScript"` // 本字段不进数据库
	CreateUserName string      `json:"createUserName"`
	EnableDelete   int         `json:"enableDelete"` // 1 开启删除 2 禁止删除 默认为2
}

func (r *createOrUpdateJobReq) ParseIDs() {
	if r.RawID != nil {
		switch v := r.RawID.(type) {
		case float64:
			r.ID = uint(v)
		case int:
			r.ID = uint(v)
		case string:
			if num, err := strconv.Atoi(v); err == nil && num > 0 {
				r.ID = uint(num)
			}
		}
	}
	if r.RawInstanceID != nil {
		switch v := r.RawInstanceID.(type) {
		case float64:
			r.InstanceID = uint(v)
		case int:
			r.InstanceID = uint(v)
		case string:
			if num, err := strconv.Atoi(v); err == nil && num > 0 {
				r.InstanceID = uint(num)
			}
		}
	}
}

// createJenkinsJob 新建 Job (调用官方语法严审后同步建表与入远端端点，杜绝无效或脚本废案进库)
func createJenkinsJob(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var req createOrUpdateJobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		sc.Logger.Error("创建 Jenkins Job JSON 入参绑定失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("参数绑定错误: %v", err), c)
		return
	}
	req.ParseIDs()
	if req.InstanceID == 0 {
		common.ReqBadFailWithMessage("参数缺失: 需要合法的 instanceId 实例配置", c)
		return
	}

	jobName := req.JobName
	if jobName == "" {
		jobName = req.Name
	}
	if jobName == "" {
		common.ReqBadFailWithMessage("服务名(jobName)为必填属性", c)
		return
	}

	// 合并解析 Folder 参数：优先级为显性入参 folder > projectName
	folder := req.Folder
	if folder == "" && req.ProjectName != "" {
		folder = req.ProjectName
	}

	jcVal, ok := c.Get(common.GIN_CTX_JENKINS_CACHE)
	if !ok {
		common.ReqBadFailWithMessage("JenkinsCache 缓存实例挂载失败", c)
		return
	}
	client := jcVal.(*cache.JenkinsCache).GetJenkinsClientById(req.InstanceID)
	if client == nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("实例 (ID: %d) 未连接或状态离线", req.InstanceID), c)
		return
	}

	createdJob, err := doCreateJenkinsJobCore(c.Request.Context(), sc, client, req)
	if err != nil {
		sc.Logger.Error("创建 Jenkins Job 失败", zap.Error(err))
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("Job 任务及项目构架 '%s' 校验完好并落地建档成真！", createdJob.Name), c)
}

// doCreateJenkinsJobCore 纯粹底层的创建 Jenkins Job 逻辑，既供 API 控制器调用，也供工单系统机器人调用
func doCreateJenkinsJobCore(ctx context.Context, sc *config.ServerConfig, client *gojenkins.Jenkins, req createOrUpdateJobReq) (*models.JenkinsJob, error) {
	req.ParseIDs()
	if req.InstanceID == 0 {
		return nil, fmt.Errorf("参数缺失: 需要合法的 instanceId 实例配置")
	}

	jobName := req.JobName
	if jobName == "" {
		jobName = req.Name
	}
	if jobName == "" {
		return nil, fmt.Errorf("服务名(jobName)为必填属性")
	}

	folder := req.Folder
	if folder == "" && req.ProjectName != "" {
		folder = req.ProjectName
	}

	folder, realJobName, fullJobName := parseFolderAndJobName(folder, jobName)

	script := req.PipelineScript
	if script == "" {
		script = fmt.Sprintf("pipeline {\n    agent any\n    stages {\n        stage('Hello') {\n            steps {\n                echo 'Hello World from job: %s'\n            }\n        }\n    }\n}", fullJobName)
	}

	// 核心风控限制：提交时必须先调用官方语法检测，检测失败不允许远端创建！
	valid, errMsg := verifyJenkinsScriptSyntax(req.InstanceID, script)
	if !valid {
		sc.Logger.Warn("Jenkins Pipeline 语法终审被主动拒止", zap.String("error", errMsg))
		return nil, fmt.Errorf("⚠️ Jenkinsfile 官方面向语法解析不合规，已封锁远端创建申请: \n%s", errMsg)
	}

	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(script))
	escapedScript := buf.String()

	propertiesXml := GenerateJobPropertiesXml(script)

	xmlConfig := fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <description>Created via BigDevOps Baseline Architecture</description>
  <keepDependencies>false</keepDependencies>
  %s
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps">
    <script>%s</script>
    <sandbox>true</sandbox>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>`, propertiesXml, escapedScript)

	var err error
	var jenkinsURL string

	if folder != "" {
		folders := strings.Split(folder, "/")
		for i := range folders {
			subFolder := folders[i]
			subParent := folders[:i]
			if len(subParent) == 0 {
				_, _ = client.CreateFolder(ctx, subFolder)
			} else {
				_, _ = client.CreateFolder(ctx, subFolder, subParent...)
			}
		}
		_, err = client.CreateJobInFolder(ctx, xmlConfig, realJobName, folders...)
		jenkinsURL = fmt.Sprintf("%s/job/%s/job/%s/", strings.TrimRight(client.Server, "/"), strings.ReplaceAll(folder, "/", "/job/"), realJobName)
	} else {
		_, err = client.CreateJob(ctx, xmlConfig, realJobName)
		jenkinsURL = fmt.Sprintf("%s/job/%s/", strings.TrimRight(client.Server, "/"), realJobName)
	}

	if err != nil {
		sc.Logger.Error("调用远程 Jenkins 引擎创建作业过程报错", zap.Error(err))
		return nil, fmt.Errorf("远端创建执行发生错误: %v", err)
	}

	branch := req.GitBranch
	if branch == "" {
		branch = "main"
	}
	lang := req.Lang
	if lang == "" {
		lang = "Java"
	}

	// 新建存库规范：决不向底层存留未核或臃肿 PipelineScript 报文，完全依据精细化模型基线归档！
	dbObj := &models.JenkinsJob{
		InstanceID:     req.InstanceID,
		DeployType:     req.DeployType,
		DeployEnv:      req.DeployEnv,
		Name:           realJobName,
		ProjectName:    folder, // 与 GitLab 分组群相呼应
		GitRepo:        req.GitRepo,
		GitBranch:      branch,
		URL:            jenkinsURL,
		Lang:           lang,
		Count:          0,
		Status:         "NOT_BUILT",
		CreateUserName: req.CreateUserName,
		EnableDelete: func() int {
			if req.EnableDelete == 1 {
				return 1
			}
			return 2
		}(), // 1开启删除 2禁止删除 默认为2
	}
	if err := models.SaveOrUpdateJenkinsJob(dbObj); err != nil {
		return nil, fmt.Errorf("远端创建成功但保存基线失败: %v", err)
	}

	return dbObj, nil
}

// replaceJenkinsScriptInConfig 安全更新 Jenkins XML 中的脚本内容与参数定义，实现实时生效无需首跑激活
func replaceJenkinsScriptInConfig(oldConfig string, escapedScript string, rawScript string) string {
	res := oldConfig

	// 1. 若新脚本包含参数定义或防并发设置，智能同步/更新 <properties> 节点
	newPropsXml := GenerateJobPropertiesXml(rawScript)
	if strings.Contains(newPropsXml, "parameterDefinitions") || strings.Contains(newPropsXml, "DisableConcurrentBuildsJobProperty") {
		propStart := strings.Index(res, "<properties>")
		propEnd := strings.Index(res, "</properties>")
		if propStart != -1 && propEnd != -1 && propEnd > propStart {
			res = res[:propStart] + strings.TrimSpace(newPropsXml) + res[propEnd+len("</properties>"):]
		} else {
			selfCloseIdx := strings.Index(res, "<properties/>")
			if selfCloseIdx != -1 {
				res = res[:selfCloseIdx] + strings.TrimSpace(newPropsXml) + res[selfCloseIdx+len("<properties/>"):]
			}
		}
	}

	// 2. 替换 <script> 节点
	startTag := "<script>"
	endTag := "</script>"
	startIdx := strings.Index(res, startTag)
	endIdx := strings.Index(res, endTag)
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		return res[:startIdx+len(startTag)] + escapedScript + res[endIdx:]
	}

	// 3. 兜底：如果原有配置没有 <script> 标签，将解析的 properties 或原有 properties 移植入新配置模板
	propertiesBlock := "<properties/>"
	if strings.TrimSpace(newPropsXml) != "" && newPropsXml != "<properties/>" {
		propertiesBlock = newPropsXml
	} else {
		propRegex := regexp.MustCompile(`(?s)<properties>.*?</properties>`)
		if match := propRegex.FindString(oldConfig); match != "" {
			propertiesBlock = match
		}
	}
	return fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <description>Updated via BigDevOps Baseline Architecture</description>
  <keepDependencies>false</keepDependencies>
  %s
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps">
    <script>%s</script>
    <sandbox>true</sandbox>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>`, propertiesBlock, escapedScript)
}

// updateJenkinsJob 更新 Job (直接提交更新至远端配置不变动 DB 的 PipelineScript)
func updateJenkinsJob(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var req createOrUpdateJobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		sc.Logger.Error("更新 Jenkins Job JSON 入参绑定失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("参数绑定错误: %v", err), c)
		return
	}
	req.ParseIDs()
	if req.InstanceID == 0 {
		common.ReqBadFailWithMessage("缺少有效的 Jenkins 实例 ID (instanceId)", c)
		return
	}

	jobName := req.JobName
	if jobName == "" {
		jobName = req.Name
	}

	jcVal, ok := c.Get(common.GIN_CTX_JENKINS_CACHE)
	if !ok {
		common.ReqBadFailWithMessage("JenkinsCache 缓存故障", c)
		return
	}
	client := jcVal.(*cache.JenkinsCache).GetJenkinsClientById(req.InstanceID)
	if client == nil {
		common.ReqBadFailWithMessage("无法联线所指定的云 Jenkins 系统实例", c)
		return
	}

	folder := req.Folder
	if folder == "" && req.ProjectName != "" {
		folder = req.ProjectName
	}
	folder, realJobName, fullJobName := parseFolderAndJobName(folder, jobName)

	script := req.PipelineScript
	if script != "" {
		valid, errMsg := verifyJenkinsScriptSyntax(req.InstanceID, script)
		if !valid {
			common.ReqBadFailWithMessage(fmt.Sprintf("⚠️ Jenkins 语法检疫拒绝: \n%s", errMsg), c)
			return
		}

		var buf bytes.Buffer
		xml.EscapeText(&buf, []byte(script))
		escapedScript := buf.String()
		propertiesXml := GenerateJobPropertiesXml(script)
		xmlConfig := fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <description>Updated via BigDevOps Baseline Architecture</description>
  <keepDependencies>false</keepDependencies>
  %s
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps">
    <script>%s</script>
    <sandbox>true</sandbox>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>`, propertiesXml, escapedScript)

		ctx := c.Request.Context()
		job, err := getJenkinsJobHelper(ctx, client, fullJobName, folder)
		if err == nil && job != nil {
			targetXmlConfig := xmlConfig
			oldConfig, getConfigErr := job.GetConfig(ctx)
			if getConfigErr == nil && strings.TrimSpace(oldConfig) != "" {
				targetXmlConfig = replaceJenkinsScriptInConfig(oldConfig, escapedScript, script)
			}
			if updateErr := job.UpdateConfig(ctx, targetXmlConfig); updateErr != nil {
				sc.Logger.Error("向 Jenkins 写入新流水线配置失败", zap.Error(updateErr))
				common.ReqBadFailWithMessage(fmt.Sprintf("向 Jenkins 写入新流水线失败: %v", updateErr), c)
				return
			}
		} else {
			if folder != "" {
				folders := strings.Split(folder, "/")
				for i := range folders {
					subFolder := folders[i]
					subParent := folders[:i]
					if len(subParent) == 0 {
						_, _ = client.CreateFolder(ctx, subFolder)
					} else {
						_, _ = client.CreateFolder(ctx, subFolder, subParent...)
					}
				}
				_, _ = client.CreateJobInFolder(ctx, xmlConfig, realJobName, folders...)
			} else {
				_, _ = client.CreateJob(ctx, xmlConfig, realJobName)
			}
		}
	}

	var dbJob models.JenkinsJob
	var found bool
	if req.ID > 0 {
		if err := models.Db.Where("id = ?", req.ID).First(&dbJob).Error; err == nil {
			found = true
		}
	}
	if !found {
		q := models.Db.Where("instance_id = ? AND name = ?", req.InstanceID, realJobName)
		if folder != "" {
			q = q.Where("project_name = ?", folder)
		} else {
			q = q.Where("project_name = '' OR project_name IS NULL")
		}
		if err := q.First(&dbJob).Error; err == nil {
			found = true
		}
	}

	enableDelete := req.EnableDelete
	if enableDelete != 1 && enableDelete != 2 {
		if found && (dbJob.EnableDelete == 1 || dbJob.EnableDelete == 2) {
			enableDelete = dbJob.EnableDelete
		} else {
			enableDelete = 2
		}
	}

	if !found {
		dbJob = models.JenkinsJob{
			InstanceID:     req.InstanceID,
			Name:           realJobName,
			ProjectName:    folder,
			DeployType:     req.DeployType,
			DeployEnv:      req.DeployEnv,
			GitRepo:        req.GitRepo,
			GitBranch:      req.GitBranch,
			Lang:           req.Lang,
			CreateUserName: req.CreateUserName,
			EnableDelete:   enableDelete,
		}
	} else {
		dbJob.InstanceID = req.InstanceID
		dbJob.DeployType = req.DeployType
		dbJob.DeployEnv = req.DeployEnv
		dbJob.Name = realJobName
		dbJob.ProjectName = folder
		dbJob.GitRepo = req.GitRepo
		if req.GitBranch != "" {
			dbJob.GitBranch = req.GitBranch
		}
		if req.Lang != "" {
			dbJob.Lang = req.Lang
		}
		if req.CreateUserName != "" {
			dbJob.CreateUserName = req.CreateUserName
		}
		dbJob.EnableDelete = enableDelete
	}

	if err := models.SaveOrUpdateJenkinsJob(&dbJob); err != nil {
		sc.Logger.Error("执行同步变更基线字段失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("记录持久化更新失败: %v", err), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("Job 基础建树 '%s' 参数与云端脚本完满再版成功！", fullJobName), c)
}

func extractScriptFromXml(xmlContent string) string {
	// 1. 优先从 <definition> 标签块中精准提取流水线脚本
	defIdx := strings.Index(xmlContent, "<definition ")
	if defIdx == -1 {
		defIdx = strings.Index(xmlContent, "<definition>")
	}

	if defIdx != -1 {
		defEnd := strings.Index(xmlContent[defIdx:], "</definition>")
		var defContent string
		if defEnd != -1 {
			defContent = xmlContent[defIdx : defIdx+defEnd+13]
		} else {
			defContent = xmlContent[defIdx:]
		}

		startTag := "<script>"
		endTag := "</script>"
		startIdx := strings.Index(defContent, startTag)
		if startIdx != -1 {
			startIdx += len(startTag)
			endIdx := strings.LastIndex(defContent, endTag)
			if endIdx > startIdx {
				return unescapeJenkinsXml(defContent[startIdx:endIdx])
			}
		}

		if strings.Contains(defContent, "CpsScmFlowDefinition") {
			return "// 此任务配置为【Pipeline script from SCM】模式 (直接从代码仓库拉取 Jenkinsfile)\n// 任务脚本由代码仓库维护。"
		}
	}

	// 2. 兜底方案：寻找包含 pipeline 关键字的 script 块
	re := regexp.MustCompile(`(?s)<script>(.*?pipeline\s*\{.*?)</script>`)
	matches := re.FindStringSubmatch(xmlContent)
	if len(matches) > 1 {
		return unescapeJenkinsXml(matches[1])
	}

	return ""
}

func extractGitRepoFromXml(xmlContent string) string {
	// 匹配常见 git url 格式
	re := regexp.MustCompile(`(?i)(?:https?|ssh|git)://[^\s"'<>\&]+?\.git`)
	matches := re.FindAllString(xmlContent, -1)
	for _, m := range matches {
		if !strings.Contains(m, "gitlab.com") || len(matches) == 1 {
			return m
		}
	}
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func unescapeJenkinsXml(script string) string {
	script = strings.ReplaceAll(script, "&amp;", "&")
	script = strings.ReplaceAll(script, "&lt;", "<")
	script = strings.ReplaceAll(script, "&gt;", ">")
	script = strings.ReplaceAll(script, "&quot;", "\"")
	script = strings.ReplaceAll(script, "&apos;", "'")
	return strings.TrimSpace(script)
}

// getJenkinsJobRemotePipeline 独占查询方法：拉取远端的 Pipeline 脚本(不落库纯在线取样渲染)
func getJenkinsJobRemotePipeline(c *gin.Context) {
	client, instanceId, ok := getJenkinsClientFromCtx(c)
	if !ok {
		return
	}

	jobName := c.Query("jobName")
	folder := c.DefaultQuery("folder", c.Query("projectName"))
	if jobName == "" {
		common.ReqBadFailWithMessage("需附带目标任务全称名(jobName)", c)
		return
	}

	ctx := c.Request.Context()
	job, err := getJenkinsJobHelper(ctx, client, jobName, folder)
	if err != nil || job == nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("无法从指定的 Jenkins 上定位并获取 Job: %v", err), c)
		return
	}

	configXml, err := job.GetConfig(ctx)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("向 Jenkins 请求 XML 设置流过程中超时或者被拒: %v", err), c)
		return
	}

	script := extractScriptFromXml(configXml)
	gitRepo := extractGitRepoFromXml(configXml)

	// 如果数据库中该 Job 的 git_repo 为空，顺便异步回填更新到本地数据库
	if gitRepo != "" && instanceId > 0 {
		go func(instId uint, jName, repo string) {
			_ = models.Db.Model(&models.JenkinsJob{}).
				Where("instance_id = ? AND (name = ? OR name LIKE ?)", instId, jName, "%"+jName).
				Where("git_repo IS NULL OR git_repo = ''").
				Update("git_repo", repo).Error
		}(instanceId, jobName, gitRepo)
	}

	common.OkWithData(gin.H{
		"pipelineScript": script,
		"gitRepo":        gitRepo,
		"rawXml":         configXml,
	}, c)
}

type toggleDeleteLockReq struct {
	InstanceID  uint        `json:"instanceId"`
	JobName     string      `json:"jobName"`
	Enable      interface{} `json:"enable"` // 1 开启删除 2 禁止删除 默认为2
	Folder      string      `json:"folder"`
	ProjectName string      `json:"projectName"`
}

// toggleJenkinsJobDeleteLock 创建job后锁定 开关控制 开启后删除按钮可以使用 关闭时删除按钮禁用
func toggleJenkinsJobDeleteLock(c *gin.Context) {
	var req toggleDeleteLockReq
	if err := c.ShouldBindJSON(&req); err != nil || req.InstanceID == 0 || req.JobName == "" {
		common.ReqBadFailWithMessage("请求载体缺损，核实入参状态", c)
		return
	}

	fullName := req.JobName
	folder := req.Folder
	if folder == "" {
		folder = req.ProjectName
	}
	if folder != "" && !strings.HasPrefix(req.JobName, folder+"/") && req.JobName != folder {
		fullName = folder + "/" + req.JobName
	}

	targetVal := 2
	switch v := req.Enable.(type) {
	case float64:
		if int(v) == 1 {
			targetVal = 1
		}
	case int:
		if v == 1 {
			targetVal = 1
		}
	case bool:
		if v {
			targetVal = 1
		}
	}

	err := models.Db.Model(&models.JenkinsJob{}).
		Where("instance_id = ? AND (name = ? OR name = ?)", req.InstanceID, req.JobName, fullName).
		Update("enable_delete", targetVal).Error

	if err != nil {
		common.ReqBadFailWithMessage("变更专属安全锁状态失败！", c)
		return
	}
	statusText := "禁止删除 (默认防误删安全态)"
	if targetVal == 1 {
		statusText = "开启删除 (此任务已开放删除)"
	}
	common.OkWithMessage(fmt.Sprintf("任务 [%s] 防护状态更新为: %s", req.JobName, statusText), c)
}

// deleteJenkinsJob 删除 Job (严格联动锁定拦截审核)
func deleteJenkinsJob(c *gin.Context) {
	client, instanceId, ok := getJenkinsClientFromCtx(c)
	if !ok {
		return
	}

	jobName := c.Query("jobName")
	folder := c.DefaultQuery("folder", c.Query("projectName"))
	if jobName == "" {
		common.ReqBadFailWithMessage("必须具有明确的待删标点及作业代号", c)
		return
	}

	fullJobName := jobName
	if folder != "" && !strings.HasPrefix(jobName, folder+"/") && jobName != folder {
		fullJobName = folder + "/" + jobName
	}

	var existing models.JenkinsJob
	err := models.Db.Where("instance_id = ? AND (name = ? OR name = ?)", instanceId, jobName, fullJobName).First(&existing).Error
	if err == nil && existing.EnableDelete != 1 {
		common.ReqBadFailWithMessage(fmt.Sprintf("安全风控触发：作业 '%s' 目前处于禁止删除状态！如需删除请在编辑配置中开启删除权限！", jobName), c)
		return
	}

	err = deleteJenkinsJobHelper(c.Request.Context(), client, jobName, folder)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("发起 Jenkins 云间真实删除工单未获正常回应: %v", err), c)
		return
	}

	_ = models.Db.Where("instance_id = ? AND (name = ? OR name = ?)", instanceId, jobName, fullJobName).Delete(&models.JenkinsJob{}).Error
	common.OkWithMessage(fmt.Sprintf("作业服务架构 '%s' 及其一切挂载目录记录已干净无残留消除！", jobName), c)
}

type triggerBuildReq struct {
	InstanceID     uint                   `json:"instanceId"`
	JobName        string                 `json:"jobName"`
	Branch         string                 `json:"branch"`
	DeployType     string                 `json:"deployType"` // 允许触发时动态改变或更新发布选点与策略
	GitRepo        string                 `json:"gitRepo"`
	DeployEnv      string                 `json:"deployEnv"`
	Folder         string                 `json:"folder"`
	ProjectName    string                 `json:"projectName"`
	CreateUserName string                 `json:"createUserName"`
	ScanCode       bool                   `json:"scanCode"`
	BuildNode      string                 `json:"buildNode"`
	JdkVersion     string                 `json:"jdkVersion"`
	BuildCommand   string                 `json:"buildCommand"`
	Module         string                 `json:"module"`
	ConfigFile     string                 `json:"configFile"`
	Port           string                 `json:"port"`
	TargetHost     string                 `json:"targetHost"`
	CustomParams   map[string]interface{} `json:"customParams"`
}

// executeJenkinsGroovy 向 Jenkins 发送并执行 Groovy 脚本，返回纯文本响应
func executeJenkinsGroovy(ctx context.Context, inst *models.JenkinsInstance, script string) (string, error) {
	data := url.Values{}
	data.Set("script", script)
	endpoint := strings.TrimRight(inst.URL, "/") + "/scriptText"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(inst.Username, inst.ApiToken)

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

// ensureUserJenkinsToken 方案2：自动为用户代生成并代管 Jenkins 专属 API Token
func ensureUserJenkinsToken(ctx context.Context, logger *zap.Logger, inst *models.JenkinsInstance, username string) (string, error) {
	if username == "" || strings.EqualFold(username, inst.Username) {
		return inst.ApiToken, nil
	}

	// 1. 优先从数据库查询是否已为该用户生成并代管了专属 API Token
	userToken, err := models.GetJenkinsUserToken(inst.ID, username)
	if err == nil && userToken != "" {
		if logger != nil {
			logger.Info("[Jenkins专属Token] 命中本地代管Token",
				zap.String("username", username),
				zap.Uint("instanceId", inst.ID))
		}
		return userToken, nil
	}

	if logger != nil {
		logger.Info("[Jenkins专属Token] 本地无该用户Token，正在通过管理员权限为用户自动生成专属Token...",
			zap.String("username", username),
			zap.Uint("instanceId", inst.ID))
	}

	groovyScript := fmt.Sprintf(`
try {
    def user = hudson.model.User.getById("%s", true)
    if (user != null) {
        def prop = user.getProperty(jenkins.security.ApiTokenProperty.class)
        if (prop == null) {
            prop = new jenkins.security.ApiTokenProperty()
            user.addProperty(prop)
        }
        def result = prop.tokenStore.generateNewToken("bigdevops-auto-token")
        user.save()
        def tokenVal = result.hasProperty("plainValue") ? result.plainValue : (result.hasProperty("tokenValue") ? result.tokenValue : result.toString())
        println "TOKEN_OK:" + tokenVal
    } else {
        println "ERROR:USER_NOT_FOUND"
    }
} catch(Exception e) {
    println "ERROR:" + e.getMessage()
}
`, username)

	respContent, err := executeJenkinsGroovy(ctx, inst, groovyScript)
	if err != nil {
		if logger != nil {
			logger.Warn("[Jenkins专属Token] Groovy执行请求失败", zap.Error(err))
		}
		return "", err
	}

	if strings.Contains(respContent, "TOKEN_OK:") {
		idx := strings.Index(respContent, "TOKEN_OK:")
		newToken := strings.TrimSpace(respContent[idx+len("TOKEN_OK:"):])
		if endIdx := strings.IndexAny(newToken, "\r\n"); endIdx != -1 {
			newToken = strings.TrimSpace(newToken[:endIdx])
		}
		if newToken != "" {
			_ = models.SaveJenkinsUserToken(inst.ID, username, newToken)
			if logger != nil {
				logger.Info("[Jenkins专属Token] 成功为用户自动生成并代管专属Token！",
					zap.String("username", username),
					zap.Uint("instanceId", inst.ID))
			}
			return newToken, nil
		}
	}

	if logger != nil {
		logger.Warn("[Jenkins专属Token] 自动生成专属Token未成功",
			zap.String("username", username),
			zap.String("respContent", respContent))
	}
	return "", fmt.Errorf("生成专属Token失败: %s", respContent)
}

// triggerJenkinsBuildWithCause 原生注入操作人的 UserIdCause 调度 Jenkins 构建，并安全透传所有参数
func triggerJenkinsBuildWithCause(ctx context.Context, inst *models.JenkinsInstance, jobFullName string, operator string, params map[string]string) (int64, error) {
	var paramStatements []string
	for k, v := range params {
		escapedK := strings.ReplaceAll(k, "\"", "\\\"")
		escapedV := strings.ReplaceAll(v, "\"", "\\\"")
		paramStatements = append(paramStatements, fmt.Sprintf(`paramValues.add(new StringParameterValue("%s", "%s"))`, escapedK, escapedV))
	}

	groovyScript := fmt.Sprintf(`
import hudson.model.*

def job = jenkins.model.Jenkins.instance.getItemByFullName("%s")
if (job == null) {
    println "ERROR:JOB_NOT_FOUND"
    return
}

def cause = new Cause.UserIdCause("%s")
def causeAction = new CauseAction(cause)

def paramValues = []
%s

def actions = [causeAction]
if (!paramValues.isEmpty()) {
    actions.add(new ParametersAction(paramValues))
}

def queueItem = jenkins.model.Jenkins.instance.queue.schedule2(job, 0, actions).getItem()
if (queueItem != null) {
    println "QUEUE_OK:" + queueItem.getId()
} else {
    println "ERROR:QUEUE_FAILED"
}
`, jobFullName, operator, strings.Join(paramStatements, "\n"))

	respContent, err := executeJenkinsGroovy(ctx, inst, groovyScript)
	if err != nil {
		return 0, err
	}

	if strings.Contains(respContent, "QUEUE_OK:") {
		idx := strings.Index(respContent, "QUEUE_OK:")
		queueStr := strings.TrimSpace(respContent[idx+len("QUEUE_OK:"):])
		if endIdx := strings.IndexAny(queueStr, "\r\n"); endIdx != -1 {
			queueStr = strings.TrimSpace(queueStr[:endIdx])
		}
		return strconv.ParseInt(queueStr, 10, 64)
	}

	return 0, fmt.Errorf("调度响应异常: %s", respContent)
}

// triggerJenkinsBuild 触发构建部署 (构建时可带入并承载分级环境变更命令与快调)
func triggerJenkinsBuild(c *gin.Context) {
	var req triggerBuildReq
	if err := c.ShouldBindJSON(&req); err != nil || req.InstanceID == 0 || req.JobName == "" {
		common.ReqBadFailWithMessage("触发展望入参存在短缺异常", c)
		return
	}

	var logger *zap.Logger
	if scVal, ok := c.Get(common.GIN_CTX_CONFIG_CONFIG); ok {
		if sc, ok := scVal.(*config.ServerConfig); ok {
			logger = sc.Logger
		}
	}

	// 优先取登录人真实账号（Keycloak 登录名，如 xin），杜绝中文真实姓名干扰
	operator := ""
	if userNameVal, ok := c.Get(common.GIN_CTX_JWT_USER_NAME); ok {
		operator = strings.TrimSpace(fmt.Sprintf("%v", userNameVal))
	}
	if operator == "" {
		operator = strings.TrimSpace(req.CreateUserName)
	}

	ctx := c.Request.Context()

	// 数据权限安全校验：判断当前用户所属角色是否被授予该项目、该部署环境的构建权限
	if scVal, ok := c.Get(common.GIN_CTX_CONFIG_CONFIG); ok {
		if sc, ok := scVal.(*config.ServerConfig); ok {
			var dbUser *models.SystemUser
			if operator != "" {
				dbUser, _ = models.GetUserByUsername(operator)
			}
			targetProject := req.ProjectName
			if targetProject == "" {
				targetProject = req.Folder
			}
			targetEnv := ""
			var localJob models.JenkinsJob
			if err := models.Db.Where("instance_id = ? AND name = ?", req.InstanceID, req.JobName).First(&localJob).Error; err == nil {
				if targetProject == "" {
					targetProject = localJob.ProjectName
				}
				targetEnv = localJob.DeployEnv
			}
			if !models.CheckJobOperationPermission(dbUser, sc.SuperRoleName, targetProject, targetEnv, req.JobName, true) {
				common.Req403WithMessage(fmt.Sprintf("权限拒绝：您所属角色无权对作业 [%s] 发起构建部署！", req.JobName), c)
				return
			}
		}
	}

	inst, err := models.GetJenkinsInstanceById(req.InstanceID)
	if err != nil || inst == nil {
		common.ReqBadFailWithMessage("Jenkins 实例配置不存在", c)
		return
	}

	// 方案2：为当前操作人代管/代生成 Jenkins 专属 Token (存入 jenkins_user_token)
	if operator != "" {
		_, _ = ensureUserJenkinsToken(ctx, logger, inst, operator)
	}

	jcVal, ok := c.Get(common.GIN_CTX_JENKINS_CACHE)
	if !ok {
		common.ReqBadFailWithMessage("缓存层系统断裂", c)
		return
	}
	client := jcVal.(*cache.JenkinsCache).GetJenkinsClientById(req.InstanceID)
	if client == nil {
		common.ReqBadFailWithMessage("宿主节点尚离线或是超时拒接中", c)
		return
	}

	job, err := getJenkinsJobHelper(ctx, client, req.JobName, req.Folder, req.ProjectName)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("解析远程任务定义发生奔溃或对象已失联: %v", err), c)
		return
	}

	// 防重复构建校验：如果远端任务正在排队中，或者当前最后一次构建仍在运行中，阻断并发重复触发
	if job.Raw.InQueue {
		common.ReqBadFailWithMessage(fmt.Sprintf("任务 [%s] 当前已在 Jenkins 调度队列中排队，请勿重复触发！", req.JobName), c)
		return
	}
	if lb, lbErr := job.GetLastBuild(ctx); lbErr == nil && lb != nil {
		if lb.IsRunning(ctx) {
			common.ReqBadFailWithMessage(fmt.Sprintf("任务 [%s] 当前正在构建中（构建号 #%d），请等待当前构建结束后再触发！", req.JobName, lb.GetBuildNumber()), c)
			return
		}
	}

	params := make(map[string]string)
	updates := map[string]interface{}{
		"status": "BUILDING",
	}

	if operator != "" {
		updates["create_user_name"] = operator
	}

	branch := strings.TrimSpace(req.Branch)
	if branch == "" && req.CustomParams != nil {
		for k, v := range req.CustomParams {
			lk := strings.ToLower(strings.TrimSpace(k))
			if lk == "branch" || lk == "gitbranch" || lk == "git_branch" || lk == "branchname" || strings.Contains(k, "分支") {
				if strVal := strings.TrimSpace(fmt.Sprintf("%v", v)); strVal != "" {
					branch = strVal
					break
				}
			}
		}
	}
	if branch != "" {
		updates["git_branch"] = branch
	}
	if req.DeployType != "" {
		updates["deploy_type"] = req.DeployType
	}
	if req.GitRepo != "" {
		updates["git_repo"] = req.GitRepo
	}
	if req.DeployEnv != "" {
		updates["deploy_env"] = req.DeployEnv
	}

	// 纯净模式：严禁硬编码注入 BUILD_USER、OPERATOR 等变量
	// 完全以用户表单提交的真实参数 (CustomParams) 精准透传给 Jenkins
	for k, v := range req.CustomParams {
		key := strings.TrimSpace(k)
		if key != "" && v != nil {
			params[key] = fmt.Sprintf("%v", v)
		}
	}

	jobFullName := req.JobName
	if req.Folder != "" && !strings.HasPrefix(req.JobName, req.Folder+"/") {
		jobFullName = req.Folder + "/" + req.JobName
	} else if req.ProjectName != "" && !strings.HasPrefix(req.JobName, req.ProjectName+"/") {
		jobFullName = req.ProjectName + "/" + req.JobName
	}

	var queueID int64
	// 优先使用 UserIdCause 调度，确保 Jenkins 构建记录原生就是 Started by user <operator>
	if operator != "" {
		queueID, err = triggerJenkinsBuildWithCause(ctx, inst, jobFullName, operator, params)
	}

	// 若未指定 operator 或 Cause 调度失败，优雅回退到 client.InvokeSimple
	if queueID == 0 || err != nil {
		if err != nil && logger != nil {
			logger.Warn("[Jenkins构建调度] UserIdCause调度异常，回退到默认客户端调度", zap.Error(err))
		}
		queueID, err = job.InvokeSimple(ctx, params)
		if err != nil {
			if logger != nil {
				logger.Error("[Jenkins构建调度失败]", zap.String("operator", operator), zap.String("jobName", req.JobName), zap.Error(err))
			}
			common.ReqBadFailWithMessage(fmt.Sprintf("调度触发流失速报错: %v", err), c)
			return
		}
	}

	if logger != nil {
		logger.Info("[Jenkins构建调度成功]",
			zap.String("operator", operator),
			zap.String("jobName", req.JobName),
			zap.Int64("queueID", queueID))
	}

	var buildNumber int64 = 0
	now := time.Now()
	updates["last_build_time"] = &now
	// 尝试快速短轮询（300ms x 6次），尽量在首次响应时直接拿到真实构建号
	for i := 0; i < 6; i++ {
		build, _ := client.GetBuildFromQueueID(ctx, job, queueID)
		if build != nil {
			buildNumber = build.GetBuildNumber()
			updates["count"] = buildNumber
			t := build.GetTimestamp()
			if !t.IsZero() {
				updates["last_build_time"] = &t
			}
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	targetJobName := req.JobName
	if strings.Contains(targetJobName, "/") {
		parts := strings.Split(targetJobName, "/")
		targetJobName = parts[len(parts)-1]
	}
	dbQuery := models.Db.Model(&models.JenkinsJob{}).Where("instance_id = ? AND name = ?", req.InstanceID, targetJobName)
	if req.Folder != "" {
		dbQuery = dbQuery.Where("project_name = ?", req.Folder)
	} else if req.ProjectName != "" {
		dbQuery = dbQuery.Where("project_name = ?", req.ProjectName)
	}
	_ = dbQuery.Updates(updates).Error

	common.OkWithData(gin.H{
		"queueId":     queueID,
		"buildNumber": buildNumber,
	}, c)
}

type stopBuildReq struct {
	InstanceID  uint   `json:"instanceId"`
	JobName     string `json:"jobName"`
	BuildNumber int64  `json:"buildNumber"`
	Folder      string `json:"folder"`
	ProjectName string `json:"projectName"`
}

// getJenkinsBuildSafe 安全获取目标构建对象，规避 gojenkins 库在 Jenkins 域名反代/内网IP不一致时的 URL 拼接 Bug
func getJenkinsBuildSafe(ctx context.Context, job *gojenkins.Job, buildNum int64) (*gojenkins.Build, error) {
	if buildNum <= 0 {
		return job.GetLastBuild(ctx)
	}

	// 1. 如果刚好是最后一次构建
	if lb, err := job.GetLastBuild(ctx); err == nil && lb != nil && lb.GetBuildNumber() == buildNum {
		return lb, nil
	}

	// 2. 使用稳定的相对 Base 路径规避原生库的 URL 拼接 Bug
	base := fmt.Sprintf("%s/%d", strings.TrimRight(job.Base, "/"), buildNum)
	build := &gojenkins.Build{
		Jenkins: job.Jenkins,
		Depth:   1,
		Job:     job,
		Raw:     new(gojenkins.BuildResponse),
		Base:    base,
	}
	status, err := build.Poll(ctx)
	if err == nil && status == 200 {
		return build, nil
	}

	// 兜底尝试原生方法
	return job.GetBuild(ctx, buildNum)
}

func stopJenkinsBuild(c *gin.Context) {
	var req stopBuildReq
	if err := c.ShouldBindJSON(&req); err != nil || req.InstanceID == 0 || req.JobName == "" {
		common.ReqBadFailWithMessage("请求内容缺少基础支撑", c)
		return
	}

	jcVal, ok := c.Get(common.GIN_CTX_JENKINS_CACHE)
	if !ok {
		return
	}
	client := jcVal.(*cache.JenkinsCache).GetJenkinsClientById(req.InstanceID)
	if client == nil {
		return
	}

	ctx := c.Request.Context()
	job, err := getJenkinsJobHelper(ctx, client, req.JobName, req.Folder, req.ProjectName)
	if err != nil {
		common.ReqBadFailWithMessage("作业获取遇到异常", c)
		return
	}

	build, err := getJenkinsBuildSafe(ctx, job, req.BuildNumber)
	if err != nil || build == nil {
		common.ReqBadFailWithMessage("远端 Jenkins 不存在目标构建序列号号段", c)
		return
	}

	_, err = build.Stop(ctx)
	if err != nil && !strings.Contains(err.Error(), "invalid character") {
		common.ReqBadFailWithMessage(fmt.Sprintf("对流水执行的截断下撤未响应: %v", err), c)
		return
	}

	_ = models.Db.Model(&models.JenkinsJob{}).Where("instance_id = ? AND name = ?", req.InstanceID, req.JobName).Update("status", "ABORTED").Error

	common.OkWithMessage(fmt.Sprintf("构建进程 #%d 执行紧急收归终止动作指令下沉完毕！", build.GetBuildNumber()), c)
}

func stripJenkinsAnnotations(logStr string) string {
	if logStr == "" {
		return ""
	}
	logStr = ansiConcealRegex.ReplaceAllString(logStr, "")
	logStr = jenkinsAnnotationRegex.ReplaceAllString(logStr, "")
	return logStr
}

// getJenkinsBuildLogs 增量拉取彩色日志接口，同步修正并写进当前工作真实状态与期数 (Count / Status)
func getJenkinsBuildLogs(c *gin.Context) {
	client, instanceId, ok := getJenkinsClientFromCtx(c)
	if !ok {
		return
	}

	jobName := c.DefaultQuery("jobName", c.DefaultQuery("job_name", c.Query("name")))
	folder := c.DefaultQuery("folder", c.Query("projectName"))
	buildNumStr := c.DefaultQuery("buildNumber", c.DefaultQuery("build_number", c.Query("buildNum")))
	offsetStr := c.DefaultQuery("offset", "0")
	buildNum, _ := strconv.ParseInt(buildNumStr, 10, 64)
	offset, _ := strconv.ParseInt(offsetStr, 10, 64)

	if jobName == "" {
		common.ReqBadFailWithMessage("未能判定监控哪个指定系统名称", c)
		return
	}

	ctx := c.Request.Context()
	job, err := getJenkinsJobHelper(ctx, client, jobName, folder)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("向 Jenkins 问询 Job 信息无所得: %v", err), c)
		return
	}

	build, err := getJenkinsBuildSafe(ctx, job, buildNum)
	if err != nil || build == nil {
		// 检查任务是否在 Jenkins 调度队列中排队中
		if job.Raw.InQueue {
			common.OkWithDetailed(gin.H{
				"content":     "⏳ 任务已成功提交至 Jenkins 调度队列，正在等待执行节点分配，请稍候...\n",
				"offset":      0,
				"isRunning":   true,
				"status":      "QUEUEING",
				"buildNumber": 0,
			}, "构建排队中", c)
			return
		}

		common.ReqBadFailWithMessage(fmt.Sprintf("任务 [%s] 当前无可用构建记录（历史上从未构建或历史构建已被清理）。请点击【发起构建部署】进行首次构建运行。", jobName), c)
		return
	}

	res, err := build.GetConsoleOutputFromIndex(ctx, offset)
	content := ""
	if err == nil {
		content = stripJenkinsAnnotations(res.Content)
	}

	isRunning := build.IsRunning(ctx)
	buildResult := build.GetResult()
	curBuildNum := build.GetBuildNumber()

	if curBuildNum > 0 {
		updates := map[string]interface{}{
			"count": curBuildNum,
		}
		if isRunning {
			updates["status"] = "BUILDING"
		} else if buildResult != "" {
			updates["status"] = buildResult
		}
		_ = models.Db.Model(&models.JenkinsJob{}).Where("instance_id = ? AND name = ?", instanceId, jobName).Updates(updates).Error
	}

	common.OkWithData(gin.H{
		"content":     content,
		"offset":      res.Offset,
		"isRunning":   isRunning,
		"result":      buildResult,
		"buildNumber": curBuildNum,
	}, c)
}

// getJenkinsJobStageView 联线 Jenkins 提取真实的 Stage View 流水线阶段与运行状态
func getJenkinsJobStageView(c *gin.Context) {
	client, _, ok := getJenkinsClientFromCtx(c)
	if !ok {
		return
	}

	jobName := c.DefaultQuery("jobName", c.DefaultQuery("job_name", c.Query("name")))
	folder := c.DefaultQuery("folder", c.Query("projectName"))
	if jobName == "" {
		common.ReqBadFailWithMessage("缺少必要的作业名称", c)
		return
	}

	ctx := c.Request.Context()
	job, err := getJenkinsJobHelper(ctx, client, jobName, folder)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("向 Jenkins 问询 Job 信息无所得: %v", err), c)
		return
	}

	type StageItem struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Status         string `json:"status"` // SUCCESS, FAILED, IN_PROGRESS, NOT_EXECUTED, ABORTED, PAUSED
		DurationMillis int64  `json:"durationMillis"`
	}

	type StageViewDescribe struct {
		ID     string      `json:"id"`
		Name   string      `json:"name"`
		Status string      `json:"status"`
		Stages []StageItem `json:"stages"`
	}

	var describe StageViewDescribe
	wfEndpoint := fmt.Sprintf("%s/wfapi/describe", strings.TrimRight(job.Base, "/"))
	if build, err := job.GetLastBuild(ctx); err == nil && build != nil {
		wfEndpoint = fmt.Sprintf("%s/wfapi/describe", strings.TrimRight(build.Base, "/"))
	}

	resp, err := client.Requester.GetJSON(ctx, wfEndpoint, &describe, nil)
	if err == nil && resp.StatusCode == 200 && len(describe.Stages) > 0 {
		common.OkWithData(gin.H{
			"buildNumber": describe.Name,
			"status":      describe.Status,
			"stages":      describe.Stages,
		}, c)
		return
	}

	// 2. 若未跑过构建或未得到 describe，则提取 config.xml 分析 Groovy 代码中的 stage('...')
	configXML, err := job.GetConfig(ctx)
	if err == nil && configXML != "" {
		re := regexp.MustCompile(`stage\s*\(\s*['"]([^'"]+)['"]\s*\)`)
		matches := re.FindAllStringSubmatch(configXML, -1)
		if len(matches) > 0 {
			var fallbackStages []StageItem
			for idx, match := range matches {
				fallbackStages = append(fallbackStages, StageItem{
					ID:             fmt.Sprintf("%d", idx+1),
					Name:           match[1],
					Status:         "NOT_EXECUTED",
					DurationMillis: 0,
				})
			}
			common.OkWithData(gin.H{
				"buildNumber": "-",
				"status":      "NOT_BUILT",
				"stages":      fallbackStages,
			}, c)
			return
		}
	}

	// 3. 兜底默认 Stage
	defaultStages := []StageItem{
		{ID: "1", Name: "拉取代码 (Checkout)", Status: "NOT_EXECUTED"},
		{ID: "2", Name: "编译打包 (Compile)", Status: "NOT_EXECUTED"},
		{ID: "3", Name: "镜像推送 (Docker Push)", Status: "NOT_EXECUTED"},
		{ID: "4", Name: "云端发布 (Deploy)", Status: "NOT_EXECUTED"},
	}
	common.OkWithData(gin.H{
		"buildNumber": "-",
		"status":      "NOT_BUILT",
		"stages":      defaultStages,
	}, c)
}

// JobParameterDefItem 动态 Jenkins 任务参数定义
type JobParameterDefItem struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"` // boolean, choice, string, branch
	DefaultValue string   `json:"defaultValue"`
	Description  string   `json:"description"`
	Choices      []string `json:"choices,omitempty"`
}

func isIgnoredParam(name string) bool {
	upper := strings.ToUpper(strings.TrimSpace(name))
	switch upper {
	case "BUILD_USER", "BUILD_USER_ID", "OPERATOR", "BRANCH", "DEPLOYENV", "DEPLOY_ENV", "DEPLOYTYPE", "DEPLOY_TYPE", "GITREPO", "GIT_REPO":
		return true
	}
	return false
}

// parseJenkinsXmlParams 从 Jenkins Job XML 报文中解析真实的参数定义，杜绝写死表单导致传参失灵
func parseJenkinsXmlParams(xmlContent string) []JobParameterDefItem {
	var params []JobParameterDefItem

	startTag := "<parameterDefinitions>"
	endTag := "</parameterDefinitions>"
	sIdx := strings.Index(xmlContent, startTag)
	eIdx := strings.Index(xmlContent, endTag)
	if sIdx == -1 || eIdx == -1 || eIdx <= sIdx {
		return params
	}

	innerXml := xmlContent[sIdx+len(startTag) : eIdx]
	decoder := xml.NewDecoder(strings.NewReader("<root>" + innerXml + "</root>"))

	var curParam *JobParameterDefItem
	var curTag string
	var inChoices bool

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			curTag = t.Name.Local
			tagLower := strings.ToLower(curTag)
			if strings.Contains(tagLower, "parameterdefinition") || strings.Contains(tagLower, "choiceparameter") {
				curParam = &JobParameterDefItem{Type: "string"}
				if strings.Contains(tagLower, "boolean") {
					curParam.Type = "boolean"
				} else if strings.Contains(tagLower, "choice") {
					curParam.Type = "choice"
				}
			}
			if curTag == "choices" {
				inChoices = true
			}
		case xml.EndElement:
			endTagLocal := t.Name.Local
			tagLower := strings.ToLower(endTagLocal)
			if curTag == "choices" {
				inChoices = false
			}
			if (strings.Contains(tagLower, "parameterdefinition") || strings.Contains(tagLower, "choiceparameter")) && curParam != nil {
				pName := strings.TrimSpace(curParam.Name)
				if isIgnoredParam(pName) {
					curParam = nil
					curTag = ""
					continue
				}
				if strings.Contains(curParam.Name, "分支") || strings.EqualFold(curParam.Name, "branch") {
					curParam.Type = "branch"
				}
				params = append(params, *curParam)
				curParam = nil
			}
			curTag = ""
		case xml.CharData:
			val := strings.TrimSpace(string(t))
			if curParam != nil && val != "" {
				switch curTag {
				case "name":
					curParam.Name = val
				case "description":
					curParam.Description = val
				case "defaultValue":
					curParam.DefaultValue = val
					if curParam.Type == "choice" && len(curParam.Choices) == 0 {
						curParam.Choices = append(curParam.Choices, val)
					}
				case "string":
					if inChoices {
						curParam.Choices = append(curParam.Choices, val)
					}
				}
			}
		}
	}
	return params
}

// getJenkinsJobParameters 动态提取 Jenkins Job 真实参数结构，供前端自适应渲染
func getJenkinsJobParameters(c *gin.Context) {
	client, _, ok := getJenkinsClientFromCtx(c)
	if !ok {
		return
	}

	jobName := c.DefaultQuery("jobName", c.DefaultQuery("job_name", c.Query("name")))
	folder := c.DefaultQuery("folder", c.Query("projectName"))
	if jobName == "" {
		common.ReqBadFailWithMessage("缺少必要的任务名称(jobName)", c)
		return
	}

	ctx := c.Request.Context()
	job, err := getJenkinsJobHelper(ctx, client, jobName, folder)
	if err != nil || job == nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("无法定位 Jenkins 作业: %v", err), c)
		return
	}

	configXML, err := job.GetConfig(ctx)
	if err != nil || configXML == "" {
		common.ReqBadFailWithMessage(fmt.Sprintf("获取作业配置失败: %v", err), c)
		return
	}

	params := parseJenkinsXmlParams(configXML)
	common.OkWithData(params, c)
}

// JobBuildHistoryItem 历史构建记录结构
type JobBuildHistoryItem struct {
	BuildNumber       int64                  `json:"buildNumber"`
	Result            string                 `json:"result"`
	Building          bool                   `json:"building"`
	Timestamp         int64                  `json:"timestamp"`
	Duration          int64                  `json:"duration"`
	DurationFormatted string                 `json:"durationFormatted"`
	TriggerUser       string                 `json:"triggerUser"`
	Parameters        map[string]interface{} `json:"parameters"`
	ParamList         []JobBuildParamKV      `json:"paramList"`
}

type JobBuildParamKV struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

func formatDurationMs(ms int64) string {
	if ms <= 0 {
		return "0s"
	}
	sec := ms / 1000
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	min := sec / 60
	remSec := sec % 60
	return fmt.Sprintf("%dm %ds", min, remSec)
}

// getJenkinsJobBuildHistory 查询指定 Job 的历史构建列表（含构建入参、耗时、触发人等）
func getJenkinsJobBuildHistory(c *gin.Context) {
	client, _, ok := getJenkinsClientFromCtx(c)
	if !ok {
		return
	}

	jobName := c.DefaultQuery("jobName", c.DefaultQuery("job_name", c.Query("name")))
	folder := c.DefaultQuery("folder", c.Query("projectName"))
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	if jobName == "" {
		common.ReqBadFailWithMessage("缺少必要的任务名称(jobName)", c)
		return
	}

	ctx := c.Request.Context()
	job, err := getJenkinsJobHelper(ctx, client, jobName, folder)
	if err != nil || job == nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("无法定位 Jenkins 作业: %v", err), c)
		return
	}

	type TreeAction struct {
		Class      string `json:"_class"`
		Parameters []struct {
			Name  string      `json:"name"`
			Value interface{} `json:"value"`
		} `json:"parameters"`
		Causes []struct {
			ShortDescription string `json:"shortDescription"`
			UserName         string `json:"userName"`
			UserId           string `json:"userId"`
		} `json:"causes"`
	}

	type TreeBuild struct {
		Number    int64        `json:"number"`
		Result    string       `json:"result"`
		Timestamp int64        `json:"timestamp"`
		Duration  float64      `json:"duration"`
		Building  bool         `json:"building"`
		Actions   []TreeAction `json:"actions"`
	}

	type TreeJobResp struct {
		Builds []TreeBuild `json:"builds"`
	}

	var items []JobBuildHistoryItem

	// 1. 优先通过 Jenkins tree 属性高效批量拉取
	var treeResp TreeJobResp
	query := map[string]string{
		"tree": fmt.Sprintf("builds[number,result,timestamp,duration,building,actions[parameters[name,value],causes[shortDescription,userName,userId]]]{0,%d}", limit),
	}
	resp, err := client.Requester.GetJSON(ctx, job.Base, &treeResp, query)
	if err == nil && resp != nil && resp.StatusCode == 200 && len(treeResp.Builds) > 0 {
		for _, b := range treeResp.Builds {
			item := JobBuildHistoryItem{
				BuildNumber:       b.Number,
				Result:            b.Result,
				Building:          b.Building,
				Timestamp:         b.Timestamp,
				Duration:          int64(b.Duration),
				DurationFormatted: formatDurationMs(int64(b.Duration)),
				Parameters:        make(map[string]interface{}),
				ParamList:         make([]JobBuildParamKV, 0),
			}
			if item.Building {
				item.Result = "BUILDING"
			} else if item.Result == "" {
				item.Result = "SUCCESS"
			}

			for _, act := range b.Actions {
				for _, p := range act.Parameters {
					item.Parameters[p.Name] = p.Value
					item.ParamList = append(item.ParamList, JobBuildParamKV{
						Name:  p.Name,
						Value: p.Value,
					})
				}
				if item.TriggerUser == "" {
					for _, cause := range act.Causes {
						if cause.UserName != "" {
							item.TriggerUser = cause.UserName
							break
						} else if cause.UserId != "" {
							item.TriggerUser = cause.UserId
							break
						} else if cause.ShortDescription != "" {
							item.TriggerUser = cause.ShortDescription
							break
						}
					}
				}
			}
			if item.TriggerUser == "" {
				item.TriggerUser = "System / SCM"
			}
			items = append(items, item)
		}
	} else {
		// 2. 兜底回退：逐个构建安全加载
		var buildNums []int64
		for _, b := range job.Raw.Builds {
			buildNums = append(buildNums, b.Number)
			if len(buildNums) >= limit {
				break
			}
		}

		for _, bNum := range buildNums {
			bObj, bErr := getJenkinsBuildSafe(ctx, job, bNum)
			if bErr != nil || bObj == nil {
				continue
			}

			isRun := bObj.IsRunning(ctx)
			resStr := bObj.GetResult()
			if isRun {
				resStr = "BUILDING"
			}

			dur := int64(bObj.GetDuration())
			item := JobBuildHistoryItem{
				BuildNumber:       bNum,
				Result:            resStr,
				Building:          isRun,
				Timestamp:         bObj.GetTimestamp().UnixMilli(),
				Duration:          dur,
				DurationFormatted: formatDurationMs(dur),
				Parameters:        make(map[string]interface{}),
				ParamList:         make([]JobBuildParamKV, 0),
			}

			params := bObj.GetParameters()
			for _, p := range params {
				item.Parameters[p.Name] = p.Value
				item.ParamList = append(item.ParamList, JobBuildParamKV{
					Name:  p.Name,
					Value: p.Value,
				})
			}

			causes, _ := bObj.GetCauses(ctx)
			for _, cItem := range causes {
				if u, ok := cItem["userName"].(string); ok && u != "" {
					item.TriggerUser = u
					break
				}
				if u, ok := cItem["userId"].(string); ok && u != "" {
					item.TriggerUser = u
					break
				}
				if desc, ok := cItem["shortDescription"].(string); ok && desc != "" {
					item.TriggerUser = desc
					break
				}
			}
			if item.TriggerUser == "" {
				item.TriggerUser = "System / SCM"
			}
			items = append(items, item)
		}
	}

	common.OkWithData(gin.H{
		"items": items,
		"total": len(items),
	}, c)
}

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
	for _, j := range allJobs {
		var count int64 = j.Raw.LastBuild.Number
		if count == 0 {
			if lb, lbErr := j.GetLastBuild(ctx); lbErr == nil && lb != nil {
				count = lb.GetBuildNumber()
			}
		}
		folder, shortJobName, fullJobName := parseFolderAndJobName("", j.Raw.Name)

		var existing models.JenkinsJob
		_ = models.Db.Where("instance_id = ? AND (name = ? OR name = ?)", instanceId, shortJobName, fullJobName).First(&existing).Error

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
			EnableDelete:   existing.EnableDelete,
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

	// 1. 先从数据库提取条件结果
	dbJobs, err := models.GetJenkinsJobListByParam(&param)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("获取 Job 列表失败: %v", err), c)
		return
	}

	// 2. 首次查询无数据则同步一轮，否则自动开启底层后台轮询异步更新实况状态
	if len(dbJobs) == 0 {
		_ = SyncJenkinsJobsToDB(c.Request.Context(), instanceId, client)
		dbJobs, _ = models.GetJenkinsJobListByParam(&param)
	} else {
		go func(instId uint, cli *gojenkins.Jenkins) {
			_ = SyncJenkinsJobsToDB(context.Background(), instId, cli)
		}(instanceId, client)
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
	ID             uint   `json:"id"`
	InstanceID     uint   `json:"instanceId"`
	DeployType     string `json:"deployType"`
	DeployEnv      string `json:"deployEnv"`
	Name           string `json:"name"`
	JobName        string `json:"jobName"`
	ProjectName    string `json:"projectName"` // Git仓库group组名/文件夹名
	Folder         string `json:"folder"`      // 支持可选入参 不入库
	GitRepo        string `json:"gitRepo"`     // 仓库全息链接
	GitBranch      string `json:"gitBranch"`   // 编译部署选定分支
	Lang           string `json:"lang"`
	PipelineScript string `json:"pipelineScript"` // 本字段不进数据库
	CreateUserName string `json:"createUserName"`
	EnableDelete   bool   `json:"enableDelete"`
}

// createJenkinsJob 新建 Job (调用官方语法严审后同步建表与入远端端点，杜绝无效或脚本废案进库)
func createJenkinsJob(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var req createOrUpdateJobReq
	if err := c.ShouldBindJSON(&req); err != nil || req.InstanceID == 0 {
		common.ReqBadFailWithMessage("参数缺失: 需要合法的 instanceId 及服务名与目录配置", c)
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

	folder, realJobName, fullJobName := parseFolderAndJobName(folder, jobName)

	script := req.PipelineScript
	if script == "" {
		script = fmt.Sprintf("pipeline {\n    agent any\n    stages {\n        stage('Hello') {\n            steps {\n                echo 'Hello World from job: %s'\n            }\n        }\n    }\n}", fullJobName)
	}

	// 核心风控限制：提交时必须先调用官方语法检测，检测失败不允许远端创建！
	valid, errMsg := verifyJenkinsScriptSyntax(req.InstanceID, script)
	if !valid {
		sc.Logger.Warn("Jenkins Pipeline 语法终审被主动拒止", zap.String("error", errMsg))
		common.ReqBadFailWithMessage(fmt.Sprintf("⚠️ Jenkinsfile 官方面向语法解析不合规，已封锁远端创建申请: \n%s", errMsg), c)
		return
	}

	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(script))
	escapedScript := buf.String()

	xmlConfig := fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <description>Created via BigDevOps Baseline Architecture</description>
  <keepDependencies>false</keepDependencies>
  <properties>
    <hudson.model.ParametersDefinitionProperty>
      <parameterDefinitions>
        <hudson.model.StringParameterDefinition>
          <name>BUILD_USER</name>
          <description>BigDevOps 触发人账号</description>
          <defaultValue>系统/未知</defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
        <hudson.model.StringParameterDefinition>
          <name>OPERATOR</name>
          <description>操作人账号</description>
          <defaultValue>系统/未知</defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
        <hudson.model.StringParameterDefinition>
          <name>branch</name>
          <description>Git 分支</description>
          <defaultValue>main</defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
        <hudson.model.StringParameterDefinition>
          <name>deployEnv</name>
          <description>部署环境</description>
          <defaultValue>dev</defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
        <hudson.model.StringParameterDefinition>
          <name>deployType</name>
          <description>部署目标类型</description>
          <defaultValue>host</defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
        <hudson.model.StringParameterDefinition>
          <name>gitRepo</name>
          <description>Git 仓库地址</description>
          <defaultValue></defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
      </parameterDefinitions>
    </hudson.model.ParametersDefinitionProperty>
  </properties>
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps">
    <script>%s</script>
    <sandbox>true</sandbox>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>`, escapedScript)

	ctx := c.Request.Context()
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
		common.ReqBadFailWithMessage(fmt.Sprintf("远端创建执行发生错误: %v", err), c)
		return
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
		EnableDelete:   false, // 创建Job后默认锁定
	}
	if err := models.SaveOrUpdateJenkinsJob(dbObj); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("远端创建成功但保存基线失败: %v", err), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("Job 任务及项目构架 '%s' 校验完好并落地建档成真！", fullJobName), c)
}

// updateJenkinsJob 更新 Job (直接提交更新至远端配置不变动 DB 的 PipelineScript)
func updateJenkinsJob(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var req createOrUpdateJobReq
	if err := c.ShouldBindJSON(&req); err != nil || req.InstanceID == 0 {
		common.ReqBadFailWithMessage("入参无法匹配，缺少重要指引", c)
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
		xmlConfig := fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <description>Updated via BigDevOps Baseline Architecture</description>
  <keepDependencies>false</keepDependencies>
  <properties>
    <hudson.model.ParametersDefinitionProperty>
      <parameterDefinitions>
        <hudson.model.StringParameterDefinition>
          <name>BUILD_USER</name>
          <description>BigDevOps 触发人账号</description>
          <defaultValue>系统/未知</defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
        <hudson.model.StringParameterDefinition>
          <name>OPERATOR</name>
          <description>操作人账号</description>
          <defaultValue>系统/未知</defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
        <hudson.model.StringParameterDefinition>
          <name>branch</name>
          <description>Git 分支</description>
          <defaultValue>main</defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
        <hudson.model.StringParameterDefinition>
          <name>deployEnv</name>
          <description>部署环境</description>
          <defaultValue>dev</defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
        <hudson.model.StringParameterDefinition>
          <name>deployType</name>
          <description>部署目标类型</description>
          <defaultValue>host</defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
        <hudson.model.StringParameterDefinition>
          <name>gitRepo</name>
          <description>Git 仓库地址</description>
          <defaultValue></defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
      </parameterDefinitions>
    </hudson.model.ParametersDefinitionProperty>
  </properties>
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps">
    <script>%s</script>
    <sandbox>true</sandbox>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>`, escapedScript)

		ctx := c.Request.Context()
		job, err := getJenkinsJobHelper(ctx, client, fullJobName, folder)
		if err == nil && job != nil {
			_ = job.UpdateConfig(ctx, xmlConfig)
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
	err := models.Db.Where("instance_id = ? AND (name = ? OR name = ?)", req.InstanceID, realJobName, fullJobName).First(&dbJob).Error
	if err != nil && req.ID > 0 {
		_ = models.Db.Where("id = ?", req.ID).First(&dbJob).Error
	}

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
	dbJob.EnableDelete = req.EnableDelete

	if err := models.SaveOrUpdateJenkinsJob(&dbJob); err != nil {
		sc.Logger.Error("执行同步变更基线字段失败", zap.Error(err))
		common.ReqBadFailWithMessage("记录持久化更新遇到意外拦截", c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("Job 基础建树 '%s' 参数与云端脚本完满再版成功！", fullJobName), c)
}

func extractScriptFromXml(xmlContent string) string {
	startTag := "<script>"
	endTag := "</script>"
	startIdx := strings.Index(xmlContent, startTag)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(startTag)
	endIdx := strings.Index(xmlContent[startIdx:], endTag)
	if endIdx == -1 {
		return ""
	}
	escapedScript := xmlContent[startIdx : startIdx+endIdx]

	script := strings.ReplaceAll(escapedScript, "&amp;", "&")
	script = strings.ReplaceAll(script, "&lt;", "<")
	script = strings.ReplaceAll(script, "&gt;", ">")
	script = strings.ReplaceAll(script, "&quot;", "\"")
	script = strings.ReplaceAll(script, "&apos;", "'")
	return strings.TrimSpace(script)
}

// getJenkinsJobRemotePipeline 独占查询方法：拉取远端的 Pipeline 脚本(不落库纯在线取样渲染)
func getJenkinsJobRemotePipeline(c *gin.Context) {
	client, _, ok := getJenkinsClientFromCtx(c)
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
	common.OkWithData(gin.H{
		"pipelineScript": script,
		"rawXml":         configXml,
	}, c)
}

type toggleDeleteLockReq struct {
	InstanceID  uint   `json:"instanceId"`
	JobName     string `json:"jobName"`
	Enable      bool   `json:"enable"`
	Folder      string `json:"folder"`
	ProjectName string `json:"projectName"`
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

	err := models.Db.Model(&models.JenkinsJob{}).
		Where("instance_id = ? AND (name = ? OR name = ?)", req.InstanceID, req.JobName, fullName).
		Update("enable_delete", req.Enable).Error

	if err != nil {
		common.ReqBadFailWithMessage("变更专属安全锁状态失败！", c)
		return
	}
	statusText := "锁死不可删除 (保持高度容灾与安全态)"
	if req.Enable {
		statusText = "安全锁已开启 (此任务已开放直接销毁)"
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
	if err == nil && !existing.EnableDelete {
		common.ReqBadFailWithMessage(fmt.Sprintf("安全风控触发：作业 '%s' 目前处于删防关闭锁定态！严禁非法暴力强行释放。如需废止请在视图操作表中明确拨开【释放删除锁】开关！", jobName), c)
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

// triggerJenkinsBuild 触发构建部署 (构建时可带入并承载分级环境变更命令与快调)
func triggerJenkinsBuild(c *gin.Context) {
	var req triggerBuildReq
	if err := c.ShouldBindJSON(&req); err != nil || req.InstanceID == 0 || req.JobName == "" {
		common.ReqBadFailWithMessage("触发展望入参存在短缺异常", c)
		return
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

	ctx := c.Request.Context()
	job, err := getJenkinsJobHelper(ctx, client, req.JobName, req.Folder, req.ProjectName)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("解析远程任务定义发生奔溃或对象已失联: %v", err), c)
		return
	}

	params := make(map[string]string)
	updates := map[string]interface{}{
		"status": "BUILDING",
	}

	operator := req.CreateUserName
	if operator == "" {
		if userNameVal, ok := c.Get(common.GIN_CTX_JWT_USER_NAME); ok {
			operator = fmt.Sprintf("%v", userNameVal)
		}
	}
	if operator == "" {
		authHeaderString := c.Request.Header.Get("Authorization")
		if authHeaderString != "" {
			parts := strings.SplitN(authHeaderString, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
				if claims, err := models.ParseToken(parts[1], sc); err == nil && claims != nil {
					if claims.SystemUser != nil {
						if claims.SystemUser.Username != "" {
							operator = claims.SystemUser.Username
						} else if claims.SystemUser.RealName != "" {
							operator = claims.SystemUser.RealName
						}
					}
				}
			}
		}
	}
	if operator != "" {
		params["BUILD_USER"] = operator
		params["buildUser"] = operator
		params["BUILD_USER_ID"] = operator
		params["OPERATOR"] = operator
		params["operator"] = operator
		updates["create_user_name"] = operator
	}

	if req.Branch != "" {
		params["branch"] = req.Branch
		params["BRANCH"] = req.Branch
		updates["git_branch"] = req.Branch
	}
	if req.DeployType != "" {
		params["deployType"] = req.DeployType
		params["DEPLOY_TYPE"] = req.DeployType
		updates["deploy_type"] = req.DeployType
	}
	if req.GitRepo != "" {
		params["gitRepo"] = req.GitRepo
		params["GIT_REPO"] = req.GitRepo
		updates["git_repo"] = req.GitRepo
	}
	if req.DeployEnv != "" {
		params["deployEnv"] = req.DeployEnv
		params["DEPLOY_ENV"] = req.DeployEnv
		updates["deploy_env"] = req.DeployEnv
	}

	// 生产环境适配参数全量映射
	if req.ScanCode {
		params["scanCode"] = "true"
		params["SCAN_CODE"] = "true"
	} else {
		params["scanCode"] = "false"
		params["SCAN_CODE"] = "false"
	}
	if req.BuildNode != "" {
		params["node"] = req.BuildNode
		params["NODE"] = req.BuildNode
		params["buildNode"] = req.BuildNode
	}
	if req.JdkVersion != "" {
		params["jdkVersion"] = req.JdkVersion
		params["JDK_VERSION"] = req.JdkVersion
	}
	if req.BuildCommand != "" {
		params["buildCommand"] = req.BuildCommand
		params["BUILD_COMMAND"] = req.BuildCommand
	}
	if req.Module != "" {
		params["module"] = req.Module
		params["MODULE"] = req.Module
	}
	if req.ConfigFile != "" {
		params["configFile"] = req.ConfigFile
		params["CONFIG_FILE"] = req.ConfigFile
	}
	if req.Port != "" {
		params["port"] = req.Port
		params["PORT"] = req.Port
	}
	if req.TargetHost != "" {
		params["targetHost"] = req.TargetHost
		params["TARGET_HOST"] = req.TargetHost
	}

	// 自由新增参数 CustomParams 无缝注入
	for k, v := range req.CustomParams {
		if strings.TrimSpace(k) != "" && v != nil {
			strVal := fmt.Sprintf("%v", v)
			params[k] = strVal
			params[strings.ToUpper(k)] = strVal
		}
	}

	queueID, err := job.InvokeSimple(ctx, params)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("调度触发流失速报错: %v", err), c)
		return
	}

	var buildNumber int64 = 0
	build, _ := client.GetBuildFromQueueID(ctx, job, queueID)
	if build != nil {
		buildNumber = build.GetBuildNumber()
		updates["count"] = buildNumber
	}

	_ = models.Db.Model(&models.JenkinsJob{}).Where("instance_id = ? AND name = ?", req.InstanceID, req.JobName).Updates(updates).Error

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

	var build *gojenkins.Build
	if req.BuildNumber > 0 {
		build, err = job.GetBuild(ctx, req.BuildNumber)
	} else {
		build, err = job.GetLastBuild(ctx)
	}
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

	var build *gojenkins.Build
	if buildNum > 0 {
		build, err = job.GetBuild(ctx, buildNum)
	} else {
		build, err = job.GetLastBuild(ctx)
	}
	if err != nil || build == nil {
		common.ReqBadFailWithMessage("当前管道可能从未来得及触发实际运作或已被释放清偿，请手动试运行一期", c)
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

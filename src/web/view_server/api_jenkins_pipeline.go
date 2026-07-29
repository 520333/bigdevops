package view_server

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
)

// getJenkinsPipelineList 获取流水线配置列表
func getJenkinsPipelineList(c *gin.Context) {
	lang := c.Query("lang")
	keyword := c.Query("keyword")

	objs, err := models.GetJenkinsPipelineConfigList(lang, keyword)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("获取流水线配置列表失败: %v", err), c)
		return
	}

	common.OkWithDetailed(gin.H{
		"items": objs,
		"total": len(objs),
	}, "获取成功", c)
}

// createJenkinsPipeline 创建流水线配置
func createJenkinsPipeline(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var req models.JenkinsPipelineConfig
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		common.ReqBadFailWithMessage("流水线名称为必填项", c)
		return
	}

	if err := req.CreateOne(); err != nil {
		sc.Logger.Error("创建流水线配置失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("创建失败: %v", err), c)
		return
	}

	common.OkWithData(req, c)
}

// updateJenkinsPipeline 更新流水线配置
func updateJenkinsPipeline(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var req models.JenkinsPipelineConfig
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		common.ReqBadFailWithMessage("参数不正确，ID 不能为空", c)
		return
	}

	if err := req.UpdateOne(); err != nil {
		sc.Logger.Error("更新流水线配置失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("更新失败: %v", err), c)
		return
	}

	common.OkWithData(req, c)
}

// deleteJenkinsPipeline 删除流水线配置
func deleteJenkinsPipeline(c *gin.Context) {
	idStr := c.Query("id")
	id, _ := strconv.Atoi(idStr)
	if id == 0 {
		common.ReqBadFailWithMessage("缺少配置ID", c)
		return
	}

	obj := &models.JenkinsPipelineConfig{Model: models.Model{ID: uint(id)}}
	if err := obj.DeleteOne(); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("删除失败: %v", err), c)
		return
	}

	common.OkWithMessage("删除流水线配置成功！", c)
}

type ValidatePipelineReq struct {
	PipelineScript string `json:"pipelineScript"`
	InstanceID     uint   `json:"instanceId"`
}

// validateJenkinsPipeline 直接调用 Jenkins 官方 Linter API (/pipeline-model-converter/validate) 进行语法校验
func validateJenkinsPipeline(c *gin.Context) {
	var req ValidatePipelineReq
	if err := c.ShouldBindJSON(&req); err != nil || req.PipelineScript == "" {
		common.ReqBadFailWithMessage("请输入需要检测的 Pipeline 脚本", c)
		return
	}

	instances, err := models.GetJenkinsInstanceAll()
	if err != nil || len(instances) == 0 {
		common.ReqBadFailWithMessage("系统中未找到可用的 Jenkins 实例，无法使用官方 Linter 进行检测", c)
		return
	}

	var targetInst *models.JenkinsInstance
	if req.InstanceID > 0 {
		for _, inst := range instances {
			if inst.ID == req.InstanceID {
				targetInst = inst
				break
			}
		}
	}
	if targetInst == nil {
		targetInst = instances[0]
	}

	if targetInst.URL == "" {
		common.ReqBadFailWithMessage("目标 Jenkins 实例配置的 URL 为空", c)
		return
	}

	apiURL := fmt.Sprintf("%s/pipeline-model-converter/validate", strings.TrimRight(targetInst.URL, "/"))
	formData := url.Values{}
	formData.Set("jenkinsfile", req.PipelineScript)

	httpReq, err := http.NewRequest("POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("创建 Linter 请求失败: %v", err), c)
		return
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if targetInst.Username != "" && targetInst.ApiToken != "" {
		httpReq.SetBasicAuth(targetInst.Username, targetInst.ApiToken)
	}

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("请求 Jenkins 官方 Linter 校验失败: %v", err), c)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		common.ReqBadFailWithMessage("读取 Jenkins Linter 响应失败", c)
		return
	}

	respText := string(bodyBytes)

	if strings.Contains(respText, "Jenkinsfile successfully validated") {
		common.OkWithDetailed(gin.H{
			"valid":  true,
			"errors": []string{},
		}, "Jenkins 官方检测通过：Jenkinsfile successfully validated", c)
		return
	}

	var syntaxErrors []string
	lines := strings.Split(respText, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !strings.Contains(l, "Errors encountered validating") {
			syntaxErrors = append(syntaxErrors, l)
		}
	}
	if len(syntaxErrors) == 0 {
		syntaxErrors = append(syntaxErrors, respText)
	}

	common.OkWithDetailed(gin.H{
		"valid":  false,
		"errors": syntaxErrors,
	}, "Jenkins 官方语法检测未通过 ⚠️", c)
}

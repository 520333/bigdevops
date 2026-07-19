package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/gin-gonic/gin"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"
)

// 创建 Git 实例配置
func createCodeGitServer(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.CodeGitServer
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		sc.Logger.Error("解析新增CodeGitServer请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 注入创建人ID (假设上下文中能拿到 userName，再转 ID，这里直接模拟)
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err == nil {
		reqObj.UserID = dbUser.ID
	}

	reqObj.Status = "disconnected" // 默认未连接状态，需经过 Ping 测试

	if err := reqObj.CreateOne(); err != nil {
		sc.Logger.Error("新增CodeGitServer入库失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("创建成功", c)
}

// 更新 Git 实例配置
func updateCodeGitServer(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.CodeGitServer
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		sc.Logger.Error("解析更新CodeGitServer请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	_, err := models.GetCodeGitServerById(int(reqObj.ID))
	if err != nil {
		common.FailWithMessage("要更新的配置不存在", c)
		return
	}
	if err := reqObj.UpdateOne(); err != nil {
		sc.Logger.Error("更新CodeGitServer失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("更新成功", c)
}

// 删除 Git 实例配置
func deleteCodeGitServer(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id, _ := strconv.Atoi(c.Param("id"))

	dbObj, err := models.GetCodeGitServerById(id)
	if err != nil {
		common.FailWithMessage("未找到该配置", c)
		return
	}

	if err := dbObj.DeleteOne(); err != nil {
		sc.Logger.Error("删除CodeGitServer失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("删除成功", c)
}

// 获取 Git 实例列表 (带分页)
func getCodeGitServerList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	name := c.Query("name")
	platform := c.Query("platform")
	endpoint := c.Query("endpoint")

	offset := 0
	if currentPage > 1 {
		offset = (currentPage - 1) * pageSize
	}
	objs, total, err := models.GetCodeGitServerList(pageSize, offset, name, platform, endpoint)

	if err != nil {
		sc.Logger.Error("获取CodeGitServer列表失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	for _, obj := range objs {
		obj.FillFrontAllData()
	}
	common.OkWithDetailed(gin.H{
		"items": objs,
		"total": total,
	}, "ok", c)
}

// Ping 测试连通性 (支持在创建前发送 JSON 测试，或直接测试已有配置)
func pingCodeGitServer(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.CodeGitServer
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("参数解析失败", c)
		return
	}

	// 初始化 HTTP Client (处理自签证书跳过校验)
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	if reqObj.SkipVerify {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	var err error
	if reqObj.Platform == "gitlab" {
		gitLabClient, errClient := gitlab.NewClient(reqObj.Token, gitlab.WithBaseURL(reqObj.Endpoint), gitlab.WithHTTPClient(httpClient))
		if errClient != nil {
			err = errClient
		} else {
			_, _, err = gitLabClient.Version.GetVersion()
		}
	} else if reqObj.Platform == "gitea" {
		giteaClient, errClient := gitea.NewClient(reqObj.Endpoint, gitea.SetToken(reqObj.Token), gitea.SetHTTPClient(httpClient))
		if errClient != nil {
			err = errClient
		} else {
			_, _, err = giteaClient.GetMyUserInfo()
		}
	} else {
		common.FailWithMessage("不支持的平台类型", c)
		return
	}

	if err == nil {
		// 测试成功，如果是数据库中已存在的记录，可以顺手更新状态
		if reqObj.ID > 0 {
			reqObj.Status = "connected"
			_ = reqObj.UpdateOne()
		}
		common.OkWithMessage("连接成功，鉴权通过！", c)
	} else {
		if reqObj.ID > 0 {
			reqObj.Status = "failed"
			_ = reqObj.UpdateOne()
		}

		sc.Logger.Warn("Git 鉴权失败", zap.Error(err))
		common.FailWithMessage(fmt.Sprintf("连接失败：凭证无效或网络异常 (%v)", err), c)
	}
}

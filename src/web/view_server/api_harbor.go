package view_server

import (
	"fmt"
	"strconv"

	"bigdevops/src/common"
	"bigdevops/src/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// getHarborConfig 获取 Harbor 基本公开配置（供前端拼接拉取命令及直达官网）
func getHarborConfig(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	if sc.ArtifactC == nil || sc.ArtifactC.Harbor == nil {
		common.OkWithData(gin.H{
			"enable":  false,
			"baseUrl": "",
		}, c)
		return
	}
	common.OkWithData(gin.H{
		"enable":  sc.ArtifactC.Harbor.Enable,
		"baseUrl": sc.ArtifactC.Harbor.BaseURL,
	}, c)
}

// getHarborProjects 获取项目列表
func getHarborProjects(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewHarborClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "100"))
	name := c.Query("name")

	projects, err := client.GetProjects(page, pageSize, name)
	if err != nil {
		sc.Logger.Error("[Harbor] 获取项目列表失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("获取 Harbor 项目列表失败: %v", err), c)
		return
	}

	common.OkWithData(projects, c)
}

type createHarborProjectReq struct {
	ProjectName  string `json:"projectName" binding:"required"`
	IsPublic     bool   `json:"isPublic"`
	StorageLimit int64  `json:"storageLimit"` // -1 表示无限制
}

// createHarborProject 创建 Harbor 项目
func createHarborProject(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewHarborClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	var req createHarborProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage("参数错误: "+err.Error(), c)
		return
	}

	err = client.CreateProject(req.ProjectName, req.IsPublic, req.StorageLimit)
	if err != nil {
		sc.Logger.Error("[Harbor] 创建项目失败", zap.Error(err))
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("项目创建成功", c)
}

// getHarborRepositories 获取指定项目下的镜像仓库列表
func getHarborRepositories(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewHarborClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	projectName := c.Query("projectName")
	if projectName == "" {
		common.ReqBadFailWithMessage("缺少 projectName 参数", c)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "100"))
	query := c.Query("query")

	repos, err := client.GetRepositories(projectName, page, pageSize, query)
	if err != nil {
		sc.Logger.Error("[Harbor] 获取镜像列表失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("获取镜像仓库列表失败: %v", err), c)
		return
	}

	common.OkWithData(repos, c)
}

// getHarborArtifacts 获取镜像仓库下的所有版本 / Tags
func getHarborArtifacts(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewHarborClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	projectName := c.Query("projectName")
	repoName := c.Query("repoName")
	if projectName == "" || repoName == "" {
		common.ReqBadFailWithMessage("缺少 projectName 或 repoName 参数", c)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "100"))

	artifacts, err := client.GetArtifacts(projectName, repoName, page, pageSize)
	if err != nil {
		sc.Logger.Error("[Harbor] 获取制品列表失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("获取镜像版本失败: %v", err), c)
		return
	}

	common.OkWithData(artifacts, c)
}

type deleteHarborArtifactReq struct {
	ProjectName string `json:"projectName" binding:"required"`
	RepoName    string `json:"repoName" binding:"required"`
	Reference   string `json:"reference" binding:"required"` // digest 或 tag
}

// deleteHarborArtifact 删除指定版本镜像
func deleteHarborArtifact(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewHarborClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	var req deleteHarborArtifactReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage("参数错误: "+err.Error(), c)
		return
	}

	err = client.DeleteArtifact(req.ProjectName, req.RepoName, req.Reference)
	if err != nil {
		sc.Logger.Error("[Harbor] 删除镜像制品失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("删除镜像版本失败: %v", err), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

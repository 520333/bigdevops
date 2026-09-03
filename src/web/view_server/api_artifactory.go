package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type saveArtifactoryFileReq struct {
	Project string `json:"project" binding:"required"`
	Path    string `json:"path" binding:"required"`
	Content string `json:"content"`
}

type deleteArtifactoryFileReq struct {
	Project string `json:"project" binding:"required"`
	Path    string `json:"path" binding:"required"`
}

// @Summary      获取 Artifactory 所有仓库列表
// @Tags         artifactory
// @Accept       json
// @Produce      json
// @Router       /artifactory/repos [get]
func getArtifactoryRepositories(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArtifactoryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	repos, err := client.GetRepositories()
	if err != nil {
		sc.Logger.Error("[Artifactory] 获取仓库列表失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("获取仓库列表失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(repos, c)
}

// @Summary      获取指定文件/构件的详细元数据与统计信息
// @Tags         artifactory
// @Accept       json
// @Produce      json
// @Param        project query string true "项目仓库名"
// @Param        path    query string true "文件相对路径"
// @Router       /artifactory/info [get]
func getArtifactoryFileInfo(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArtifactoryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	project := c.Query("project")
	path := c.Query("path")
	if project == "" || path == "" {
		common.ReqBadFailWithMessage("缺少必填参数 project 或 path", c)
		return
	}

	info, err := client.GetFileInfo(project, path)
	if err != nil {
		sc.Logger.Error("[Artifactory] 获取文件元数据失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("获取元数据失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(info, c)
}

// @Summary      获取 Artifactory 文件树/元数据
// @Tags         artifactory
// @Accept       json
// @Produce      json
// @Param        project query string true "项目仓库名"
// @Param        path    query string false "相对路径"
// @Router       /artifactory/tree [get]
func getArtifactoryFileTree(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArtifactoryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	project := c.Query("project")
	if project == "" {
		common.ReqBadFailWithMessage("缺少必填参数 project", c)
		return
	}
	path := c.Query("path")

	treeResp, err := client.GetFileTree(project, path)
	if err != nil {
		sc.Logger.Error("[Artifactory] 获取文件树失败", zap.String("project", project), zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("获取文件树失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(treeResp, c)
}

// @Summary      读取 Artifactory 文本文件内容
// @Tags         artifactory
// @Accept       json
// @Produce      json
// @Param        project query string true "项目仓库名"
// @Param        path    query string true "文件相对路径"
// @Router       /artifactory/content [get]
func getArtifactoryFileContent(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArtifactoryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	project := c.Query("project")
	path := c.Query("path")
	if project == "" || path == "" {
		common.ReqBadFailWithMessage("缺少必填参数 project 或 path", c)
		return
	}

	contentBytes, err := client.GetFileContent(project, path)
	if err != nil {
		sc.Logger.Error("[Artifactory] 读取文件内容失败", zap.String("project", project), zap.String("path", path), zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("读取文件失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(string(contentBytes), c)
}

// @Summary      修改/保存 Artifactory 配置文件内容
// @Tags         artifactory
// @Accept       json
// @Produce      json
// @Router       /artifactory/save [post]
func saveArtifactoryFileContent(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArtifactoryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	var req saveArtifactoryFileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("请求参数绑定失败: %v", err.Error()), c)
		return
	}

	if err := client.SaveFileContent(req.Project, req.Path, []byte(req.Content)); err != nil {
		sc.Logger.Error("[Artifactory] 保存文件失败", zap.String("project", req.Project), zap.String("path", req.Path), zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("保存文件失败: %v", err.Error()), c)
		return
	}

	common.OkWithMessage("配置文件修改保存成功", c)
}

// @Summary      上传二进制 Jar 包或配置文件到 Artifactory
// @Tags         artifactory
// @Accept       multipart/form-data
// @Produce      json
// @Router       /artifactory/upload [post]
func uploadArtifactoryFile(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArtifactoryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	project := c.PostForm("project")
	subPath := c.PostForm("path") // 可选，如 "target" 或 ""
	if project == "" {
		common.ReqBadFailWithMessage("缺少必填参数 project", c)
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		common.ReqBadFailWithMessage("接收上传文件失败", c)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		common.ReqBadFailWithMessage("打开上传文件失败", c)
		return
	}
	defer file.Close()

	targetPath := fileHeader.Filename
	if subPath != "" {
		targetPath = fmt.Sprintf("%s/%s", subPath, fileHeader.Filename)
	}

	if err := client.UploadFile(project, targetPath, file, fileHeader.Size); err != nil {
		sc.Logger.Error("[Artifactory] 上传文件失败", zap.String("project", project), zap.String("path", targetPath), zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("文件上传到 Artifactory 失败: %v", err.Error()), c)
		return
	}

	common.OkWithMessage("文件上传成功", c)
}

// @Summary      删除 Artifactory 中的文件或产物
// @Tags         artifactory
// @Accept       json
// @Produce      json
// @Router       /artifactory/delete [post]
func deleteArtifactoryFile(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArtifactoryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	var req deleteArtifactoryFileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("参数错误: %v", err.Error()), c)
		return
	}

	if err := client.DeleteFile(req.Project, req.Path); err != nil {
		sc.Logger.Error("[Artifactory] 删除文件失败", zap.String("project", req.Project), zap.String("path", req.Path), zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("删除失败: %v", err.Error()), c)
		return
	}

	common.OkWithMessage("文件删除成功", c)
}

// @Summary      下载 Artifactory 文件/产物
// @Tags         artifactory
// @Produce      octet-stream
// @Param        project query string true "项目仓库名"
// @Param        path    query string true "文件相对路径"
// @Router       /artifactory/download [get]
func downloadArtifactoryFile(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArtifactoryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	project := c.Query("project")
	path := c.Query("path")
	if project == "" || path == "" {
		common.ReqBadFailWithMessage("缺少必填参数 project 或 path", c)
		return
	}

	data, err := client.GetFileContent(project, path)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("文件下载失败: %v", err.Error()), c)
		return
	}

	filename := path
	if idx := len(path) - 1; idx >= 0 {
		for i := len(path) - 1; i >= 0; i-- {
			if path[i] == '/' {
				filename = path[i+1:]
				break
			}
		}
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(200, "application/octet-stream", data)
}

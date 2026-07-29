package view_server

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"bigdevops/src/common"
	"bigdevops/src/models"
)

// getJenkinsParamList 获取构建参数字典列表
func getJenkinsParamList(c *gin.Context) {
	paramType := c.Query("type")
	keyword := c.Query("keyword")

	objs, err := models.GetJenkinsBuildParamList(paramType, keyword)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("获取构建参数列表失败: %v", err), c)
		return
	}

	common.OkWithDetailed(gin.H{
		"items": objs,
		"total": len(objs),
	}, "获取成功", c)
}

// createJenkinsParam 创建构建参数
func createJenkinsParam(c *gin.Context) {
	var req models.JenkinsBuildParam
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		common.ReqBadFailWithMessage("构建参数名称为必填项", c)
		return
	}

	if err := req.CreateOne(); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("创建构建参数失败: %v", err), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("构建参数 '%s' 创建成功！", req.Name), c)
}

// updateJenkinsParam 更新构建参数
func updateJenkinsParam(c *gin.Context) {
	var req models.JenkinsBuildParam
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		common.ReqBadFailWithMessage("缺少必要的 ID", c)
		return
	}

	if err := req.UpdateOne(); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("更新构建参数失败: %v", err), c)
		return
	}

	common.OkWithMessage("构建参数更新成功！", c)
}

// deleteJenkinsParam 删除构建参数
func deleteJenkinsParam(c *gin.Context) {
	idStr := c.Query("id")
	id, _ := strconv.Atoi(idStr)
	if id == 0 {
		common.ReqBadFailWithMessage("缺少 ID", c)
		return
	}

	obj := &models.JenkinsBuildParam{Model: models.Model{ID: uint(id)}}
	if err := obj.DeleteOne(); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("删除构建参数失败: %v", err), c)
		return
	}

	common.OkWithMessage("删除构建参数成功！", c)
}

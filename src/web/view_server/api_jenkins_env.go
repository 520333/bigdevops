package view_server

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"bigdevops/src/common"
	"bigdevops/src/models"
)

// getJenkinsEnvList 获取环境变量定义列表
func getJenkinsEnvList(c *gin.Context) {
	group := c.Query("group")
	keyword := c.Query("keyword")

	objs, err := models.GetJenkinsEnvVarList(group, keyword)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("获取环境变量列表失败: %v", err), c)
		return
	}

	common.OkWithDetailed(gin.H{
		"items": objs,
		"total": len(objs),
	}, "获取成功", c)
}

// createJenkinsEnv 创建环境变量
func createJenkinsEnv(c *gin.Context) {
	var req models.JenkinsEnvVar
	if err := c.ShouldBindJSON(&req); err != nil || req.Key == "" {
		common.ReqBadFailWithMessage("环境变量 Key 为必填项", c)
		return
	}

	if err := req.CreateOne(); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("创建环境变量失败: %v", err), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("环境变量 '%s' 创建成功！", req.Key), c)
}

// updateJenkinsEnv 更新环境变量
func updateJenkinsEnv(c *gin.Context) {
	var req models.JenkinsEnvVar
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		common.ReqBadFailWithMessage("缺少必要的 ID", c)
		return
	}

	if err := req.UpdateOne(); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("更新环境变量失败: %v", err), c)
		return
	}

	common.OkWithMessage("环境变量更新成功！", c)
}

// deleteJenkinsEnv 删除环境变量
func deleteJenkinsEnv(c *gin.Context) {
	idStr := c.Query("id")
	id, _ := strconv.Atoi(idStr)
	if id == 0 {
		common.ReqBadFailWithMessage("缺少 ID", c)
		return
	}

	obj := &models.JenkinsEnvVar{Model: models.Model{ID: uint(id)}}
	if err := obj.DeleteOne(); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("删除环境变量失败: %v", err), c)
		return
	}

	common.OkWithMessage("删除环境变量成功！", c)
}

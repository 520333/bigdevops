package view_server

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"bigdevops/src/common"
	"bigdevops/src/models"
)

// getJenkinsStageList 获取 Stage 阶段定义字典列表
func getJenkinsStageList(c *gin.Context) {
	category := c.Query("category")
	keyword := c.Query("keyword")

	objs, err := models.GetJenkinsStageList(category, keyword)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("获取 Stage 列表失败: %v", err), c)
		return
	}

	common.OkWithDetailed(gin.H{
		"items": objs,
		"total": len(objs),
	}, "获取成功", c)
}

// createJenkinsStage 创建 Stage 阶段模块
func createJenkinsStage(c *gin.Context) {
	var req models.JenkinsStage
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		common.ReqBadFailWithMessage("Stage 名称为必填项", c)
		return
	}

	if err := req.CreateOne(); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("创建 Stage 失败: %v", err), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("Stage 模块 '%s' 创建成功！", req.Name), c)
}

// updateJenkinsStage 更新 Stage 阶段模块
func updateJenkinsStage(c *gin.Context) {
	var req models.JenkinsStage
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		common.ReqBadFailWithMessage("缺少必要的 Stage ID", c)
		return
	}

	if err := req.UpdateOne(); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("更新 Stage 失败: %v", err), c)
		return
	}

	common.OkWithMessage("Stage 模块更新成功！", c)
}

// deleteJenkinsStage 删除 Stage 阶段模块
func deleteJenkinsStage(c *gin.Context) {
	idStr := c.Query("id")
	id, _ := strconv.Atoi(idStr)
	if id == 0 {
		common.ReqBadFailWithMessage("缺少 Stage ID", c)
		return
	}

	obj := &models.JenkinsStage{Model: models.Model{ID: uint(id)}}
	if err := obj.DeleteOne(); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("删除 Stage 失败: %v", err), c)
		return
	}

	common.OkWithMessage("删除 Stage 模块成功！", c)
}

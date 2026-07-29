package view_server

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// getJenkinsInstanceList 获取 Jenkins 实例列表
func getJenkinsInstanceList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	jc, hasCache := c.Get(common.GIN_CTX_JENKINS_CACHE)

	objs, err := models.GetJenkinsInstanceAll()
	if err != nil {
		sc.Logger.Error("获取 Jenkins 实例列表数据库错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("获取列表失败: %v", err), c)
		return
	}

	if hasCache && jc != nil {
		jenkinsCache := jc.(*cache.JenkinsCache)
		for _, obj := range objs {
			obj.LastProbSuccess = jenkinsCache.GetJenkinsProbeResultById(obj.ID)
			obj.LastProbErrMsg = jenkinsCache.GetJenkinsProbeErrMsgById(obj.ID)
		}
	}

	common.OkWithDetailed(gin.H{
		"items": objs,
		"total": len(objs),
	}, "获取成功", c)
}

// createJenkinsInstance 创建 Jenkins 实例
func createJenkinsInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var obj models.JenkinsInstance
	if err := c.ShouldBindJSON(&obj); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("参数绑定错误: %v", err), c)
		return
	}

	if obj.Name == "" || obj.URL == "" {
		common.ReqBadFailWithMessage("实例名称和URL不可为空", c)
		return
	}

	if err := obj.CreateOne(); err != nil {
		sc.Logger.Error("创建 Jenkins 实例失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("创建失败: %v", err), c)
		return
	}

	if jc, ok := c.Get(common.GIN_CTX_JENKINS_CACHE); ok && jc != nil {
		go jc.(*cache.JenkinsCache).ReNewJenkinsClientsMap(context.Background())
	}

	common.OkWithMessage("创建 Jenkins 实例成功", c)
}

// updateJenkinsInstance 修改 Jenkins 实例
func updateJenkinsInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var obj models.JenkinsInstance
	if err := c.ShouldBindJSON(&obj); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("参数绑定错误: %v", err), c)
		return
	}

	if obj.ID == 0 {
		common.ReqBadFailWithMessage("缺少实例ID", c)
		return
	}

	if err := obj.UpdateOne(); err != nil {
		sc.Logger.Error("修改 Jenkins 实例失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("更新失败: %v", err), c)
		return
	}

	if jc, ok := c.Get(common.GIN_CTX_JENKINS_CACHE); ok && jc != nil {
		go jc.(*cache.JenkinsCache).ReNewJenkinsClientsMap(context.Background())
	}

	common.OkWithMessage("更新 Jenkins 实例成功", c)
}

// deleteJenkinsInstance 删除 Jenkins 实例
func deleteJenkinsInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	idStr := c.Query("id")
	id, _ := strconv.Atoi(idStr)
	if id == 0 {
		common.ReqBadFailWithMessage("缺少实例ID", c)
		return
	}

	obj := &models.JenkinsInstance{Model: models.Model{ID: uint(id)}}
	if err := obj.DeleteOne(); err != nil {
		sc.Logger.Error("删除 Jenkins 实例失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("删除失败: %v", err), c)
		return
	}

	// 同步级联清理该实例关联的 JenkinsJob 本地数据库记录
	_ = models.Db.Where("instance_id = ?", id).Delete(&models.JenkinsJob{}).Error

	if jc, ok := c.Get(common.GIN_CTX_JENKINS_CACHE); ok && jc != nil {
		go jc.(*cache.JenkinsCache).ReNewJenkinsClientsMap(context.Background())
	}

	common.OkWithMessage("删除 Jenkins 实例成功", c)
}

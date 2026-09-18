package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// @Summary      获取Prometheus采集集群池列表
// @Description  获取Prometheus采集集群池列表 接口
// @Tags         monitor-prom
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取Prometheus采集集群池列表 响应结果"
// @Router       /monitor/getMonitorPromScrapePoolList [get]
// @Security     Bearer
func getMonitorPromScrapePoolList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")
	searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchTitle := c.DefaultQuery("name", "")
	searchCreateUserName := c.DefaultQuery("createUserName", "")

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	objs, err := models.GetMonitorPromScrapePoolAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的采集池执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的采集池执行错误：%v", err.Error()), c)
		return
	}
	allIds := []int{}

	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}

		if searchTitle != "" && !strings.Contains(obj.Name, searchTitle) {
			continue
		}

		// 填充前端需要的数据（拿到组合好的 CreateUserName）
		obj.FillFrontAllData()

		// 🚀 修复 1：对比的应该是刚填充好的 CreateUserName 字段，而不是 UserID
		if searchCreateUserName != "" && !strings.Contains(obj.CreateUserName, searchCreateUserName) {
			continue
		}

		allIds = append(allIds, int(obj.ID))
	}

	// 如果过滤后没有数据，直接返回空列表
	if len(allIds) == 0 {
		common.OkWithDetailed(gin.H{
			"items": []models.MonitorPromScrapePool{},
			"total": 0,
		}, "ok", c)
		return
	}

	// 根据过滤后的 ID 进行分页查询
	pagedObjs, err := models.GetMonitorPromScrapePoolByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的采集池执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的采集池执行错误：%v", err.Error()), c)
		return
	}

	// 🚀 修复 2：分页查出来的新对象，必须再次遍历填充一次虚拟字段，否则响应里还是空的！
	for _, obj := range pagedObjs {
		obj.FillFrontAllData()
	}

	common.OkWithDetailed(gin.H{
		"items": pagedObjs,
		"total": len(allIds),
	}, "ok", c)
}

// @Summary      创建Prometheus采集集群池
// @Description  创建Prometheus采集集群池 接口
// @Tags         monitor-prom
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "创建Prometheus采集集群池 响应结果"
// @Router       /monitor/createMonitorPromScrapePool [post]
// @Security     Bearer
func createMonitorPromScrapePool(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj models.MonitorPromScrapePool
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增采集池执行请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 获取当前用户ID
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if reqObj.CheckInstanceIpExists() {
		msg := "ip和其他采集池重复"
		sc.Logger.Error(msg, zap.Error(err))
		common.FailWithMessage(msg, c)
		return
	}
	if err == nil && dbUser != nil {
		reqObj.UserID = dbUser.ID
	}
	reqObj.FillDefaultData()
	// 存入数据库
	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增采集池执行数据库失败", zap.Error(err))
		common.FailWithMessage("存入数据库失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

// @Summary      更新Prometheus采集集群池
// @Description  更新Prometheus采集集群池 接口
// @Tags         monitor-prom
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "更新Prometheus采集集群池 响应结果"
// @Router       /monitor/updateMonitorPromScrapePool [post]
// @Security     Bearer
func updateMonitorPromScrapePool(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	// 🚀 致命修复：同上
	var reqObj models.MonitorPromScrapePool
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新采集池请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	if reqObj.CheckInstanceIpExists() {
		msg := "ip和其他采集池重复"
		sc.Logger.Error(msg, zap.Error(err))
		common.FailWithMessage(msg, c)
		return
	}
	// 检查是否存在
	dbPool, err := models.GetMonitorPromScrapePoolById(int(reqObj.ID))
	if err != nil {
		common.FailWithMessage("采集池不存在", c)
		return
	}

	// 检查采集池名称是否被其他采集池占用
	var sameNameCount int64
	models.Db.Model(&models.MonitorPromScrapePool{}).Where("name = ? AND id != ?", reqObj.Name, reqObj.ID).Count(&sameNameCount)
	if sameNameCount > 0 {
		common.FailWithMessage("采集池名称已存在", c)
		return
	}

	// 保持原有的创建人，防止编辑时丢失
	reqObj.UserID = dbPool.UserID

	// 更新
	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新采集池执行错误", zap.Error(err))
		common.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

// @Summary      删除Prometheus采集集群池
// @Description  删除Prometheus采集集群池 接口
// @Tags         monitor-prom
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "删除Prometheus采集集群池 响应结果"
// @Router       /monitor/deleteMonitorPromScrapePool/{id} [delete]
// @Security     Bearer
func deleteMonitorPromScrapePool(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetMonitorPromScrapePoolById(intVar)
	if err != nil {
		common.FailWithMessage("采集池不存在", c)
		return
	}

	jobs, err := models.GetMonitorPromScrapeJobByPoolId(uint(intVar))
	if err != nil {
		sc.Logger.Error("查询关联采集任务失败", zap.Error(err))
		common.FailWithMessage("查询关联采集任务失败: "+err.Error(), c)
		return
	}

	// 🚀 2. 核心修复：如果查出来的任务数量大于 0，说明有关联任务，绝对禁止删除！
	if len(jobs) > 0 {
		sc.Logger.Warn("该采集池已绑定采集任务，禁止直接删除！", zap.Any("采集池ID", id))
		common.FailWithMessage("该采集池下存在关联的采集任务，禁止直接删除！请先转移或清理任务。", c)
		return
	}

	err = dbObj.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除采集池执行错误", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

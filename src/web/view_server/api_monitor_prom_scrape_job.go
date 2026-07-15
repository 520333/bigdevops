package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

func getMonitorPromScrapeJobList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")
	searchEnable := c.DefaultQuery("enable", "")

	searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchTitle := c.DefaultQuery("name", "")
	searchEnableInt, _ := strconv.Atoi(searchEnable)
	searchCreateUserName := c.DefaultQuery("createUserName", "")

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	objs, err := models.GetMonitorPromScrapeJobAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的采集任务执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的采集任务执行错误：%v", err.Error()), c)
		return
	}
	allIds := []int{}

	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}
		if searchEnable != "" && obj.Enable != searchEnableInt {
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
			"items": []models.MonitorPromScrapeJob{},
			"total": 0,
		}, "ok", c)
		return
	}

	// 根据过滤后的 ID 进行分页查询
	pagedObjs, err := models.GetMonitorPromScrapeJobByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的采集任务执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的采集任务执行错误：%v", err.Error()), c)
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

func getMonitorPromScrapeJobOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("采集任务实例", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetJobTaskById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找采集任务实例错误", zap.Any("采集任务实例", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbObj.FillFrontAllData()

	common.OkWithDetailed(dbObj, "ok", c)
}

func createMonitorPromScrapeJob(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj models.MonitorPromScrapeJob
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增采集任务执行请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = reqObj.ValidateRelabelConfigsYamlString()
	if err != nil {
		msg := "[监控模块]新增采集任务-解析relabelConfig错误"
		sc.Logger.Error(msg, zap.Any("采集池", reqObj.Name), zap.Any("yaml", reqObj.RelabelConfigsYamlString), zap.Error(err))
		common.FailWithMessage(msg, c)
		return
	}

	// 获取当前用户ID
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)

	if err == nil && dbUser != nil {
		reqObj.UserID = dbUser.ID
	}
	// 存入数据库
	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增采集任务执行数据库失败", zap.Error(err))
		common.FailWithMessage("存入数据库失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

func updateMonitorPromScrapeJob(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	// 🚀 致命修复：同上
	var reqObj models.MonitorPromScrapeJob
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新采集任务请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 检查是否存在
	_, err = models.GetMonitorPromScrapeJobById(int(reqObj.ID))
	if err != nil {
		common.FailWithMessage("采集任务不存在", c)
		return
	}
	err = reqObj.ValidateRelabelConfigsYamlString()
	if err != nil {
		msg := "[监控模块]新增采集任务-解析relabelConfig错误"
		sc.Logger.Error(msg, zap.Any("采集池", reqObj.Name), zap.Any("yaml", reqObj.RelabelConfigsYamlString), zap.Error(err))
		common.FailWithMessage(msg, c)
		return
	}
	// 更新
	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新采集任务执行错误", zap.Error(err))
		common.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

// setScrapeJobEnableReq 请求参数结构体
type setScrapeJobEnableReq struct {
	Id     uint `json:"id" validate:"required"`
	Enable int  `json:"enable" validate:"required,oneof=1 2"` // 假设 1=启用 2=禁用
}

// setMonitorPromScrapeJobStatus 设置采集任务的启用/禁用状态
func setMonitorPromScrapeJobStatus(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj setScrapeJobEnableReq
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析修改采集任务状态请求失败", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 结构体数据校验
	err = validate.Struct(&reqObj)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	// 1. 查询数据库中原有的记录
	dbJob, err := models.GetMonitorPromScrapeJobById(int(reqObj.Id))
	if err != nil {
		sc.Logger.Error("根据id查找采集任务错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 2. 内存中修改状态
	dbJob.Enable = reqObj.Enable

	// 3. 执行更新
	err = dbJob.UpdateEnable()
	if err != nil {
		sc.Logger.Error("更新采集任务状态错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("状态修改成功", c)
}

func deleteMonitorPromScrapeJob(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetMonitorPromScrapeJobById(intVar)
	if err != nil {
		common.FailWithMessage("采集任务不存在", c)
		return
	}

	err = dbObj.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除采集任务执行错误", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

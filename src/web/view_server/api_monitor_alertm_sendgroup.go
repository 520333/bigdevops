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

// @Summary      获取AlertManager发送组列表
// @Description  获取AlertManager发送组列表 接口
// @Tags         monitor-alert
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取AlertManager发送组列表 响应结果"
// @Router       /monitor/getMonitorAlertManagerSendGroupList [get]
// @Security     Bearer
func getMonitorAlertManagerSendGroupList(c *gin.Context) {
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
	objs, err := models.GetMonitorAlertManagerSendGroupAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的发送组配置执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的发送组配置执行错误：%v", err.Error()), c)
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
			"items": []models.MonitorAlertManagerSendGroup{},
			"total": 0,
		}, "ok", c)
		return
	}

	// 根据过滤后的 ID 进行分页查询
	pagedObjs, err := models.GetMonitorAlertManagerSendGroupByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的发送组配置执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的发送组配置执行错误：%v", err.Error()), c)
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

// @Summary      创建AlertManager发送组
// @Description  创建AlertManager发送组 接口
// @Tags         monitor-alert
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "创建AlertManager发送组 响应结果"
// @Router       /monitor/createMonitorAlertManagerSendGroup [post]
// @Security     Bearer
func createMonitorAlertManagerSendGroup(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj models.MonitorAlertManagerSendGroup
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增发送组配置执行请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 获取当前用户ID
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)

	if dbUser != nil {
		reqObj.UserID = dbUser.ID
	}
	reqObj.FirstUpgradeUsers = commonGetUsersByNames(reqObj.FirstUserNames, sc.Logger, c)
	// 存入数据库
	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增发送组配置执行数据库失败", zap.Error(err))
		common.FailWithMessage("存入数据库失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

// @Summary      更新AlertManager发送组
// @Description  更新AlertManager发送组 接口
// @Tags         monitor-alert
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "更新AlertManager发送组 响应结果"
// @Router       /monitor/updateMonitorAlertManagerSendGroup [post]
// @Security     Bearer
func updateMonitorAlertManagerSendGroup(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	// 🚀 致命修复：同上
	var reqObj models.MonitorAlertManagerSendGroup
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新发送组配置请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 检查是否存在
	dbOld, err := models.GetMonitorAlertManagerSendGroupById(int(reqObj.ID))
	if err != nil {
		common.FailWithMessage("发送组配置不存在", c)
		return
	}
	reqObj.UserID = dbOld.UserID

	tmpUsers := commonGetUsersByNames(reqObj.FirstUserNames, sc.Logger, c)
	//reqObj.FirstUpgradeUsers = commonGetUsersByNames(reqObj.FirstUserNames, sc.Logger, c)
	// 更新
	err = reqObj.TransactionUpdate(tmpUsers)
	if err != nil {
		sc.Logger.Error("更新发送组配置执行错误", zap.Error(err))
		common.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

// setScrapeJobEnableReq 请求参数结构体
type setAlertManagerSendGroupEnableReq struct {
	Id     uint `json:"id" validate:"required"`
	Enable int  `json:"enable" validate:"required,oneof=1 2"` // 假设 1=启用 2=禁用
}

// @Summary      设置AlertManager发送组状态
// @Description  设置AlertManager发送组状态 接口
// @Tags         monitor-alert
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "设置AlertManager发送组状态 响应结果"
// @Router       /monitor/setAlertManagerSendGroupStatus [post]
// @Security     Bearer
func setAlertManagerSendGroupStatus(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj setAlertManagerSendGroupEnableReq
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析修改发送组配置状态请求失败", zap.Any("req", reqObj), zap.Error(err))
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
	dbJob, err := models.GetMonitorAlertManagerSendGroupById(int(reqObj.Id))
	if err != nil {
		sc.Logger.Error("根据id查找发送组配置错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 2. 内存中修改状态
	dbJob.Enable = reqObj.Enable

	// 3. 执行更新
	err = dbJob.UpdateEnable()
	if err != nil {
		sc.Logger.Error("更新发送组配置状态错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("状态修改成功", c)
}

// @Summary      删除AlertManager发送组
// @Description  删除AlertManager发送组 接口
// @Tags         monitor-alert
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "删除AlertManager发送组 响应结果"
// @Router       /monitor/deleteMonitorAlertManagerSendGroup/{id} [delete]
// @Security     Bearer
func deleteMonitorAlertManagerSendGroup(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetMonitorAlertManagerSendGroupById(intVar)
	if err != nil {
		common.FailWithMessage("发送组配置不存在", c)
		return
	}
	dbPromAlertRule, _ := models.GetMonitorPromAlertRuleBySendGroupId(uint(intVar))
	if dbPromAlertRule != nil && len(dbPromAlertRule) > 0 {
		sc.Logger.Warn("该发送组已经绑定了发送组，禁止直接删除！", zap.Any("", id))
		common.FailWithMessage("该发送组已经绑定了告警规则，禁止直接删除！", c)
		return
	}
	err = dbObj.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除发送组配置执行错误", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

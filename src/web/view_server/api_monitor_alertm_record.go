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
	"github.com/prometheus/prometheus/promql/parser"
	"go.uber.org/zap"
)

func getMonitorRecordRuleList(c *gin.Context) {
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
	objs, err := models.GetMonitorRecordRuleAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的聚合规则配置执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的聚合规则配置执行错误：%v", err.Error()), c)
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
			"items": []models.MonitorRecordRule{},
			"total": 0,
		}, "ok", c)
		return
	}

	// 根据过滤后的 ID 进行分页查询
	pagedObjs, err := models.GetMonitorRecordRuleByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的聚合规则配置执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的聚合规则配置执行错误：%v", err.Error()), c)
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

func createMonitorRecordRule(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj models.MonitorRecordRule
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增聚合规则配置执行请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 获取当前用户ID
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)

	pt := commonPromqlExprCheck(reqObj.Expr)

	if !pt.Success {
		sc.Logger.Error("promql语法校验失败", zap.Any("promql", pt))
		common.ReqBadFailWithMessage(fmt.Sprintf("promql语法校验失败: %v", pt.Err), c)
		return
	}

	dbUser, err := models.GetUserByUsername(userName)

	if dbUser != nil {
		reqObj.UserID = dbUser.ID
	}

	reqObj.FillDefaultData()
	// 存入数据库
	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增聚合规则配置执行数据库失败", zap.Error(err))
		common.FailWithMessage("存入数据库失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

func updateMonitorRecordRule(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	// 🚀 致命修复：同上
	var reqObj models.MonitorRecordRule
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新聚合规则配置请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 检查是否存在
	dbOld, err := models.GetMonitorRecordRuleById(int(reqObj.ID))
	if err != nil {
		common.FailWithMessage("聚合规则配置不存在", c)
		return
	}
	reqObj.UserID = dbOld.UserID
	pt := commonPromqlExprCheck(reqObj.Expr)
	if !pt.Success {
		sc.Logger.Error("promql语法校验失败", zap.Any("promql", pt))
		common.ReqBadFailWithMessage(fmt.Sprintf("promql语法校验失败: %v", pt.Err), c)
		return
	}
	reqObj.FillDefaultData()

	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新聚合规则配置执行错误", zap.Error(err))
		common.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

func deleteMonitorRecordRule(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetMonitorRecordRuleById(intVar)
	if err != nil {
		common.FailWithMessage("聚合规则配置不存在", c)
		return
	}

	err = dbObj.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除聚合规则配置执行错误", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

// 1. 定义批量删除的请求体
type deleteRecordRuleBatchReq struct {
	Ids []uint `json:"ids" validate:"required,min=1"` // 要求至少传 1 个 ID
}

// 2. 批量删除的处理函数
func deleteMonitorRecordRuleBatch(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj deleteRecordRuleBatchReq
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析批量删除聚合规则请求失败", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// （可选）如果你需要像单条删除那样检查依赖关系，可以在这里写个 for 循环检查 reqObj.Ids
	// 为了极致性能，这里直接调用批量删除
	err = models.DeleteMonitorRecordRuleBatch(reqObj.Ids)
	if err != nil {
		sc.Logger.Error("批量删除聚合规则执行错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage("批量删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("成功删除了 %d 条聚合规则", len(reqObj.Ids)), c)
}

// setScrapeJobEnableReq 请求参数结构体
type setRecordRuleEnableReq struct {
	Id     uint `json:"id" validate:"required"`
	Enable int  `json:"enable" validate:"required,oneof=1 2"` // 假设 1=启用 2=禁用
}

// setScrapeJobStatus 设置聚合规则配置的启用/禁用状态
func setRecordRuleStatus(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj setRecordRuleEnableReq
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("修改聚合规则状态请求失败", zap.Any("req", reqObj), zap.Error(err))
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
	dbJob, err := models.GetMonitorRecordRuleById(int(reqObj.Id))
	if err != nil {
		sc.Logger.Error("根据id查找聚合规则配置错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 2. 内存中修改状态
	dbJob.Enable = reqObj.Enable

	// 3. 执行更新
	err = dbJob.UpdateEnable()
	if err != nil {
		sc.Logger.Error("更新聚合规则配置状态错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("状态修改成功", c)
}

func setRecordRuleStatusBatch(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj setRecordRuleEnableBatchReq
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析批量修改聚合规则配置状态请求失败", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 结构体数据校验
	err = validate.Struct(&reqObj)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求参数有误", c)
			return
		}
	}

	// 执行数据库批量更新
	err = models.UpdateMonitorRecordRuleEnableBatch(reqObj.Ids, reqObj.Enable)
	if err != nil {
		sc.Logger.Error("批量更新聚合规则状态错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage("批量更新失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("成功修改了 %d 条聚合规则的状态", len(reqObj.Ids)), c)
}

type RecordRulePromQLCheckResult struct {
	Success bool   `json:"success"`
	Err     string `json:"err"`
}

func recordRuleCommonPromqlExprCheck(ql string) (pt RecordRulePromQLCheckResult) {
	pt.Success = true
	_, err := parser.NewParser(parser.Options{}).ParseExpr(ql)
	if err != nil {
		pt.Success = false
		pt.Err = err.Error()
	}
	return
}

func recordRulePromqlExprCheck(c *gin.Context) {
	ql := c.DefaultQuery("ql", "")
	pt := recordRuleCommonPromqlExprCheck(ql)

	if !pt.Success {
		sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
		sc.Logger.Warn("前端请求promql语法校验失败", zap.Any("promql", pt))

		common.ReqBadFailWithMessage(fmt.Sprintf("promql语法校验失败: %v", pt.Err), c)
		return
	}

	common.OkWithDetailed(pt, "语法校验通过", c)
}

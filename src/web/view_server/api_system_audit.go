package view_server

import (
	"strconv"

	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// @Summary      获取操作审计日志列表
// @Description  获取操作审计日志列表 接口
// @Tags         system-audit
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取操作审计日志列表 响应结果"
// @Router       /system/getAuditLogList [get]
// @Security     Bearer
func getAuditLogList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	statusCode, _ := strconv.Atoi(c.DefaultQuery("status", "0"))

	filter := models.AuditLogFilter{
		Username:  c.DefaultQuery("userName", ""),
		Module:    c.DefaultQuery("module", ""),
		Status:    statusCode,
		StartDate: c.DefaultQuery("startDate", ""),
		EndDate:   c.DefaultQuery("endDate", ""),
	}

	logs, total, err := models.GetAuditLogList(filter, page, pageSize)
	if err != nil {
		sc.Logger.Error("查询审计日志列表失败", zap.Error(err))
		common.FailWithMessage("查询审计日志失败: "+err.Error(), c)
		return
	}

	if logs == nil {
		logs = []*models.SystemAuditLog{}
	}

	common.OkWithDetailed(gin.H{
		"items": logs,
		"total": total,
	}, "ok", c)
}

// @Summary      获取登录审计日志列表
// @Description  获取用户登录/登出审计日志列表 接口
// @Tags         system-audit
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取登录审计日志列表 响应结果"
// @Router       /system/getLoginLogList [get]
// @Security     Bearer
func getLoginLogList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	var statusPtr *int
	if statusStr := c.Query("status"); statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil {
			statusPtr = &s
		}
	}

	filter := models.LoginLogFilter{
		Username:  c.DefaultQuery("userName", ""),
		LoginType: c.DefaultQuery("loginType", ""),
		Status:    statusPtr,
		StartDate: c.DefaultQuery("startDate", ""),
		EndDate:   c.DefaultQuery("endDate", ""),
	}

	logs, total, err := models.GetLoginLogList(filter, page, pageSize)
	if err != nil {
		sc.Logger.Error("查询登录日志列表失败", zap.Error(err))
		common.FailWithMessage("查询登录日志失败: "+err.Error(), c)
		return
	}

	if logs == nil {
		logs = []*models.SystemLoginLog{}
	}

	common.OkWithDetailed(gin.H{
		"items": logs,
		"total": total,
	}, "ok", c)
}

package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// @Summary      获取 Archery 配置与基础信息
// @Tags         archery
// @Accept       json
// @Produce      json
// @Router       /archery/config [get]
func getArcheryConfig(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	if sc.ArcheryC == nil || !sc.ArcheryC.Enable {
		common.OkWithData(gin.H{
			"enable":  false,
			"baseUrl": "",
		}, c)
		return
	}

	common.OkWithData(gin.H{
		"enable":  true,
		"baseUrl": sc.ArcheryC.BaseURL,
	}, c)
}

// @Summary      获取 Archery 数据库实例列表
// @Tags         archery
// @Accept       json
// @Produce      json
// @Router       /archery/instances [get]
func getArcheryInstances(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArcheryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	instances, err := client.GetInstances()
	if err != nil {
		sc.Logger.Error("[Archery] 获取数据库实例列表失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("获取数据库实例失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(instances, c)
}

// @Summary      获取指定实例下的数据库列表
// @Tags         archery
// @Accept       json
// @Produce      json
// @Param        instanceId query int true "实例ID"
// @Router       /archery/databases [get]
func getArcheryDatabases(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArcheryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	instanceIdStr := c.Query("instanceId")
	instanceId, err := strconv.Atoi(instanceIdStr)
	if err != nil || instanceId <= 0 {
		common.ReqBadFailWithMessage("无效的 instanceId", c)
		return
	}

	dbs, err := client.GetDatabases(instanceId)
	if err != nil {
		sc.Logger.Error("[Archery] 获取数据库列表失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("获取数据库列表失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(dbs, c)
}

type archerySqlCheckReq struct {
	InstanceId int    `json:"instanceId" binding:"required"`
	DbName     string `json:"dbName" binding:"required"`
	FullSql    string `json:"fullSql" binding:"required"`
}

// @Summary      基于 Inception 执行 SQL 语法合规预审核
// @Tags         archery
// @Accept       json
// @Produce      json
// @Param        body body archerySqlCheckReq true "SQL 检测请求"
// @Router       /archery/sqlCheck [post]
func checkArcherySql(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArcheryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	var req archerySqlCheckReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("参数错误: %v", err.Error()), c)
		return
	}

	result, err := client.SqlCheck(req.InstanceId, req.DbName, req.FullSql)
	if err != nil {
		sc.Logger.Error("[Archery] SQL检测失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("SQL检测失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(result, c)
}

// @Summary      获取 Archery SQL 上线工单列表
// @Tags         archery
// @Accept       json
// @Produce      json
// @Param        page query int false "当前页"
// @Param        pageSize query int false "页大小"
// @Param        status query string false "工单状态"
// @Param        instanceId query int false "实例ID"
// @Param        groupId query int false "组ID"
// @Param        startTime query string false "起始时间"
// @Param        endTime query string false "结束时间"
// @Param        search query string false "搜索关键词"
// @Router       /archery/workflows [get]
func getArcheryWorkflows(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArcheryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	search := c.Query("search")
	status := c.Query("status")
	instanceId, _ := strconv.Atoi(c.Query("instanceId"))
	groupId, _ := strconv.Atoi(c.Query("groupId"))
	startTime := c.Query("startTime")
	endTime := c.Query("endTime")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filter := common.ArcheryWorkflowFilter{
		Page:       page,
		PageSize:   pageSize,
		Search:     search,
		Status:     status,
		InstanceId: instanceId,
		GroupId:    groupId,
		StartTime:  startTime,
		EndTime:    endTime,
	}

	resp, err := client.GetWorkflows(filter)
	if err != nil {
		sc.Logger.Error("[Archery] 获取工单列表失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("获取工单列表失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(gin.H{
		"total": resp.Count,
		"items": resp.Results,
	}, c)
}

// @Summary      获取 Archery 组列表
// @Tags         archery
// @Accept       json
// @Produce      json
// @Router       /archery/groups [get]
func getArcheryGroups(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArcheryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	groups, err := client.GetGroups()
	if err != nil {
		sc.Logger.Error("[Archery] 获取组列表失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("获取组列表失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(groups, c)
}

// @Summary      提交 SQL 上线变更工单
// @Tags         archery
// @Accept       json
// @Produce      json
// @Param        body body common.ArcheryCreateWorkflowReq true "工单提交内容"
// @Router       /archery/createWorkflow [post]
func createArcheryWorkflow(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArcheryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	var req common.ArcheryCreateWorkflowReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("参数错误: %v", err.Error()), c)
		return
	}

	currentUserName := c.GetString(common.GIN_CTX_JWT_USER_NAME)
	if req.Engineer == "" {
		req.Engineer = currentUserName
	}
	if req.Engineer == "" && sc.ArcheryC != nil {
		req.Engineer = sc.ArcheryC.Username
	}

	wfID, err := client.CreateWorkflow(req)
	if err != nil {
		sc.Logger.Error("[Archery] 提交工单失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("提交工单失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(gin.H{
		"workflowId": wfID,
		"message":    "提交 SQL 变更工单成功",
	}, c)
}

type archeryAuditReq struct {
	WorkflowId  int    `json:"workflowId" binding:"required"`
	AuditType   string `json:"auditType" binding:"required"` // "pass" or "cancel"
	AuditRemark string `json:"auditRemark"`
	Engineer    string `json:"engineer"`
}

// @Summary      审核 Archery 工单
// @Tags         archery
// @Accept       json
// @Produce      json
// @Param        body body archeryAuditReq true "审批请求"
// @Router       /archery/audit [post]
func auditArcheryWorkflow(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArcheryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	var req archeryAuditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("参数错误: %v", err.Error()), c)
		return
	}

	engineer := req.Engineer
	if engineer == "" {
		engineer = c.GetString(common.GIN_CTX_JWT_USER_NAME)
	}
	if engineer == "" && sc.ArcheryC != nil {
		engineer = sc.ArcheryC.Username
	}

	msg, err := client.AuditWorkflow(req.WorkflowId, req.AuditType, req.AuditRemark, engineer)
	if err != nil {
		sc.Logger.Error("[Archery] 审核工单失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("审核工单失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(gin.H{
		"message": msg,
	}, c)
}

type archeryExecuteReq struct {
	WorkflowId int    `json:"workflowId" binding:"required"`
	Mode       string `json:"mode"` // "auto" or "manual"
	Engineer   string `json:"engineer"`
}

// @Summary      执行 Archery 上线工单
// @Tags         archery
// @Accept       json
// @Produce      json
// @Param        body body archeryExecuteReq true "执行请求"
// @Router       /archery/execute [post]
func executeArcheryWorkflow(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArcheryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	var req archeryExecuteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("参数错误: %v", err.Error()), c)
		return
	}

	engineer := req.Engineer
	if engineer == "" {
		engineer = c.GetString(common.GIN_CTX_JWT_USER_NAME)
	}
	if engineer == "" && sc.ArcheryC != nil {
		engineer = sc.ArcheryC.Username
	}

	msg, err := client.ExecuteWorkflow(req.WorkflowId, req.Mode, engineer)
	if err != nil {
		sc.Logger.Error("[Archery] 执行工单失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("执行工单失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(gin.H{
		"message": msg,
	}, c)
}

// @Summary      获取 Archery 工单操作日志
// @Tags         archery
// @Accept       json
// @Produce      json
// @Param        workflowId query int true "工单ID"
// @Router       /archery/logs [get]
func getArcheryWorkflowLogs(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	client, err := common.NewArcheryClient(sc)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	workflowId, _ := strconv.Atoi(c.Query("workflowId"))
	if workflowId <= 0 {
		common.ReqBadFailWithMessage("缺少必填参数 workflowId", c)
		return
	}

	logs, err := client.GetWorkflowLogs(workflowId)
	if err != nil {
		sc.Logger.Error("[Archery] 获取工单操作日志失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("获取工单日志失败: %v", err.Error()), c)
		return
	}

	common.OkWithData(logs, c)
}

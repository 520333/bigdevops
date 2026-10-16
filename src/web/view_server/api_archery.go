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

	// 针对当前登录用户进行实例数据权限隔离：
	// 仅返回该用户所属资源组关联的实例，杜绝跨组越权查看其他业务/生产实例
	currentUsername := c.GetString(common.GIN_CTX_JWT_USER_NAME)
	if currentUsername != "" {
		isPrivileged := false
		if dbUser, errUser := models.GetUserByUsername(currentUsername); errUser == nil && dbUser != nil {
			for _, r := range dbUser.Roles {
				rv := strings.ToLower(r.RoleValue)
				if rv == "super" || rv == "admin" || rv == "bot_super" {
					isPrivileged = true
					break
				}
			}
		}

		if !isPrivileged {
			archeryUser, errUser := client.GetUserDetail(currentUsername)
			if errUser == nil && archeryUser != nil && !archeryUser.IsSuperuser {
				allowedMap := make(map[int]bool)
				for _, gid := range archeryUser.ResourceGroup {
					allowedMap[gid] = true
				}
				var filtered []common.ArcheryInstance
				for _, inst := range instances {
					hasAccess := false
					for _, rg := range inst.ResourceGroup {
						if allowedMap[rg] {
							hasAccess = true
							break
						}
					}
					if hasAccess {
						filtered = append(filtered, inst)
					}
				}
				common.OkWithData(filtered, c)
				return
			}
		}
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

	// 方案2：按所属资源组隔离 (超级管理员具备全量工单透视权限)
	currentUsername := c.GetString(common.GIN_CTX_JWT_USER_NAME)
	isPrivileged := false
	var allowedGroupIds []int

	if currentUsername != "" {
		if dbUser, errUser := models.GetUserByUsername(currentUsername); errUser == nil && dbUser != nil {
			for _, r := range dbUser.Roles {
				rv := strings.ToLower(r.RoleValue)
				if rv == "super" || rv == "admin" || rv == "bot_super" {
					isPrivileged = true
					break
				}
			}
		}

		if !isPrivileged {
			archeryUser, errUser := client.GetUserDetail(currentUsername)
			if errUser == nil && archeryUser != nil {
				if archeryUser.IsSuperuser {
					isPrivileged = true
				} else {
					allowedGroupIds = archeryUser.ResourceGroup
				}
			}
		}
	}

	filter := common.ArcheryWorkflowFilter{
		Page:            page,
		PageSize:        pageSize,
		Search:          search,
		Status:          status,
		InstanceId:      instanceId,
		GroupId:         groupId,
		StartTime:       startTime,
		EndTime:         endTime,
		AllowedGroupIds: allowedGroupIds,
		CurrentUser:     currentUsername,
		IsPrivileged:    isPrivileged,
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

	// 针对当前登录用户进行所属资源组数据权限隔离：
	// 若当前用户在 Archery 中配置了专属的 resource_group，且不是超级管理员，则严格对其可见组进行过滤
	currentUsername := c.GetString(common.GIN_CTX_JWT_USER_NAME)
	if currentUsername != "" {
		isPrivileged := false
		if dbUser, errUser := models.GetUserByUsername(currentUsername); errUser == nil && dbUser != nil {
			for _, r := range dbUser.Roles {
				rv := strings.ToLower(r.RoleValue)
				if rv == "super" || rv == "admin" || rv == "bot_super" {
					isPrivileged = true
					break
				}
			}
		}

		if !isPrivileged {
			archeryUser, errUser := client.GetUserDetail(currentUsername)
			if errUser == nil && archeryUser != nil && !archeryUser.IsSuperuser {
				allowedMap := make(map[int]bool)
				for _, gid := range archeryUser.ResourceGroup {
					allowedMap[gid] = true
				}
				var filtered []common.ArcheryGroupItem
				for _, g := range groups {
					if allowedMap[g.ID] {
						filtered = append(filtered, g)
					}
				}
				common.OkWithData(filtered, c)
				return
			}
		}
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

	currentUsername := c.GetString(common.GIN_CTX_JWT_USER_NAME)
	engineer := req.Engineer
	if engineer == "" {
		engineer = currentUsername
	}
	if engineer == "" && sc.ArcheryC != nil {
		engineer = sc.ArcheryC.Username
	}

	// 环境分级与自批合规校验：
	// - 测试/开发环境：允许发起人自批自测闭环；
	// - 生产环境 (Prod)：仅允许非发起人（DBA / 运维 / 超管）审批放行，严禁发起人自行审批通过。
	if req.AuditType == "pass" {
		wfItem, errWf := client.GetWorkflowDetail(req.WorkflowId)
		if errWf == nil && wfItem != nil {
			isProd := client.IsProductionWorkflow(&wfItem.Workflow)
			// 发起人账号与当前操作人账号一致（发起人尝试自批）
			if isProd && strings.EqualFold(wfItem.Workflow.Engineer, currentUsername) {
				// 检查当前用户是否具备超级管理员特权角色 (super / admin / bot_super)
				isPrivileged := false
				if dbUser, errUser := models.GetUserByUsername(currentUsername); errUser == nil && dbUser != nil {
					for _, r := range dbUser.Roles {
						rv := strings.ToLower(r.RoleValue)
						if rv == "super" || rv == "admin" || rv == "bot_super" {
							isPrivileged = true
							break
						}
					}
				}
				if !isPrivileged {
					common.ReqBadFailWithMessage("【合规拦截】生产环境工单严禁发起人自行审批，请由 DBA 或指定审核人复核！", c)
					return
				}
			}
		}
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

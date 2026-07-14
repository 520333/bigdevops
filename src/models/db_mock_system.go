package models

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"fmt"

	"go.uber.org/zap"
)

func mockSystemData(sc *config.ServerConfig) *User {
	menus := []*Menu{
		{Name: "System", Title: "系统管理", Icon: "ant-design:setting-outlined", Type: "0", Show: "1", OrderNo: 90, Component: "LAYOUT", Redirect: "system/account", Path: "/system"},
		{Name: "MenuManagement", Title: "菜单管理", Icon: "ant-design:menu-outlined", Type: "1", Show: "1", OrderNo: 91, Component: "system/menu/index", Pid: 1, Path: "menu"},
		{Name: "AccountManagement", Title: "用户管理", Icon: "ant-design:user-outlined", Type: "1", Show: "1", OrderNo: 92, Component: "system/account/index", Pid: 1, Path: "account"},
		{Name: "RoleManagement", Title: "角色管理", Icon: "ant-design:solution-outlined", Type: "1", Show: "1", OrderNo: 93, Component: "system/role/index", Pid: 1, Path: "role"},
		{Name: "ChangePassword", Title: "修改密码", Icon: "ant-design:key-outlined", Type: "1", Show: "1", OrderNo: 94, Component: "system/password/index", Pid: 1, Path: "changePassword"},
		{Name: "ApiManagement", Title: "接口授权", Icon: "ant-design:api-outlined", Type: "1", Show: "1", OrderNo: 95, Component: "system/api/index", Pid: 1, Path: "api"},

		{Name: "PermissionManagement", Title: "权限管理", Icon: "ion:layers-outline", Type: "0", Show: "1", OrderNo: 100, Component: "LAYOUT", Redirect: "/permission/front/page", Path: "/permission"},
		{Name: "PermissionFront", Title: "前端权限管理", Icon: "ion:layers-outline", Type: "1", Show: "1", OrderNo: 101, Component: "/permission/front/index", Pid: 7, Path: "front"},

		{Name: "ServiceTree", Title: "CMDB资产管理", Icon: "ant-design:database-outlined", Type: "0", Show: "1", OrderNo: 10, Component: "LAYOUT", Path: "/serviceTree", Redirect: "/serviceTree/service/index"},
		{Name: "ServiceTreeIndexAsync", Title: "服务树", Icon: "ant-design:node-index-outlined", Type: "1", Show: "1", OrderNo: 11, Component: "stree/stree/indexAsync", Pid: 9, Path: "streeAsync"},

		{Name: "WorkOrder", Title: "工单服务", Icon: "ant-design:reconciliation-outlined", Type: "0", Show: "1", OrderNo: 20, Component: "LAYOUT", Path: "/workOrder", Redirect: "/workOrder/process/index"},
		{Name: "ProcessManagement", Title: "审批流程管理", Icon: "ant-design:apartment-outlined", Type: "1", Show: "1", OrderNo: 21, Component: "workorder/process/index", Pid: 11, Path: "process"},
		{Name: "FormManagement", Title: "表单设计管理", Icon: "ant-design:form-outlined", Type: "1", Show: "1", OrderNo: 22, Component: "workorder/formDesign/index", Pid: 11, Path: "formDesign"},
		{Name: "WorkOrderTemplateManagement", Title: "工单模板管理", Icon: "ant-design:layout-outlined", Type: "1", Show: "1", OrderNo: 23, Component: "workorder/template/index", Pid: 11, Path: "template"},
		{Name: "WorkOrderTicket", Title: "工单申请", Icon: "ant-design:profile-outlined", Type: "1", Show: "1", OrderNo: 24, Component: "workorder/ticket/index", Pid: 11, Path: "ticket"},
		{Name: "WorkOrderCreate", Title: "工单填写", Icon: "ant-design:form-outlined", Type: "1", Show: "0", OrderNo: 25, Component: "workorder/ticket/create", Pid: 11, Path: "create"},
		{Name: "WorkOrderSearch", Title: "我的工单", Icon: "ant-design:profile-outlined", Type: "1", Show: "1", OrderNo: 26, Component: "workorder/ticket/search", Pid: 11, Path: "search"},

		{Name: "JobExec", Title: "任务执行中心", Icon: "ant-design:thunderbolt-outlined", Type: "0", Show: "1", OrderNo: 30, Component: "LAYOUT", Path: "/jobExec", Redirect: "/jobExec/task/index"},
		{Name: "JobExecScript", Title: "脚本管理", Icon: "ant-design:code-outlined", Type: "1", Show: "1", OrderNo: 31, Component: "jobExec/script/index", Pid: 18, Path: "script"},
		{Name: "JobExecTask", Title: "任务管理", Icon: "ant-design:schedule-outlined", Type: "1", Show: "1", OrderNo: 32, Component: "jobExec/task/index", Pid: 18, Path: "task"},

		{Name: "Monitor", Title: "监控中心", Icon: "ant-design:dashboard-outlined", Type: "0", Show: "1", OrderNo: 40, Component: "LAYOUT", Path: "/monitor", Redirect: "/monitor/scrape/index"},
		{Name: "MonitorPromPool", Title: "prom集群实例管理", Icon: "ant-design:database-outlined", Type: "1", Show: "1", OrderNo: 41, Component: "monitor/pool/index", Pid: 21, Path: "pool"},
		{Name: "MonitorScrapeJob", Title: "prom采集任务管理", Icon: "ant-design:api-outlined", Type: "1", Show: "1", OrderNo: 42, Component: "monitor/scrape/index", Pid: 21, Path: "scrape"},
		{Name: "MonitorAlertRule", Title: "prom告警规则管理", Icon: "ant-design:fund-view-outlined", Type: "1", Show: "1", OrderNo: 43, Component: "monitor/alertrule/index", Pid: 21, Path: "alertrule"},
		{Name: "MonitorRecordRule", Title: "prom聚合规则管理", Icon: "ant-design:fund-view-outlined", Type: "1", Show: "1", OrderNo: 44, Component: "monitor/recordrule/index", Pid: 21, Path: "recordrule"},

		{Name: "MonitorAlertPool", Title: "alert集群实例管理", Icon: "ant-design:alert-outlined", Type: "1", Show: "1", OrderNo: 45, Component: "monitor/alertmanager/index", Pid: 21, Path: "alertmanager"},
		{Name: "MonitorSendGroup", Title: "alert发送组管理", Icon: "ant-design:dingding-outlined", Type: "1", Show: "1", OrderNo: 46, Component: "monitor/sendgroup/index", Pid: 21, Path: "sendgroup"},
		{Name: "MonitorAlertEvent", Title: "alert告警事件管理", Icon: "ant-design:project-outlined", Type: "1", Show: "1", OrderNo: 47, Component: "monitor/event/index", Pid: 21, Path: "event"},

		{Name: "MonitorOnDutyGroup", Title: "值班组设置", Icon: "ant-design:ungroup-outlined", Type: "1", Show: "1", OrderNo: 48, Component: "monitor/ondutygroup/index", Pid: 21, Path: "ondutygroup"},
		{Name: "MonitorOnDutyGroupPlan", Title: "轮值排班表", Icon: "ant-design:calendar-outlined", Type: "1", Show: "1", OrderNo: 49, Component: "monitor/plan/index", Pid: 21, Path: "plan"},
	}

	apis := []*Api{
		// ================== 系统管理模块 ==================
		{Path: "/api/system", Method: "GET", Title: "系统管理模块", Type: "0", Pid: 0},
		// 基础信息
		{Path: "/api/getUserInfo", Method: "GET", Title: "获取用户信息", Type: "1", Pid: 1},
		{Path: "/api/getPermCode", Method: "GET", Title: "获取用户权限码", Type: "1", Pid: 1},
		// 菜单路由
		{Path: "/api/system/getMenuList", Method: "GET", Title: "获取用户菜单", Type: "1", Pid: 1},
		{Path: "/api/system/getMenuListAll", Method: "GET", Title: "获取全量菜单", Type: "1", Pid: 1},
		{Path: "/api/system/createMenu", Method: "POST", Title: "创建菜单", Type: "1", Pid: 1},
		{Path: "/api/system/updateMenu", Method: "POST", Title: "更新菜单", Type: "1", Pid: 1},
		{Path: "/api/system/deleteMenu/:id", Method: "DELETE", Title: "删除菜单", Type: "1", Pid: 1},
		// 角色路由
		{Path: "/api/system/getRoleListAll", Method: "GET", Title: "获取全量角色", Type: "1", Pid: 1},
		{Path: "/api/system/createRole", Method: "POST", Title: "创建角色", Type: "1", Pid: 1},
		{Path: "/api/system/updateRole", Method: "POST", Title: "更新角色", Type: "1", Pid: 1},
		{Path: "/api/system/setRoleStatus", Method: "POST", Title: "设置角色状态", Type: "1", Pid: 1},
		{Path: "/api/system/deleteRole/:id", Method: "DELETE", Title: "删除角色", Type: "1", Pid: 1},
		// 账号路由
		{Path: "/api/system/getAccountList", Method: "GET", Title: "获取账号列表", Type: "1", Pid: 1},
		{Path: "/api/system/createAccount", Method: "POST", Title: "创建账号", Type: "1", Pid: 1},
		{Path: "/api/system/updateAccount", Method: "POST", Title: "更新账号", Type: "1", Pid: 1},
		{Path: "/api/system/setAccountStatus", Method: "POST", Title: "设置账号状态", Type: "1", Pid: 1},
		{Path: "/api/system/accountExist", Method: "POST", Title: "检查账号存在", Type: "1", Pid: 1},
		{Path: "/api/system/deleteAccount/:id", Method: "DELETE", Title: "删除账号", Type: "1", Pid: 1},
		{Path: "/api/system/changePassword", Method: "POST", Title: "修改密码", Type: "1", Pid: 1},
		{Path: "/api/system/getAllUserAndRoles", Method: "GET", Title: "获取所有用户与角色", Type: "1", Pid: 1},
		// API路由
		{Path: "/api/system/getApiList", Method: "GET", Title: "获取API列表", Type: "1", Pid: 1},
		{Path: "/api/system/getApiListAll", Method: "GET", Title: "获取全量API", Type: "1", Pid: 1},
		{Path: "/api/system/createApi", Method: "POST", Title: "创建API", Type: "1", Pid: 1},
		{Path: "/api/system/updateApi", Method: "POST", Title: "更新API", Type: "1", Pid: 1},
		{Path: "/api/system/deleteApi/:id", Method: "DELETE", Title: "删除API", Type: "1", Pid: 1},

		//// ================== CMDB资产管理模块 ==================
		{Path: "/api/stree", Method: "GET", Title: "CMDB资产管理模块", Type: "0"},
		//// 服务树
		{Path: "/api/stree/getStreeNodeList", Method: "GET", Title: "获取服务树节点列表", Type: "1", Pid: 27},
		{Path: "/api/stree/getStreeNodeSelect", Method: "GET", Title: "获取服务树下拉选择", Type: "1", Pid: 27},
		{Path: "/api/stree/getTopStreeNodes", Method: "GET", Title: "获取服务树顶层节点", Type: "1", Pid: 27},
		{Path: "/api/stree/createStreeNode", Method: "POST", Title: "创建服务树节点", Type: "1", Pid: 27},
		{Path: "/api/stree/updateStreeNode", Method: "POST", Title: "更新服务树节点", Type: "1", Pid: 27},
		{Path: "/api/stree/deleteStreeNode/:id", Method: "DELETE", Title: "删除服务树节点", Type: "1", Pid: 27},
		{Path: "/api/stree/getChildrenStreeNodes/:pid", Method: "GET", Title: "获取服务树子节点", Type: "1", Pid: 27},
		{Path: "/api/stree/getLeafStreeNodes", Method: "GET", Title: "获取服务树叶子节点", Type: "1", Pid: 27},
		{Path: "/api/stree/fetchResourceByNode", Method: "GET", Title: "根据节点获取资源", Type: "1", Pid: 27},
		// ECS
		{Path: "/api/stree/getResourceEcsUnbindList", Method: "GET", Title: "获取未绑定ECS", Type: "1", Pid: 27},
		{Path: "/api/stree/bindEcsToStreeNode", Method: "POST", Title: "绑定ECS到节点", Type: "1", Pid: 27},
		{Path: "/api/stree/unBindEcsToStreeNode", Method: "POST", Title: "解绑节点ECS", Type: "1", Pid: 27},
		{Path: "/api/stree/getStreeNodeEcsList/:id", Method: "GET", Title: "获取节点ECS列表", Type: "1", Pid: 27},
		{Path: "/api/stree/getResourceEcsList", Method: "GET", Title: "获取资源ECS列表", Type: "1", Pid: 27},
		// ELB
		{Path: "/api/stree/getResourceElbUnbindList", Method: "GET", Title: "获取未绑定ELB", Type: "1", Pid: 27},
		{Path: "/api/stree/bindElbToStreeNode", Method: "POST", Title: "绑定ELB到节点", Type: "1", Pid: 27},
		{Path: "/api/stree/unBindElbToStreeNode", Method: "POST", Title: "解绑节点ELB", Type: "1", Pid: 27},
		// RDS
		{Path: "/api/stree/getResourceRdsUnbindList", Method: "GET", Title: "获取未绑定RDS", Type: "1", Pid: 27},
		{Path: "/api/stree/bindRdsToStreeNode", Method: "POST", Title: "绑定RDS到节点", Type: "1", Pid: 27},
		{Path: "/api/stree/unBindRdsToStreeNode", Method: "POST", Title: "解绑节点RDS", Type: "1", Pid: 27},

		// ================== 工单服务模块 ==================
		{Path: "/api/workorder", Method: "GET", Title: "工单服务模块", Type: "0"},
		// 流程
		{Path: "/api/workorder/getProcessList", Method: "GET", Title: "获取流程列表", Type: "1", Pid: 48},
		{Path: "/api/workorder/createProcess", Method: "POST", Title: "创建流程", Type: "1", Pid: 48},
		{Path: "/api/workorder/updateProcess", Method: "POST", Title: "更新流程", Type: "1", Pid: 48},
		{Path: "/api/workorder/deleteProcess/:id", Method: "DELETE", Title: "删除流程", Type: "1", Pid: 48},
		// 表单设计
		{Path: "/api/workorder/getFormDesignList", Method: "GET", Title: "获取表单设计列表", Type: "1", Pid: 48},
		{Path: "/api/workorder/createFormDesign", Method: "POST", Title: "创建表单设计", Type: "1", Pid: 48},
		{Path: "/api/workorder/updateFormDesign", Method: "POST", Title: "更新表单设计", Type: "1", Pid: 48},
		{Path: "/api/workorder/deleteFormDesign/:id", Method: "DELETE", Title: "删除表单设计", Type: "1", Pid: 48},
		// 工单模板
		{Path: "/api/workorder/getWorkOrderTemplateList", Method: "GET", Title: "获取工单模板列表", Type: "1", Pid: 48},
		{Path: "/api/workorder/getWorkOrderTemplateDetail/:id", Method: "GET", Title: "获取工单模板详情", Type: "1", Pid: 48},
		{Path: "/api/workorder/createWorkOrderTemplate", Method: "POST", Title: "创建工单模板", Type: "1", Pid: 48},
		{Path: "/api/workorder/updateWorkOrderTemplate", Method: "POST", Title: "更新工单模板", Type: "1", Pid: 48},
		{Path: "/api/workorder/deleteWorkOrderTemplate/:id", Method: "DELETE", Title: "删除工单模板", Type: "1", Pid: 48},
		// 工单实例
		{Path: "/api/workorder/getWorkOrderInstanceList", Method: "GET", Title: "获取工单实例列表", Type: "1", Pid: 48},
		{Path: "/api/workorder/getWorkOrderInstanceDetail/:id", Method: "GET", Title: "获取工单实例详情", Type: "1", Pid: 48},
		{Path: "/api/workorder/createWorkOrderInstance", Method: "POST", Title: "创建工单实例", Type: "1", Pid: 48},
		{Path: "/api/workorder/updateWorkOrderInstance", Method: "POST", Title: "更新工单实例", Type: "1", Pid: 48},
		{Path: "/api/workorder/deleteWorkOrderInstance/:id", Method: "DELETE", Title: "删除工单实例", Type: "1", Pid: 48},
		{Path: "/api/workorder/approvalWorkOrderInstance/:id", Method: "POST", Title: "审批工单", Type: "1", Pid: 48},
		{Path: "/api/workorder/actionWorkOrderInstance/:id", Method: "POST", Title: "执行工单动作", Type: "1", Pid: 48},
		{Path: "/api/workorder/commentWorkOrderInstance/:id", Method: "POST", Title: "评论工单", Type: "1", Pid: 48},

		// ================== 任务执行中心模块 ==================
		{Path: "/api/jobexec", Method: "GET", Title: "任务执行中心模块", Type: "0"},
		// 脚本管理
		{Path: "/api/jobexec/getJobExecScriptList", Method: "GET", Title: "获取脚本列表", Type: "1", Pid: 70},
		{Path: "/api/jobexec/getJobExecScriptSelect", Method: "GET", Title: "获取脚本下拉选择", Type: "1", Pid: 70},
		{Path: "/api/jobexec/getJobExecScriptOne/:id", Method: "GET", Title: "获取单个脚本", Type: "1", Pid: 70},
		{Path: "/api/jobexec/getJobExecScriptDetail/:id", Method: "GET", Title: "获取脚本详情", Type: "1", Pid: 70},
		{Path: "/api/jobexec/createJobExecScript", Method: "POST", Title: "创建脚本", Type: "1", Pid: 70},
		{Path: "/api/jobexec/updateJobExecScript", Method: "POST", Title: "更新脚本", Type: "1", Pid: 70},
		{Path: "/api/jobexec/deleteJobExecScript/:id", Method: "DELETE", Title: "删除脚本", Type: "1", Pid: 70},
		// 任务管理
		{Path: "/api/jobexec/getJobExecTaskList", Method: "GET", Title: "获取任务列表", Type: "1", Pid: 70},
		{Path: "/api/jobexec/getJobExecTaskOne/:id", Method: "GET", Title: "获取单个任务", Type: "1", Pid: 70},
		{Path: "/api/jobexec/createJobExecTask", Method: "POST", Title: "创建任务", Type: "1", Pid: 70},
		{Path: "/api/jobexec/updateJobExecTask", Method: "POST", Title: "更新任务", Type: "1", Pid: 70},
		{Path: "/api/jobexec/deleteJobExecTask/:id", Method: "DELETE", Title: "删除任务", Type: "1", Pid: 70},
		{Path: "/api/jobexec/actionJobExecTaskOne/:id", Method: "POST", Title: "执行任务", Type: "1", Pid: 70},
		{Path: "/api/jobexec/getJobExecResultByJobId", Method: "GET", Title: "获取任务结果", Type: "1", Pid: 70},

		// ================== 监控中心模块 ==================
		{Path: "/api/monitor", Method: "GET", Title: "监控中心模块", Type: "0"},
		// Prometheus 集群
		{Path: "/api/monitor/getMonitorScrapePoolList", Method: "GET", Title: "获取Prom集群列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorScrapePool", Method: "POST", Title: "创建Prom集群", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorScrapePool", Method: "POST", Title: "更新Prom集群", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorScrapePool/:id", Method: "DELETE", Title: "删除Prom集群", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorPrometheusYamlOne", Method: "GET", Title: "获取Prom主配置", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorPrometheusAlertRuleYamlOne", Method: "GET", Title: "获取告警规则配置", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorPrometheusRecordRuleYamlOne", Method: "GET", Title: "获取预聚合规则配置", Type: "1", Pid: 85},
		// Prometheus 采集任务
		{Path: "/api/monitor/getMonitorScrapeJobList", Method: "GET", Title: "获取采集任务列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorScrapeJobOne", Method: "GET", Title: "获取单个采集任务", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorScrapeJob", Method: "POST", Title: "创建采集任务", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorScrapeJob", Method: "POST", Title: "更新采集任务", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorScrapeJob/:id", Method: "DELETE", Title: "删除采集任务", Type: "1", Pid: 85},
		{Path: "/api/monitor/setScrapeJobStatus", Method: "POST", Title: "设置采集任务状态", Type: "1", Pid: 85},
		// 值班组
		{Path: "/api/monitor/getMonitorOndutyGroupList", Method: "GET", Title: "获取值班组列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorOndutyGroupOne/:id", Method: "GET", Title: "获取单个值班组", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorOndutyGroupFuturePlan/:id", Method: "GET", Title: "获取排班计划", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorOndutyGroup", Method: "POST", Title: "创建值班组", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorOndutyGroup", Method: "POST", Title: "更新值班组", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorOndutyGroup/:id", Method: "DELETE", Title: "删除值班组", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorOndutyChange", Method: "POST", Title: "创建交接班记录", Type: "1", Pid: 85},
		{Path: "/api/monitor/setOnDutyStatus", Method: "POST", Title: "设置值班状态", Type: "1", Pid: 85},
		// AlertManager 集群
		{Path: "/api/monitor/getMonitorAlertManagerPoolList", Method: "GET", Title: "获取Alert集群列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorAlertManagerYamlOne", Method: "GET", Title: "获取Alert主配置", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorAlertManagerPool", Method: "POST", Title: "创建Alert集群", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorAlertManagerPool", Method: "POST", Title: "更新Alert集群", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorAlertManagerPool/:id", Method: "DELETE", Title: "删除Alert集群", Type: "1", Pid: 85},
		// AlertManager 发送组
		{Path: "/api/monitor/getMonitorAlertManagerSendGroupList", Method: "GET", Title: "获取发送组列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorAlertManagerSendGroup", Method: "POST", Title: "创建发送组", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorAlertManagerSendGroup", Method: "POST", Title: "更新发送组", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorAlertManagerSendGroup/:id", Method: "DELETE", Title: "删除发送组", Type: "1", Pid: 85},
		{Path: "/api/monitor/setAlertManagerSendGroupStatus", Method: "POST", Title: "设置发送组状态", Type: "1", Pid: 85},
		// 告警规则
		{Path: "/api/monitor/getMonitorAlertRuleList", Method: "GET", Title: "获取告警规则列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/promqlExprCheck", Method: "GET", Title: "检查PromQL表达式", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorAlertRule", Method: "POST", Title: "创建告警规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorAlertRule", Method: "POST", Title: "更新告警规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/setAlertRuleStatus", Method: "POST", Title: "设置告警规则状态", Type: "1", Pid: 85},
		{Path: "/api/monitor/setAlertRuleStatusBatch", Method: "POST", Title: "批量设置告警规则状态", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorAlertRule/:id", Method: "DELETE", Title: "删除告警规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorAlertRuleBatch", Method: "DELETE", Title: "批量删除告警规则", Type: "1", Pid: 85},
		// 预聚合规则
		{Path: "/api/monitor/getMonitorRecordRuleList", Method: "GET", Title: "获取聚合规则列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/recordRulePromqlExprCheck", Method: "GET", Title: "检查聚合规则表达式", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorRecordRule", Method: "POST", Title: "创建聚合规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorRecordRule", Method: "POST", Title: "更新聚合规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/setRecordRuleStatus", Method: "POST", Title: "设置聚合规则状态", Type: "1", Pid: 85},
		{Path: "/api/monitor/setRecordRuleStatusBatch", Method: "POST", Title: "批量设置聚合规则状态", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorRecordRule/:id", Method: "DELETE", Title: "删除聚合规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorRecordRuleBatch", Method: "DELETE", Title: "批量删除聚合规则", Type: "1", Pid: 85},
		// 告警事件
		{Path: "/api/monitor/getMonitorAlertEventList", Method: "GET", Title: "获取告警事件列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/alertEventSilence/:id", Method: "POST", Title: "静默告警事件", Type: "1", Pid: 85},
		{Path: "/api/monitor/alertEventUnSilence/:id", Method: "POST", Title: "解除静默", Type: "1", Pid: 85},
		{Path: "/api/monitor/alertEventBatchSilence", Method: "POST", Title: "批量静默告警", Type: "1", Pid: 85},
		{Path: "/api/monitor/alertEventBatchUnSilence", Method: "POST", Title: "批量解除静默", Type: "1", Pid: 85},
		{Path: "/api/monitor/alertEventReLing/:id", Method: "POST", Title: "认领告警事件", Type: "1", Pid: 85},

		//{Path: "/api/system/menu", Method: "GET", Title: "系统管理-菜单相关", Type: "0"},
		//{Path: "/api/system/getMenuList", Method: "GET", Pid: 1, Title: "系统管理-根据用户获取菜单", Type: "1"},
		//{Path: "/api/system/getMenuListAll", Method: "GET", Pid: 1, Title: "系统管理-获取用户全量菜单", Type: "1"},
		//{Path: "/api/system/updateMenu", Method: "POST", Pid: 1, Title: "系统管理-修改菜单", Type: "1"},
		//{Path: "/api/system/createMenu", Method: "POST", Pid: 1, Title: "系统管理-创建菜单", Type: "1"},
		//{Path: "/api/system/deleteMenu/:id", Method: "DELETE", Pid: 1, Title: "系统管理-删除菜单", Type: "1"},
		//{Path: "/api/getUserInfo", Method: "GET", Pid: 1, Title: "获取用户信息", Type: "1"},
		//{Path: "/api/getPermCode", Method: "GET", Pid: 1, Title: "获得用户code", Type: "1"},
		//{Path: "/api/system/getAccountList", Method: "GET", Pid: 1, Title: "获取用户列表", Type: "1"},

		// ================== 全局权限 ==================
		//{Path: "/api/*", Method: "ALL", Title: "所有API权限", Type: "0"},
		//{Path: "/api/*", Method: "GET", Pid: 2, Title: "所有api GET权限", Type: "1"},
		//{Path: "/api/*", Method: "POST", Pid: 2, Title: "所有api POST权限", Type: "1"},
		//{Path: "/api/*", Method: "DELETE", Pid: 2, Title: "所有api DELETE权限", Type: "1"},
		//{Path: "/api/*", Method: "PATCH", Pid: 2, Title: "所有api PATCH权限", Type: "1"},
		//{Path: "/api/*", Method: "HEAD", Pid: 2, Title: "所有api HEAD权限", Type: "1"},
		//{Path: "/api/*", Method: "OPTIONS", Pid: 2, Title: "所有api OPTIONS权限", Type: "1"},
		//{Path: "/api/*", Method: "CONNECT", Pid: 2, Title: "所有api CONNECT权限", Type: "1"},
		//{Path: "/api/*", Method: "TRACE", Pid: 2, Title: "所有api TRACE权限", Type: "1"},
	}

	for _, menu := range menus {
		if err := Db.Create(&menu).Error; err != nil {
			fmt.Printf("创建menu错误:%v\n", err)
		}
	}

	adminUser := &User{
		Username:     "admin",
		Password:     common.BcryptHash("tingbao89.."),
		RealName:     "海绵宝宝",
		FeiShuUserId: "b75ag4g4",
		HomePath:     "/system/role",
		Enable:       1,
		Roles: []*Role{
			{RoleName: "超级管理员", RoleValue: "super", Menus: menus},
		},
	}

	testUser := &User{
		Username:     "test",
		Password:     common.BcryptHash("123456"),
		RealName:     "派大星",
		FeiShuUserId: "b75ag4g4",
		HomePath:     "/system/role",
		Enable:       1,
		Roles: []*Role{
			{RoleName: "前端管理员", RoleValue: "frontAdmin"},
		},
	}

	botUser := &User{
		Username:     sc.WorkOrderAutoActionC.ServiceAccount,
		Password:     common.BcryptHash("123456"),
		RealName:     "自动工单执行机器人",
		FeiShuUserId: "b75ag4g4",
		HomePath:     "/system/role",
		Enable:       1,
		Roles: []*Role{
			{RoleName: "集群超级管理员", RoleValue: "bot_super", Menus: menus},
		},
	}

	if err := Db.Create(adminUser).Error; err != nil {
		sc.Logger.Error("模拟管理员注册失败", zap.Error(err))
		return nil
	}
	_ = Db.Create(testUser)
	_ = Db.Create(botUser)

	//_ = Db.Create(apis)
	var currentParentID uint // 用于动态记录当前分类的真实自增 ID
	var activeApis []*Api    // 用于存放真正写入数据库成功的 API，供后续 Casbin 和 Role 使用

	for _, api := range apis {
		if api.Type == "0" {
			// 1. 遇到大模块分类，先把它洗干净（Pid置0），存入数据库获取真实 ID
			api.Pid = 0
			if err := Db.Create(api).Error; err != nil {
				sc.Logger.Error("注入API分类失败", zap.String("title", api.Title), zap.Error(err))
				continue
			}
			currentParentID = api.ID // GORM 会自动将数据库生成的自增 ID 回填到这个结构体中
			activeApis = append(activeApis, api)
		} else {
			// 2. 遇到子接口，动态绑定刚才拿到的最新分类 ID，彻底告别硬编码
			api.Pid = int(currentParentID)
			if err := Db.Create(api).Error; err != nil {
				// 如果某一条接口因为重复等原因报错，这里会清晰打印，且不会影响其他正确的接口
				sc.Logger.Error("注入子API失败", zap.String("path", api.Path), zap.Error(err))
			} else {
				activeApis = append(activeApis, api)
			}
		}
	}

	// === 后续联动修改：使用真正写入成功的 activeApis ===
	dbRole, _ := GetRoleByRoleValue("super")
	_ = dbRole.UpdateApis(activeApis) // 确保关联表数据的准确性

	for _, api := range activeApis {
		_, err := CasbinEnforcer.AddPolicy("super", api.Path, api.Method)
		if err != nil {
			sc.Logger.Error("给super角色绑定策略失败", zap.Error(err))
		}
	}

	users := []string{"蟹老板", "珊迪", "章鱼哥", "皮老板", "小蜗"}
	num := 5
	for i := 0; i < num; i++ {
		mockUser := &User{
			Username:     fmt.Sprintf("mock%d", i),
			Password:     common.BcryptHash("123456"),
			RealName:     users[i],
			FeiShuUserId: "b75ag4g4", //5egcb786
			HomePath:     "/system/role",
			Enable:       1,
		}
		_ = mockUser.CreateOne()
	}

	dbRole, _ = GetRoleByRoleValue("super")
	_ = dbRole.UpdateApis(apis)

	for _, api := range apis {
		_, err := CasbinEnforcer.AddPolicy("super", api.Path, api.Method)
		if err != nil {
			sc.Logger.Error("给super角色绑定策略失败", zap.Error(err))
		}
	}

	sc.Logger.Info("系统模块 Mock 数据注入成功")
	return adminUser
}

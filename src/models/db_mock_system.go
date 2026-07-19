package models

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"fmt"

	"go.uber.org/zap"
)

func mockSystemData(sc *config.ServerConfig) *User {
	menus := []*Menu{
		{Name: "Dashboard", Title: "工作台", Icon: "ant-design:dashboard-outlined", Type: "0", Show: "1", OrderNo: 1, Component: "LAYOUT", Path: "/dashboard", Redirect: "/dashboard/analysis"},
		{Name: "Analysis", Title: "概览分析", Icon: "ant-design:area-chart-outlined", Type: "1", Show: "1", OrderNo: 2, Component: "dashboard/analysis/index", Pid: 1, Path: "analysis"},

		{Name: "ServiceTree", Title: "资产管理", Icon: "ant-design:database-outlined", Type: "0", Show: "1", OrderNo: 10, Component: "LAYOUT", Path: "/serviceTree", Redirect: "/ServiceTree/streeAsync"},
		{Name: "ServiceTreeIndexAsync", Title: "CMDB服务树", Icon: "ant-design:node-index-outlined", Type: "1", Show: "1", OrderNo: 11, Component: "stree/stree/indexAsync", Pid: 3, Path: "streeAsync"},

		{Name: "WorkOrder", Title: "工单服务", Icon: "ant-design:reconciliation-outlined", Type: "0", Show: "1", OrderNo: 20, Component: "LAYOUT", Path: "/workOrder", Redirect: "/workOrder/process"},
		{Name: "ProcessManagement", Title: "审批流程管理", Icon: "ant-design:apartment-outlined", Type: "1", Show: "1", OrderNo: 21, Component: "workorder/process/index", Pid: 5, Path: "process"},
		{Name: "FormManagement", Title: "表单设计管理", Icon: "ant-design:form-outlined", Type: "1", Show: "1", OrderNo: 22, Component: "workorder/formDesign/index", Pid: 5, Path: "formDesign"},
		{Name: "WorkOrderTemplateManagement", Title: "工单模板管理", Icon: "ant-design:layout-outlined", Type: "1", Show: "1", OrderNo: 23, Component: "workorder/template/index", Pid: 5, Path: "template"},
		{Name: "WorkOrderTicket", Title: "工单申请", Icon: "ant-design:profile-outlined", Type: "1", Show: "1", OrderNo: 24, Component: "workorder/ticket/index", Pid: 5, Path: "ticket"},
		{Name: "WorkOrderCreate", Title: "工单填写", Icon: "ant-design:form-outlined", Type: "1", Show: "0", OrderNo: 25, Component: "workorder/ticket/create", Pid: 5, Path: "create"},
		{Name: "WorkOrderSearch", Title: "我的工单", Icon: "ant-design:profile-outlined", Type: "1", Show: "1", OrderNo: 26, Component: "workorder/ticket/search", Pid: 5, Path: "search"},

		{Name: "JobExec", Title: "任务执行", Icon: "ant-design:thunderbolt-outlined", Type: "0", Show: "1", OrderNo: 30, Component: "LAYOUT", Path: "/jobExec", Redirect: "/jobExec/script"},
		{Name: "JobExecTask", Title: "任务管理", Icon: "ant-design:schedule-outlined", Type: "1", Show: "1", OrderNo: 32, Component: "jobExec/task/index", Pid: 12, Path: "task"},
		{Name: "JobExecScript", Title: "脚本管理", Icon: "ant-design:code-outlined", Type: "1", Show: "1", OrderNo: 31, Component: "jobExec/script/index", Pid: 12, Path: "script"},

		{Name: "Monitor", Title: "监控中心", Icon: "ant-design:dashboard-outlined", Type: "0", Show: "1", OrderNo: 40, Component: "LAYOUT", Path: "/monitor", Redirect: "/monitor/prom_instance"},
		{Name: "MonitorPromPool", Title: "prom集群实例管理", Icon: "ant-design:database-outlined", Type: "1", Show: "1", OrderNo: 41, Component: "monitor/prom_instance/index", Pid: 15, Path: "prom_instance"},
		{Name: "MonitorPromScrapeJob", Title: "prom采集任务管理", Icon: "ant-design:api-outlined", Type: "1", Show: "1", OrderNo: 42, Component: "monitor/prom_scrape/index", Pid: 15, Path: "prom_scrape"},
		{Name: "MonitorPromAlertRule", Title: "prom告警规则管理", Icon: "ant-design:fund-view-outlined", Type: "1", Show: "1", OrderNo: 43, Component: "monitor/prom_alertrule/index", Pid: 15, Path: "prom_alertrule"},
		{Name: "MonitorPromRecordRule", Title: "prom聚合规则管理", Icon: "ant-design:fund-view-outlined", Type: "1", Show: "1", OrderNo: 44, Component: "monitor/prom_recordrule/index", Pid: 15, Path: "prom_recordrule"},
		{Name: "MonitorAlertPool", Title: "alert集群实例管理", Icon: "ant-design:alert-outlined", Type: "1", Show: "1", OrderNo: 45, Component: "monitor/alert_manager/index", Pid: 15, Path: "alert_manager"},
		{Name: "MonitorAlertSendGroup", Title: "alert发送组管理", Icon: "ant-design:dingding-outlined", Type: "1", Show: "1", OrderNo: 46, Component: "monitor/alert_sendgroup/index", Pid: 15, Path: "alert_sendgroup"},
		{Name: "MonitorAlertManagerEvent", Title: "alert告警事件管理", Icon: "ant-design:project-outlined", Type: "1", Show: "1", OrderNo: 47, Component: "monitor/alert_event/index", Pid: 15, Path: "alert_event"},
		{Name: "MonitorOnDutyGroup", Title: "值班组设置", Icon: "ant-design:ungroup-outlined", Type: "1", Show: "1", OrderNo: 48, Component: "monitor/onduty_group/index", Pid: 15, Path: "onduty_group"},
		{Name: "MonitorOnDutyGroupPlan", Title: "轮值排班表", Icon: "ant-design:calendar-outlined", Type: "1", Show: "1", OrderNo: 49, Component: "monitor/onduty_plan/index", Pid: 15, Path: "onduty_plan"},

		{Name: "K8sManagement", Title: "容器集群", Icon: "ant-design:kubernetes-outlined", Type: "0", Show: "1", OrderNo: 50, Component: "LAYOUT", Path: "/k8s", Redirect: "/k8s/node"},
		{Name: "K8sNode", Title: "节点管理", Icon: "ant-design:kubernetes-outlined", Type: "1", Show: "1", OrderNo: 51, Component: "k8s/node/index", Pid: 25, Path: "node"},

		{Name: "CiCdManagement", Title: "持续交付", Icon: "ant-design:calendar-outlined", Type: "0", Show: "1", OrderNo: 60, Component: "LAYOUT", Path: "/cicd", Redirect: "/cicd/workorder"},
		{Name: "CiCdWorkList", Title: "工单列表", Icon: "ant-design:database-outlined", Type: "1", Show: "1", OrderNo: 61, Component: "cicd/workorder/index", Pid: 27, Path: "workorder"},
		{Name: "CiCdDeployList", Title: "发布工单", Icon: "ant-design:rocket-outlined", Type: "1", Show: "1", OrderNo: 62, Component: "cicd/deploy/index", Pid: 27, Path: "deploy"},
		{Name: "CiCdServiceBaseline", Title: "服务基线", Icon: "ant-design:sliders-outlined", Type: "1", Show: "1", OrderNo: 63, Component: "cicd/baseline/index", Pid: 27, Path: "baseline"},
		{Name: "CiCdPipeline", Title: "流水线管理", Icon: "ant-design:branches-outlined", Type: "1", Show: "1", OrderNo: 64, Component: "cicd/pipeline/index", Pid: 27, Path: "pipeline"},
		{Name: "CiCdEnvManagement", Title: "环境配置", Icon: "ant-design:cloud-server-outlined", Type: "1", Show: "1", OrderNo: 65, Component: "cicd/environment/index", Pid: 27, Path: "environment"},

		{Name: "CodeManagement", Title: "代码管理", Icon: "ant-design:gitlab-filled", Type: "0", Show: "1", OrderNo: 70, Component: "LAYOUT", Path: "/code", Redirect: "/code/repo"},
		{Name: "CodeRepoManagement", Title: "仓库管理", Icon: "ant-design:database-outlined", Type: "1", Show: "1", OrderNo: 71, Component: "code/repo/index", Pid: 33, Path: "repo"},
		{Name: "CodeMergeManagement", Title: "合并请求", Icon: "ant-design:merge-cells-outlined", Type: "1", Show: "1", OrderNo: 72, Component: "code/merge/index", Pid: 33, Path: "merge"},
		{Name: "CodeServerManagement", Title: "实例管理", Icon: "ant-design:api-outlined", Type: "1", Show: "1", OrderNo: 73, Component: "code/server/index", Pid: 33, Path: "server"},

		{Name: "DORAManagement", Title: "效能度量", Icon: "ant-design:line-chart-outlined", Type: "0", Show: "1", OrderNo: 80, Component: "LAYOUT", Path: "/dora", Redirect: "/dora/dashboard"},
		{Name: "EffDashboard", Title: "效能看板", Icon: "ant-design:code-outlined", Type: "1", Show: "1", OrderNo: 81, Component: "dora/dashboard/index", Pid: 37, Path: "dashboard"},
		{Name: "DeployStat", Title: "部署统计", Icon: "ant-design:database-outlined", Type: "1", Show: "1", OrderNo: 82, Component: "dora/deployStat/index", Pid: 37, Path: "deployStat"},

		{Name: "System", Title: "系统管理", Icon: "ant-design:setting-outlined", Type: "0", Show: "1", OrderNo: 90, Component: "LAYOUT", Path: "/system", Redirect: "/system/changePassword"},
		{Name: "MenuManagement", Title: "菜单管理", Icon: "ant-design:menu-outlined", Type: "1", Show: "1", OrderNo: 91, Component: "system/menu/index", Pid: 40, Path: "menu"},
		{Name: "AccountManagement", Title: "用户管理", Icon: "ant-design:user-outlined", Type: "1", Show: "1", OrderNo: 92, Component: "system/account/index", Pid: 40, Path: "account"},
		{Name: "RoleManagement", Title: "角色管理", Icon: "ant-design:solution-outlined", Type: "1", Show: "1", OrderNo: 93, Component: "system/role/index", Pid: 40, Path: "role"},
		{Name: "ChangePassword", Title: "修改密码", Icon: "ant-design:key-outlined", Type: "1", Show: "1", OrderNo: 94, Component: "system/password/index", Pid: 40, Path: "changePassword"},
		{Name: "ApiManagement", Title: "接口授权", Icon: "ant-design:api-outlined", Type: "1", Show: "1", OrderNo: 95, Component: "system/api/index", Pid: 40, Path: "api"},
		{Name: "SystemSetting", Title: "系统设置", Icon: "ant-design:setting-twotone", Type: "1", Show: "1", OrderNo: 96, Component: "system/settings/index", Pid: 40, Path: "settings"},

		{Name: "PermissionManagement", Title: "权限管理", Icon: "ion:layers-outline", Type: "0", Show: "1", OrderNo: 100, Component: "LAYOUT", Path: "/permission", Redirect: "/permission/front"},
		{Name: "PermissionFront", Title: "前端权限管理", Icon: "ion:layers-outline", Type: "1", Show: "1", OrderNo: 101, Component: "/permission/front/index", Pid: 47, Path: "front"},
		{Name: "CodeUserManagement", Title: "Git用户管理", Icon: "ant-design:usergroup-add-outlined", Type: "1", Show: "1", OrderNo: 74, Component: "code/user/index", Pid: 33, Path: "user"},
		{Name: "CodeNamespaceManagement", Title: "命名空间管理", Icon: "ant-design:appstore-outlined", Type: "1", Show: "1", OrderNo: 75, Component: "code/namespace/index", Pid: 33, Path: "namespace"},
	}

	apis := []*Api{
		// ================== 系统管理模块 ==================
		{Path: "/api/system", Method: "GET", Title: "系统管理", Type: "0", Pid: 0},
		// 基础信息
		{Path: "/api/getUserInfo", Method: "GET", Title: "[用户模块]获取用户信息", Type: "1", Pid: 1},
		{Path: "/api/getPermCode", Method: "GET", Title: "[用户模块]获取用户权限码", Type: "1", Pid: 1},
		// 账号路由
		{Path: "/api/system/getAccountList", Method: "GET", Title: "[用户模块]获取账号列表", Type: "1", Pid: 1},
		{Path: "/api/system/createAccount", Method: "POST", Title: "[用户模块]创建账号", Type: "1", Pid: 1},
		{Path: "/api/system/updateAccount", Method: "POST", Title: "[用户模块]更新账号", Type: "1", Pid: 1},
		{Path: "/api/system/deleteAccount/:id", Method: "DELETE", Title: "[用户模块]删除账号", Type: "1", Pid: 1},
		{Path: "/api/system/setAccountStatus", Method: "POST", Title: "[用户模块]设置账号状态", Type: "1", Pid: 1},
		{Path: "/api/system/accountExist", Method: "POST", Title: "[用户模块]检查账号存在", Type: "1", Pid: 1},
		{Path: "/api/system/changePassword", Method: "POST", Title: "[用户模块]修改密码", Type: "1", Pid: 1},
		{Path: "/api/system/getAllUserAndRoles", Method: "GET", Title: "[用户模块]获取所有用户与角色", Type: "1", Pid: 1},
		// 菜单路由
		{Path: "/api/system/getMenuList", Method: "GET", Title: "[菜单模块]获取用户菜单", Type: "1", Pid: 1},
		{Path: "/api/system/getMenuListAll", Method: "GET", Title: "[菜单模块]获取全量菜单", Type: "1", Pid: 1},
		{Path: "/api/system/createMenu", Method: "POST", Title: "[菜单模块]创建菜单", Type: "1", Pid: 1},
		{Path: "/api/system/updateMenu", Method: "POST", Title: "[菜单模块]更新菜单", Type: "1", Pid: 1},
		{Path: "/api/system/deleteMenu/:id", Method: "DELETE", Title: "[菜单模块]删除菜单", Type: "1", Pid: 1},
		// 角色路由
		{Path: "/api/system/getRoleListAll", Method: "GET", Title: "[角色模块]获取所有角色", Type: "1", Pid: 1},
		{Path: "/api/system/createRole", Method: "POST", Title: "[角色模块]创建角色", Type: "1", Pid: 1},
		{Path: "/api/system/updateRole", Method: "POST", Title: "[角色模块]更新角色", Type: "1", Pid: 1},
		{Path: "/api/system/deleteRole/:id", Method: "DELETE", Title: "[角色模块]删除角色", Type: "1", Pid: 1},
		{Path: "/api/system/setRoleStatus", Method: "POST", Title: "[角色模块]设置角色状态", Type: "1", Pid: 1},
		// API路由
		{Path: "/api/system/getApiList", Method: "GET", Title: "[接口模块]获取API列表", Type: "1", Pid: 1},
		{Path: "/api/system/getApiListAll", Method: "GET", Title: "[接口模块]获取全量API", Type: "1", Pid: 1},
		{Path: "/api/system/createApi", Method: "POST", Title: "[接口模块]创建API", Type: "1", Pid: 1},
		{Path: "/api/system/updateApi", Method: "POST", Title: "[接口模块]更新API", Type: "1", Pid: 1},
		{Path: "/api/system/deleteApi/:id", Method: "DELETE", Title: "[接口模块]删除API", Type: "1", Pid: 1},

		//// ================== CMDB资产管理模块 ==================
		{Path: "/api/stree", Method: "GET", Title: "CMDB资产管理模块", Type: "0"},
		//// 服务树
		{Path: "/api/stree/getStreeNodeList", Method: "GET", Title: "[cmdb树节点]获取服务树节点列表", Type: "1", Pid: 27},
		{Path: "/api/stree/getStreeNodeSelect", Method: "GET", Title: "[cmdb树节点]获取服务树下拉选择", Type: "1", Pid: 27},
		{Path: "/api/stree/getTopStreeNodes", Method: "GET", Title: "[cmdb树节点]获取服务树顶层节点", Type: "1", Pid: 27},
		{Path: "/api/stree/getChildrenStreeNodes/:pid", Method: "GET", Title: "[cmdb树节点]获取服务树子节点", Type: "1", Pid: 27},
		{Path: "/api/stree/createStreeNode", Method: "POST", Title: "[cmdb树节点]创建服务树节点", Type: "1", Pid: 27},
		{Path: "/api/stree/updateStreeNode", Method: "POST", Title: "[cmdb树节点]更新服务树节点", Type: "1", Pid: 27},
		{Path: "/api/stree/deleteStreeNode/:id", Method: "DELETE", Title: "[cmdb树节点]删除服务树节点", Type: "1", Pid: 27},
		{Path: "/api/stree/getLeafStreeNodes", Method: "GET", Title: "[cmdb树节点]获取服务树叶子节点", Type: "1", Pid: 27},
		{Path: "/api/stree/fetchResourceByNode", Method: "GET", Title: "[cmdb树节点]根据节点获取资源", Type: "1", Pid: 27},
		// ECS
		{Path: "/api/stree/bindEcsToStreeNode", Method: "POST", Title: "[cmdb云主机]节点绑定ECS", Type: "1", Pid: 27},
		{Path: "/api/stree/unBindEcsToStreeNode", Method: "POST", Title: "[cmdb云主机]节点解绑ECS", Type: "1", Pid: 27},
		{Path: "/api/stree/getStreeNodeEcsList/:id", Method: "GET", Title: "[cmdb云主机]获取节点已绑定ECS列表", Type: "1", Pid: 27},
		{Path: "/api/stree/getResourceEcsUnbindList", Method: "GET", Title: "[cmdb云主机]获取节点未绑定ECS列表", Type: "1", Pid: 27},
		{Path: "/api/stree/getResourceEcsList", Method: "GET", Title: "[cmdb云主机]获取节点ECS列表", Type: "1", Pid: 27},
		// ELB
		{Path: "/api/stree/getResourceElbUnbindList", Method: "GET", Title: "[cmdb负载均衡]获取未绑定ELB", Type: "1", Pid: 27},
		{Path: "/api/stree/bindElbToStreeNode", Method: "POST", Title: "[cmdb负载均衡]节点绑定ELB", Type: "1", Pid: 27},
		{Path: "/api/stree/unBindElbToStreeNode", Method: "POST", Title: "[cmdb负载均衡]节点解绑ELB", Type: "1", Pid: 27},
		// RDS
		{Path: "/api/stree/getResourceRdsUnbindList", Method: "GET", Title: "[cmdb数据库]获取未绑定RDS", Type: "1", Pid: 27},
		{Path: "/api/stree/bindRdsToStreeNode", Method: "POST", Title: "[cmdb数据库]节点绑定RDS", Type: "1", Pid: 27},
		{Path: "/api/stree/unBindRdsToStreeNode", Method: "POST", Title: "[cmdb数据库]节点解绑RDS", Type: "1", Pid: 27},

		// ================== 工单服务模块 ==================
		{Path: "/api/workorder", Method: "GET", Title: "工单服务模块", Type: "0"},
		// 流程
		{Path: "/api/workorder/getProcessList", Method: "GET", Title: "[工单模块]获取流程列表", Type: "1", Pid: 48},
		{Path: "/api/workorder/createProcess", Method: "POST", Title: "[工单模块]创建流程", Type: "1", Pid: 48},
		{Path: "/api/workorder/updateProcess", Method: "POST", Title: "[工单模块]更新流程", Type: "1", Pid: 48},
		{Path: "/api/workorder/deleteProcess/:id", Method: "DELETE", Title: "[工单模块]删除流程", Type: "1", Pid: 48},
		// 表单设计
		{Path: "/api/workorder/getFormDesignList", Method: "GET", Title: "[工单模块]获取表单设计列表", Type: "1", Pid: 48},
		{Path: "/api/workorder/createFormDesign", Method: "POST", Title: "[工单模块]创建表单设计", Type: "1", Pid: 48},
		{Path: "/api/workorder/updateFormDesign", Method: "POST", Title: "[工单模块]更新表单设计", Type: "1", Pid: 48},
		{Path: "/api/workorder/deleteFormDesign/:id", Method: "DELETE", Title: "[工单模块]删除表单设计", Type: "1", Pid: 48},
		// 工单模板
		{Path: "/api/workorder/getWorkOrderTemplateList", Method: "GET", Title: "[工单模块]获取工单模板列表", Type: "1", Pid: 48},
		{Path: "/api/workorder/createWorkOrderTemplate", Method: "POST", Title: "[工单模块]创建工单模板", Type: "1", Pid: 48},
		{Path: "/api/workorder/updateWorkOrderTemplate", Method: "POST", Title: "[工单模块]更新工单模板", Type: "1", Pid: 48},
		{Path: "/api/workorder/deleteWorkOrderTemplate/:id", Method: "DELETE", Title: "[工单模块]删除工单模板", Type: "1", Pid: 48},
		{Path: "/api/workorder/getWorkOrderTemplateDetail/:id", Method: "GET", Title: "[工单模块]获取工单模板详情", Type: "1", Pid: 48},

		// 工单实例
		{Path: "/api/workorder/getWorkOrderInstanceList", Method: "GET", Title: "[工单模块]获取工单实例列表", Type: "1", Pid: 48},
		{Path: "/api/workorder/createWorkOrderInstance", Method: "POST", Title: "[工单模块]创建工单实例", Type: "1", Pid: 48},
		{Path: "/api/workorder/updateWorkOrderInstance", Method: "POST", Title: "[工单模块]更新工单实例", Type: "1", Pid: 48},
		{Path: "/api/workorder/deleteWorkOrderInstance/:id", Method: "DELETE", Title: "[工单模块]删除工单实例", Type: "1", Pid: 48},
		{Path: "/api/workorder/approvalWorkOrderInstance/:id", Method: "POST", Title: "[工单模块]审批工单", Type: "1", Pid: 48},
		{Path: "/api/workorder/actionWorkOrderInstance/:id", Method: "POST", Title: "[工单模块]执行工单动作", Type: "1", Pid: 48},
		{Path: "/api/workorder/getWorkOrderInstanceDetail/:id", Method: "GET", Title: "[工单模块]获取工单实例详情", Type: "1", Pid: 48},
		{Path: "/api/workorder/commentWorkOrderInstance/:id", Method: "POST", Title: "[工单模块]评论工单", Type: "1", Pid: 48},

		// ================== 任务执行中心模块 ==================
		{Path: "/api/jobexec", Method: "GET", Title: "任务执行中心模块", Type: "0"},
		// 脚本管理
		{Path: "/api/jobexec/getJobExecScriptList", Method: "GET", Title: "[任务执行中心]获取脚本列表", Type: "1", Pid: 70},
		{Path: "/api/jobexec/createJobExecScript", Method: "POST", Title: "[任务执行中心]创建脚本", Type: "1", Pid: 70},
		{Path: "/api/jobexec/updateJobExecScript", Method: "POST", Title: "[任务执行中心]更新脚本", Type: "1", Pid: 70},
		{Path: "/api/jobexec/deleteJobExecScript/:id", Method: "DELETE", Title: "[任务执行中心]删除脚本", Type: "1", Pid: 70},
		{Path: "/api/jobexec/getJobExecScriptSelect", Method: "GET", Title: "[任务执行中心]获取脚本下拉选择", Type: "1", Pid: 70},
		{Path: "/api/jobexec/getJobExecScriptOne/:id", Method: "GET", Title: "[任务执行中心]获取单个脚本", Type: "1", Pid: 70},
		{Path: "/api/jobexec/getJobExecScriptDetail/:id", Method: "GET", Title: "[任务执行中心]获取脚本详情", Type: "1", Pid: 70},

		// 任务管理
		{Path: "/api/jobexec/getJobExecTaskList", Method: "GET", Title: "[任务执行中心]获取任务列表", Type: "1", Pid: 70},
		{Path: "/api/jobexec/getJobExecTaskOne/:id", Method: "GET", Title: "[任务执行中心]获取单个任务", Type: "1", Pid: 70},
		{Path: "/api/jobexec/createJobExecTask", Method: "POST", Title: "[任务执行中心]创建任务", Type: "1", Pid: 70},
		{Path: "/api/jobexec/updateJobExecTask", Method: "POST", Title: "[任务执行中心]更新任务", Type: "1", Pid: 70},
		{Path: "/api/jobexec/deleteJobExecTask/:id", Method: "DELETE", Title: "[任务执行中心]删除任务", Type: "1", Pid: 70},
		{Path: "/api/jobexec/actionJobExecTaskOne/:id", Method: "POST", Title: "[任务执行中心]执行任务", Type: "1", Pid: 70},
		{Path: "/api/jobexec/getJobExecResultByJobId", Method: "GET", Title: "[任务执行中心]获取任务结果", Type: "1", Pid: 70},

		// ================== 监控中心模块 ==================
		{Path: "/api/monitor", Method: "GET", Title: "监控中心模块", Type: "0"},
		// Prometheus 集群
		{Path: "/api/monitor/getMonitorPromScrapePoolList", Method: "GET", Title: "[prometheus]获取Prom集群列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorPromScrapePool", Method: "POST", Title: "[prometheus]创建Prom集群", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorPromScrapePool", Method: "POST", Title: "[prometheus]更新Prom集群", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorPromScrapePool/:id", Method: "DELETE", Title: "[prometheus]删除Prom集群", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorPrometheusYamlOne", Method: "GET", Title: "[prometheus]获取Prom主配置", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorPrometheusAlertRuleYamlOne", Method: "GET", Title: "[prometheus]获取告警规则配置", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorPrometheusRecordRuleYamlOne", Method: "GET", Title: "[prometheus]获取预聚合规则配置", Type: "1", Pid: 85},
		// Prometheus 采集任务
		{Path: "/api/monitor/getMonitorPromScrapeJobList", Method: "GET", Title: "[prometheus]获取采集任务列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorPromScrapeJobOne", Method: "GET", Title: "[prometheus]获取单个采集任务", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorPromScrapeJob", Method: "POST", Title: "[prometheus]创建采集任务", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorPromScrapeJob", Method: "POST", Title: "[prometheus]更新采集任务", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorPromScrapeJob/:id", Method: "DELETE", Title: "[prometheus]删除采集任务", Type: "1", Pid: 85},
		{Path: "/api/monitor/setMonitorPromScrapeJobStatus", Method: "POST", Title: "[prometheus]设置采集任务状态", Type: "1", Pid: 85},
		// 告警规则
		{Path: "List/api/monitor/getMonitorPromAlertRuleList", Method: "GET", Title: "[prometheus]获取告警规则列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorPromAlertRule", Method: "POST", Title: "[prometheus]创建告警规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorPromAlertRule", Method: "POST", Title: "[prometheus]更新告警规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorPromAlertRule/:id", Method: "DELETE", Title: "[prometheus]删除告警规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorPromAlertRuleBatch", Method: "DELETE", Title: "[prometheus]批量删除告警规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/setMonitorPromAlertRuleStatus", Method: "POST", Title: "[prometheus]设置告警规则状态", Type: "1", Pid: 85},
		{Path: "/api/monitor/setMonitorPromAlertRuleStatusBatch", Method: "POST", Title: "[prometheus]批量设置告警规则状态", Type: "1", Pid: 85},
		{Path: "/api/monitor/promqlExprCheck", Method: "GET", Title: "[prometheus]检查PromQL表达式", Type: "1", Pid: 85},
		// 预聚合规则
		{Path: "/api/monitor/getMonitorPromRecordRuleList", Method: "GET", Title: "[prometheus]获取聚合规则列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorPromRecordRule", Method: "POST", Title: "[prometheus]创建聚合规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorPromRecordRule", Method: "POST", Title: "[prometheus]更新聚合规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorPromRecordRule/:id", Method: "DELETE", Title: "[prometheus]删除聚合规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorPromRecordRuleBatch", Method: "DELETE", Title: "[prometheus]批量删除聚合规则", Type: "1", Pid: 85},
		{Path: "/api/monitor/setMonitorPromRecordRuleStatus", Method: "POST", Title: "[prometheus]设置聚合规则状态", Type: "1", Pid: 85},
		{Path: "/api/monitor/setMonitorPromRecordRuleStatusBatch", Method: "POST", Title: "[prometheus]批量设置聚合规则状态", Type: "1", Pid: 85},
		{Path: "/api/monitor/recordRulePromqlExprCheck", Method: "GET", Title: "[prometheus]检查聚合规则表达式", Type: "1", Pid: 85},

		// AlertManager 集群
		{Path: "/api/monitor/getMonitorAlertManagerPoolList", Method: "GET", Title: "[alertmanager]获取Alert集群列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorAlertManagerPool", Method: "POST", Title: "[alertmanager]创建Alert集群", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorAlertManagerPool", Method: "POST", Title: "[alertmanager]更新Alert集群", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorAlertManagerPool/:id", Method: "DELETE", Title: "[alertmanager]删除Alert集群", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorAlertManagerYamlOne", Method: "GET", Title: "[alertmanager]获取Alert主配置", Type: "1", Pid: 85},
		// AlertManager 发送组
		{Path: "/api/monitor/getMonitorAlertManagerSendGroupList", Method: "GET", Title: "[告警发送组]获取发送组列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorAlertManagerSendGroup", Method: "POST", Title: "[告警发送组]创建发送组", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorAlertManagerSendGroup", Method: "POST", Title: "[告警发送组]更新发送组", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorAlertManagerSendGroup/:id", Method: "DELETE", Title: "[告警发送组]删除发送组", Type: "1", Pid: 85},
		{Path: "/api/monitor/setAlertManagerSendGroupStatus", Method: "POST", Title: "[告警发送组]设置发送组状态", Type: "1", Pid: 85},

		// 告警事件
		{Path: "/api/monitor/getMonitorAlertManagerEventList", Method: "GET", Title: "[告警事件]获取告警事件列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/alertManagerEventSilence/:id", Method: "POST", Title: "[告警事件]静默告警事件", Type: "1", Pid: 85},
		{Path: "/api/monitor/alertManagerEventUnSilence/:id", Method: "POST", Title: "[告警事件]解除静默", Type: "1", Pid: 85},
		{Path: "/api/monitor/alertManagerEventBatchSilence", Method: "POST", Title: "[告警事件]批量静默告警", Type: "1", Pid: 85},
		{Path: "/api/monitor/alertManagerEventBatchUnSilence", Method: "POST", Title: "[告警事件]批量解除静默", Type: "1", Pid: 85},
		{Path: "/api/monitor/alertManagerEventReLing/:id", Method: "POST", Title: "[告警事件]认领告警事件", Type: "1", Pid: 85},

		// 值班组
		{Path: "/api/monitor/getMonitorOndutyGroupList", Method: "GET", Title: "[值班组]获取值班组列表", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorOndutyGroupOne/:id", Method: "GET", Title: "[值班组]获取单个值班组", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorOndutyGroup", Method: "POST", Title: "[值班组]创建值班组", Type: "1", Pid: 85},
		{Path: "/api/monitor/updateMonitorOndutyGroup", Method: "POST", Title: "[值班组]更新值班组", Type: "1", Pid: 85},
		{Path: "/api/monitor/deleteMonitorOndutyGroup/:id", Method: "DELETE", Title: "[值班组]删除值班组", Type: "1", Pid: 85},
		{Path: "/api/monitor/setMonitorOndutyStatus", Method: "POST", Title: "[排班表]设置值班状态", Type: "1", Pid: 85},
		{Path: "/api/monitor/getMonitorOndutyGroupFuturePlan/:id", Method: "GET", Title: "[排班表]获取排班计划", Type: "1", Pid: 85},
		{Path: "/api/monitor/createMonitorOndutyChange", Method: "POST", Title: "[排班表]创建替班记录", Type: "1", Pid: 85},

		// ================== 全局权限 ==================
		{Path: "/api", Method: "GET", Title: "所有api权限", Type: "0"},
		{Path: "/api/*", Method: "ALL", Title: "所有api ALL权限", Type: "1", Pid: 139},
		{Path: "/api/*", Method: "GET", Title: "所有api GET权限", Type: "1", Pid: 139},
		{Path: "/api/*", Method: "POST", Title: "所有api POST权限", Type: "1", Pid: 139},
		{Path: "/api/*", Method: "DELETE", Title: "所有api DELETE权限", Type: "1", Pid: 139},
		{Path: "/api/*", Method: "PATCH", Title: "所有api PATCH权限", Type: "1", Pid: 139},
		{Path: "/api/*", Method: "HEAD", Title: "所有api HEAD权限", Type: "1", Pid: 139},
		{Path: "/api/*", Method: "OPTIONS", Title: "所有api OPTIONS权限", Type: "1", Pid: 139},
		{Path: "/api/*", Method: "CONNECT", Title: "所有api CONNECT权限", Type: "1", Pid: 139},
		{Path: "/api/*", Method: "TRACE", Title: "所有api TRACE权限", Type: "1", Pid: 139},

		//{Path: "/api/system/menu", Method: "GET", Title: "系统管理-菜单相关", Type: "0"},
		//{Path: "/api/system/getMenuList", Method: "GET", Pid: 1, Title: "系统管理-根据用户获取菜单", Type: "1"},
		//{Path: "/api/system/getMenuListAll", Method: "GET", Pid: 1, Title: "系统管理-获取用户全量菜单", Type: "1"},
		//{Path: "/api/system/updateMenu", Method: "POST", Pid: 1, Title: "系统管理-修改菜单", Type: "1"},
		//{Path: "/api/system/createMenu", Method: "POST", Pid: 1, Title: "系统管理-创建菜单", Type: "1"},
		//{Path: "/api/system/deleteMenu", Method: "DELETE", Pid: 1, Title: "系统管理-删除菜单", Type: "1"},
		//{Path: "/api/getUserInfo", Method: "GET", Pid: 1, Title: "获取用户信息", Type: "1"},
		//{Path: "/api/getPermCode", Method: "GET", Pid: 1, Title: "获得用户code", Type: "1"},
		//{Path: "/api/system/getAccountList", Method: "GET", Pid: 1, Title: "获取用户列表", Type: "1"},

		{Path: "/api/code", Method: "GET", Title: "代码管理", Type: "0"},
		{Path: "/api/system/setting/get", Method: "GET", Pid: 1, Title: "系统管理-全局设置", Type: "1"},
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
		HomePath:     "/dashboard/analysis",
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
		HomePath:     "/dashboard/analysis",
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
		HomePath:     "/dashboard/analysis",
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

	_ = Db.Create(apis)

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

	dbRole, _ := GetRoleByRoleValue("super")
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

package models

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"fmt"

	"go.uber.org/zap"
)

func mockSystemData(sc *config.ServerConfig) *SystemUser {
	type MenuModule struct {
		Parent   *SystemMenu
		Children []*SystemMenu
	}

	modules := []MenuModule{
		// 工作台
		{
			Parent: &SystemMenu{Name: "Dashboard", Title: "工作台", Icon: "ant-design:dashboard-outlined", Type: "0", Show: "1", OrderNo: 1, Component: "LAYOUT", Path: "/dashboard", Redirect: "/dashboard/analysis"},
			Children: []*SystemMenu{
				{Name: "Analysis", Title: "概览分析", Icon: "ant-design:area-chart-outlined", Type: "1", Show: "1", OrderNo: 2, Component: "dashboard/analysis/index", Path: "analysis"},
			},
		},

		// 服务树
		{
			Parent: &SystemMenu{Name: "ServiceTree", Title: "资产管理", Icon: "ant-design:database-outlined", Type: "0", Show: "1", OrderNo: 10, Component: "LAYOUT", Path: "/serviceTree", Redirect: "/ServiceTree/streeAsync"},
			Children: []*SystemMenu{
				{Name: "ServiceTreeIndexAsync", Title: "CMDB服务树", Icon: "ant-design:node-index-outlined", Type: "1", Show: "1", OrderNo: 11, Component: "stree/stree/indexAsync", Path: "streeAsync"},
			},
		},

		// IT工单
		{
			Parent: &SystemMenu{Name: "WorkOrder", Title: "工单服务", Icon: "ant-design:reconciliation-outlined", Type: "0", Show: "1", OrderNo: 20, Component: "LAYOUT", Path: "/workOrder", Redirect: "/workOrder/process"},
			Children: []*SystemMenu{
				{Name: "ProcessManagement", Title: "审批流程管理", Icon: "ant-design:apartment-outlined", Type: "1", Show: "1", OrderNo: 21, Component: "workorder/process/index", Path: "process"},
				{Name: "FormManagement", Title: "表单设计管理", Icon: "ant-design:form-outlined", Type: "1", Show: "1", OrderNo: 22, Component: "workorder/formDesign/index", Path: "formDesign"},
				{Name: "WorkOrderTemplateManagement", Title: "工单模板管理", Icon: "ant-design:layout-outlined", Type: "1", Show: "1", OrderNo: 23, Component: "workorder/template/index", Path: "template"},
				{Name: "WorkOrderTicket", Title: "工单申请", Icon: "ant-design:profile-outlined", Type: "1", Show: "1", OrderNo: 24, Component: "workorder/ticket/index", Path: "ticket"},
				{Name: "WorkOrderCreate", Title: "工单填写", Icon: "ant-design:form-outlined", Type: "1", Show: "0", OrderNo: 25, Component: "workorder/ticket/create", Path: "create"},
				{Name: "WorkOrderSearch", Title: "我的工单", Icon: "ant-design:profile-outlined", Type: "1", Show: "1", OrderNo: 26, Component: "workorder/ticket/search", Path: "search"},
				{Name: "WorkOrderDetail", Title: "工单详情", Icon: "ant-design:file-text-outlined", Type: "1", Show: "0", OrderNo: 27, Component: "workorder/detail/index", Path: "detail"},
			},
		},

		// 任务执行
		{
			Parent: &SystemMenu{Name: "JobExec", Title: "任务执行", Icon: "ant-design:thunderbolt-outlined", Type: "0", Show: "1", OrderNo: 30, Component: "LAYOUT", Path: "/jobExec", Redirect: "/jobExec/script"},
			Children: []*SystemMenu{
				{Name: "JobExecTask", Title: "任务管理", Icon: "ant-design:schedule-outlined", Type: "1", Show: "1", OrderNo: 32, Component: "jobExec/task/index", Path: "task"},
				{Name: "JobExecScript", Title: "脚本管理", Icon: "ant-design:code-outlined", Type: "1", Show: "1", OrderNo: 31, Component: "jobExec/script/index", Path: "script"},
			},
		},

		// 监控中心
		{
			Parent: &SystemMenu{Name: "Monitor", Title: "监控中心", Icon: "ant-design:dashboard-outlined", Type: "0", Show: "1", OrderNo: 40, Component: "LAYOUT", Path: "/monitor", Redirect: "/monitor/prom_instance"},
			Children: []*SystemMenu{
				{Name: "MonitorPromPool", Title: "prom集群实例管理", Icon: "ant-design:database-outlined", Type: "1", Show: "1", OrderNo: 41, Component: "monitor/prom_instance/index", Path: "prom_instance"},
				{Name: "MonitorPromScrapeJob", Title: "prom采集任务管理", Icon: "ant-design:api-outlined", Type: "1", Show: "1", OrderNo: 42, Component: "monitor/prom_scrape/index", Path: "prom_scrape"},
				{Name: "MonitorPromAlertRule", Title: "prom告警规则管理", Icon: "ant-design:fund-view-outlined", Type: "1", Show: "1", OrderNo: 43, Component: "monitor/prom_alertrule/index", Path: "prom_alertrule"},
				{Name: "MonitorPromRecordRule", Title: "prom聚合规则管理", Icon: "ant-design:fund-view-outlined", Type: "1", Show: "1", OrderNo: 44, Component: "monitor/prom_recordrule/index", Path: "prom_recordrule"},
				{Name: "MonitorAlertPool", Title: "alert集群实例管理", Icon: "ant-design:alert-outlined", Type: "1", Show: "1", OrderNo: 45, Component: "monitor/alert_manager/index", Path: "alert_manager"},
				{Name: "MonitorAlertSendGroup", Title: "alert发送组管理", Icon: "ant-design:dingding-outlined", Type: "1", Show: "1", OrderNo: 46, Component: "monitor/alert_sendgroup/index", Path: "alert_sendgroup"},
				{Name: "MonitorAlertManagerEvent", Title: "alert告警事件管理", Icon: "ant-design:project-outlined", Type: "1", Show: "1", OrderNo: 47, Component: "monitor/alert_event/index", Path: "alert_event"},
				{Name: "MonitorOnDutyGroup", Title: "值班组设置", Icon: "ant-design:ungroup-outlined", Type: "1", Show: "1", OrderNo: 48, Component: "monitor/onduty_group/index", Path: "onduty_group"},
				{Name: "MonitorOnDutyGroupPlan", Title: "轮值排班表", Icon: "ant-design:calendar-outlined", Type: "1", Show: "1", OrderNo: 49, Component: "monitor/onduty_plan/index", Path: "onduty_plan"},
			},
		},

		// 容器集群
		{
			Parent: &SystemMenu{Name: "K8sManagement", Title: "容器集群", Icon: "ant-design:kubernetes-outlined", Type: "0", Show: "1", OrderNo: 50, Component: "LAYOUT", Path: "/k8s", Redirect: "/k8s/cluster"},
			Children: []*SystemMenu{
				{Name: "K8sClusterManagement", Title: "集群管理", Icon: "ant-design:cloud-outlined", Type: "1", Show: "1", OrderNo: 51, Component: "k8s/cluster/index", Path: "cluster"},
				{Name: "K8sNodeManagement", Title: "集群节点", Icon: "ant-design:desktop-outlined", Type: "1", Show: "1", OrderNo: 52, Component: "k8s/node/index", Path: "node"},
				{Name: "K8sPodManagement", Title: "Pod 管理", Icon: "ant-design:appstore-outlined", Type: "1", Show: "1", OrderNo: 53, Component: "k8s/pod/index", Path: "pod"},
				{Name: "K8sWorkloadManagement", Title: "工作负载", Icon: "ant-design:deployment-unit-outlined", Type: "1", Show: "1", OrderNo: 54, Component: "k8s/workload/index", Path: "workload"},
				{Name: "K8sConfigManagement", Title: "配置与密钥", Icon: "ant-design:key-outlined", Type: "1", Show: "1", OrderNo: 55, Component: "k8s/config/index", Path: "config"},
				{Name: "K8sNetworkManagement", Title: "服务与路由", Icon: "ant-design:gateway-outlined", Type: "1", Show: "1", OrderNo: 56, Component: "k8s/network/index", Path: "network"},
				{Name: "K8sYamlTemplateManagement", Title: "YAML 模板", Icon: "ant-design:file-text-outlined", Type: "1", Show: "1", OrderNo: 57, Component: "k8s/yamlTemplate/index", Path: "yamlTemplate"},
				{Name: "K8sYamlTaskManagement", Title: "YAML 发布任务", Icon: "ant-design:rocket-outlined", Type: "1", Show: "1", OrderNo: 58, Component: "k8s/yamlTask/index", Path: "yamlTask"},
				{Name: "K8sProjectAppInstanceManagement", Title: "k8s项目管理", Icon: "ant-design:project-outlined", Type: "1", Show: "1", OrderNo: 59, Component: "k8s/instance/index", Path: "instance"},
			},
		},

		// cicd
		{
			Parent: &SystemMenu{Name: "CiCdManagement", Title: "持续交付", Icon: "ant-design:rocket-filled", Type: "0", Show: "1", OrderNo: 60, Component: "LAYOUT", Path: "/cicd", Redirect: "/cicd/baseline"},
			Children: []*SystemMenu{
				{Name: "JenkinsInstanceManagement", Title: "实例管理", Icon: "ant-design:cloud-server-outlined", Type: "1", Show: "1", OrderNo: 61, Component: "cicd/instance/index", Path: "instance"},
				{Name: "CiCdWorkList", Title: "工单列表", Icon: "ant-design:audit-outlined", Type: "1", Show: "1", OrderNo: 62, Component: "cicd/workorder/index", Path: "workorder"},
				{Name: "CiCdDeployList", Title: "发布工单", Icon: "ant-design:send-outlined", Type: "1", Show: "1", OrderNo: 63, Component: "cicd/deploy/index", Path: "deploy"},
				{Name: "CiCdServiceBaseline", Title: "服务基线", Icon: "ant-design:sliders-outlined", Type: "1", Show: "1", OrderNo: 64, Component: "cicd/baseline/index", Path: "baseline"},
				{Name: "CiCdPipeline", Title: "流水线管理", Icon: "ant-design:partition-outlined", Type: "1", Show: "1", OrderNo: 65, Component: "cicd/pipeline/index", Path: "pipeline"},
				//{Name: "CiCdEnvManagement", Title: "环境配置", Icon: "ant-design:cloud-server-outlined", Type: "1", Show: "1", OrderNo: 65, Component: "cicd/environment/index", Path: "environment"},
			},
		},

		// 代码管理
		{
			Parent: &SystemMenu{Name: "CodeManagement", Title: "代码管理", Icon: "ant-design:gitlab-filled", Type: "0", Show: "1", OrderNo: 70, Component: "LAYOUT", Path: "/code", Redirect: "/code/repo"},
			Children: []*SystemMenu{
				{Name: "CodeRepoManagement", Title: "仓库管理", Icon: "ant-design:folder-open-outlined", Type: "1", Show: "1", OrderNo: 71, Component: "code/repo/index", Path: "repo"},
				{Name: "CodeMergeManagement", Title: "合并请求", Icon: "ant-design:merge-cells-outlined", Type: "1", Show: "1", OrderNo: 72, Component: "code/merge/index", Path: "merge"},
				{Name: "CodeServerManagement", Title: "实例管理", Icon: "ant-design:code-outlined", Type: "1", Show: "1", OrderNo: 73, Component: "code/server/index", Path: "server"},
				{Name: "CodeUserManagement", Title: "Git用户管理", Icon: "ant-design:usergroup-add-outlined", Type: "1", Show: "1", OrderNo: 74, Component: "code/user/index", Path: "user"},
				{Name: "CodeNamespaceManagement", Title: "命名空间管理", Icon: "ant-design:tags-outlined", Type: "1", Show: "1", OrderNo: 75, Component: "code/namespace/index", Path: "namespace"},
			},
		},

		// 效能度量
		{
			Parent: &SystemMenu{Name: "DORAManagement", Title: "效能度量", Icon: "ant-design:line-chart-outlined", Type: "0", Show: "1", OrderNo: 80, Component: "LAYOUT", Path: "/dora", Redirect: "/dora/dashboard"},
			Children: []*SystemMenu{
				{Name: "EffDashboard", Title: "效能看板", Icon: "ant-design:pie-chart-outlined", Type: "1", Show: "1", OrderNo: 81, Component: "dora/dashboard/index", Path: "dashboard"},
				{Name: "DeployStat", Title: "部署统计", Icon: "ant-design:dot-chart-outlined", Type: "1", Show: "1", OrderNo: 82, Component: "dora/deployStat/index", Path: "deployStat"},
			},
		},
		// 制品与配置库
		{
			Parent: &SystemMenu{Name: "ArtifactoryManagement", Title: "制品与配置库", Icon: "ant-design:folder-open-outlined", Type: "0", Show: "1", OrderNo: 85, Component: "LAYOUT", Path: "/artifactory", Redirect: "/artifactory/manage"},
			Children: []*SystemMenu{
				{Name: "ArtifactoryManageIndex", Title: "制品与配置在线管理", Icon: "ant-design:code-outlined", Type: "1", Show: "1", OrderNo: 86, Component: "artifactory/index", Path: "manage"},
			},
		},
		// 系统管理
		{
			Parent: &SystemMenu{Name: "System", Title: "系统管理", Icon: "ant-design:setting-outlined", Type: "0", Show: "1", OrderNo: 90, Component: "LAYOUT", Path: "/system", Redirect: "/system/changePassword"},
			Children: []*SystemMenu{
				{Name: "MenuManagement", Title: "菜单管理", Icon: "ant-design:menu-outlined", Type: "1", Show: "1", OrderNo: 91, Component: "system/menu/index", Path: "menu"},
				{Name: "AccountManagement", Title: "用户管理", Icon: "ant-design:user-outlined", Type: "1", Show: "1", OrderNo: 92, Component: "system/account/index", Path: "account"},
				{Name: "RoleManagement", Title: "角色管理", Icon: "ant-design:solution-outlined", Type: "1", Show: "1", OrderNo: 93, Component: "system/role/index", Path: "role"},
				{Name: "ChangePassword", Title: "修改密码", Icon: "ant-design:key-outlined", Type: "1", Show: "1", OrderNo: 94, Component: "system/password/index", Path: "changePassword"},
				{Name: "ApiManagement", Title: "接口授权", Icon: "ant-design:safety-certificate-outlined", Type: "1", Show: "1", OrderNo: 95, Component: "system/api/index", Path: "api"},
				{Name: "SystemSetting", Title: "系统设置", Icon: "ant-design:control-outlined", Type: "1", Show: "1", OrderNo: 96, Component: "system/settings/index", Path: "settings"},
				{Name: "AuditManagement", Title: "操作审计", Icon: "ant-design:security-scan-outlined", Type: "1", Show: "1", OrderNo: 97, Component: "system/audit/index", Path: "audit"},
			},
		},
	}

	var menus []*SystemMenu
	for _, mod := range modules {
		if err := Db.Create(mod.Parent).Error; err != nil {
			fmt.Printf("创建父菜单错误:%v\n", err)
			continue
		}
		menus = append(menus, mod.Parent)
		for _, child := range mod.Children {
			child.Pid = int(mod.Parent.ID) // Typecast uint to int to match tbl_system_menu.go
			if err := Db.Create(child).Error; err != nil {
				fmt.Printf("创建子菜单错误:%v\n", err)
				continue
			}
			menus = append(menus, child)
		}
	}

	type ApiModule struct {
		Parent   *SystemApi
		Children []*SystemApi
	}

	apiModules := []ApiModule{
		// 系统管理
		{
			Parent: &SystemApi{Path: "/api/system", Method: "GET", Title: "系统管理", Type: "0", Pid: 0},
			Children: []*SystemApi{
				{Path: "/api/getUserInfo", Method: "GET", Title: "[用户模块]获取用户信息", Type: "1"},
				{Path: "/api/getPermCode", Method: "GET", Title: "[用户模块]获取用户权限码", Type: "1"},
				{Path: "/api/system/getAccountList", Method: "GET", Title: "[用户模块]获取账号列表", Type: "1"},
				{Path: "/api/system/createAccount", Method: "POST", Title: "[用户模块]创建账号", Type: "1"},
				{Path: "/api/system/updateAccount", Method: "POST", Title: "[用户模块]更新账号", Type: "1"},
				{Path: "/api/system/deleteAccount/:id", Method: "DELETE", Title: "[用户模块]删除账号", Type: "1"},
				{Path: "/api/system/setAccountStatus", Method: "POST", Title: "[用户模块]设置账号状态", Type: "1"},
				{Path: "/api/system/accountExist", Method: "POST", Title: "[用户模块]检查账号存在", Type: "1"},
				{Path: "/api/system/changePassword", Method: "POST", Title: "[用户模块]修改密码", Type: "1"},
				{Path: "/api/system/updateUserInfo", Method: "POST", Title: "[用户模块]修改个人设置", Type: "1"},
				{Path: "/api/system/uploadAvatar", Method: "POST", Title: "[用户模块]上传个人头像", Type: "1"},
				{Path: "/api/system/getAllUserAndRoles", Method: "GET", Title: "[用户模块]获取所有用户与角色", Type: "1"},
				{Path: "/api/system/getMenuList", Method: "GET", Title: "[菜单模块]获取用户菜单", Type: "1"},
				{Path: "/api/system/getMenuListAll", Method: "GET", Title: "[菜单模块]获取全量菜单", Type: "1"},
				{Path: "/api/system/createMenu", Method: "POST", Title: "[菜单模块]创建菜单", Type: "1"},
				{Path: "/api/system/updateMenu", Method: "POST", Title: "[菜单模块]更新菜单", Type: "1"},
				{Path: "/api/system/deleteMenu/:id", Method: "DELETE", Title: "[菜单模块]删除菜单", Type: "1"},
				{Path: "/api/system/getRoleListAll", Method: "GET", Title: "[角色模块]获取所有角色", Type: "1"},
				{Path: "/api/system/createRole", Method: "POST", Title: "[角色模块]创建角色", Type: "1"},
				{Path: "/api/system/updateRole", Method: "POST", Title: "[角色模块]更新角色", Type: "1"},
				{Path: "/api/system/deleteRole/:id", Method: "DELETE", Title: "[角色模块]删除角色", Type: "1"},
				{Path: "/api/system/setRoleStatus", Method: "POST", Title: "[角色模块]设置角色状态", Type: "1"},
				{Path: "/api/system/getApiList", Method: "GET", Title: "[接口模块]获取API列表", Type: "1"},
				{Path: "/api/system/getApiListAll", Method: "GET", Title: "[接口模块]获取全量API", Type: "1"},
				{Path: "/api/system/createApi", Method: "POST", Title: "[接口模块]创建API", Type: "1"},
				{Path: "/api/system/updateApi", Method: "POST", Title: "[接口模块]更新API", Type: "1"},
				{Path: "/api/system/deleteApi/:id", Method: "DELETE", Title: "[接口模块]删除API", Type: "1"},
				{Path: "/api/system/setting/get", Method: "GET", Title: "系统管理-全局设置", Type: "1"},
				{Path: "/api/system/setting/update", Method: "PUT", Title: "系统管理-更新设置", Type: "1"},
				{Path: "/api/system/getAuditLogList", Method: "GET", Title: "[审计模块]获取操作审计日志列表", Type: "1"},
			},
		},

		// 服务树
		{
			Parent: &SystemApi{Path: "/api/stree", Method: "GET", Title: "CMDB资产管理模块", Type: "0"},
			Children: []*SystemApi{
				{Path: "/api/stree/getStreeNodeList", Method: "GET", Title: "[cmdb树节点]获取服务树节点列表", Type: "1"},
				{Path: "/api/stree/getStreeNodeSelect", Method: "GET", Title: "[cmdb树节点]获取服务树下拉选择", Type: "1"},
				{Path: "/api/stree/getTopStreeNodes", Method: "GET", Title: "[cmdb树节点]获取服务树顶层节点", Type: "1"},
				{Path: "/api/stree/getChildrenStreeNodes/:pid", Method: "GET", Title: "[cmdb树节点]获取服务树子节点", Type: "1"},
				{Path: "/api/stree/createStreeNode", Method: "POST", Title: "[cmdb树节点]创建服务树节点", Type: "1"},
				{Path: "/api/stree/updateStreeNode", Method: "POST", Title: "[cmdb树节点]更新服务树节点", Type: "1"},
				{Path: "/api/stree/deleteStreeNode/:id", Method: "DELETE", Title: "[cmdb树节点]删除服务树节点", Type: "1"},
				{Path: "/api/stree/getLeafStreeNodes", Method: "GET", Title: "[cmdb树节点]获取服务树叶子节点", Type: "1"},
				{Path: "/api/stree/fetchResourceByNode", Method: "GET", Title: "[cmdb树节点]根据节点获取资源", Type: "1"},
				{Path: "/api/stree/bindEcsToStreeNode", Method: "POST", Title: "[cmdb云主机]节点绑定ECS", Type: "1"},
				{Path: "/api/stree/unBindEcsToStreeNode", Method: "POST", Title: "[cmdb云主机]节点解绑ECS", Type: "1"},
				{Path: "/api/stree/getStreeNodeEcsList/:id", Method: "GET", Title: "[cmdb云主机]获取节点已绑定ECS列表", Type: "1"},
				{Path: "/api/stree/getResourceEcsUnbindList", Method: "GET", Title: "[cmdb云主机]获取节点未绑定ECS列表", Type: "1"},
				{Path: "/api/stree/getResourceEcsList", Method: "GET", Title: "[cmdb云主机]获取节点ECS列表", Type: "1"},
				{Path: "/api/stree/getResourceElbUnbindList", Method: "GET", Title: "[cmdb负载均衡]获取未绑定ELB", Type: "1"},
				{Path: "/api/stree/bindElbToStreeNode", Method: "POST", Title: "[cmdb负载均衡]节点绑定ELB", Type: "1"},
				{Path: "/api/stree/unBindElbToStreeNode", Method: "POST", Title: "[cmdb负载均衡]节点解绑ELB", Type: "1"},
				{Path: "/api/stree/getResourceRdsUnbindList", Method: "GET", Title: "[cmdb数据库]获取未绑定RDS", Type: "1"},
				{Path: "/api/stree/bindRdsToStreeNode", Method: "POST", Title: "[cmdb数据库]节点绑定RDS", Type: "1"},
				{Path: "/api/stree/unBindRdsToStreeNode", Method: "POST", Title: "[cmdb数据库]节点解绑RDS", Type: "1"},
				{Path: "/api/stree/getResourceDnsUnbindList", Method: "GET", Title: "[cmdb域名]获取未绑定DNS", Type: "1"},
				{Path: "/api/stree/bindDnsToStreeNode", Method: "POST", Title: "[cmdb域名]节点绑定DNS", Type: "1"},
				{Path: "/api/stree/unBindDnsToStreeNode", Method: "POST", Title: "[cmdb域名]节点解绑DNS", Type: "1"},
			},
		},

		// it工单
		{
			Parent: &SystemApi{Path: "/api/workorder", Method: "GET", Title: "工单服务模块", Type: "0"},
			Children: []*SystemApi{
				{Path: "/api/workorder/getProcessList", Method: "GET", Title: "[工单模块]获取流程列表", Type: "1"},
				{Path: "/api/workorder/createProcess", Method: "POST", Title: "[工单模块]创建流程", Type: "1"},
				{Path: "/api/workorder/updateProcess", Method: "POST", Title: "[工单模块]更新流程", Type: "1"},
				{Path: "/api/workorder/deleteProcess/:id", Method: "DELETE", Title: "[工单模块]删除流程", Type: "1"},
				{Path: "/api/workorder/getFormDesignList", Method: "GET", Title: "[工单模块]获取表单设计列表", Type: "1"},
				{Path: "/api/workorder/createFormDesign", Method: "POST", Title: "[工单模块]创建表单设计", Type: "1"},
				{Path: "/api/workorder/updateFormDesign", Method: "POST", Title: "[工单模块]更新表单设计", Type: "1"},
				{Path: "/api/workorder/deleteFormDesign/:id", Method: "DELETE", Title: "[工单模块]删除表单设计", Type: "1"},
				{Path: "/api/workorder/getWorkOrderTemplateList", Method: "GET", Title: "[工单模块]获取工单模板列表", Type: "1"},
				{Path: "/api/workorder/createWorkOrderTemplate", Method: "POST", Title: "[工单模块]创建工单模板", Type: "1"},
				{Path: "/api/workorder/updateWorkOrderTemplate", Method: "POST", Title: "[工单模块]更新工单模板", Type: "1"},
				{Path: "/api/workorder/deleteWorkOrderTemplate/:id", Method: "DELETE", Title: "[工单模块]删除工单模板", Type: "1"},
				{Path: "/api/workorder/getWorkOrderTemplateDetail/:id", Method: "GET", Title: "[工单模块]获取工单模板详情", Type: "1"},
				{Path: "/api/workorder/getWorkOrderInstanceList", Method: "GET", Title: "[工单模块]获取工单实例列表", Type: "1"},
				{Path: "/api/workorder/createWorkOrderInstance", Method: "POST", Title: "[工单模块]创建工单实例", Type: "1"},
				{Path: "/api/workorder/updateWorkOrderInstance", Method: "POST", Title: "[工单模块]更新工单实例", Type: "1"},
				{Path: "/api/workorder/deleteWorkOrderInstance/:id", Method: "DELETE", Title: "[工单模块]删除工单实例", Type: "1"},
				{Path: "/api/workorder/approvalWorkOrderInstance/:id", Method: "POST", Title: "[工单模块]审批工单", Type: "1"},
				{Path: "/api/workorder/actionWorkOrderInstance/:id", Method: "POST", Title: "[工单模块]执行工单动作", Type: "1"},
				{Path: "/api/workorder/getWorkOrderInstanceDetail/:id", Method: "GET", Title: "[工单模块]获取工单实例详情", Type: "1"},
				{Path: "/api/workorder/commentWorkOrderInstance/:id", Method: "POST", Title: "[工单模块]评论工单", Type: "1"},
				{Path: "/api/workorder/getNotificationList", Method: "GET", Title: "[工单模块]获取顶部通知/待办", Type: "1"},
				{Path: "/api/workorder/markNotifyRead", Method: "POST", Title: "[工单模块]标记通知已读", Type: "1"},
				{Path: "/api/workorder/clearNotifyTab", Method: "POST", Title: "[工单模块]清空通知列表", Type: "1"},
			},
		},

		// 任务执行
		{
			Parent: &SystemApi{Path: "/api/jobexec", Method: "GET", Title: "任务执行中心模块", Type: "0"},
			Children: []*SystemApi{
				{Path: "/api/jobexec/getJobExecScriptList", Method: "GET", Title: "[任务执行中心]获取脚本列表", Type: "1"},
				{Path: "/api/jobexec/createJobExecScript", Method: "POST", Title: "[任务执行中心]创建脚本", Type: "1"},
				{Path: "/api/jobexec/updateJobExecScript", Method: "POST", Title: "[任务执行中心]更新脚本", Type: "1"},
				{Path: "/api/jobexec/deleteJobExecScript/:id", Method: "DELETE", Title: "[任务执行中心]删除脚本", Type: "1"},
				{Path: "/api/jobexec/getJobExecScriptSelect", Method: "GET", Title: "[任务执行中心]获取脚本下拉选择", Type: "1"},
				{Path: "/api/jobexec/getJobExecScriptOne/:id", Method: "GET", Title: "[任务执行中心]获取单个脚本", Type: "1"},
				{Path: "/api/jobexec/getJobExecScriptDetail/:id", Method: "GET", Title: "[任务执行中心]获取脚本详情", Type: "1"},
				{Path: "/api/jobexec/getJobExecTaskList", Method: "GET", Title: "[任务执行中心]获取任务列表", Type: "1"},
				{Path: "/api/jobexec/getJobExecTaskOne/:id", Method: "GET", Title: "[任务执行中心]获取单个任务", Type: "1"},
				{Path: "/api/jobexec/createJobExecTask", Method: "POST", Title: "[任务执行中心]创建任务", Type: "1"},
				{Path: "/api/jobexec/updateJobExecTask", Method: "POST", Title: "[任务执行中心]更新任务", Type: "1"},
				{Path: "/api/jobexec/deleteJobExecTask/:id", Method: "DELETE", Title: "[任务执行中心]删除任务", Type: "1"},
				{Path: "/api/jobexec/actionJobExecTaskOne/:id", Method: "POST", Title: "[任务执行中心]执行任务", Type: "1"},
				{Path: "/api/jobexec/getJobExecResultByJobId", Method: "GET", Title: "[任务执行中心]获取任务结果", Type: "1"},
			},
		},

		// 监控中心
		{
			Parent: &SystemApi{Path: "/api/monitor", Method: "GET", Title: "监控中心模块", Type: "0"},
			Children: []*SystemApi{
				{Path: "/api/monitor/getMonitorPromScrapePoolList", Method: "GET", Title: "[prometheus]获取Prom集群列表", Type: "1"},
				{Path: "/api/monitor/createMonitorPromScrapePool", Method: "POST", Title: "[prometheus]创建Prom集群", Type: "1"},
				{Path: "/api/monitor/updateMonitorPromScrapePool", Method: "POST", Title: "[prometheus]更新Prom集群", Type: "1"},
				{Path: "/api/monitor/deleteMonitorPromScrapePool/:id", Method: "DELETE", Title: "[prometheus]删除Prom集群", Type: "1"},
				{Path: "/api/monitor/getMonitorPrometheusYamlOne", Method: "GET", Title: "[prometheus]获取Prom主配置", Type: "1"},
				{Path: "/api/monitor/getMonitorPrometheusAlertRuleYamlOne", Method: "GET", Title: "[prometheus]获取告警规则配置", Type: "1"},
				{Path: "/api/monitor/getMonitorPrometheusRecordRuleYamlOne", Method: "GET", Title: "[prometheus]获取预聚合规则配置", Type: "1"},
				{Path: "/api/monitor/getMonitorPromScrapeJobList", Method: "GET", Title: "[prometheus]获取采集任务列表", Type: "1"},
				{Path: "/api/monitor/getMonitorPromScrapeJobOne", Method: "GET", Title: "[prometheus]获取单个采集任务", Type: "1"},
				{Path: "/api/monitor/createMonitorPromScrapeJob", Method: "POST", Title: "[prometheus]创建采集任务", Type: "1"},
				{Path: "/api/monitor/updateMonitorPromScrapeJob", Method: "POST", Title: "[prometheus]更新采集任务", Type: "1"},
				{Path: "/api/monitor/deleteMonitorPromScrapeJob/:id", Method: "DELETE", Title: "[prometheus]删除采集任务", Type: "1"},
				{Path: "/api/monitor/setMonitorPromScrapeJobStatus", Method: "POST", Title: "[prometheus]设置采集任务状态", Type: "1"},
				{Path: "/api/monitor/getMonitorPromAlertRuleList", Method: "GET", Title: "[prometheus]获取告警规则列表", Type: "1"},
				{Path: "/api/monitor/createMonitorPromAlertRule", Method: "POST", Title: "[prometheus]创建告警规则", Type: "1"},
				{Path: "/api/monitor/updateMonitorPromAlertRule", Method: "POST", Title: "[prometheus]更新告警规则", Type: "1"},
				{Path: "/api/monitor/deleteMonitorPromAlertRule/:id", Method: "DELETE", Title: "[prometheus]删除告警规则", Type: "1"},
				{Path: "/api/monitor/deleteMonitorPromAlertRuleBatch", Method: "DELETE", Title: "[prometheus]批量删除告警规则", Type: "1"},
				{Path: "/api/monitor/setMonitorPromAlertRuleStatus", Method: "POST", Title: "[prometheus]设置告警规则状态", Type: "1"},
				{Path: "/api/monitor/setMonitorPromAlertRuleStatusBatch", Method: "POST", Title: "[prometheus]批量设置告警规则状态", Type: "1"},
				{Path: "/api/monitor/promqlExprCheck", Method: "GET", Title: "[prometheus]检查PromQL表达式", Type: "1"},
				{Path: "/api/monitor/getMonitorPromRecordRuleList", Method: "GET", Title: "[prometheus]获取聚合规则列表", Type: "1"},
				{Path: "/api/monitor/createMonitorPromRecordRule", Method: "POST", Title: "[prometheus]创建聚合规则", Type: "1"},
				{Path: "/api/monitor/updateMonitorPromRecordRule", Method: "POST", Title: "[prometheus]更新聚合规则", Type: "1"},
				{Path: "/api/monitor/deleteMonitorPromRecordRule/:id", Method: "DELETE", Title: "[prometheus]删除聚合规则", Type: "1"},
				{Path: "/api/monitor/deleteMonitorPromRecordRuleBatch", Method: "DELETE", Title: "[prometheus]批量删除聚合规则", Type: "1"},
				{Path: "/api/monitor/setMonitorPromRecordRuleStatus", Method: "POST", Title: "[prometheus]设置聚合规则状态", Type: "1"},
				{Path: "/api/monitor/setMonitorPromRecordRuleStatusBatch", Method: "POST", Title: "[prometheus]批量设置聚合规则状态", Type: "1"},
				{Path: "/api/monitor/recordRulePromqlExprCheck", Method: "GET", Title: "[prometheus]检查聚合规则表达式", Type: "1"},
				{Path: "/api/monitor/getMonitorAlertManagerPoolList", Method: "GET", Title: "[alertmanager]获取Alert集群列表", Type: "1"},
				{Path: "/api/monitor/createMonitorAlertManagerPool", Method: "POST", Title: "[alertmanager]创建Alert集群", Type: "1"},
				{Path: "/api/monitor/updateMonitorAlertManagerPool", Method: "POST", Title: "[alertmanager]更新Alert集群", Type: "1"},
				{Path: "/api/monitor/deleteMonitorAlertManagerPool/:id", Method: "DELETE", Title: "[alertmanager]删除Alert集群", Type: "1"},
				{Path: "/api/monitor/getMonitorAlertManagerYamlOne", Method: "GET", Title: "[alertmanager]获取Alert主配置", Type: "1"},
				{Path: "/api/monitor/getMonitorAlertManagerSendGroupList", Method: "GET", Title: "[告警发送组]获取发送组列表", Type: "1"},
				{Path: "/api/monitor/createMonitorAlertManagerSendGroup", Method: "POST", Title: "[告警发送组]创建发送组", Type: "1"},
				{Path: "/api/monitor/updateMonitorAlertManagerSendGroup", Method: "POST", Title: "[告警发送组]更新发送组", Type: "1"},
				{Path: "/api/monitor/deleteMonitorAlertManagerSendGroup/:id", Method: "DELETE", Title: "[告警发送组]删除发送组", Type: "1"},
				{Path: "/api/monitor/setAlertManagerSendGroupStatus", Method: "POST", Title: "[告警发送组]设置发送组状态", Type: "1"},
				{Path: "/api/monitor/getMonitorAlertManagerEventList", Method: "GET", Title: "[告警事件]获取告警事件列表", Type: "1"},
				{Path: "/api/monitor/alertManagerEventSilence/:id", Method: "POST", Title: "[告警事件]静默告警事件", Type: "1"},
				{Path: "/api/monitor/alertManagerEventUnSilence/:id", Method: "POST", Title: "[告警事件]解除静默", Type: "1"},
				{Path: "/api/monitor/alertManagerEventBatchSilence", Method: "POST", Title: "[告警事件]批量静默告警", Type: "1"},
				{Path: "/api/monitor/alertManagerEventBatchUnSilence", Method: "POST", Title: "[告警事件]批量解除静默", Type: "1"},
				{Path: "/api/monitor/alertManagerEventReLing/:id", Method: "POST", Title: "[告警事件]认领告警事件", Type: "1"},
				{Path: "/api/monitor/getMonitorOndutyGroupList", Method: "GET", Title: "[值班组]获取值班组列表", Type: "1"},
				{Path: "/api/monitor/getMonitorOndutyGroupOne/:id", Method: "GET", Title: "[值班组]获取单个值班组", Type: "1"},
				{Path: "/api/monitor/createMonitorOndutyGroup", Method: "POST", Title: "[值班组]创建值班组", Type: "1"},
				{Path: "/api/monitor/updateMonitorOndutyGroup", Method: "POST", Title: "[值班组]更新值班组", Type: "1"},
				{Path: "/api/monitor/deleteMonitorOndutyGroup/:id", Method: "DELETE", Title: "[值班组]删除值班组", Type: "1"},
				{Path: "/api/monitor/setMonitorOndutyStatus", Method: "POST", Title: "[排班表]设置值班状态", Type: "1"},
				{Path: "/api/monitor/getMonitorOndutyGroupFuturePlan/:id", Method: "GET", Title: "[排班表]获取排班计划", Type: "1"},
				{Path: "/api/monitor/createMonitorOndutyChange", Method: "POST", Title: "[排班表]创建替班记录", Type: "1"},
			},
		},

		// 代码管理
		{
			Parent: &SystemApi{Path: "/api/code", Method: "GET", Title: "代码管理", Type: "0"},
			Children: []*SystemApi{
				{Path: "/api/code/getCodeGitServerList", Method: "GET", Title: "[代码管理]获取Git服务器列表", Type: "1"},
				{Path: "/api/code/createCodeGitServer", Method: "POST", Title: "[代码管理]创建Git服务器", Type: "1"},
				{Path: "/api/code/updateCodeGitServer", Method: "POST", Title: "[代码管理]更新Git服务器", Type: "1"},
				{Path: "/api/code/deleteCodeGitServer/:id", Method: "DELETE", Title: "[代码管理]删除Git服务器", Type: "1"},
				{Path: "/api/code/pingCodeGitServer", Method: "POST", Title: "[代码管理]测试Git服务器连接", Type: "1"},
				{Path: "/api/code/getCodeGitRepoList", Method: "GET", Title: "[代码管理]获取代码仓库列表", Type: "1"},
				{Path: "/api/code/createCodeGitRepo", Method: "POST", Title: "[代码管理]创建代码仓库", Type: "1"},
				{Path: "/api/code/updateCodeGitRepo", Method: "POST", Title: "[代码管理]更新代码仓库", Type: "1"},
				{Path: "/api/code/getGitNamespaces", Method: "GET", Title: "[代码管理]获取命名空间列表", Type: "1"},
				{Path: "/api/code/createGitNamespace", Method: "POST", Title: "[代码管理]创建命名空间", Type: "1"},
				{Path: "/api/code/updateGitNamespace", Method: "POST", Title: "[代码管理]更新命名空间", Type: "1"},
				{Path: "/api/code/getGitUsers", Method: "GET", Title: "[代码管理]获取远端用户列表", Type: "1"},
				{Path: "/api/code/createGitUser", Method: "POST", Title: "[代码管理]创建远端用户", Type: "1"},
				{Path: "/api/code/updateGitUser", Method: "POST", Title: "[代码管理]更新远端用户", Type: "1"},
				{Path: "/api/code/getRepoMembers", Method: "GET", Title: "[代码管理]获取仓库成员", Type: "1"},
				{Path: "/api/code/addRepoMember", Method: "POST", Title: "[代码管理]添加/更新仓库成员", Type: "1"},
				{Path: "/api/code/removeRepoMember", Method: "DELETE", Title: "[代码管理]移除仓库成员", Type: "1"},
				{Path: "/api/code/getRepoBranches", Method: "GET", Title: "[代码管理]获取仓库分支列表", Type: "1"},
				{Path: "/api/code/getMergeRequests", Method: "GET", Title: "[代码管理]获取合并请求列表", Type: "1"},
				{Path: "/api/code/createMergeRequest", Method: "POST", Title: "[代码管理]创建合并请求", Type: "1"},
				{Path: "/api/code/mergeMergeRequest", Method: "POST", Title: "[代码管理]合并合并请求", Type: "1"},
				{Path: "/api/code/closeMergeRequest", Method: "POST", Title: "[代码管理]关闭合并请求", Type: "1"},
			},
		},
		{
			Parent: &SystemApi{Path: "/api/k8s", Method: "GET", Title: "容器集群模块", Type: "0"},
			Children: []*SystemApi{
				{Path: "/api/k8s/getK8sClusterList", Method: "GET", Title: "[k8s集群]获取集群列表", Type: "1"},
				{Path: "/api/k8s/createK8sCluster", Method: "POST", Title: "[k8s集群]创建集群", Type: "1"},
				{Path: "/api/k8s/updateK8sCluster", Method: "POST", Title: "[k8s集群]更新集群", Type: "1"},
				{Path: "/api/k8s/deleteK8sCluster/:id", Method: "DELETE", Title: "[k8s集群]删除集群", Type: "1"},
				{Path: "/api/k8s/deleteK8sClusterBatch", Method: "DELETE", Title: "[k8s集群]批量删除集群", Type: "1"},
				{Path: "/api/k8s/getK8sNodeList", Method: "GET", Title: "[k8s节点]获取节点列表", Type: "1"},
				{Path: "/api/k8s/getClusterForSelect", Method: "GET", Title: "[k8s集群]获取集群下拉", Type: "1"},
				{Path: "/api/k8s/scheduleEnableSwitchK8sNodesOne", Method: "POST", Title: "[k8s节点]开启/关闭调度", Type: "1"},
				{Path: "/api/k8s/labelK8sNodes", Method: "POST", Title: "[k8s节点]标签管理", Type: "1"},
				{Path: "/api/k8s/taintK8sNodes", Method: "POST", Title: "[k8s节点]污点管理", Type: "1"},
				{Path: "/api/k8s/drainK8sNodes", Method: "POST", Title: "[k8s节点]驱逐节点", Type: "1"},
				{Path: "/api/k8s/getPodListByNodeName", Method: "GET", Title: "[k8s节点]获取节点Pod列表", Type: "1"},
				{Path: "/api/k8s/getK8sYamlTemplateList", Method: "GET", Title: "[k8s YAML]获取模板列表", Type: "1"},
				{Path: "/api/k8s/createK8sYamlTemplate", Method: "POST", Title: "[k8s YAML]创建模板", Type: "1"},
				{Path: "/api/k8s/updateK8sYamlTemplate", Method: "POST", Title: "[k8s YAML]更新模板", Type: "1"},
				{Path: "/api/k8s/deleteK8sYamlTemplate/:id", Method: "DELETE", Title: "[k8s YAML]删除模板", Type: "1"},
				{Path: "/api/k8s/getK8sYamlTaskList", Method: "GET", Title: "[k8s YAML]获取发布任务列表", Type: "1"},
				{Path: "/api/k8s/createK8sYamlTask", Method: "POST", Title: "[k8s YAML]创建发布任务", Type: "1"},
				{Path: "/api/k8s/updateK8sYamlTask", Method: "POST", Title: "[k8s YAML]更新发布任务", Type: "1"},
				{Path: "/api/k8s/deleteK8sYamlTask/:id", Method: "DELETE", Title: "[k8s YAML]删除发布任务", Type: "1"},
				{Path: "/api/k8s/applyK8sYamlTaskOne/:id", Method: "POST", Title: "[k8s YAML]执行任务发布", Type: "1"},
				{Path: "/api/k8s/getK8sYamlTaskLogList", Method: "GET", Title: "[k8s YAML]获取任务发布历史记录", Type: "1"},
				{Path: "/api/k8s/getK8sNamespaceList", Method: "GET", Title: "[k8s Pod]获取命名空间列表", Type: "1"},
				{Path: "/api/k8s/getK8sPodList", Method: "GET", Title: "[k8s Pod]获取Pod列表", Type: "1"},
				{Path: "/api/k8s/getK8sPodYaml", Method: "GET", Title: "[k8s Pod]获取Pod YAML源码", Type: "1"},
				{Path: "/api/k8s/createK8sPod", Method: "POST", Title: "[k8s Pod]创建Pod", Type: "1"},
				{Path: "/api/k8s/updateK8sPod", Method: "POST", Title: "[k8s Pod]更新Pod", Type: "1"},
				{Path: "/api/k8s/deleteK8sPod", Method: "POST", Title: "[k8s Pod]删除单个Pod", Type: "1"},
				{Path: "/api/k8s/deleteK8sPodBatch", Method: "POST", Title: "[k8s Pod]批量删除Pod", Type: "1"},
				{Path: "/api/k8s/getK8sPodLogs", Method: "GET", Title: "[k8s Pod]获取Pod日志", Type: "1"},
				{Path: "/api/k8s/wsK8sPodExec", Method: "GET", Title: "[k8s Pod]在线Exec Terminal", Type: "1"},
				{Path: "/api/k8s/wsK8sPodWatch", Method: "GET", Title: "[k8s Pod]Watch Pod 实时变更推送", Type: "1"},
				{Path: "/api/k8s/wsK8sPodLogs", Method: "GET", Title: "[k8s Pod]WebSocket 实时日志持续流", Type: "1"},
				{Path: "/api/k8s/downloadK8sPodFile", Method: "GET", Title: "[k8s Pod]下载容器内部文件流", Type: "1"},
				{Path: "/api/k8s/getK8sPodFileList", Method: "GET", Title: "[k8s Pod]获取容器内部目录与文件列表", Type: "1"},
				{Path: "/api/k8s/uploadK8sPodFile", Method: "POST", Title: "[k8s Pod]上传文件到容器", Type: "1"},
				{Path: "/api/k8s/deleteK8sPodFile", Method: "POST", Title: "[k8s Pod]删除容器内部文件/目录", Type: "1"},
				{Path: "/api/k8s/readK8sPodFileContent", Method: "GET", Title: "[k8s Pod]在线预览容器文件内容", Type: "1"},
				{Path: "/api/k8s/saveK8sPodFileContent", Method: "POST", Title: "[k8s Pod]在线编辑保存容器文件内容", Type: "1"},
				{Path: "/api/k8s/getK8sDeploymentList", Method: "GET", Title: "[k8s Deployment]获取Deployment列表", Type: "1"},
				{Path: "/api/k8s/getK8sDeploymentYaml", Method: "GET", Title: "[k8s Deployment]获取Deployment YAML源码", Type: "1"},
				{Path: "/api/k8s/createK8sDeployment", Method: "POST", Title: "[k8s Deployment]创建Deployment", Type: "1"},
				{Path: "/api/k8s/updateK8sDeployment", Method: "POST", Title: "[k8s Deployment]更新Deployment", Type: "1"},
				{Path: "/api/k8s/scaleK8sDeployment", Method: "POST", Title: "[k8s Deployment]扩缩容Deployment副本", Type: "1"},
				{Path: "/api/k8s/restartK8sDeployment", Method: "POST", Title: "[k8s Deployment]滚动重启Deployment", Type: "1"},
				{Path: "/api/k8s/deleteK8sDeployment", Method: "POST", Title: "[k8s Deployment]删除单个Deployment", Type: "1"},
				{Path: "/api/k8s/deleteK8sDeploymentBatch", Method: "POST", Title: "[k8s Deployment]批量删除Deployment", Type: "1"},
				{Path: "/api/k8s/wsK8sDeploymentWatch", Method: "GET", Title: "[k8s Deployment]Watch Deployment 实时变更推送", Type: "1"},
				{Path: "/api/k8s/getK8sStatefulSetList", Method: "GET", Title: "[k8s StatefulSet]获取StatefulSet列表", Type: "1"},
				{Path: "/api/k8s/getK8sStatefulSetYaml", Method: "GET", Title: "[k8s StatefulSet]获取StatefulSet YAML", Type: "1"},
				{Path: "/api/k8s/createK8sStatefulSet", Method: "POST", Title: "[k8s StatefulSet]创建StatefulSet", Type: "1"},
				{Path: "/api/k8s/updateK8sStatefulSet", Method: "POST", Title: "[k8s StatefulSet]更新StatefulSet", Type: "1"},
				{Path: "/api/k8s/scaleK8sStatefulSet", Method: "POST", Title: "[k8s StatefulSet]扩缩容StatefulSet", Type: "1"},
				{Path: "/api/k8s/restartK8sStatefulSet", Method: "POST", Title: "[k8s StatefulSet]重启StatefulSet", Type: "1"},
				{Path: "/api/k8s/deleteK8sStatefulSet", Method: "POST", Title: "[k8s StatefulSet]删除StatefulSet", Type: "1"},
				{Path: "/api/k8s/deleteK8sStatefulSetBatch", Method: "POST", Title: "[k8s StatefulSet]批量删除StatefulSet", Type: "1"},
				{Path: "/api/k8s/getK8sDaemonSetList", Method: "GET", Title: "[k8s DaemonSet]获取DaemonSet列表", Type: "1"},
				{Path: "/api/k8s/getK8sDaemonSetYaml", Method: "GET", Title: "[k8s DaemonSet]获取DaemonSet YAML", Type: "1"},
				{Path: "/api/k8s/createK8sDaemonSet", Method: "POST", Title: "[k8s DaemonSet]创建DaemonSet", Type: "1"},
				{Path: "/api/k8s/updateK8sDaemonSet", Method: "POST", Title: "[k8s DaemonSet]更新DaemonSet", Type: "1"},
				{Path: "/api/k8s/restartK8sDaemonSet", Method: "POST", Title: "[k8s DaemonSet]重启DaemonSet", Type: "1"},
				{Path: "/api/k8s/deleteK8sDaemonSet", Method: "POST", Title: "[k8s DaemonSet]删除DaemonSet", Type: "1"},
				{Path: "/api/k8s/deleteK8sDaemonSetBatch", Method: "POST", Title: "[k8s DaemonSet]批量删除DaemonSet", Type: "1"},
				{Path: "/api/k8s/getK8sConfigMapList", Method: "GET", Title: "[k8s ConfigMap]获取ConfigMap列表", Type: "1"},
				{Path: "/api/k8s/getK8sConfigMapYaml", Method: "GET", Title: "[k8s ConfigMap]获取ConfigMap YAML", Type: "1"},
				{Path: "/api/k8s/createK8sConfigMap", Method: "POST", Title: "[k8s ConfigMap]创建ConfigMap", Type: "1"},
				{Path: "/api/k8s/updateK8sConfigMap", Method: "POST", Title: "[k8s ConfigMap]更新ConfigMap", Type: "1"},
				{Path: "/api/k8s/deleteK8sConfigMap", Method: "POST", Title: "[k8s ConfigMap]删除ConfigMap", Type: "1"},
				{Path: "/api/k8s/deleteK8sConfigMapBatch", Method: "POST", Title: "[k8s ConfigMap]批量删除ConfigMap", Type: "1"},
				{Path: "/api/k8s/getK8sSecretList", Method: "GET", Title: "[k8s Secret]获取Secret列表", Type: "1"},
				{Path: "/api/k8s/getK8sSecretYaml", Method: "GET", Title: "[k8s Secret]获取Secret YAML", Type: "1"},
				{Path: "/api/k8s/createK8sSecret", Method: "POST", Title: "[k8s Secret]创建Secret", Type: "1"},
				{Path: "/api/k8s/updateK8sSecret", Method: "POST", Title: "[k8s Secret]更新Secret", Type: "1"},
				{Path: "/api/k8s/deleteK8sSecret", Method: "POST", Title: "[k8s Secret]删除Secret", Type: "1"},
				{Path: "/api/k8s/deleteK8sSecretBatch", Method: "POST", Title: "[k8s Secret]批量删除Secret", Type: "1"},
				{Path: "/api/k8s/getK8sServiceList", Method: "GET", Title: "[k8s Service]获取Service列表", Type: "1"},
				{Path: "/api/k8s/getK8sServiceYaml", Method: "GET", Title: "[k8s Service]获取Service YAML", Type: "1"},
				{Path: "/api/k8s/createK8sService", Method: "POST", Title: "[k8s Service]创建Service", Type: "1"},
				{Path: "/api/k8s/updateK8sService", Method: "POST", Title: "[k8s Service]更新Service", Type: "1"},
				{Path: "/api/k8s/deleteK8sService", Method: "POST", Title: "[k8s Service]删除Service", Type: "1"},
				{Path: "/api/k8s/deleteK8sServiceBatch", Method: "POST", Title: "[k8s Service]批量删除Service", Type: "1"},
				{Path: "/api/k8s/getK8sIngressList", Method: "GET", Title: "[k8s Ingress]获取Ingress列表", Type: "1"},
				{Path: "/api/k8s/getK8sIngressYaml", Method: "GET", Title: "[k8s Ingress]获取Ingress YAML", Type: "1"},
				{Path: "/api/k8s/createK8sIngress", Method: "POST", Title: "[k8s Ingress]创建Ingress", Type: "1"},
				{Path: "/api/k8s/updateK8sIngress", Method: "POST", Title: "[k8s Ingress]更新Ingress", Type: "1"},
				{Path: "/api/k8s/deleteK8sIngress", Method: "POST", Title: "[k8s Ingress]删除Ingress", Type: "1"},
				{Path: "/api/k8s/deleteK8sIngressBatch", Method: "POST", Title: "[k8s Ingress]批量删除Ingress", Type: "1"},
				{Path: "/api/k8s/getK8sProjectList", Method: "GET", Title: "[k8s应用管理]获取项目列表", Type: "1"},
				{Path: "/api/k8s/getK8sProjectOne/:id", Method: "GET", Title: "[k8s应用管理]获取单个项目", Type: "1"},
				{Path: "/api/k8s/createK8sProject", Method: "POST", Title: "[k8s应用管理]创建项目", Type: "1"},
				{Path: "/api/k8s/updateK8sProject", Method: "POST", Title: "[k8s应用管理]更新项目", Type: "1"},
				{Path: "/api/k8s/deleteK8sProject/:id", Method: "DELETE", Title: "[k8s应用管理]删除项目", Type: "1"},
				{Path: "/api/k8s/getK8sAppList", Method: "GET", Title: "[k8s应用管理]获取应用列表", Type: "1"},
				{Path: "/api/k8s/getK8sAppOne/:id", Method: "GET", Title: "[k8s应用管理]获取单个应用", Type: "1"},
				{Path: "/api/k8s/createK8sApp", Method: "POST", Title: "[k8s应用管理]创建应用", Type: "1"},
				{Path: "/api/k8s/updateK8sApp", Method: "POST", Title: "[k8s应用管理]更新应用", Type: "1"},
				{Path: "/api/k8s/deleteK8sApp/:id", Method: "DELETE", Title: "[k8s应用管理]删除应用", Type: "1"},
				{Path: "/api/k8s/getK8sInstanceList", Method: "GET", Title: "[k8s应用管理]获取实例列表", Type: "1"},
				{Path: "/api/k8s/getK8sInstanceOne/:id", Method: "GET", Title: "[k8s应用管理]获取单个实例", Type: "1"},
				{Path: "/api/k8s/createK8sInstance", Method: "POST", Title: "[k8s应用管理]创建实例", Type: "1"},
				{Path: "/api/k8s/updateK8sInstance", Method: "POST", Title: "[k8s应用管理]更新实例", Type: "1"},
				{Path: "/api/k8s/deleteK8sInstance/:id", Method: "DELETE", Title: "[k8s应用管理]删除实例", Type: "1"},
				{Path: "/api/k8s/deployK8sInstance/:id", Method: "POST", Title: "[k8s应用管理]部署实例到集群", Type: "1"},
			},
		},

		{
			Parent: &SystemApi{Path: "/api/cicd", Method: "GET", Title: "Jenkins服务模块", Type: "0"},
			Children: []*SystemApi{
				{Path: "/api/cicd/getJenkinsInstanceList", Method: "GET", Title: "[Jenkins]获取实例列表", Type: "1"},
				{Path: "/api/cicd/createJenkinsInstance", Method: "POST", Title: "[Jenkins]创建实例", Type: "1"},
				{Path: "/api/cicd/updateJenkinsInstance", Method: "POST", Title: "[Jenkins]更新实例", Type: "1"},
				{Path: "/api/cicd/deleteJenkinsInstance", Method: "DELETE", Title: "[Jenkins]删除实例", Type: "1"},
				{Path: "/api/cicd/getJenkinsJobList", Method: "GET", Title: "[Jenkins]获取Job列表", Type: "1"},
				{Path: "/api/cicd/createJenkinsJob", Method: "POST", Title: "[Jenkins]创建Job", Type: "1"},
				{Path: "/api/cicd/updateJenkinsJob", Method: "POST", Title: "[Jenkins]更新Job", Type: "1"},
				{Path: "/api/cicd/deleteJenkinsJob", Method: "DELETE", Title: "[Jenkins]删除Job", Type: "1"},
				{Path: "/api/cicd/triggerJenkinsBuild", Method: "POST", Title: "[Jenkins]触发构建", Type: "1"},
				{Path: "/api/cicd/stopJenkinsBuild", Method: "POST", Title: "[Jenkins]停止构建", Type: "1"},
				{Path: "/api/cicd/getJenkinsBuildLogs", Method: "GET", Title: "[Jenkins]获取构建日志", Type: "1"},
				{Path: "/api/cicd/getJenkinsJobRemotePipeline", Method: "GET", Title: "[Jenkins]获取远程Pipeline定义", Type: "1"},
				{Path: "/api/cicd/getJenkinsJobStageView", Method: "GET", Title: "[Jenkins]获取Stage视图", Type: "1"},
				{Path: "/api/cicd/toggleJenkinsJobDeleteLock", Method: "POST", Title: "[Jenkins]切换删除锁", Type: "1"},
				{Path: "/api/cicd/getJenkinsPipelineList", Method: "GET", Title: "[Jenkins]获取流水线配置列表", Type: "1"},
				{Path: "/api/cicd/createJenkinsPipeline", Method: "POST", Title: "[Jenkins]创建流水线配置", Type: "1"},
				{Path: "/api/cicd/updateJenkinsPipeline", Method: "POST", Title: "[Jenkins]更新流水线配置", Type: "1"},
				{Path: "/api/cicd/deleteJenkinsPipeline", Method: "DELETE", Title: "[Jenkins]删除流水线配置", Type: "1"},
				{Path: "/api/cicd/validateJenkinsPipeline", Method: "POST", Title: "[Jenkins]校验Pipeline语法", Type: "1"},
				{Path: "/api/cicd/getJenkinsStageList", Method: "GET", Title: "[Jenkins Stage]获取Stage列表", Type: "1"},
				{Path: "/api/cicd/createJenkinsStage", Method: "POST", Title: "[Jenkins Stage]创建Stage", Type: "1"},
				{Path: "/api/cicd/updateJenkinsStage", Method: "POST", Title: "[Jenkins Stage]更新Stage", Type: "1"},
				{Path: "/api/cicd/deleteJenkinsStage", Method: "DELETE", Title: "[Jenkins Stage]删除Stage", Type: "1"},
				{Path: "/api/cicd/getJenkinsEnvList", Method: "GET", Title: "[Jenkins 环境变量]获取环境变量列表", Type: "1"},
				{Path: "/api/cicd/createJenkinsEnv", Method: "POST", Title: "[Jenkins 环境变量]创建环境变量", Type: "1"},
				{Path: "/api/cicd/updateJenkinsEnv", Method: "POST", Title: "[Jenkins 环境变量]更新环境变量", Type: "1"},
				{Path: "/api/cicd/deleteJenkinsEnv", Method: "DELETE", Title: "[Jenkins 环境变量]删除环境变量", Type: "1"},
				{Path: "/api/cicd/getJenkinsParamList", Method: "GET", Title: "[Jenkins 构建参数]获取构建参数列表", Type: "1"},
				{Path: "/api/cicd/createJenkinsParam", Method: "POST", Title: "[Jenkins 构建参数]创建构建参数", Type: "1"},
				{Path: "/api/cicd/updateJenkinsParam", Method: "POST", Title: "[Jenkins 构建参数]更新构建参数", Type: "1"},
				{Path: "/api/cicd/deleteJenkinsParam", Method: "DELETE", Title: "[Jenkins 构建参数]删除构建参数", Type: "1"},
			},
		},

		{
			Parent: &SystemApi{Path: "/api/artifactory", Method: "GET", Title: "Artifactory制品与配置库", Type: "0"},
			Children: []*SystemApi{
				{Path: "/api/artifactory/repos", Method: "GET", Title: "[Artifactory]获取全量仓库列表", Type: "1"},
				{Path: "/api/artifactory/info", Method: "GET", Title: "[Artifactory]获取文件构件元数据", Type: "1"},
				{Path: "/api/artifactory/tree", Method: "GET", Title: "[Artifactory]获取文件树", Type: "1"},
				{Path: "/api/artifactory/content", Method: "GET", Title: "[Artifactory]读取文本内容", Type: "1"},
				{Path: "/api/artifactory/save", Method: "POST", Title: "[Artifactory]保存修改文件", Type: "1"},
				{Path: "/api/artifactory/upload", Method: "POST", Title: "[Artifactory]上传文件构件", Type: "1"},
				{Path: "/api/artifactory/delete", Method: "POST", Title: "[Artifactory]删除文件构件", Type: "1"},
				{Path: "/api/artifactory/download", Method: "GET", Title: "[Artifactory]下载文件构件", Type: "1"},
			},
		},

		{
			Parent: &SystemApi{Path: "/api", Method: "GET", Title: "所有api权限", Type: "0"},
			Children: []*SystemApi{
				{Path: "/api/*", Method: "ALL", Title: "所有api ALL权限", Type: "1"},
				{Path: "/api/*", Method: "GET", Title: "所有api GET权限", Type: "1"},
				{Path: "/api/*", Method: "POST", Title: "所有api POST权限", Type: "1"},
				{Path: "/api/*", Method: "DELETE", Title: "所有api DELETE权限", Type: "1"},
				{Path: "/api/*", Method: "PATCH", Title: "所有api PATCH权限", Type: "1"},
				{Path: "/api/*", Method: "HEAD", Title: "所有api HEAD权限", Type: "1"},
				{Path: "/api/*", Method: "OPTIONS", Title: "所有api OPTIONS权限", Type: "1"},
				{Path: "/api/*", Method: "CONNECT", Title: "所有api CONNECT权限", Type: "1"},
				{Path: "/api/*", Method: "TRACE", Title: "所有api TRACE权限", Type: "1"},
			},
		},
	}

	var apis []*SystemApi
	for _, mod := range apiModules {
		if err := Db.Create(mod.Parent).Error; err != nil {
			fmt.Printf("创建父API错误:%v\n", err)
			continue
		}
		apis = append(apis, mod.Parent)
		for _, child := range mod.Children {
			child.Pid = int(mod.Parent.ID) // Typecast uint to int to match tbl_system_api.go
			if err := Db.Create(child).Error; err != nil {
				fmt.Printf("创建子API错误:%v\n", err)
				continue
			}
			apis = append(apis, child)
		}
	}

	// Define dynamic menu filters for different roles
	var menusRoleOps []*SystemMenu
	var menusRoleRebot []*SystemMenu = menus
	var menusRoleK8sAdmin []*SystemMenu

	var systemParentID, k8sParentID uint
	for _, mod := range modules {
		if mod.Parent.Name == "System" {
			systemParentID = mod.Parent.ID
		} else if mod.Parent.Name == "K8sClusterManagement" {
			k8sParentID = mod.Parent.ID
		}
	}

	for _, m := range menus {
		// Ops gets all menus except System Management
		if m.ID != systemParentID && m.Pid != int(systemParentID) {
			menusRoleOps = append(menusRoleOps, m)
		}
		// K8s Admin gets K8sClusterManagement menu and its children
		if m.ID == k8sParentID || m.Pid == int(k8sParentID) {
			menusRoleK8sAdmin = append(menusRoleK8sAdmin, m)
		}
	}

	// Define dynamic API filters for different roles
	var opsApis []*SystemApi
	var k8sApis []*SystemApi

	var systemApiParentID, streeApiParentID, monitorApiParentID, k8sApiParentID uint
	for _, mod := range apiModules {
		if mod.Parent.Path == "/api/system" {
			systemApiParentID = mod.Parent.ID
		} else if mod.Parent.Path == "/api/stree" {
			streeApiParentID = mod.Parent.ID
		} else if mod.Parent.Path == "/api/monitor" {
			monitorApiParentID = mod.Parent.ID
		} else if mod.Parent.Path == "/api/k8s" {
			k8sApiParentID = mod.Parent.ID
		}
	}

	for _, api := range apis {
		// Ops gets all APIs except System Management
		if api.ID != systemApiParentID && api.Pid != int(systemApiParentID) {
			opsApis = append(opsApis, api)
		}
		// K8s Admin gets CMDB stree, Monitor and K8s APIs
		if api.ID == streeApiParentID || api.Pid == int(streeApiParentID) ||
			api.ID == monitorApiParentID || api.Pid == int(monitorApiParentID) ||
			api.ID == k8sApiParentID || api.Pid == int(k8sApiParentID) {
			k8sApis = append(k8sApis, api)
		}
	}

	roleSuper := &SystemRole{
		RoleName: "超级管理员", RoleValue: "super", Menus: menus,
	}
	roleOps := &SystemRole{
		RoleName: "运维", RoleValue: "ops", Menus: menusRoleOps,
	}
	roleNoLogin := &SystemRole{
		RoleName: "自动机器人", RoleValue: "bot_super", Menus: menusRoleRebot,
	}
	roleK8sAdmin := &SystemRole{
		RoleName: "k8s集群管理员", RoleValue: "k8s_admin", Menus: menusRoleK8sAdmin,
	}

	adminUser := &SystemUser{
		Username:     "admin",
		Password:     common.BcryptHash("tingbao89.."),
		RealName:     "海绵宝宝",
		FeiShuUserId: "b75ag4g4",
		HomePath:     "/dashboard/analysis",
		Enable:       1,
		Roles: []*SystemRole{
			roleSuper,
		},
	}

	opsUser := &SystemUser{
		Username:     "test",
		Password:     common.BcryptHash("123456"),
		RealName:     "派大星",
		FeiShuUserId: "b75ag4g4",
		HomePath:     "/dashboard/analysis",
		Enable:       1,
		Roles: []*SystemRole{
			roleOps,
		},
	}

	botUser := &SystemUser{
		Username:     sc.WorkOrderAutoActionC.ServiceAccount,
		Password:     common.BcryptHash("123456"),
		RealName:     "后台机器人",
		FeiShuUserId: "b75ag4g4",
		HomePath:     "/dashboard/analysis",
		Enable:       1,
		Roles: []*SystemRole{
			roleNoLogin,
		},
	}

	k8sUser := &SystemUser{
		Username:     "k8s-admin-01",
		Password:     common.BcryptHash("123456"),
		RealName:     "k8s管理员01",
		FeiShuUserId: "b75ag4g4",
		HomePath:     "/k8s/node",
		Enable:       1,
		Roles: []*SystemRole{
			roleK8sAdmin,
		},
	}

	if err := Db.Create(adminUser).Error; err != nil {
		sc.Logger.Error("模拟管理员注册失败", zap.Error(err))
		return nil
	}
	_ = Db.Create(opsUser)
	_ = Db.Create(botUser)
	_ = Db.Create(k8sUser)

	users := []string{"蟹老板", "珊迪", "章鱼哥", "皮老板", "小蜗"}
	num := 5
	for i := 0; i < num; i++ {
		mockUser := &SystemUser{
			Username:     fmt.Sprintf("mock%d", i),
			Password:     common.BcryptHash("123456"),
			RealName:     users[i],
			FeiShuUserId: "b75ag4g4", //5egcb786
			HomePath:     "/system/role",
			Enable:       1,
		}
		_ = mockUser.CreateOne()
	}

	// Update GORM APIs
	_ = roleSuper.UpdateApis(apis)
	_ = roleOps.UpdateApis(opsApis)
	_ = roleNoLogin.UpdateApis(apis)
	_ = roleK8sAdmin.UpdateApis(k8sApis)

	// Add policies to Casbin
	for _, api := range apis {
		_, _ = CasbinEnforcer.AddPolicy("super", api.Path, api.Method)
		_, _ = CasbinEnforcer.AddPolicy("bot_super", api.Path, api.Method)
	}
	for _, api := range opsApis {
		_, _ = CasbinEnforcer.AddPolicy("ops", api.Path, api.Method)
	}
	for _, api := range k8sApis {
		_, _ = CasbinEnforcer.AddPolicy("k8s_admin", api.Path, api.Method)
	}

	var userMenus []*SystemMenu
	for _, m := range menus {
		if m.Name == "Dashboard" || m.Name == "Analysis" || m.Name == "System" || m.Name == "ChangePassword" {
			userMenus = append(userMenus, m)
		}
	}
	// 2. 筛选普通用户（user）所需的 API 接口
	var userApis []*SystemApi
	userApiPathMap := map[string]bool{
		"/api/getUserInfo":                   true,
		"/api/getPermCode":                   true,
		"/api/system/getMenuList":            true,
		"/api/system/setting/get":            true,
		"/api/system/changePassword":         true,
		"/api/system/updateUserInfo":         true,
		"/api/system/uploadAvatar":           true,
		"/api/workorder/getNotificationList": true, // 获取通知待办
		"/api/workorder/markNotifyRead":      true, // 标记已读
		"/api/workorder/clearNotifyTab":      true, // 清空通知列表
	}
	for _, api := range apis {
		if userApiPathMap[api.Path] {
			userApis = append(userApis, api)
		}
	}
	// 3. 定义并创建“普通用户”角色对象
	roleUser := &SystemRole{
		RoleName:  "普通用户",
		RoleValue: "user",
		Menus:     userMenus,
	}
	if err := Db.Create(roleUser).Error; err != nil {
		sc.Logger.Error("模拟普通用户角色创建失败", zap.Error(err))
	}
	// 4. 绑定 API 接口关联表 (同步更新 role_apis 中间表)
	_ = roleUser.UpdateApis(userApis)
	// 5. 将该角色的接口访问权限写入 Casbin RBAC 策略表
	for _, api := range userApis {
		_, _ = CasbinEnforcer.AddPolicy("user", api.Path, api.Method)
	}

	sc.Logger.Info("系统模块 Mock 数据注入成功")
	return adminUser
}

func EnsureJenkinsPipelineMenu(sc *config.ServerConfig) {
	var parent SystemMenu
	err := Db.Where("name = ?", "JenkinsManagement").First(&parent).Error
	if err != nil || parent.ID == 0 {
		return
	}

	var menu SystemMenu
	err = Db.Where("name = ?", "JenkinsPipelineManagement").First(&menu).Error
	if err == nil && menu.ID > 0 {
		if menu.Title != "流水线配置" {
			_ = Db.Model(&menu).Update("title", "流水线配置").Error
		}
		return
	}

	newMenu := &SystemMenu{
		Name:      "JenkinsPipelineManagement",
		Title:     "流水线配置",
		Icon:      "ant-design:branches-outlined",
		Type:      "1",
		Show:      "1",
		OrderNo:   79,
		Component: "cicd/pipeline/index",
		Path:      "pipeline",
		Pid:       int(parent.ID),
	}
	if err := Db.Create(newMenu).Error; err == nil {
		sc.Logger.Info("自动插入菜单：[Jenkins] 流水线配置 成功 🚀")
		var superAdminRole SystemRole
		if Db.Where("role_name = ?", "超级管理员").First(&superAdminRole).Error == nil {
			_ = Db.Model(&superAdminRole).Association("Menus").Append(newMenu)
		}
	}
	SeedJenkinsPipelinePresets(sc)
}

func SeedJenkinsPipelinePresets(sc *config.ServerConfig) {
	var stageCount int64
	Db.Model(&JenkinsStage{}).Count(&stageCount)
	if stageCount == 0 {
		stages := []*JenkinsStage{
			{Name: "获取代码", CodeKey: "stg-checkout", Category: "frontend", AgentType: "none", Steps: "cleanWs()\ncheckout scmGit(branches: [[name: \"${params.分支名}\"]], userRemoteConfigs: [[url: \"${GIT仓库}\"]])", Description: "清空工作区并获取 Git 仓库分支源码", OrderNo: 1, Enabled: true},
			{Name: "编译代码", CodeKey: "stg-build", Category: "frontend", AgentType: "docker", DockerImage: "harbor.cathayquantum.net/devops/node:${params.NODE版本}", Steps: "script {\n    sh \"\"\"\n    cd ${WORK_SPACES}\n    ${构建命令}\n    \"\"\"\n}", Description: "Docker Node 容器编译前端静态产物", OrderNo: 2, Enabled: true},
			{Name: "代码漏扫", CodeKey: "stg-sonar", Category: "frontend", AgentType: "none", WhenExpr: "expression { 扫描代码 == 'true' }", Steps: `script {
    ID = sh returnStdout: true, script: "git rev-parse HEAD"
    echo "${ID}"
    withSonarQubeEnv(credentialsId: 'sonar-token') {
        sh """
            /opt/sonar-scanner/bin/sonar-scanner \
            -Dsonar.token=sqa_b0ea8ca78578f35f6b097a3c86a42874404eb6ef -Dsonar.projectVersion=${params.分支名} \
            -Dsonar.projectKey=${JOB_BASE_NAME} \
            -Dsonar.projectName=${JOB_BASE_NAME} \
            -Dsonar.sources=src \
            -Dsonar.exclusions=**/node_modules/**,**/dist/**,**/build/**,**/coverage/** \
            -Dsonar.javascript.file.suffixes=.js,.jsx,.ts,.tsx,.vue \
            -Dsonar.typescript.tsconfigPath=tsconfig.json \
            -Dsonar.sourceEncoding=UTF-8 \
            -Dsonar.branch.name=${params.分支名}
        """
    }
}`, Description: "SonarQube 静态代码质量与安全性检测", OrderNo: 3, Enabled: true},
			{Name: "生成镜像tag", CodeKey: "stg-tag", Category: "frontend", AgentType: "none", Steps: `script {
    env.COMMITID = sh(returnStdout: true, script: "git log -n 1 --pretty=format:'%h'").trim()
    env.COMMIT_LOG = sh(returnStdout: true, script: "git log -n 1 --pretty=format:'%cn：%s'").trim()
    env.BUILDTIME = sh(returnStdout: true, script: "date +%Y%m%d_%H%M%S").trim()
    env.IMAGE_TAG = COMMITID + "_" + BUILDTIME
}`, Description: "提取 Git Commit 与时间戳生成 IMAGE_TAG", OrderNo: 4, Enabled: true},
			{Name: "构建容器镜像", CodeKey: "stg-docker", Category: "frontend", AgentType: "none", Steps: `script {
    withCredentials([usernamePassword(credentialsId: 'art', usernameVariable: 'ART_USER', passwordVariable: 'ART_PASS')]) {
        sh '''
            curl -u "$ART_USER:$ART_PASS" -O https://artifactory.cathayquantum.net/artifactory/huahua/nginx.conf
            curl -u "$ART_USER:$ART_PASS" -O https://artifactory.cathayquantum.net/artifactory/huahua/Dockerfile
        '''
    }
}
withCredentials([usernamePassword(credentialsId: 'harbor-local', passwordVariable: 'PASSWORD', usernameVariable: 'USERNAME')]) {
    sh 'echo "${PASSWORD}" | docker login ${HARBOR_URL} -u "${USERNAME}" --password-stdin'
    sh 'cd ${WORK_SPACES} && docker build -t ${HARBOR_URL}/${JOB_NAME}:${IMAGE_TAG} . --push'
    sh 'docker rmi ${HARBOR_URL}/${JOB_NAME}:${IMAGE_TAG}'
}`, Description: "打包 Docker 镜像并推送至 Harbor 镜像仓库", OrderNo: 5, Enabled: true},
			{Name: "启动前端容器", CodeKey: "stg-deploy", Category: "frontend", AgentType: "none", Steps: `script {
    def hosts = params.目标主机.split(',')
    for (host in hosts) {
        def ip = host.split('【')[0]
        echo ">>>>执行主机: ${ip}"
        sh """
            export ANSIBLE_PYTHON_INTERPRETER=/usr/bin/python3.12
            ansible all -m shell -a "docker pull ${HARBOR_URL}/${JOB_NAME}:${IMAGE_TAG}" -i ${ip},
            ansible all -m shell -a "docker rm -f ${JOB_BASE_NAME}" -i ${ip},
            ansible all -m shell -a "docker run -d --name ${JOB_BASE_NAME} -p82:80 ${HARBOR_URL}/${JOB_NAME}:${IMAGE_TAG}" -i ${ip},
        """
    }
}`, Description: "Ansible 批量拉取镜像并部署运行容器", OrderNo: 6, Enabled: true},
		}
		for _, s := range stages {
			_ = Db.Create(s).Error
		}
		sc.Logger.Info("预置数据插入：[JenkinsStage] 6 大标准 Stage 模块注入成功 🚀")
	}

	var envCount int64
	Db.Model(&JenkinsEnvVar{}).Count(&envCount)
	if envCount == 0 {
		envs := []*JenkinsEnvVar{
			{EnvGroup: "global", Key: "WORK_SPACES", Value: "${WORKSPACE}", Description: "工作区目录路径"},
			{EnvGroup: "global", Key: "HARBOR_URL", Value: "harbor.cathayquantum.net", Description: "Harbor 镜像仓库域名"},
			{EnvGroup: "global", Key: "ANSIBLE_FORCE_COLOR", Value: "true", Description: "Ansible 染色输出选项"},
		}
		for _, e := range envs {
			_ = Db.Create(e).Error
		}
		sc.Logger.Info("预置数据插入：[JenkinsEnvVar] 环境变量注入成功 🚀")
	}

	var paramCount int64
	Db.Model(&JenkinsBuildParam{}).Count(&paramCount)
	if paramCount == 0 {
		params := []*JenkinsBuildParam{
			{Name: "扫描代码", Type: "boolean", DefaultValue: "false", Description: "是否对分支代码质量扫描、安全性静态分析"},
			{Name: "分支名", Type: "reactiveChoice", Script: `def gettags = ("git ls-remote -t -h ssh://git@gitlab.cathayquantum.net:222/centurypay/centurypay_admin.git" ).execute()
def repoNameList=gettags.text.readLines().collect {it.split()[1].replaceAll('refs/heads/', '').replaceAll('refs/tags/', '')}
repoNameList.add(0, 'release:selected')
return repoNameList`, Description: "动态 Git 远程分支与 Tag 列表"},
			{Name: "GIT仓库", Type: "string", DefaultValue: "ssh://git@gitlab.cathayquantum.net:222/centurypay/centurypay_admin.git", Description: "Git 源代码仓库地址"},
			{Name: "构建节点", Type: "choice", Choices: []string{"master", "ec2-jp", "ec2-hk"}, Description: "选择 Jenkins 构建 Node 节点"},
			{Name: "NODE版本", Type: "choice", Choices: []string{"16"}, Description: "Node.js 编译版本"},
			{Name: "构建命令", Type: "string", DefaultValue: "npm install --prefer-offline --registry=https://registry.npmmirror.com/ --loglevel=error && npm run build:prod -- --silent", Description: "前端编译构建指令"},
			{Name: "构建产出目录", Type: "string", DefaultValue: "dist", Description: "构建产物产出目录名称"},
			{Name: "目标主机", Type: "choice", Choices: []string{"172.30.12.255"}, Description: "172.30.12.255 --- 生产目标主机"},
		}
		for _, p := range params {
			_ = Db.Create(p).Error
		}
		sc.Logger.Info("预置数据插入：[JenkinsBuildParam] 构建参数注入成功 🚀")
	}
}

func EnsureAccountSettingMenu(sc *config.ServerConfig) {
	var parent SystemMenu
	err := Db.Where("name = ?", "System").First(&parent).Error
	if err != nil || parent.ID == 0 {
		return
	}

	var menu SystemMenu
	err = Db.Where("name = ?", "AccountSetting").First(&menu).Error
	if err != nil || menu.ID == 0 {
		menu = SystemMenu{
			Name:      "AccountSetting",
			Title:     "个人设置",
			Icon:      "ant-design:user-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   97,
			Component: "system/account/setting/index",
			Path:      "accountSetting",
			Pid:       int(parent.ID),
		}
		if createErr := Db.Create(&menu).Error; createErr != nil {
			return
		}
		sc.Logger.Info("自动插入菜单：[系统管理] 个人设置 成功 🚀")
	} else if menu.Title != "个人设置" || menu.Component != "system/account/setting/index" {
		_ = Db.Model(&menu).Updates(map[string]interface{}{
			"title":     "个人设置",
			"component": "system/account/setting/index",
			"path":      "accountSetting",
		})
	}

	// 确保所有角色均已关联个人设置菜单
	var roles []SystemRole
	if Db.Find(&roles).Error == nil {
		for _, r := range roles {
			_ = Db.Model(&r).Association("Menus").Append(&menu)
		}
	}
}

func EnsureAuditLogMenu(sc *config.ServerConfig) {
	var parent SystemMenu
	err := Db.Where("name = ?", "System").First(&parent).Error
	if err != nil || parent.ID == 0 {
		return
	}

	var menu SystemMenu
	err = Db.Where("name = ?", "AuditManagement").First(&menu).Error
	if err != nil || menu.ID == 0 {
		menu = SystemMenu{
			Name:      "AuditManagement",
			Title:     "日志审计",
			Icon:      "ant-design:security-scan-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   98,
			Component: "system/audit/index",
			Path:      "audit",
			Pid:       int(parent.ID),
		}
		if createErr := Db.Create(&menu).Error; createErr != nil {
			return
		}
		sc.Logger.Info("自动插入菜单：[系统管理] 日志审计 成功 🚀")
	} else if menu.Title != "日志审计" || menu.Component != "system/audit/index" || menu.Path != "audit" {
		_ = Db.Model(&menu).Updates(map[string]interface{}{
			"title":     "日志审计",
			"icon":      "ant-design:security-scan-outlined",
			"type":      "1",
			"show":      "1",
			"order_no":  98,
			"component": "system/audit/index",
			"path":      "audit",
			"pid":       int(parent.ID),
		})
	}

	// 确保需要的 API 接口存在
	apisToAdd := []SystemApi{
		{Path: "/api/system/getAuditLogList", Method: "GET", Title: "[审计模块]获取操作审计日志列表", Type: "1"},
		{Path: "/api/system/getOnlineUserList", Method: "GET", Title: "[用户模块]获取在线用户列表", Type: "1"},
		{Path: "/api/system/kickoutUser", Method: "POST", Title: "[用户模块]强退指定在线用户", Type: "1"},
	}

	var createdApis []*SystemApi
	for _, apiItem := range apisToAdd {
		var existingApi SystemApi
		if err := Db.Where("path = ? AND method = ?", apiItem.Path, apiItem.Method).First(&existingApi).Error; err != nil || existingApi.ID == 0 {
			newApi := apiItem
			if createApiErr := Db.Create(&newApi).Error; createApiErr == nil {
				createdApis = append(createdApis, &newApi)
			}
		} else {
			createdApis = append(createdApis, &existingApi)
		}
	}

	// 确保所有角色均关联日志审计菜单与相关 API
	var roles []SystemRole
	if Db.Find(&roles).Error == nil {
		for _, r := range roles {
			_ = Db.Model(&r).Association("Menus").Append(&menu)
			for _, api := range createdApis {
				_ = Db.Model(&r).Association("Apis").Append(api)
				if CasbinEnforcer != nil && r.RoleValue != "" {
					_, _ = CasbinEnforcer.AddPolicy(r.RoleValue, api.Path, api.Method)
				}
			}
		}
	}
}

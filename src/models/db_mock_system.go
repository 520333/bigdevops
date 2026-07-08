package models

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"fmt"

	"go.uber.org/zap"
)

func mockSystemData(sc *config.ServerConfig) *User {
	menus := []*Menu{
		{Name: "System", Title: "系统管理", Icon: "ant-design:setting-outlined", Type: "0", Show: "1", OrderNo: 90, Component: "LAYOUT", Redirect: "/system/account", Path: "/system"},
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

		{Name: "PrometheusMonitor", Title: "监控中心", Icon: "ant-design:dashboard-outlined", Type: "0", Show: "1", OrderNo: 40, Component: "LAYOUT", Path: "/monitor", Redirect: "/monitor/scrape/index"},
		{Name: "MonitorPool", Title: "采集池管理", Icon: "ant-design:database-outlined", Type: "1", Show: "1", OrderNo: 41, Component: "monitor/pool/index", Pid: 21, Path: "pool"},
		{Name: "MonitorScrapeJob", Title: "采集任务管理", Icon: "ant-design:api-outlined", Type: "1", Show: "1", OrderNo: 42, Component: "monitor/scrape/index", Pid: 21, Path: "scrape"},
	}

	apis := []*Api{
		{Path: "/api/system/menu", Method: "GET", Title: "系统管理-菜单相关", Type: "0"},
		{Path: "/api/*", Method: "ALL", Title: "api的所有的权限", Type: "0"},
		{Path: "/api/system/getMenuList", Method: "GET", Pid: 1, Title: "系统管理-根据用户获取菜单", Type: "1"},
		{Path: "/api/system/getMenuListAll", Method: "GET", Pid: 1, Title: "系统管理-获取用户全量菜单", Type: "1"},
		{Path: "/api/system/updateMenu", Method: "POST", Pid: 1, Title: "系统管理-修改菜单", Type: "1"},
		{Path: "/api/system/createMenu", Method: "POST", Pid: 1, Title: "系统管理-创建菜单", Type: "1"},
		{Path: "/api/system/deleteMenu/:id", Method: "DELETE", Pid: 1, Title: "系统管理-删除菜单", Type: "1"},
		{Path: "/api/getUserInfo", Method: "GET", Pid: 1, Title: "获取用户信息", Type: "1"},
		{Path: "/api/getPermCode", Method: "GET", Pid: 1, Title: "获得用户code", Type: "1"},
		{Path: "/api/system/getAccountList", Method: "GET", Pid: 1, Title: "获取用户列表", Type: "1"},
		{Path: "/api/*", Method: "GET", Pid: 2, Title: "所有api GET权限", Type: "1"},
		{Path: "/api/*", Method: "POST", Pid: 2, Title: "所有api POST权限", Type: "1"},
		{Path: "/api/*", Method: "DELETE", Pid: 2, Title: "所有api DELETE权限", Type: "1"},
		{Path: "/api/*", Method: "PATCH", Pid: 2, Title: "所有api PATCH权限", Type: "1"},
		{Path: "/api/*", Method: "HEAD", Pid: 2, Title: "所有api HEAD权限", Type: "1"},
		{Path: "/api/*", Method: "OPTIONS", Pid: 2, Title: "所有api OPTIONS权限", Type: "1"},
		{Path: "/api/*", Method: "CONNECT", Pid: 2, Title: "所有api CONNECT权限", Type: "1"},
		{Path: "/api/*", Method: "TRACE", Pid: 2, Title: "所有api TRACE权限", Type: "1"},
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

	_ = Db.Create(apis)

	users := []string{"蟹老板", "珊迪", "章鱼哥", "皮老板", "小窝"}
	num := 5
	for i := 0; i < num; i++ {
		mockUser := &User{
			Username:     fmt.Sprintf("mock%d", i),
			Password:     common.BcryptHash("123456"),
			RealName:     users[i],
			FeiShuUserId: "b75ag4g4",
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

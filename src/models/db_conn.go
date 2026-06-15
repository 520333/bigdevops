package models

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"fmt"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/util"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	Db             *gorm.DB
	CasbinEnforcer *casbin.Enforcer
)

func InitDb(sc *config.ServerConfig) error {
	db, err := gorm.Open(mysql.Open(sc.MysqlC.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}
	Db = db

	// 这里初始化casbin

	return nil
}

func InitCasBin(sc *config.ServerConfig) error {
	a, err := gormadapter.NewAdapterByDB(Db)
	if err != nil {
		sc.Logger.Error("InitCasBin error", zap.Error(err))
		fmt.Printf("数据库初始化错误：%v \n", err)
		return err
	}
	// 初始化模型
	modelText := `
		[request_definition]
		r = sub, obj, act
		
		[policy_definition]
		p = sub, obj, act
		
		[role_definition]
		g = _, _
		
		[policy_effect]
		e = some(where (p.eft == allow))
		
		[matchers]
		m = r.sub == p.sub && KeyMatch2(r.obj, p.obj) && r.act == p.act
		`
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		sc.Logger.Error("casbin字符串加载模型失败", zap.Error(err))
		return err
	}
	casbinEnforcer, err := casbin.NewEnforcer(m, a)
	if err != nil {
		sc.Logger.Error("casbin创建Enforcer失败", zap.Error(err))
		return err
	}

	casbinEnforcer.AddFunction("KeyMatch2", func(args ...interface{}) (interface{}, error) {
		name1 := args[0].(string)
		name2 := args[1].(string)
		return bool(util.KeyMatch2(name1, name2)), nil
	})

	CasbinEnforcer = casbinEnforcer
	//_ = CasbinEnforcer.LoadPolicy()
	return nil
}

func MigrateTable() error {
	return Db.AutoMigrate(
		&User{},
		&Role{},
		&Menu{},
		&Api{},
		&StreeNode{},
		&ResourceEcs{},
		&ResourceElb{},
		&ResourceRds{},
		&ResourceDns{},
		&WorkOrderProcess{},
		&WorkOrderFlowNode{},
		&WorkOrderFormDesign{},
		&WorkOrderTemplate{},
		&WorkOrderInstance{},
	)
}

func MockUserRegister(sc *config.ServerConfig) {
	var count int64
	err := Db.Model(&User{}).Where("username = ?", "admin").Count(&count).Error
	if err == nil && count > 0 {
		sc.Logger.Info("检测到数据库已完成初始化，跳过 Mock 数据注入 🛡️")
		return // 💡 直接返回，不执行后面的任何 Create 代码
	}
	menus := []*Menu{
		{
			Name:      "System",
			Title:     "系统管理",
			Icon:      "ant-design:setting-outlined",
			Type:      "0",
			Show:      "1",
			OrderNo:   90,
			Component: "LAYOUT",
			Redirect:  "/system/account",
			Path:      "/system",
		},
		{
			Name:      "MenuManagement",
			Title:     "菜单管理",
			Icon:      "ant-design:menu-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   91,
			Component: "system/menu/index",
			Pid:       1,
			Path:      "menu",
		},
		{
			Name:      "AccountManagement",
			Title:     "用户管理",
			Icon:      "ant-design:user-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   92,
			Component: "system/account/index",
			Pid:       1,
			Path:      "account",
		},
		{
			Name:      "RoleManagement",
			Title:     "角色管理",
			Icon:      "ant-design:solution-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   93,
			Component: "system/role/index",
			Pid:       1,
			Path:      "role",
		},
		{
			Name:      "ChangePassword",
			Title:     "修改密码",
			Icon:      "ant-design:key-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   94,
			Component: "system/password/index",
			Pid:       1,
			Path:      "changePassword",
		},
		{
			Name:      "ApiManagement",
			Title:     "接口授权",
			Icon:      "ant-design:api-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   95,
			Component: "system/api/index",
			Pid:       1,
			Path:      "api",
		},
		{
			Name:      "PermissionManagement",
			Title:     "权限管理",
			Icon:      "ion:layers-outline",
			Type:      "0",
			Show:      "1",
			OrderNo:   100,
			Component: "LAYOUT",
			//ParentMenu: "2",
			Redirect: "/permission/front/page",
			Path:     "/permission",
		},
		{
			Name:      "PermissionFront",
			Title:     "前端权限管理",
			Icon:      "ion:layers-outline",
			Type:      "1",
			Show:      "1",
			OrderNo:   101,
			Component: "/permission/front/index",
			Pid:       7,
			Path:      "front",
		},

		{
			Name:      "ServiceTree",
			Title:     "服务树与CMDB",
			Icon:      "ant-design:database-outlined",
			Type:      "0",
			Show:      "1",
			OrderNo:   10,
			Component: "LAYOUT",
			Path:      "/serviceTree",
			Redirect:  "/serviceTree/service/index",
		},
		//{
		//	Name:      "ServiceTreeIndex",
		//	Title:     "服务树",
		//	Icon:      "ant-design:cluster-outlined",
		//	Type:      "1",
		//	Show:      "1",
		//	OrderNo:   19,
		//	Component: "stree/stree/index",
		//	Pid:       9,
		//	Path:      "stree",
		//},
		{
			Name:      "ServiceTreeIndexAsync",
			Title:     "服务树",
			Icon:      "ant-design:node-index-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   11,
			Component: "stree/stree/indexAsync",
			Pid:       9,
			Path:      "streeAsync",
		},
		{
			Name:      "WorkOrder",
			Title:     "工单服务",
			Icon:      "ant-design:reconciliation-outlined",
			Type:      "0",
			Show:      "1",
			OrderNo:   20,
			Component: "LAYOUT",
			Path:      "/workOrder",
			Redirect:  "/workOrder/process/index",
		},
		{
			Name:      "ProcessManagement",
			Title:     "审批流程管理",
			Icon:      "ant-design:apartment-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   21,
			Component: "workorder/process/index",
			Pid:       11,
			Path:      "process",
		},
		{
			Name:      "FormManagement",
			Title:     "表单设计管理",
			Icon:      "ant-design:form-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   22,
			Component: "workorder/formDesign/index",
			Pid:       11,
			Path:      "formDesign",
		},
		{
			Name:      "WorkOrderTemplateManagement",
			Title:     "工单模板管理",
			Icon:      "ant-design:layout-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   23,
			Component: "workorder/template/index",
			Pid:       11,
			Path:      "template",
		},
		{
			Name:      "WorkOrderTicket",
			Title:     "工单申请",
			Icon:      "ant-design:profile-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   24,
			Component: "workorder/ticket/index",
			Pid:       11,
			Path:      "ticket",
		},
		{
			Name:      "WorkOrderCreate",
			Title:     "工单填写",
			Icon:      "ant-design:form-outlined",
			Type:      "1",
			Show:      "0",
			OrderNo:   25,
			Component: "workorder/ticket/create",
			Pid:       11,
			Path:      "create",
		},
		{
			Name:      "WorkOrderSearch",
			Title:     "我的工单",
			Icon:      "ant-design:profile-outlined",
			Type:      "1",
			Show:      "1",
			Component: "workorder/ticket/search", // 对应你的列表页
			Pid:       11,
			Path:      "search",
		},
		//{
		//	Name:      "WorkOrderDetail",
		//	Title:     "工单详情",
		//	Icon:      "ant-design:profile-outlined",
		//	Type:      "1",
		//	Show:      "0",
		//	Component: "workorder/detail/index", // 对应你的列表页
		//	Pid:       11,
		//	Path:      "detail",
		//},
	}
	apis := []*Api{
		{
			Path:   "/api/system/menu",
			Method: "GET",
			Title:  "系统管理-菜单相关",
			Type:   "0",
		},
		{
			Path:   "/api/*",
			Method: "ALL",
			Title:  "api的所有的权限",
			Type:   "0",
		},
		{
			Path:   "/api/system/getMenuList",
			Method: "GET",
			Pid:    1,
			Title:  "系统管理-根据用户获取菜单",
			Type:   "1",
		},
		{
			Path:   "/api/system/getMenuListAll",
			Method: "GET",
			Pid:    1,
			Title:  "系统管理-获取用户全量菜单",
			Type:   "1",
		},
		{
			Path:   "/api/system/updateMenu",
			Method: "POST",
			Pid:    1,
			Title:  "系统管理-获取用户获取菜单",
			Type:   "1",
		},
		{
			Path:   "/api/system/createMenu",
			Method: "POST",
			Pid:    1,
			Title:  "系统管理-创建菜单",
			Type:   "1",
		},
		{
			Path:   "/api/system/deleteMenu/:id",
			Method: "DELETE",
			Pid:    1,
			Title:  "系统管理-删除菜单",
			Type:   "1",
		},
		{
			Path:   "/api/getUserInfo",
			Method: "GET",
			Pid:    1,
			Title:  "获取用户信息",
			Type:   "1",
		},
		{
			Path:   "/api/getPermCode",
			Method: "GET",
			Pid:    1,
			Title:  "获得用户code",
			Type:   "1",
		},
		{
			Path:   "/api/system/getAccountList",
			Method: "GET",
			Pid:    1,
			Title:  "获取用户列表",
			Type:   "1",
		},
		{
			Path:   "/api/*",
			Method: "GET",
			Pid:    2,
			Title:  "所有api GET权限",
			Type:   "1",
		},
		{
			Path:   "/api/*",
			Method: "POST",
			Pid:    2,
			Title:  "所有api POST权限",
			Type:   "1",
		},
		{
			Path:   "/api/*",
			Method: "DELETE",
			Pid:    2,
			Title:  "所有api DELETE权限",
			Type:   "1",
		},
		{
			Path:   "/api/*",
			Method: "PATCH",
			Pid:    2,
			Title:  "所有api PATCH权限",
			Type:   "1",
		},
		{
			Path:   "/api/*",
			Method: "HEAD",
			Pid:    2,
			Title:  "所有api HEAD权限",
			Type:   "1",
		},
		{
			Path:   "/api/*",
			Method: "OPTIONS",
			Pid:    2,
			Title:  "所有api OPTIONS权限",
			Type:   "1",
		},
		{
			Path:   "/api/*",
			Method: "CONNECT",
			Pid:    2,
			Title:  "所有api CONNECT权限",
			Type:   "1",
		},
		{
			Path:   "/api/*",
			Method: "TRACE",
			Pid:    2,
			Title:  "所有api TRACE权限",
			Type:   "1",
		},
	}

	for _, menu := range menus {
		menu := menu
		err := Db.Create(&menu).Error
		if err != nil {
			fmt.Printf("创建menu错误:%v\n", err)
		}
	}
	u1 := User{
		Username: "admin",
		Password: common.BcryptHash("tingbao89.."),
		RealName: "超管",
		//Avatar:   "",
		Desc:     "",
		HomePath: "/system/role",
		Enable:   1,
		Roles: []*Role{
			{
				RoleName:  "超级管理员",
				RoleValue: "super",
				Menus:     menus,
			},
			//{
			//	RoleName:  "前端管理员",
			//	RoleValue: "frontAdmin",
			//	//Menus:     menus,
			//},
		},
	}

	u2 := User{
		Username: "test",
		Password: "123456",
		RealName: "测试",
		Desc:     "",
		HomePath: "/system/role",
		Enable:   1,
		Roles: []*Role{
			{
				RoleName:  "前端管理员",
				RoleValue: "frontAdmin",
			},
		},
	}
	u3 := User{
		Username: sc.WorkOrderAutoActionC.ServiceAccount,
		Password: "123456",
		RealName: "自动工单执行机器人",
		Desc:     "",
		HomePath: "/system/role",
		Enable:   1,
		Roles: []*Role{
			{
				RoleName:  "集群超级管理员",
				RoleValue: "bot_super",
				Menus:     menus,
			},
		},
	}

	//u1.Password = common.BcryptHash(u1.Password)
	//u2.Password = common.BcryptHash(u2.Password)
	//u3.Password = common.BcryptHash(u3.Password)
	if err := Db.Create(&u1).Error; err != nil {
		sc.Logger.Error("模拟用户注册失败", zap.Any("错误", err.Error()))
		//return
	}
	if err := Db.Create(&u2).Error; err != nil {
		sc.Logger.Error("模拟用户注册失败", zap.Any("错误", err.Error()))
		//return
	}
	if err := Db.Create(&u3).Error; err != nil {
		sc.Logger.Error("模拟用户注册失败", zap.Any("错误", err.Error()))
		//return
	}
	// 新增apis
	Db.Create(apis)
	// 查询一下 super这个roles
	dbRole, _ := GetRoleByRoleValue("super")
	dbRole.Apis = apis
	err = dbRole.UpdateApis(apis)
	sc.Logger.Info("更新api结果", zap.Any("err", err))

	sc.Logger.Info("模拟用户注册成功")

	// 往casbin mock一些默认的规则

	for _, api := range apis {
		api := api
		_, err := CasbinEnforcer.AddPolicy("super", api.Path, api.Method)
		if err != nil {
			sc.Logger.Error("给super角色绑定角色失败",
				zap.Any("err", err),
				zap.String("api", api.Path),
				zap.String("title", api.Title),
				zap.String("method", api.Method),
			)
		}

	}

	streeNodes := []*StreeNode{
		{
			Title:     "TORKEY",
			Desc:      "拓基时代-国际事业",
			Pid:       0,
			Level:     1,
			OpsAdmins: []*User{&u1},
		},
		{
			Title: "研发一组",
			Pid:   1,
			Level: 2,
			Desc:  "umipay、binance、upipay项目",
		},
		{
			Title:  "umipay项目",
			Pid:    2,
			Level:  2,
			IsLeaf: true,
			Desc:   "umipay项目",
		},
		{
			Title:  "binance项目",
			Pid:    2,
			Level:  2,
			IsLeaf: true,
			Desc:   "binance项目",
		},
		{
			Title:  "upipay项目",
			Pid:    2,
			Level:  2,
			IsLeaf: true,
			Desc:   "upipay项目",
		},
		{
			Title: "研发二组",
			Pid:   1,
			Level: 2,
			Desc:  "发卡项目",
		},
		{
			Title:  "花花卡项目",
			Pid:    6,
			Level:  2,
			IsLeaf: true,
			Desc:   "花花卡项目",
		},
		{
			Title: "研发三组",
			Pid:   1,
			Level: 2,
			Desc:  "资管项目",
		},
		{
			Title:  "资管项目",
			Pid:    8,
			Level:  2,
			IsLeaf: true,
			Desc:   "资管项目",
		},
	}
	for _, node := range streeNodes {
		node := node
		node.CreateOne()
		//// 执行创建
		//if err := Db.Create(node).Error; err != nil {
		//	sc.Logger.Error("Mock服务树节点失败", zap.String("title", node.Title), zap.Error(err))
		//	continue
		//}
		//
		//// 💡 顺便把之前写好的 Key 和 NodePath 初始化进去
		//node.Key = node.ID
		//_ = node.GetFullNodePath() // 调用你写的递归方法计算路径
		//Db.Updates(node)           // 更新 NodePath 到数据库
	}

	// ==========================================
	// 🌟 开始 Mock 工单系统核心数据 (表单、流程、模板)
	// ==========================================

	// 1. Mock 动态表单 (FormDesign)
	forms := []*WorkOrderFormDesign{
		{
			Name:   "基础资源申请表单",
			UserID: u1.ID,
			// 模拟 Vben Admin Form 的 JSON 配置结构
			FormConfig: `{"schemas":[{"field":"resourceName","label":"资源名称","component":"Input","required":true},{"field":"reason","label":"申请原因","component":"InputTextArea","required":true}]}`,
		},
		{
			Name:       "权限开通申请表单",
			UserID:     u1.ID,
			FormConfig: `{"schemas":[{"field":"systemName","label":"系统名称","component":"Input","required":true},{"field":"roleName","label":"需要开通的角色","component":"Input","required":true},{"field":"expireTime","label":"过期时间","component":"DatePicker","required":false}]}`,
		},
	}

	for _, form := range forms {
		// 注意：根据你 model 里的定义，如果 CreateOne 没有处理好 ID 回填，
		// 这里直接用 Db.Create(&form) 可以确保自增 ID 准确挂载回 struct 上
		if err := Db.Create(form).Error; err != nil {
			sc.Logger.Error("Mock FormDesign 失败", zap.Error(err))
		}
	}

	// 2. Mock 审批流程 (Process)
	processes := []*WorkOrderProcess{
		{
			Name:   "基础直线审批",
			UserID: u1.ID,
			FlowNodes: []WorkOrderFlowNode{
				{Type: "起始节点", DefineUserOrGroup: "test"},
				{Type: "审批节点", DefineUserOrGroup: "admin"}, // 直接指定用户
				{Type: "结束节点", DefineUserOrGroup: "test"},
			},
		},
		{
			Name:   "带执行的标准流程",
			UserID: u1.ID,
			FlowNodes: []WorkOrderFlowNode{
				{Type: "起始节点", DefineUserOrGroup: "test"},
				{Type: "审批节点", DefineUserOrGroup: "组@超级管理员"}, // 指定由超级管理员组审批
				{Type: "执行节点", DefineUserOrGroup: "admin"},
				{Type: "结束节点", DefineUserOrGroup: "bot_super"},
			},
		},
	}

	for _, p := range processes {
		// 如果 p.CreateOne() 有完善的级联创建逻辑也可以用 p.CreateOne()
		if err := Db.Create(p).Error; err != nil {
			sc.Logger.Error("Mock Process 失败", zap.Error(err))
		}
	}

	// 3. Mock 工单模板 (WorkOrderTemplate) -> 将表单和流程绑定
	templates := []*WorkOrderTemplate{
		{
			Name:         "【测试】通用资源申请",
			UserID:       u1.ID,
			FormDesignID: forms[0].ID,     // 绑定第一个表单
			ProcessID:    processes[0].ID, // 绑定第一个流程
		},
		{
			Name:         "【生产】核心系统权限申请",
			UserID:       u1.ID,
			FormDesignID: forms[1].ID,     // 绑定第二个表单
			ProcessID:    processes[1].ID, // 绑定第二个流程
		},
	}

	for _, tmpl := range templates {
		if err := Db.Create(tmpl).Error; err != nil {
			sc.Logger.Error("Mock WorkOrderTemplate 失败", zap.Error(err))
		}
	}

	sc.Logger.Info("工单系统 (表单、流程、模板) Mock 数据初始化完成 🚀")

	// ==========================================
	// 🤖 补充 Mock: 自动执行专属工单数据
	// ==========================================

	// 1. 自动执行表单
	autoForm := &WorkOrderFormDesign{
		Name:       "自动购买ECS资源表单",
		UserID:     u1.ID,
		FormConfig: `{"schemas":[{"field":"HostNames","label":"主机名(多行换行)","component":"InputTextArea","required":true}]}`,
	}
	Db.Create(autoForm)

	// 2. 自动执行流程 (把执行节点指派给机器人)
	autoProcess := &WorkOrderProcess{
		Name:   "全自动ECS交付流程",
		UserID: u1.ID,
		FlowNodes: []WorkOrderFlowNode{
			{Type: "起始节点", DefineUserOrGroup: "test"},
			// 🚨 这里直接指派给机器人的 Username
			{Type: "执行节点", DefineUserOrGroup: sc.WorkOrderAutoActionC.ServiceAccount},
			{Type: "结束节点", DefineUserOrGroup: "test"},
		},
	}
	Db.Create(autoProcess)

	// 3. 自动执行模板 (模板名字必须与配置文件中的 AutoTemplateNameBuyEcs 完全一致)
	autoTemplate := &WorkOrderTemplate{
		Name:         sc.WorkOrderAutoActionC.AutoTemplateNameBuyEcs,
		UserID:       u1.ID,
		FormDesignID: autoForm.ID,
		ProcessID:    autoProcess.ID,
	}
	Db.Create(autoTemplate)
}

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
		&Process{},
		&FlowNode{},
	)
}

func MockUserRegister(sc *config.ServerConfig) {
	//ecss := []*ResourceEcs{
	//	{
	//		InstanceId:        " i-rj9fona9oz6au9sju2wi",
	//		InstanceName:      "launch-advisor-20260415",
	//		InstanceType:      "",
	//		VpcId:             "",
	//		OSType:            "",
	//		ZoneId:            "",
	//		Status:            "",
	//		Cpu:               0,
	//		Memory:            0,
	//		OSName:            "",
	//		Description:       "",
	//		ImageId:           "",
	//		HostName:          "",
	//		SecurityGroupIds:  StringArray{"安全组1", "安全组2"},
	//		PrivateIpAddress:  nil,
	//		PublicIpAddresses: nil,
	//		NetworkInterfaces: nil,
	//		DiskIds:           nil,
	//	},
	//}
	//for _, ecs := range ecss {
	//	ecs := ecs
	//	err := ecs.CreateOne()
	//	if err != nil {
	//		sc.Logger.Error("创建ecs错误", zap.Error(err))
	//	}
	//
	//}
	// 查询
	//ecs, err := GetResourceEcsAll()
	//for _, ecs := range ecss {
	//	ecs := ecs
	//	if err != nil {
	//		sc.Logger.Error("主机", zap.Any("主机名", ecs.InstanceId),
	//			zap.Any("", ecs.InstanceName),
	//		)
	//	}
	//
	//}

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
			Title:     "流程管理",
			Icon:      "ant-design:branches-outlined",
			Type:      "1",
			Show:      "1",
			OrderNo:   21,
			Component: "workorder/process/index",
			Pid:       11,
			Path:      "process",
		},
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
		Password: "tingbao89..",
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

	u1.Password = common.BcryptHash(u1.Password)
	u2.Password = common.BcryptHash(u2.Password)
	if err := Db.Create(&u1).Error; err != nil {
		sc.Logger.Error("模拟用户注册失败", zap.Any("错误", err.Error()))
		//return
	}
	if err := Db.Create(&u2).Error; err != nil {
		sc.Logger.Error("模拟用户注册失败", zap.Any("错误", err.Error()))
		//return
	}
	// 新增apis
	Db.Create(apis)
	// 查询一下 super这个roles
	dbRole, _ := GetRoleByRoleValue("super")
	dbRole.Apis = apis
	err := dbRole.UpdateApis(apis)
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

}

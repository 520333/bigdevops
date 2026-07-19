package models

import (
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

func InitDb(dsn string) error {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
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
		# m = r.sub == p.sub && KeyMatch2(r.obj, p.obj) && r.act == p.act
		# ====== 核心修改点：允许 p.act 为 ALL 或 * ======
		m = r.sub == p.sub && KeyMatch2(r.obj, p.obj) && (r.act == p.act || p.act == "ALL")
		# m = r.sub == p.sub && KeyMatch2(r.obj, p.obj) && (r.act == p.act || p.act == "ALL" || p.act == "*")
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
		&SystemSetting{},

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

		&JobScript{},
		&JobTask{},
		&JobResult{},

		&MonitorPromScrapePool{},
		&MonitorPromScrapeJob{},
		&MonitorPromAlertRule{},
		&MonitorPromRecordRule{},

		&MonitorAlertManagerPool{},
		&MonitorAlertManagerSendGroup{},

		// 值班
		&MonitorOndutyGroup{},
		&MonitorOndutyHistory{},
		&MonitorOndutyChange{},

		&MonitorAlertManagerEvent{},

		// git
		&CodeGitServer{},
	)
}

func MockUserRegister(sc *config.ServerConfig) {
	var count int64
	err := Db.Model(&User{}).Where("username = ?", "admin").Count(&count).Error
	if err == nil && count > 0 {
		sc.Logger.Info("检测到数据库已完成初始化，跳过 Mock 数据注入 🛡️")
		return
	}
	// 1. 系统基础数据（依赖顺序：最优先执行，返回超管用户供后续模块绑定关系）
	adminUser := mockSystemData(sc)
	if adminUser == nil {
		sc.Logger.Error("核心系统用户 Mock 失败，终止后续模块数据注入")
		return
	}

	// 2. CMDB 资源/服务树数据
	mockResourceData(sc, adminUser)

	// 3. 工单系统数据
	mockWorkOrderData(sc, adminUser)

	// 4. 任务执行数据
	mockJobExecData(sc, adminUser)

	// 5.监控模块数据
	mockMonitorData(sc, adminUser)

	//
	mockCodeGitData(sc, adminUser)

	sc.Logger.Info("全模块 Mock 基础数据初始化成功 🚀")
}

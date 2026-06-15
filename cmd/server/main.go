package main

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/cron"
	"bigdevops/src/models"
	"bigdevops/src/web"
	"context"
	"flag"
	"fmt"

	esl "github.com/ning1875/errgroup-signal/signal"
	"go.uber.org/zap"
)

func main() {
	// 配置文件参数 命令行启动参数
	var (
		configFile string
	)
	flag.StringVar(&configFile, "config_file", "./server.yml", "The config yml")
	flag.Parse()

	sc, err := config.LoadServer(configFile)
	if err != nil {
		fmt.Printf("load server config err:%v\n", err.Error())
		return
	}
	logger := common.NewLogger(sc.LogLevel, sc.LogFilePath)
	defer func(logger *zap.Logger) {
		_ = logger.Sync()
	}(logger)

	//logger.Info("failed to fatch URL",
	//	zap.String("url", "logger.Info"),
	//	zap.Int("attempt", 3),
	//	zap.Duration("backoff", time.Second),
	//)

	//logger.Info("测试DEBUG", zap.String("级别", sc.LogLevel))
	sc.Logger = logger
	logger.Info("解析主配置文件成功 logger初始化成功")

	//for i := 0; i < 100000; i++ {
	//	logger.Info("测试当前行数", zap.Int("行数：", i))
	//}

	// 初始化数据库
	err = models.InitDb(sc)
	if err != nil {
		logger.Error("初始化gorm-db错误", zap.String("错误", err.Error()))
		return
	}
	logger.Info("初始化gorm-db成功")

	err = models.InitCasBin(sc)
	if err != nil {
		logger.Error("初始化casbin错误", zap.Error(err))
		return
	}
	logger.Info("初始化casbin成功")

	// 同步表结构
	err = models.MigrateTable()
	if err != nil {
		logger.Error("同步表结构错误", zap.String("错误", err.Error()))
		return
	}
	logger.Info("同步表结构成功")
	fmt.Printf("主配置文件路径:%v  sc:%v\n", configFile, sc)

	// TODO 测试用 后期删除
	models.MockUserRegister(sc)

	// 初始化cronManager
	cm := cron.NewCronManager(sc)

	// 初始化group
	group, stopChan := esl.SetupStopSignalContext()
	ctxAll, cancelAll := context.WithCancel(context.Background())
	group.Go(func() error {
		logger.Info("[stopchan监听启动]")
		for {
			select {
			case <-stopChan:
				logger.Info("捕获退出信号 停止ctx 通知所有任务退出")
				cancelAll()
				return nil
			}
		}

	})
	// TODO 这里添加任务
	group.Go(func() error {
		logger.Info("计划任务--同步公有云--启动")
		err := cm.SyncCloudResourceManager(ctxAll)
		if err != nil {
			logger.Error("计划任务--同步公有云--报错", zap.Error(err))
		}
		return err
	})
	// 工单自动执行模块
	//group.Go(func() error {
	//	logger.Info("计划任务--工单自动执行模块--启动")
	//	err := cm.AuthOrderManager(ctxAll)
	//	if err != nil {
	//		logger.Error("计划任务--工单自动执行模块--报错", zap.Error(err))
	//	}
	//	return err
	//})
	group.Go(func() error {
		errChan := make(chan error, 1)
		go func() {
			errChan <- web.StartGin(sc)
		}()
		logger.Info("[web启动成功]")
		select {
		case err := <-errChan:
			logger.Error("[gin报错]", zap.Error(err))
			return err
		case <-ctxAll.Done():
			logger.Info("gin收到其他任务退出信号")
			return nil
		}
	})
	group.Wait()

	//err = web.StartGin(sc)
}

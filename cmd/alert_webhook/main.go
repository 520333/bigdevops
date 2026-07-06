package main

import (
	"bigdevops/src/alert_webhook/cron"
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"bigdevops/src/web"
	"context"
	"flag"
	"fmt"

	esl "github.com/ning1875/errgroup-signal/signal"
	"github.com/prometheus/alertmanager/template"
	"go.uber.org/zap"
)

func main() {
	// 配置文件参数 命令行启动参数
	var (
		configFile string
	)
	flag.StringVar(&configFile, "config_file", "./alert_webhook.yml", "The config yml")
	flag.Parse()

	sc, err := config.LoadAlertWebhook(configFile)
	if err != nil {
		fmt.Printf("load server config err:%v\n", err.Error())
		return
	}
	logger := common.NewLogger(sc.LogLevel, sc.LogFilePath)
	defer func(logger *zap.Logger) {
		_ = logger.Sync()
	}(logger)

	sc.Logger = logger
	logger.Info("解析主配置文件成功 logger初始化成功")

	err = models.InitDb(sc.MysqlC.DSN)
	if err != nil {
		logger.Error("初始化gorm-db错误", zap.Error(err))
		return
	}
	logger.Info("初始化gorm-db成功")

	alertReceiveQ := make(chan template.Alert)

	// new cache

	cacheHasSynced := make(chan struct{})
	ac := cron.NewAlertCache(sc, alertReceiveQ, cacheHasSynced)

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
	// TODO
	group.Go(func() error {
		logger.Info("计划任务-receive模块-启动")
		err := ac.RenewMapManager(ctxAll)
		if err != nil {
			logger.Error("计划任务-receive模块-报错", zap.Error(err))
		}
		return err
	})

	group.Go(func() error {
		logger.Info("计划任务-receive模块-启动")
		err := ac.AlertReceiveConsumerManager(ctxAll)
		if err != nil {
			logger.Error("计划任务-receive模块-报错", zap.Error(err))
		}
		return err
	})

	group.Go(func() error {
		<-cacheHasSynced
		errChan := make(chan error, 1)
		go func() {
			errChan <- web.AlertWebhookStartGin(sc, alertReceiveQ)
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

	_ = group.Wait()
}

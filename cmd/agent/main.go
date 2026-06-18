package main

import (
	"bigdevops/src/agent"
	"bigdevops/src/agent/cron"
	"bigdevops/src/agent/job"
	"bigdevops/src/common"
	"bigdevops/src/config"
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
	flag.StringVar(&configFile, "config_file", "./agent.yml", "The config yml")
	flag.Parse()

	sc, err := config.LoadAgent(configFile)
	if err != nil {
		fmt.Printf("load agent config err:%v\n", err.Error())
		return
	}

	hostName := common.GetHostName()
	localIp := common.GetLocalIP()
	sc.HostName = hostName
	sc.LocalIp = localIp
	logger := common.NewLogger(sc.LogLevel, sc.LogFilePath).With(
		zap.String("hostName", sc.HostName),
		zap.String("localIp", sc.LocalIp),
	)

	defer func(logger *zap.Logger) {
		_ = logger.Sync()
	}(logger)

	sc.Logger = logger
	logger.Info("解析主配置文件成功 logger初始化成功")
	client := agent.NewGrpcClient(sc)

	tm := job.NewTaskManager(sc)
	//job.MockTaskRun(sc)
	cm := cron.NewCronManager(client, tm)

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

	group.Go(func() error {
		errChan := make(chan error, 1)
		go func() {
			errChan <- web.AgentStartGin(sc)
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

	// TODO 这里添加任务
	{
		if sc.InfoCollect.Enable {
			group.Go(func() error {
				logger.Info("计划任务-信息采集上报-启动")
				err := cm.InfoReportManager(ctxAll)
				if err != nil {
					logger.Error("计划任务-信息采集上报-报错", zap.Error(err))
				}
				return err
			})
		} else {
			logger.Info("计划任务-信息采集上报-关闭")
		}
	}
	{
		if sc.JobExecC.Enable {
			logger.Info("计划任务-任务执行-开启")
			//_ = os.MkdirAll(sc.JobExecC.TaskDir, os.ModePerm)
			//_ = os.Chmod(sc.JobExecC.TaskDir, 0777)
			//job.MockTaskRun(sc)

			group.Go(func() error {
				logger.Info("计划任务-任务执行-启动")
				err := cm.JobExecManager(ctxAll)
				if err != nil {
					logger.Error("计划任务-任务执行-报错", zap.Error(err))
				}
				return err
			})
		} else {
			logger.Info("计划任务-任务执行-关闭")
		}
	}
	_ = group.Wait()
}

package cron

import (
	"bigdevops/src/pbms"
	"context"
	"time"

	"github.com/shimingyah/pool"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (c *CronManager) JobExecManager(ctx context.Context) error {
	go wait.UntilWithContext(ctx, c.RunTaskReport, time.Duration(c.Client.Sc.JobExecC.RunIntervalSeconds)*time.Second)
	<-ctx.Done()
	c.Client.Sc.Logger.Info("JobExecManager收到其他任务退出信号")
	return nil
}

func (c *CronManager) RunTaskReport(ctx context.Context) {
	conn, err := c.Client.Pool.Get()
	if err != nil {
		c.Client.Sc.Logger.Error("[计划任务-任务执行结果上报]在grpc连接池中获取连接错误", zap.Error(err))
		return
	}
	defer func(conn pool.Conn) {
		_ = conn.Close()
	}(conn)

	client := pbms.NewJobExecClient(conn.Value())
	ctx, cancel := c.Client.GenTwContext()
	defer cancel()

	taskResults := c.TaskManager.GetResults()

	resp, err := client.TaskReport(ctx, &pbms.TaskReportRequest{
		Hostname: c.Client.Sc.HostName,
		Ip:       c.Client.Sc.LocalIp,
		Results:  taskResults,
	})
	if err != nil {
		c.Client.Sc.Logger.Error("[计划任务-执行结果上报] 出错", zap.Error(err))
		return
	}
	c.Client.Sc.Logger.Info("[计划任务-任务执行结果上报] 服务端返回结果", zap.Any("待分配的新任务的数量", len(resp.Tasks)), zap.Any("结果", resp.ReceivedTaskIds))

	// 任务分配
	c.TaskManager.AssignTask(resp)

}

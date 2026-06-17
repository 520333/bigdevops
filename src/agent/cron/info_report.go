package cron

import (
	"bigdevops/src/common"
	"bigdevops/src/pbms"
	"context"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/shimingyah/pool"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (c *CronManager) InfoReportManager(ctx context.Context) error {
	go wait.UntilWithContext(ctx, c.RunInfoReport, time.Duration(c.Client.Sc.InfoCollect.RunIntervalSeconds)*time.Second)
	<-ctx.Done()
	c.Client.Sc.Logger.Info("InfoReportManager收到其他任务退出信号")
	return nil

}

func (c *CronManager) RunInfoReport(ctx context.Context) {
	conn, err := c.Client.Pool.Get()
	if err != nil {
		c.Client.Sc.Logger.Error("[计划任务-信息采集上报]在grpc连接池中获取连接错误", zap.Error(err))
		return
	}
	defer func(conn pool.Conn) {
		_ = conn.Close()
	}(conn)

	client := pbms.NewInfoReporterClient(conn.Value())
	ctx, cancel := c.Client.GenTwContext()
	defer cancel()

	nodeInfo := common.GetNodeInfo()

	const GB float64 = 1024 * 1024 * 1024 // 注意：把 GB 声明为 float64
	const MB float64 = 1024 * 1024
	// 使用 math.Ceil 向上取整
	memMB := int32(math.Ceil(float64(nodeInfo.MemTotal) / MB))
	diskGB := int32(math.Ceil(float64(nodeInfo.DiskTotal) / GB))
	resp, err := client.AgentInfoReport(ctx, &pbms.AgentInfoReportRequest{
		Hostname:     c.Client.Sc.HostName,
		Ip:           c.Client.Sc.LocalIp,
		Env:          os.Getenv(common.AGENT_VAR_ENV),
		OsType:       runtime.GOOS,
		OsName:       nodeInfo.OSName,
		Sn:           nodeInfo.MachineID,
		AgentVersion: common.AGENT_VERSION,
		Cpu:          nodeInfo.CPUCore,
		Mem:          memMB,
		Disk:         diskGB,
	})
	if err != nil {
		c.Client.Sc.Logger.Error("[计划任务-信息采集上报] 出错", zap.Error(err))
		return
	}
	c.Client.Sc.Logger.Info("[计划任务-信息采集上报] 结果", zap.String("结果", resp.GetStatus()))
}

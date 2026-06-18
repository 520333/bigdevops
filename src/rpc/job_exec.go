package rpc

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/pbms"
	"context"

	"go.uber.org/zap"
)

type JobExecServer struct {
	pbms.JobExecServer
	SC *config.ServerConfig
}

func (s *JobExecServer) TaskReport(ctx context.Context, in *pbms.TaskReportRequest) (resp *pbms.TaskReportResponse, err error) {
	s.SC.Logger.Info("收到任务执行结果上报请求",
		zap.String("ip", in.GetIp()),
		zap.String("hostname", in.GetHostname()),
	)

	// 1.结果落库
	// 2 是否有新任务需要下发
	resp = &pbms.TaskReportResponse{}
	for _, result := range in.GetResults() {
		result := result
		s.SC.Logger.Info("收到结果上报的请求：每个任务结果",
			zap.String("ip", in.GetIp()),
			zap.String("hostname", in.GetHostname()),
			zap.Any("任务id", result.Id),
			zap.Any("任务状态", result.Status),
			zap.Any("任务stdout", result.Stdout),
			zap.Any("任务stderr", result.Stderr),
		)
		resp.ReceivedTaskIds = append(resp.ReceivedTaskIds, result.Id)
	}
	tasks := []*pbms.TaskAssignOne{}
	num := 4
	for i := 0; i < num; i++ {
		task := &pbms.TaskAssignOne{
			Id:            int32(i + 1),
			ScriptContext: "kubectl get node",
			Account:       "root",
			Action:        common.AGENT_TASK_ACTION_START,
		}
		tasks = append(tasks, task)
	}
	tPing := &pbms.TaskAssignOne{
		Id:            5,
		ScriptContext: "ping baidu.com",
		Account:       "root",
		//Action:        common.AGENT_TASK_ACTION_START,
		Action:             common.AGENT_TASK_ACTION_KILL,
		ExecTimeoutSeconds: 10000,
	}
	tasks = append(tasks, tPing)
	resp.Tasks = tasks
	return
}

package rpc

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"bigdevops/src/pbms"
	"context"

	"go.uber.org/zap"
)

type JobExecServer struct {
	pbms.JobExecServer
	SC        *config.ServerConfig
	taskCache *cache.TaskCache
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

		dbResult, err := models.GetJobResultByJobIdAndHostIp(int(result.Id), in.GetIp())
		if err != nil {
			s.SC.Logger.Error("收到结果上报的请求：更新结果前查询记录错误",
				zap.Error(err),
				zap.String("ip", in.GetIp()),
				zap.String("hostname", in.GetHostname()),
				zap.Any("任务id", result.Id),
				zap.Any("任务状态", result.Status),
				zap.Any("任务stdout", result.Stdout),
				zap.Any("任务stderr", result.Stderr),
			)
			continue
		}

		// 赋值并更新
		dbResult.Status = result.Status
		dbResult.Stdout = result.Stdout
		dbResult.Stderr = result.Stderr

		err = dbResult.UpdateOne()
		if err != nil {
			s.SC.Logger.Error("收到结果上报的请求：更新结果 错误",
				zap.Error(err),
				zap.String("ip", in.GetIp()),
				zap.String("hostname", in.GetHostname()),
				zap.Any("任务id", result.Id),
			)
			continue
		}

		// 记录回执，告知 Agent 成功接收
		resp.ReceivedTaskIds = append(resp.ReceivedTaskIds, result.Id)

		// 执行原有的单机错误熔断策略
		_ = dbResult.JudgeByOnErrorStrategy()

		// 🚀 终极修复：每次更新完单台机器状态后，检查整个主任务是否已经全部完工
		dbJobTask, _ := models.GetJobTaskById(int(result.Id))
		if dbJobTask != nil && dbJobTask.Status == common.JOB_STATUS_RUNNING {
			err = dbJobTask.CheckAndComplete()
			if err != nil {
				s.SC.Logger.Error("更新主任务完工状态失败", zap.Error(err), zap.Int32("taskId", result.Id))
			}
		}
	}

	// 取到任务 下发
	finalTasks := []*pbms.TaskAssignOne{}
	tasks := s.taskCache.GetTaskByIp(in.GetIp())

	// 遍历task转化为 proto
	for _, task := range tasks {
		task := task

		pt := &pbms.TaskAssignOne{
			Id:                 int32(task.ID),
			ExecTimeoutSeconds: int32(task.ExecTimeoutSeconds),
			ScriptContext:      task.ScriptContent,
			Account:            task.Account,
			Args:               task.Args,
			Action:             task.Action,
			Lang:               task.Lang,
		}
		finalTasks = append(finalTasks, pt)

		// 新建结果表 status设置为running 目的是下次cache中就没有这个任务了
		dbJobResult := &models.JobResult{
			JobId:    task.ID,
			Status:   common.JOB_STATUS_RUNNING,
			HostIP:   in.GetIp(),
			HostName: in.GetHostname(),
		}
		err = dbJobResult.GetOrCreate()
		if err != nil {
			s.SC.Logger.Error("grpc收到agent上报数据，下发新任务，初始化创建任务结果表错误",
				zap.String("ip", in.GetIp()),
				zap.String("hostname", in.GetHostname()),
				zap.Any("任务id", task.ID),
				zap.Error(err),
			)
		}
	}

	resp.Tasks = finalTasks
	return
}

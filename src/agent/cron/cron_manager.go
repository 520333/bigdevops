package cron

import (
	"bigdevops/src/agent"
	"bigdevops/src/agent/job"
)

// CronManager 定义执行计划任务对象
type CronManager struct {
	Client      *agent.Client
	TaskManager *job.TaskManager
}

func NewCronManager(c *agent.Client, tm *job.TaskManager) *CronManager {
	return &CronManager{
		Client:      c,
		TaskManager: tm,
	}
}

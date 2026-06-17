package cron

import (
	"bigdevops/src/agent"
)

// CronManager 定义执行计划任务对象
type CronManager struct {
	Client *agent.Client
}

func NewCronManager(c *agent.Client) *CronManager {
	return &CronManager{
		Client: c,
	}
}

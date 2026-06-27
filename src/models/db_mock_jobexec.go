package models

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"encoding/json"
	"fmt"
)

var (
	mockScriptContentNoArgs   = `kubectl get node2`
	mockScriptContentWithArgs = `kubectl get node $1`
	mockScriptContentSleep    = `date
echo hello
sleep 100`
	mockScriptContents = []string{
		mockScriptContentWithArgs,
		mockScriptContentNoArgs,
		mockScriptContentSleep,
	}
)

func mockJobExecData(sc *config.ServerConfig, adminUser *User) {
	num := 5
	hosts := []string{"192.168.50.200"}
	for i := 0; i < num; i++ {
		hosts = append(hosts, fmt.Sprintf("192.168.50.20%d", i+1))
	}
	hostJson, _ := json.Marshal(hosts)

	for i, c := range mockScriptContents {
		job := JobTask{
			Title:              fmt.Sprintf("测试的job%v", i),
			Account:            "root",
			Args:               "",
			ScriptContent:      c,
			ExecTimeoutSeconds: 60,
			HostsRaw:           string(hostJson),
			BatchSize:          i,
			Action:             "",
			OnErrorStrategy:    common.JOB_ONERROR_STRATEGY_PAUSE,
			Status:             common.JOB_STATUS_RUNNING,
			UserID:             adminUser.ID,
		}
		job.CreateOne()
	}
	sc.Logger.Info("任务执行模块 Mock 数据注入成功")
}

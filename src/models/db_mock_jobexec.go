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
	ansible = `aaa: 
	- name: "aaa"
  	value: "111"
bbb: "222"`
	shell = `#!/bin/bash
VERSION=123
OS-VERSION=$(cat /etc/os-release)
echo $VERSION

if [ $? eq 0]
 echo "未找到"
fi`
	python = `import secrets
import string

def generate_password(length=12):
	"""生成一个包含大小写字母、数字和符号的随机密码"""
  alphabet = string.ascii_letters + string.digits + "!@#$%^&*"
	password = ''.join(secrets.choice(alphabet) for i in range(length))
	return password

# 使用示例
print(f"你的新密码是: {generate_password(16)}")`
	js = `{
  "code": 200,
  "msg": "operation successfully",
  "result": {
    "test_get": {
      "pageNum": 1,
      "pageSize": 10,
      "dataSize": 1,
      "totalPage": 1,
      "totalCount": 1,
      "data": [
        {
          "ZsTestPO": {
            "name": "ccc",
            "salary": 2200
          }
        }
      ]
    }
  },
  "log": null
}`
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

	script1 := JobScript{
		Name:    "ansible",
		Lang:    "ansible",
		UserID:  1,
		Content: ansible,
	}
	script1.Create()

	script2 := JobScript{
		Name:    "shell",
		Lang:    "shell",
		UserID:  1,
		Content: shell,
	}
	script2.Create()

	script3 := JobScript{
		Name:    "python",
		Lang:    "python",
		UserID:  1,
		Content: python,
	}
	script3.Create()

	script4 := JobScript{
		Name:    "json",
		Lang:    "json",
		UserID:  1,
		Content: js,
	}
	script4.Create()
}

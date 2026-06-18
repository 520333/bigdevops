package job

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/pbms"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
)

type TaskManager struct {
	Sc *config.AgentConfig
	LM map[int]*Task
	sync.RWMutex
}

func NewTaskManager(sc *config.AgentConfig) *TaskManager {
	tm := &TaskManager{
		Sc: sc,
		LM: make(map[int]*Task),
	}
	lm := tm.GetAllTaskFromDisk()
	tm.LM = lm
	return tm
}

func (tm *TaskManager) SetTask(t *Task) {
	tm.Lock()
	defer tm.Unlock()
	tm.LM[t.Id] = t
}

func (tm *TaskManager) GetTask(tId int) *Task {
	tm.RLock()
	defer tm.RUnlock()
	return tm.LM[tId]
}

// GetAllTaskFromDisk 从磁盘获取完成的结果
func (tm *TaskManager) GetAllTaskFromDisk() (res map[int]*Task) {
	res = make(map[int]*Task)
	items, err := os.ReadDir(tm.Sc.JobExecC.TaskDir)
	if err != nil {
		tm.Sc.Logger.Error("遍历结果目录失败", zap.Error(err))
		return
	}
	for _, item := range items {
		if !item.IsDir() {
			continue
		}
		tId, _ := strconv.Atoi(item.Name())
		if tId == 0 {
			continue
		}

		taskDir := fmt.Sprintf("%s/%s", tm.Sc.JobExecC.TaskDir, item.Name())

		stdOut, _ := common.ReadFile(fmt.Sprintf("%s/stdout", taskDir))
		// 如果时间太早的就丢弃
		startTime, _ := common.ReadFile(fmt.Sprintf("%s/startTime", taskDir))
		loc, _ := time.LoadLocation("Asia/Shanghai")
		tt, _ := time.ParseInLocation("2006-01-02 15:04:05", startTime, loc)
		if time.Now().Sub(tt) > time.Hour {
			continue
		}

		stdErr, _ := common.ReadFile(fmt.Sprintf("%s/stderr", taskDir))
		status, _ := common.ReadFile(fmt.Sprintf("%s/status", taskDir))
		t := &Task{
			Id:         tId,
			Status:     status,
			ScriptPath: "",
			TaskDir:    "",
			Stdout:     *bytes.NewBuffer([]byte(stdOut)),
			Stderr:     *bytes.NewBuffer([]byte(stdErr)),
			Sc:         tm.Sc,
		}
		res[tId] = t
	}
	return
}

// AssignTask 分配task
func (tm *TaskManager) AssignTask(resp *pbms.TaskReportResponse) {

	diskTaskMap := tm.GetAllTaskFromDisk()

	tm.Lock()
	defer tm.Unlock()
	// 1. 遍历远端下发的任务
	for _, t := range resp.Tasks {
		t := t
		localT := tm.LM[int(t.Id)]
		if localT == nil {
			// 远端有，本地内存没有。检查磁盘是否有遗留。
			_, ok := diskTaskMap[int(t.Id)]
			if ok {
				//// 🚀 修复隐患 2：说明 Agent 发生过重启，或者 Server 在重试。
				//// 必须从磁盘读取遗留数据并塞进内存，否则 GetResults 永远不会上报它！
				//localT = &Task{Id: int(t.Id), Sc: tm.Sc}
				//localT.SetTaskDir()
				//
				//// 恢复状态和输出，这样后续 GetResults 就能正常抓取了
				//status, _ := os.ReadFile(localT.TaskDir + "/status")
				//stdout, _ := os.ReadFile(localT.TaskDir + "/stdout")
				//stderr, _ := os.ReadFile(localT.TaskDir + "/stderr")
				//
				//localT.Status = string(status)
				//localT.Stdout.Write(stdout)
				//localT.Stderr.Write(stderr)
				//
				//tm.LM[localT.Id] = localT
				continue // 恢复到内存后即可跳过，无需重新执行
			}
			if t.Action == common.AGENT_TASK_ACTION_KILL || t.Action == common.AGENT_TASK_ACTION_STOP {
				tm.Sc.Logger.Warn("收到陌生任务的Kill指令，已忽略", zap.Int32("taskId", t.Id))
				continue
			}
			// 真正的全新任务，正常初始化
			localT = &Task{
				Id:                 int(t.Id),
				Account:            t.Account,
				ScriptContext:      t.ScriptContext,
				Args:               t.Args,
				ExecTimeoutSeconds: int(t.ExecTimeoutSeconds),
				Sc:                 tm.Sc,
			}
			localT.SetTaskDir()
			tm.LM[localT.Id] = localT
			go localT.Start()
		} else {
			// 紧急的kill任务
			if t.Action == common.AGENT_TASK_ACTION_KILL {
				go localT.Kill()
			}
		}
	}
	// 2.  修复隐患 1：服务端已经确定收到了，连同内存和磁盘一起彻底清理！
	for _, tId := range resp.ReceivedTaskIds {
		// 清理内存
		delete(tm.LM, int(tId))

		// 清理磁盘 (强制组装路径并删除)
		//taskDir := fmt.Sprintf("%s/%d", tm.Sc.JobExecC.TaskDir, tId)
		//err := os.RemoveAll(taskDir)
		//if err != nil {
		//	tm.Sc.Logger.Error("清理本地任务遗留目录失败", zap.Int32("taskId", tId), zap.Error(err))
		//} else {
		//	tm.Sc.Logger.Info("任务生命周期完美闭环，本地已清理", zap.Int32("taskId", tId))
		//}
	}
}

// GetResults 获取本次本地已经结束的任务结果 并上报
func (tm *TaskManager) GetResults() []*pbms.TaskResultOne {
	results := []*pbms.TaskResultOne{}
	tm.RLock()
	defer tm.RUnlock()
	for tId, t := range tm.LM {
		tId := tId
		t := t
		status := t.GetStatus()
		// 跳过running态
		if status == common.AGENT_TASK_STATUS_RUNNING {
			continue
		}
		resultOne := &pbms.TaskResultOne{
			Id:     int32(tId),
			Status: status,
			Stdout: t.GetStdOut(),
			Stderr: t.GetStdErr(),
		}
		results = append(results, resultOne)
	}

	return results
}

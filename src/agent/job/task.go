package job

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

type Task struct {
	sync.Mutex
	Id       int    `json:"id"`       // 任务id
	ExecType string `json:"execType"` // 脚本类型
	//ExecType           string              `json:"lang"`
	Account            string              `json:"account"` // 执行命令的账号: root devops ubuntu
	Args               string              `json:"args"`    // 脚本执行参数
	Status             string              `json:"status"`  // 执行状态
	StartTime          string              `json:"startTime"`
	ExecTimeoutSeconds int                 `json:"execTimeoutSeconds"`
	ScriptPath         string              `json:"scriptPath"`
	TaskDir            string              `json:"taskDir"` // 多个任务的结果 保存到本地 可以进行问题调查
	Sc                 *config.AgentConfig `json:"-"`
	Cmd                *exec.Cmd           `json:"-"`
	ScriptContext      string              `json:"-"` // 脚本内容 来自模板 从server端获取
	Stdout, Stderr     bytes.Buffer        `json:"-"` // 执行结果：标准输出输入

}

var (
	mockShellNormal        = `echo "====== 基础 Shell 测试 ======"; echo "当前用户: $(whoami)"`
	mockShellWithArgs      = `kubectl get node $1`
	mockScriptContextSleep = `date
echo hello
sleep 10`
	mockTimeoutScript = `echo "====== 超时强杀测试 ======"; echo "准备睡 10 秒..."; sleep 10; echo "这段话绝对不应该被打印出来!"`
	mockScrpitNotRoot = `pwd
ls -l
echo nyy
date >/root/abc`
	mockPythonNoArgs = `
import time
print("哈哈.sleep100")
time.sleep(100)
`
	mockPythonArgs = `
import sys
if __name__ == "__main__":
    args = sys.argv
    print("参数", args)

    total = sum(int(x) for x in args[1:])
    print("加法结果", total)
`
)

func MockTaskRun(sc *config.AgentConfig) {
	t1 := &Task{
		Account:       "root",
		ScriptContext: mockShellNormal,
		Id:            1,
		Sc:            sc,
	}
	t2 := &Task{
		Account:       "root",
		ScriptContext: mockShellWithArgs,
		Args:          "k8s-master",
		Id:            2,
		Sc:            sc,
	}
	t3 := &Task{
		Account:       "root",
		ScriptContext: mockScriptContextSleep,
		Id:            3,
		Sc:            sc,
	}
	t4 := &Task{
		Id:                 4,
		Account:            "root",
		ScriptContext:      mockTimeoutScript,
		ExecTimeoutSeconds: 2, // 🚀 关键：2秒后触发强杀
		Sc:                 sc,
	}
	t5 := &Task{
		Id:            5,
		Account:       "devops",
		ScriptContext: mockScrpitNotRoot,
		Sc:            sc,
	}
	t6 := &Task{
		Id:            6,
		Account:       "root",
		ExecType:      common.AGENT_TASK_EXEC_PYTHON,
		ScriptContext: mockPythonNoArgs,
		Sc:            sc,
	}
	t7 := &Task{
		Id:            7,
		Account:       "root",
		ExecType:      common.AGENT_TASK_EXEC_PYTHON,
		ScriptContext: mockPythonArgs,
		Args:          "10 20 30 40",
		Sc:            sc,
	}
	ts := []*Task{t1, t2, t3, t4, t5, t6, t7}
	for _, t := range ts {
		t := t
		t.SetTaskDir()
		err := t.Start()
		fmt.Println(err)
	}

}

func (t *Task) SetTaskDir() {
	//t.TaskDir = fmt.Sprintf("%s/%d", t.Sc.JobExecC.TaskDir, t.Id)

	absDir, err := filepath.Abs(t.Sc.JobExecC.TaskDir)
	if err != nil {
		// 如果解析失败，兜底使用原路径
		absDir = t.Sc.JobExecC.TaskDir
	}
	t.TaskDir = fmt.Sprintf("%s/%d", absDir, t.Id)

	if t.ExecType == "" {
		t.ExecType = common.AGENT_TASK_EXEC_SHELL
	}

}
func (t *Task) SetStatus(status string) {
	t.Lock()
	defer t.Unlock()
	t.Status = status
}
func (t *Task) GetStdOut() string {
	t.Lock()
	defer t.Unlock()
	return t.Stdout.String()
}
func (t *Task) GetStdErr() string {
	t.Lock()
	defer t.Unlock()
	return t.Stderr.String()
}
func (t *Task) GetStatus() string {
	t.Lock()
	defer t.Unlock()
	return t.Status
}

// Prepare 任务开始前的准备工作
func (t *Task) Prepare() (err error) {
	err = os.MkdirAll(t.TaskDir, os.ModePerm)
	if err != nil {
		t.Sc.Logger.Error("任务开始前创建元信息目录失败", zap.Error(err), zap.Any("任务id", t.Id))
		return
	}
	scriptPath := fmt.Sprintf("%s/script", t.TaskDir)
	t.ScriptPath = scriptPath
	err = os.WriteFile(scriptPath, []byte(t.ScriptContext), 0777)
	if err != nil {
		t.Sc.Logger.Error("任务开始前写入脚本失败", zap.Error(err), zap.Any("任务id", t.Id))
		return
	}
	metaJson, err := json.Marshal(t)
	if err != nil {
		t.Sc.Logger.Error("任务开始前写入元信息json错误", zap.Error(err), zap.Any("任务id", t.Id))
		return
	}
	err = os.WriteFile(fmt.Sprintf("%s/meta.json", t.TaskDir), metaJson, 0666)
	if err != nil {
		t.Sc.Logger.Error("任务开始前写入元信息文件json失败", zap.Error(err), zap.Any("任务id", t.Id))
		return
	}
	startTime := time.Now().Format("2006-01-02 15:04:05")
	err = os.WriteFile(fmt.Sprintf("%s/startTime", t.TaskDir), []byte(startTime), 0666)
	if err != nil {
		t.Sc.Logger.Error("任务开始前写入启动时间文件失败", zap.Error(err), zap.Any("任务id", t.Id))
		return
	}
	return nil
}

// Kill 紧急kill
func (t *Task) Kill() {
	t.Sc.Logger.Info("任务准备kill", zap.Any("任务id", t.Id), zap.Any("进程id", t.Cmd.Process.Pid))
	// 2. 标记任务状态为 killed
	t.Status = common.AGENT_TASK_STATUS_KILLED
	syscall.Kill(-t.Cmd.Process.Pid, syscall.SIGKILL)
}

// Kill 紧急kill (安全版)
//func (t *Task) Kill() {
//	t.Lock()
//	defer t.Unlock()
//
//	// 🚀 安全防线 1：只有状态依然是 running 时，才允许击杀
//	if t.Status != common.AGENT_TASK_STATUS_RUNNING {
//		t.Sc.Logger.Warn("任务未处于运行状态，无需kill", zap.Int("任务id", t.Id), zap.String("当前状态", t.Status))
//		return
//	}
//
//	// 🚀 安全防线 2：防空指针。如果 Cmd 为空，说明这是 Agent 重启后从磁盘恢复的死任务
//	if t.Cmd == nil || t.Cmd.Process == nil {
//		t.Sc.Logger.Warn("任务进程对象不存在(可能已重启)，无法执行强杀", zap.Int("任务id", t.Id))
//		// 既然找不到进程，就把它强制标记为 failed，防止它一直卡在 running 状态
//		t.Status = common.AGENT_TASK_STATUS_FAILED
//		return
//	}
//
//	t.Sc.Logger.Info("接收到手动 Kill 指令，准备强杀整个进程组", zap.Int("任务id", t.Id), zap.Int("进程组id", -t.Cmd.Process.Pid))
//
//	// 执行进程组强制击杀
//	err := syscall.Kill(-t.Cmd.Process.Pid, syscall.SIGKILL)
//	if err != nil {
//		// 屏蔽 "no such process" 报错，因为这说明进程自己已经跑完退出了
//		if strings.Contains(err.Error(), "no such process") {
//			t.Sc.Logger.Info("进程已自行结束，无需强杀", zap.Int("任务id", t.Id))
//		} else {
//			t.Sc.Logger.Error("进程组强杀失败", zap.Error(err), zap.Int("任务id", t.Id))
//		}
//	} else {
//		t.Sc.Logger.Info("进程组强杀信号发送成功", zap.Int("任务id", t.Id))
//	}
//}

// Start 直接context版本
func (t *Task) Start() error {
	err := t.Prepare()
	if err != nil {
		t.Sc.Logger.Error("任务准备失败", zap.Error(err), zap.Int("id", t.Id))
		return err
	}
	if t.ExecTimeoutSeconds == 0 {
		t.ExecTimeoutSeconds = t.Sc.JobExecC.ExecTimeoutSeconds
	}
	//if t.ExecTimeoutSeconds == 0 {
	//	t.ExecTimeoutSeconds = 600
	//}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(t.ExecTimeoutSeconds)*time.Second)

	// 🚀 核心改造 1：底层统一使用 env 命令启动
	cmdName := "env"
	cmdArgs := []string{}

	// 🚀 核心改造 2：将具体的引擎作为 env 的参数传入
	switch t.ExecType {
	case common.AGENT_TASK_EXEC_PYTHON:
		cmdArgs = []string{"python3", t.ScriptPath}
	case common.AGENT_TASK_EXEC_ANSIBLE:
		cmdArgs = []string{"ansible-playbook", "-i", "localhost,", "-c", "local", t.ScriptPath}
	default: // SHELL
		cmdArgs = []string{"bash", t.ScriptPath}
	}

	// 追加用户自定义参数 (安全切分)
	if strings.TrimSpace(t.Args) != "" {
		userArgs := strings.Split(strings.TrimSpace(t.Args), " ")
		cmdArgs = append(cmdArgs, userArgs...)
	}

	// 处理 Linux 下的非 Root 降权执行
	finalCmdName := cmdName
	finalCmdArgs := cmdArgs

	if t.Account != "" && t.Account != "root" {
		finalCmdName = "su"
		// 🚀 核心改造 3：包装进 su -c。
		// 最终效果类似：su - devops -c "env python3 /tmp/script.py arg1"
		// 这保证了 devops 用户在加载完自己的 ~/.bash_profile 后，再用 env 寻找属于自己的 python3
		innerCmdStr := cmdName + " " + strings.Join(cmdArgs, " ")
		finalCmdArgs = []string{"-", t.Account, "-c", innerCmdStr}
	}

	// 生成命令实例
	cmd := exec.Command(finalCmdName, finalCmdArgs...)
	t.Sc.Logger.Info("任务命令打印", zap.Int("任务id", t.Id), zap.String("脚本类型", t.ExecType), zap.Any("cmdName", finalCmdName), zap.Any("cmdArgs", finalCmdArgs))
	cmd.Stdout = &t.Stdout
	cmd.Stderr = &t.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	t.Cmd = cmd

	err = cmd.Start()
	t.SetStatus(common.AGENT_TASK_STATUS_RUNNING)
	if err != nil {
		cancel()
		t.Sc.Logger.Error("任务Start失败", zap.Error(err), zap.Any("任务id", t.Id))
		return err
	}

	// 启动超时强杀协程
	go func() {
		<-ctx.Done()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()

	go t.runProcess(cancel)
	return nil
}

// 设置状态等待结果写入文件

func (t *Task) runProcess(cancel context.CancelFunc) {
	defer cancel()
	err := t.Cmd.Wait()
	if err != nil {
		if strings.Contains(err.Error(), "signal: killed") {
			t.SetStatus(common.AGENT_TASK_STATUS_KILLED)
			t.Sc.Logger.Error("任务被人为或触发超时kill", zap.Error(err), zap.Int("任务id", t.Id))
		} else {
			t.SetStatus(common.AGENT_TASK_STATUS_FAILED)
			t.Sc.Logger.Error("任务执行出错", zap.Error(err), zap.Int("任务id", t.Id))
		}
	} else {
		t.SetStatus(common.AGENT_TASK_STATUS_SUCCESS)
		t.Sc.Logger.Info("任务执行成功", zap.Int("任务id", t.Id))
	}
	t.persistResult()
}

// 持久化任务保持到文件
func (t *Task) persistResult() {
	//taskDir := fmt.Sprintf("%s/%d", t.MetaDir, t.Id)
	sdtOutPath := fmt.Sprintf("%s/stdout", t.TaskDir)
	sdtErrPath := fmt.Sprintf("%s/stderr", t.TaskDir)
	statusPath := fmt.Sprintf("%s/status", t.TaskDir)

	// 写标准输出
	err := common.WriteFileWithString(sdtOutPath, t.GetStdOut())
	if err != nil {
		t.Sc.Logger.Error("[任务执行]落盘 stdout 出错", zap.Error(err), zap.Int("任务id", t.Id), zap.Any("标准输出", t.Stdout.String()))
	}

	// 写标准错误
	err = common.WriteFileWithString(sdtErrPath, t.GetStdErr())
	if err != nil {
		t.Sc.Logger.Error("[任务执行]落盘 stderr 出错", zap.Error(err), zap.Int("任务id", t.Id), zap.Any("标准错误", t.Stderr.String()))
	}

	// 写最终状态
	err = common.WriteFileWithString(statusPath, t.GetStatus())
	if err != nil {
		t.Sc.Logger.Error("[任务执行]落盘 status 出错", zap.Error(err), zap.Int("任务id", t.Id), zap.Any("状态结果", t.Status))
	}
	return
}

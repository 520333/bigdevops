package config

import (
	"os"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type AgentConfig struct {
	RpcServerAddr         string `yaml:"rpc_server_addr"`
	RpcCallTimeoutSeconds int    `yaml:"rpc_call_timeout_seconds"`
	HttpAddr              string `yaml:"http_addr"`
	LogLevel              string `yaml:"log_level"`
	LogFilePath           string `yaml:"log_file_path"`

	InfoCollect *InfoCollect `yaml:"info_collect"`
	JobExecC    *JobExec     `yaml:"job_exec"`
	HostName    string       `yaml:"-"`
	LocalIp     string       `yaml:"-"`
	Logger      *zap.Logger  `yaml:"-"`
}

// LoadAgent 根据io read 读取配置文件后的字符串解析yaml
func LoadAgent(filename string) (*AgentConfig, error) {
	cfg := &AgentConfig{}
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(content, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, err
}

type InfoCollect struct {
	RunIntervalSeconds int  `yaml:"run_interval_seconds"`
	Enable             bool `yaml:"enable"`
}

type JobExec struct {
	TaskDir            string `yaml:"task_dir"`
	ExecTimeoutSeconds int    `yaml:"execTimeoutSeconds"`
	RunIntervalSeconds int    `yaml:"run_interval_seconds"`
	PythonBinPath      string `yaml:"python_bin_path"`
	BashBinPath        string `yaml:"bash_bin_path"`
	Enable             bool   `yaml:"enable"`
}

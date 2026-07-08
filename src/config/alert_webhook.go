package config

import (
	"os"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/mysql"
)

type AlertWebhookConfig struct {
	HttpAddr                        string        `yaml:"http_addr"`
	LogLevel                        string        `yaml:"log_level"`
	LogFilePath                     string        `yaml:"log_file_path"`
	AlertReceiveQ                   int           `yaml:"alert_receive_queue_size"`
	MysqlC                          *mysql.Config `yaml:"mysql"`
	HttpRequestGlobalTimeoutSeconds int           `yaml:"http_request_global_timeout_seconds"`
	CommonMapRenewIntervalSeconds   int           `yaml:"common_map_renew_interval_seconds"`
	AlertManagerApi                 string        `yaml:"alert_manager_api"`
	AlertTimezone                   string        `yaml:"alert_timezone"`
	LocalIp                         string        `yaml:"-"`
	Logger                          *zap.Logger   `yaml:"-"`
	FrontDomain                     string        `yaml:"front_domain"`
	BackendDomain                   string        `yaml:"backend_domain"`
}

// LoadAlertWebhook 根据io read 读取配置文件后的字符串解析yaml
func LoadAlertWebhook(filename string) (*AlertWebhookConfig, error) {
	cfg := &AlertWebhookConfig{}
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

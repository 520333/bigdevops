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

	ImC *IMConfig `yaml:"im"`
}

//type FeiShu struct {
//	URL       string `yaml:"group_webhook"` // 自定义机器人webhook地址
//	Secret    string `yaml:"secret"`        // 自定义机器人签名校验
//	AppId     string `yaml:"app_id"`        // 应用机器人id
//	AppSecret string `yaml:"app_secret"`    // 应用机器人密钥
//}

type IMConfig struct {
	FeiShu   *FeiShuConfig     `yaml:"feishu"`
	DingDing *DingDingConfig   `yaml:"dingding"`
	QYWX     *QiYeWeiXinConfig `yaml:"qywx"` // 企业微信
}

// FeiShuConfig 飞书规范
type FeiShuConfig struct {
	Enabled bool   `yaml:"enabled"`
	Webhook string `yaml:"webhook"` // 自定义机器人 Webhook
	Secret  string `yaml:"secret"`  // 签名校验

	TenantAccessTokenApi  string `yaml:"tenant_access_token_api"` // 应用机器人api地址
	AppID                 string `yaml:"app_id"`                  // 应用ID
	AppSecret             string `yaml:"app_secret"`              // 应用密钥
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"` // 请求超时时间
}

// DingDingConfig 钉钉规范
type DingDingConfig struct {
	Enabled               bool   `yaml:"enabled"`
	Webhook               string `yaml:"webhook"`
	Secret                string `yaml:"secret"`                  // 加签密钥
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"` // 请求超时时间
}

// QiYeWeiXinConfig 企业微信规范
type QiYeWeiXinConfig struct {
	Enabled               bool   `yaml:"enabled"`
	Webhook               string `yaml:"webhook"` // 群机器人 Webhook
	CorpID                string `yaml:"corp_id"`
	AgentID               string `yaml:"agent_id"`
	AppSecret             string `yaml:"app_secret"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"` // 请求超时时间
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

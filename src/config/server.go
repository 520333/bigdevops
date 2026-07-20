package config

import (
	"os"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/mysql"
)

type ServerConfig struct {
	HttpAddr                        string               `yaml:"http_addr"`
	HttpRequestGlobalTimeoutSeconds int                  `yaml:"http_request_global_timeout_seconds"`
	MysqlC                          *mysql.Config        `yaml:"mysql"` //
	LogLevel                        string               `yaml:"log_level"`
	LogFilePath                     string               `yaml:"log_file_path"`
	SuperRoleName                   string               `yaml:"super_role_name"`
	PublicCloudSyncC                *PublicCloudSync     `yaml:"public_cloud_sync"`
	JWTC                            *JWT                 `yaml:"jwt"`
	WorkOrderAutoActionC            *WorkOrderAutoAction `yaml:"work_order_auto_action"`
	GrpcServerConfig                *GrpcServerConfig    `yaml:"grpc_server_config"`
	JobExec                         *ServerJobExec       `yaml:"job_exec"`
	K8sClusterC                     *K8sCluster          `yaml:"k8s_cluster"`
	MonitorComputeC                 *MonitorCompute      `yaml:"monitor_compute"`
	Logger                          *zap.Logger          `yaml:"-"`
	Domain                          string               `yaml:"front_domain"`
	ImC                             *IMConfig            `yaml:"im"`
	AlertManagerApi                 string               `yaml:"alert_manager_api"`
}

type K8sCluster struct {
	Enable             bool `yaml:"enable"`
	RunIntervalSeconds int  `yaml:"run_interval_seconds"`
	ExecTimeoutSeconds int  `yaml:"execTimeoutSeconds"`
}
type ServerJobExec struct {
	Enable             bool `yaml:"enable"`
	RunIntervalSeconds int  `yaml:"run_interval_seconds"`
}
type MonitorCompute struct {
	Enable             bool   `yaml:"enable"`
	RunIntervalSeconds int    `yaml:"run_interval_seconds"`
	ExecTimeoutSeconds int    `yaml:"exec_timeout_seconds"`
	HttpSdApi          string `yaml:"http_sd_api"`
	AlertWebhookAddr   string `yaml:"alert_web_hook_addr"`
}

type GrpcServerConfig struct {
	Addr string `yaml:"addr"`
}

type WorkOrderAutoAction struct {
	ServiceAccount         string `yaml:"service_account"` // 服务账号名称
	Enable                 bool   `yaml:"enable"`
	RunIntervalSeconds     int    `yaml:"run_interval_seconds"`
	BatchNum               int    `yaml:"batch_num"`
	AutoTemplateNameBuyEcs string `yaml:"auto_template_name_buy_ecs"`
	AutoTemplateNameRmEcs  string `yaml:"auto_template_name_rm_ecs"`
}

type PublicCloudSync struct {
	RunIntervalSeconds int         `yaml:"run_interval_seconds"`
	Enable             bool        `yaml:"enable"`
	AliCloud           []*AliCloud `yaml:"ali_cloud"`
	AwsCloud           []*AwsCloud `yaml:"aws_cloud"`

	GodaddyDns *GodaddyDns `yaml:"godaddy_dns"`
	DynadotDns *DynadotDns `yaml:"dynadot_dns"`
}

type AliCloud struct {
	Enable          bool   `yaml:"enable"`
	AccountName     string `yaml:"account_name"`
	RegionId        string `yaml:"region_id"`
	AccessKeyId     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
}

// AwsCloud 👉 新增 AWS 的配置结构体
type AwsCloud struct {
	Enable          bool   `yaml:"enable"`
	AccountName     string `yaml:"account_name"`
	RegionId        string `yaml:"region_id"`
	AccessKeyId     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"` // 注意：AWS 习惯称之为 Secret Access Key
}

// 域名供应商
type GodaddyDns struct {
	Enable          bool     `yaml:"enable"`
	AccessKeyId     string   `yaml:"access_key_id"`
	AccessKeySecret string   `yaml:"access_key_secret"`
	Domains         []string `yaml:"domains"` // 支持多个域名同步
}

type DynadotDns struct {
	Enable  bool     `yaml:"enable"`
	ApiKey  string   `yaml:"api_key"`
	Domains []string `yaml:"domains"`
}

// LoadServer 根据io read 读取配置文件后的字符串解析yaml
func LoadServer(filename string) (*ServerConfig, error) {
	cfg := &ServerConfig{}
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(content, cfg)
	if err != nil {
		return nil, err
	}
	exd, err := time.ParseDuration(cfg.JWTC.ExpiresTime)
	if err != nil {
		return nil, err
	}
	bud, err := time.ParseDuration(cfg.JWTC.BufferTime)
	if err != nil {
		return nil, err
	}
	cfg.JWTC.ExpiresDuration = exd
	cfg.JWTC.BufferDuration = bud
	return cfg, err
}

type JWT struct {
	SigningKey      string        `yaml:"signing_key" json:"signing_key"`   // 签名
	ExpiresTime     string        `yaml:"expires_time" json:"expires-time"` // 过期时间
	ExpiresDuration time.Duration `yaml:"-"`
	BufferTime      string        `yaml:"buffer_time" json:"buffer-time"` // 缓冲时间
	BufferDuration  time.Duration `yaml:"-"`                              // 缓冲时间
	Issuer          string        `yaml:"issuer" json:"issuer"`           // 签发者
}

package common

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// NacosClientOptions Nacos 连接配置参数
type NacosClientOptions struct {
	ServerAddr  string
	Port        uint64
	NamespaceId string
	Username    string
	Password    string
}

// NewNacosConfigClient 使用官方 v2 SDK 创建 Nacos 配置客户端
func NewNacosConfigClient(opt *NacosClientOptions) (config_client.IConfigClient, error) {
	// 去除可能的 http:// 或 https:// 前缀
	addr := opt.ServerAddr
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimRight(addr, "/")

	port := opt.Port
	if port == 0 {
		port = 8848
	}

	// 1. 服务端集群配置
	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig(addr, port),
	}

	// 2. 客户端参数配置
	tmpDir := os.TempDir()
	clientConfig := *constant.NewClientConfig(
		constant.WithNamespaceId(opt.NamespaceId),
		constant.WithTimeoutMs(5000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogDir(tmpDir+"/nacos/log"),
		constant.WithCacheDir(tmpDir+"/nacos/cache"),
		constant.WithLogLevel("error"),
		constant.WithUsername(opt.Username),
		constant.WithPassword(opt.Password),
	)

	// 3. 构建并返回 IConfigClient
	return clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig:  &clientConfig,
		ServerConfigs: serverConfigs,
	})
}

// TestNacosConnection 测试与远端 Nacos 服务端的连通性 (严格校验网络与账号密码)
func TestNacosConnection(opt *NacosClientOptions) error {
	addr := opt.ServerAddr
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimRight(addr, "/")

	port := opt.Port
	if port == 0 {
		port = 8848
	}

	httpClient := &http.Client{Timeout: 4 * time.Second}

	// 1. 如果配置了账号或密码，首先严格进行 Nacos 鉴权握手接口校验 (POST /nacos/v1/auth/users/login)
	if opt.Username != "" || opt.Password != "" {
		loginUrl := fmt.Sprintf("http://%s:%d/nacos/v1/auth/users/login", addr, port)
		data := url.Values{}
		data.Set("username", opt.Username)
		data.Set("password", opt.Password)

		resp, err := httpClient.Post(loginUrl, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		if err != nil {
			return fmt.Errorf("无法连接 Nacos 鉴权服务: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			msg := strings.TrimSpace(string(body))
			if msg == "" {
				msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
			return fmt.Errorf("账号或密码错误: %s", msg)
		}
	} else {
		// 免密模式：快速健康探活
		healthUrl := fmt.Sprintf("http://%s:%d/nacos/v1/console/health/readiness", addr, port)
		resp, err := httpClient.Get(healthUrl)
		if err != nil {
			metricsUrl := fmt.Sprintf("http://%s:%d/nacos/v1/ns/operator/metrics", addr, port)
			resp2, err2 := httpClient.Get(metricsUrl)
			if err2 != nil {
				return fmt.Errorf("无法连接 Nacos 服务端 [%s:%d]: %v", addr, port, err)
			}
			resp2.Body.Close()
		} else {
			resp.Body.Close()
		}
	}

	// 2. 使用官方 SDK 创建客户端验证链路
	client, err := NewNacosConfigClient(opt)
	if err != nil {
		return fmt.Errorf("创建 Nacos 客户端失败: %w", err)
	}

	// 3. 通过 SDK 进行探活
	_, _ = client.GetConfig(vo.ConfigParam{
		DataId: "__nacos_ping_probe__",
		Group:  "DEFAULT_GROUP",
	})

	return nil
}

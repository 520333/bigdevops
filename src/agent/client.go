package agent

import (
	"bigdevops/src/config"
	"context"
	"time"

	"github.com/shimingyah/pool"
	"go.uber.org/zap"
)

type Client struct {
	Pool pool.Pool
	Sc   *config.AgentConfig
}

func (c *Client) GenTwContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(c.Sc.RpcCallTimeoutSeconds)*time.Second)
}

func NewGrpcClient(sc *config.AgentConfig) *Client {
	client := &Client{}
	p, err := pool.New(sc.RpcServerAddr, pool.DefaultOptions)
	if err != nil {
		sc.Logger.Fatal("创建grpc连接池错误:", zap.Error(err))
	}
	client.Pool = p
	client.Sc = sc
	return client
}

package rpc

import (
	"bigdevops/src/cache"
	"bigdevops/src/config"
	"bigdevops/src/pbms"
	"net"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func StartServerGrpc(sc *config.ServerConfig, taskCache *cache.TaskCache) error {
	kaep := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second, // 允许客户端每 5 秒发一次 ping
		PermitWithoutStream: true,            // 允许在没有活跃 RPC 流时发送 ping
	}
	lis, err := net.Listen("tcp", sc.GrpcServerConfig.Addr)
	if err != nil {
		sc.Logger.Fatal("grpc server 监听失败", zap.Error(err))
	}
	infoReportServer := &InfoReportServer{SC: sc}
	jobExecReport := &JobExecServer{SC: sc, taskCache: taskCache}
	s := grpc.NewServer(grpc.KeepaliveEnforcementPolicy(kaep))
	pbms.RegisterInfoReporterServer(s, infoReportServer)
	pbms.RegisterJobExecServer(s, jobExecReport)
	sc.Logger.Info("grpc server 启动", zap.Any("监听地址", lis.Addr()))
	err = s.Serve(lis)
	if err != nil {
		sc.Logger.Info("grpc server 启动失败", zap.Any("监听地址", lis.Addr()), zap.Error(err))
	}
	return err
}

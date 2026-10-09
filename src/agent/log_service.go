package agent

import (
	"bigdevops/src/config"
	"bigdevops/src/pbms"
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type AgentLogServer struct {
	pbms.UnimplementedAgentLogServiceServer
	sc *config.AgentConfig
}

// PullRealtimeLog 服务端主动发起请求，Agent 被动建立流并持续回传日志
func (s *AgentLogServer) PullRealtimeLog(req *pbms.LogPullRequest, stream pbms.AgentLogService_PullRealtimeLogServer) error {
	ctx := stream.Context()
	tailLines := req.GetTailLines()
	if tailLines <= 0 {
		tailLines = 200
	}

	var cmd *exec.Cmd

	switch req.GetLogType() {
	case "bin":
		logPath := strings.TrimSpace(req.GetLogPath())
		if logPath == "" {
			return stream.Send(&pbms.LogChunkResponse{
				IsError:  true,
				ErrorMsg: "日志文件路径不能为空",
			})
		}

		// 安全防御：校验路径是否包含非法命令拼接字符
		if strings.ContainsAny(logPath, ";|&$><`\\") {
			return stream.Send(&pbms.LogChunkResponse{
				IsError:  true,
				ErrorMsg: "非法的日志路径格式，包含危险字符",
			})
		}

		// 检查本地文件是否存在
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			return stream.Send(&pbms.LogChunkResponse{
				IsError:  true,
				ErrorMsg: fmt.Sprintf("指定的目标日志文件不存在: %s", logPath),
			})
		}

		s.sc.Logger.Info("[实时日志] 服务端主动拉取二进制日志",
			zap.String("logPath", logPath),
			zap.Int32("tailLines", tailLines),
		)

		// 启动系统的 tail 进程实时读取
		if req.GetFollow() {
			cmd = exec.CommandContext(ctx, "tail", "-n", strconv.Itoa(int(tailLines)), "-F", logPath)
		} else {
			cmd = exec.CommandContext(ctx, "tail", "-n", strconv.Itoa(int(tailLines)), logPath)
		}

	case "docker":
		container := strings.TrimSpace(req.GetContainerName())
		if container == "" {
			return stream.Send(&pbms.LogChunkResponse{
				IsError:  true,
				ErrorMsg: "Docker 容器名称或 ID 不能为空",
			})
		}

		// 安全防御
		if strings.ContainsAny(container, ";|&$><`\\") {
			return stream.Send(&pbms.LogChunkResponse{
				IsError:  true,
				ErrorMsg: "非法的容器名称",
			})
		}

		s.sc.Logger.Info("[实时日志] 服务端主动拉取 Docker 容器日志",
			zap.String("container", container),
			zap.Int32("tailLines", tailLines),
		)

		args := []string{"logs", "--tail", strconv.Itoa(int(tailLines))}
		if req.GetFollow() {
			args = append(args, "-f")
		}
		args = append(args, container)
		cmd = exec.CommandContext(ctx, "docker", args...)

	default:
		return stream.Send(&pbms.LogChunkResponse{
			IsError:  true,
			ErrorMsg: fmt.Sprintf("不支持的运行时日志类型: %s (仅支持 bin 或 docker)", req.GetLogType()),
		})
	}

	// 捕获标准输出和标准错误合并流
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.sc.Logger.Error("创建 stdout pipe 失败", zap.Error(err))
		return stream.Send(&pbms.LogChunkResponse{IsError: true, ErrorMsg: err.Error()})
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		s.sc.Logger.Error("启动日志子进程失败", zap.Error(err))
		errMsg := err.Error()
		if strings.Contains(errMsg, "docker") && strings.Contains(errMsg, "not found") {
			errMsg = "未在系统 PATH 中找到 docker 命令。若 Agent 运行在容器内，请在 compose 中挂载 /var/run/docker.sock 及 /usr/bin/docker，或使用安装了 docker-cli 的镜像"
		}
		return stream.Send(&pbms.LogChunkResponse{IsError: true, ErrorMsg: errMsg})
	}

	// 确保在退出、报错或 Context 取消时杀掉子进程
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// 使用 bufio.Reader 按行读取，确保多字节字符（如中文 UTF-8）不被截断导致前端 WebSocket 解码报错
	reader := bufio.NewReader(stdout)
	for {
		select {
		case <-ctx.Done():
			s.sc.Logger.Info("[实时日志] 服务端已断开连接，停止读取日志")
			return nil
		default:
			line, rErr := reader.ReadBytes('\n')
			if len(line) > 0 {
				if sErr := stream.Send(&pbms.LogChunkResponse{Content: line}); sErr != nil {
					s.sc.Logger.Warn("[实时日志] 发送日志块失败或流已终止", zap.Error(sErr))
					return sErr
				}
			}
			if rErr != nil {
				if rErr == io.EOF {
					return nil
				}
				s.sc.Logger.Error("读取管道出错", zap.Error(rErr))
				return rErr
			}
		}
	}
}

// StartAgentGrpcServer 在 Agent 端启动 gRPC 服务，监听来自服务端的拉取请求
func StartAgentGrpcServer(sc *config.AgentConfig) error {
	lis, err := net.Listen("tcp", sc.GrpcAddr)
	if err != nil {
		sc.Logger.Error("Agent gRPC Server 监听失败", zap.String("addr", sc.GrpcAddr), zap.Error(err))
		return err
	}

	s := grpc.NewServer()
	pbms.RegisterAgentLogServiceServer(s, &AgentLogServer{sc: sc})

	sc.Logger.Info("Agent gRPC Server 启动成功 [就绪等待服务端主动拉取日志]", zap.String("addr", sc.GrpcAddr))
	return s.Serve(lis)
}

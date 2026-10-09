package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/pbms"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// wsBaselineRealtimeLogs 服务基线统一实时日志 WebSocket 接口
// 支持:
// 1. bin: 目标机器运行的二进制进程日志 (服务端主动通过 gRPC 连接 Agent 按需拉取，可配置 logPath)
// 2. docker: 宿主机运行的 Docker 容器日志 (服务端主动通过 gRPC 连接 Agent 拉取)
// 3. k8s: K8s 集群 Pod 日志 (通过 client-go Pod Log Stream 持续拉取)
func wsBaselineRealtimeLogs(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	runtimeType := strings.ToLower(strings.TrimSpace(c.Query("runtimeType")))
	tailLinesStr := c.DefaultQuery("tailLines", "200")
	tailLines, _ := strconv.Atoi(tailLinesStr)
	if tailLines <= 0 {
		tailLines = 200
	}

	wsConn, err := podExecUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		sc.Logger.Error("服务基线 WebSocket 升级握手失败", zap.Error(err))
		return
	}
	defer wsConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听前端主动断开事件，及时释放底层连接与子进程
	go func() {
		for {
			if _, _, err := wsConn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	switch runtimeType {
	case "bin", "docker":
		hostIp := strings.TrimSpace(c.Query("hostIp"))
		grpcPort := c.DefaultQuery("grpcPort", "9091")
		logPath := strings.TrimSpace(c.Query("logPath"))
		containerName := strings.TrimSpace(c.Query("containerName"))

		if hostIp == "" {
			_ = wsConn.WriteMessage(websocket.TextMessage, []byte("[错误] hostIp 不能为空\r\n"))
			return
		}
		if runtimeType == "bin" && logPath == "" {
			_ = wsConn.WriteMessage(websocket.TextMessage, []byte("[错误] bin 模式必须配置有效的日志文件绝对路径 logPath\r\n"))
			return
		}
		if runtimeType == "docker" && containerName == "" {
			_ = wsConn.WriteMessage(websocket.TextMessage, []byte("[错误] docker 模式必须配置容器名称 containerName\r\n"))
			return
		}

		targetAgentAddr := fmt.Sprintf("%s:%s", hostIp, grpcPort)
		_ = wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\x1b[36m[系统提示] 正在连接目标主机 Agent (%s) 主动拉取实时日志...\x1b[0m\r\n", targetAgentAddr)))

		// 1. 服务端作为客户端，主动通过 gRPC 连接 Agent
		connCtx, connCancel := context.WithTimeout(ctx, 5*time.Second)
		grpcConn, err := grpc.DialContext(connCtx, targetAgentAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		connCancel()
		if err != nil {
			sc.Logger.Error("服务端主动连接 Agent gRPC 失败", zap.String("agentAddr", targetAgentAddr), zap.Error(err))
			_ = wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\x1b[31m[错误] 连接目标主机 Agent 失败 (%s): %v, 请确认 Agent 已启动并开放 %s 端口\x1b[0m\r\n", targetAgentAddr, err, grpcPort)))
			return
		}
		defer grpcConn.Close()

		// 2. 调用 Agent 的 PullRealtimeLog 服务端流接口
		agentClient := pbms.NewAgentLogServiceClient(grpcConn)
		logStream, err := agentClient.PullRealtimeLog(ctx, &pbms.LogPullRequest{
			LogType:       runtimeType,
			LogPath:       logPath,
			ContainerName: containerName,
			TailLines:     int32(tailLines),
			Follow:        true,
		})
		if err != nil {
			sc.Logger.Error("发起实时日志流拉取失败", zap.Error(err))
			_ = wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\x1b[31m[错误] 发起日志拉取失败: %v\x1b[0m\r\n", err)))
			return
		}

		_ = wsConn.WriteMessage(websocket.TextMessage, []byte("\x1b[32m[系统提示] 成功挂载实时日志流 (按需拉取模式，无操作时自动释放)...\x1b[0m\r\n"))

		// 3. 持续将 Agent 回送的增量日志转写到 WebSocket
		for {
			chunk, err := logStream.Recv()
			if err != nil {
				if err == io.EOF || ctx.Err() != nil {
					_ = wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[33m[系统提示] 实时日志流已关闭\x1b[0m\r\n"))
					return
				}
				sc.Logger.Warn("接收 Agent 日志块出错", zap.Error(err))
				_ = wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[31m[流中断] %v\x1b[0m\r\n", err)))
				return
			}

			if chunk.GetIsError() {
				_ = wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\x1b[31m[Agent 报错] %s\x1b[0m\r\n", chunk.GetErrorMsg())))
				return
			}

			if len(chunk.GetContent()) > 0 {
				if err := wsConn.WriteMessage(websocket.TextMessage, chunk.GetContent()); err != nil {
					return
				}
			}
		}

	case "k8s":
		clusterName := strings.TrimSpace(c.Query("clusterName"))
		namespace := strings.TrimSpace(c.Query("namespace"))
		podName := strings.TrimSpace(c.Query("podName"))
		containerName := strings.TrimSpace(c.Query("containerName"))

		if clusterName == "" || namespace == "" || podName == "" {
			_ = wsConn.WriteMessage(websocket.TextMessage, []byte("[错误] K8s 模式下 clusterName、namespace 和 podName 不能为空\r\n"))
			return
		}

		_, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
		if err != nil {
			_ = wsConn.WriteMessage(websocket.TextMessage, []byte("获取集群配置失败: "+err.Error()))
			return
		}

		_, kSetStream, _, err := common.GenK8sClientSetByKubeconfigContent(dbCluster.KubeConfigContent, 0)
		if err != nil {
			_ = wsConn.WriteMessage(websocket.TextMessage, []byte("生成 K8s Stream 客户端失败: "+err.Error()))
			return
		}

		if containerName == "" {
			tCtx, tCancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
			p, pErr := kSetStream.CoreV1().Pods(namespace).Get(tCtx, podName, metav1.GetOptions{})
			tCancel()
			if pErr == nil && p != nil && len(p.Spec.Containers) > 0 {
				containerName = p.Spec.Containers[0].Name
			}
		}

		tailLinesInt64 := int64(tailLines)
		opts := &v1.PodLogOptions{
			Container: containerName,
			TailLines: &tailLinesInt64,
			Follow:    true,
		}

		req := kSetStream.CoreV1().Pods(namespace).GetLogs(podName, opts)
		stream, err := req.Stream(ctx)
		if err != nil {
			_ = wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\x1b[31m打开 K8s Pod 日志流失败: %v\x1b[0m\r\n", err)))
			return
		}
		defer stream.Close()

		_ = wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\x1b[32m[系统提示] 成功挂载 K8s Pod [%s/%s] 容器 (%s) 实时日志流...\x1b[0m\r\n", namespace, podName, containerName)))

		buf := make([]byte, 2048)
		for {
			n, err := stream.Read(buf)
			if n > 0 {
				if wErr := wsConn.WriteMessage(websocket.TextMessage, buf[:n]); wErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}

	default:
		_ = wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("未知的 runtimeType: %s, 仅支持 bin、docker、k8s\r\n", runtimeType)))
		return
	}
}

package view_server

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	yaml3 "gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

type OnePod struct {
	Name           string   `json:"name"`
	Namespace      string   `json:"namespace"`
	Status         string   `json:"status"`
	Ready          string   `json:"ready"`          // "1/1"
	ReadyContainer int      `json:"readyContainer"` // 1
	TotalContainer int      `json:"totalContainer"` // 1
	PodIP          string   `json:"podIP"`
	HostIP         string   `json:"hostIP"`
	NodeName       string   `json:"nodeName"`
	Restarts       int32    `json:"restarts"`
	LastRestartAgo string   `json:"lastRestartAgo"` // "(48d ago)"
	Age            string   `json:"age"`            // "211 天"
	Containers     []string `json:"containers"`     // 原生 K8s 容器名称 (c.Name)
	Images         []string `json:"images"`
	CreatedAt      string   `json:"createdAt"`
}

type K8sCreatePodReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	YamlContent string `json:"yamlContent"`
}

type K8sDeletePodReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

type K8sPodItem struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type K8sDeletePodBatchReq struct {
	ClusterName string       `json:"clusterName"`
	Namespace   string       `json:"namespace"`
	Names       []string     `json:"names"`
	Items       []K8sPodItem `json:"items"`
}

func podConvert(p *v1.Pod) *OnePod {
	if p == nil {
		return nil
	}
	status := string(p.Status.Phase)
	if p.DeletionTimestamp != nil {
		status = "Terminating"
	} else {
		for _, cs := range p.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
				status = cs.State.Waiting.Reason
				break
			}
		}
	}

	totalContainers := len(p.Spec.Containers)
	readyContainers := 0
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Ready {
			readyContainers++
		}
	}
	readyStr := fmt.Sprintf("%d/%d", readyContainers, totalContainers)

	var restarts int32
	var latestFinishedAt time.Time
	for _, cs := range p.Status.ContainerStatuses {
		restarts += cs.RestartCount
		if cs.LastTerminationState.Terminated != nil && !cs.LastTerminationState.Terminated.FinishedAt.IsZero() {
			if cs.LastTerminationState.Terminated.FinishedAt.Time.After(latestFinishedAt) {
				latestFinishedAt = cs.LastTerminationState.Terminated.FinishedAt.Time
			}
		}
	}

	lastRestartAgo := ""
	if restarts > 0 && !latestFinishedAt.IsZero() {
		dur := time.Since(latestFinishedAt)
		if days := int(dur.Hours() / 24); days > 0 {
			lastRestartAgo = fmt.Sprintf("(%dd ago)", days)
		} else if hours := int(dur.Hours()); hours > 0 {
			lastRestartAgo = fmt.Sprintf("(%dh ago)", hours)
		} else if mins := int(dur.Minutes()); mins > 0 {
			lastRestartAgo = fmt.Sprintf("(%dm ago)", mins)
		} else {
			lastRestartAgo = fmt.Sprintf("(%ds ago)", int(dur.Seconds()))
		}
	}

	ageStr := "-"
	if !p.CreationTimestamp.IsZero() {
		dur := time.Since(p.CreationTimestamp.Time)
		if days := int(dur.Hours() / 24); days > 0 {
			ageStr = fmt.Sprintf("%d 天", days)
		} else if hours := int(dur.Hours()); hours > 0 {
			ageStr = fmt.Sprintf("%d 小时", hours)
		} else if mins := int(dur.Minutes()); mins > 0 {
			ageStr = fmt.Sprintf("%d 分钟", mins)
		} else {
			ageStr = fmt.Sprintf("%d 秒", int(dur.Seconds()))
		}
	}

	var containers []string
	var images []string
	for _, c := range p.Spec.Containers {
		containers = append(containers, c.Name)
		images = append(images, c.Image)
	}
	createdAt := ""
	if !p.CreationTimestamp.IsZero() {
		createdAt = p.CreationTimestamp.Format("2006-01-02 15:04:05")
	}

	return &OnePod{
		Name:           p.Name,
		Namespace:      p.Namespace,
		Status:         status,
		Ready:          readyStr,
		ReadyContainer: readyContainers,
		TotalContainer: totalContainers,
		PodIP:          p.Status.PodIP,
		HostIP:         p.Status.HostIP,
		NodeName:       p.Spec.NodeName,
		Restarts:       restarts,
		LastRestartAgo: lastRestartAgo,
		Age:            ageStr,
		Containers:     containers,
		Images:         images,
		CreatedAt:      createdAt,
	}
}

// getClusterClientsetHelper 获取 K8s ClientSet 辅助函数
func getClusterClientsetHelper(c *gin.Context, clusterName string) (*kubernetes.Clientset, *dynamic.DynamicClient, *models.K8sCluster, error) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	dbCluster, err := models.GetK8sClusterByName(clusterName)
	if err != nil {
		sc.Logger.Error("根据name找k8s集群错误", zap.String("clusterName", clusterName), zap.Error(err))
		return nil, nil, nil, err
	}

	kc := c.MustGet(common.GIN_CTX_K8S_CACHE).(*cache.K8sClusterCache)
	kSet := kc.GetClusterClientSetById(dbCluster.ID)
	dSet := kc.GetClusterDynamicClientById(dbCluster.ID)

	if kSet == nil || dSet == nil {
		restConfig, clientSet, _, err := common.GenK8sClientSetByKubeconfigContent(dbCluster.KubeConfigContent, dbCluster.ActionTimeoutSeconds)
		if err != nil {
			return nil, nil, dbCluster, err
		}
		dyClient, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			return nil, nil, dbCluster, err
		}
		kSet = clientSet
		dSet = dyClient
	}
	return kSet, dSet, dbCluster, nil
}

// getK8sNamespaceList 获取指定 K8s 集群的命名空间列表
func getK8sNamespaceList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	if clusterName == "" {
		common.FailWithMessage("clusterName 参数不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		sc.Logger.Error("获取集群 ClientSet 失败", zap.String("clusterName", clusterName), zap.Error(err))
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	nsList, err := kSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		sc.Logger.Error("查询命名空间列表失败", zap.String("clusterName", clusterName), zap.Error(err))
		common.FailWithMessage("查询命名空间列表失败: "+err.Error(), c)
		return
	}

	var res []string
	for _, ns := range nsList.Items {
		res = append(res, ns.Name)
	}

	common.OkWithDetailed(res, "ok", c)
}

// getK8sPodList 获取指定 K8s 集群与命名空间的 Pod 列表
func getK8sPodList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")
	keyword := strings.TrimSpace(c.Query("keyword"))

	if clusterName == "" {
		common.FailWithMessage("clusterName 参数不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		sc.Logger.Error("获取集群 ClientSet 失败", zap.String("clusterName", clusterName), zap.Error(err))
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	podList, err := kSet.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		sc.Logger.Error("获取 Pod 列表失败", zap.String("clusterName", clusterName), zap.String("namespace", namespace), zap.Error(err))
		common.FailWithMessage("获取 Pod 列表失败: "+err.Error(), c)
		return
	}

	var items []*OnePod
	for i := range podList.Items {
		p := &podList.Items[i]
		if keyword != "" {
			if !strings.Contains(p.Name, keyword) && !strings.Contains(p.Status.PodIP, keyword) && !strings.Contains(p.Spec.NodeName, keyword) {
				continue
			}
		}

		podObj := podConvert(p)
		if podObj != nil {
			items = append(items, podObj)
		}
	}

	common.OkWithDetailed(gin.H{
		"items": items,
		"total": len(items),
	}, "ok", c)
}

// getK8sPodYaml 获取单个 Pod 的完整 YAML 源码
func getK8sPodYaml(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")
	name := c.Query("name")

	if clusterName == "" || namespace == "" || name == "" {
		common.FailWithMessage("clusterName、namespace 和 name 参数不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	pod, err := kSet.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 Pod 详情失败", zap.String("name", name), zap.Error(err))
		common.FailWithMessage("获取 Pod 详情失败: "+err.Error(), c)
		return
	}

	podBytes, err := json.Marshal(pod)
	if err != nil {
		common.FailWithMessage("序列化 Pod 失败: "+err.Error(), c)
		return
	}

	var mapObj map[string]interface{}
	_ = json.Unmarshal(podBytes, &mapObj)

	// 🚀 Client-Go typed Struct 序列化时不会自动带 apiVersion 和 kind，此处手动补全保证标准 YAML 格式
	if mapObj["apiVersion"] == nil || mapObj["apiVersion"] == "" {
		mapObj["apiVersion"] = "v1"
	}
	if mapObj["kind"] == nil || mapObj["kind"] == "" {
		mapObj["kind"] = "Pod"
	}

	delete(mapObj, "status") // 移除运行时状态冗余字段

	if metadata, ok := mapObj["metadata"].(map[string]interface{}); ok {
		delete(metadata, "managedFields")
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "generation")
		delete(metadata, "creationTimestamp")
	}

	yamlBytes, err := yaml3.Marshal(mapObj)
	if err != nil {
		common.FailWithMessage("转换为 YAML 失败: "+err.Error(), c)
		return
	}

	common.OkWithDetailed(string(yamlBytes), "ok", c)
}

// createK8sPod 新建 Pod (动态 Apply)
func createK8sPod(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sCreatePodReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("请求参数格式解析失败: "+err.Error(), c)
		return
	}

	if reqObj.ClusterName == "" || reqObj.YamlContent == "" {
		common.FailWithMessage("集群名称与 YAML 内容不能为空", c)
		return
	}

	kSet, dSet, dbCluster, err := getClusterClientsetHelper(c, reqObj.ClusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	err = DynamicObjApply(kSet, dSet, dbCluster.ActionTimeoutSeconds, []byte(reqObj.YamlContent))
	if err != nil {
		sc.Logger.Error("创建 Pod 失败", zap.Error(err))
		common.FailWithMessage("创建 Pod 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("Pod 创建/应用成功", c)
}

// updateK8sPod 更新 Pod (动态 Apply)
func updateK8sPod(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sCreatePodReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("请求参数格式解析失败: "+err.Error(), c)
		return
	}

	if reqObj.ClusterName == "" || reqObj.YamlContent == "" {
		common.FailWithMessage("集群名称与 YAML 内容不能为空", c)
		return
	}

	kSet, dSet, dbCluster, err := getClusterClientsetHelper(c, reqObj.ClusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	err = DynamicObjApply(kSet, dSet, dbCluster.ActionTimeoutSeconds, []byte(reqObj.YamlContent))
	if err != nil {
		sc.Logger.Error("更新 Pod 失败", zap.Error(err))
		common.FailWithMessage("更新 Pod 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("Pod 更新成功", c)
}

// deleteK8sPod 删除单个 Pod
func deleteK8sPod(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sDeletePodReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("请求参数格式解析失败: "+err.Error(), c)
		return
	}

	if reqObj.ClusterName == "" || reqObj.Namespace == "" || reqObj.Name == "" {
		common.FailWithMessage("集群名称、命名空间与 Pod 名称不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, reqObj.ClusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	err = kSet.CoreV1().Pods(reqObj.Namespace).Delete(ctx, reqObj.Name, metav1.DeleteOptions{})
	if err != nil {
		sc.Logger.Error("删除 Pod 失败", zap.String("pod", reqObj.Name), zap.Error(err))
		common.FailWithMessage("删除 Pod 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("Pod [%s] 删除请求已提交", reqObj.Name), c)
}

// deleteK8sPodBatch 批量删除 Pod
func deleteK8sPodBatch(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sDeletePodBatchReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("请求参数格式解析失败: "+err.Error(), c)
		return
	}

	if reqObj.ClusterName == "" {
		common.FailWithMessage("集群名称不能为空", c)
		return
	}

	var targets []K8sPodItem
	if len(reqObj.Items) > 0 {
		targets = reqObj.Items
	} else if len(reqObj.Names) > 0 {
		for _, name := range reqObj.Names {
			targets = append(targets, K8sPodItem{
				Namespace: reqObj.Namespace,
				Name:      name,
			})
		}
	}

	if len(targets) == 0 {
		common.FailWithMessage("要删除的 Pod 列表不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, reqObj.ClusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	var successNames []string
	var failNames []string

	for _, item := range targets {
		if item.Name == "" {
			continue
		}
		ns := item.Namespace
		if ns == "" {
			ns = reqObj.Namespace
		}
		if ns == "" {
			p, err := kSet.CoreV1().Pods("").Get(ctx, item.Name, metav1.GetOptions{})
			if err == nil && p != nil {
				ns = p.Namespace
			}
		}
		if ns == "" {
			ns = "default"
		}

		err := kSet.CoreV1().Pods(ns).Delete(ctx, item.Name, metav1.DeleteOptions{})
		if err != nil {
			sc.Logger.Error("批量删除中单个 Pod 删除失败", zap.String("name", item.Name), zap.Error(err))
			failNames = append(failNames, item.Name)
		} else {
			successNames = append(successNames, item.Name)
		}
	}

	common.OkWithDetailed(gin.H{
		"successCount": len(successNames),
		"failCount":    len(failNames),
		"successNames": successNames,
		"failNames":    failNames,
	}, "批量删除完成", c)
}

// getK8sPodLogs 获取 Pod 日志
func getK8sPodLogs(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")
	name := c.Query("name")
	container := c.Query("container")
	tailLinesStr := c.Query("tailLines")

	if clusterName == "" || namespace == "" || name == "" {
		common.FailWithMessage("clusterName、namespace 和 name 不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	tailLines := int64(200)
	if tailLinesStr != "" {
		if t, err := strconv.ParseInt(tailLinesStr, 10, 64); err == nil && t > 0 {
			tailLines = t
		}
	}

	// 如果未指定 container，先获取 Pod 信息的第一个容器
	if container == "" {
		ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
		p, err := kSet.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		cancel()
		if err == nil && p != nil && len(p.Spec.Containers) > 0 {
			container = p.Spec.Containers[0].Name
		}
	}

	opts := &v1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	req := kSet.CoreV1().Pods(namespace).GetLogs(name, opts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		sc.Logger.Error("获取 Pod 日志失败", zap.String("pod", name), zap.Error(err))
		common.FailWithMessage("获取 Pod 日志失败: "+err.Error(), c)
		return
	}
	defer podLogs.Close()

	logBytes, err := io.ReadAll(podLogs)
	if err != nil {
		common.FailWithMessage("读取 Pod 日志数据失败: "+err.Error(), c)
		return
	}

	common.OkWithDetailed(gin.H{
		"log":       string(logBytes),
		"container": container,
	}, "ok", c)
}

var podExecUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type TerminalMessage struct {
	Op   string `json:"op"` // "stdin" | "resize"
	Data string `json:"data"`
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

type TerminalSession struct {
	wsConn   *websocket.Conn
	sizeChan chan remotecommand.TerminalSize
}

func (t *TerminalSession) Read(p []byte) (int, error) {
	_, message, err := t.wsConn.ReadMessage()
	if err != nil {
		return 0, err
	}
	var msg TerminalMessage
	if err := json.Unmarshal(message, &msg); err == nil {
		if msg.Op == "resize" && msg.Cols > 0 && msg.Rows > 0 {
			t.sizeChan <- remotecommand.TerminalSize{Width: msg.Cols, Height: msg.Rows}
			return 0, nil
		}
		if msg.Op == "stdin" || msg.Op == "" {
			copy(p, []byte(msg.Data))
			return len(msg.Data), nil
		}
	}
	copy(p, message)
	return len(message), nil
}

func (t *TerminalSession) Write(p []byte) (int, error) {
	err := t.wsConn.WriteMessage(websocket.TextMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (t *TerminalSession) Next() *remotecommand.TerminalSize {
	size, ok := <-t.sizeChan
	if !ok || size.Width == 0 || size.Height == 0 {
		return nil
	}
	return &size
}

// wsK8sPodExec 在线 Exec 终端 WebSocket 接口
func wsK8sPodExec(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")
	name := c.Query("name")
	container := c.Query("container")

	if clusterName == "" || namespace == "" || name == "" {
		common.FailWithMessage("clusterName、namespace 与 name 参数不能为空", c)
		return
	}

	wsConn, err := podExecUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		sc.Logger.Error("WebSocket 握手失败", zap.Error(err))
		return
	}
	defer wsConn.Close()

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		_ = wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m获取集群客户端失败: "+err.Error()+"\x1b[0m\r\n"))
		return
	}

	restConfig, _, _, err := common.GenK8sClientSetByKubeconfigContent(dbCluster.KubeConfigContent, dbCluster.ActionTimeoutSeconds)
	if err != nil {
		_ = wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m生成 RESTConfig 失败: "+err.Error()+"\x1b[0m\r\n"))
		return
	}

	if container == "" {
		ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
		p, err := kSet.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		cancel()
		if err == nil && p != nil && len(p.Spec.Containers) > 0 {
			container = p.Spec.Containers[0].Name
		}
	}

	shell := c.Query("shell")
	cmdList := []string{"/bin/sh", "-c", "TERM=xterm-256color exec /bin/bash || exec /bin/sh || exec sh"}
	switch strings.ToLower(shell) {
	case "bash":
		cmdList = []string{"/bin/sh", "-c", "TERM=xterm-256color exec /bin/bash || exec bash"}
	case "sh":
		cmdList = []string{"/bin/sh", "-c", "TERM=xterm-256color exec /bin/sh || exec sh"}
	case "dash":
		cmdList = []string{"/bin/sh", "-c", "TERM=xterm-256color exec /bin/dash || exec dash"}
	case "zsh":
		cmdList = []string{"/bin/sh", "-c", "TERM=xterm-256color exec /bin/zsh || exec zsh"}
	}

	req := kSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(name).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: container,
			Command:   cmdList,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		_ = wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m创建 SPDY Terminal 执行器失败: "+err.Error()+"\x1b[0m\r\n"))
		return
	}

	session := &TerminalSession{
		wsConn:   wsConn,
		sizeChan: make(chan remotecommand.TerminalSize, 10),
	}

	err = exec.StreamWithContext(c.Request.Context(), remotecommand.StreamOptions{
		Stdin:             session,
		Stdout:            session,
		Stderr:            session,
		Tty:               true,
		TerminalSizeQueue: session,
	})
	if err != nil {
		_ = wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mTerminal 会话结束: "+err.Error()+"\x1b[0m\r\n"))
	}
}

// wsK8sPodWatch 类似于 kubectl get pod -w 的实时 Watch WebSocket 接口
func wsK8sPodWatch(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")

	if clusterName == "" {
		common.FailWithMessage("clusterName 参数不能为空", c)
		return
	}

	wsConn, err := podExecUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		sc.Logger.Error("WebSocket Watch 握手失败", zap.Error(err))
		return
	}
	defer wsConn.Close()

	_, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		sc.Logger.Error("获取集群 ClientSet 失败", zap.String("clusterName", clusterName), zap.Error(err))
		return
	}

	_, kSetStream, _, err := common.GenK8sClientSetByKubeconfigContent(dbCluster.KubeConfigContent, 0)
	if err != nil {
		sc.Logger.Error("生成 K8s Stream 客户端失败", zap.Error(err))
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			if _, _, err := wsConn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// 1. 先 List 获取当前集群最新的 ResourceVersion
	podList, err := kSetStream.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		sc.Logger.Error("Watch 前获取 Pod 列表失败", zap.Error(err))
		_ = wsConn.WriteJSON(gin.H{"type": "ERROR", "message": "获取 Pod 列表失败: " + err.Error()})
		return
	}

	opts := metav1.ListOptions{
		ResourceVersion: podList.ResourceVersion,
	}

	watcher, err := kSetStream.CoreV1().Pods(namespace).Watch(ctx, opts)
	if err != nil {
		sc.Logger.Error("创建 Pod Watch 失败", zap.String("clusterName", clusterName), zap.String("namespace", namespace), zap.Error(err))
		_ = wsConn.WriteJSON(gin.H{"type": "ERROR", "message": "创建 Watch 失败: " + err.Error()})
		return
	}
	defer watcher.Stop()

	sc.Logger.Info("Pod Watch 成功建立并开启事件推送", zap.String("cluster", clusterName), zap.String("namespace", namespace))

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				sc.Logger.Warn("Pod Watch Channel 已关闭", zap.String("cluster", clusterName))
				return
			}
			p, ok := event.Object.(*v1.Pod)
			if !ok || p == nil {
				continue
			}

			podObj := podConvert(p)
			if podObj == nil {
				continue
			}

			err := wsConn.WriteJSON(gin.H{
				"type": string(event.Type), // "ADDED" | "MODIFIED" | "DELETED"
				"pod":  podObj,
			})
			if err != nil {
				return
			}
		}
	}
}

// wsK8sPodLogs 类似于 kubectl logs -f 的 WebSocket 实时日志持续流接口
func wsK8sPodLogs(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")
	name := c.Query("name")
	container := c.Query("container")
	tailLinesStr := c.Query("tailLines")

	if clusterName == "" || namespace == "" || name == "" {
		common.FailWithMessage("clusterName、namespace 和 name 不能为空", c)
		return
	}

	wsConn, err := podExecUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		sc.Logger.Error("WebSocket Logs 握手失败", zap.Error(err))
		return
	}
	defer wsConn.Close()

	_, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		_ = wsConn.WriteMessage(websocket.TextMessage, []byte("获取集群配置失败: "+err.Error()))
		return
	}

	// 🚀 核心关键修复：建立 Timeout = 0 (无超时限制) 的 K8s ClientSet，防止 HTTP Client 30s 自动切断流
	_, kSetStream, _, err := common.GenK8sClientSetByKubeconfigContent(dbCluster.KubeConfigContent, 0)
	if err != nil {
		_ = wsConn.WriteMessage(websocket.TextMessage, []byte("生成 K8s Stream 客户端失败: "+err.Error()))
		return
	}

	tailLines := int64(200)
	if tailLinesStr != "" {
		if t, err := strconv.ParseInt(tailLinesStr, 10, 64); err == nil && t > 0 {
			tailLines = t
		}
	}

	if container == "" {
		ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
		p, err := kSetStream.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		cancel()
		if err == nil && p != nil && len(p.Spec.Containers) > 0 {
			container = p.Spec.Containers[0].Name
		}
	}

	opts := &v1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
		Follow:    true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听前端 WebSocket 关闭事件，实时释放 stream 连接
	go func() {
		for {
			if _, _, err := wsConn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	req := kSetStream.CoreV1().Pods(namespace).GetLogs(name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		sc.Logger.Error("获取 Pod 日志流失败", zap.String("pod", name), zap.Error(err))
		_ = wsConn.WriteMessage(websocket.TextMessage, []byte("获取 Pod 日志流失败: "+err.Error()))
		return
	}
	defer stream.Close()

	buf := make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := stream.Read(buf)
			if n > 0 {
				err := wsConn.WriteMessage(websocket.TextMessage, buf[:n])
				if err != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					_ = wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n[日志流结束: "+err.Error()+"]"))
				}
				return
			}
		}
	}
}

// downloadK8sPodFile 下载 Pod 容器内部的文件
func downloadK8sPodFile(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")
	name := c.Query("name")
	container := c.Query("container")
	path := c.Query("path")

	if clusterName == "" || namespace == "" || name == "" || path == "" {
		common.FailWithMessage("clusterName、namespace、name 和 path 不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	restConfig, _, _, err := common.GenK8sClientSetByKubeconfigContent(dbCluster.KubeConfigContent, dbCluster.ActionTimeoutSeconds)
	if err != nil {
		common.FailWithMessage("生成 RESTConfig 失败: "+err.Error(), c)
		return
	}

	if container == "" {
		ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
		p, err := kSet.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		cancel()
		if err == nil && p != nil && len(p.Spec.Containers) > 0 {
			container = p.Spec.Containers[0].Name
		}
	}

	filename := filepath.Base(path)
	if filename == "" || filename == "." || filename == "/" {
		filename = "download_file"
	}

	cmdList := []string{"cat", path}

	req := kSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(name).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: container,
			Command:   cmdList,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		sc.Logger.Error("创建文件下载 SPDY 执行器失败", zap.Error(err))
		common.FailWithMessage("创建文件下载执行器失败: "+err.Error(), c)
		return
	}

	var stderrBuf bytes.Buffer
	var stdoutBuf bytes.Buffer

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
		Tty:    false,
	})

	if err != nil || stderrBuf.Len() > 0 {
		errMsg := stderrBuf.String()
		if errMsg == "" && err != nil {
			errMsg = err.Error()
		}
		sc.Logger.Error("容器文件读取失败", zap.String("path", path), zap.String("err", errMsg))
		common.FailWithMessage("容器文件读取失败: "+errMsg, c)
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(200, "application/octet-stream", stdoutBuf.Bytes())
}

// execContainerCmdRaw 在容器内简易同步执行 Shell 命令并返回字节流
func execContainerCmdRaw(c *gin.Context, clusterName, namespace, podName, containerName string, cmdList []string, stdin io.Reader) ([]byte, []byte, error) {
	kSet, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		return nil, nil, err
	}

	restConfig, _, _, err := common.GenK8sClientSetByKubeconfigContent(dbCluster.KubeConfigContent, dbCluster.ActionTimeoutSeconds)
	if err != nil {
		return nil, nil, err
	}

	if containerName == "" {
		ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
		p, err := kSet.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		cancel()
		if err == nil && p != nil && len(p.Spec.Containers) > 0 {
			containerName = p.Spec.Containers[0].Name
		}
	}

	req := kSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: containerName,
			Command:   cmdList,
			Stdin:     stdin != nil,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, err
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
		Tty:    false,
	})

	return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
}

func isShellNotFoundError(err error, stderr []byte) bool {
	msg := string(stderr)
	if err != nil {
		msg += " " + err.Error()
	}
	return strings.Contains(msg, "stat /bin/sh") ||
		strings.Contains(msg, "stat /bin/bash") ||
		strings.Contains(msg, "exec: \"/bin/sh\"") ||
		strings.Contains(msg, "exec: \"/bin/bash\"") ||
		strings.Contains(msg, "exec: \"sh\"") ||
		strings.Contains(msg, "exec: \"bash\"") ||
		strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "unable to start container process")
}

// execContainerShellCmd 智能尝试使用 /bin/sh, /bin/bash, sh, bash 执行 Shell 命令
func execContainerShellCmd(c *gin.Context, clusterName, namespace, podName, containerName string, cmdStr string, stdin io.Reader) ([]byte, []byte, error) {
	shells := [][]string{
		{"/bin/sh", "-c", cmdStr},
		{"/bin/bash", "-c", cmdStr},
		{"sh", "-c", cmdStr},
		{"bash", "-c", cmdStr},
	}

	var lastStdout, lastStderr []byte
	var lastErr error

	for _, shellCmd := range shells {
		stdout, stderr, err := execContainerCmdRaw(c, clusterName, namespace, podName, containerName, shellCmd, stdin)
		if err == nil {
			return stdout, stderr, nil
		}

		if isShellNotFoundError(err, stderr) {
			lastStdout = stdout
			lastStderr = stderr
			lastErr = err
			continue
		}

		return stdout, stderr, err
	}

	return lastStdout, lastStderr, lastErr
}

type PodFileItem struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	SizeStr string `json:"sizeStr"`
	Mode    string `json:"mode"`
	ModTime string `json:"modTime"`
}

func parseLsOutputLine(line, parentPath string) *PodFileItem {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "total ") {
		return nil
	}

	fields := strings.Fields(line)
	if len(fields) < 8 {
		return nil
	}

	mode := fields[0]
	isDir := strings.HasPrefix(mode, "d")

	// 找到文件名位置 (可能文件名包含空格)
	var name string
	var size int64
	var modTime string

	// ISO 时间格式 (2026-07-22 16:30) 属于 8 字段模式或更长
	if len(fields) >= 8 && strings.Contains(fields[5], "-") {
		_ = json.Unmarshal([]byte(fields[4]), &size)
		modTime = fields[5] + " " + fields[6]
		name = strings.Join(fields[7:], " ")
	} else if len(fields) >= 9 {
		_ = json.Unmarshal([]byte(fields[4]), &size)
		modTime = fields[5] + " " + fields[6] + " " + fields[7]
		name = strings.Join(fields[8:], " ")
	} else {
		name = fields[len(fields)-1]
	}

	if name == "." || name == ".." {
		return nil
	}

	// 处理符号链接 a -> b
	if strings.Contains(name, " -> ") {
		parts := strings.Split(name, " -> ")
		name = parts[0]
	}

	fullPath := filepath.ToSlash(filepath.Join(parentPath, name))

	sizeStr := "-"
	if !isDir {
		if size >= 1024*1024*1024 {
			sizeStr = fmt.Sprintf("%.2f GB", float64(size)/(1024*1024*1024))
		} else if size >= 1024*1024 {
			sizeStr = fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
		} else if size >= 1024 {
			sizeStr = fmt.Sprintf("%.2f KB", float64(size)/1024)
		} else {
			sizeStr = fmt.Sprintf("%d B", size)
		}
	}

	return &PodFileItem{
		Name:    name,
		Path:    fullPath,
		IsDir:   isDir,
		Size:    size,
		SizeStr: sizeStr,
		Mode:    mode,
		ModTime: modTime,
	}
}

// getK8sPodFileList 获取 Pod 容器内指定目录的文件/文件夹列表 (像 Kuboard 一样文件浏览器)
func getK8sPodFileList(c *gin.Context) {
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")
	name := c.Query("name")
	container := c.Query("container")
	targetPath := c.Query("path")

	if clusterName == "" || namespace == "" || name == "" {
		common.FailWithMessage("clusterName、namespace 和 name 不能为空", c)
		return
	}

	if targetPath == "" {
		targetPath = "/"
	}

	cmdStr := fmt.Sprintf("ls -la --time-style=long-iso '%s' || ls -la '%s' || /bin/ls -la '%s' || /usr/bin/ls -la '%s' || busybox ls -la '%s'", targetPath, targetPath, targetPath, targetPath, targetPath)

	stdoutBytes, stderrBytes, err := execContainerShellCmd(c, clusterName, namespace, name, container, cmdStr, nil)
	if err != nil && len(stdoutBytes) == 0 {
		errMsg := string(stderrBytes)
		if errMsg == "" {
			errMsg = err.Error()
		}
		if isShellNotFoundError(err, stderrBytes) {
			common.FailWithMessage("容器未内置 Shell 环境(如 Distroless/Scratch 极简镜像)，无法在线浏览文件系统", c)
			return
		}
		if strings.Contains(errMsg, "ls: not found") || strings.Contains(errMsg, "ls: command not found") {
			common.FailWithMessage("容器内部缺少 ls 工具包，无法列出目录内容", c)
			return
		}
		common.FailWithMessage("读取容器目录失败: "+errMsg, c)
		return
	}

	output := string(stdoutBytes)
	lines := strings.Split(output, "\n")

	var dirs []*PodFileItem
	var files []*PodFileItem

	for _, line := range lines {
		item := parseLsOutputLine(line, targetPath)
		if item == nil {
			continue
		}
		if item.IsDir {
			dirs = append(dirs, item)
		} else {
			files = append(files, item)
		}
	}

	// 文件夹排在前面，文件排在后面
	allItems := append(dirs, files...)

	common.OkWithDetailed(gin.H{
		"path":  targetPath,
		"items": allItems,
		"total": len(allItems),
	}, "ok", c)
}

// uploadK8sPodFile 上传本地文件到容器内部指定目录
func uploadK8sPodFile(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.PostForm("clusterName")
	namespace := c.PostForm("namespace")
	name := c.PostForm("name")
	container := c.PostForm("container")
	targetDir := c.PostForm("path")

	if clusterName == "" || namespace == "" || name == "" {
		common.FailWithMessage("clusterName、namespace 和 name 不能为空", c)
		return
	}

	if targetDir == "" {
		targetDir = "/"
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		common.FailWithMessage("获取上传文件失败: "+err.Error(), c)
		return
	}

	fileStream, err := fileHeader.Open()
	if err != nil {
		common.FailWithMessage("打开上传文件流失败: "+err.Error(), c)
		return
	}
	defer fileStream.Close()

	destFilePath := filepath.ToSlash(filepath.Join(targetDir, fileHeader.Filename))
	cmdStr := fmt.Sprintf("cat > '%s'", destFilePath)

	_, stderrBytes, err := execContainerShellCmd(c, clusterName, namespace, name, container, cmdStr, fileStream)
	if err != nil || len(stderrBytes) > 0 {
		errMsg := string(stderrBytes)
		if errMsg == "" && err != nil {
			errMsg = err.Error()
		}
		if isShellNotFoundError(err, stderrBytes) {
			common.FailWithMessage("容器未内置 Shell 环境(如 Distroless/Scratch 极简镜像)，无法上传文件", c)
			return
		}
		sc.Logger.Error("文件写入容器失败", zap.String("dest", destFilePath), zap.Error(err))
		common.FailWithMessage("上传文件到容器失败: "+errMsg, c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("文件 [%s] 成功上传至容器 %s", fileHeader.Filename, destFilePath), c)
}

// deleteK8sPodFile 删除容器内的指定文件或目录
func deleteK8sPodFile(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj struct {
		ClusterName string `json:"clusterName"`
		Namespace   string `json:"namespace"`
		Name        string `json:"name"`
		Container   string `json:"container"`
		Path        string `json:"path"`
	}

	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	if reqObj.ClusterName == "" || reqObj.Namespace == "" || reqObj.Name == "" || reqObj.Path == "" {
		common.FailWithMessage("clusterName、namespace、name 和 path 不能为空", c)
		return
	}

	if reqObj.Path == "/" || reqObj.Path == "/*" {
		common.FailWithMessage("禁止直接删除根目录 /", c)
		return
	}

	cmdStr := fmt.Sprintf("rm -rf '%s'", reqObj.Path)

	_, stderrBytes, err := execContainerShellCmd(c, reqObj.ClusterName, reqObj.Namespace, reqObj.Name, reqObj.Container, cmdStr, nil)
	if err != nil || len(stderrBytes) > 0 {
		errMsg := string(stderrBytes)
		if errMsg == "" && err != nil {
			errMsg = err.Error()
		}
		if isShellNotFoundError(err, stderrBytes) {
			common.FailWithMessage("容器未内置 Shell 环境(如 Distroless/Scratch 极简镜像)，无法删除文件", c)
			return
		}
		sc.Logger.Error("删除容器文件失败", zap.String("path", reqObj.Path), zap.Error(err))
		common.FailWithMessage("删除容器文件失败: "+errMsg, c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("容器文件 [%s] 已删除", reqObj.Path), c)
}

// readK8sPodFileContent 预览/读取容器内文本文件内容
func readK8sPodFileContent(c *gin.Context) {
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")
	name := c.Query("name")
	container := c.Query("container")
	targetPath := c.Query("path")

	if clusterName == "" || namespace == "" || name == "" || targetPath == "" {
		common.FailWithMessage("clusterName、namespace、name 和 path 不能为空", c)
		return
	}

	cmdStr := fmt.Sprintf("head -c 524288 '%s' || cat '%s'", targetPath, targetPath)

	stdoutBytes, stderrBytes, err := execContainerShellCmd(c, clusterName, namespace, name, container, cmdStr, nil)
	if err != nil && len(stdoutBytes) == 0 {
		errMsg := string(stderrBytes)
		if errMsg == "" {
			errMsg = err.Error()
		}
		if isShellNotFoundError(err, stderrBytes) {
			common.FailWithMessage("容器未内置 Shell 环境(如 Distroless/Scratch 极简镜像)，无法读取文件内容", c)
			return
		}
		common.FailWithMessage("读取文件内容失败: "+errMsg, c)
		return
	}

	common.OkWithDetailed(gin.H{
		"path":    targetPath,
		"content": string(stdoutBytes),
	}, "ok", c)
}

// saveK8sPodFileContent 在线修改并保存容器内文本文件内容
func saveK8sPodFileContent(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj struct {
		ClusterName string `json:"clusterName"`
		Namespace   string `json:"namespace"`
		Name        string `json:"name"`
		Container   string `json:"container"`
		Path        string `json:"path"`
		Content     string `json:"content"`
	}

	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	if reqObj.ClusterName == "" || reqObj.Namespace == "" || reqObj.Name == "" || reqObj.Path == "" {
		common.FailWithMessage("clusterName、namespace、name 和 path 不能为空", c)
		return
	}

	cmdStr := fmt.Sprintf("cat > '%s'", reqObj.Path)

	reader := strings.NewReader(reqObj.Content)
	_, stderrBytes, err := execContainerShellCmd(c, reqObj.ClusterName, reqObj.Namespace, reqObj.Name, reqObj.Container, cmdStr, reader)
	if err != nil || len(stderrBytes) > 0 {
		errMsg := string(stderrBytes)
		if errMsg == "" && err != nil {
			errMsg = err.Error()
		}
		if isShellNotFoundError(err, stderrBytes) {
			common.FailWithMessage("容器未内置 Shell 环境(如 Distroless/Scratch 极简镜像)，无法保存文件", c)
			return
		}
		sc.Logger.Error("保存容器文件内容失败", zap.String("path", reqObj.Path), zap.Error(err))
		common.FailWithMessage("保存容器文件失败: "+errMsg, c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("文件 [%s] 内容修改保存成功", reqObj.Path), c)
}

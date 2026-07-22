package view_server

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type K8sDeletePodBatchReq struct {
	ClusterName string   `json:"clusterName"`
	Namespace   string   `json:"namespace"`
	Names       []string `json:"names"`
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
	delete(mapObj, "status") // 移除运行时状态冗余字段

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

	if reqObj.ClusterName == "" || len(reqObj.Names) == 0 {
		common.FailWithMessage("集群名称与要删除的 Pod 名称列表不能为空", c)
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

	for _, name := range reqObj.Names {
		ns := reqObj.Namespace
		if ns == "" {
			p, err := kSet.CoreV1().Pods("").Get(ctx, name, metav1.GetOptions{})
			if err == nil && p != nil {
				ns = p.Namespace
			}
		}
		if ns == "" {
			ns = "default"
		}

		err := kSet.CoreV1().Pods(ns).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			sc.Logger.Error("批量删除中单个 Pod 删除失败", zap.String("name", name), zap.Error(err))
			failNames = append(failNames, name)
		} else {
			successNames = append(successNames, name)
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

	req := kSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(name).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: container,
			Command:   []string{"/bin/sh", "-c", "TERM=xterm-256color exec /bin/sh || exec /bin/bash || exec sh"},
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

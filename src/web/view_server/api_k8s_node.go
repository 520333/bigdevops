package view_server

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsClientSet "k8s.io/metrics/pkg/client/clientset/versioned"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type K8sOneNode struct {
	// 分类list列表要的数据
	Name           string   `json:"name"`
	Status         string   `json:"status"`
	ScheduleEnable bool     `json:"scheduleEnable"`
	Roles          []string `json:"roles"`
	Age            string   `json:"age"`
	Ip             string   `json:"ip"`
	PodNum         int      `json:"podNum"`

	CpuRequestInfo string `json:"cpuRequestInfo"` //requestSum/total (requestRate% ) 比如：申请10核 总共50核心 10/50(20%)
	CpuLimitInfo   string `json:"cpuLimitInfo"`
	CpuUsageInfo   string `json:"cpuUsageInfo"`

	MemoryRequestInfo string `json:"memoryRequestInfo"` //requestSum/total (requestRate% ) 比如：申请10核 总共50核心 10/50(20%)
	MemoryLimitInfo   string `json:"memoryLimitInfo"`
	MemoryUsageInfo   string `json:"memoryUsageInfo"`

	PodNumInfo string `json:"podNumInfo"` // podNum/total (rate%)

	CpuCores         string `json:"cpuCores"`
	MemGibs          string `json:"memGibs"`
	EphemeralStorage string `json:"ephemeralStorage"`

	// 详细字段
	KubeletVersion string `json:"kubeletVersion"`
	CriVersion     string `json:"criVersion"`
	OsVersion      string `json:"osVersion"`

	KernelVersion string                 `json:"kernelVersion"`
	LabelPairs    map[string]string      `json:"labelPairs"`
	Labels        []string               `json:"labels"`
	Annotation    map[string]string      `json:"annotation"`
	Conditions    []corev1.NodeCondition `json:"conditions"`
	Taints        []corev1.Taint         `json:"taints"`
	Events        []OneEvent             `json:"events"`
	Pods          []OnePod               `json:"pods"`
}

type OneEvent struct {
	Type           string `json:"type"`
	Component      string `json:"component"`
	Count          int    `json:"count"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	Object         string `json:"object"`
	FirstTimestamp string `json:"firstTimestamp"`
	LastTimestamp  string `json:"lastTimestamp"`
}

// 封装 k8s node --> my node
func nodeConvert(knode corev1.Node, kCluster *models.K8sCluster, kSet *kubernetes.Clientset, mSet *metricsClientSet.Clientset) *K8sOneNode {
	res := &K8sOneNode{}
	res.Name = knode.Name

	// 1. 提取调度状态 (Spec.Unschedulable = true 代表禁止调度)
	res.ScheduleEnable = !knode.Spec.Unschedulable

	// 获取节点状态
	conditionMap := make(map[corev1.NodeConditionType]*corev1.NodeCondition)
	NodeAllConditions := []corev1.NodeConditionType{corev1.NodeReady}
	for i := range knode.Status.Conditions {
		cond := knode.Status.Conditions[i]
		conditionMap[cond.Type] = &cond
	}
	var status []string
	for _, validCondition := range NodeAllConditions {
		if condition, ok := conditionMap[validCondition]; ok {
			if condition.Status == corev1.ConditionTrue {
				status = append(status, string(condition.Type))
			} else {
				status = append(status, "Not"+string(condition.Type))
			}
		}
	}
	if len(status) == 0 {
		status = append(status, "Unknown")
	}
	if knode.Spec.Unschedulable {
		status = append(status, "SchedulingDisabled")
	}
	res.Status = strings.Join(status, ",")
	res.Roles = findNodeRoles(&knode)

	res.Age = translateTimestampSince(knode.CreationTimestamp)
	//提取 Node IP 地址
	res.Ip = getNodeInternalIp(&knode)

	res.KubeletVersion = knode.Status.NodeInfo.KubeletVersion
	res.CriVersion = knode.Status.NodeInfo.ContainerRuntimeVersion
	res.OsVersion = knode.Status.NodeInfo.OSImage
	res.KernelVersion = knode.Status.NodeInfo.KernelVersion

	allocCpuMilli := knode.Status.Allocatable.Cpu().MilliValue()
	allocMemBytes := knode.Status.Allocatable.Memory().Value()

	// 获取节点上 Pod 列表
	nodePods, err := getPodsOnNode(knode.Name, kCluster.ActionTimeoutSeconds, kSet)
	if err == nil && nodePods != nil {
		res.PodNum = len(nodePods)

		// 统计节点关联 Pod 的 Request 与 Limit 申请量汇总
		var cpuRequestTotal, cpuLimitTotal int64
		var memoryRequestTotal, memoryLimitTotal int64

		for _, pod := range nodePods {
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				continue
			}
			for _, container := range pod.Spec.Containers {
				cpuRequestTotal += container.Resources.Requests.Cpu().MilliValue()
				cpuLimitTotal += container.Resources.Limits.Cpu().MilliValue()
				memoryRequestTotal += container.Resources.Requests.Memory().Value()
				memoryLimitTotal += container.Resources.Limits.Memory().Value()
			}
		}

		// 计算 CPU 百分比
		var cpuReqRate, cpuLimitRate float64
		if allocCpuMilli > 0 {
			cpuReqRate = float64(cpuRequestTotal) / float64(allocCpuMilli) * 100
			cpuLimitRate = float64(cpuLimitTotal) / float64(allocCpuMilli) * 100
		}
		res.CpuRequestInfo = fmt.Sprintf("%.2f/%.2f核 (%.1f%%)", float64(cpuRequestTotal)/1000, float64(allocCpuMilli)/1000, cpuReqRate)
		res.CpuLimitInfo = fmt.Sprintf("%.2f/%.2f核 (%.1f%%)", float64(cpuLimitTotal)/1000, float64(allocCpuMilli)/1000, cpuLimitRate)

		// 计算 内存 百分比与单位转换 (GB)
		var memReqRate, memLimitRate float64
		if allocMemBytes > 0 {
			memReqRate = float64(memoryRequestTotal) / float64(allocMemBytes) * 100
			memLimitRate = float64(memoryLimitTotal) / float64(allocMemBytes) * 100
		}
		const gb = 1024 * 1024 * 1024
		res.MemoryRequestInfo = fmt.Sprintf("%.2f/%.2f GB (%.1f%%)", float64(memoryRequestTotal)/gb, float64(allocMemBytes)/gb, memReqRate)
		res.MemoryLimitInfo = fmt.Sprintf("%.2f/%.2f GB (%.1f%%)", float64(memoryLimitTotal)/gb, float64(allocMemBytes)/gb, memLimitRate)

		res.LabelPairs = knode.Labels            // 标签
		res.Annotation = knode.Annotations       // 注解
		res.Taints = knode.Spec.Taints           // 污点
		res.Conditions = knode.Status.Conditions // 状况

		res.CpuCores = fmt.Sprintf("%.2f/%.2f核", float64(knode.Status.Allocatable.Cpu().MilliValue())/1000, float64(knode.Status.Capacity.Cpu().MilliValue())/1000)
		res.MemGibs = fmt.Sprintf("%.2f/%.2f GB",
			float64(knode.Status.Allocatable.Memory().Value())/gb,
			float64(knode.Status.Capacity.Memory().Value())/gb,
		)
		res.EphemeralStorage = fmt.Sprintf("%.2f/%.2f GB",
			float64(knode.Status.Allocatable.StorageEphemeral().Value())/gb,
			float64(knode.Status.Capacity.StorageEphemeral().Value())/gb,
		)

		// 节点真实使用率计算 (来自 Metrics Server)
		if mSet != nil {
			nodeMetrics, err := getNodeUsageByMetricsServer(knode.Name, kCluster.ActionTimeoutSeconds, mSet)
			if err == nil && nodeMetrics != nil {
				cpuUsageMilli := nodeMetrics.Usage.Cpu().MilliValue()
				memUsageBytes := nodeMetrics.Usage.Memory().Value()

				var cpuUsageRate, memUsageRate float64
				if allocCpuMilli > 0 {
					cpuUsageRate = float64(cpuUsageMilli) / float64(allocCpuMilli) * 100
				}
				if allocMemBytes > 0 {
					memUsageRate = float64(memUsageBytes) / float64(allocMemBytes) * 100
				}

				res.CpuUsageInfo = fmt.Sprintf("%.2f/%.2f核 (%.1f%%)", float64(cpuUsageMilli)/1000, float64(allocCpuMilli)/1000, cpuUsageRate)
				res.MemoryUsageInfo = fmt.Sprintf("%.2f/%.2f GB (%.1f%%)", float64(memUsageBytes)/gb, float64(allocMemBytes)/gb, memUsageRate)
			} else {
				res.CpuUsageInfo = "未安装MetricsServer"
				res.MemoryUsageInfo = "未安装MetricsServer"
			}
		} else {
			res.CpuUsageInfo = "未初始化Metrics"
			res.MemoryUsageInfo = "未初始化Metrics"
		}

		res.Labels = common.GentStringArrayByMap(knode.Labels)

		// 节点 event 转化为精致的 DTO 结构
		events, _ := getEventOnNode(knode.Name, kCluster.ActionTimeoutSeconds, kSet)
		myEvents := make([]OneEvent, 0, len(events))
		for _, event := range events {
			var firstTimeStr, lastTimeStr string
			if !event.FirstTimestamp.IsZero() {
				firstTimeStr = event.FirstTimestamp.Time.Format("2006-01-02 15:04:05")
			}
			if !event.LastTimestamp.IsZero() {
				lastTimeStr = event.LastTimestamp.Time.Format("2006-01-02 15:04:05")
			}

			objStr := event.InvolvedObject.Name
			if event.InvolvedObject.Kind != "" {
				objStr = fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name)
			}

			myEvents = append(myEvents, OneEvent{
				Type:           event.Type,
				Component:      event.Source.Component,
				Count:          int(event.Count),
				Reason:         event.Reason,
				Message:        event.Message,
				Object:         objStr,
				FirstTimestamp: firstTimeStr,
				LastTimestamp:  lastTimeStr,
			})
		}
		res.Events = myEvents
	}
	return res
}

func findNodeRoles(node *corev1.Node) []string {
	roles := sets.New[string]()
	for k, v := range node.Labels {
		switch {
		case strings.HasPrefix(k, common.LabelNodeRolePrefix):
			if role := strings.TrimPrefix(k, common.LabelNodeRolePrefix); len(role) > 0 {
				roles.Insert(role)
			}
		case k == common.NodeLabelRole && v != "":
			roles.Insert(v)
		}
	}
	return roles.UnsortedList()
}

func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}
	return duration.HumanDuration(time.Since(timestamp.Time))
}

func getNodeInternalIp(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return "<none>"
}

func getPodsOnNode(nodeName string, tw int, kSet *kubernetes.Clientset) ([]corev1.Pod, error) {
	if kSet == nil {
		return nil, fmt.Errorf("kSet is nil")
	}
	ctx1, cancel1 := common.GenTimeoutContext(tw)
	defer cancel1()

	// 🚀 修复 1：使用 Client-go 官方推荐的类型安全构建器生成 FieldSelector
	// 避免 fields.ParseSelector("fieldSelector=spec...") 解析语法错误返回 nil 导致 nil 指针 panic
	selector := fields.OneTermEqualSelector("spec.nodeName", nodeName).String()

	pods, err := kSet.CoreV1().Pods("").List(ctx1, metav1.ListOptions{
		FieldSelector: selector,
	})
	// 🚀 修复 2：安全判断 err，避免在请求失败 pods 为 nil 时访问 pods.Items 导致二次 panic
	if err != nil || pods == nil {
		return nil, err
	}
	return pods.Items, nil
}

func getEventOnNode(nodeName string, tw int, kSet *kubernetes.Clientset) ([]corev1.Event, error) {
	ctx1, cancel1 := common.GenTimeoutContext(tw)
	defer cancel1()

	selector := fields.Set{
		"involvedObject.kind": "Node",
		"involvedObject.name": nodeName,
	}.AsSelector().String()

	events, err := kSet.CoreV1().Events("").List(ctx1, metav1.ListOptions{FieldSelector: selector})
	if err != nil || events == nil {
		return nil, err
	}
	return events.Items, nil
}
func getNodeUsageByMetricsServer(nodeName string, tw int, mSet *metricsClientSet.Clientset) (*metricsv1beta1.NodeMetrics, error) {
	if mSet == nil {
		return nil, fmt.Errorf("metricsClientSet is nil")
	}
	ctx1, cancel1 := common.GenTimeoutContext(tw)
	defer cancel1()

	nodeMetrics, err := mSet.MetricsV1beta1().NodeMetricses().Get(ctx1, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return nodeMetrics, nil
}

// @Summary      获取K8s集群Node节点列表
// @Description  根据集群名称分页获取该集群下的 Node 节点列表
// @Tags         k8s集群管理模块
// @Accept       json
// @Produce      json
// @Param        cluster   query     string  true   "K8s集群名称"
// @Param        name      query     string  false  "Node节点名称搜索"
// @Param        page      query     int     false  "页码" default(1)
// @Param        pageSize  query     int     false  "每页数量" default(10)
// @Success      200       {object}  map[string]interface{} "成功响应"
// @Failure      400       {object}  map[string]interface{} "请求参数错误"
// @Failure      500       {object}  map[string]interface{} "服务器内部错误"
// @Security     Bearer
// @Router       /k8s/getK8sNodeList [get]
func getK8sNodeList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchName := c.DefaultQuery("name", "")
	searchClusterName := c.DefaultQuery("cluster", "")

	// 1. 校验必须参数 cluster
	if searchClusterName == "" {
		msg := "集群名称未传入"
		sc.Logger.Error(msg)
		common.ReqBadFailWithMessage(msg, c)
		return
	}

	// 2. 查询集群信息
	dbObj, err := models.GetK8sClusterByName(searchClusterName)
	if err != nil {
		sc.Logger.Error("根据名称找k8s集群错误", zap.Error(err))
		common.FailWithMessage(fmt.Sprintf("根据名称查找集群失败: %v", err.Error()), c)
		return
	}

	// 3. 获取集群 ClientSet 及 MetricsClientSet
	kc := c.MustGet(common.GIN_CTX_K8S_CACHE).(*cache.K8sClusterCache)
	kSet := kc.GetClusterClientSetById(dbObj.ID)
	if kSet == nil {
		sc.Logger.Error("根据id获取k8s集群ClientSet失败", zap.Uint("cluster_id", dbObj.ID))
		common.FailWithMessage("获取集群客户端句柄失败，请检查集群连接状态", c)
		return
	}
	mSet := kc.GetClusterMetricsSetById(dbObj.ID)

	ctx1, cancel1 := common.GenTimeoutContext(dbObj.ActionTimeoutSeconds)
	defer cancel1()
	nodes, err := kSet.CoreV1().Nodes().List(ctx1, metav1.ListOptions{})
	if err != nil {
		sc.Logger.Error("根据k8s集群的kset获取集群错误", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 5. 内存按名称过滤
	filteredNodes := make([]corev1.Node, 0)
	for _, node := range nodes.Items {
		if searchName != "" && !strings.Contains(node.Name, searchName) {
			continue
		}
		filteredNodes = append(filteredNodes, node)
	}

	total := len(filteredNodes)

	// 6. 进行内存分页切片计算
	offset := (currentPage - 1) * pageSize
	limit := offset + pageSize
	pagedNodes := make([]corev1.Node, 0)
	if offset < total {
		if limit > total {
			limit = total
		}
		pagedNodes = filteredNodes[offset:limit]
	}

	// 7. 🚀 使用 Goroutine 并发多线程转换分页后的节点，避免串行网络等待拖慢接口响应
	mNodes := make([]K8sOneNode, len(pagedNodes))
	var wg sync.WaitGroup
	for i, node := range pagedNodes {
		wg.Add(1)
		go func(idx int, n corev1.Node) {
			defer wg.Done()
			mNode := nodeConvert(n, dbObj, kSet, mSet)
			if mNode != nil {
				mNodes[idx] = *mNode
			}
		}(i, node)
	}
	wg.Wait()

	// 8. 返回结果
	common.OkWithDetailed(gin.H{
		"items": mNodes,
		"total": total,
	}, "ok", c)
}

type K8sClusterNodeRequest struct {
	ClusterName  string   `json:"clusterName" validate:"required"`
	NodeNames    []string `json:"nodeNames" validate:"required"`
	TargetEnable *bool    `json:"targetEnable"` // 可选: true 允许调度, false 停止调度; 不传则自动翻转
}
type LabelK8sNodeRequest struct {
	K8sClusterNodeRequest
	Labels []string `json:"labels"` // k=v
}
type TaintK8sNodeRequest struct {
	ClusterName string         `json:"clusterName" validate:"required"`
	NodeNames   []string       `json:"nodeNames" validate:"required"`
	Taints      []corev1.Taint `json:"taints"`
	DeletedKeys []string       `json:"deletedKeys"` // 仅在批量模式下生效: 显式删除的污点 Key 列表
}

// @Summary      切换K8s集群节点调度状态
// @Description  切换指定K8s集群中某个/多个 Node 节点的 Cordon (禁止调度) / Uncordon (允许调度) 状态
// @Tags         k8s集群管理模块
// @Accept       json
// @Produce      json
// @Param        body  body      K8sClusterNodeRequest  true  "请求体"
// @Success      200   {object}  map[string]interface{} "成功响应"
// @Failure      400   {object}  map[string]interface{} "请求参数错误"
// @Failure      500   {object}  map[string]interface{} "服务器内部错误"
// @Security     Bearer
// @Router       /k8s/scheduleEnableSwitchK8sNodesOne [post]
func scheduleEnableSwitchK8sNodesOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj K8sClusterNodeRequest
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析k8s集群节点调度状态变化请求失败", zap.Any("req", reqObj), zap.Error(err))
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	// 结构体数据校验
	err = validate.Struct(&reqObj)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求参数校验失败", c)
			return
		}
	}

	// 1. 查询数据库中集群配置
	dbObj, err := models.GetK8sClusterByName(reqObj.ClusterName)
	if err != nil {
		sc.Logger.Error("根据名称找k8s集群失败", zap.String("cluster", reqObj.ClusterName), zap.Error(err))
		common.FailWithMessage(fmt.Sprintf("根据名称查找集群失败: %v", err.Error()), c)
		return
	}

	kc := c.MustGet(common.GIN_CTX_K8S_CACHE).(*cache.K8sClusterCache)
	kSet := kc.GetClusterClientSetById(dbObj.ID)
	if kSet == nil {
		sc.Logger.Error("根据id获取k8s集群ClientSet失败", zap.Uint("cluster_id", dbObj.ID))
		common.FailWithMessage("获取集群客户端句柄失败，请检查集群连接状态", c)
		return
	}

	// 2. 遍历批量更新 Node 调度状态
	for _, nodeName := range reqObj.NodeNames {
		updateErr := func() error {
			ctx1, cancel1 := common.GenTimeoutContext(dbObj.ActionTimeoutSeconds)
			defer cancel1()

			kNode, err := kSet.CoreV1().Nodes().Get(ctx1, nodeName, metav1.GetOptions{})
			if err != nil {
				sc.Logger.Error("获取Node节点配置失败", zap.String("node", nodeName), zap.Error(err))
				return fmt.Errorf("获取节点 %s 失败: %v", nodeName, err)
			}

			// 决定新的 Unschedulable 状态
			if reqObj.TargetEnable != nil {
				// targetEnable 为 true 代表允许调度 (Unschedulable = false)
				kNode.Spec.Unschedulable = !*reqObj.TargetEnable
			} else {
				// 未显式传参则反转状态
				kNode.Spec.Unschedulable = !kNode.Spec.Unschedulable
			}

			_, err = kSet.CoreV1().Nodes().Update(ctx1, kNode, metav1.UpdateOptions{})
			if err != nil {
				sc.Logger.Error("更新Node节点调度状态失败", zap.String("cluster", reqObj.ClusterName), zap.String("node", nodeName), zap.Error(err))
				return fmt.Errorf("更新节点 %s 调度状态失败: %v", nodeName, err)
			}
			return nil
		}()

		if updateErr != nil {
			common.FailWithMessage(updateErr.Error(), c)
			return
		}
	}

	common.OkWithMessage("节点调度状态更新成功", c)
}

// @Summary      修改K8s集群节点标签
// @Description  给指定K8s集群中的某个/多个 Node 节点批量设置/更新 Label 标签
// @Tags         k8s集群管理模块
// @Accept       json
// @Produce      json
// @Param        body  body      LabelK8sNodeRequest    true  "请求体"
// @Success      200   {object}  map[string]interface{} "成功响应"
// @Failure      400   {object}  map[string]interface{} "请求参数错误"
// @Failure      500   {object}  map[string]interface{} "服务器内部错误"
// @Security     Bearer
// @Router       /k8s/labelK8sNodes [post]
func labelK8sNodes(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj LabelK8sNodeRequest
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析k8s集群标签配置变化请求失败", zap.Any("req", reqObj), zap.Error(err))
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	// 结构体数据校验
	err = validate.Struct(&reqObj)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求参数校验失败", c)
			return
		}
	}

	// 1. 查询数据库中集群配置
	dbObj, err := models.GetK8sClusterByName(reqObj.ClusterName)
	if err != nil {
		sc.Logger.Error("根据名称找k8s集群失败", zap.String("cluster", reqObj.ClusterName), zap.Error(err))
		common.FailWithMessage(fmt.Sprintf("根据名称查找集群失败: %v", err.Error()), c)
		return
	}

	kc := c.MustGet(common.GIN_CTX_K8S_CACHE).(*cache.K8sClusterCache)
	kSet := kc.GetClusterClientSetById(dbObj.ID)
	if kSet == nil {
		sc.Logger.Error("根据id获取k8s集群ClientSet失败", zap.Uint("cluster_id", dbObj.ID))
		common.FailWithMessage("获取集群客户端句柄失败，请检查集群连接状态", c)
		return
	}

	// 2. 遍历批量更新 Node 节点标签
	for _, nodeName := range reqObj.NodeNames {
		updateErr := func() error {
			ctx1, cancel1 := common.GenTimeoutContext(dbObj.ActionTimeoutSeconds)
			defer cancel1()

			kNode, err := kSet.CoreV1().Nodes().Get(ctx1, nodeName, metav1.GetOptions{})
			if err != nil {
				sc.Logger.Error("获取Node节点配置失败", zap.String("node", nodeName), zap.Error(err))
				return fmt.Errorf("获取节点 %s 失败: %v", nodeName, err)
			}

			if kNode.Labels == nil {
				kNode.Labels = make(map[string]string)
			}

			lm := common.GenMapByKvString(reqObj.Labels)

			// 🚀 校验 Label 键与值的合法性 (规范对齐 K8s 官方标准)
			for k, v := range lm {
				if errs := validation.IsQualifiedName(k); len(errs) > 0 {
					return fmt.Errorf("标签 Key [%s] 格式不符合 K8s 规范: %s", k, strings.Join(errs, "; "))
				}
				if v != "" {
					if errs := validation.IsValidLabelValue(v); len(errs) > 0 {
						return fmt.Errorf("标签 [%s] 的 Value [%s] 格式不符合 K8s 规范: %s", k, v, strings.Join(errs, "; "))
					}
				}
			}

			// 🚀 单节点修改模式：以前端提交的为准，把原节点中存在、但本次提交中没有的 key 从 kNode.Labels 中删掉
			if len(reqObj.NodeNames) == 1 {
				for oldKey := range kNode.Labels {
					if _, exists := lm[oldKey]; !exists {
						delete(kNode.Labels, oldKey)
					}
				}
			}

			// 覆盖/新增/显式删除(v=="")
			for k, v := range lm {
				if v == "" {
					delete(kNode.Labels, k)
				} else {
					kNode.Labels[k] = v
				}
			}

			_, err = kSet.CoreV1().Nodes().Update(ctx1, kNode, metav1.UpdateOptions{})
			if err != nil {
				sc.Logger.Error("更新Node节点标签错误", zap.String("cluster", reqObj.ClusterName), zap.String("node", nodeName), zap.Error(err))
				return fmt.Errorf("更新节点 %s 标签失败: %v", nodeName, err)
			}
			return nil
		}()

		if updateErr != nil {
			common.FailWithMessage(updateErr.Error(), c)
			return
		}
	}

	common.OkWithMessage("节点标签更新成功", c)
}

// @Summary      修改K8s集群节点污点
// @Description  给指定K8s集群中的某个/多个 Node 节点设置/更新 Taint 污点配置
// @Tags         k8s集群管理模块
// @Accept       json
// @Produce      json
// @Param        body  body      TaintK8sNodeRequest    true  "请求体"
// @Success      200   {object}  map[string]interface{} "成功响应"
// @Failure      400   {object}  map[string]interface{} "请求参数错误"
// @Failure      500   {object}  map[string]interface{} "服务器内部错误"
// @Security     Bearer
// @Router       /k8s/taintK8sNodes [post]
func taintK8sNodes(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj TaintK8sNodeRequest
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析k8s集群污点配置变化请求失败", zap.Any("req", reqObj), zap.Error(err))
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	// 结构体数据校验
	err = validate.Struct(&reqObj)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求参数校验失败", c)
			return
		}
	}

	// 1. 查询数据库中集群配置
	dbObj, err := models.GetK8sClusterByName(reqObj.ClusterName)
	if err != nil {
		sc.Logger.Error("根据名称找k8s集群失败", zap.String("cluster", reqObj.ClusterName), zap.Error(err))
		common.FailWithMessage(fmt.Sprintf("根据名称查找集群失败: %v", err.Error()), c)
		return
	}

	kc := c.MustGet(common.GIN_CTX_K8S_CACHE).(*cache.K8sClusterCache)
	kSet := kc.GetClusterClientSetById(dbObj.ID)
	if kSet == nil {
		sc.Logger.Error("根据id获取k8s集群ClientSet失败", zap.Uint("cluster_id", dbObj.ID))
		common.FailWithMessage("获取集群客户端句柄失败，请检查集群连接状态", c)
		return
	}

	// 2. 遍历更新 Node 节点污点
	for _, nodeName := range reqObj.NodeNames {
		updateErr := func() error {
			ctx1, cancel1 := common.GenTimeoutContext(dbObj.ActionTimeoutSeconds)
			defer cancel1()

			kNode, err := kSet.CoreV1().Nodes().Get(ctx1, nodeName, metav1.GetOptions{})
			if err != nil {
				sc.Logger.Error("获取Node节点配置失败", zap.String("node", nodeName), zap.Error(err))
				return fmt.Errorf("获取节点 %s 失败: %v", nodeName, err)
			}

			// 🚀 校验 Taint 键、值与 Effect 的合法性 (规范对齐 K8s 官方标准)
			for _, taint := range reqObj.Taints {
				if errs := validation.IsQualifiedName(taint.Key); len(errs) > 0 {
					return fmt.Errorf("污点 Key [%s] 格式不符合 K8s 规范: %s", taint.Key, strings.Join(errs, "; "))
				}
				if taint.Value != "" {
					if errs := validation.IsValidLabelValue(taint.Value); len(errs) > 0 {
						return fmt.Errorf("污点 [%s] 的 Value [%s] 格式不符合 K8s 规范: %s", taint.Key, taint.Value, strings.Join(errs, "; "))
					}
				}
				switch taint.Effect {
				case corev1.TaintEffectNoSchedule, corev1.TaintEffectPreferNoSchedule, corev1.TaintEffectNoExecute:
				default:
					return fmt.Errorf("污点 [%s] 的 Effect [%s] 无效，必须为 NoSchedule, PreferNoSchedule 或 NoExecute", taint.Key, taint.Effect)
				}
			}

			if len(reqObj.NodeNames) == 1 {
				// 单节点模式：直接同步覆盖当前节点的全量污点
				kNode.Spec.Taints = reqObj.Taints
			} else {
				// 批量节点模式：保留各节点独有的原生污点，仅对公共/提交的污点进行增量添加、修改或显式删除
				newTaintMap := make(map[string]corev1.Taint)
				for _, t := range reqObj.Taints {
					newTaintMap[t.Key] = t
				}

				deletedKeyMap := make(map[string]bool)
				for _, dk := range reqObj.DeletedKeys {
					deletedKeyMap[dk] = true
				}

				var updatedTaints []corev1.Taint
				// 1. 保留未被显式删除且不在提交列表里的节点原生独有污点
				for _, oldTaint := range kNode.Spec.Taints {
					if deletedKeyMap[oldTaint.Key] {
						continue // 被显式删除的污点从节点中剔除
					}
					if _, exists := newTaintMap[oldTaint.Key]; !exists {
						updatedTaints = append(updatedTaints, oldTaint) // 保留节点原生独有污点
					}
				}

				// 2. 追加或更新提交的批量污点
				for _, newTaint := range reqObj.Taints {
					updatedTaints = append(updatedTaints, newTaint)
				}

				kNode.Spec.Taints = updatedTaints
			}

			_, err = kSet.CoreV1().Nodes().Update(ctx1, kNode, metav1.UpdateOptions{})
			if err != nil {
				sc.Logger.Error("更新Node节点污点错误", zap.String("cluster", reqObj.ClusterName), zap.String("node", nodeName), zap.Error(err))
				return fmt.Errorf("更新节点 %s 污点失败: %v", nodeName, err)
			}
			return nil
		}()

		if updateErr != nil {
			common.FailWithMessage(updateErr.Error(), c)
			return
		}
	}

	common.OkWithMessage("节点污点更新成功", c)
}

// @Summary      驱逐K8s集群节点 Pod (Drain Node)
// @Description  将指定K8s集群的一个/多个 Node 节点设为禁止调度(Cordon)，并主动驱逐(Evict)节点上运行的所有非 DaemonSet 应用 Pod
// @Tags         k8s集群管理模块
// @Accept       json
// @Produce      json
// @Param        body  body      K8sClusterNodeRequest  true  "请求体"
// @Success      200   {object}  map[string]interface{} "成功响应"
// @Failure      400   {object}  map[string]interface{} "请求参数错误"
// @Failure      500   {object}  map[string]interface{} "服务器内部错误"
// @Security     Bearer
// @Router       /k8s/drainK8sNodes [post]
func drainK8sNodes(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj K8sClusterNodeRequest
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析k8s集群节点驱逐请求失败", zap.Any("req", reqObj), zap.Error(err))
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	// 结构体数据校验
	err = validate.Struct(&reqObj)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求参数校验失败", c)
			return
		}
	}

	// 1. 查询数据库中集群配置
	dbObj, err := models.GetK8sClusterByName(reqObj.ClusterName)
	if err != nil {
		sc.Logger.Error("根据名称找k8s集群失败", zap.String("cluster", reqObj.ClusterName), zap.Error(err))
		common.FailWithMessage(fmt.Sprintf("根据名称查找集群失败: %v", err.Error()), c)
		return
	}

	kc := c.MustGet(common.GIN_CTX_K8S_CACHE).(*cache.K8sClusterCache)
	kSet := kc.GetClusterClientSetById(dbObj.ID)
	if kSet == nil {
		sc.Logger.Error("根据id获取k8s集群ClientSet失败", zap.Uint("cluster_id", dbObj.ID))
		common.FailWithMessage("获取集群客户端句柄失败，请检查集群连接状态", c)
		return
	}

	// 2. 遍历执行 Drain 驱逐逻辑
	evictedCountTotal := 0
	for _, nodeName := range reqObj.NodeNames {
		drainErr := func() error {
			ctx1, cancel1 := common.GenTimeoutContext(dbObj.ActionTimeoutSeconds)
			defer cancel1()

			// 步骤 A: 设置 Node 为 Cordon (Unschedulable = true) 阻止新 Pod 调度进来
			kNode, err := kSet.CoreV1().Nodes().Get(ctx1, nodeName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("获取节点 %s 失败: %v", nodeName, err)
			}

			if !kNode.Spec.Unschedulable {
				kNode.Spec.Unschedulable = true
				_, err = kSet.CoreV1().Nodes().Update(ctx1, kNode, metav1.UpdateOptions{})
				if err != nil {
					return fmt.Errorf("设置节点 %s 为禁止调度失败: %v", nodeName, err)
				}
			}

			// 步骤 B: 查询该 Node 上运行的所有 Pod
			fieldSelector := fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
			pods, err := kSet.CoreV1().Pods("").List(ctx1, metav1.ListOptions{FieldSelector: fieldSelector})
			if err != nil {
				return fmt.Errorf("获取节点 %s 上的 Pod 列表失败: %v", nodeName, err)
			}

			// 步骤 C: 筛选并驱逐用户/应用 Pod (跳过 DaemonSet 和 静态 Mirror Pod)
			for _, pod := range pods.Items {
				// 跳过 静态 Pod (Mirror Pod)
				if _, isMirror := pod.Annotations["kubernetes.io/config.mirror"]; isMirror {
					continue
				}

				// 跳过 DaemonSet 控制的 Pod
				isDaemonSet := false
				for _, owner := range pod.OwnerReferences {
					if owner.Kind == "DaemonSet" {
						isDaemonSet = true
						break
					}
				}
				if isDaemonSet {
					continue
				}

				// 驱逐 (Evict) Pod
				eviction := &policyv1.Eviction{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pod.Name,
						Namespace: pod.Namespace,
					},
				}
				err = kSet.CoreV1().Pods(pod.Namespace).EvictV1(ctx1, eviction)
				if err != nil {
					// 若 Evict 接口兼容性问题报错，兜底重试 Delete 强行移除
					_ = kSet.CoreV1().Pods(pod.Namespace).Delete(ctx1, pod.Name, metav1.DeleteOptions{})
				}
				evictedCountTotal++
			}

			return nil
		}()

		if drainErr != nil {
			common.FailWithMessage(drainErr.Error(), c)
			return
		}
	}

	common.OkWithMessage(fmt.Sprintf("节点驱逐成功，共触发驱逐 %d 个应用 Pod", evictedCountTotal), c)
}

// @Summary      获取指定Node节点上的Pod列表
// @Description  根据集群名称与Node节点名称获取该节点上运行的所有 Pod 列表
// @Tags         k8s集群管理模块
// @Accept       json
// @Produce      json
// @Param        cluster  query     string  true  "K8s集群名称"
// @Param        node     query     string  true  "Node节点名称"
// @Success      200      {object}  map[string]interface{} "成功响应"
// @Failure      400      {object}  map[string]interface{} "请求参数错误"
// @Failure      500      {object}  map[string]interface{} "服务器内部错误"
// @Security     Bearer
// @Router       /k8s/getPodListByNodeName [get]
func getPodListByNodeName(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	searchClusterName := c.Query("cluster")
	nodeName := c.Query("node")

	if searchClusterName == "" || nodeName == "" {
		common.ReqBadFailWithMessage("集群名称 (cluster) 和节点名称 (node) 不能为空", c)
		return
	}

	dbObj, err := models.GetK8sClusterByName(searchClusterName)
	if err != nil {
		sc.Logger.Error("根据名称找k8s集群错误", zap.String("cluster", searchClusterName), zap.Error(err))
		common.FailWithMessage(fmt.Sprintf("根据名称查找集群失败: %v", err.Error()), c)
		return
	}

	kc := c.MustGet(common.GIN_CTX_K8S_CACHE).(*cache.K8sClusterCache)
	kSet := kc.GetClusterClientSetById(dbObj.ID)
	if kSet == nil {
		sc.Logger.Error("根据id获取k8s集群ClientSet失败", zap.Uint("cluster_id", dbObj.ID))
		common.FailWithMessage("获取集群客户端句柄失败，请检查集群连接状态", c)
		return
	}

	pods, err := getPodsOnNode(nodeName, dbObj.ActionTimeoutSeconds, kSet)
	if err != nil {
		sc.Logger.Error("获取节点Pod列表失败", zap.String("cluster", searchClusterName), zap.String("node", nodeName), zap.Error(err))
		common.FailWithMessage(fmt.Sprintf("获取节点 %s 上的 Pod 列表失败: %v", nodeName, err), c)
		return
	}

	resList := make([]*OnePod, 0, len(pods))
	for i := range pods {
		if podObj := podConvert(&pods[i]); podObj != nil {
			resList = append(resList, podObj)
		}
	}

	common.OkWithData(map[string]interface{}{
		"items": resList,
		"total": len(resList),
	}, c)
}

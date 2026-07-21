package view_server

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type K8sOneNode struct {
	// 分类list列表要的数据
	Name           string   `form:"name"`
	Status         string   `form:"status"`
	ScheduleStatus bool     `form:"scheduleStatus"`
	Roles          []string `form:"roles"`
	Age            string   `form:"age"`
	Ip             string   `form:"ip"`
	PodNum         int      `form:"podNum"`

	CpuRequestInfo string `form:"cpuRequestInfo"` //requestSum/total (requestRate% ) 比如：申请10核 总共50核心 10/50(20%)
	CpuUsageInfo   string `form:"cpuUsageInfo"`

	MemoryRequestInfo string `form:"memoryRequestInfo"` //requestSum/total (requestRate% ) 比如：申请10核 总共50核心 10/50(20%)
	MemoryUsageInfo   string `form:"memoryRequestInfo"`

	PodNumInfo string `form:"podNumInfo"` // podNum/total (rate%)

	CpuCores int `form:"cpuCores"`
	MemGibs  int `form:"memGibs"`

	// 详细字段
	KubeletVersion   string               `form:"kubeletVersion"`
	CriVersion       string               `form:"criVersion"`
	OsVersion        string               `form:"osVersion"`
	KernelVersion    string               `form:"kernelVersion"`
	LabelPairs       map[string]string    `form:"labelPairs"`
	Annotation       map[string]string    `form:"annotation"`
	Conditions       corev1.NodeCondition `form:"conditions"`
	Taints           []corev1.Taint       `form:"taints"`
	Events           []corev1.Event       `form:"events"`
	EphemeralStorage int64                `form:"ephemeralStorage"`
}

// 封装 k8s node --> my node
func nodeConvert(knode corev1.Node) *K8sOneNode {
	res := &K8sOneNode{}
	res.Name = knode.Name
	//res.Status = knode
	return res

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

	// 3. 获取集群 ClientSet (修复: 补充了强转断言的右括号)
	kc := c.MustGet(common.GIN_CTX_K8S_CACHE).(*cache.K8sClusterCache)
	kset := kc.GetClusterClientSetById(dbObj.ID)
	if kset == nil {
		sc.Logger.Error("根据id获取k8s集群ClientSet失败", zap.Uint("cluster_id", dbObj.ID))
		common.FailWithMessage("获取集群客户端句柄失败，请检查集群连接状态", c)
		return
	}
	ctx1, cancel1 := common.GenTimeoutContext(dbObj.ActionTimeoutSeconds)
	defer cancel1()

	nodes, err := kset.CoreV1().Nodes().List(ctx1, metav1.ListOptions{})
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
	// 7. 返回结果
	common.OkWithDetailed(gin.H{
		"items": pagedNodes,
		"total": total,
	}, "ok", c)
}

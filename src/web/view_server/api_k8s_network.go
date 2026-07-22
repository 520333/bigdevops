package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	yaml3 "gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ==================== Service ====================

type OneService struct {
	Name        string   `json:"name"`
	Namespace   string   `json:"namespace"`
	Type        string   `json:"type"`
	ClusterIP   string   `json:"clusterIP"`
	Ports       []string `json:"ports"`
	ExternalIPs []string `json:"externalIPs"`
	Age         string   `json:"age"`
	CreatedAt   string   `json:"createdAt"`
}

type K8sNetworkItem struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type K8sCreateServiceReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	YamlContent string `json:"yamlContent"`
}

type K8sDeleteServiceReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

type K8sDeleteServiceBatchReq struct {
	ClusterName string           `json:"clusterName"`
	Namespace   string           `json:"namespace"`
	Names       []string         `json:"names"`
	Items       []K8sNetworkItem `json:"items"`
}

func serviceConvert(svc *v1.Service) *OneService {
	if svc == nil {
		return nil
	}
	var ports []string
	for _, p := range svc.Spec.Ports {
		pStr := fmt.Sprintf("%d/%s", p.Port, p.Protocol)
		if p.NodePort > 0 {
			pStr = fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol)
		}
		ports = append(ports, pStr)
	}

	var extIPs []string
	extIPs = append(extIPs, svc.Spec.ExternalIPs...)
	for _, lb := range svc.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			extIPs = append(extIPs, lb.IP)
		} else if lb.Hostname != "" {
			extIPs = append(extIPs, lb.Hostname)
		}
	}

	ageStr := "-"
	createdAtStr := "-"
	if !svc.CreationTimestamp.IsZero() {
		createdAtStr = svc.CreationTimestamp.Format("2006-01-02 15:04:05")
		diff := time.Since(svc.CreationTimestamp.Time)
		if diff.Hours() >= 24 {
			ageStr = fmt.Sprintf("%d 天", int(diff.Hours()/24))
		} else if diff.Hours() >= 1 {
			ageStr = fmt.Sprintf("%d 小时", int(diff.Hours()))
		} else {
			ageStr = fmt.Sprintf("%d 分钟", int(diff.Minutes()))
		}
	}

	return &OneService{
		Name:        svc.Name,
		Namespace:   svc.Namespace,
		Type:        string(svc.Spec.Type),
		ClusterIP:   svc.Spec.ClusterIP,
		Ports:       ports,
		ExternalIPs: extIPs,
		Age:         ageStr,
		CreatedAt:   createdAtStr,
	}
}

func getK8sServiceList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")
	keyword := c.Query("keyword")

	if clusterName == "" {
		common.FailWithMessage("clusterName 参数不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	svcList, err := kSet.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		sc.Logger.Error("获取 Service 列表失败", zap.Error(err))
		common.FailWithMessage("获取 Service 列表失败: "+err.Error(), c)
		return
	}

	var items []*OneService
	for i := range svcList.Items {
		svc := &svcList.Items[i]
		if keyword != "" && !strings.Contains(svc.Name, keyword) {
			continue
		}
		itemObj := serviceConvert(svc)
		if itemObj != nil {
			items = append(items, itemObj)
		}
	}

	common.OkWithDetailed(gin.H{
		"items": items,
		"total": len(items),
	}, "ok", c)
}

func getK8sServiceYaml(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")
	name := c.Query("name")

	if clusterName == "" || namespace == "" || name == "" {
		common.FailWithMessage("clusterName、namespace 和 name 不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	svc, err := kSet.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 Service 详情失败", zap.Error(err))
		common.FailWithMessage("获取 Service 详情失败: "+err.Error(), c)
		return
	}

	svcBytes, err := json.Marshal(svc)
	if err != nil {
		common.FailWithMessage("序列化 Service 失败: "+err.Error(), c)
		return
	}

	var mapObj map[string]interface{}
	_ = json.Unmarshal(svcBytes, &mapObj)

	if mapObj["apiVersion"] == nil || mapObj["apiVersion"] == "" {
		mapObj["apiVersion"] = "v1"
	}
	if mapObj["kind"] == nil || mapObj["kind"] == "" {
		mapObj["kind"] = "Service"
	}

	delete(mapObj, "status")
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

func createServiceHelper(c *gin.Context, isUpdate bool) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sCreateServiceReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析请求参数失败: "+err.Error(), c)
		return
	}

	if reqObj.ClusterName == "" || reqObj.YamlContent == "" {
		common.FailWithMessage("clusterName 和 yamlContent 不能为空", c)
		return
	}

	_, dSet, dbCluster, err := getClusterClientsetHelper(c, reqObj.ClusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	dec := yaml3.NewDecoder(strings.NewReader(reqObj.YamlContent))
	var rawObj map[string]interface{}
	if err := dec.Decode(&rawObj); err != nil {
		common.FailWithMessage("解析 YAML 内容失败: "+err.Error(), c)
		return
	}

	unstructObj := &unstructured.Unstructured{Object: rawObj}
	gvk := unstructObj.GroupVersionKind()
	if gvk.Kind == "" {
		unstructObj.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"})
		gvk = unstructObj.GroupVersionKind()
	}

	targetNs := unstructObj.GetNamespace()
	if targetNs == "" {
		targetNs = reqObj.Namespace
	}
	if targetNs == "" {
		targetNs = "default"
	}
	unstructObj.SetNamespace(targetNs)

	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: "services",
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	_, err = dSet.Resource(gvr).Namespace(targetNs).Apply(ctx, unstructObj.GetName(), unstructObj, metav1.ApplyOptions{
		FieldManager: "bigdevops-admin",
		Force:        true,
	})
	if err != nil {
		sc.Logger.Error("Apply Service 失败", zap.Error(err))
		common.FailWithMessage("保存 Service 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("Service 保存应用成功", c)
}

func createK8sService(c *gin.Context) { createServiceHelper(c, false) }
func updateK8sService(c *gin.Context) { createServiceHelper(c, true) }

func deleteK8sService(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sDeleteServiceReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, reqObj.ClusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	err = kSet.CoreV1().Services(reqObj.Namespace).Delete(ctx, reqObj.Name, metav1.DeleteOptions{})
	if err != nil {
		sc.Logger.Error("删除 Service 失败", zap.Error(err))
		common.FailWithMessage("删除 Service 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("Service [%s] 已删除", reqObj.Name), c)
}

func deleteK8sServiceBatch(c *gin.Context) {
	var reqObj K8sDeleteServiceBatchReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	var targets []K8sNetworkItem
	if len(reqObj.Items) > 0 {
		targets = reqObj.Items
	} else if reqObj.Namespace != "" && len(reqObj.Names) > 0 {
		for _, name := range reqObj.Names {
			targets = append(targets, K8sNetworkItem{Namespace: reqObj.Namespace, Name: name})
		}
	}

	if len(targets) == 0 {
		common.FailWithMessage("待删除列表不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, reqObj.ClusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	var errs []string
	for _, item := range targets {
		if item.Namespace == "" || item.Name == "" {
			continue
		}
		err := kSet.CoreV1().Services(item.Namespace).Delete(ctx, item.Name, metav1.DeleteOptions{})
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s/%s: %s", item.Namespace, item.Name, err.Error()))
		}
	}

	if len(errs) > 0 {
		common.FailWithMessage("部分删除失败: "+strings.Join(errs, "; "), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("批量删除 %d 个 Service 成功", len(targets)), c)
}

// ==================== Ingress ====================

type OneIngress struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Class     string   `json:"class"`
	Hosts     []string `json:"hosts"`
	Address   string   `json:"address"`
	Ports     string   `json:"ports"`
	Age       string   `json:"age"`
	CreatedAt string   `json:"createdAt"`
}

type K8sDeleteIngressReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

type K8sDeleteIngressBatchReq struct {
	ClusterName string           `json:"clusterName"`
	Namespace   string           `json:"namespace"`
	Names       []string         `json:"names"`
	Items       []K8sNetworkItem `json:"items"`
}

func ingressConvert(ing *networkingv1.Ingress) *OneIngress {
	if ing == nil {
		return nil
	}
	var hosts []string
	for _, r := range ing.Spec.Rules {
		if r.Host != "" {
			hosts = append(hosts, r.Host)
		}
	}
	if len(hosts) == 0 {
		hosts = append(hosts, "*")
	}

	var addrs []string
	for _, lb := range ing.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			addrs = append(addrs, lb.IP)
		} else if lb.Hostname != "" {
			addrs = append(addrs, lb.Hostname)
		}
	}
	addrStr := strings.Join(addrs, ", ")
	if addrStr == "" {
		addrStr = "-"
	}

	className := "-"
	if ing.Spec.IngressClassName != nil {
		className = *ing.Spec.IngressClassName
	}

	portsStr := "80"
	if len(ing.Spec.TLS) > 0 {
		portsStr = "80, 443"
	}

	ageStr := "-"
	createdAtStr := "-"
	if !ing.CreationTimestamp.IsZero() {
		createdAtStr = ing.CreationTimestamp.Format("2006-01-02 15:04:05")
		diff := time.Since(ing.CreationTimestamp.Time)
		if diff.Hours() >= 24 {
			ageStr = fmt.Sprintf("%d 天", int(diff.Hours()/24))
		} else if diff.Hours() >= 1 {
			ageStr = fmt.Sprintf("%d 小时", int(diff.Hours()))
		} else {
			ageStr = fmt.Sprintf("%d 分钟", int(diff.Minutes()))
		}
	}

	return &OneIngress{
		Name:      ing.Name,
		Namespace: ing.Namespace,
		Class:     className,
		Hosts:     hosts,
		Address:   addrStr,
		Ports:     portsStr,
		Age:       ageStr,
		CreatedAt: createdAtStr,
	}
}

func getK8sIngressList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")
	keyword := c.Query("keyword")

	if clusterName == "" {
		common.FailWithMessage("clusterName 参数不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	ingList, err := kSet.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		sc.Logger.Error("获取 Ingress 列表失败", zap.Error(err))
		common.FailWithMessage("获取 Ingress 列表失败: "+err.Error(), c)
		return
	}

	var items []*OneIngress
	for i := range ingList.Items {
		ing := &ingList.Items[i]
		if keyword != "" && !strings.Contains(ing.Name, keyword) {
			continue
		}
		itemObj := ingressConvert(ing)
		if itemObj != nil {
			items = append(items, itemObj)
		}
	}

	common.OkWithDetailed(gin.H{
		"items": items,
		"total": len(items),
	}, "ok", c)
}

func getK8sIngressYaml(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")
	name := c.Query("name")

	if clusterName == "" || namespace == "" || name == "" {
		common.FailWithMessage("clusterName、namespace 和 name 不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	ing, err := kSet.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 Ingress 详情失败", zap.Error(err))
		common.FailWithMessage("获取 Ingress 详情失败: "+err.Error(), c)
		return
	}

	ingBytes, err := json.Marshal(ing)
	if err != nil {
		common.FailWithMessage("序列化 Ingress 失败: "+err.Error(), c)
		return
	}

	var mapObj map[string]interface{}
	_ = json.Unmarshal(ingBytes, &mapObj)

	if mapObj["apiVersion"] == nil || mapObj["apiVersion"] == "" {
		mapObj["apiVersion"] = "networking.k8s.io/v1"
	}
	if mapObj["kind"] == nil || mapObj["kind"] == "" {
		mapObj["kind"] = "Ingress"
	}

	delete(mapObj, "status")
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

func createIngressHelper(c *gin.Context, isUpdate bool) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sCreateServiceReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析请求参数失败: "+err.Error(), c)
		return
	}

	if reqObj.ClusterName == "" || reqObj.YamlContent == "" {
		common.FailWithMessage("clusterName 和 yamlContent 不能为空", c)
		return
	}

	_, dSet, dbCluster, err := getClusterClientsetHelper(c, reqObj.ClusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	dec := yaml3.NewDecoder(strings.NewReader(reqObj.YamlContent))
	var rawObj map[string]interface{}
	if err := dec.Decode(&rawObj); err != nil {
		common.FailWithMessage("解析 YAML 内容失败: "+err.Error(), c)
		return
	}

	unstructObj := &unstructured.Unstructured{Object: rawObj}
	gvk := unstructObj.GroupVersionKind()
	if gvk.Kind == "" {
		unstructObj.SetGroupVersionKind(schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"})
		gvk = unstructObj.GroupVersionKind()
	}

	targetNs := unstructObj.GetNamespace()
	if targetNs == "" {
		targetNs = reqObj.Namespace
	}
	if targetNs == "" {
		targetNs = "default"
	}
	unstructObj.SetNamespace(targetNs)

	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: "ingresses",
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	_, err = dSet.Resource(gvr).Namespace(targetNs).Apply(ctx, unstructObj.GetName(), unstructObj, metav1.ApplyOptions{
		FieldManager: "bigdevops-admin",
		Force:        true,
	})
	if err != nil {
		sc.Logger.Error("Apply Ingress 失败", zap.Error(err))
		common.FailWithMessage("保存 Ingress 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("Ingress 保存应用成功", c)
}

func createK8sIngress(c *gin.Context) { createIngressHelper(c, false) }
func updateK8sIngress(c *gin.Context) { createIngressHelper(c, true) }

func deleteK8sIngress(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sDeleteIngressReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, reqObj.ClusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	err = kSet.NetworkingV1().Ingresses(reqObj.Namespace).Delete(ctx, reqObj.Name, metav1.DeleteOptions{})
	if err != nil {
		sc.Logger.Error("删除 Ingress 失败", zap.Error(err))
		common.FailWithMessage("删除 Ingress 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("Ingress [%s] 已删除", reqObj.Name), c)
}

func deleteK8sIngressBatch(c *gin.Context) {
	var reqObj K8sDeleteIngressBatchReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	var targets []K8sNetworkItem
	if len(reqObj.Items) > 0 {
		targets = reqObj.Items
	} else if reqObj.Namespace != "" && len(reqObj.Names) > 0 {
		for _, name := range reqObj.Names {
			targets = append(targets, K8sNetworkItem{Namespace: reqObj.Namespace, Name: name})
		}
	}

	if len(targets) == 0 {
		common.FailWithMessage("待删除列表不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, reqObj.ClusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	var errs []string
	for _, item := range targets {
		if item.Namespace == "" || item.Name == "" {
			continue
		}
		err := kSet.NetworkingV1().Ingresses(item.Namespace).Delete(ctx, item.Name, metav1.DeleteOptions{})
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s/%s: %s", item.Namespace, item.Name, err.Error()))
		}
	}

	if len(errs) > 0 {
		common.FailWithMessage("部分删除失败: "+strings.Join(errs, "; "), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("批量删除 %d 个 Ingress 成功", len(targets)), c)
}

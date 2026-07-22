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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ==================== StatefulSet ====================

type OneStatefulSet struct {
	Name            string   `json:"name"`
	Namespace       string   `json:"namespace"`
	Replicas        int32    `json:"replicas"`
	ReadyReplicas   int32    `json:"readyReplicas"`
	UpdatedReplicas int32    `json:"updatedReplicas"`
	Ready           string   `json:"ready"`
	ServiceName     string   `json:"serviceName"`
	Images          []string `json:"images"`
	Age             string   `json:"age"`
	CreatedAt       string   `json:"createdAt"`
}

type K8sStatefulSetItem struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type K8sCreateStatefulSetReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	YamlContent string `json:"yamlContent"`
}

type K8sScaleStatefulSetReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Replicas    int32  `json:"replicas"`
}

type K8sRestartStatefulSetReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

type K8sDeleteStatefulSetReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

type K8sDeleteStatefulSetBatchReq struct {
	ClusterName string               `json:"clusterName"`
	Namespace   string               `json:"namespace"`
	Names       []string             `json:"names"`
	Items       []K8sStatefulSetItem `json:"items"`
}

func statefulSetConvert(s *appsv1.StatefulSet) *OneStatefulSet {
	if s == nil {
		return nil
	}
	var images []string
	for _, c := range s.Spec.Template.Spec.Containers {
		images = append(images, c.Image)
	}

	replicas := int32(0)
	if s.Spec.Replicas != nil {
		replicas = *s.Spec.Replicas
	}
	readyReplicas := s.Status.ReadyReplicas
	readyStr := fmt.Sprintf("%d/%d", readyReplicas, replicas)

	ageStr := "-"
	createdAtStr := "-"
	if !s.CreationTimestamp.IsZero() {
		createdAtStr = s.CreationTimestamp.Format("2006-01-02 15:04:05")
		diff := time.Since(s.CreationTimestamp.Time)
		if diff.Hours() >= 24 {
			ageStr = fmt.Sprintf("%d 天", int(diff.Hours()/24))
		} else if diff.Hours() >= 1 {
			ageStr = fmt.Sprintf("%d 小时", int(diff.Hours()))
		} else {
			ageStr = fmt.Sprintf("%d 分钟", int(diff.Minutes()))
		}
	}

	return &OneStatefulSet{
		Name:            s.Name,
		Namespace:       s.Namespace,
		Replicas:        replicas,
		ReadyReplicas:   readyReplicas,
		UpdatedReplicas: s.Status.UpdatedReplicas,
		Ready:           readyStr,
		ServiceName:     s.Spec.ServiceName,
		Images:          images,
		Age:             ageStr,
		CreatedAt:       createdAtStr,
	}
}

func getK8sStatefulSetList(c *gin.Context) {
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

	stsList, err := kSet.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		sc.Logger.Error("获取 StatefulSet 列表失败", zap.Error(err))
		common.FailWithMessage("获取 StatefulSet 列表失败: "+err.Error(), c)
		return
	}

	var items []*OneStatefulSet
	for i := range stsList.Items {
		sts := &stsList.Items[i]
		if keyword != "" {
			match := strings.Contains(sts.Name, keyword)
			if !match {
				for _, img := range sts.Spec.Template.Spec.Containers {
					if strings.Contains(img.Image, keyword) {
						match = true
						break
					}
				}
			}
			if !match {
				continue
			}
		}

		itemObj := statefulSetConvert(sts)
		if itemObj != nil {
			items = append(items, itemObj)
		}
	}

	common.OkWithDetailed(gin.H{
		"items": items,
		"total": len(items),
	}, "ok", c)
}

func getK8sStatefulSetYaml(c *gin.Context) {
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

	sts, err := kSet.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 StatefulSet 详情失败", zap.Error(err))
		common.FailWithMessage("获取 StatefulSet 详情失败: "+err.Error(), c)
		return
	}

	stsBytes, err := json.Marshal(sts)
	if err != nil {
		common.FailWithMessage("序列化 StatefulSet 失败: "+err.Error(), c)
		return
	}

	var mapObj map[string]interface{}
	_ = json.Unmarshal(stsBytes, &mapObj)

	if mapObj["apiVersion"] == nil || mapObj["apiVersion"] == "" {
		mapObj["apiVersion"] = "apps/v1"
	}
	if mapObj["kind"] == nil || mapObj["kind"] == "" {
		mapObj["kind"] = "StatefulSet"
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

func createStatefulSetHelper(c *gin.Context, isUpdate bool) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sCreateStatefulSetReq
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
		unstructObj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"})
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
		Resource: "statefulsets",
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	_, err = dSet.Resource(gvr).Namespace(targetNs).Apply(ctx, unstructObj.GetName(), unstructObj, metav1.ApplyOptions{
		FieldManager: "bigdevops-admin",
		Force:        true,
	})
	if err != nil {
		sc.Logger.Error("Apply StatefulSet 失败", zap.Error(err))
		common.FailWithMessage("保存/应用 StatefulSet 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("StatefulSet 保存/应用成功", c)
}

func createK8sStatefulSet(c *gin.Context) { createStatefulSetHelper(c, false) }
func updateK8sStatefulSet(c *gin.Context) { createStatefulSetHelper(c, true) }

func scaleK8sStatefulSet(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sScaleStatefulSetReq
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

	sts, err := kSet.AppsV1().StatefulSets(reqObj.Namespace).Get(ctx, reqObj.Name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 StatefulSet 失败", zap.Error(err))
		common.FailWithMessage("获取 StatefulSet 失败: "+err.Error(), c)
		return
	}

	replicas := reqObj.Replicas
	if replicas < 0 {
		replicas = 0
	}
	sts.Spec.Replicas = &replicas

	_, err = kSet.AppsV1().StatefulSets(reqObj.Namespace).Update(ctx, sts, metav1.UpdateOptions{})
	if err != nil {
		sc.Logger.Error("扩缩容 StatefulSet 失败", zap.Error(err))
		common.FailWithMessage("扩缩容 StatefulSet 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("StatefulSet [%s] 副本数已调整为 %d", reqObj.Name, replicas), c)
}

func restartK8sStatefulSet(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sRestartStatefulSetReq
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

	sts, err := kSet.AppsV1().StatefulSets(reqObj.Namespace).Get(ctx, reqObj.Name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 StatefulSet 失败", zap.Error(err))
		common.FailWithMessage("获取 StatefulSet 失败: "+err.Error(), c)
		return
	}

	if sts.Spec.Template.Annotations == nil {
		sts.Spec.Template.Annotations = make(map[string]string)
	}
	sts.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = kSet.AppsV1().StatefulSets(reqObj.Namespace).Update(ctx, sts, metav1.UpdateOptions{})
	if err != nil {
		sc.Logger.Error("重启 StatefulSet 失败", zap.Error(err))
		common.FailWithMessage("重启 StatefulSet 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("StatefulSet [%s] 滚动重启指令已下发", reqObj.Name), c)
}

func deleteK8sStatefulSet(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sDeleteStatefulSetReq
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

	propagationPolicy := metav1.DeletePropagationBackground
	err = kSet.AppsV1().StatefulSets(reqObj.Namespace).Delete(ctx, reqObj.Name, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		sc.Logger.Error("删除 StatefulSet 失败", zap.Error(err))
		common.FailWithMessage("删除 StatefulSet 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("StatefulSet [%s] 已删除", reqObj.Name), c)
}

func deleteK8sStatefulSetBatch(c *gin.Context) {
	var reqObj K8sDeleteStatefulSetBatchReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	var targets []K8sStatefulSetItem
	if len(reqObj.Items) > 0 {
		targets = reqObj.Items
	} else if reqObj.Namespace != "" && len(reqObj.Names) > 0 {
		for _, name := range reqObj.Names {
			targets = append(targets, K8sStatefulSetItem{Namespace: reqObj.Namespace, Name: name})
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
	propagationPolicy := metav1.DeletePropagationBackground
	for _, item := range targets {
		if item.Namespace == "" || item.Name == "" {
			continue
		}
		err := kSet.AppsV1().StatefulSets(item.Namespace).Delete(ctx, item.Name, metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s/%s: %s", item.Namespace, item.Name, err.Error()))
		}
	}

	if len(errs) > 0 {
		common.FailWithMessage("部分删除失败: "+strings.Join(errs, "; "), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("批量删除 %d 个 StatefulSet 成功", len(targets)), c)
}

// ==================== DaemonSet ====================

type OneDaemonSet struct {
	Name                   string   `json:"name"`
	Namespace              string   `json:"namespace"`
	DesiredNumberScheduled int32    `json:"desiredNumberScheduled"`
	CurrentNumberScheduled int32    `json:"currentNumberScheduled"`
	NumberReady            int32    `json:"numberReady"`
	NumberAvailable        int32    `json:"numberAvailable"`
	Ready                  string   `json:"ready"`
	Images                 []string `json:"images"`
	Age                    string   `json:"age"`
	CreatedAt              string   `json:"createdAt"`
}

type K8sDaemonSetItem struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type K8sCreateDaemonSetReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	YamlContent string `json:"yamlContent"`
}

type K8sRestartDaemonSetReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

type K8sDeleteDaemonSetReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

type K8sDeleteDaemonSetBatchReq struct {
	ClusterName string             `json:"clusterName"`
	Namespace   string             `json:"namespace"`
	Names       []string           `json:"names"`
	Items       []K8sDaemonSetItem `json:"items"`
}

func daemonSetConvert(ds *appsv1.DaemonSet) *OneDaemonSet {
	if ds == nil {
		return nil
	}
	var images []string
	for _, c := range ds.Spec.Template.Spec.Containers {
		images = append(images, c.Image)
	}

	readyStr := fmt.Sprintf("%d/%d", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)

	ageStr := "-"
	createdAtStr := "-"
	if !ds.CreationTimestamp.IsZero() {
		createdAtStr = ds.CreationTimestamp.Format("2006-01-02 15:04:05")
		diff := time.Since(ds.CreationTimestamp.Time)
		if diff.Hours() >= 24 {
			ageStr = fmt.Sprintf("%d 天", int(diff.Hours()/24))
		} else if diff.Hours() >= 1 {
			ageStr = fmt.Sprintf("%d 小时", int(diff.Hours()))
		} else {
			ageStr = fmt.Sprintf("%d 分钟", int(diff.Minutes()))
		}
	}

	return &OneDaemonSet{
		Name:                   ds.Name,
		Namespace:              ds.Namespace,
		DesiredNumberScheduled: ds.Status.DesiredNumberScheduled,
		CurrentNumberScheduled: ds.Status.CurrentNumberScheduled,
		NumberReady:            ds.Status.NumberReady,
		NumberAvailable:        ds.Status.NumberAvailable,
		Ready:                  readyStr,
		Images:                 images,
		Age:                    ageStr,
		CreatedAt:              createdAtStr,
	}
}

func getK8sDaemonSetList(c *gin.Context) {
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

	dsList, err := kSet.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		sc.Logger.Error("获取 DaemonSet 列表失败", zap.Error(err))
		common.FailWithMessage("获取 DaemonSet 列表失败: "+err.Error(), c)
		return
	}

	var items []*OneDaemonSet
	for i := range dsList.Items {
		ds := &dsList.Items[i]
		if keyword != "" {
			match := strings.Contains(ds.Name, keyword)
			if !match {
				for _, img := range ds.Spec.Template.Spec.Containers {
					if strings.Contains(img.Image, keyword) {
						match = true
						break
					}
				}
			}
			if !match {
				continue
			}
		}

		itemObj := daemonSetConvert(ds)
		if itemObj != nil {
			items = append(items, itemObj)
		}
	}

	common.OkWithDetailed(gin.H{
		"items": items,
		"total": len(items),
	}, "ok", c)
}

func getK8sDaemonSetYaml(c *gin.Context) {
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

	ds, err := kSet.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 DaemonSet 详情失败", zap.Error(err))
		common.FailWithMessage("获取 DaemonSet 详情失败: "+err.Error(), c)
		return
	}

	dsBytes, err := json.Marshal(ds)
	if err != nil {
		common.FailWithMessage("序列化 DaemonSet 失败: "+err.Error(), c)
		return
	}

	var mapObj map[string]interface{}
	_ = json.Unmarshal(dsBytes, &mapObj)

	if mapObj["apiVersion"] == nil || mapObj["apiVersion"] == "" {
		mapObj["apiVersion"] = "apps/v1"
	}
	if mapObj["kind"] == nil || mapObj["kind"] == "" {
		mapObj["kind"] = "DaemonSet"
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

func createDaemonSetHelper(c *gin.Context, isUpdate bool) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sCreateDaemonSetReq
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
		unstructObj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"})
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
		Resource: "daemonsets",
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	_, err = dSet.Resource(gvr).Namespace(targetNs).Apply(ctx, unstructObj.GetName(), unstructObj, metav1.ApplyOptions{
		FieldManager: "bigdevops-admin",
		Force:        true,
	})
	if err != nil {
		sc.Logger.Error("Apply DaemonSet 失败", zap.Error(err))
		common.FailWithMessage("保存/应用 DaemonSet 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("DaemonSet 保存/应用成功", c)
}

func createK8sDaemonSet(c *gin.Context) { createDaemonSetHelper(c, false) }
func updateK8sDaemonSet(c *gin.Context) { createDaemonSetHelper(c, true) }

func restartK8sDaemonSet(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sRestartDaemonSetReq
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

	ds, err := kSet.AppsV1().DaemonSets(reqObj.Namespace).Get(ctx, reqObj.Name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 DaemonSet 失败", zap.Error(err))
		common.FailWithMessage("获取 DaemonSet 失败: "+err.Error(), c)
		return
	}

	if ds.Spec.Template.Annotations == nil {
		ds.Spec.Template.Annotations = make(map[string]string)
	}
	ds.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = kSet.AppsV1().DaemonSets(reqObj.Namespace).Update(ctx, ds, metav1.UpdateOptions{})
	if err != nil {
		sc.Logger.Error("重启 DaemonSet 失败", zap.Error(err))
		common.FailWithMessage("重启 DaemonSet 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("DaemonSet [%s] 滚动重启指令已下发", reqObj.Name), c)
}

func deleteK8sDaemonSet(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sDeleteDaemonSetReq
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

	propagationPolicy := metav1.DeletePropagationBackground
	err = kSet.AppsV1().DaemonSets(reqObj.Namespace).Delete(ctx, reqObj.Name, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		sc.Logger.Error("删除 DaemonSet 失败", zap.Error(err))
		common.FailWithMessage("删除 DaemonSet 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("DaemonSet [%s] 已删除", reqObj.Name), c)
}

func deleteK8sDaemonSetBatch(c *gin.Context) {
	var reqObj K8sDeleteDaemonSetBatchReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	var targets []K8sDaemonSetItem
	if len(reqObj.Items) > 0 {
		targets = reqObj.Items
	} else if reqObj.Namespace != "" && len(reqObj.Names) > 0 {
		for _, name := range reqObj.Names {
			targets = append(targets, K8sDaemonSetItem{Namespace: reqObj.Namespace, Name: name})
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
	propagationPolicy := metav1.DeletePropagationBackground
	for _, item := range targets {
		if item.Namespace == "" || item.Name == "" {
			continue
		}
		err := kSet.AppsV1().DaemonSets(item.Namespace).Delete(ctx, item.Name, metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s/%s: %s", item.Namespace, item.Name, err.Error()))
		}
	}

	if len(errs) > 0 {
		common.FailWithMessage("部分删除失败: "+strings.Join(errs, "; "), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("批量删除 %d 个 DaemonSet 成功", len(targets)), c)
}

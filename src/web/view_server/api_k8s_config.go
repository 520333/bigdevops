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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ==================== ConfigMap ====================

type OneConfigMap struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	DataCount int    `json:"dataCount"`
	Age       string `json:"age"`
	CreatedAt string `json:"createdAt"`
}

type K8sConfigItem struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type K8sCreateConfigMapReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	YamlContent string `json:"yamlContent"`
}

type K8sDeleteConfigMapReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

type K8sDeleteConfigMapBatchReq struct {
	ClusterName string          `json:"clusterName"`
	Namespace   string          `json:"namespace"`
	Names       []string        `json:"names"`
	Items       []K8sConfigItem `json:"items"`
}

func configMapConvert(cm *v1.ConfigMap) *OneConfigMap {
	if cm == nil {
		return nil
	}
	dataCount := len(cm.Data) + len(cm.BinaryData)

	ageStr := "-"
	createdAtStr := "-"
	if !cm.CreationTimestamp.IsZero() {
		createdAtStr = cm.CreationTimestamp.Format("2006-01-02 15:04:05")
		diff := time.Since(cm.CreationTimestamp.Time)
		if diff.Hours() >= 24 {
			ageStr = fmt.Sprintf("%d 天", int(diff.Hours()/24))
		} else if diff.Hours() >= 1 {
			ageStr = fmt.Sprintf("%d 小时", int(diff.Hours()))
		} else {
			ageStr = fmt.Sprintf("%d 分钟", int(diff.Minutes()))
		}
	}

	return &OneConfigMap{
		Name:      cm.Name,
		Namespace: cm.Namespace,
		DataCount: dataCount,
		Age:       ageStr,
		CreatedAt: createdAtStr,
	}
}

func getK8sConfigMapList(c *gin.Context) {
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

	cmList, err := kSet.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		sc.Logger.Error("获取 ConfigMap 列表失败", zap.Error(err))
		common.FailWithMessage("获取 ConfigMap 列表失败: "+err.Error(), c)
		return
	}

	var items []*OneConfigMap
	for i := range cmList.Items {
		cm := &cmList.Items[i]
		if keyword != "" && !strings.Contains(cm.Name, keyword) {
			continue
		}
		itemObj := configMapConvert(cm)
		if itemObj != nil {
			items = append(items, itemObj)
		}
	}

	common.OkWithDetailed(gin.H{
		"items": items,
		"total": len(items),
	}, "ok", c)
}

func getK8sConfigMapYaml(c *gin.Context) {
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

	cm, err := kSet.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 ConfigMap 详情失败", zap.Error(err))
		common.FailWithMessage("获取 ConfigMap 详情失败: "+err.Error(), c)
		return
	}

	cmBytes, err := json.Marshal(cm)
	if err != nil {
		common.FailWithMessage("序列化 ConfigMap 失败: "+err.Error(), c)
		return
	}

	var mapObj map[string]interface{}
	_ = json.Unmarshal(cmBytes, &mapObj)

	if mapObj["apiVersion"] == nil || mapObj["apiVersion"] == "" {
		mapObj["apiVersion"] = "v1"
	}
	if mapObj["kind"] == nil || mapObj["kind"] == "" {
		mapObj["kind"] = "ConfigMap"
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

func createConfigMapHelper(c *gin.Context, isUpdate bool) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sCreateConfigMapReq
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
		unstructObj.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
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
		Resource: "configmaps",
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	_, err = dSet.Resource(gvr).Namespace(targetNs).Apply(ctx, unstructObj.GetName(), unstructObj, metav1.ApplyOptions{
		FieldManager: "bigdevops-admin",
		Force:        true,
	})
	if err != nil {
		sc.Logger.Error("Apply ConfigMap 失败", zap.Error(err))
		common.FailWithMessage("保存 ConfigMap 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("ConfigMap 保存应用成功", c)
}

func createK8sConfigMap(c *gin.Context) { createConfigMapHelper(c, false) }
func updateK8sConfigMap(c *gin.Context) { createConfigMapHelper(c, true) }

func deleteK8sConfigMap(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sDeleteConfigMapReq
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

	err = kSet.CoreV1().ConfigMaps(reqObj.Namespace).Delete(ctx, reqObj.Name, metav1.DeleteOptions{})
	if err != nil {
		sc.Logger.Error("删除 ConfigMap 失败", zap.Error(err))
		common.FailWithMessage("删除 ConfigMap 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("ConfigMap [%s] 已删除", reqObj.Name), c)
}

func deleteK8sConfigMapBatch(c *gin.Context) {
	var reqObj K8sDeleteConfigMapBatchReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	var targets []K8sConfigItem
	if len(reqObj.Items) > 0 {
		targets = reqObj.Items
	} else if reqObj.Namespace != "" && len(reqObj.Names) > 0 {
		for _, name := range reqObj.Names {
			targets = append(targets, K8sConfigItem{Namespace: reqObj.Namespace, Name: name})
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
		err := kSet.CoreV1().ConfigMaps(item.Namespace).Delete(ctx, item.Name, metav1.DeleteOptions{})
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s/%s: %s", item.Namespace, item.Name, err.Error()))
		}
	}

	if len(errs) > 0 {
		common.FailWithMessage("部分删除失败: "+strings.Join(errs, "; "), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("批量删除 %d 个 ConfigMap 成功", len(targets)), c)
}

// ==================== Secret ====================

type OneSecret struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	DataCount int    `json:"dataCount"`
	Age       string `json:"age"`
	CreatedAt string `json:"createdAt"`
}

type K8sDeleteSecretReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

type K8sDeleteSecretBatchReq struct {
	ClusterName string          `json:"clusterName"`
	Namespace   string          `json:"namespace"`
	Names       []string        `json:"names"`
	Items       []K8sConfigItem `json:"items"`
}

func secretConvert(sec *v1.Secret) *OneSecret {
	if sec == nil {
		return nil
	}
	dataCount := len(sec.Data) + len(sec.StringData)

	ageStr := "-"
	createdAtStr := "-"
	if !sec.CreationTimestamp.IsZero() {
		createdAtStr = sec.CreationTimestamp.Format("2006-01-02 15:04:05")
		diff := time.Since(sec.CreationTimestamp.Time)
		if diff.Hours() >= 24 {
			ageStr = fmt.Sprintf("%d 天", int(diff.Hours()/24))
		} else if diff.Hours() >= 1 {
			ageStr = fmt.Sprintf("%d 小时", int(diff.Hours()))
		} else {
			ageStr = fmt.Sprintf("%d 分钟", int(diff.Minutes()))
		}
	}

	return &OneSecret{
		Name:      sec.Name,
		Namespace: sec.Namespace,
		Type:      string(sec.Type),
		DataCount: dataCount,
		Age:       ageStr,
		CreatedAt: createdAtStr,
	}
}

func getK8sSecretList(c *gin.Context) {
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

	secList, err := kSet.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		sc.Logger.Error("获取 Secret 列表失败", zap.Error(err))
		common.FailWithMessage("获取 Secret 列表失败: "+err.Error(), c)
		return
	}

	var items []*OneSecret
	for i := range secList.Items {
		sec := &secList.Items[i]
		if keyword != "" && !strings.Contains(sec.Name, keyword) {
			continue
		}
		itemObj := secretConvert(sec)
		if itemObj != nil {
			items = append(items, itemObj)
		}
	}

	common.OkWithDetailed(gin.H{
		"items": items,
		"total": len(items),
	}, "ok", c)
}

func getK8sSecretYaml(c *gin.Context) {
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

	sec, err := kSet.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 Secret 详情失败", zap.Error(err))
		common.FailWithMessage("获取 Secret 详情失败: "+err.Error(), c)
		return
	}

	secBytes, err := json.Marshal(sec)
	if err != nil {
		common.FailWithMessage("序列化 Secret 失败: "+err.Error(), c)
		return
	}

	var mapObj map[string]interface{}
	_ = json.Unmarshal(secBytes, &mapObj)

	if mapObj["apiVersion"] == nil || mapObj["apiVersion"] == "" {
		mapObj["apiVersion"] = "v1"
	}
	if mapObj["kind"] == nil || mapObj["kind"] == "" {
		mapObj["kind"] = "Secret"
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

func createSecretHelper(c *gin.Context, isUpdate bool) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sCreateConfigMapReq
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
		unstructObj.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"})
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
		Resource: "secrets",
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	_, err = dSet.Resource(gvr).Namespace(targetNs).Apply(ctx, unstructObj.GetName(), unstructObj, metav1.ApplyOptions{
		FieldManager: "bigdevops-admin",
		Force:        true,
	})
	if err != nil {
		sc.Logger.Error("Apply Secret 失败", zap.Error(err))
		common.FailWithMessage("保存 Secret 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("Secret 保存应用成功", c)
}

func createK8sSecret(c *gin.Context) { createSecretHelper(c, false) }
func updateK8sSecret(c *gin.Context) { createSecretHelper(c, true) }

func deleteK8sSecret(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sDeleteSecretReq
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

	err = kSet.CoreV1().Secrets(reqObj.Namespace).Delete(ctx, reqObj.Name, metav1.DeleteOptions{})
	if err != nil {
		sc.Logger.Error("删除 Secret 失败", zap.Error(err))
		common.FailWithMessage("删除 Secret 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("Secret [%s] 已删除", reqObj.Name), c)
}

func deleteK8sSecretBatch(c *gin.Context) {
	var reqObj K8sDeleteSecretBatchReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	var targets []K8sConfigItem
	if len(reqObj.Items) > 0 {
		targets = reqObj.Items
	} else if reqObj.Namespace != "" && len(reqObj.Names) > 0 {
		for _, name := range reqObj.Names {
			targets = append(targets, K8sConfigItem{Namespace: reqObj.Namespace, Name: name})
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
		err := kSet.CoreV1().Secrets(item.Namespace).Delete(ctx, item.Name, metav1.DeleteOptions{})
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s/%s: %s", item.Namespace, item.Name, err.Error()))
		}
	}

	if len(errs) > 0 {
		common.FailWithMessage("部分删除失败: "+strings.Join(errs, "; "), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("批量删除 %d 个 Secret 成功", len(targets)), c)
}

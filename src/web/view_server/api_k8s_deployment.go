package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"context"
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

type OneDeployment struct {
	Name              string   `json:"name"`
	Namespace         string   `json:"namespace"`
	Replicas          int32    `json:"replicas"`
	ReadyReplicas     int32    `json:"readyReplicas"`
	AvailableReplicas int32    `json:"availableReplicas"`
	UpdatedReplicas   int32    `json:"updatedReplicas"`
	Ready             string   `json:"ready"`
	Strategy          string   `json:"strategy"`
	Images            []string `json:"images"`
	Age               string   `json:"age"`
	CreatedAt         string   `json:"createdAt"`
}

type K8sCreateDeploymentReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	YamlContent string `json:"yamlContent"`
}

type K8sScaleDeploymentReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Replicas    int32  `json:"replicas"`
}

type K8sRestartDeploymentReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

type K8sDeleteDeploymentReq struct {
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

type K8sDeploymentItem struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type K8sDeleteDeploymentBatchReq struct {
	ClusterName string              `json:"clusterName"`
	Namespace   string              `json:"namespace"`
	Names       []string            `json:"names"`
	Items       []K8sDeploymentItem `json:"items"`
}

func deploymentConvert(d *appsv1.Deployment) *OneDeployment {
	if d == nil {
		return nil
	}
	var images []string
	for _, c := range d.Spec.Template.Spec.Containers {
		images = append(images, c.Image)
	}

	strategy := string(d.Spec.Strategy.Type)
	if strategy == "" {
		strategy = "RollingUpdate"
	}

	replicas := int32(0)
	if d.Spec.Replicas != nil {
		replicas = *d.Spec.Replicas
	}

	readyReplicas := d.Status.ReadyReplicas
	availableReplicas := d.Status.AvailableReplicas
	updatedReplicas := d.Status.UpdatedReplicas

	readyStr := fmt.Sprintf("%d/%d", readyReplicas, replicas)

	ageStr := "-"
	createdAtStr := "-"
	if !d.CreationTimestamp.IsZero() {
		createdAtStr = d.CreationTimestamp.Format("2006-01-02 15:04:05")
		diff := time.Since(d.CreationTimestamp.Time)
		if diff.Hours() >= 24 {
			ageStr = fmt.Sprintf("%d 天", int(diff.Hours()/24))
		} else if diff.Hours() >= 1 {
			ageStr = fmt.Sprintf("%d 小时", int(diff.Hours()))
		} else {
			ageStr = fmt.Sprintf("%d 分钟", int(diff.Minutes()))
		}
	}

	return &OneDeployment{
		Name:              d.Name,
		Namespace:         d.Namespace,
		Replicas:          replicas,
		ReadyReplicas:     readyReplicas,
		AvailableReplicas: availableReplicas,
		UpdatedReplicas:   updatedReplicas,
		Ready:             readyStr,
		Strategy:          strategy,
		Images:            images,
		Age:               ageStr,
		CreatedAt:         createdAtStr,
	}
}

// getK8sDeploymentList 获取指定集群与命名空间的 Deployment 控制器列表
func getK8sDeploymentList(c *gin.Context) {
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
		sc.Logger.Error("获取集群 ClientSet 失败", zap.String("clusterName", clusterName), zap.Error(err))
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	deployList, err := kSet.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		sc.Logger.Error("获取 Deployment 列表失败", zap.String("clusterName", clusterName), zap.String("namespace", namespace), zap.Error(err))
		common.FailWithMessage("获取 Deployment 列表失败: "+err.Error(), c)
		return
	}

	var items []*OneDeployment
	for i := range deployList.Items {
		d := &deployList.Items[i]
		if keyword != "" {
			match := strings.Contains(d.Name, keyword)
			if !match {
				for _, img := range d.Spec.Template.Spec.Containers {
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

		itemObj := deploymentConvert(d)
		if itemObj != nil {
			items = append(items, itemObj)
		}
	}

	common.OkWithDetailed(gin.H{
		"items": items,
		"total": len(items),
	}, "ok", c)
}

// getK8sDeploymentYaml 获取单个 Deployment 的完整 YAML 源码
func getK8sDeploymentYaml(c *gin.Context) {
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

	deploy, err := kSet.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 Deployment 详情失败", zap.String("name", name), zap.Error(err))
		common.FailWithMessage("获取 Deployment 详情失败: "+err.Error(), c)
		return
	}

	deployBytes, err := json.Marshal(deploy)
	if err != nil {
		common.FailWithMessage("序列化 Deployment 失败: "+err.Error(), c)
		return
	}

	var mapObj map[string]interface{}
	_ = json.Unmarshal(deployBytes, &mapObj)

	if mapObj["apiVersion"] == nil || mapObj["apiVersion"] == "" {
		mapObj["apiVersion"] = "apps/v1"
	}
	if mapObj["kind"] == nil || mapObj["kind"] == "" {
		mapObj["kind"] = "Deployment"
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

// createK8sDeployment 新建 Deployment (动态 Apply)
func createK8sDeployment(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sCreateDeploymentReq
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
		unstructObj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
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
		Resource: "deployments",
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	_, err = dSet.Resource(gvr).Namespace(targetNs).Apply(ctx, unstructObj.GetName(), unstructObj, metav1.ApplyOptions{
		FieldManager: "bigdevops-admin",
		Force:        true,
	})
	if err != nil {
		sc.Logger.Error("Apply Deployment 失败", zap.String("name", unstructObj.GetName()), zap.Error(err))
		common.FailWithMessage("创建/应用 Deployment 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("Deployment 创建/应用成功", c)
}

// updateK8sDeployment 更新 Deployment
func updateK8sDeployment(c *gin.Context) {
	createK8sDeployment(c)
}

// scaleK8sDeployment 扩缩容 Deployment 副本数
func scaleK8sDeployment(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sScaleDeploymentReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	if reqObj.ClusterName == "" || reqObj.Namespace == "" || reqObj.Name == "" {
		common.FailWithMessage("clusterName、namespace 和 name 参数不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, reqObj.ClusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	deploy, err := kSet.AppsV1().Deployments(reqObj.Namespace).Get(ctx, reqObj.Name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 Deployment 失败", zap.String("name", reqObj.Name), zap.Error(err))
		common.FailWithMessage("获取 Deployment 失败: "+err.Error(), c)
		return
	}

	replicas := reqObj.Replicas
	if replicas < 0 {
		replicas = 0
	}
	deploy.Spec.Replicas = &replicas

	_, err = kSet.AppsV1().Deployments(reqObj.Namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		sc.Logger.Error("扩缩容 Deployment 失败", zap.String("name", reqObj.Name), zap.Error(err))
		common.FailWithMessage("扩缩容 Deployment 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("Deployment [%s] 副本数已调整为 %d", reqObj.Name, replicas), c)
}

// restartK8sDeployment 滚动重启 Deployment (Rollout Restart)
func restartK8sDeployment(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sRestartDeploymentReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	if reqObj.ClusterName == "" || reqObj.Namespace == "" || reqObj.Name == "" {
		common.FailWithMessage("clusterName、namespace 和 name 参数不能为空", c)
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, reqObj.ClusterName)
	if err != nil {
		common.FailWithMessage("获取集群客户端失败: "+err.Error(), c)
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	deploy, err := kSet.AppsV1().Deployments(reqObj.Namespace).Get(ctx, reqObj.Name, metav1.GetOptions{})
	if err != nil {
		sc.Logger.Error("获取 Deployment 失败", zap.String("name", reqObj.Name), zap.Error(err))
		common.FailWithMessage("获取 Deployment 失败: "+err.Error(), c)
		return
	}

	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = kSet.AppsV1().Deployments(reqObj.Namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		sc.Logger.Error("重启 Deployment 失败", zap.String("name", reqObj.Name), zap.Error(err))
		common.FailWithMessage("重启 Deployment 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("Deployment [%s] 重新滚动重启指令已下发", reqObj.Name), c)
}

// deleteK8sDeployment 删除单个 Deployment
func deleteK8sDeployment(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj K8sDeleteDeploymentReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	if reqObj.ClusterName == "" || reqObj.Namespace == "" || reqObj.Name == "" {
		common.FailWithMessage("clusterName、namespace 和 name 不能为空", c)
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
	err = kSet.AppsV1().Deployments(reqObj.Namespace).Delete(ctx, reqObj.Name, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		sc.Logger.Error("删除 Deployment 失败", zap.String("name", reqObj.Name), zap.Error(err))
		common.FailWithMessage("删除 Deployment 失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("Deployment [%s] 已删除", reqObj.Name), c)
}

// deleteK8sDeploymentBatch 批量删除 Deployment
func deleteK8sDeploymentBatch(c *gin.Context) {
	var reqObj K8sDeleteDeploymentBatchReq
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析参数失败: "+err.Error(), c)
		return
	}

	if reqObj.ClusterName == "" {
		common.FailWithMessage("clusterName 参数不能为空", c)
		return
	}

	var targets []K8sDeploymentItem
	if len(reqObj.Items) > 0 {
		targets = reqObj.Items
	} else if reqObj.Namespace != "" && len(reqObj.Names) > 0 {
		for _, name := range reqObj.Names {
			targets = append(targets, K8sDeploymentItem{
				Namespace: reqObj.Namespace,
				Name:      name,
			})
		}
	}

	if len(targets) == 0 {
		common.FailWithMessage("待删除的 Deployment 列表不能为空", c)
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
		err := kSet.AppsV1().Deployments(item.Namespace).Delete(ctx, item.Name, metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s/%s: %s", item.Namespace, item.Name, err.Error()))
		}
	}

	if len(errs) > 0 {
		common.FailWithMessage("部分 Deployment 删除失败: "+strings.Join(errs, "; "), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("批量删除 %d 个 Deployment 成功", len(targets)), c)
}

// wsK8sDeploymentWatch 实时 Watch Deployment 变更
func wsK8sDeploymentWatch(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	clusterName := c.Query("clusterName")
	namespace := c.Query("namespace")

	if clusterName == "" {
		common.FailWithMessage("clusterName 参数不能为空", c)
		return
	}

	wsConn, err := podExecUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		sc.Logger.Error("WebSocket Watch 握线失败", zap.Error(err))
		return
	}
	defer wsConn.Close()

	_, _, dbCluster, err := getClusterClientsetHelper(c, clusterName)
	if err != nil {
		sc.Logger.Error("获取集群配置失败", zap.Error(err))
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

	deployList, err := kSetStream.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		sc.Logger.Error("Watch 前获取 Deployment 列表失败", zap.Error(err))
		_ = wsConn.WriteJSON(gin.H{"type": "ERROR", "message": "获取 Deployment 列表失败: " + err.Error()})
		return
	}

	opts := metav1.ListOptions{
		ResourceVersion: deployList.ResourceVersion,
	}

	watcher, err := kSetStream.AppsV1().Deployments(namespace).Watch(ctx, opts)
	if err != nil {
		sc.Logger.Error("创建 Deployment Watch 失败", zap.Error(err))
		_ = wsConn.WriteJSON(gin.H{"type": "ERROR", "message": "创建 Watch 失败: " + err.Error()})
		return
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return
			}
			d, ok := event.Object.(*appsv1.Deployment)
			if !ok || d == nil {
				continue
			}

			itemObj := deploymentConvert(d)
			if itemObj == nil {
				continue
			}

			err := wsConn.WriteJSON(gin.H{
				"type":       string(event.Type),
				"deployment": itemObj,
			})
			if err != nil {
				return
			}
		}
	}
}

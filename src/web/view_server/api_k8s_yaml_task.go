package view_server

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
)

// getK8sYamlTaskList 获取K8s YAML发布任务列表
// @Summary      获取K8s YAML发布任务列表
// @Description  分页查询K8s YAML发布任务列表
// @Tags         k8s YAML管理模块
// @Accept       json
// @Produce      json
// @Param        page            query     int     false  "页码" default(1)
// @Param        pageSize        query     int     false  "每页数量" default(10)
// @Param        name            query     string  false  "任务名称"
// @Param        createUserName  query     string  false  "创建人名称"
// @Success      200             {object}  map[string]interface{} "成功响应"
// @Failure      400             {object}  map[string]interface{} "请求错误"
// @Failure      500             {object}  map[string]interface{} "服务器错误"
// @Security     Bearer
// @Router       /k8s/getK8sYamlTaskList [get]
func getK8sYamlTaskList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")

	searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchTitle := c.DefaultQuery("name", "")
	searchCreateUserName := c.DefaultQuery("createUserName", "")
	searchTemplateName := c.DefaultQuery("templateName", "")

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}
	objs, err := models.GetK8sYamlTaskAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的k8s集群yaml执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的k8s集群yaml执行错误：%v", err.Error()), c)
		return
	}
	allIds := []int{}
	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}

		if searchTitle != "" && !strings.Contains(obj.Name, searchTitle) {
			continue
		}

		obj.FillFrontAllData()

		if searchCreateUserName != "" && !strings.Contains(obj.CreateUserName, searchCreateUserName) {
			continue
		}

		if searchTemplateName != "" && !strings.Contains(obj.TemplateName, searchTemplateName) {
			continue
		}

		allIds = append(allIds, int(obj.ID))
	}

	if len(allIds) == 0 {
		common.OkWithDetailed(gin.H{
			"items": []models.K8sYamlTask{},
			"total": 0,
		}, "ok", c)
		return
	}

	pagedObjs, err := models.GetK8sYamlTaskByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的k8s集群yaml执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的k8s集群yaml执行错误：%v", err.Error()), c)
		return
	}

	for _, obj := range pagedObjs {
		obj.FillFrontAllData()
	}

	common.OkWithDetailed(gin.H{
		"items": pagedObjs,
		"total": len(allIds),
	}, "ok", c)
}

// createK8sYamlTask 创建K8s YAML发布任务
// @Summary      创建K8s YAML发布任务
// @Description  创建新的K8s YAML发布任务
// @Tags         k8s YAML管理模块
// @Accept       json
// @Produce      json
// @Param        data  body      models.K8sYamlTask  true  "YAML任务数据"
// @Success      200   {object}  map[string]interface{} "成功响应"
// @Failure      400   {object}  map[string]interface{} "请求错误"
// @Failure      500   {object}  map[string]interface{} "服务器错误"
// @Security     Bearer
// @Router       /k8s/createK8sYamlTask [post]
func createK8sYamlTask(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj models.K8sYamlTask
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增k8s集群yaml任务执行请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 获取当前用户ID
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)

	dbUser, err := models.GetUserByUsername(userName)
	if dbUser != nil {
		reqObj.UserID = dbUser.ID
	}
	reqObj.Status = common.K8S_YAMLTASK_STATUS_PENDING

	// 存入数据库
	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增k8s集群yaml执行数据库失败", zap.Error(err))
		common.FailWithMessage("存入数据库失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

// updateK8sYamlTask 更新K8s YAML发布任务
// @Summary      更新K8s YAML发布任务
// @Description  更新现有的K8s YAML发布任务
// @Tags         k8s YAML管理模块
// @Accept       json
// @Produce      json
// @Param        data  body      models.K8sYamlTask  true  "YAML任务数据"
// @Success      200   {object}  map[string]interface{} "成功响应"
// @Failure      400   {object}  map[string]interface{} "请求错误"
// @Failure      500   {object}  map[string]interface{} "服务器错误"
// @Security     Bearer
// @Router       /k8s/updateK8sYamlTask [post]
func updateK8sYamlTask(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj models.K8sYamlTask
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新k8s集群yaml任务请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 检查是否存在
	dbOld, err := models.GetK8sYamlTaskById(int(reqObj.ID))
	if err != nil {
		common.FailWithMessage("k8s集群yaml任务不存在", c)
		return
	}

	reqObj.UserID = dbOld.UserID

	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新k8s集群yaml任务执行错误", zap.Error(err))
		common.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

// deleteK8sYamlTask 删除K8s YAML发布任务
// @Summary      删除K8s YAML发布任务
// @Description  根据ID删除K8s YAML发布任务
// @Tags         k8s YAML管理模块
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "任务ID"
// @Success      200  {object}  map[string]interface{} "成功响应"
// @Failure      400  {object}  map[string]interface{} "请求错误"
// @Failure      500  {object}  map[string]interface{} "服务器错误"
// @Security     Bearer
// @Router       /k8s/deleteK8sYamlTask/{id} [delete]
func deleteK8sYamlTask(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetK8sYamlTaskById(intVar)
	if err != nil {
		common.FailWithMessage("k8s集群yaml任务不存在", c)
		return
	}

	err = dbObj.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除k8s集群yaml任务执行错误", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

// applyK8sYamlTaskOne 执行K8s YAML发布任务 (Apply)
// @Summary      执行K8s YAML发布任务 (Apply)
// @Description  根据ID对指定的K8s集群动态应用 (Apply) 渲染后的YAML
// @Tags         k8s YAML管理模块
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "任务ID"
// @Success      200  {object}  map[string]interface{} "成功响应"
// @Failure      400  {object}  map[string]interface{} "请求错误"
// @Failure      500  {object}  map[string]interface{} "服务器错误"
// @Security     Bearer
// @Router       /k8s/applyK8sYamlTaskOne/{id} [post]
func applyK8sYamlTaskOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetK8sYamlTaskById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找yaml任务执行错误", zap.Any("yaml任务", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 获取模板
	dbTemplate, err := models.GetK8sYamlTemplateById(int(dbObj.TemplateId))
	if err != nil {
		sc.Logger.Error("根据id找yaml任务错误", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 获取集群
	dbCluster, err := models.GetK8sClusterByName(dbObj.ClusterName)
	if err != nil {
		sc.Logger.Error("根据name找k8s集群错误", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	kc := c.MustGet(common.GIN_CTX_K8S_CACHE).(*cache.K8sClusterCache)
	kSet := kc.GetClusterClientSetById(dbCluster.ID)
	dSet := kc.GetClusterDynamicClientById(dbCluster.ID)
	if kSet == nil || dSet == nil {
		sc.Logger.Error("根据id获取k8s集群ClientSet/DynamicClient失败", zap.String("clusterName", dbObj.ClusterName))
		common.FailWithMessage("获取集群客户端句柄失败，请检查集群连接状态", c)
		return
	}
	// 拼接yaml变量 拿到kSet apply执行
	yamlString := dbTemplate.Content
	for _, kv := range dbObj.Variables {
		kvs := strings.Split(kv, "=")
		if len(kvs) != 2 {
			continue
		}
		k := kvs[0]
		v := kvs[1]
		yamlString = strings.ReplaceAll(yamlString, k, v)
	}

	var operatorUserID uint
	if userName, ok := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string); ok && userName != "" {
		if dbUser, _ := models.GetUserByUsername(userName); dbUser != nil {
			operatorUserID = dbUser.ID
		}
	}

	logObj := &models.K8sYamlTaskLog{
		TaskId:      dbObj.ID,
		TaskName:    dbObj.Name,
		ClusterName: dbObj.ClusterName,
		TemplateId:  dbObj.TemplateId,
		YamlContent: yamlString,
		UserID:      operatorUserID,
	}

	err = DynamicObjApply(kSet, dSet, dbCluster.ActionTimeoutSeconds, []byte(yamlString))
	if err != nil {
		logObj.Status = "FAILED"
		logObj.ErrMsg = err.Error()
		_ = logObj.CreateOne()

		sc.Logger.Error("执行YAML动态应用失败", zap.Error(err))
		dbObj.Status = common.K8S_YAMLTASK_STATUS_FAILED
		_ = dbObj.UpdateOne()
		common.FailWithMessage("执行YAML动态应用失败: "+err.Error(), c)
		return
	}

	logObj.Status = "SUCCESS"
	_ = logObj.CreateOne()

	dbObj.Status = common.K8S_YAMLTASK_STATUS_APPLIED
	err = dbObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新任务状态错误", zap.Error(err))
		common.FailWithMessage("更新任务状态错误: "+err.Error(), c)
		return
	}

	dbObj.FillFrontAllData()
	common.OkWithDetailed(dbObj, "ok", c)
}

func DynamicObjApply(kClient kubernetes.Interface, dyClient dynamic.Interface, tw int, fileBytes []byte) error {
	ctx1, cancel1 := common.GenTimeoutContext(tw)
	defer cancel1()

	// 1. 创建 RestMapper (用于将 GroupVersionKind 映射为 RESTMapping / GroupVersionResource)
	grs, err := restmapper.GetAPIGroupResources(kClient.Discovery())
	if err != nil {
		return fmt.Errorf("获取 Discovery APIGroupResources 失败: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(grs)

	// 2. 初始化 YAML / JSON 解码器 (bufferSize: 4096)
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(fileBytes), 4096)

	// 3. 循环切割并解码 YAML (支持单个 YAML 文件中包含多段 --- 分隔符)
	for {
		ext := runtime.RawExtension{}
		if err := decoder.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("解码 YAML 字节流失败: %w", err)
		}

		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}

		// 4. 将 Raw 字节流反序列化为 Unstructured 对象
		obj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(ext.Raw, nil, nil)
		if err != nil {
			return fmt.Errorf("解析 Unstructured 对象失败: %w", err)
		}

		unstructObj, ok := obj.(*unstructured.Unstructured)
		if !ok || unstructObj == nil {
			continue
		}

		// 5. 查找 GVK 对应的 RESTMapping (获取 Resource 映射及 Namespace Scope)
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("查找 GVK [%s] 的 RESTMapping 失败: %w", gvk.String(), err)
		}

		// 6. 构建 ResourceInterface (区分 Namespace 资源与 Cluster 级别资源)
		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			ns := unstructObj.GetNamespace()
			if ns == "" {
				ns = "default"
			}
			dr = dyClient.Resource(mapping.Resource).Namespace(ns)
		} else {
			dr = dyClient.Resource(mapping.Resource)
		}

		// 7. 执行 Server-side Apply / Fallback Create or Update
		data, err := json.Marshal(unstructObj)
		if err != nil {
			return fmt.Errorf("序列化 Unstructured 对象失败: %w", err)
		}

		name := unstructObj.GetName()
		force := true
		_, err = dr.Patch(ctx1, name, types.ApplyPatchType, data, metav1.PatchOptions{
			FieldManager: "bigdevops-yaml-applier",
			Force:        &force,
		})
		if err != nil {
			// 如果 Patch 失败，退回到 Get + Create/Update 模式
			_, getErr := dr.Get(ctx1, name, metav1.GetOptions{})
			if getErr != nil {
				_, err = dr.Create(ctx1, unstructObj, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("应用资源 [%s/%s] 失败: %w", gvk.Kind, name, err)
				}
			} else {
				unstructObj.SetResourceVersion("")
				_, err = dr.Update(ctx1, unstructObj, metav1.UpdateOptions{})
				if err != nil {
					return fmt.Errorf("更新资源 [%s/%s] 失败: %w", gvk.Kind, name, err)
				}
			}
		}
	}

	return nil
}

// getK8sYamlTaskLogList 获取K8s YAML发布历史记录列表
// @Summary      获取K8s YAML发布历史记录列表
// @Description  根据任务ID分页获取历史应用记录
// @Tags         k8s YAML管理模块
// @Accept       json
// @Produce      json
// @Param        taskId   query     int     false  "任务ID"
// @Param        page     query     int     false  "页码"
// @Param        pageSize query     int     false  "每页数量"
// @Success      200      {object}  map[string]interface{} "成功响应"
// @Security     Bearer
// @Router       /k8s/getK8sYamlTaskLogList [get]
func getK8sYamlTaskLogList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	taskIdStr := c.Query("taskId")
	pageStr := c.Query("page")
	pageSizeStr := c.Query("pageSize")

	taskId, _ := strconv.Atoi(taskIdStr)
	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	objs, total, err := models.GetK8sYamlTaskLogListByTaskId(taskId, page, pageSize)
	if err != nil {
		sc.Logger.Error("查询YAML任务执行历史记录列表失败", zap.Error(err))
		common.FailWithMessage("查询历史记录列表失败: "+err.Error(), c)
		return
	}

	for _, obj := range objs {
		obj.FillFrontAllData()
	}

	common.OkWithDetailed(gin.H{
		"items":    objs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	}, "ok", c)
}

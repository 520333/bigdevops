package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ==================== K8s Project APIs ====================

// getK8sProjectList 获取K8s项目列表
// @Summary      获取K8s项目列表
// @Tags         k8s项目应用实例模块
// @Accept       json
// @Produce      json
// @Param        page            query     int     false  "页码" default(1)
// @Param        pageSize        query     int     false  "每页数量" default(10)
// @Param        name            query     string  false  "项目名称"
// @Param        cluster         query     string  false  "集群名称"
// @Param        createUserName  query     string  false  "创建人名称"
// @Success      200             {object}  map[string]interface{} "成功响应"
// @Router       /k8s/getK8sProjectList [get]
// @Security     Bearer
func getK8sProjectList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")
	searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchTitle := c.DefaultQuery("name", "")
	searchCluster := c.DefaultQuery("cluster", "")
	searchCreateUserName := c.DefaultQuery("createUserName", "")

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	objs, err := models.GetK8sProjectAll()
	if err != nil {
		sc.Logger.Error("查询所有K8s项目错误", zap.Error(err))
		common.ReqBadFailWithMessage("查询K8s项目列表失败: "+err.Error(), c)
		return
	}

	var allIds []int
	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}
		if searchTitle != "" && !strings.Contains(obj.Name, searchTitle) && !strings.Contains(obj.NameZh, searchTitle) {
			continue
		}
		if searchCluster != "" && !strings.Contains(obj.Cluster, searchCluster) {
			continue
		}

		obj.FillFrontAllData()
		if searchCreateUserName != "" && !strings.Contains(obj.CreateUserName, searchCreateUserName) {
			continue
		}
		allIds = append(allIds, int(obj.ID))
	}

	if len(allIds) == 0 {
		common.OkWithDetailed(gin.H{
			"items": []models.K8sProject{},
			"total": 0,
		}, "ok", c)
		return
	}

	pagedObjs, err := models.GetK8sProjectByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("分页查询K8s项目失败", zap.Error(err))
		common.ReqBadFailWithMessage("分页查询K8s项目失败: "+err.Error(), c)
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

// getK8sProjectOne 获取单个K8s项目详情
// @Summary      获取单个K8s项目详情
// @Tags         k8s项目应用实例模块
// @Router       /k8s/getK8sProjectOne/:id [get]
// @Security     Bearer
func getK8sProjectOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	idStr := c.Param("id")
	if idStr == "" {
		idStr = c.Query("id")
	}
	id, _ := strconv.Atoi(idStr)
	if id <= 0 {
		common.FailWithMessage("ID参数错误", c)
		return
	}

	obj, err := models.GetK8sProjectById(id)
	if err != nil {
		sc.Logger.Error("获取K8s项目失败", zap.Error(err))
		common.FailWithMessage("获取K8s项目失败: "+err.Error(), c)
		return
	}
	obj.FillFrontAllData()

	common.OkWithData(obj, c)
}

// createK8sProject 创建K8s项目
// @Summary      创建K8s项目
// @Tags         k8s项目应用实例模块
// @Router       /k8s/createK8sProject [post]
// @Security     Bearer
func createK8sProject(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.K8sProject
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增K8s项目请求失败", zap.Error(err))
		common.FailWithMessage("请求参数格式错误: "+err.Error(), c)
		return
	}

	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, _ := models.GetUserByUsername(userName)
	if dbUser != nil {
		reqObj.UserID = dbUser.ID
	}

	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增K8s项目到数据库失败", zap.Error(err))
		common.FailWithMessage("创建失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

// updateK8sProject 更新K8s项目
// @Summary      更新K8s项目
// @Tags         k8s项目应用实例模块
// @Router       /k8s/updateK8sProject [post]
// @Security     Bearer
func updateK8sProject(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.K8sProject
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新K8s项目请求失败", zap.Error(err))
		common.FailWithMessage("请求参数格式错误: "+err.Error(), c)
		return
	}

	// 策略1防呆锁：如果关联应用非空，禁止修改集群
	if reqObj.ID > 0 {
		oldProj, err := models.GetK8sProjectById(int(reqObj.ID))
		if err == nil && oldProj != nil {
			if reqObj.Cluster != "" && reqObj.Cluster != oldProj.Cluster {
				if len(oldProj.K8sApps) > 0 {
					common.FailWithMessage("该项目下已存在 "+strconv.Itoa(len(oldProj.K8sApps))+" 个关联应用/实例，为防止旧集群残留孤儿资源，禁止直接修改项目绑定的集群！请先清理下属实例后重试。", c)
					return
				}
			}
		}
	}

	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新K8s项目失败", zap.Error(err))
		common.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

// deleteK8sProject 删除K8s项目
// @Summary      删除K8s项目
// @Tags         k8s项目应用实例模块
// @Router       /k8s/deleteK8sProject/:id [delete]
// @Security     Bearer
func deleteK8sProject(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	idStr := c.Param("id")
	if idStr == "" {
		idStr = c.Query("id")
	}
	id, _ := strconv.Atoi(idStr)
	if id <= 0 {
		common.FailWithMessage("ID参数错误", c)
		return
	}

	proj, err := models.GetK8sProjectById(id)
	if err == nil && proj != nil {
		if len(proj.K8sApps) > 0 {
			common.FailWithMessage("该项目下包含 "+strconv.Itoa(len(proj.K8sApps))+" 个关联应用，请先删除关联应用与实例后再删除项目。", c)
			return
		}
	}

	err = models.DeleteK8sProjectById(id)
	if err != nil {
		sc.Logger.Error("删除K8s项目失败", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

// ==================== K8s App APIs ====================

// getK8sAppList 获取K8s应用列表
// @Summary      获取K8s应用列表
// @Tags         k8s项目应用实例模块
// @Router       /k8s/getK8sAppList [get]
// @Security     Bearer
func getK8sAppList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")
	searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchTitle := c.DefaultQuery("name", "")
	searchCreateUserName := c.DefaultQuery("createUserName", "")
	searchProjectId := c.DefaultQuery("k8sProjectId", "")
	searchProjectIdInt, _ := strconv.Atoi(searchProjectId)

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	objs, err := models.GetK8sAppAll()
	if err != nil {
		sc.Logger.Error("查询所有K8s应用错误", zap.Error(err))
		common.ReqBadFailWithMessage("查询K8s应用列表失败: "+err.Error(), c)
		return
	}

	var allIds []int
	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}
		if searchProjectId != "" && int(obj.K8sProjectId) != searchProjectIdInt {
			continue
		}
		if searchTitle != "" && !strings.Contains(obj.Name, searchTitle) {
			continue
		}

		_ = obj.FillFrontAllData()
		if searchCreateUserName != "" && !strings.Contains(obj.CreateUserName, searchCreateUserName) {
			continue
		}
		allIds = append(allIds, int(obj.ID))
	}

	if len(allIds) == 0 {
		common.OkWithDetailed(gin.H{
			"items": []models.K8sApp{},
			"total": 0,
		}, "ok", c)
		return
	}

	pagedObjs, err := models.GetK8sAppByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("分页查询K8s应用失败", zap.Error(err))
		common.ReqBadFailWithMessage("分页查询K8s应用失败: "+err.Error(), c)
		return
	}

	for _, obj := range pagedObjs {
		_ = obj.FillFrontAllData()
	}

	common.OkWithDetailed(gin.H{
		"items": pagedObjs,
		"total": len(allIds),
	}, "ok", c)
}

// getK8sAppOne 获取单个K8s应用详情
// @Summary      获取单个K8s应用详情
// @Tags         k8s项目应用实例模块
// @Router       /k8s/getK8sAppOne/:id [get]
// @Security     Bearer
func getK8sAppOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	idStr := c.Param("id")
	if idStr == "" {
		idStr = c.Query("id")
	}
	id, _ := strconv.Atoi(idStr)
	if id <= 0 {
		common.FailWithMessage("ID参数错误", c)
		return
	}

	obj, err := models.GetK8sAppById(id)
	if err != nil {
		sc.Logger.Error("获取K8s应用失败", zap.Error(err))
		common.FailWithMessage("获取K8s应用失败: "+err.Error(), c)
		return
	}
	_ = obj.FillFrontAllData()

	common.OkWithData(obj, c)
}

// createK8sApp 创建K8s应用
// @Summary      创建K8s应用
// @Tags         k8s项目应用实例模块
// @Router       /k8s/createK8sApp [post]
// @Security     Bearer
func createK8sApp(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.K8sApp
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增K8s应用请求失败", zap.Error(err))
		common.FailWithMessage("请求参数格式错误: "+err.Error(), c)
		return
	}

	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, _ := models.GetUserByUsername(userName)
	if dbUser != nil {
		reqObj.UserID = dbUser.ID
	}

	if len(reqObj.VolumeJsonFront) > 0 {
		bs, _ := json.Marshal(reqObj.VolumeJsonFront)
		reqObj.VolumeJson = string(bs)
	}
	if len(reqObj.PortJsonFront) > 0 {
		bs, _ := json.Marshal(reqObj.PortJsonFront)
		reqObj.PortJson = string(bs)
	}

	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增K8s应用到数据库失败", zap.Error(err))
		common.FailWithMessage("创建失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

// updateK8sApp 更新K8s应用
// @Summary      更新K8s应用
// @Tags         k8s项目应用实例模块
// @Router       /k8s/updateK8sApp [post]
// @Security     Bearer
func updateK8sApp(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.K8sApp
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新K8s应用请求失败", zap.Error(err))
		common.FailWithMessage("请求参数格式错误: "+err.Error(), c)
		return
	}

	if len(reqObj.VolumeJsonFront) > 0 {
		bs, _ := json.Marshal(reqObj.VolumeJsonFront)
		reqObj.VolumeJson = string(bs)
	}
	if len(reqObj.PortJsonFront) > 0 {
		bs, _ := json.Marshal(reqObj.PortJsonFront)
		reqObj.PortJson = string(bs)
	}

	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新K8s应用失败", zap.Error(err))
		common.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

// deleteK8sApp 删除K8s应用
// @Summary      删除K8s应用
// @Tags         k8s项目应用实例模块
// @Router       /k8s/deleteK8sApp/:id [delete]
// @Security     Bearer
func deleteK8sApp(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	idStr := c.Param("id")
	if idStr == "" {
		idStr = c.Query("id")
	}
	id, _ := strconv.Atoi(idStr)
	if id <= 0 {
		common.FailWithMessage("ID参数错误", c)
		return
	}

	err := models.DeleteK8sAppById(id)
	if err != nil {
		sc.Logger.Error("删除K8s应用失败", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

// ==================== K8s Instance APIs ====================

// getK8sInstanceList 获取K8s实例列表
// @Summary      获取K8s实例列表
// @Tags         k8s项目应用实例模块
// @Router       /k8s/getK8sInstanceList [get]
// @Security     Bearer
func getK8sInstanceList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")
	searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchTitle := c.DefaultQuery("name", "")
	searchCreateUserName := c.DefaultQuery("createUserName", "")
	searchAppId := c.DefaultQuery("k8sAppId", "")
	searchAppIdInt, _ := strconv.Atoi(searchAppId)

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	objs, err := models.GetK8sInstanceAll()
	if err != nil {
		sc.Logger.Error("查询所有K8s实例错误", zap.Error(err))
		common.ReqBadFailWithMessage("查询K8s实例列表失败: "+err.Error(), c)
		return
	}

	var allIds []int
	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}
		if searchAppId != "" && int(obj.K8sAppId) != searchAppIdInt {
			continue
		}
		if searchTitle != "" && !strings.Contains(obj.Name, searchTitle) {
			continue
		}

		obj.FillFrontAllData()
		if searchCreateUserName != "" && !strings.Contains(obj.CreateUserName, searchCreateUserName) {
			continue
		}
		allIds = append(allIds, int(obj.ID))
	}

	if len(allIds) == 0 {
		common.OkWithDetailed(gin.H{
			"items": []models.K8sInstance{},
			"total": 0,
		}, "ok", c)
		return
	}

	pagedObjs, err := models.GetK8sInstanceByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("分页查询K8s实例失败", zap.Error(err))
		common.ReqBadFailWithMessage("分页查询K8s实例失败: "+err.Error(), c)
		return
	}

	for _, obj := range pagedObjs {
		obj.FillFrontAllData()
		if obj.K8sAppObj != nil && obj.K8sAppObj.K8sProjectId > 0 {
			project, err := models.GetK8sProjectById(int(obj.K8sAppObj.K8sProjectId))
			if err == nil && project != nil && project.Cluster != "" {
				kSet, _, dbCluster, err := getClusterClientsetHelper(c, project.Cluster)
				if err == nil && kSet != nil {
					ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
					dep, err := kSet.AppsV1().Deployments(obj.K8sAppObj.Namespace).Get(ctx, obj.Name, metav1.GetOptions{})
					cancel()
					if err == nil && dep != nil {
						obj.ReadyReplicas = dep.Status.ReadyReplicas
						obj.ClusterStatus = fmt.Sprintf("%d/%d Ready", dep.Status.ReadyReplicas, dep.Status.Replicas)
					} else {
						obj.ClusterStatus = "未部署/同步"
					}
				}
			}
		}
	}

	common.OkWithDetailed(gin.H{
		"items": pagedObjs,
		"total": len(allIds),
	}, "ok", c)
}

// getK8sInstanceOne 获取单个K8s实例详情
// @Summary      获取单个K8s实例详情
// @Tags         k8s项目应用实例模块
// @Router       /k8s/getK8sInstanceOne/:id [get]
// @Security     Bearer
func getK8sInstanceOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	idStr := c.Param("id")
	if idStr == "" {
		idStr = c.Query("id")
	}
	id, _ := strconv.Atoi(idStr)
	if id <= 0 {
		common.FailWithMessage("ID参数错误", c)
		return
	}

	obj, err := models.GetK8sInstanceById(id)
	if err != nil {
		sc.Logger.Error("获取K8s实例失败", zap.Error(err))
		common.FailWithMessage("获取K8s实例失败: "+err.Error(), c)
		return
	}
	obj.FillFrontAllData()

	common.OkWithData(obj, c)
}

// createK8sInstance 创建K8s实例
// @Summary      创建K8s实例
// @Tags         k8s项目应用实例模块
// @Router       /k8s/createK8sInstance [post]
// @Security     Bearer
func createK8sInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.K8sInstance
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增K8s实例请求失败", zap.Error(err))
		common.FailWithMessage("请求参数格式错误: "+err.Error(), c)
		return
	}

	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, _ := models.GetUserByUsername(userName)
	if dbUser != nil {
		reqObj.UserID = dbUser.ID
	}

	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增K8s实例到数据库失败", zap.Error(err))
		common.FailWithMessage("创建失败: "+err.Error(), c)
		return
	}

	// 尝试同步到集群 Deployment
	_ = syncK8sDeployment(c, &reqObj)

	common.OkWithMessage("创建成功", c)
}

// updateK8sInstance 更新K8s实例
// @Summary      更新K8s实例
// @Tags         k8s项目应用实例模块
// @Router       /k8s/updateK8sInstance [post]
// @Security     Bearer
func updateK8sInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.K8sInstance
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新K8s实例请求失败", zap.Error(err))
		common.FailWithMessage("请求参数格式错误: "+err.Error(), c)
		return
	}

	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新K8s实例失败", zap.Error(err))
		common.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}

	// 尝试同步到集群 Deployment
	_ = syncK8sDeployment(c, &reqObj)

	common.OkWithMessage("更新成功", c)
}

// deleteK8sInstance 删除K8s实例
// @Summary      删除K8s实例
// @Tags         k8s项目应用实例模块
// @Router       /k8s/deleteK8sInstance/:id [delete]
// @Security     Bearer
func deleteK8sInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	idStr := c.Param("id")
	if idStr == "" {
		idStr = c.Query("id")
	}
	id, _ := strconv.Atoi(idStr)
	if id <= 0 {
		common.FailWithMessage("ID参数错误", c)
		return
	}

	obj, err := models.GetK8sInstanceById(id)
	if err == nil && obj != nil {
		deleteK8sResources(c, obj)
	}

	err = models.DeleteK8sInstanceById(id)
	if err != nil {
		sc.Logger.Error("删除K8s实例失败", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

// deployK8sInstance 手动触发部署K8s实例到集群
// @Summary      部署K8s实例到集群
// @Tags         k8s项目应用实例模块
// @Router       /k8s/deployK8sInstance/:id [post]
// @Security     Bearer
func deployK8sInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	idStr := c.Param("id")
	if idStr == "" {
		idStr = c.Query("id")
	}
	id, _ := strconv.Atoi(idStr)
	if id <= 0 {
		common.FailWithMessage("ID参数错误", c)
		return
	}

	obj, err := models.GetK8sInstanceById(id)
	if err != nil {
		common.FailWithMessage("获取实例详情失败: "+err.Error(), c)
		return
	}

	err = syncK8sDeployment(c, obj)
	if err != nil {
		sc.Logger.Error("部署实例到K8s集群失败", zap.Error(err))
		common.FailWithMessage("部署失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("部署成功", c)
}

// Helper: 同步 Instance 到 K8s 工作负载控制器 (Deployment/StatefulSet/DaemonSet/Pod) 及 Service/Ingress
func syncK8sDeployment(c *gin.Context, obj *models.K8sInstance) error {
	obj.FillFrontAllData()
	if obj.K8sAppObj == nil {
		return fmt.Errorf("实例未绑定关联的应用")
	}
	if obj.K8sAppObj.K8sProjectId <= 0 {
		return fmt.Errorf("应用未绑定关联的项目")
	}

	project, err := models.GetK8sProjectById(int(obj.K8sAppObj.K8sProjectId))
	if err != nil || project == nil || project.Cluster == "" {
		return fmt.Errorf("项目未配置绑定K8s集群")
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, project.Cluster)
	if err != nil || kSet == nil {
		return fmt.Errorf("获取集群Clientset失败: %v", err)
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	workloadType := obj.WorkloadType
	if workloadType == "" {
		workloadType = "Deployment"
	}

	switch workloadType {
	case "StatefulSet":
		sts, err := obj.GetK8sStatefulSet()
		if err != nil {
			return fmt.Errorf("生成StatefulSet结构失败: %v", err)
		}
		existing, err := kSet.AppsV1().StatefulSets(sts.Namespace).Get(ctx, sts.Name, metav1.GetOptions{})
		if err != nil || existing == nil {
			_, err = kSet.AppsV1().StatefulSets(sts.Namespace).Create(ctx, sts, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("创建StatefulSet失败: %v", err)
			}
		} else {
			sts.ResourceVersion = existing.ResourceVersion
			_, err = kSet.AppsV1().StatefulSets(sts.Namespace).Update(ctx, sts, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("更新StatefulSet失败: %v", err)
			}
		}

	case "DaemonSet":
		ds, err := obj.GetK8sDaemonSet()
		if err != nil {
			return fmt.Errorf("生成DaemonSet结构失败: %v", err)
		}
		existing, err := kSet.AppsV1().DaemonSets(ds.Namespace).Get(ctx, ds.Name, metav1.GetOptions{})
		if err != nil || existing == nil {
			_, err = kSet.AppsV1().DaemonSets(ds.Namespace).Create(ctx, ds, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("创建DaemonSet失败: %v", err)
			}
		} else {
			ds.ResourceVersion = existing.ResourceVersion
			_, err = kSet.AppsV1().DaemonSets(ds.Namespace).Update(ctx, ds, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("更新DaemonSet失败: %v", err)
			}
		}

	case "Pod":
		pod, err := obj.GetK8sStandalonePod()
		if err != nil {
			return fmt.Errorf("生成Standalone Pod结构失败: %v", err)
		}
		existing, err := kSet.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil || existing == nil {
			_, err = kSet.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("创建Standalone Pod失败: %v", err)
			}
		}

	default: // Deployment
		dep, err := obj.GetK8sDeployment()
		if err != nil {
			return fmt.Errorf("生成Deployment结构失败: %v", err)
		}
		existing, err := kSet.AppsV1().Deployments(dep.Namespace).Get(ctx, dep.Name, metav1.GetOptions{})
		if err != nil || existing == nil {
			_, err = kSet.AppsV1().Deployments(dep.Namespace).Create(ctx, dep, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("创建K8s Deployment失败: %v", err)
			}
		} else {
			dep.ResourceVersion = existing.ResourceVersion
			_, err = kSet.AppsV1().Deployments(dep.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("更新K8s Deployment失败: %v", err)
			}
		}
	}

	// 2. 如果开启了 Service 关联，自动创建/更新 Service
	if obj.EnableSvc {
		svc, err := obj.GetK8sService()
		if err == nil && svc != nil {
			existingSvc, err := kSet.CoreV1().Services(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
			if err != nil || existingSvc == nil {
				_, _ = kSet.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
			} else {
				svc.ResourceVersion = existingSvc.ResourceVersion
				svc.Spec.ClusterIP = existingSvc.Spec.ClusterIP
				_, _ = kSet.CoreV1().Services(svc.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
			}
		}
	}

	// 3. 如果开启了 Ingress 域名关联，自动创建/更新 Ingress
	if obj.EnableIngress && obj.IngressHost != "" {
		ing, err := obj.GetK8sIngress()
		if err == nil && ing != nil {
			existingIng, err := kSet.NetworkingV1().Ingresses(ing.Namespace).Get(ctx, ing.Name, metav1.GetOptions{})
			if err != nil || existingIng == nil {
				_, _ = kSet.NetworkingV1().Ingresses(ing.Namespace).Create(ctx, ing, metav1.CreateOptions{})
			} else {
				ing.ResourceVersion = existingIng.ResourceVersion
				_, _ = kSet.NetworkingV1().Ingresses(ing.Namespace).Update(ctx, ing, metav1.UpdateOptions{})
			}
		}
	}

	return nil
}

// Helper: 从 K8s 集群中连带干净清理 Workload Controller (Deployment/StatefulSet/DaemonSet/Pod)、Service 与 Ingress
func deleteK8sResources(c *gin.Context, obj *models.K8sInstance) {
	obj.FillFrontAllData()
	if obj.K8sAppObj == nil || obj.K8sAppObj.K8sProjectId <= 0 {
		return
	}
	project, err := models.GetK8sProjectById(int(obj.K8sAppObj.K8sProjectId))
	if err != nil || project == nil || project.Cluster == "" {
		return
	}

	kSet, _, dbCluster, err := getClusterClientsetHelper(c, project.Cluster)
	if err != nil || kSet == nil {
		return
	}

	ctx, cancel := common.GenTimeoutContext(dbCluster.ActionTimeoutSeconds)
	defer cancel()

	ns := obj.K8sAppObj.Namespace
	fullName := obj.GetK8sResourceName()
	shortName := obj.Name

	// 核心亮点：使用 DeletePropagationBackground 告诉 K8s 级联清理 Deployment/StatefulSet/DaemonSet 及其下属 ReplicaSet 和 Pods
	propagationPolicy := metav1.DeletePropagationBackground
	delOpts := metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}

	// 兼容支持清理 fullName (例: order-service-ins-prod-01) 与 shortName (例: ins-prod-01)
	for _, name := range []string{fullName, shortName} {
		if name == "" {
			continue
		}
		_ = kSet.AppsV1().Deployments(ns).Delete(ctx, name, delOpts)
		_ = kSet.AppsV1().StatefulSets(ns).Delete(ctx, name, delOpts)
		_ = kSet.AppsV1().DaemonSets(ns).Delete(ctx, name, delOpts)
		_ = kSet.CoreV1().Pods(ns).Delete(ctx, name, delOpts)

		_ = kSet.CoreV1().Services(ns).Delete(ctx, name, delOpts)
		_ = kSet.NetworkingV1().Ingresses(ns).Delete(ctx, name, delOpts)
	}
}

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
)

func getK8sClusterList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")

	searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchTitle := c.DefaultQuery("name", "")
	searchCreateUserName := c.DefaultQuery("createUserName", "")

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}
	objs, err := models.GetK8sClusterAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的k8s集群配置执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的k8s集群配置执行错误：%v", err.Error()), c)
		return
	}
	allIds := []int{}
	kc := c.MustGet(common.GIN_CTX_K8S_CACHE).(*cache.K8sClusterCache)
	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}

		if searchTitle != "" && !strings.Contains(obj.Name, searchTitle) {
			continue
		}
		obj.LastProbSuccess = kc.GetClusterProbeResultById(obj.ID)
		//sc.Logger.Info("[k8s模块] 获取集群探活状态", zap.Any("obj.ID", obj.ID), zap.Any("LastProbSuccess", obj.LastProbSuccess), zap.Any("cachedAliveMap", kc.KubeClientsAlive))
		// 填充前端需要的数据（拿到组合好的 CreateUserName）
		obj.FillFrontAllData()

		// 🚀 修复 1：对比的应该是刚填充好的 CreateUserName 字段，而不是 UserID
		if searchCreateUserName != "" && !strings.Contains(obj.CreateUserName, searchCreateUserName) {
			continue
		}

		allIds = append(allIds, int(obj.ID))
	}

	// 如果过滤后没有数据，直接返回空列表
	if len(allIds) == 0 {
		common.OkWithDetailed(gin.H{
			"items": []models.K8sCluster{},
			"total": 0,
		}, "ok", c)
		return
	}

	// 根据过滤后的 ID 进行分页查询
	pagedObjs, err := models.GetK8sClusterByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的k8s集群配置执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的k8s集群配置执行错误：%v", err.Error()), c)
		return
	}

	// 🚀 修复 2：分页查出来的新对象，必须再次遍历填充一次虚拟字段，否则响应里还是空的！
	for _, obj := range pagedObjs {
		obj.LastProbSuccess = kc.GetClusterProbeResultById(obj.ID)
		obj.FillFrontAllData()
	}

	common.OkWithDetailed(gin.H{
		"items": pagedObjs,
		"total": len(allIds),
	}, "ok", c)
}

func createK8sCluster(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj models.K8sCluster
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增k8s集群配置执行请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 获取当前用户ID
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)

	dbUser, err := models.GetUserByUsername(userName)

	if dbUser != nil {
		reqObj.UserID = dbUser.ID
	}

	err = reqObj.FillDefaultData()
	if err != nil {
		sc.Logger.Error("获取集群版本错误", zap.Error(err))
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	// 解析kubeconfig

	// 存入数据库
	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增k8s集群配置执行数据库失败", zap.Error(err))
		common.FailWithMessage("存入数据库失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

func updateK8sCluster(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	// 🚀 致命修复：同上
	var reqObj models.K8sCluster
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新k8s集群配置请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 检查是否存在
	dbOld, err := models.GetK8sClusterById(int(reqObj.ID))
	if err != nil {
		common.FailWithMessage("k8s集群配置不存在", c)
		return
	}
	reqObj.UserID = dbOld.UserID
	err = reqObj.FillDefaultData()
	if err != nil {
		sc.Logger.Error("更新集群配置解析错误", zap.Error(err))
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新k8s集群配置执行错误", zap.Error(err))
		common.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

func deleteK8sCluster(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetK8sClusterById(intVar)
	if err != nil {
		common.FailWithMessage("k8s集群配置不存在", c)
		return
	}

	err = dbObj.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除k8s集群配置执行错误", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

// 1. 定义批量删除的请求体
type deleteK8sClusterBatchReq struct {
	Ids []uint `json:"ids" validate:"required,min=1"` // 要求至少传 1 个 ID
}

// 2. 批量删除的处理函数
func deleteK8sClusterBatch(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj deleteK8sClusterBatchReq
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析批量删除k8s集群请求失败", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// （可选）如果你需要像单条删除那样检查依赖关系，可以在这里写个 for 循环检查 reqObj.Ids
	// 为了极致性能，这里直接调用批量删除
	err = models.DeleteK8sClusterBatch(reqObj.Ids)
	if err != nil {
		sc.Logger.Error("批量删除k8s集群执行错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage("批量删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage(fmt.Sprintf("成功删除了 %d 条k8s集群", len(reqObj.Ids)), c)
}

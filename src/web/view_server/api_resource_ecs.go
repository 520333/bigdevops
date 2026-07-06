package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

func getResourceEcsUnbindList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	allEcs, err := models.GetResourceEcsAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的ecs错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的ecs错误：%v", err.Error()), c)
		return
	}

	finnalList := []*models.ResourceEcs{}
	for _, ecs := range allEcs {
		ecs := ecs
		if len(ecs.BindNodes) > 0 {
			continue
		}
		finnalList = append(finnalList, ecs)
	}
	common.OkWithDetailed(finnalList, "ok", c)
}

func bindEcsToStreeNode(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqBind models.BindResourceToStreeNodeRequest
	err := c.ShouldBindJSON(&reqBind)
	if err != nil {
		sc.Logger.Error("解析节点绑定ecs请求失败", zap.Any("reqBind", reqBind), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqBind)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}
	// 根据 nodeId找到Node
	dbNode, err := models.GetStreeNodeById(reqBind.NodeId)
	if err != nil {
		sc.Logger.Error("根据id找树节点错误", zap.Any("树节点", reqBind.NodeId), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 遍历 ecs id 数组找到ecs对象
	for _, ridStr := range reqBind.ResourceIds {
		rid, _ := strconv.Atoi(ridStr)
		dbResource, err := models.GetResourceEcsById(rid)

		if err != nil {
			sc.Logger.Error("根据id找树节点错误", zap.Any("ecs", rid), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		thisNode := []*models.StreeNode{}
		tmpM := map[uint]*models.StreeNode{}
		for _, node := range dbResource.BindNodes {
			node := node
			tmpM[node.ID] = node

		}
		tmpM[dbNode.ID] = dbNode
		for _, node := range tmpM {
			node := node
			thisNode = append(thisNode, node)

		}
		// 更新资源的bindnode
		err = dbResource.UpdateBindNodes(thisNode)
		if err != nil {
			sc.Logger.Error("更新ecs绑定节点错误",
				zap.Any("ecs", rid),
				zap.Any("nodes", thisNode),
				zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}

	}
	common.OkWithMessage("更新成功", c)
}

func unBindEcsToStreeNode(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqBind models.BindResourceToStreeNodeRequest
	err := c.ShouldBindJSON(&reqBind)
	if err != nil {
		sc.Logger.Error("解析节点解绑ecs请求失败", zap.Any("reqBind", reqBind), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqBind)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}
	// 根据 nodeId找到Node
	dbNode, err := models.GetStreeNodeById(reqBind.NodeId)
	if err != nil {
		sc.Logger.Error("根据id找树节点错误", zap.Any("树节点", reqBind.NodeId), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 遍历 ecs id 数组找到ecs对象
	for _, ridStr := range reqBind.ResourceIds {
		rid, _ := strconv.Atoi(ridStr)
		dbResource, err := models.GetResourceEcsById(rid)

		if err != nil {
			sc.Logger.Error("根据id找树节点错误", zap.Any("ecs", rid), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		thisNode := []*models.StreeNode{}
		tmpM := map[uint]*models.StreeNode{}
		for _, node := range dbResource.BindNodes {
			node := node
			tmpM[node.ID] = node

		}
		delete(tmpM, dbNode.ID)
		for _, node := range tmpM {
			node := node
			thisNode = append(thisNode, node)

		}
		// 更新资源的bindnode
		err = dbResource.UpdateBindNodes(thisNode)
		if err != nil {
			sc.Logger.Error("更新ecs绑定节点错误",
				zap.Any("ecs", rid),
				zap.Any("nodes", thisNode),
				zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}

	}
	common.OkWithMessage("解绑成功", c)
}

// 根据树节点 ID，拉取当前节点及所有子节点下绑定的完整 ECS 列表
func getStreeNodeEcsList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id, _ := strconv.Atoi(c.Param("id"))

	// 1. 查找当前节点
	node, err := models.GetStreeNodeById(id)
	if err != nil {
		sc.Logger.Error("根据id找树节点错误", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 2. 收集当前节点以及递归获取所有子孙节点
	allNodes := []*models.StreeNode{node}
	children, err := models.GetAllLeafNodes(int(node.ID))
	if err == nil && children != nil {
		allNodes = append(allNodes, children...)
	}

	// 3. 对所有绑定的 ECS 资源进行去重过滤
	allEcsMap := map[uint]*models.ResourceEcs{}
	for _, n := range allNodes {
		if n.BindEcss == nil {
			continue
		}
		for _, ecs := range n.BindEcss {
			allEcsMap[ecs.ID] = ecs
		}
	}

	// 4. 装载成切片返回
	ecsList := []*models.ResourceEcs{}
	for _, ecs := range allEcsMap {
		ecsList = append(ecsList, ecs)
	}

	common.OkWithDetailed(ecsList, "ok", c)
}

func getResourceEcsList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "500"))

	searchInstance := c.DefaultQuery("instance", "")

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	objs, err := models.GetResourceEcsAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的机器执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的机器执行错误：%v", err.Error()), c)
		return
	}
	allIds := []int{}

	for _, obj := range objs {
		obj := obj
		if searchInstance != "" && obj.InstanceName != searchInstance {
			continue
		}
		obj.FillFrontAllData()

		allIds = append(allIds, int(obj.ID))
	}

	// 如果过滤后没有数据，直接返回空列表
	if len(allIds) == 0 {
		common.OkWithDetailed(gin.H{
			"items": []models.ResourceEcs{},
			"total": 0,
		}, "ok", c)
		return
	}

	// 根据过滤后的 ID 进行分页查询
	pagedObjs, err := models.GetResourceEcsByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的采集池执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的采集池执行错误：%v", err.Error()), c)
		return
	}

	// 🚀 修复 2：分页查出来的新对象，必须再次遍历填充一次虚拟字段，否则响应里还是空的！
	for _, obj := range pagedObjs {
		obj.FillFrontAllData()
	}

	common.OkWithDetailed(gin.H{
		"items": pagedObjs,
		"total": len(allIds),
	}, "ok", c)
}

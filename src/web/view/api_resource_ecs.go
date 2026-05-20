package view

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

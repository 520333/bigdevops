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

// getResourceRdsUnbindList 获取所有 RDS 列表 (供前端穿梭框或绑定弹窗使用)
func getResourceRdsUnbindList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	allRds, err := models.GetResourceRdsAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的Rds错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的Rds错误：%v", err.Error()), c)
		return
	}

	// 与 ELB 保持一致，这里直接返回全部数据，前端可根据 bindNodes 长度过滤，也可在此处过滤
	common.OkWithDetailed(allRds, "ok", c)
}

// bindRdsToStreeNode 将 RDS 绑定到服务树节点
func bindRdsToStreeNode(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqBind models.BindResourceToStreeNodeRequest
	err := c.ShouldBindJSON(&reqBind)
	if err != nil {
		sc.Logger.Error("解析节点绑定rds请求失败", zap.Any("reqBind", reqBind), zap.Error(err))
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

	// 1. 根据 nodeId 找到 Node
	dbNode, err := models.GetStreeNodeById(reqBind.NodeId)
	if err != nil {
		sc.Logger.Error("根据id找树节点错误", zap.Any("树节点", reqBind.NodeId), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 2. 遍历 Rds id 数组找到 Rds 对象并更新绑定关系
	for _, ridStr := range reqBind.ResourceIds {
		rid, _ := strconv.Atoi(ridStr)
		// 注意：根据你之前 tbl_resource_rds.go 的定义，GetResourceRdsById 的参数可能是 string 也可能是 int。这里参考 ELB 传入 string
		dbResource, err := models.GetResourceRdsById(strconv.Itoa(rid))

		if err != nil {
			sc.Logger.Error("根据id找Rds资源错误", zap.Any("rds", rid), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		thisNode := []*models.StreeNode{}
		tmpM := map[uint]*models.StreeNode{}

		// 获取资源原有的绑定节点
		for _, node := range dbResource.BindNodes {
			node := node
			tmpM[node.ID] = node
		}

		// 加上当前要绑定的节点 (利用 map 去重)
		tmpM[dbNode.ID] = dbNode
		for _, node := range tmpM {
			node := node
			thisNode = append(thisNode, node)
		}

		// 更新资源的 bindnode
		err = dbResource.UpdateBindNodes(thisNode)
		if err != nil {
			sc.Logger.Error("更新rds绑定节点错误", zap.Any("rds", rid), zap.Any("nodes", thisNode), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
	}
	common.OkWithMessage("更新成功", c)
}

// unBindRdsToStreeNode 解除 RDS 与服务树节点的绑定
func unBindRdsToStreeNode(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqBind models.BindResourceToStreeNodeRequest
	err := c.ShouldBindJSON(&reqBind)
	if err != nil {
		sc.Logger.Error("解析节点解绑Rds请求失败", zap.Any("reqBind", reqBind), zap.Error(err))
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

	dbNode, err := models.GetStreeNodeById(reqBind.NodeId)
	if err != nil {
		sc.Logger.Error("根据id找树节点错误", zap.Any("树节点", reqBind.NodeId), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	for _, ridStr := range reqBind.ResourceIds {
		rid, _ := strconv.Atoi(ridStr)
		dbResource, err := models.GetResourceRdsById(strconv.Itoa(rid))

		if err != nil {
			sc.Logger.Error("根据id找Rds资源错误", zap.Any("RDS", rid), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		thisNode := []*models.StreeNode{}
		tmpM := map[uint]*models.StreeNode{}
		for _, node := range dbResource.BindNodes {
			node := node
			tmpM[node.ID] = node
		}

		// 核心：从 map 中删除当前的节点 ID
		delete(tmpM, dbNode.ID)

		for _, node := range tmpM {
			node := node
			thisNode = append(thisNode, node)
		}

		// 更新资源的 bindnode
		err = dbResource.UpdateBindNodes(thisNode)
		if err != nil {
			sc.Logger.Error("更新Rds绑定节点错误", zap.Any("RDS", rid), zap.Any("nodes", thisNode), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
	}
	common.OkWithMessage("解绑成功", c)
}

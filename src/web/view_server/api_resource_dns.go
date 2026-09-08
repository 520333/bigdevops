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

// @Summary      获取未绑定的DNS域名列表
// @Description  获取未绑定的DNS域名列表 接口
// @Tags         resource-dns
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取未绑定的DNS域名列表 响应结果"
// @Router       /stree/getResourceDnsUnbindList [get]
// @Security     Bearer
func getResourceDnsUnbindList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	allDns, err := models.GetResourceDnsAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的DNS错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的DNS错误：%v", err.Error()), c)
		return
	}
	common.OkWithDetailed(allDns, "ok", c)
}

// @Summary      绑定DNS到服务树节点
// @Description  绑定DNS到服务树节点 接口
// @Tags         resource-dns
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "绑定DNS到服务树节点 响应结果"
// @Router       /stree/bindDnsToStreeNode [post]
// @Security     Bearer
func bindDnsToStreeNode(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqBind models.BindResourceToStreeNodeRequest
	err := c.ShouldBindJSON(&reqBind)
	if err != nil {
		sc.Logger.Error("解析节点绑定DNS请求失败", zap.Any("reqBind", reqBind), zap.Error(err))
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
	// 根据 nodeId 找到 Node
	dbNode, err := models.GetStreeNodeById(reqBind.NodeId)
	if err != nil {
		sc.Logger.Error("根据id找树节点错误", zap.Any("树节点", reqBind.NodeId), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 遍历 DNS id 数组找到 DNS 对象并绑定
	for _, ridStr := range reqBind.ResourceIds {
		rid, _ := strconv.Atoi(ridStr)
		dbResource, err := models.GetResourceDnsById(strconv.Itoa(rid))
		if err != nil {
			sc.Logger.Error("根据id找DNS记录错误", zap.Any("dns", rid), zap.Error(err))
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
		err = dbResource.UpdateBindNodes(thisNode)
		if err != nil {
			sc.Logger.Error("更新DNS绑定节点错误",
				zap.Any("dns", rid),
				zap.Any("nodes", thisNode),
				zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
	}
	common.OkWithMessage("更新成功", c)
}

// @Summary      解绑DNS与服务树关系
// @Description  解绑DNS与服务树关系 接口
// @Tags         resource-dns
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "解绑DNS与服务树关系 响应结果"
// @Router       /stree/unBindDnsToStreeNode [post]
// @Security     Bearer
func unBindDnsToStreeNode(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqBind models.BindResourceToStreeNodeRequest
	err := c.ShouldBindJSON(&reqBind)
	if err != nil {
		sc.Logger.Error("解析节点解绑DNS请求失败", zap.Any("reqBind", reqBind), zap.Error(err))
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
		dbResource, err := models.GetResourceDnsById(strconv.Itoa(rid))
		if err != nil {
			sc.Logger.Error("根据id找DNS记录错误", zap.Any("DNS", rid), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		thisNode := []*models.StreeNode{}
		tmpM := map[uint]*models.StreeNode{}
		for _, node := range dbResource.BindNodes {
			node := node
			if node.ID != dbNode.ID {
				tmpM[node.ID] = node
			}
		}
		for _, node := range tmpM {
			node := node
			thisNode = append(thisNode, node)
		}
		err = dbResource.UpdateBindNodes(thisNode)
		if err != nil {
			sc.Logger.Error("更新DNS解绑节点错误",
				zap.Any("dns", rid),
				zap.Any("nodes", thisNode),
				zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
	}
	common.OkWithMessage("解绑成功", c)
}

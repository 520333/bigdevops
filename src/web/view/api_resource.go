package view

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"sort"
	"strings"

	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ResponseResourceCommon struct {
	Total int         `json:"total"`
	Items interface{} `json:"items"`
}

func fetchResourceByNode(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	nodeId := c.DefaultQuery("nodeId", "")
	resourceType := c.DefaultQuery("resourceType", "")
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchKeyword := c.DefaultQuery("keyword", "")
	searchStatus := c.DefaultQuery("status", "")
	searchVendor := c.DefaultQuery("vendor", "")
	searchAccount := c.DefaultQuery("account", "")

	offset := 0
	limit := 0
	limit = pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit

	}

	if nodeId == "" || resourceType == "" {
		common.FailWithMessage("nodeId or resourceType empty", c)
		return
	}

	var allNodes []*models.StreeNode
	inVar, _ := strconv.Atoi(nodeId)
	dbNode, err := models.GetStreeNodeById(inVar)
	if err != nil {
		sc.Logger.Error("根据id找子节点错误", zap.Any("树节点", nodeId), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	//childrens, err := models.GetStreeNodesByPId(inVar)
	childrens, err := models.GetAllLeafNodes(inVar)
	if err != nil {
		sc.Logger.Error("根据pid找子节点错误", zap.Any("树节点", nodeId), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	if dbNode != nil {
		allNodes = append(allNodes, dbNode)
	}
	allNodes = append(allNodes, childrens...)
	var objs interface{}
	var allResourceIds []int
	allResourceIdsMap := map[uint]struct{}{}
	resp := ResponseResourceCommon{}

	switch resourceType {
	case common.RESOURCE_TYPE_ECS:
		for _, node := range allNodes {
			node := node
			for _, obj := range node.BindEcss {
				obj := obj
				// 1. 厂商过滤
				if searchVendor != "" && obj.Vendor != searchVendor {
					continue
				}
				if searchAccount != "" {
					if !strings.Contains(strings.ToLower(obj.AccountName), strings.ToLower(searchAccount)) {
						continue
					}
				}
				// 2. 状态过滤
				if searchStatus != "" && strings.ToLower(obj.Status) != strings.ToLower(searchStatus) {
					continue
				}
				// 3. 关键字过滤
				if searchKeyword != "" {
					lowerKeyword := strings.ToLower(searchKeyword)

					matchInstanceName := strings.Contains(strings.ToLower(obj.InstanceName), lowerKeyword)
					matchHost := strings.Contains(strings.ToLower(obj.HostName), lowerKeyword)
					matchInstId := strings.Contains(strings.ToLower(obj.InstanceId), lowerKeyword)

					// 🌟 新增：检查私网 IP 数组
					matchPrivateIp := false
					for _, ip := range obj.PrivateIpAddress {
						if strings.Contains(strings.ToLower(ip), lowerKeyword) {
							matchPrivateIp = true
							break
						}
					}

					// 🌟 新增：检查公网 IP 数组
					matchPublicIp := false
					for _, ip := range obj.PublicIpAddresses {
						if strings.Contains(strings.ToLower(ip), lowerKeyword) {
							matchPublicIp = true
							break
						}
					}

					// 如果这 5 个字段都没有包含这个关键字，才跳过这台机器
					if !matchInstanceName && !matchHost && !matchInstId && !matchPrivateIp && !matchPublicIp {
						continue
					}
				}
				allResourceIdsMap[obj.ID] = struct{}{}
			}
		}
		for id := range allResourceIdsMap {
			allResourceIds = append(allResourceIds, int(id))
		}
		sort.Ints(allResourceIds)
		resp.Total = len(allResourceIds)
		// 如果过滤后没有任何数据，直接返回，不查数据库
		if resp.Total == 0 {
			resp.Items = []interface{}{}
			common.OkWithDetailed(resp, "ok", c)
			return
		}
		objs, err = models.GetResourceEcsByIdsWithLimitOffset(allResourceIds, limit, offset)

		if err != nil {
			sc.Logger.Error("根据id limit offset 出错", zap.Any("树节点", nodeId), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		resp.Items = objs
	}
	common.OkWithDetailed(resp, "ok", c)
}

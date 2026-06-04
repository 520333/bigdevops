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

	searchType := c.DefaultQuery("type", "")

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
	case common.RESOURCE_TYPE_ELB:
		for _, node := range allNodes {
			node := node
			// 遍历当前节点绑定的所有 ELB
			for _, obj := range node.BindElbs {
				obj := obj
				// 1. 厂商过滤
				if searchVendor != "" && obj.Vendor != searchVendor {
					continue
				}
				// 2. 账号过滤
				if searchAccount != "" {
					if !strings.Contains(strings.ToLower(obj.AccountName), strings.ToLower(searchAccount)) {
						continue
					}
				}
				// 3. 状态过滤
				if searchStatus != "" && strings.ToLower(obj.Status) != strings.ToLower(searchStatus) {
					continue
				}
				// 4. 关键字过滤
				if searchKeyword != "" {
					lowerKeyword := strings.ToLower(searchKeyword)

					matchName := strings.Contains(strings.ToLower(obj.LoadBalancerName), lowerKeyword)
					matchId := strings.Contains(strings.ToLower(obj.LoadBalancerId), lowerKeyword)
					matchDNS := strings.Contains(strings.ToLower(obj.DNSName), lowerKeyword)

					matchPublicIp := false
					for _, ip := range obj.PublicIpAddresses {
						if strings.Contains(strings.ToLower(ip), lowerKeyword) {
							matchPublicIp = true
							break
						}
					}

					if !matchName && !matchId && !matchDNS && !matchPublicIp {
						continue
					}
				}

				// 🌟 核心：使用 map 去重！因为一个 ELB 可能同时绑定在子节点和父节点上
				allResourceIdsMap[obj.ID] = struct{}{}
			}
		}

		// 将 map 中的 ID 转回 slice
		for id := range allResourceIdsMap {
			allResourceIds = append(allResourceIds, int(id))
		}
		// 排序保证每次请求列表顺序稳定
		sort.Ints(allResourceIds)
		resp.Total = len(allResourceIds)

		// 如果这个节点（以及子节点）下没有任何符合条件的 ELB，直接返回空数组
		if resp.Total == 0 {
			resp.Items = []interface{}{}
			common.OkWithDetailed(resp, "ok", c)
			return
		}

		// 最终：调用已有的方法，传入【所有相关 ELB 的 ID 集合】去查数据库并分页
		objs, err = models.GetResourceLbByIdsWithLimitOffset(allResourceIds, limit, offset)

		if err != nil {
			sc.Logger.Error("根据id limit offset 出错", zap.Any("树节点", nodeId), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		resp.Items = objs
	case common.RESOURCE_TYPE_RDS:
		for _, node := range allNodes {
			node := node
			for _, obj := range node.BindRds {
				obj := obj
				// 1. 厂商过滤
				if searchVendor != "" && obj.Vendor != searchVendor {
					continue
				}
				// 2. 账号过滤
				if searchAccount != "" {
					if !strings.Contains(strings.ToLower(obj.AccountName), strings.ToLower(searchAccount)) {
						continue
					}
				}
				// 3. 状态过滤
				if searchStatus != "" && strings.ToLower(obj.DBInstanceStatus) != strings.ToLower(searchStatus) {
					continue
				}
				// 4. 关键字过滤
				if searchKeyword != "" {
					lowerKeyword := strings.ToLower(searchKeyword)

					matchName := strings.Contains(strings.ToLower(obj.Name), lowerKeyword)
					matchId := strings.Contains(strings.ToLower(obj.DBInstanceId), lowerKeyword)
					matchHost := strings.Contains(strings.ToLower(obj.Host), lowerKeyword)

					if !matchName && !matchId && !matchHost {
						continue
					}
				}

				// 5. 核心：使用 map 去重，防止同一个实例挂在父子节点被重复统计
				allResourceIdsMap[obj.ID] = struct{}{}
			}
		}

		for id := range allResourceIdsMap {
			allResourceIds = append(allResourceIds, int(id))
		}

		sort.Ints(allResourceIds)
		resp.Total = len(allResourceIds)

		// 如果没有符合条件的数据，直接返回
		if resp.Total == 0 {
			resp.Items = []interface{}{}
			common.OkWithDetailed(resp, "ok", c)
			return
		}

		// 调用数据库方法获取详情并分页
		objs, err = models.GetResourceRdsByIdsWithLimitOffset(allResourceIds, limit, offset)

		if err != nil {
			sc.Logger.Error("根据id limit offset 查询RDS出错", zap.Any("树节点", nodeId), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		resp.Items = objs
	case common.RESOURCE_TYPE_DNS:
		var ecsInstanceIds []string
		var elbInstanceIds []string

		// 1. 遍历当前节点及所有子节点，提取所有的 ECS InstanceId 和 ELB LoadBalancerId
		for _, node := range allNodes {
			for _, ecs := range node.BindEcss {
				if ecs.InstanceId != "" {
					ecsInstanceIds = append(ecsInstanceIds, ecs.InstanceId)
				}
			}
			for _, elb := range node.BindElbs {
				if elb.LoadBalancerId != "" {
					elbInstanceIds = append(elbInstanceIds, elb.LoadBalancerId)
				}
			}
		}

		// 2. 如果节点下没有任何机器和 ELB，直接返回空
		if len(ecsInstanceIds) == 0 && len(elbInstanceIds) == 0 {
			resp.Total = 0
			resp.Items = []interface{}{}
			common.OkWithDetailed(resp, "ok", c)
			return
		}

		// 3. 构建 GORM 查询
		query := models.Db.Model(&models.ResourceDns{})

		// 查找绑定的记录：ECS 或 ELB 任意匹配一个即可
		query = query.Where("ecs_instance_id IN ? OR associated_instance_id IN ?", ecsInstanceIds, elbInstanceIds)

		// 4. 处理前端表单过滤
		if searchVendor != "" {
			query = query.Where("vendor = ?", searchVendor)
		}
		if searchType != "" {
			query = query.Where("type = ?", searchType)
		}
		if searchKeyword != "" {
			kw := "%" + searchKeyword + "%"
			query = query.Where("domain LIKE ? OR name LIKE ? OR value LIKE ?", kw, kw, kw)
		}

		// 5. 统计过滤后的总数
		var total int64
		if err := query.Count(&total).Error; err != nil {
			sc.Logger.Error("查询 DNS 总数出错", zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		resp.Total = int(total)

		if total == 0 {
			resp.Items = []interface{}{}
			common.OkWithDetailed(resp, "ok", c)
			return
		}

		// 6. 分页获取数据
		var dnsList []models.ResourceDns
		if err := query.Order("updated_at desc").Offset(offset).Limit(limit).Find(&dnsList).Error; err != nil {
			sc.Logger.Error("查询 DNS 列表出错", zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}

		resp.Items = dnsList
	}
	common.OkWithDetailed(resp, "ok", c)
}

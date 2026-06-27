package view

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"go.uber.org/zap"
)

type tmpNode struct {
	Title    string     `json:"title"`
	Key      string     `json:"key"`
	Children []*tmpNode `json:"children"`
	Pid      int        `json:"pId"`
	Level    int        `json:"level"`
}

func getStreeNodeListMock(c *gin.Context) {
	//sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	f1 := &tmpNode{
		Title:    "infra",
		Key:      "infra",
		Children: []*tmpNode{},
		Level:    1,
	}
	f2 := &tmpNode{
		Title:    "data",
		Key:      "data",
		Children: []*tmpNode{},
		Level:    1,
	}
	nodes := []*tmpNode{}
	for i := 0; i < 3; i++ {
		n1 := &tmpNode{
			Title:    fmt.Sprintf("infra-%d", i+1),
			Key:      fmt.Sprintf("infra-%d", i+1),
			Children: []*tmpNode{},
			Level:    2,
		}
		n2 := &tmpNode{
			Title:    fmt.Sprintf("data-%d", i+1),
			Key:      fmt.Sprintf("data-%d", i+1),
			Children: []*tmpNode{},
			Level:    2,
		}
		for j := 0; j < 3; j++ { // 💡 内层循环改用 j
			n11 := &tmpNode{
				// 💡 拼接外层的 i 和内层的 j，确保全局唯一 (如: infra-1-n111)
				Title:    fmt.Sprintf("infra-%d-n11%d", i+1, j+1),
				Key:      fmt.Sprintf("infra-%d-n11%d", i+1, j+1),
				Children: []*tmpNode{},
			}
			n22 := &tmpNode{
				// 💡 同理 (如: data-1-n211)
				Title:    fmt.Sprintf("data-%d-n21%d", i+1, j+1),
				Key:      fmt.Sprintf("data-%d-n21%d", i+1, j+1),
				Children: []*tmpNode{},
			}
			n1.Children = append(n1.Children, n11)
			n2.Children = append(n2.Children, n22)
		}
		f1.Children = append(f1.Children, n1)
		f2.Children = append(f2.Children, n2)
	}
	nodes = append(nodes, f1)
	nodes = append(nodes, f2)
	common.OkWithDetailed(nodes, "ok", c)

}

func getStreeNodeList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	streeNodes, err := models.GetStreeNodeAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的树节点错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的树节点错误：%v", err.Error()), c)
		return
	}
	// TODO 拼接children
	// 全员id map
	allMap := map[uint]*models.StreeNode{}
	topMap := map[uint]*models.StreeNode{}
	pidMap := map[uint][]*models.StreeNode{}
	//// 遍历第一轮
	for _, streeNode := range streeNodes {
		streeNode := streeNode
		allMap[streeNode.ID] = streeNode
		if streeNode.Pid == 0 {
			topMap[streeNode.ID] = streeNode
			continue
		}
		if streeNode.Level == 1 {
			topMap[streeNode.ID] = streeNode
		}
		// 同一层级的放到一起
		childs, ok := pidMap[streeNode.Pid]
		if !ok {
			childs = []*models.StreeNode{}
		}
		childs = append(childs, streeNode)
		pidMap[streeNode.Pid] = childs

	}

	// 再遍历allMap 回填children列表
	for id, node := range allMap {
		id := id
		node := node
		childs, ok := pidMap[id]
		if ok {
			node.Children = childs
		}

	}

	finalNodes := []*models.StreeNode{}
	// 遍历第一层级
	//for _, topNode := range topMap {
	//	//topPid := topPid
	//	topNode := topNode
	//	finalNodes = append(finalNodes, topNode)
	//
	//}

	for _, node := range streeNodes {
		if node.Pid == 0 || node.Level == 1 {
			finalNodes = append(finalNodes, node)
		}
	}

	// 遍历顶级节点

	common.OkWithDetailed(finalNodes, "ok", c)
}

func getStreeNodeSelect(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	streeNodes, err := models.GetStreeNodeAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的树节点错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的树节点错误：%v", err.Error()), c)
		return
	}
	for _, streeNode := range streeNodes {
		streeNode := streeNode
		streeNode.Key = streeNode.ID
	}
	common.OkWithDetailed(streeNodes, "ok", c)

}

// crud权限通用校验方法
func streeNodeOpsAdminPermissionCheck(node *models.StreeNode, c *gin.Context) (bool, error) {
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	reqUser, err := models.GetUserByUsername(userName)
	if err != nil {
		sc.Logger.Error("通过token解析到的userName去数据库中找User失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("通过token解析到的userName去数据库中找User失败%s", err.Error()), c)
		return false, err
	}

	isSuper := false
	for _, role := range reqUser.Roles {
		if role.RoleValue == sc.SuperRoleName {
			isSuper = true
			break
		}
	}
	if isSuper {
		return true, nil
	}

	pass := false

	for node != nil {

		for _, user := range node.OpsAdmins {
			user := user
			if user.Username == reqUser.Username {
				pass = true
				break
			}
		}
		if pass {
			break
		}
		father, _ := models.GetStreeNodeById(int(node.Pid))
		//if err != nil {
		//	return false, err
		//}
		node = father
	}
	sc.Logger.Info("校验服务树节点权限校验结果", zap.Bool("是否通过", pass),
		zap.String("用户", reqUser.Username),
		zap.Any("节点", node))
	return pass, nil

}

func createStreeNode(c *gin.Context) {
	// 校验StreeNode字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqNode models.StreeNode
	err := c.ShouldBindJSON(&reqNode)
	if err != nil {
		sc.Logger.Error("解析新增StreeNode请求失败", zap.Any("StreeNode", reqNode), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqNode)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	// 校验权限
	pass, err := streeNodeOpsAdminPermissionCheck(&reqNode, c)
	if err != nil {
		sc.Logger.Error("校验服务树节点权限校验失败", zap.Any("StreeNode", reqNode), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	if !pass {
		sc.Logger.Error("校验服务树节点权限校验失败未通过", zap.Any("StreeNode", reqNode), zap.Error(err))
		common.Req403WithMessage("服务树节点权限校验未通过", c)
		return
	}

	err = reqNode.CreateOne()
	if err != nil {
		sc.Logger.Error("创建StreeNode错误", zap.Any("StreeNode", reqNode), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("创建成功", c)
}

func deleteStreeNode(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("删除树节点", zap.Any("id", id))

	intVar, _ := strconv.Atoi(id)
	dbNode, err := models.GetStreeNodeById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找树节点错误", zap.Any("树节点", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	pass, err := streeNodeOpsAdminPermissionCheck(dbNode, c)
	if err != nil {
		sc.Logger.Error("校验服务树节点权限校验失败", zap.Any("StreeNode", dbNode), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	if !pass {
		sc.Logger.Error("校验服务树节点权限校验失败未通过", zap.Any("StreeNode", dbNode), zap.Error(err))
		common.Req403WithMessage("服务树节点权限校验未通过", c)
		return
	}

	// 根据dbNode的Pid去查询
	childrens, _ := models.GetStreeNodesByPId(int(dbNode.ID))

	// 如果dbNode的children不为空 不允许删除
	if childrens != nil && len(childrens) > 0 {
		err = errors.New(fmt.Sprintf("不允许删除非叶子节点 id:%v title:%v", id, dbNode.Title))
		sc.Logger.Error("不允许删除非叶子节点", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = dbNode.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除树节点错误", zap.Any("树节点", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

func getLeafStreeNodes(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	topNodes, err := models.GetStreeNodeAllLeaf()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的叶子节点错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的叶子节点错误：%v", err.Error()), c)
		return
	}
	leafNodes := []*models.StreeNode{}
	for _, node := range topNodes {
		node := node
		node.FillFrontAllData()
		if node.BindEcss == nil || len(node.BindEcss) == 0 {
			continue
		}
		leafNodes = append(leafNodes, node)
	}

	for _, node := range topNodes {
		node := node
		node.FillFrontAllData()
		//for _, user := range node.OpsAdmins {
		//	user := user
		//	node.OpsAdminUsers = append(node.OpsAdminUsers, user.Username)
		//}
	}
	common.OkWithDetailed(leafNodes, "ok", c)
}

// 获取所有的叶子节点
func getTopStreeNodes(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	topNodes, err := models.GetStreeNodeByLevel(1)
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的顶级服务树节点错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的顶级服务树节点错误：%v", err.Error()), c)
		return
	}

	for _, node := range topNodes {
		node := node
		node.FillFrontAllData()

		for _, user := range node.OpsAdmins {
			user := user
			node.OpsAdminUsers = append(node.OpsAdminUsers, user.Username)
		}
	}
	common.OkWithDetailed(topNodes, "ok", c)
}

// 根据id查下一级接口
func getChildrenStreeNodes(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	pid := c.Param("pid")
	sc.Logger.Info("获取下一层级的节点", zap.Any("pid", pid))
	intVar, _ := strconv.Atoi(pid)

	childrens, err := models.GetStreeNodesByPId(intVar)
	if err != nil {
		sc.Logger.Error("根据pid找子节点错误", zap.Error(err))
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}
	for _, node := range childrens {
		node := node
		node.FillFrontAllData()
		for _, user := range node.OpsAdmins {
			user := user
			node.OpsAdminUsers = append(node.OpsAdminUsers, user.Username)
		}
	}

	common.OkWithDetailed(childrens, "ok", c)
}

func updateStreeNode(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqNode models.StreeNode
	err := c.ShouldBindJSON(&reqNode)
	if err != nil {
		sc.Logger.Error("解析更新服务树节点请求失败", zap.Any("树节点", reqNode), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqNode)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	_, err = models.GetStreeNodeById(int(reqNode.ID))
	if err != nil {
		sc.Logger.Error("根据id找树节点错误", zap.Any("树节点", reqNode), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	users := make([]*models.User, 0)
	for _, userName := range reqNode.OpsAdminUsers {
		dbUser, err := models.GetUserByUsername(userName)
		if err != nil {
			sc.Logger.Error("树节点根据userName找用户错误", zap.Any("树节点", reqNode), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}

		users = append(users, dbUser)
	}

	reqNode.OpsAdmins = users
	// 更新
	err = reqNode.UpdateStreeNode()
	if err != nil {
		sc.Logger.Error("更新树节点和关联的运维负责人错误", zap.Any("树节点", reqNode), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

// 叶子节点id数组 ids port 传入 返回所有相关叶子节点ip+port的数组
func getLeafStreeNodeBindIps(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	leafNodeIds := c.DefaultQuery("leafNodeIds", "")
	port, _ := strconv.Atoi(c.DefaultQuery("port", ""))
	if port == 0 {
		err := fmt.Errorf("prometheus监控http服务树发现未传参port")
		sc.Logger.Error(err.Error())
		common.FailWithMessage(err.Error(), c)
		return
	}

	if leafNodeIds == "" {
		err := fmt.Errorf("prometheus监控http服务树发现未传参leafNodeIds")
		sc.Logger.Error(err.Error())
		common.FailWithMessage(err.Error(), c)
		return
	}
	//sc.Logger.Info("获取下一层级的节点", zap.Any("pid", leafNodeIds))
	leafNodeIdsArr := strings.Split(leafNodeIds, ",")
	if len(leafNodeIdsArr) == 0 {
		return
	}
	leafNodeIdsInt := []int{}
	for _, ids := range leafNodeIdsArr {
		idInt, err := strconv.Atoi(ids)
		if err != nil {
			continue
		}
		leafNodeIdsInt = append(leafNodeIdsInt, idInt)
	}

	if len(leafNodeIdsInt) == 0 {
		return
	}
	leafNodes, err := models.GetStreeNodeByIds(leafNodeIdsInt)
	if err != nil {
		msg := "prometheus监控http服务树发现 根据ids获取叶子节点错误"
		sc.Logger.Error(msg, zap.Error(err), zap.Any("叶子节点", leafNodeIds))
		return
	}

	sdTargetsMap := map[string]*targetgroup.Group{}
	// 遍历叶子节点 找到绑定的机器
	//for _, leafNode := range leafNodes {
	//	leafNode := leafNode
	//	if leafNode.BindEcss == nil {
	//		continue
	//	}
	//	if len(leafNode.BindEcss) == 0 {
	//		continue
	//	}
	//	for _, ecs := range leafNode.BindEcss {
	//		ecs := ecs
	//		oneTarget := targetgroup.Group{
	//			Targets: []model.LabelSet{
	//				{model.AddressLabel: model.LabelValue(fmt.Sprintf("%v:%v", ecs.PrivateIpAddress, port))},
	//			},
	//			Labels: model.LabelSet{},
	//		}
	//		// 设置虚拟机的标签
	//		for _, tag := range ecs.Tags {
	//			tags := strings.Split(tag, "=")
	//			tagK := tags[0]
	//			tagV := tags[1]
	//			oneTarget.Labels[model.LabelName(tagK)] = model.LabelValue(tagV)
	//		}
	//
	//		sdTargetsMap[ecs.PrivateIpAddress] = oneTarget
	//	}
	//}
	for _, leafNode := range leafNodes {
		if leafNode.BindEcss == nil || len(leafNode.BindEcss) == 0 {
			continue
		}
		for _, ecs := range leafNode.BindEcss {
			// 解决报错2：PrivateIpAddress 是 StringArray，提取第一个 IP
			if len(ecs.PrivateIpAddress) == 0 {
				continue // 如果没有私网IP则跳过
			}
			primaryIp := ecs.PrivateIpAddress[0] // 取出数组中的第一个 IP 字符串

			oneTarget := targetgroup.Group{
				Targets: []model.LabelSet{
					// 使用提取出的 primaryIp
					{model.AddressLabel: model.LabelValue(fmt.Sprintf("%v:%v", primaryIp, port))},
				},
				Labels: model.LabelSet{},
			}
			// 设置虚拟机的标签
			for _, tag := range ecs.Tags {
				tags := strings.SplitN(tag, "=", 2) // 使用 SplitN 防止 tag 值里有 "=" 报错
				if len(tags) == 2 {
					tagK := tags[0]
					tagV := tags[1]
					oneTarget.Labels[model.LabelName(tagK)] = model.LabelValue(tagV)
				}
			}

			// 解决报错1：将 oneTarget 的内存地址存入 map (因为 map value 期望的是指针 *)
			sdTargetsMap[primaryIp] = &oneTarget
		}
	}
	fRes := []*targetgroup.Group{}
	for _, v := range sdTargetsMap {
		v := v
		fRes = append(fRes, v)
	}
	c.JSON(200, fRes)
}

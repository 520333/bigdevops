package view

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

func approvalWorkOrderInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		common.ReqBadFailWithMessage("用户校验失败", c)
		return
	}

	approvalAction := c.Query("approvalAction")
	if _, ok := common.ApprovalActionMap[approvalAction]; !ok {
		common.ReqBadFailWithMessage("审批动作传参错误或没传", c)
		return
	}

	message := c.Query("message")
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)
	dbObj, err := models.GetWorkOrderInstanceById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找工单实例错误", zap.Any("工单实例", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	var flowNodes []models.WorkOrderFlowNode
	err = json.Unmarshal([]byte(dbObj.ActualFlowData), &flowNodes)
	if err != nil {
		errMsg := "工单实例模板解析为空或失败"
		sc.Logger.Error(errMsg, zap.Any("工单实例", id), zap.Error(err))
		common.FailWithMessage(errMsg, c)
		return
	}

	// 1. 寻找当前待处理的节点索引 (第一个 ActualUser 为空的节点)
	thisNodeIndex := -1
	for index, flowNode := range flowNodes {
		if flowNode.ActualUser == "" {
			thisNodeIndex = index
			break
		}
	}

	if thisNodeIndex == -1 {
		common.FailWithMessage("该工单已无待处理节点", c)
		return
	}

	isPass := false
	outPut := ""
	// 2. 根据审批动作计算状态流转
	switch approvalAction {
	case common.ApprovalActionPass:
		// 默认假设当前节点通过后，工单就走完了
		isPass = true
		outPut = "审批通过"
		dbObj.Status = common.WORKORDER_INSTANCE_FINISHED
		dbObj.CurrentFlowNode = "" // 流程走完，当前节点置空

		// 遍历寻找下一个需要处理的节点
		for nextIndex := thisNodeIndex + 1; nextIndex < len(flowNodes); nextIndex++ {
			nextNode := flowNodes[nextIndex]

			// 兼容前端传来的 Type (可能是常量枚举，也可能是中文名，根据你的 flowNode 结构决定)
			if nextNode.Type == common.FLOW_TYPE_APPROVAL || nextNode.Type == "审批节点" {
				dbObj.Status = common.WORKORDER_INSTANCE_PENDINGAPPROVAL
				dbObj.CurrentFlowNode = nextNode.DefineUserOrGroup
				break
			} else if nextNode.Type == common.FLOW_TYPE_ACTION || nextNode.Type == "执行节点" {
				dbObj.Status = common.WORKORDER_INSTANCE_PENDING_ACTION
				dbObj.CurrentFlowNode = nextNode.DefineUserOrGroup
				break
			} else if nextNode.Type == "Start" || nextNode.Type == "起始节点" || nextNode.Type == "开始节点" {
				continue
			} else if nextNode.Type == "Stop" || nextNode.Type == "结束节点" {
				break
			}
		}

	case common.ApprovalActionReject:
		// 拒绝的话，直接打回，流程终止
		isPass = false
		outPut = "审批拒绝"
		if message != "" {
			outPut = message
		}
		dbObj.Status = common.WORKORDER_INSTANCE_APPROVAL_REJECT
		dbObj.CurrentFlowNode = ""
	}

	// 3. 记录当前节点的执行人和完成时间
	flowNodes[thisNodeIndex].ActualUser = dbUser.Username
	flowNodes[thisNodeIndex].EndTime = time.Now().Format("2006-01-02 15:04:05")
	flowNodes[thisNodeIndex].IsPassOrIsSuccess = isPass
	flowNodes[thisNodeIndex].OutPut = outPut

	flownodeStr, _ := json.Marshal(flowNodes)
	dbObj.ActualFlowData = string(flownodeStr)

	// 4. 🚨 核心修复：使用 Map 强制更新数据库
	// GORM 默认结构体更新会忽略零值(空字符串)，导致结束或拒绝时 current_flow_node 无法在数据库中被清空
	err = models.Db.Model(&dbObj).Where("id = ?", dbObj.ID).Updates(map[string]interface{}{
		"status":            dbObj.Status,
		"current_flow_node": dbObj.CurrentFlowNode,
		"actual_flow_data":  dbObj.ActualFlowData,
	}).Error

	if err != nil {
		sc.Logger.Error("更新审批记录错误", zap.Any("工单实例", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 5. 返回结果
	msg := "认领并审批通过成功"
	if approvalAction == common.ApprovalActionReject {
		msg = "已拒绝该审批"
	}
	common.OkWithMessage(msg, c)
}

type actionWorkOrderOneResult struct {
	IsSuccess bool   `json:"isSuccess"`
	Output    string `json:"output"  validate:"required,min=1"`
}

func actionWorkOrderInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		common.ReqBadFailWithMessage("用户校验失败", c)
		return
	}

	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)
	dbObj, err := models.GetWorkOrderInstanceById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找工单实例错误", zap.Any("工单实例", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	var reqObj actionWorkOrderOneResult
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析工单更新结果参数解析失败", c)
		return
	}

	var flowNodes []models.WorkOrderFlowNode
	err = json.Unmarshal([]byte(dbObj.ActualFlowData), &flowNodes)
	if err != nil {
		errMsg := "工单实例模板解析为空或失败"
		sc.Logger.Error(errMsg, zap.Any("工单实例", id), zap.Error(err))
		common.FailWithMessage(errMsg, c)
		return
	}

	// 1. 安全寻找当前正在执行的节点 (第一个 ActualUser 为空的节点)
	thisNodeIndex := -1
	for index, flowNode := range flowNodes {
		if flowNode.ActualUser == "" {
			thisNodeIndex = index
			break
		}
	}

	if thisNodeIndex == -1 {
		common.FailWithMessage("该工单已无待处理节点", c)
		return
	}

	// 2. 提前记录当前节点的执行结果和输出
	flowNodes[thisNodeIndex].ActualUser = dbUser.Username
	flowNodes[thisNodeIndex].OutPut = reqObj.Output
	flowNodes[thisNodeIndex].IsPassOrIsSuccess = reqObj.IsSuccess
	flowNodes[thisNodeIndex].EndTime = time.Now().Format("2006-01-02 15:04:05")

	// 3. 根据执行结果动态判断流转状态
	if reqObj.IsSuccess {
		// 默认假设执行成功后，工单就彻底结束了
		dbObj.Status = common.WORKORDER_INSTANCE_FINISHED
		dbObj.CurrentFlowNode = "" // 流程走完，当前节点置空

		// 往后遍历，看看是不是还有下一个节点要走（比如连续两个执行节点）
		for nextIndex := thisNodeIndex + 1; nextIndex < len(flowNodes); nextIndex++ {
			nextNode := flowNodes[nextIndex]

			if nextNode.Type == common.FLOW_TYPE_APPROVAL || nextNode.Type == "审批节点" {
				dbObj.Status = common.WORKORDER_INSTANCE_PENDINGAPPROVAL
				dbObj.CurrentFlowNode = nextNode.DefineUserOrGroup
				break
			} else if nextNode.Type == common.FLOW_TYPE_ACTION || nextNode.Type == "执行节点" {
				dbObj.Status = common.WORKORDER_INSTANCE_PENDING_ACTION
				dbObj.CurrentFlowNode = nextNode.DefineUserOrGroup
				break
			} else if nextNode.Type == "Start" || nextNode.Type == "起始节点" || nextNode.Type == "开始节点" {
				continue
			} else if nextNode.Type == "Stop" || nextNode.Type == "结束节点" {
				break
			}
		}
	} else {
		// 💡 如果执行失败，直接中断流程。你可以定义为 FAILED，这里我先复用驳回状态
		dbObj.Status = common.WORKORDER_INSTANCE_APPROVAL_REJECT
		dbObj.CurrentFlowNode = ""
	}

	// 4. 落盘保存
	flownodeStr, _ := json.Marshal(flowNodes)
	dbObj.ActualFlowData = string(flownodeStr)

	// 🚨 核心：必须使用 Map 强制更新，确保能把 CurrentFlowNode 更新为 ""
	err = models.Db.Model(&dbObj).Where("id = ?", dbObj.ID).Updates(map[string]interface{}{
		"status":            dbObj.Status,
		"current_flow_node": dbObj.CurrentFlowNode,
		"actual_flow_data":  dbObj.ActualFlowData,
	}).Error

	if err != nil {
		sc.Logger.Error("更新执行记录错误", zap.Any("工单实例", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("执行结果提交成功", c)
}

func commentWorkOrderInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		common.ReqBadFailWithMessage("用户校验失败", c)
		return
	}

	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)
	dbObj, err := models.GetWorkOrderInstanceById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找工单实例错误", zap.Any("工单实例", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	var reqObj models.WorkOrderInstanceComment

	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage("解析工单更新结果参数解析失败", c)
		return
	}

	var comments []models.WorkOrderInstanceComment
	if dbObj.Comments != "" {
		err = json.Unmarshal([]byte(dbObj.Comments), &comments)
		if err != nil {
			errMsg := "工单实例模板解析为空或失败"
			sc.Logger.Error(errMsg, zap.Any("工单实例", id), zap.Error(err))
			common.FailWithMessage(errMsg, c)
			return
		}
	}

	reqObj.UserNameTime = fmt.Sprintf("%s %s", dbUser.Username, time.Now().Format("2006-01-02 15:04:05"))

	comments = append(comments, reqObj)
	commentStr, _ := json.Marshal(comments)
	dbObj.Comments = string(commentStr)
	err = dbObj.UpdateOne()

	if err != nil {
		sc.Logger.Error("更新评论记录错误", zap.Any("工单实例", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("评论成功", c)
}

func getWorkOrderInstanceList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")
	searchTitle := c.DefaultQuery("title", "")
	// 🚨 1. 新增接收前端传来的 status 参数
	searchStatus := c.DefaultQuery("status", "")

	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		common.ReqBadFailWithMessage("用户校验失败", c)
		return
	}

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	var objs []*models.WorkOrderInstance
	isRelatedWithMe := false
	queryModel := c.DefaultQuery("queryModel", common.WORKORDER_INSTANCE_QUERYMODE_ALL)

	switch queryModel {
	case common.WORKORDER_INSTANCE_QUERYMODE_MINE:
		// 🚨 2. 优化 MINE 模式：使用 GORM 动态拼接查询条件
		dbQuery := models.Db.Model(&models.WorkOrderInstance{}).Where("user_id = ?", dbUser.ID)

		if searchStatus != "" {
			dbQuery = dbQuery.Where("status = ?", searchStatus)
		}
		if searchTitle != "" {
			dbQuery = dbQuery.Where("title LIKE ?", "%"+searchTitle+"%")
		}

		var total int64
		dbQuery.Count(&total)
		err = dbQuery.Limit(limit).Offset(offset).Find(&objs).Error

		if err != nil {
			sc.Logger.Error(fmt.Sprintf("去数据库中拿用户%v创建的工单实例错误", dbUser.Username), zap.Error(err))
			common.ReqBadFailWithMessage(fmt.Sprintf("用户:%v创建工单错误%v", dbUser.Username, err.Error()), c)
			return
		}

		for _, obj := range objs {
			obj.FillFrontAllData()
		}

		common.OkWithDetailed(gin.H{
			"items": objs,
			"total": total,
		}, "ok", c)

	case common.WORKORDER_INSTANCE_QUERYMODE_ALL:
		objs, err = models.GetWorkOrderInstanceAll()
		if err != nil {
			sc.Logger.Error("去数据库中拿所有的工单实例错误", zap.Error(err))
			common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的工单实例错误：%v", err.Error()), c)
			return
		}
		allIds := []int{}

		for _, obj := range objs {
			obj.FillFrontAllData()

			if searchUserID != "" && !strings.Contains(obj.CreateUserName, searchUserID) {
				continue
			}
			if searchTitle != "" && !strings.Contains(obj.Title, searchTitle) {
				continue
			}
			// 🚨 3. 在 ALL 模式下，新增对 Status 的内存过滤
			if searchStatus != "" && obj.Status != searchStatus {
				continue
			}

			allIds = append(allIds, int(obj.ID))
		}

		if len(allIds) == 0 {
			common.OkWithDetailed(gin.H{
				"items": []models.WorkOrderInstance{},
				"total": 0,
			}, "ok", c)
			return
		}

		pagedObjs, err := models.GetWorkOrderInstanceByIdsWithLimitOffset(allIds, limit, offset)
		if err != nil {
			sc.Logger.Error("limit-offset 去数据库中拿所有的工单实例错误", zap.Error(err))
			common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的工单实例错误：%v", err.Error()), c)
			return
		}
		for _, obj := range pagedObjs {
			obj.FillFrontAllData()
		}

		common.OkWithDetailed(gin.H{
			"items": pagedObjs,
			"total": len(allIds),
		}, "ok", c)

	case common.WORKORDER_INSTANCE_QUERYMODE_APPROVAL:
		isRelatedWithMe = true
		currentFlowNodes := []string{
			fmt.Sprintf("用户@%s", dbUser.Username),
			dbUser.Username,
		}
		for _, role := range dbUser.Roles {
			currentFlowNodes = append(currentFlowNodes, fmt.Sprintf("组@%s", role.RoleName))
			currentFlowNodes = append(currentFlowNodes, role.RoleName)
		}

		total := models.GetWorkOrderInstanceByStatusAndCurrentCount(common.WORKORDER_INSTANCE_PENDINGAPPROVAL, currentFlowNodes)

		if total > 0 {
			objs, err = models.GetWorkOrderInstanceByStatusAndCurrentFlowNodes(common.WORKORDER_INSTANCE_PENDINGAPPROVAL, currentFlowNodes, limit, offset)
			if err != nil {
				sc.Logger.Error("获取待审批工单失败", zap.Error(err))
				common.ReqBadFailWithMessage("获取待审批工单失败", c)
				return
			}

			for _, obj := range objs {
				obj.IsRelatedWithMe = isRelatedWithMe
				obj.FillFrontAllData()
			}
		} else {
			objs = []*models.WorkOrderInstance{}
		}

		common.OkWithDetailed(gin.H{
			"items": objs,
			"total": total,
		}, "ok", c)

	case common.WORKORDER_INSTANCE_QUERYMODE_ACTION:
		isRelatedWithMe = true
		currentFlowNodes := []string{
			fmt.Sprintf("用户@%s", dbUser.Username),
			dbUser.Username,
		}
		for _, role := range dbUser.Roles {
			currentFlowNodes = append(currentFlowNodes, fmt.Sprintf("组@%s", role.RoleName))
			currentFlowNodes = append(currentFlowNodes, role.RoleName)
		}

		total := models.GetWorkOrderInstanceByStatusAndCurrentCount(common.WORKORDER_INSTANCE_PENDING_ACTION, currentFlowNodes)

		if total > 0 {
			objs, err = models.GetWorkOrderInstanceByStatusAndCurrentFlowNodes(common.WORKORDER_INSTANCE_PENDING_ACTION, currentFlowNodes, limit, offset)
			if err != nil {
				sc.Logger.Error("获取待执行工单失败", zap.Error(err))
				common.ReqBadFailWithMessage("获取待执行工单失败", c)
				return
			}

			for _, obj := range objs {
				obj.IsRelatedWithMe = isRelatedWithMe
				obj.FillFrontAllData()
			}
		} else {
			objs = []*models.WorkOrderInstance{}
		}

		common.OkWithDetailed(gin.H{
			"items": objs,
			"total": total,
		}, "ok", c)
	}
}
func getWorkOrderInstanceDetail(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("工单实例", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetWorkOrderInstanceById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找工单实例错误", zap.Any("工单实例", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbObj.FillFrontAllData()

	common.OkWithDetailed(dbObj, "ok", c)
}

type CreateInstanceReq struct {
	Title            string `json:"title"`
	TemplateId       uint   `json:"templateId"`
	FormValues       string `json:"formValues"`
	DesireFinishTime string `json:"desireFinishTime"` // 接收字符串
}

func createWorkOrderInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	// 1. 使用 DTO 接收数据
	var req CreateInstanceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.FailWithMessage("参数解析失败", c)
		return
	}

	// 2. 手动转换模型
	reqObj := models.WorkOrderInstance{
		Title:             req.Title,
		TemplateId:        req.TemplateId,
		ActualApiJsonData: req.FormValues,
	}

	// 3. 转换时间字符串为 time.Time
	if req.DesireFinishTime != "" {
		// 解析格式需与前端 format('YYYY-MM-DD HH:mm:ss') 一致
		t, err := time.ParseInLocation("2006-01-02 15:04:05", req.DesireFinishTime, time.Local)
		if err == nil {
			reqObj.DesireFinishTime = &t // 这里需要模型里是 *time.Time
		}
	}

	// 4. 后续逻辑保持不变...
	_, err := models.GetWorkOrderTemplateById(int(reqObj.TemplateId))
	if err != nil {
		common.ReqBadFailWithMessage("根据模板ID找Template失败", c)
		return
	}

	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		common.ReqBadFailWithMessage("用户校验失败", c)
		return
	}

	reqObj.UserID = dbUser.ID

	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增工单实例数据库失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("创建成功", c)
}

func updateWorkOrderInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.WorkOrderInstance
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增工单实例请求失败", zap.Any("工单实例", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqObj)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	_, err = models.GetWorkOrderInstanceById(int(reqObj.ID))
	if err != nil {
		sc.Logger.Error("根据id找工单实例错误", zap.Any("工单实例", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新工单实例错误", zap.Any("工单实例", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

func deleteWorkOrderInstance(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("删除工单实例", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbRole, err := models.GetWorkOrderInstanceById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找工单实例错误", zap.Any("工单实例", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = dbRole.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除工单实例错误", zap.Any("工单实例", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("删除成功", c)
}

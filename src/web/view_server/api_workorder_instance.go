package view_server

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

// @Summary      审批工单申请
// @Description  审批工单申请 接口
// @Tags         workorder-instance
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "审批工单申请 响应结果"
// @Router       /workorder/approvalWorkOrderInstance/{id} [post]
// @Security     Bearer
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

// @Summary      处理执行工单节点
// @Description  处理执行工单节点 接口
// @Tags         workorder-instance
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "处理执行工单节点 响应结果"
// @Router       /workorder/actionWorkOrderInstance/{id} [post]
// @Security     Bearer
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

// @Summary      发表工单评论/留言
// @Description  发表工单评论/留言 接口
// @Tags         workorder-instance
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "发表工单评论/留言 响应结果"
// @Router       /workorder/commentWorkOrderInstance/{id} [post]
// @Security     Bearer
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

// @Summary      获取工单申请实例列表
// @Description  获取工单申请实例列表 接口
// @Tags         workorder-instance
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取工单申请实例列表 响应结果"
// @Router       /workorder/getWorkOrderInstanceList [get]
// @Security     Bearer
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

// @Summary      获取工单申请实例详情与流转轨迹
// @Description  获取工单申请实例详情与流转轨迹 接口
// @Tags         workorder-instance
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取工单申请实例详情与流转轨迹 响应结果"
// @Router       /workorder/getWorkOrderInstanceDetail/{id} [get]
// @Security     Bearer
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

// @Summary      提交流程工单申请
// @Description  提交流程工单申请 接口
// @Tags         workorder-instance
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "提交流程工单申请 响应结果"
// @Router       /workorder/createWorkOrderInstance [post]
// @Security     Bearer
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

// @Summary      更新工单申请实例
// @Description  更新工单申请实例 接口
// @Tags         workorder-instance
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "更新工单申请实例 响应结果"
// @Router       /workorder/updateWorkOrderInstance [post]
// @Security     Bearer
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

// @Summary      删除工单申请实例
// @Description  删除工单申请实例 接口
// @Tags         workorder-instance
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "删除工单申请实例 响应结果"
// @Router       /workorder/deleteWorkOrderInstance/{id} [delete]
// @Security     Bearer
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

// @Summary      获取顶部通知/待办/消息列表
// @Description  获取当前登录用户的工单待办、通知与消息接口
// @Tags         workorder-instance
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "通知/待办 响应结果"
// @Router       /workorder/getNotificationList [get]
// @Security     Bearer
func getWorkOrderNotificationList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		sc.Logger.Error("查询用户失败", zap.Error(err))
		common.FailWithMessage("用户校验失败", c)
		return
	}

	isAdminUser := (userName == "admin")
	for _, r := range dbUser.Roles {
		if r.RoleName == "admin" {
			isAdminUser = true
			break
		}
	}

	type ListItem struct {
		ID          string `json:"id"`
		Avatar      string `json:"avatar"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Datetime    string `json:"datetime"`
		Type        string `json:"type"`
		Extra       string `json:"extra,omitempty"`
		Color       string `json:"color,omitempty"`
		TicketID    uint   `json:"ticketId,omitempty"`
		Read        bool   `json:"read"`
		TitleDelete bool   `json:"titleDelete"`
	}

	type TabItem struct {
		Key  string     `json:"key"`
		Name string     `json:"name"`
		List []ListItem `json:"list"`
	}

	var todoList []ListItem
	var noticeList []ListItem
	var msgList []ListItem

	// 1. 查询【待办】：状态为 pendingApproval 或 pendingAction 的工单
	var pendingOrders []*models.WorkOrderInstance
	err = models.Db.Where("status IN ?", []string{common.WORKORDER_INSTANCE_PENDINGAPPROVAL, common.WORKORDER_INSTANCE_PENDING_ACTION}).
		Order("created_at desc").Limit(20).Find(&pendingOrders).Error
	if err == nil {
		for _, order := range pendingOrders {
			order.FillFrontAllData()

			// 严谨判断：只有当前流转节点匹配当前用户或当前用户所在角色时，才是当前用户的待办
			isMyTodo := false
			if order.CurrentFlowNode == userName || strings.Contains(order.CurrentFlowNode, userName) {
				isMyTodo = true
			} else {
				for _, role := range dbUser.Roles {
					if order.CurrentFlowNode == role.RoleName || strings.Contains(order.CurrentFlowNode, role.RoleName) {
						isMyTodo = true
						break
					}
				}
			}
			if order.CurrentFlowNode == "admin" && isAdminUser {
				isMyTodo = true
			}

			if isMyTodo {
				statusText := "待审批"
				color := "orange"
				if order.Status == common.WORKORDER_INSTANCE_PENDING_ACTION {
					statusText = "待执行"
					color = "blue"
				}
				timeStr := order.CreatedAt.Format("2006-01-02 15:04")
				todoList = append(todoList, ListItem{
					ID:          fmt.Sprintf("todo-%d", order.ID),
					Avatar:      "",
					Title:       fmt.Sprintf("工单待办：%s", order.Title),
					Description: fmt.Sprintf("申请人：%s | 当前节点：%s", order.CreateUserName, order.CurrentFlowNode),
					Datetime:    timeStr,
					Type:        "3",
					Extra:       statusText,
					Color:       color,
					TicketID:    order.ID,
				})
			}
		}
	}

	// 2. 查询【通知】：当前用户发起的近期已完成或终止的工单
	var myOrders []*models.WorkOrderInstance
	err = models.Db.Where("user_id = ?", dbUser.ID).Order("updated_at desc").Limit(10).Find(&myOrders).Error
	if err == nil {
		for _, order := range myOrders {
			if order.Status == common.WORKORDER_INSTANCE_FINISHED || order.Status == common.WORKORDER_INSTANCE_APPROVAL_REJECT {
				timeStr := order.UpdatedAt.Format("2006-01-02 15:04")
				statusMsg := "已完成"
				color := "green"
				if order.Status == common.WORKORDER_INSTANCE_APPROVAL_REJECT {
					statusMsg = "已被驳回"
					color = "red"
				}
				desireTime := "无"
				if order.DesireFinishTime != nil {
					desireTime = order.DesireFinishTime.Format("2006-01-02 15:04")
				}
				noticeList = append(noticeList, ListItem{
					ID:          fmt.Sprintf("notice-%d", order.ID),
					Avatar:      "",
					Title:       fmt.Sprintf("工单：%s", order.Title),
					Description: fmt.Sprintf("期望完成时间：%s", desireTime),
					Datetime:    timeStr,
					Type:        "1",
					Extra:       statusMsg,
					Color:       color,
					TicketID:    order.ID,
				})
			}
		}
	}

	// 3. 查询【消息】：他人对工单发起的评论互动 (排除用户自己评论自己的通知)
	var commentedOrders []*models.WorkOrderInstance
	if isAdminUser {
		err = models.Db.Where("comments IS NOT NULL AND comments != '' AND comments != '[]'").
			Order("updated_at desc").Limit(30).Find(&commentedOrders).Error
	} else {
		userPattern := "%" + userName + "%"
		err = models.Db.Where("(user_id = ? OR actual_flow_data LIKE ? OR comments LIKE ?) AND comments IS NOT NULL AND comments != '' AND comments != '[]'",
			dbUser.ID, userPattern, userPattern).
			Order("updated_at desc").Limit(20).Find(&commentedOrders).Error
	}

	if err == nil {
		for _, order := range commentedOrders {
			var comments []map[string]interface{}
			if err := json.Unmarshal([]byte(order.Comments), &comments); err == nil && len(comments) > 0 {
				// 从最新评论倒序查找第一条【他人】发的评论
				for i := len(comments) - 1; i >= 0; i-- {
					c := comments[i]
					userNameTime := fmt.Sprintf("%v", c["userNameTime"])
					parts := strings.SplitN(userNameTime, " ", 2)
					author := parts[0]
					commentTime := ""
					if len(parts) > 1 {
						commentTime = parts[1]
					}

					// 屏蔽当前用户自己评论自己的消息通知
					if author == userName {
						continue
					}

					commentContent := fmt.Sprintf("%v", c["comment"])
					msgList = append(msgList, ListItem{
						ID:          fmt.Sprintf("msg-%d-%d", order.ID, i),
						Avatar:      "",
						Title:       fmt.Sprintf("%s 评论了工单【%s】", author, order.Title),
						Description: commentContent,
						Datetime:    commentTime,
						Type:        "2",
						TicketID:    order.ID,
					})
					break
				}
			}
		}
	}

	// 4. 🚨 从数据库查询当前用户的已读和已清空持久化状态进行过滤与标记
	notifyStatusMap, _ := models.GetUserNotifyStatusMap(userName)
	filterAndMarkList := func(list []ListItem) []ListItem {
		var result []ListItem
		for _, item := range list {
			statusVal, exists := notifyStatusMap[item.ID]
			if exists && statusVal == models.NOTIFY_STATUS_CLEARED {
				// 已清空 -> 直接在后端过滤，不返回前端
				continue
			}
			if exists && statusVal == models.NOTIFY_STATUS_READ {
				// 已读 -> 标记 read 和 titleDelete
				item.Read = true
				item.TitleDelete = true
			}
			result = append(result, item)
		}
		if result == nil {
			result = []ListItem{}
		}
		return result
	}

	todoList = filterAndMarkList(todoList)
	noticeList = filterAndMarkList(noticeList)
	msgList = filterAndMarkList(msgList)

	resData := []TabItem{
		{Key: "1", Name: "通知", List: noticeList},
		{Key: "2", Name: "消息", List: msgList},
		{Key: "3", Name: "待办", List: todoList},
	}

	common.OkWithData(resData, c)
}

type markNotifyReadReq struct {
	NoticeID string `json:"noticeId"`
}

// @Summary      标记单个通知为已读
// @Description  标记单个通知为已读 接口
// @Tags         workorder-instance
// @Accept       json
// @Produce      json
// @Router       /workorder/markNotifyRead [post]
// @Security     Bearer
func markWorkOrderNotifyRead(c *gin.Context) {
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	var req markNotifyReadReq
	if err := c.ShouldBindJSON(&req); err != nil || req.NoticeID == "" {
		common.FailWithMessage("参数解析失败", c)
		return
	}

	err := models.MarkNotifyStatus(userName, req.NoticeID, models.NOTIFY_STATUS_READ)
	if err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("已标记已读", c)
}

type clearNotifyTabReq struct {
	NoticeIDs []string `json:"noticeIds"`
}

// @Summary      批量清空通知/消息/待办
// @Description  批量清空通知/消息/待办 接口
// @Tags         workorder-instance
// @Accept       json
// @Produce      json
// @Router       /workorder/clearNotifyTab [post]
// @Security     Bearer
func clearWorkOrderNotifyTab(c *gin.Context) {
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	var req clearNotifyTabReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.FailWithMessage("参数解析失败", c)
		return
	}

	err := models.BatchMarkNotifyStatus(userName, req.NoticeIDs, models.NOTIFY_STATUS_CLEARED)
	if err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("已清空", c)
}

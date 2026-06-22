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

func getJobExecTaskList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")
	searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchTitle := c.DefaultQuery("title", "")
	searchCreateUserName := c.DefaultQuery("createUserName", "")

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	// 数据库中拿到所有的JobTask列表
	objs, err := models.GetJobTaskAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的任务执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的任务执行错误：%v", err.Error()), c)
		return
	}
	allIds := []int{}

	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}

		if searchTitle != "" && !strings.Contains(obj.Title, searchTitle) {
			continue
		}

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
			"items": []models.JobTask{},
			"total": 0,
		}, "ok", c)
		return
	}

	// 根据过滤后的 ID 进行分页查询
	pagedObjs, err := models.GetJobTaskByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的任务执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的任务执行错误：%v", err.Error()), c)
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

func getJobExecTaskOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("任务实例", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetJobTaskById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找任务实例错误", zap.Any("任务实例", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbObj.FillFrontAllData()

	common.OkWithDetailed(dbObj, "ok", c)
}

func createJobExecTask(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.JobTask
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增任务执行请求失败", zap.Any("任务执行", reqObj), zap.Error(err))
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
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		sc.Logger.Error("通过token解析到的userName去数据库中找User", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("通过token解析到的userName去数据库中找User失败 %v", err.Error()), c)
		return
	}
	reqObj.UserID = dbUser.ID

	var hostIds []string

	err = json.Unmarshal([]byte(reqObj.HostsIdsRaw), &hostIds)
	if err != nil {
		errMsg := "通过hostIdsRaw解析到的hostIdsRaw解析 json 到的hostIds错误"
		sc.Logger.Error(errMsg, zap.Error(err))
		common.ReqBadFailWithMessage(errMsg, c)
		return
	}
	if len(hostIds) == 0 {
		errMsg := "通过hostIdsRaw解析到的hostIds为空"
		sc.Logger.Error(errMsg, zap.Error(err))
		common.ReqBadFailWithMessage(errMsg, c)
		return
	}
	hostIdsInt := []int{}
	for _, idS := range hostIds {
		intStr, _ := strconv.Atoi(idS)
		hostIdsInt = append(hostIdsInt, intStr)
	}
	hosts, err := models.GetResourceEcsByIdsWithLimitOffset(hostIdsInt, 10000, 0)
	if err != nil {
		errMsg := "通过hostId找机器错误"
		sc.Logger.Error(errMsg, zap.Error(err))
		common.ReqBadFailWithMessage(errMsg, c)
		return
	}
	hostIps := []string{}
	for _, host := range hosts {
		// 💡 修复点：判断数组是否为空，如果不为空，则取第一个私有 IP
		if len(host.PrivateIpAddress) > 0 {
			hostIps = append(hostIps, host.PrivateIpAddress[0])
		}
	}
	hostIpsRaw, err := json.Marshal(hostIps)
	if err != nil {
		common.ReqBadFailWithMessage("IP序列化失败", c)
		return
	}
	reqObj.HostsRaw = string(hostIpsRaw)
	reqObj.TotalNum = len(hosts)
	reqObj.Status = common.JOB_STATUS_PENDING

	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增任务执行数据库失败", zap.Any("任务执行", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
	}
	common.OkWithMessage("创建成功", c)
}

func updateJobExecTask(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.JobTask
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增任务执行请求失败", zap.Any("任务执行", reqObj), zap.Error(err))
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

	_, err = models.GetJobTaskById(int(reqObj.ID))
	if err != nil {
		sc.Logger.Error("根据id找任务执行错误", zap.Any("任务执行", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新任务执行错误", zap.Any("任务执行", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

func deleteJobExecTask(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("删除任务执行", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbRole, err := models.GetJobTaskById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找任务执行错误", zap.Any("任务执行", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbTemplate, err := models.GetJobTaskById(intVar)

	// 1. 如果有 err 并且不是“未找到记录”的错误，说明数据库查询出错了
	if err != nil && err.Error() != "WorkOrderTemplate不存在" { // 这里的字符串取决于你 Get 方法里的定义
		sc.Logger.Error("检查表单关联模板时发生数据库错误", zap.Error(err))
		common.FailWithMessage("检查模板关联失败", c)
		return
	}

	// 2. 如果成功查到了模板，说明被占用了，明确拒绝并返回自定义提示
	if dbTemplate != nil && dbTemplate.ID > 0 {
		errMsg := fmt.Sprintf("该任务执行已被工单模板【%s】绑定，禁止直接删除！", dbTemplate.Title)
		sc.Logger.Warn(errMsg, zap.Any("表单ID", id))
		common.FailWithMessage(errMsg, c)
		return
	}
	err = dbRole.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除任务执行错误", zap.Any("任务执行", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("删除成功", c)
}

func actionJobExecTaskOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("任务动作", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetJobTaskById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找任务执行错误", zap.Any("任务执行", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	action := c.Query("action")
	nextStatus, exist := common.JOB_ACTION_NEXT_STATUS_MAP[action]
	if !exist {
		sc.Logger.Error("传入的动作错误", zap.Any("任务执行", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	if action == common.AGENT_TASK_ACTION_KILL {
		dbObj.Action = common.AGENT_TASK_ACTION_KILL
	}

	// ==========================================
	// 💡 补充记录任务流 到 ActualFlowData json
	// ==========================================

	// 1. 获取当前执行操作的用户
	userName := "系统"
	if claimUser, exists := c.Get(common.GIN_CTX_JWT_USER_NAME); exists {
		userName = claimUser.(string)
	}

	// 2. 动作中文映射 (提升前端时间轴的易读性)
	actionNameMap := map[string]string{
		common.AGENT_TASK_ACTION_START:  "手动下发执行",
		common.AGENT_TASK_ACTION_KILL:   "强行Kill终止",
		common.AGENT_TASK_ACTION_PAUSE:  "手动暂停任务",
		common.AGENT_TASK_ACTION_RESUME: "恢复执行任务",
		common.AGENT_TASK_ACTION_STOP:   "手动标记停止",
	}
	actionName := actionNameMap[action]
	if actionName == "" {
		actionName = action
	}

	// 3. 反序列化原有的历史流程记录
	var flowNodes []map[string]interface{}
	if dbObj.ActualFlowData != "" {
		err := json.Unmarshal([]byte(dbObj.ActualFlowData), &flowNodes)
		if err != nil {
			sc.Logger.Warn("解析原有的 ActualFlowData 失败，将初始化为空", zap.Error(err))
			flowNodes = []map[string]interface{}{}
		}
	}

	// 4. 构建新的时间轴节点
	// 属性命名与你在 Vue 前端 timeline 期望的字段完全对齐
	newNode := map[string]interface{}{
		"type":              actionName,                                    // 节点标题
		"endTime":           time.Now().Format("2006-01-02 15:04:05"),      // 操作时间
		"actualUser":        userName,                                      // 执行人
		"isPassOrIsSuccess": true,                                          // 渲染蓝色/绿色Tag
		"outPut":            fmt.Sprintf("指令下发成功，状态扭转为: [%s]", nextStatus), // 详情描述
	}

	// 5. 追加并重新序列化为 JSON 字符串
	flowNodes = append(flowNodes, newNode)
	flowBytes, _ := json.Marshal(flowNodes)
	dbObj.ActualFlowData = string(flowBytes)

	// ==========================================

	sc.Logger.Info("任务动作", zap.Any("id", id), zap.Any("动作", action), zap.Any("nextStatus", nextStatus))

	dbObj.Status = nextStatus
	err = dbObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新任务执行错误", zap.Any("任务执行", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("更新成功", c)

}

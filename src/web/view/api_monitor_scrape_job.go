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
	"go.uber.org/zap"
)

func getMonitorScrapeJobList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")
	searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchTitle := c.DefaultQuery("name", "")
	searchCreateUserName := c.DefaultQuery("createUserName", "")

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	objs, err := models.GetMonitorScrapeJobAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的采集任务执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的采集任务执行错误：%v", err.Error()), c)
		return
	}
	allIds := []int{}

	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}

		if searchTitle != "" && !strings.Contains(obj.Name, searchTitle) {
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
			"items": []models.MonitorScrapeJob{},
			"total": 0,
		}, "ok", c)
		return
	}

	// 根据过滤后的 ID 进行分页查询
	pagedObjs, err := models.GetMonitorScrapeJobByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的采集任务执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的采集任务执行错误：%v", err.Error()), c)
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

func getMonitorScrapeJobOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("采集任务实例", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetJobTaskById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找采集任务实例错误", zap.Any("采集任务实例", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbObj.FillFrontAllData()

	common.OkWithDetailed(dbObj, "ok", c)
}

func createMonitorScrapeJob(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj models.MonitorScrapeJob
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增采集任务执行请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = reqObj.ValidateRelabelConfigsYamlString()
	if err != nil {
		msg := "[监控模块]新增采集任务-解析relabelConfig错误"
		sc.Logger.Error(msg, zap.Any("采集池", reqObj.Name), zap.Any("yaml", reqObj.RelabelConfigsYamlString), zap.Error(err))
		common.FailWithMessage(msg, c)
		return
	}

	// 获取当前用户ID
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)

	if err == nil && dbUser != nil {
		reqObj.UserID = dbUser.ID
	}
	// 存入数据库
	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增采集任务执行数据库失败", zap.Error(err))
		common.FailWithMessage("存入数据库失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

func updateMonitorScrapeJob(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	// 🚀 致命修复：同上
	var reqObj models.MonitorScrapeJob
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新采集任务请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 检查是否存在
	_, err = models.GetMonitorScrapeJobById(int(reqObj.ID))
	if err != nil {
		common.FailWithMessage("采集任务不存在", c)
		return
	}
	err = reqObj.ValidateRelabelConfigsYamlString()
	if err != nil {
		msg := "[监控模块]新增采集任务-解析relabelConfig错误"
		sc.Logger.Error(msg, zap.Any("采集池", reqObj.Name), zap.Any("yaml", reqObj.RelabelConfigsYamlString), zap.Error(err))
		common.FailWithMessage(msg, c)
		return
	}
	// 更新
	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新采集任务执行错误", zap.Error(err))
		common.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

func deleteMonitorScrapeJob(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetMonitorScrapeJobById(intVar)
	if err != nil {
		common.FailWithMessage("采集任务不存在", c)
		return
	}

	err = dbObj.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除采集任务执行错误", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

func actionMonitorScrapeJobOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("采集任务动作", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetJobTaskById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找采集任务执行错误", zap.Any("采集任务执行", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	action := c.Query("action")
	nextStatus, exist := common.JOB_ACTION_NEXT_STATUS_MAP[action]
	if !exist {
		sc.Logger.Error("传入的动作错误", zap.Any("采集任务执行", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	if action == common.AGENT_TASK_ACTION_KILL {
		dbObj.Action = common.AGENT_TASK_ACTION_KILL
	}

	// ==========================================
	// 💡 补充记录采集任务流 到 ActualFlowData json
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
		common.AGENT_TASK_ACTION_PAUSE:  "手动暂停采集任务",
		common.AGENT_TASK_ACTION_RESUME: "恢复执行采集任务",
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

	sc.Logger.Info("采集任务动作", zap.Any("id", id), zap.Any("动作", action), zap.Any("nextStatus", nextStatus))

	dbObj.Status = nextStatus
	err = dbObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新采集任务执行错误", zap.Any("采集任务执行", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("更新成功", c)

}

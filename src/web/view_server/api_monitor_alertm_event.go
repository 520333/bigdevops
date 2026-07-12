package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/common/model"
	"go.uber.org/zap"
)

func getMonitorAlertEventList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	//searchUserID := c.DefaultQuery("UserID", "")
	//searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchTitle := c.DefaultQuery("name", "")
	//searchCreateUserName := c.DefaultQuery("createUserName", "")

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	objs, err := models.GetMonitorAlertEventAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的集群执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的集群执行错误：%v", err.Error()), c)
		return
	}
	allIds := []int{}

	for _, obj := range objs {
		//if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
		//	continue
		//}

		if searchTitle != "" && !strings.Contains(obj.AlertName, searchTitle) {
			continue
		}

		// 填充前端需要的数据（拿到组合好的 CreateUserName）
		obj.FillFrontAllData()

		// 🚀 修复 1：对比的应该是刚填充好的 CreateUserName 字段，而不是 UserID
		//if searchCreateUserName != "" && !strings.Contains(obj.CreateUserName, searchCreateUserName) {
		//	continue
		//}

		allIds = append(allIds, int(obj.ID))
	}

	// 如果过滤后没有数据，直接返回空列表
	if len(allIds) == 0 {
		common.OkWithDetailed(gin.H{
			"items": []models.MonitorAlertEvent{},
			"total": 0,
		}, "ok", c)
		return
	}

	// 根据过滤后的 ID 进行分页查询
	pagedObjs, err := models.GetMonitorAlertEventByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的集群执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的集群执行错误：%v", err.Error()), c)
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

//func createMonitorAlertEvent(c *gin.Context) {
//	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
//
//	var reqObj models.MonitorAlertEvent
//	err := c.ShouldBindJSON(&reqObj)
//	if err != nil {
//		sc.Logger.Error("解析新增集群执行请求失败", zap.Error(err))
//		common.FailWithMessage(err.Error(), c)
//		return
//	}
//
//	// 获取当前用户ID
//	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
//	dbUser, err := models.GetUserByUsername(userName)
//	if reqObj.CheckInstanceIpExists() {
//		msg := "ip和其他集群重复"
//		sc.Logger.Error(msg, zap.Error(err))
//		common.FailWithMessage(msg, c)
//		return
//	}
//	if err == nil && dbUser != nil {
//		reqObj.UserID = dbUser.ID
//	}
//
//	reqObj.FillDefaultData()
//	// 存入数据库
//	err = reqObj.CreateOne()
//	if err != nil {
//		sc.Logger.Error("新增集群执行数据库失败", zap.Error(err))
//		common.FailWithMessage("存入数据库失败: "+err.Error(), c)
//		return
//	}
//
//	common.OkWithMessage("创建成功", c)
//}

//func updateMonitorAlertEvent(c *gin.Context) {
//	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
//
//	// 🚀 致命修复：同上
//	var reqObj models.MonitorAlertEvent
//	err := c.ShouldBindJSON(&reqObj)
//	if err != nil {
//		sc.Logger.Error("解析更新集群请求失败", zap.Error(err))
//		common.FailWithMessage(err.Error(), c)
//		return
//	}
//	if reqObj.CheckInstanceIpExists() {
//		msg := "ip和其他集群重复"
//		sc.Logger.Error(msg, zap.Error(err))
//		common.FailWithMessage(msg, c)
//		return
//	}
//	// 检查是否存在
//	_, err = models.GetMonitorAlertEventById(int(reqObj.ID))
//	if err != nil {
//		common.FailWithMessage("集群不存在", c)
//		return
//	}
//
//	// 更新
//	err = reqObj.UpdateOne()
//	if err != nil {
//		sc.Logger.Error("更新集群执行错误", zap.Error(err))
//		common.FailWithMessage("更新失败: "+err.Error(), c)
//		return
//	}
//
//	common.OkWithMessage("更新成功", c)
//}

func deleteMonitorAlertEvent(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetMonitorAlertEventById(intVar)
	if err != nil {
		common.FailWithMessage("集群不存在", c)
		return
	}

	jobs, err := models.GetMonitorAlertManagerSendGroupByPoolId(uint(intVar))
	if err != nil {
		sc.Logger.Error("查询关联发送组失败", zap.Error(err))
		common.FailWithMessage("查询关联发送组失败: "+err.Error(), c)
		return
	}

	// 🚀 2. 核心修复：如果查出来的任务数量大于 0，说明有关联任务，绝对禁止删除！
	if len(jobs) > 0 {
		sc.Logger.Warn("该集群已绑定发送组，禁止直接删除！", zap.Any("集群ID", id))
		common.FailWithMessage("该集群下存在关联的发送组，禁止直接删除！请先转移或清理任务。", c)
		return
	}

	err = dbObj.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除集群执行错误", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

func actionMonitorAlertEventOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("集群动作", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetJobTaskById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找集群执行错误", zap.Any("集群执行", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	action := c.Query("action")
	nextStatus, exist := common.JOB_ACTION_NEXT_STATUS_MAP[action]
	if !exist {
		sc.Logger.Error("传入的动作错误", zap.Any("集群执行", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	if action == common.AGENT_TASK_ACTION_KILL {
		dbObj.Action = common.AGENT_TASK_ACTION_KILL
	}

	// ==========================================
	// 💡 补充记录集群流 到 ActualFlowData json
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
		common.AGENT_TASK_ACTION_PAUSE:  "手动暂停集群",
		common.AGENT_TASK_ACTION_RESUME: "恢复执行集群",
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

	sc.Logger.Info("集群动作", zap.Any("id", id), zap.Any("动作", action), zap.Any("nextStatus", nextStatus))

	dbObj.Status = nextStatus
	err = dbObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新集群执行错误", zap.Any("集群执行", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("更新成功", c)

}

// AMSilence 定义发送给 Alertmanager API 的 Silence 结构体
type AMSilence struct {
	Matchers  []AMMatcher `json:"matchers"`
	StartsAt  time.Time   `json:"startsAt"`
	EndsAt    time.Time   `json:"endsAt"`
	CreatedBy string      `json:"createdBy"`
	Comment   string      `json:"comment"`
}

type AMMatcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
}

type SilenceRequest struct {
	SilenceTime string `json:"silenceTime"`
	ByName      bool   `json:"byName"`
}

type SilenceResponse struct {
	SilenceID string `json:"silenceID"`
}

func AlertEventSilence(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj SilenceRequest
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析event屏蔽请求错误", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	id := c.Param("id")
	intId, _ := strconv.Atoi(id)
	byNameBool := reqObj.ByName
	sdr, err := model.ParseDuration(reqObj.SilenceTime)
	if err != nil {
		msg := fmt.Sprintf("解析屏蔽时间错误:%v", sdr)
		sc.Logger.Error(msg, zap.Error(err))
		common.FailWithMessage(msg, c)
		return
	}

	event, err := models.GetMonitorAlertEventById(intId)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("通过id去查询event错误%v", err), c)
		return
	}

	event.FillFrontAllData()
	if event.SendGroup == nil {
		common.ReqBadFailWithMessage("event的发送组为空,无法获取alertmanager", c)
		return
	}
	alm, err := models.GetMonitorAlertManagerPoolById(int(event.SendGroup.PoolId))
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("通过发送组找alertManger实例错误%v", err), c)
		return
	}
	if len(alm.AlertManagerInstances) == 0 {
		common.ReqBadFailWithMessage(fmt.Sprintf("未找到alertManger实例%v", err), c)
		return
	}

	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		sc.Logger.Error("通过token解析userName去数据库中找user失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("通过token解析userName去数据库中找user失败: %v", err), c)
		return
	}

	event.GenMapFromKvs()
	matchers := make([]AMMatcher, 0)
	if byNameBool {
		// 🚀 核心修复：直接使用 event 表里的 AlertName 字段，不再依赖标签里是否有这个 key
		if event.AlertName != "" {
			matchers = append(matchers, AMMatcher{
				Name:    common.MONITOR_ALERT_NAME_KEY, // 这里通常是 "alertname"
				Value:   event.AlertName,
				IsRegex: false,
				IsEqual: true,
			})
		}
	} else {
		// 按所有标签屏蔽的逻辑保持不变
		for k, v := range event.LabelsM {
			matchers = append(matchers, AMMatcher{
				Name:    k,
				Value:   v,
				IsRegex: false,
				IsEqual: true,
			})
		}
	}

	if len(matchers) == 0 {
		sc.Logger.Error("无法创建静默: 数据库中该 event 没有 Labels 数据", zap.String("id", id))
		common.ReqBadFailWithMessage("静默失败：找不到该告警的标签特征，无法屏蔽", c)
		return
	}

	now := time.Now()
	si := AMSilence{
		Matchers:  matchers,
		StartsAt:  now,
		EndsAt:    now.Add(time.Duration(sdr)),
		CreatedBy: dbUser.RealName,
		Comment:   fmt.Sprintf("由前端event页面创建的静默 事件id:%v 屏蔽时长:%v小时", event.ID, reqObj.SilenceTime),
	}

	jsonStr, err := json.Marshal(si)
	if err != nil {
		c.String(http.StatusInternalServerError, "")
		common.ReqBadFailWithMessage("内部错误: 序列化失败", c)
		return
	}

	// 3. 发送请求给 Alertmanager
	almAddr := fmt.Sprintf("http://%v:9093", alm.AlertManagerInstances[0])
	url := fmt.Sprintf("%s/%s", almAddr, "api/v2/silences")
	emptyMap := map[string]string{}

	bodyBytes, err := common.PostWithJsonString(sc.Logger, "AlertSilence",
		sc.HttpRequestGlobalTimeoutSeconds,
		url, string(jsonStr), emptyMap, emptyMap)

	if err != nil {
		sc.Logger.Error("告警静默调用alertmanager失败", zap.Error(err), zap.String("payload", string(jsonStr)))
		common.ReqBadFailWithMessage(fmt.Sprintf("调用 Alertmanager 接口失败%v", err.Error()), c)
		return
	}
	// 解析屏蔽结果
	var sr *SilenceResponse
	err = json.Unmarshal(bodyBytes, &sr)
	if err != nil {
		sc.Logger.Error("告警静默解析结果失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("告警静默解析结果失败%v", err), c)
		return
	}
	if sr != nil {
		event.SilenceID = sr.SilenceID
		event.Status = common.MONITOR_ALERT_STATUS_SILIENCED
		_ = event.UpdateOne()
	}

	// 屏蔽成功后异步发送飞书消息
	msg := fmt.Sprintf("时间:%v 用户:%v 动作:%v 告警:%v",
		common.TimeNowString, dbUser.RealName, fmt.Sprintf("屏蔽告警:%v", reqObj.SilenceTime), event.AlertName)
	go func() {
		event.SendImMessageToQunLiaoByEvent(msg, sc.ImC.FeiShu.Webhook, sc.Logger, sc.ImC.FeiShu.RequestTimeoutSeconds)
	}()

	common.OkWithMessage("静默成功", c)
}

//func AlertUnSilence(c *gin.Context) {
//	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.AlertWebhookConfig)
//	fingerprint := c.DefaultQuery("fingerprint", "")
//
//	event, err := models.GetMonitorAlertEventByFingerPrintId(fingerprint)
//	if err != nil {
//		c.String(http.StatusInternalServerError, fmt.Sprintf("通过fingerprint去查询event错误 %s", err.Error()))
//		return
//	}
//
//	url := fmt.Sprintf("%s/api/v2/silence/%s", sc.AlertManagerApi, event.SilenceID)
//	emptyMap := map[string]string{}
//	_, err = common.DeleteWithId(sc.Logger, "AlertSilence",
//		sc.HttpRequestGlobalTimeoutSeconds,
//		url, emptyMap, emptyMap)
//
//	if err != nil {
//		sc.Logger.Error("取消告警静默调用alertmanager失败", zap.Error(err), zap.String("url", url))
//		c.String(http.StatusBadRequest, fmt.Sprintf("调用 Alertmanager 接口失败 %v", err.Error()))
//		return
//	}
//
//	//c.String(http.StatusOK, "取消静默成功")
//	c.Header("Content-Type", "text/html; charset=utf-8")
//	c.String(http.StatusOK, `
//            <html>
//                <body style="text-align:center; padding-top:50px; font-family:sans-serif;">
//                    <h1>取消告警静默成功</h1>
//                    <p>此页面将在 3 秒后尝试自动关闭...</p>
//                    <script>
//                        setTimeout(function() {
//                            // 尝试关闭窗口
//                            window.opener = null;
//                            window.open('', '_self');
//                            window.close();
//                        }, 3000);
//                    </script>
//                </body>
//            </html>
//        `)
//	return
//
//}

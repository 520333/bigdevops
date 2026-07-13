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
	"github.com/prometheus/common/model"
	"go.uber.org/zap"
)

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

func alertEventReLing(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intId, _ := strconv.Atoi(id)

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

	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		sc.Logger.Error("通过token解析userName去数据库中找user失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("通过token解析userName去数据库中找user失败: %v", err), c)
		return
	}
	event.Status = common.MONITOR_ALERT_STATUS_RENLING
	event.ReLingUserId = dbUser.ID
	err = event.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新event错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("更新event错误: %v", err), c)
		return
	}

	// 认领成功后异步发送飞书消息
	msg := fmt.Sprintf("用户:%v  动作:%v 告警:%v 时间:%v",
		dbUser.RealName, "认领告警", event.AlertName, common.TimeNowString())
	go event.SendImMessageToQunLiaoByEvent(msg, sc.ImC.FeiShu.Webhook, sc.Logger, sc.ImC.FeiShu.RequestTimeoutSeconds)

	common.OkWithMessage("静默成功", c)
}

func doUnSilence(sc *config.ServerConfig, eventID int, user *models.User) error {
	event, err := models.GetMonitorAlertEventById(eventID)
	if err != nil {
		return fmt.Errorf("failed to fetch event (ID: %d): %v", eventID, err)
	}

	event.FillFrontAllData()
	if event.SendGroup == nil {
		return fmt.Errorf("event (ID: %d) has no send group", eventID)
	}
	if event.SilenceID == "" {
		return fmt.Errorf("event (ID: %d) has no silence ID", eventID)
	}

	url := fmt.Sprintf("%s/%s/%v", sc.AlertManagerApi, "api/v2/silence", event.SilenceID)
	emptyMap := map[string]string{}

	bodyBytes, err := common.DeleteWithId(sc.Logger, "AlertSilence",
		sc.HttpRequestGlobalTimeoutSeconds,
		url, emptyMap, emptyMap)

	if err != nil {
		sc.Logger.Error("Failed to call alertmanager unsilence", zap.Error(err), zap.String("out", string(bodyBytes)))
		return fmt.Errorf("alertmanager API call failed for event (ID: %d): %v", eventID, err)
	}

	now := time.Now()
	event.Status = common.MONITOR_ALERT_STATUS_FIRING
	event.UnsilencedAt = &now

	_ = event.UpdateOne()

	msg := fmt.Sprintf("用户:%v  动作:%v 告警:%v 时间:%v",
		user.RealName, "解除屏蔽", event.AlertName, common.TimeNowString())
	go event.SendImMessageToQunLiaoByEvent(msg, sc.ImC.FeiShu.Webhook, sc.Logger, sc.ImC.FeiShu.RequestTimeoutSeconds)

	return nil
}

// --- Helper for Silence ---
func doSilence(sc *config.ServerConfig, eventID int, timeString string, sdr time.Duration, byNameBool bool, user *models.User) error {
	event, err := models.GetMonitorAlertEventById(eventID)
	if err != nil {
		return fmt.Errorf("failed to fetch event (ID: %d): %v", eventID, err)
	}

	event.FillFrontAllData()
	if event.SendGroup == nil {
		return fmt.Errorf("event (ID: %d) has no send group", eventID)
	}

	alm, err := models.GetMonitorAlertManagerPoolById(int(event.SendGroup.PoolId))
	if err != nil || len(alm.AlertManagerInstances) == 0 {
		return fmt.Errorf("failed to find alertmanager instance for event (ID: %d)", eventID)
	}

	event.GenMapFromKvs()
	matchers := make([]AMMatcher, 0)
	if byNameBool {
		if event.AlertName != "" {
			matchers = append(matchers, AMMatcher{
				Name:    common.MONITOR_ALERT_NAME_KEY,
				Value:   event.AlertName,
				IsRegex: false,
				IsEqual: true,
			})
		}
	} else {
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
		return fmt.Errorf("event (ID: %d) has no valid matchers", eventID)
	}

	now := time.Now()
	si := AMSilence{
		Matchers:  matchers,
		StartsAt:  now,
		EndsAt:    now.Add(sdr),
		CreatedBy: user.RealName,
		Comment:   fmt.Sprintf("由前端创建的静默 事件id:%v 屏蔽时长:%v", event.ID, timeString),
	}

	jsonStr, err := json.Marshal(si)
	if err != nil {
		return fmt.Errorf("failed to serialize silence request (ID: %d): %v", eventID, err)
	}

	almAddr := fmt.Sprintf("http://%v:9093", alm.AlertManagerInstances[0])
	url := fmt.Sprintf("%s/%s", almAddr, "api/v2/silences")
	emptyMap := map[string]string{}

	bodyBytes, err := common.PostWithJsonString(sc.Logger, "AlertSilence",
		sc.HttpRequestGlobalTimeoutSeconds,
		url, string(jsonStr), emptyMap, emptyMap)

	if err != nil {
		return fmt.Errorf("alertmanager API call failed (ID: %d): %v", eventID, err)
	}

	var sr *SilenceResponse
	if err = json.Unmarshal(bodyBytes, &sr); err != nil {
		return fmt.Errorf("failed to parse alertmanager response (ID: %d): %v", eventID, err)
	}

	if sr != nil {
		event.SilenceID = sr.SilenceID
		event.Status = common.MONITOR_ALERT_STATUS_SILIENCED
		_ = event.UpdateOne()
	}

	msg := fmt.Sprintf("用户:%v  动作:%v 告警:%v 时间:%v",
		user.RealName, fmt.Sprintf("屏蔽告警 时长:%v", timeString), event.AlertName, common.TimeNowString())
	go event.SendImMessageToQunLiaoByEvent(msg, sc.ImC.FeiShu.Webhook, sc.Logger, sc.ImC.FeiShu.RequestTimeoutSeconds)

	return nil
}

type BatchSilenceRequest struct {
	EventIDs    []int  `json:"eventIds" binding:"required"`
	SilenceTime string `json:"silenceTime" binding:"required"`
	ByName      bool   `json:"byName"`
}

type BatchUnSilenceRequest struct {
	EventIDs []int `json:"eventIds" binding:"required"`
}

// alertEventBatchSilence handles silencing multiple alerts at once
func alertEventBatchSilence(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj BatchSilenceRequest
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}

	if len(reqObj.EventIDs) == 0 {
		common.FailWithMessage("eventIds cannot be empty", c)
		return
	}

	sdr, err := model.ParseDuration(reqObj.SilenceTime)
	if err != nil {
		common.FailWithMessage(fmt.Sprintf("invalid silence time: %v", err), c)
		return
	}

	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("user not found: %v", err), c)
		return
	}

	var failedIDs []string
	for _, id := range reqObj.EventIDs {
		err := doSilence(sc, id, reqObj.SilenceTime, time.Duration(sdr), reqObj.ByName, dbUser)
		if err != nil {
			sc.Logger.Error("Batch silence failed for event", zap.Int("id", id), zap.Error(err))
			failedIDs = append(failedIDs, fmt.Sprintf("%d", id))
		}
	}

	if len(failedIDs) > 0 {
		common.OkWithMessage(fmt.Sprintf("部分屏蔽成功。失败ID: %s", strings.Join(failedIDs, ",")), c)
		return
	}

	common.OkWithMessage("批量静默成功", c)
}

// alertEventBatchUnSilence handles removing silences for multiple alerts
func alertEventBatchUnSilence(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj BatchUnSilenceRequest
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}

	if len(reqObj.EventIDs) == 0 {
		common.FailWithMessage("eventIds cannot be empty", c)
		return
	}

	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("user not found: %v", err), c)
		return
	}

	var failedIDs []string
	for _, id := range reqObj.EventIDs {
		err := doUnSilence(sc, id, dbUser)
		if err != nil {
			sc.Logger.Error("Batch unsilence failed for event", zap.Int("id", id), zap.Error(err))
			failedIDs = append(failedIDs, fmt.Sprintf("%d", id))
		}
	}

	if len(failedIDs) > 0 {
		common.OkWithMessage(fmt.Sprintf("部分解除屏蔽成功。失败ID: %s", strings.Join(failedIDs, ",")), c)
		return
	}

	common.OkWithMessage("批量解除屏蔽成功", c)
}

func alertEventSilence(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj SilenceRequest
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}

	id := c.Param("id")
	intId, _ := strconv.Atoi(id)

	sdr, err := model.ParseDuration(reqObj.SilenceTime)
	if err != nil {
		common.FailWithMessage(fmt.Sprintf("invalid silence time: %v", err), c)
		return
	}

	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("user not found: %v", err), c)
		return
	}

	err = doSilence(sc, intId, reqObj.SilenceTime, time.Duration(sdr), reqObj.ByName, dbUser)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("静默成功", c)
}

func alertEventUnSilence(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	id := c.Param("id")
	intId, _ := strconv.Atoi(id)

	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("user not found: %v", err), c)
		return
	}

	err = doUnSilence(sc, intId, dbUser)
	if err != nil {
		common.ReqBadFailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("解除屏蔽成功", c)
}

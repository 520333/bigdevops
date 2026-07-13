package cron

import (
	"bigdevops/src/common"
	"bigdevops/src/models"
	"context"
	"strconv"

	"github.com/prometheus/alertmanager/template"
	"go.uber.org/zap"
)

func (ac *AlertCache) AlertReceiveConsumerManager(ctx context.Context) error {

	for {
		select {
		case <-ctx.Done():
			ac.Sc.Logger.Info("AlertReceiveConsumerManager 收到其他任务退出信号 退出")
			return nil
		case alert := <-ac.AlertReceiveQ:
			go ac.DealWithOneAlertReceive(alert)
		}
	}
}

func (ac *AlertCache) DealWithOneAlertReceive(alert template.Alert) {
	sendGroupId, exists := alert.Labels[common.MONITOR_ALERT_MATCH_KEY]
	if !exists {
		ac.Sc.Logger.Info("alert消费者收到告警信息,sendGroup不存在",
			zap.Any("告警", alert))
		return
	}

	ruleId, exists := alert.Labels[common.MONITOR_ALERT_RULE_KEY]
	if !exists {
		ac.Sc.Logger.Info("alert消费者收到告警信息,ruleId不存在",
			zap.Any("告警", alert))
		return
	}

	ruleIdInt, _ := strconv.Atoi(ruleId)
	sendGroupIdInt, _ := strconv.Atoi(sendGroupId)

	sendGroup := ac.GetSendGroupById(sendGroupId)
	if sendGroup == nil {
		ac.Sc.Logger.Info("alert消费者收到告警信息,根据sendGroupId去缓存中查询结果不存在",
			zap.Any("sengGroupId", sendGroupId), zap.Any("告警", alert))
		return
	}

	ac.Sc.Logger.Info("alert消费者收到告警信息", zap.Any("sengGroupId", sendGroupId), zap.Any("告警", alert))

	createUser := ac.GetUserById(sendGroup.UserID)
	ac.Sc.Logger.Info("alert消费者收到告警信息", zap.Any("createUser", createUser), zap.Any("告警", alert))

	rule := ac.GetRuleById(ruleId)
	ac.Sc.Logger.Info("alert消费者收到告警信息",
		zap.Any("告警", alert), zap.Any("开始时间", alert.StartsAt), zap.Any("结束时间", alert.EndsAt),
	)
	//// 判断是否已升级
	//upgredeNeed := false
	//if alert.Status == common.MONITOR_ALERT_STATUS_FIRING && sendGroup.FirstUpgradeUsers != nil && len(sendGroup.FirstUpgradeUsers) > 0 {
	//	upgredeNeed = true
	//}
	//status := alert.Status
	//if upgredeNeed {
	//	status = common.MONITOR_ALERT_STATUS_UPGRADED
	//}

	event := &models.MonitorAlertEvent{
		AlertName:   alert.Labels[common.MONITOR_ALERT_NAME_KEY],
		FingerPrint: alert.Fingerprint,
		Status:      alert.Status,
		RuleId:      uint(ruleIdInt),
		Labels:      common.GentStringArrayByMap(alert.Labels),
		SendGroupId: uint(sendGroupIdInt),
	}
	err := event.UpdateOrCreateOne()
	if err != nil {
		ac.Sc.Logger.Error("保存alert到event出错", zap.Error(err), zap.Any("event", rule), zap.Any("告警", alert))
	}
	//event.FillFrontAllData()
	// 处理告警信息
	ac.GenerateFeiShuCardMsgOneAlert(alert, event, rule, sendGroup)

}

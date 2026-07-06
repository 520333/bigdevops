package cron

import (
	"bigdevops/src/common"
	"context"

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
	sendGroup := ac.GetSendGroupById(sendGroupId)
	if sendGroup == nil {
		ac.Sc.Logger.Info("alert消费者收到告警信息,根据sendGroupId去缓存中查询结果不存在",
			zap.Any("sengGroupId", sendGroupId), zap.Any("告警", alert))
		return
	}

	ac.Sc.Logger.Info("alert消费者收到告警信息", zap.Any("sengGroupId", sendGroupId), zap.Any("告警", alert))

	createUser := ac.GetUserById(sendGroup.UserID)
	ac.Sc.Logger.Info("alert消费者收到告警信息", zap.Any("createUser", createUser), zap.Any("告警", alert))

	//onondutyGroup := ac.GetOnDutyGroupById(sendGroup.UserID)
	//ac.Sc.Logger.Info("alert消费者收到告警信息", zap.Any("onondutyGroup", onondutyGroup), zap.Any("告警", alert))

}

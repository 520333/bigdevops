package view_alertwebhook

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/alertmanager/notify/webhook"
	"github.com/prometheus/alertmanager/template"
	"go.uber.org/zap"
)

func AlertReceive(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.AlertWebhookConfig)
	alertReceiveQ := c.MustGet(common.GIN_CTX_CONFIG_ALERTRECEIVEQ).(chan template.Alert)
	var msg webhook.Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		msg := fmt.Sprintf("解析alertmanager传过来的alert-json错误:%v", err.Error())
		sc.Logger.Error(msg, zap.Error(err))
		common.FailWithMessage(msg, c)
		return
	}
	//baseMsg := fmt.Sprintf("[状态:%s][报警条数:%d]", msg.Status, len(msg.Alerts))
	sc.Logger.Info("接收alertmanager告警打印", zap.Any("状态", msg.Status), zap.Any("条数", msg.Alerts))

	for i := 0; i < len(msg.Alerts); i++ {
		alert := msg.Alerts[i]
		sc.Logger.Debug("接收alertmanager告警详情",
			zap.Any(fmt.Sprintf("receive:%v[%d/%d]", msg.Receiver, i+1, len(msg.Alerts)), alert))
		alertReceiveQ <- alert
	}
	c.JSON(200, "ok")
}

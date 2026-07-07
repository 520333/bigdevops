package cron

import (
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"sync"
	"time"

	"github.com/prometheus/alertmanager/template"
	"k8s.io/apimachinery/pkg/util/wait"
)

type AlertCache struct {
	AlertReceiveQ            chan template.Alert
	Sc                       *config.AlertWebhookConfig
	SendGroupMap             map[string]*models.MonitorAlertManagerSendGroup
	UserMap                  map[uint]*models.User
	MonitorOnDutyGroupMap    map[uint]*models.MonitorOndutyGroup
	MonitorPromAlertRuleMap  map[string]*models.MonitorPromAlertRule
	cacheHasSynced           chan struct{}
	StartFinishRenew         bool
	SendGroupLock            sync.RWMutex
	UserLock                 sync.RWMutex
	OnDutyGroupLock          sync.RWMutex
	MonitorPromAlertRuleLock sync.RWMutex
}

func NewAlertCache(sc *config.AlertWebhookConfig, alertReceiveQ chan template.Alert, cacheHasSynced chan struct{}) *AlertCache {
	ac := &AlertCache{
		AlertReceiveQ:           alertReceiveQ,
		Sc:                      sc,
		cacheHasSynced:          cacheHasSynced,
		SendGroupMap:            make(map[string]*models.MonitorAlertManagerSendGroup),
		UserMap:                 make(map[uint]*models.User),
		MonitorOnDutyGroupMap:   make(map[uint]*models.MonitorOndutyGroup),
		MonitorPromAlertRuleMap: make(map[string]*models.MonitorPromAlertRule),
	}
	return ac
}

func (ac *AlertCache) RenewMapManager(ctx context.Context) error {
	go wait.UntilWithContext(ctx, ac.RenewMapSendGroup, time.Duration(ac.Sc.CommonMapRenewIntervalSeconds)*time.Second)
	go wait.UntilWithContext(ctx, ac.RenewMapUser, time.Duration(ac.Sc.CommonMapRenewIntervalSeconds)*time.Second)
	go wait.UntilWithContext(ctx, ac.RenewMapOnDutyGroup, time.Duration(ac.Sc.CommonMapRenewIntervalSeconds)*time.Second)
	go wait.UntilWithContext(ctx, ac.RenewMapRule, time.Duration(ac.Sc.CommonMapRenewIntervalSeconds)*time.Second)
	<-ctx.Done()
	ac.Sc.Logger.Info("RenewMapManager 收到其他任务退出信号 退出")
	return nil
}

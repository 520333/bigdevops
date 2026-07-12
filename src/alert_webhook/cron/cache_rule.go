package cron

import (
	"bigdevops/src/models"
	"context"
	"fmt"

	"go.uber.org/zap"
)

func (ac *AlertCache) RenewMapRule(ctx context.Context) {
	defer func() {
		if !ac.StartFinishRenew {
			close(ac.cacheHasSynced)
			ac.StartFinishRenew = true
		}
	}()
	rules, err := models.GetMonitorAlertRuleAll()
	if err != nil {
		ac.Sc.Logger.Error("[缓存刷新模块]扫描数据库中的rules失败", zap.Error(err))
		return
	}
	if len(rules) == 0 {
		return
	}

	tmpM := map[string]*models.MonitorAlertRule{}
	for _, rule := range rules {
		rule := rule
		tmpM[fmt.Sprintf("%d", rule.ID)] = rule
	}
	ac.MonitorPromAlertRuleLock.Lock()
	lastNum := len(ac.MonitorPromAlertRuleMap)
	ac.MonitorPromAlertRuleMap = tmpM
	thiNum := len(ac.MonitorPromAlertRuleMap)

	ac.MonitorPromAlertRuleLock.Unlock()

	ac.Sc.Logger.Info("RenewMapRule 刷新缓存结果...",
		zap.Any("上一次数量", lastNum), zap.Any("这一次数量", thiNum),
	)
}
func (ac *AlertCache) GetRuleById(id string) *models.MonitorAlertRule {
	ac.MonitorPromAlertRuleLock.RLock()
	defer ac.MonitorPromAlertRuleLock.RUnlock()
	return ac.MonitorPromAlertRuleMap[id]
}

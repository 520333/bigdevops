package cron

import (
	"bigdevops/src/models"
	"context"

	"go.uber.org/zap"
)

func (ac *AlertCache) RenewMapOnDutyGroup(ctx context.Context) {
	defer func() {
		if !ac.StartFinishRenew {
			close(ac.cacheHasSynced)
			ac.StartFinishRenew = true
		}
	}()
	onDutyGroups, err := models.GetMonitorOndutyGroupAll()
	if err != nil {
		ac.Sc.Logger.Error("[缓存刷新模块]扫描数据库中的onDutyGroups失败", zap.Error(err))
		return
	}
	if len(onDutyGroups) == 0 {
		return
	}

	tmpM := map[uint]*models.MonitorOndutyGroup{}
	for _, onDutyGroup := range onDutyGroups {
		onDutyGroup := onDutyGroup
		tmpM[onDutyGroup.ID] = onDutyGroup

	}
	ac.UserLock.Lock()
	lastNum := len(ac.UserMap)
	ac.MonitorOnDutyGroupMap = tmpM
	thiNum := len(ac.UserMap)

	ac.UserLock.Unlock()

	ac.Sc.Logger.Info("RenewMapOnDutyGroup 刷新缓存结果...",
		zap.Any("上一次数量", lastNum), zap.Any("这一次数量", thiNum),
	)
}
func (ac *AlertCache) GetOnDutyGroupById(id uint) *models.MonitorOndutyGroup {
	ac.OnDutyGroupLock.RLock()
	defer ac.OnDutyGroupLock.RUnlock()
	return ac.MonitorOnDutyGroupMap[id]
}

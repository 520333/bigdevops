package cron

import (
	"bigdevops/src/models"
	"context"
	"fmt"

	"go.uber.org/zap"
)

func (ac *AlertCache) RenewMapSendGroup(ctx context.Context) {
	defer func() {
		if !ac.StartFinishRenew {
			close(ac.cacheHasSynced)
			ac.StartFinishRenew = true
		}
	}()
	sendGroups, err := models.GetMonitorAlertManagerSendGroupAll()
	if err != nil {
		ac.Sc.Logger.Error("[缓存刷新模块]扫描数据库中的sendGroups失败", zap.Error(err))
		return
	}
	if len(sendGroups) == 0 {
		return
	}

	tmpM := map[string]*models.MonitorAlertManagerSendGroup{}
	for _, sendGroup := range sendGroups {
		sendGroup := sendGroup
		tmpM[fmt.Sprintf("%d", sendGroup.ID)] = sendGroup

	}
	//time.Sleep(30 * time.Second)
	ac.SendGroupLock.Lock()
	lastNum := len(ac.SendGroupMap)
	ac.SendGroupMap = tmpM
	thiNum := len(ac.SendGroupMap)

	if !ac.StartFinishRenew {
		close(ac.cacheHasSynced)
		ac.StartFinishRenew = true
	}
	ac.SendGroupLock.Unlock()

	ac.Sc.Logger.Info("RenewMapSendGroup 刷新缓存结果...",
		zap.Any("上一次数量", lastNum), zap.Any("这一次数量", thiNum),
	)

}

func (ac *AlertCache) GetSendGroupById(id string) *models.MonitorAlertManagerSendGroup {
	ac.SendGroupLock.RLock()
	defer ac.SendGroupLock.RUnlock()
	return ac.SendGroupMap[id]
}

package cron

import (
	"bigdevops/src/models"
	"context"

	"go.uber.org/zap"
)

func (ac *AlertCache) RenewMapUser(ctx context.Context) {
	defer func() {
		if !ac.StartFinishRenew {
			close(ac.cacheHasSynced)
			ac.StartFinishRenew = true
		}
	}()
	users, err := models.GetUserAll()
	if err != nil {
		ac.Sc.Logger.Error("[缓存刷新模块]扫描数据库中的users失败", zap.Error(err))
		return
	}
	if len(users) == 0 {
		return
	}

	tmpM := map[uint]*models.User{}
	for _, user := range users {
		user := user
		tmpM[user.ID] = user

	}
	ac.UserLock.Lock()
	lastNum := len(ac.UserMap)
	ac.UserMap = tmpM
	thiNum := len(ac.UserMap)

	ac.UserLock.Unlock()

	ac.Sc.Logger.Info("RenewMapUser 刷新缓存结果...",
		zap.Any("上一次数量", lastNum), zap.Any("这一次数量", thiNum),
	)
}
func (ac *AlertCache) GetUserById(id uint) *models.User {
	ac.UserLock.RLock()
	defer ac.UserLock.RUnlock()
	return ac.UserMap[id]
}

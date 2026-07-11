package cron

import (
	"bigdevops/src/common"
	"bigdevops/src/models"
	"context"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (cm *CronManager) FullFillOnDutyHistoryManager(ctx context.Context) error {
	go wait.UntilWithContext(ctx, cm.FullFillOnDutyHistory, 5*time.Minute)
	<-ctx.Done()
	cm.Sc.Logger.Info("FullFillOnDutyHistoryManager收到其他任务退出信号")
	return nil
}

func (cm *CronManager) FullFillOnDutyHistory(ctx context.Context) {
	groups, err := models.GetMonitorOndutyGroupAll()
	if err != nil {
		cm.Sc.Logger.Error("[值班组历史]扫描数据中的值班组失败", zap.Error(err))
		return
	}
	for _, group := range groups {
		group := group
		go cm.FullFillOnDutyHistoryOnGroup(group)

	}
}

// FullFillOnDutyHistoryOnGroup 每天凌晨第一次执行
func (cm *CronManager) FullFillOnDutyHistoryOnGroup(group *models.MonitorOndutyGroup) {
	toDayString := time.Now().Format("2006-01-02")
	// 判断是否已经设置过了
	dbHistoryToday, _ := models.GetMonitorOnDutyHistoryByOnDutyGroupIdAndDay(group.ID, toDayString)
	if dbHistoryToday.OndutyUserId > 0 {
		return
	}
	// 查换班表
	dbChange, _ := models.GetMonitorOndutyChangeByOnDutyGroupIdAndDay(group.ID, toDayString)
	if dbChange.ID > 0 {
		// 说明有人之前换班了
		history := models.MonitorOndutyHistory{
			OndutyGroupId: group.ID,
			DateString:    toDayString,
			OndutyUserId:  dbChange.OndutyUserId,
			OriginUserId:  dbChange.OriginUserId,
		}
		err := history.CreateOne()
		if err != nil {
			cm.Sc.Logger.Error("创建值班历史记录失败", zap.Error(err), zap.Any("值班组", group.Name), zap.Any("日期", toDayString))
			return
		}
		return
	}

	yesterday := common.GetDayAgoDate(-1)
	dbHistoryYesterday, _ := models.GetMonitorOnDutyHistoryByOnDutyGroupIdAndDay(group.ID, yesterday)
	// 如果昨天的人是换班的 那么就用他之前的人
	if dbHistoryYesterday.OriginUserId > 0 {
		dbHistoryYesterday.OndutyUserId = dbHistoryYesterday.OriginUserId
	}
	// 判断是否需要换班
	startDay := common.GetDayAgoDate(-1 * group.ShiftDays)
	historys, err := models.GetMonitorOnDutyHistoryByOnDutyGroupIdAndTimeRange(int(group.ID), startDay, yesterday)
	if err != nil {
		cm.Sc.Logger.Error("查询值班历史记录失败", zap.Error(err), zap.Any("值班组", group.Name), zap.Any("日期", toDayString))
		return
	}
	needChange := false
	yijingzhibantianshu := 0
	for _, history := range historys {
		history := history
		if history.OndutyUserId == dbHistoryToday.OndutyUserId {
			yijingzhibantianshu++
		}
	}
	if yijingzhibantianshu == group.ShiftDays {
		needChange = true
	}

	var onDutyUserId uint
	if dbHistoryYesterday.ID == 0 {
		onDutyUserId = group.Members[0].ID
	} else {
		if needChange {
			// 如果需要换班找昨天值班人的下一位
			targetIndex := 0
			for index, user := range group.Members {
				index := index
				user := user

				if user.ID == dbHistoryYesterday.OndutyUserId {
					targetIndex = index + 1
					break
				}
			}
			if targetIndex == len(group.Members) {
				targetIndex = len(group.Members) - 1
			}
			onDutyUserId = group.Members[targetIndex].ID
		} else {
			// 不需要换班，继续让昨天的人值班
			onDutyUserId = dbHistoryYesterday.OndutyUserId
		}

	}

	history := models.MonitorOndutyHistory{
		OndutyGroupId: group.ID,
		DateString:    toDayString,
		OndutyUserId:  onDutyUserId,
	}
	err = history.CreateOne()

	if err != nil {
		cm.Sc.Logger.Error("创建值班历史记录失败", zap.Error(err), zap.Any("值班组", group.Name), zap.Any("日期", toDayString))
		return
	}
	return
}

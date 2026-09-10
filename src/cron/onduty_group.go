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
	if group == nil || len(group.Members) == 0 {
		return
	}

	shiftDays := group.ShiftDays
	if shiftDays <= 0 {
		shiftDays = 1
	}

	toDayString := time.Now().Format("2006-01-02")
	// 判断是否已经设置过了
	dbHistoryToday, _ := models.GetMonitorOnDutyHistoryByOnDutyGroupIdAndDay(group.ID, toDayString)
	if dbHistoryToday != nil && dbHistoryToday.OndutyUserId > 0 {
		return
	}

	// 查换班表
	dbChange, _ := models.GetMonitorOndutyChangeByOnDutyGroupIdAndDay(group.ID, toDayString)
	if dbChange != nil && dbChange.ID > 0 {
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

	var onDutyUserId uint
	if dbHistoryYesterday == nil || dbHistoryYesterday.ID == 0 {
		// 昨天没有记录，使用兜底推算或从第一位成员开始
		group.FillToDayOndutyUser()
		if group.ToDayOnDutyUser != nil && group.ToDayOnDutyUser.ID > 0 {
			onDutyUserId = group.ToDayOnDutyUser.ID
		} else {
			onDutyUserId = group.Members[0].ID
		}
	} else {
		// 如果昨天的人是临时顶替的，轮转基准应当以原定人员为准
		yesterdayRealUser := dbHistoryYesterday.OndutyUserId
		if dbHistoryYesterday.OriginUserId > 0 {
			yesterdayRealUser = dbHistoryYesterday.OriginUserId
		}

		// 判断是否需要换班
		startDay := common.GetDayAgoDate(-1 * shiftDays)
		historys, err := models.GetMonitorOnDutyHistoryByOnDutyGroupIdAndTimeRange(int(group.ID), startDay, yesterday)
		if err != nil {
			cm.Sc.Logger.Error("查询值班历史记录失败", zap.Error(err), zap.Any("值班组", group.Name), zap.Any("日期", toDayString))
			return
		}

		yijingzhibantianshu := 0
		for _, history := range historys {
			historyUserId := history.OndutyUserId
			if history.OriginUserId > 0 {
				historyUserId = history.OriginUserId
			}
			if historyUserId == yesterdayRealUser {
				yijingzhibantianshu++
			}
		}

		needChange := (yijingzhibantianshu >= shiftDays)
		if needChange {
			// 如果需要换班找昨天值班人的下一位（循环取模）
			currentIndex := 0
			for index, user := range group.Members {
				if user.ID == yesterdayRealUser {
					currentIndex = index
					break
				}
			}
			targetIndex := (currentIndex + 1) % len(group.Members)
			onDutyUserId = group.Members[targetIndex].ID
		} else {
			// 不需要换班，继续让昨天的人值班
			onDutyUserId = yesterdayRealUser
		}
	}

	history := models.MonitorOndutyHistory{
		OndutyGroupId: group.ID,
		DateString:    toDayString,
		OndutyUserId:  onDutyUserId,
	}
	err := history.CreateOne()
	if err != nil {
		cm.Sc.Logger.Error("创建值班历史记录失败", zap.Error(err), zap.Any("值班组", group.Name), zap.Any("日期", toDayString))
		return
	}
	return
}

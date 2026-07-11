package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorOndutyHistory 发送任务Job对象

type MonitorOndutyHistory struct {
	Model
	//Name string `json:"name,omitempty" validate:"required,min=1,max=50" gorm:"uniqueIndex;type:varchar(100);comment:历史"`

	OndutyGroupId uint `json:"ondutyGroupId" gorm:"uniqueIndex:group_id_date;comment:名称"`

	DateString   string `json:"dateString" gorm:"uniqueIndex:group_id_date;type:varchar(50);comment:哪一天"`
	OndutyUserId uint   `json:"onDutyUserId" gorm:"comment:谁值班"`
	OriginUserId uint   `json:"originUserId" gorm:"comment:原来谁在值班"`

	Key            string `json:"key,omitempty" gorm:"-"` // 前端表格使用
	PoolName       string `json:"poolName,omitempty" gorm:"-"`
	CreateUserName string `json:"createUserName,omitempty" gorm:"-"`
}

func (obj *MonitorOndutyHistory) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorOndutyHistory) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorOndutyHistory) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *MonitorOndutyHistory) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func GetMonitorOnDutyHistoryById(id int) (*MonitorOndutyHistory, error) {
	var dbObj MonitorOndutyHistory
	err := Db.Where("id = ? ", id).First(&dbObj).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorScrapePool不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbObj, nil
}

func GetMonitorOnDutyHistoryByPoolId(poolId uint) (ps []*MonitorOndutyHistory, err error) {
	err = Db.Where("enable = 1 AND pool_id = ? ", poolId).Find(&ps).Error
	return
}

func GetMonitorOnDutyHistoryAll() (ps []*MonitorOndutyHistory, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *MonitorOndutyHistory) FillFrontAllData() {

	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetMonitorOnDutyHistoryByOnDutyGroupIdAndTimeRange(onDutyGroupId int, startDay, endDay string) (objs []*MonitorOndutyHistory, err error) {
	err = Db.Where("onduty_group_id = ? AND date_string >= ? AND date_string <= ?", onDutyGroupId, startDay, endDay).Find(&objs).Error
	return
}

func GetMonitorOnDutyHistoryByOnDutyGroupIdAndDay(onDutyGroupId uint, dateString string) (obj *MonitorOndutyHistory, err error) {
	err = Db.Where("onduty_group_id = ? AND date_string = ?", onDutyGroupId, dateString).First(&obj).Error
	return
}
func GetMonitorOnDutyHistoryByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorOndutyHistory, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

// UpdateEnable 更新发送任务的开关状态
func (obj *MonitorOndutyHistory) UpdateEnable() error {
	// 推荐使用 Select 显式指定更新 enable 字段，这样既安全又能避免潜在的零值过滤问题
	return Db.Model(obj).Select("Enable").Updates(obj).Error
}

//// SetOnDutyGroupStatus 快捷更新开启状态
//func SetOnDutyGroupStatus(id uint, enable int) error {
//	// 假设你的全局数据库对象是 global.DB 或 common.DB，请根据你的项目实际情况调整
//	err := Db.Model(&MonitorOnDutyHistory{}).Where("id = ?", id).Update("enable", enable).Error
//	return err
//}

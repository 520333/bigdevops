package models

import (
	"fmt"

	"gorm.io/gorm/clause"
)

// MonitorOndutyChange 发送任务Job对象

type MonitorOndutyChange struct {
	Model
	Remark string `json:"name,omitempty" validate:"required,min=1,max=50" gorm:"type:varchar(100);comment:换班说明"`

	OndutyGroupId uint
	UserId        uint

	DateString   string `json:"dateString" gorm:"comment:计划哪一天"`
	OndutyUserId uint   `json:"onDutyUserId" gorm:"comment:谁值班"`
	OriginUserId uint   `json:"originUserId" gorm:"comment:原来谁值班"`

	// 前端请求 2个username
	TargetUserName string `json:"targetUserName" gorm:"-"`
	OriginUserName string `json:"originUserName" gorm:"-"`

	Key            string `json:"key,omitempty" gorm:"-"` // 前端表格使用
	PoolName       string `json:"poolName,omitempty" gorm:"-"`
	CreateUserName string `json:"createUserName,omitempty" gorm:"-"`
}

func (obj *MonitorOndutyChange) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorOndutyChange) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorOndutyChange) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *MonitorOndutyChange) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func (obj *MonitorOndutyChange) FillFrontAllData() {

	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetMonitorOndutyChangeByOnDutyGroupIdAndDay(onDutyGroupId uint, dateString string) (obj *MonitorOndutyChange, err error) {
	err = Db.Where("onduty_group_id = ? AND date_string = ?", onDutyGroupId, dateString).
		Order("id desc").
		Limit(1).
		Find(&obj).Error
	return
}

// UpdateEnable 更新发送任务的开关状态
func (obj *MonitorOndutyChange) UpdateEnable() error {
	// 推荐使用 Select 显式指定更新 enable 字段，这样既安全又能避免潜在的零值过滤问题
	return Db.Model(obj).Select("Enable").Updates(obj).Error
}

package models

import (
	"bigdevops/src/common"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorOndutyGroup 值班组结构体

type MonitorOndutyGroup struct {
	Model
	Name string `json:"name,omitempty" validate:"required,min=1,max=50" gorm:"uniqueIndex;type:varchar(100);comment:发送任务名称"`

	UserID uint

	// 发送逻辑
	Members      []*User  `json:"members" gorm:"many2many:monitor_onduty_users;comment:值班人列表"`
	UserNames    []string `json:"userNames" gorm:"-"` // 前端使用
	ShiftDays    int      `json:"shiftDays" gorm:"comment:轮班周期：天、周"`
	ImRobotToken string   `json:"imRobotToken" gorm:"comment:im机器人token 对应哪个群组"`

	ToDayOnDutyUser *User `json:"toDayOnDutyUser" gorm:"-"` // 当天值班人

	Key            string `json:"key,omitempty" gorm:"-"` // 前端表格使用
	CreateUserName string `json:"createUserName,omitempty" gorm:"-"`
}

func (obj *MonitorOndutyGroup) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorOndutyGroup) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorOndutyGroup) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *MonitorOndutyGroup) FillToDayOndutyUser() {
	toDayString := common.GetDayAgoDate(0)
	dbHistoryToday, _ := GetMonitorOnDutyHistoryByOnDutyGroupIdAndDay(obj.ID, toDayString)
	if dbHistoryToday.OndutyUserId > 0 {
		user, _ := GetUserById(int(dbHistoryToday.OndutyUserId))
		if user.ID > 0 {
			obj.ToDayOnDutyUser = user
		}
	} else {
		obj.ToDayOnDutyUser = obj.Members[0]
	}
}

//func (obj *MonitorOndutyGroup) UpdateOne() error {
//	return Db.Where("id = ?", obj.ID).Updates(obj).Error
//}

func (obj *MonitorOndutyGroup) UpdateMembers() error {
	return Db.Model(obj).Association("Members").Replace(obj.Members)
}

func (obj *MonitorOndutyGroup) UpdateOne() error {
	return Db.Transaction(func(tx *gorm.DB) error {
		// 1. 先更新主表的基础字段 (Name, ShiftDays 等)
		if err := tx.Model(obj).Updates(obj).Error; err != nil {
			return err
		}

		// 2. 关键点：显式同步 Members 关联
		// Replace 会删除中间表中不存在于当前 obj.Members 列表的记录，并添加新的记录
		if err := tx.Model(obj).Association("Members").Replace(obj.Members); err != nil {
			return err
		}

		return nil
	})
}

func GetMonitorOndutyGroupById(id int) (*MonitorOndutyGroup, error) {
	var dbObj MonitorOndutyGroup
	err := Db.Where("id = ? ", id).Preload("Members").First(&dbObj).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorOndutyGroup不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbObj, nil
}

func GetMonitorSendGroupByPoolId(poolId uint) (ps []*MonitorOndutyGroup, err error) {
	err = Db.Where("enable = 1 AND pool_id = ? ", poolId).Find(&ps).Error
	return
}

func GetMonitorOndutyGroupAll() (ps []*MonitorOndutyGroup, err error) {
	err = Db.Preload("Members").Find(&ps).Error
	return
}

func (obj *MonitorOndutyGroup) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}

	var userNames []string
	for _, user := range obj.Members {
		user := user
		userNames = append(userNames, user.Username)
	}
	obj.UserNames = userNames
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetMonitorOndutyGroupByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorOndutyGroup, err error) {
	err = Db.Preload("Members").Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

// UpdateEnable 更新发送任务的开关状态
func (obj *MonitorOndutyGroup) UpdateEnable() error {
	// 推荐使用 Select 显式指定更新 enable 字段，这样既安全又能避免潜在的零值过滤问题
	return Db.Model(obj).Select("Enable").Updates(obj).Error
}

//// SetOnDutyGroupStatus 快捷更新开启状态
//func SetOnDutyGroupStatus(id uint, enable int) error {
//	// 假设你的全局数据库对象是 global.DB 或 common.DB，请根据你的项目实际情况调整
//	err := Db.Model(&MonitorOndutyGroup{}).Where("id = ?", id).Update("enable", enable).Error
//	return err
//}

package models

import (
	"bigdevops/src/common"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorOndutyGroup 发送任务Job对象

type MonitorOndutyGroup struct {
	Model
	Name string `json:"name,omitempty" validate:"required,min=1,max=50" gorm:"uniqueIndex;type:varchar(100);comment:发送任务名称"`

	UserID uint

	// 发送逻辑
	Members      []*User `json:"members" gorm:"many2many:monitor_onduty_users;comment:值班人列表"`
	ShiftDays    string  `json:"shiftDays" gorm:"comment:轮班周期：天、周"`
	ImRobotToken string  `json:"imRobotToken" gorm:"comment:im机器人token 对应哪个群组"`

	ToDayOnDutyUser *User `json:"toDayOnDutyUser" gorm:"-"` // 当天值班人

	Key            string `json:"key,omitempty" gorm:"-"` // 前端表格使用
	PoolName       string `json:"poolName,omitempty" gorm:"-"`
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
		user, err := GetUserById(int(dbHistoryToday.OndutyUserId))
		if err != nil {
			obj.ToDayOnDutyUser = user
		}
	} else {
		obj.ToDayOnDutyUser = obj.Members[0]
	}
}

func (obj *MonitorOndutyGroup) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
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

func GetMonitorOndutyGroupByPoolId(poolId uint) (ps []*MonitorOndutyGroup, err error) {
	err = Db.Where("enable = 1 AND pool_id = ? ", poolId).Find(&ps).Error
	return
}

func GetMonitorOndutyGroupAll() (ps []*MonitorOndutyGroup, err error) {
	err = Db.Preload("Members").Find(&ps).Error
	return
}

func (obj *MonitorOndutyGroup) FillFrontAllData() {
	//dbUser, _ := GetUserById(int(obj.UserID))
	//if dbUser != nil {
	//	obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	//}
	//dbPool, _ := GetMonitorOndutyGroupById(obj.Name)
	//if dbPool != nil {
	//	obj.PoolName = dbPool.Name
	//}
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetMonitorOndutyGroupByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorOndutyGroup, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
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

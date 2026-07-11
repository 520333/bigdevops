package models

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorOndutyGroup 值班组结构体

type MonitorOndutyGroup struct {
	Model
	Name string `json:"name,omitempty" validate:"required,min=1,max=50" gorm:"uniqueIndex;type:varchar(100);comment:发送任务名称"`

	UserID uint

	Enable int `json:"enable" gorm:"comment:是否被开启 1正常 2禁用"`
	// 发送逻辑
	Members                   []*User  `json:"members" gorm:"many2many:monitor_onduty_users;comment:值班人列表"`
	ShiftDays                 int      `json:"shiftDays" gorm:"comment:轮班周期：天、周"`
	ImRobotToken              string   `json:"imRobotToken" gorm:"comment:im机器人token 对应哪个群组"`
	YesterdayNormalDutyUserId uint     `json:"yesterdayNormalDutyUserId" gorm:"comment:不考虑换班的 正常排班的 昨日值班人 由cron设置"`
	UserNames                 []string `json:"userNames" gorm:"-"`       // 前端使用
	ToDayOnDutyUser           *User    `json:"toDayOnDutyUser" gorm:"-"` // 当天值班人

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

//	func (obj *MonitorOndutyGroup) FillToDayOndutyUser() {
//		toDayString := common.GetDayAgoDate(0)
//		dbHistoryToday, _ := GetMonitorOnDutyHistoryByOnDutyGroupIdAndDay(obj.ID, toDayString)
//		if dbHistoryToday.OndutyUserId > 0 {
//			user, _ := GetUserById(int(dbHistoryToday.OndutyUserId))
//			if user.ID > 0 {
//				obj.ToDayOnDutyUser = user
//			}
//		} else {
//			obj.ToDayOnDutyUser = obj.Members[0]
//		}
//	}
func (m *MonitorOndutyGroup) FillToDayOndutyUser() {
	if len(m.Members) == 0 {
		return
	}

	todayStr := time.Now().Format("2006-01-02")

	// 1. 最高优先级：检查今天是否有人“换班” (临时顶替)
	change, _ := GetMonitorOndutyChangeByOnDutyGroupIdAndDay(m.ID, todayStr)
	if change != nil && change.OndutyUserId > 0 {
		user, _ := GetUserById(int(change.OndutyUserId))
		m.ToDayOnDutyUser = user
		return
	}

	// 2. 次优先级：检查历史/计划表中今天排了谁
	history, _ := GetMonitorOnDutyHistoryByOnDutyGroupIdAndDay(m.ID, todayStr)
	if history != nil && history.OndutyUserId > 0 {
		user, _ := GetUserById(int(history.OndutyUserId))
		m.ToDayOnDutyUser = user
		return
	}

	// 3. 兜底推算：如果没有记录，根据【创建时间】和【轮换天数】动态推算
	// 抹平到当天的 0 点 0 分计算纯天数差
	now := time.Now()
	createDate := time.Date(m.CreatedAt.Year(), m.CreatedAt.Month(), m.CreatedAt.Day(), 0, 0, 0, 0, time.Local)
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	daysPassed := int(todayDate.Sub(createDate).Hours() / 24)
	if daysPassed < 0 {
		daysPassed = 0
	}

	// 拿到轮转天数，兜底防除 0
	shiftDays := int(m.ShiftDays)
	if shiftDays <= 0 {
		shiftDays = 1
	}

	// 核心算法：经过的天数 / 每个人的排班天数 = 当前经过了几个排班块
	// 然后对总人数取模，就能精准算出今天该轮到第几个人！
	memberIndex := (daysPassed / shiftDays) % len(m.Members)

	m.ToDayOnDutyUser = m.Members[memberIndex]
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

// SetOnDutyGroupStatus 快捷更新开启状态
func SetOnDutyGroupStatus(id uint, enable int) error {
	// 假设你的全局数据库对象是 global.DB 或 common.DB，请根据你的项目实际情况调整
	err := Db.Model(&MonitorOndutyGroup{}).Where("id = ?", id).Update("enable", enable).Error
	return err
}

package models

import (
	"errors"
	"fmt"

	//"github.com/prometheus/prometheus/model/relabel"
	//"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorAlertManagerSendGroup 发送任务Job对象

type MonitorAlertManagerSendGroup struct {
	Model
	Name   string `json:"name,omitempty" validate:"required,min=1,max=50" gorm:"uniqueIndex;type:varchar(100);comment:发送任务名称"`
	NameZh string `json:"nameZh,omitempty" validate:"required,min=1,max=50" gorm:"uniqueIndex;type:varchar(100);comment:发送组中文名称"`
	Enable int    `json:"enable,omitempty" gorm:"comment:是否被开启 1正常 2禁用"`

	UserID uint

	PoolId uint `json:"poolId,omitempty" gorm:"comment:关联哪个alertManager实例"`

	// 发送逻辑
	StaticReceiveUsers  []*User `json:"staticReceiveUsers" gorm:"many2many:static_receive_user;comment:静态配置的接收人列表"`
	FeiShuQunRobotToken string  `json:"feiShuQunRobotToken" gorm:"comment:im飞书自定义机器人token"`
	OnDutyGroupId       uint    `json:"onDutyGroupId,omitempty" gorm:"comment:值班表 在im中发到群组里at值班人或者私聊发送给值班人"`

	RepeatInterval     string      `json:"repeatInterval" gorm:"comment:默认重复发送间隔"`
	SendResolved       int         `json:"sendResolved" gorm:"comment:是否被开启 1=true发送 2=false不发送 "`
	NotifyMethods      StringArray `json:"notifyMethods,omitempty" gorm:"comment:通知方法：email im phone sms 组合"`
	FirstUpgradeUsers  []*User     `json:"firstUpgradeUsers" gorm:"many2many:first_upgrade_users;comment:第一升级人列表"`
	UpgradeMinutes     int         `json:"upgradeMinutes" gorm:"comment:告警多久未恢复就升级"`
	SecondUpgradeUsers []*User     `json:"secondUpgradeUsers" gorm:"many2many:second_upgrade_users;comment:第二升级人列表"`

	TreeNodeIds    StringArray `json:"treeNodeIds,omitempty" gorm:"comment:如果使用了服务树接口 通过树id获取ip列表"`
	Key            string      `json:"key,omitempty" gorm:"-"` // 前端表格使用
	PoolName       string      `json:"poolName,omitempty" gorm:"-"`
	CreateUserName string      `json:"createUserName,omitempty" gorm:"-"`
}

func (obj *MonitorAlertManagerSendGroup) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorAlertManagerSendGroup) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorAlertManagerSendGroup) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *MonitorAlertManagerSendGroup) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func (obj *MonitorAlertManagerSendGroup) IdsConvert() {
	strIds := []string{}
	for _, id := range obj.TreeNodeIds {
		strIds = append(strIds, fmt.Sprintf("%s", id))
	}
}

//func (obj *MonitorAlertManagerSendGroup) ValidateRelabelConfigsYamlString() error {
//	var relabelConfigsObj []*relabel.Config
//	return yaml.Unmarshal([]byte(obj.RelabelConfigsYamlString), &relabelConfigsObj)
//}

func GetMonitorAlertManagerSendGroupById(id int) (*MonitorAlertManagerSendGroup, error) {
	var dbMonitorAlertManagerSendGroup MonitorAlertManagerSendGroup
	err := Db.Where("id = ? ", id).First(&dbMonitorAlertManagerSendGroup).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorScrapePool不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMonitorAlertManagerSendGroup, nil
}

func GetMonitorAlertManagerSendGroupByPoolId(poolId uint) (ps []*MonitorAlertManagerSendGroup, err error) {
	err = Db.Where("enable = 1 AND pool_id = ? ", poolId).Find(&ps).Error
	return
}

func GetMonitorAlertManagerSendGroupAll() (ps []*MonitorAlertManagerSendGroup, err error) {
	err = Db.Preload("FirstUpgradeUsers").Find(&ps).Error
	return
}

func (obj *MonitorAlertManagerSendGroup) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	//dbPool, _ := GetMonitorScrapePoolById(obj.Name)
	//if dbPool != nil {
	//	obj.PoolName = dbPool.Name
	//}
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetMonitorAlertManagerSendGroupByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorAlertManagerSendGroup, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

// UpdateEnable 更新发送任务的开关状态
func (obj *MonitorAlertManagerSendGroup) UpdateEnable() error {
	// 推荐使用 Select 显式指定更新 enable 字段，这样既安全又能避免潜在的零值过滤问题
	return Db.Model(obj).Select("Enable").Updates(obj).Error
}

// SetSendGroupStatus 快捷更新开启状态
func SetSendGroupStatus(id uint, enable int) error {
	// 假设你的全局数据库对象是 global.DB 或 common.DB，请根据你的项目实际情况调整
	err := Db.Model(&MonitorAlertManagerSendGroup{}).Where("id = ?", id).Update("enable", enable).Error
	return err
}

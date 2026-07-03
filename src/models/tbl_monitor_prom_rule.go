package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorPromAlertRule 采集任务Job对象

type MonitorPromAlertRule struct {
	Model
	Name string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:采集任务名称"`

	UserID uint

	Enable      int         `json:"enable" gorm:"comment:是否被开启 1正常 2禁用"`
	SendGroupId string      `json:"sendGroupId"`
	Expr        string      `json:"expr" gorm:"type:text;comment:规则PQL"`
	FormTime    string      `json:"formTime" gorm:"comment:持续时间 到这个时间才触发"`
	Labels      StringArray `json:"labels"  gorm:"comment:标签组 k=v ,severity=critical"`
	Annotations StringArray `json:"annotations"  gorm:"comment:注解 k=v ,summary=xxx,description=xxx"`

	TreeNodeIds    StringArray `json:"treeNodeIds,omitempty" gorm:"comment:如果使用了服务树接口 通过树id获取ip列表"`
	Key            string      `json:"key" gorm:"-"` // 前端表格使用
	PoolName       string      `json:"poolName" gorm:"-"`
	CreateUserName string      `json:"createUserName" gorm:"-"`
}

func (obj *MonitorPromAlertRule) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorPromAlertRule) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorPromAlertRule) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *MonitorPromAlertRule) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

//func (obj *MonitorPromAlertRule) ValidateRelabelConfigsYamlString() error {
//	var relabelConfigsObj []*relabel.Config
//	return yaml.Unmarshal([]byte(obj.RelabelConfigsYamlString), &relabelConfigsObj)
//}

func GetMonitorPromAlertRuleById(id int) (*MonitorPromAlertRule, error) {
	var dbMonitorPromAlertRule MonitorPromAlertRule
	err := Db.Where("id = ? ", id).First(&dbMonitorPromAlertRule).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorScrapePool不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMonitorPromAlertRule, nil
}

func GetMonitorPromAlertRuleByPoolId(poolId uint) (ps []*MonitorPromAlertRule, err error) {
	err = Db.Where("enable = 1 AND pool_id = ? ", poolId).Find(&ps).Error
	return
}

func GetMonitorPromAlertRuleAll() (ps []*MonitorPromAlertRule, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *MonitorPromAlertRule) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	//dbPool, _ := GetMonitorScrapePoolById(int(obj.PoolId))
	//if dbPool != nil {
	//	obj.PoolName = dbPool.Name
	//}
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetMonitorPromAlertRuleByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorPromAlertRule, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

// UpdateEnable 更新采集任务的开关状态
func (obj *MonitorPromAlertRule) UpdateEnable() error {
	// 推荐使用 Select 显式指定更新 enable 字段，这样既安全又能避免潜在的零值过滤问题
	return Db.Model(obj).Select("Enable").Updates(obj).Error
}

// SetAlertManagerRuleStatus 快捷更新开启状态
func SetAlertManagerRuleStatus(id uint, enable int) error {
	// 假设你的全局数据库对象是 global.DB 或 common.DB，请根据你的项目实际情况调整
	err := Db.Model(&MonitorPromAlertRule{}).Where("id = ?", id).Update("enable", enable).Error
	return err
}

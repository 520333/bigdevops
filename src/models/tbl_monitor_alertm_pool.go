package models

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorAlertManagerPool alertManager实例和机器的关系
type MonitorAlertManagerPool struct {
	Model
	Name                  string      `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:alertManager实例名称"`
	AlertManagerInstances StringArray `json:"alertManagerInstances,omitempty"`

	UserID uint
	//ExternalLabels StringArray `json:"externalLabels" gorm:"comment:remote_write的时候添加的标签组 key=v"`
	Enable         int    `json:"enable" gorm:"comment:是否被开启 1正常 2禁用"`
	ResolveTimeout string `json:"resolveTimeout" gorm:"comment:默认恢复时间"`
	GroupWait      string `json:"groupWait" gorm:"comment:默认分组第一次等待时间"`
	GroupInterval  string `json:"groupInterval" gorm:"comment:默认分组间隔"`

	RepeatInterval string `json:"repeatInterval" gorm:"comment:默认重复发送间隔"`

	GroupBy  StringArray `json:"groupBy,omitempty" gorm:"comment:分组标签"`
	Receiver string      `json:"receiver,omitempty" gorm:"comment:兜底接收者"`

	ExternalLabelsFront string `json:"externalLabelsFront,omitempty" gorm:"-"`
	GroupByFront        string `json:"groupByFront,omitempty" gorm:"-"`
	Key                 string `json:"key,omitempty" gorm:"-"` // 前端表格使用
	CreateUserName      string `json:"createUserName,omitempty" gorm:"-"`
}

func (obj *MonitorAlertManagerPool) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorAlertManagerPool) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorAlertManagerPool) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *MonitorAlertManagerPool) UpdateOne() error {
	return Db.Where("id = ? ", obj.ID).Updates(obj).Error
}

func GetMonitorAlertManagerPoolById(id int) (*MonitorAlertManagerPool, error) {
	var dbMonitorAlertManagerPool MonitorAlertManagerPool
	err := Db.Where("id = ? ", id).First(&dbMonitorAlertManagerPool).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorAlertManagerPool不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMonitorAlertManagerPool, nil
}

func GetMonitorAlertManagerPoolAll() (ps []*MonitorAlertManagerPool, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *MonitorAlertManagerPool) CheckInstanceIpExists() bool {
	all, err := GetMonitorAlertManagerPoolAll()
	if err != nil {
		return true
	}
	ipMap := map[string]string{}
	for _, p := range all {
		p := p
		if p.Name == obj.Name {
			continue
		}
		for _, ip := range p.AlertManagerInstances {
			ipMap[ip] = ip
		}
		for _, ip := range obj.AlertManagerInstances {
			_, ok := ipMap[ip]
			if ok {
				return true
			}
		}
	}
	return false
}

func (obj *MonitorAlertManagerPool) FillDefaultData() {
	if obj.ResolveTimeout == "" {
		obj.ResolveTimeout = "5m"
	}
	if obj.GroupWait == "" {
		obj.GroupWait = "10s"
	}
	if obj.GroupInterval == "" {
		obj.GroupInterval = "10s"
	}
	if obj.RepeatInterval == "" {
		obj.RepeatInterval = "4h"
	}
	obj.ExternalLabelsFront = strings.Join(obj.GroupBy, "\n")
	obj.Key = fmt.Sprintf("%d", obj.ID)
	//obj.GroupBy = common.GentStringArrayByChangeLine(obj.GroupByFront)
	//if len(obj.GroupBy) == 0 {
	//	obj.GroupBy = []string{"alertname"}
	//}
}

func (obj *MonitorAlertManagerPool) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	obj.ExternalLabelsFront = strings.Join(obj.GroupBy, "\n")
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetMonitorAlertManagerPoolByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorAlertManagerPool, err error) {
	err = Db.Where("id in ? ", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorScrapePool 采集池和机器的关系
type MonitorScrapePool struct {
	Model
	Name                string      `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:采集池名称"`
	PrometheusInstances StringArray `json:"prometheus_instances,omitempty"`

	UserID uint
	// global段
	ScrapeInterval int         `json:"scrapeInterval" gorm:"comment:采集间隔"`
	ScrapeTimeout  int         `json:"scrapeTimeout" gorm:"comment:采集超时时间"`
	ExternalLabels StringArray `json:"externalLabels" gorm:"comment:remote_write的时候添加的标签组 key=v"`

	// remote_write段
	RemoteWriteUrl       string `json:"remoteWriteUrl" gorm:"comment:tsdb远程写入的地址"`
	RemoteTimeoutSeconds int    `json:"remoteTimeoutSeconds" gorm:"comment:tsdb远程写入的超时时间"`

	Key            string `json:"key" gorm:"-"` // 前端表格使用
	CreateUserName string `json:"createUserName" gorm:"-"`
}

func (obj *MonitorScrapePool) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorScrapePool) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorScrapePool) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *MonitorScrapePool) UpdateOne() error {
	return Db.Where("id = ? ", obj.ID).Updates(obj).Error
}

func GetMonitorScrapePoolById(id int) (*MonitorScrapePool, error) {
	var dbMonitorScrapePool MonitorScrapePool
	err := Db.Where("id = ? ", id).First(&dbMonitorScrapePool).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorScrapePool不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMonitorScrapePool, nil
}

func GetMonitorScrapePoolAll() (ps []*MonitorScrapePool, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *MonitorScrapePool) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetMonitorScrapePoolByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorScrapePool, err error) {
	err = Db.Where("id in ? ", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

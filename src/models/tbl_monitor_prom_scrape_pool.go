package models

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorPromScrapePool 采集池和机器的关系
type MonitorPromScrapePool struct {
	Model
	Name                string      `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:采集池名称"`
	PrometheusInstances StringArray `json:"prometheus_instances,omitempty"`

	UserID uint `json:"userId,omitempty" gorm:"comment:创建人ID"`
	// global段
	ScrapeInterval int         `json:"scrapeInterval" gorm:"comment:采集间隔"`
	ScrapeTimeout  int         `json:"scrapeTimeout" gorm:"comment:采集超时时间"`
	ExternalLabels StringArray `json:"externalLabels" gorm:"comment:remote_write的时候添加的标签组 key=v"`

	// 告警段
	SupperAlert     int    `json:"supperAlert" gorm:"comment:是否支持告警 1支持 2不支持"`
	SupperRecord    int    `json:"supperRecord" gorm:"comment:是否支持record 1支持 2不支持"`
	RemoteReadUrl   string `json:"remoteReadUrl" gorm:"comment:远程读取的地址"`
	AlertManagerUrl string `json:"alertManagerUrl" gorm:"comment:alertManager地址"`
	RuleFilePath    string `json:"ruleFilePath" gorm:"comment:rule告警规则文件路径"`
	RecordFilePath  string `json:"recordFilePath" gorm:"comment:record预聚合的文件路径"`

	// remote_write段
	RemoteWriteUrl       string `json:"remoteWriteUrl" gorm:"comment:tsdb远程写入的地址"`
	RemoteTimeoutSeconds int    `json:"remoteTimeoutSeconds" gorm:"comment:tsdb远程写入的超时时间"`

	ExternalLabelsFront string `json:"externalLabelsFront" gorm:"-"`
	Key                 string `json:"key" gorm:"-"` // 前端表格使用
	CreateUserName      string `json:"createUserName" gorm:"-"`
}

func (obj *MonitorPromScrapePool) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorPromScrapePool) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorPromScrapePool) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *MonitorPromScrapePool) UpdateOne() error {
	return Db.Model(obj).Select("*").Omit("id", "created_at", "user_id").Updates(obj).Error
}

func GetMonitorPromScrapePoolById(id int) (*MonitorPromScrapePool, error) {
	var dbMonitorScrapePool MonitorPromScrapePool
	err := Db.Where("id = ? ", id).First(&dbMonitorScrapePool).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorScrapePool不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMonitorScrapePool, nil
}

func GetMonitorPromScrapePoolAll() (ps []*MonitorPromScrapePool, err error) {
	err = Db.Find(&ps).Error
	return
}

func GetMonitorPromScrapePoolSupportAlertAll() (ps []*MonitorPromScrapePool, err error) {
	err = Db.Where("supper_alert = 1 ").Find(&ps).Error
	return
}

func GetMonitorPromScrapePoolSupportRecordAll() (ps []*MonitorPromScrapePool, err error) {
	err = Db.Where("supper_record = 1 ").Find(&ps).Error
	return
}

func (obj *MonitorPromScrapePool) CheckInstanceIpExists() bool {
	all, err := GetMonitorPromScrapePoolAll()
	if err != nil {
		return true
	}
	ipMap := map[string]string{}
	for _, p := range all {
		p := p
		if p.Name == obj.Name {
			continue
		}
		for _, ip := range p.PrometheusInstances {
			ipMap[ip] = ip
		}
		for _, ip := range obj.PrometheusInstances {
			_, ok := ipMap[ip]
			if ok {
				return true
			}
		}
	}
	return false
}

func (obj *MonitorPromScrapePool) FillDefaultData() {
	if obj.ScrapeInterval == 0 {
		obj.ScrapeInterval = 15
	}
	if obj.ScrapeTimeout == 0 {
		obj.ScrapeTimeout = 10
	}
	if obj.RemoteTimeoutSeconds == 0 {
		obj.RemoteTimeoutSeconds = 5
	}
}

func (obj *MonitorPromScrapePool) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	obj.ExternalLabelsFront = strings.Join(obj.ExternalLabels, "\n")
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetMonitorPromScrapePoolByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorPromScrapePool, err error) {
	err = Db.Where("id in ? ", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

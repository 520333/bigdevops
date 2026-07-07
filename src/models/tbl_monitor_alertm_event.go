package models

import (
	"errors"
	"fmt"
	"strings"

	"github.com/prometheus/alertmanager/template"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorAlertEvent 采集任务Job对象

type MonitorAlertEvent struct {
	Model
	AlertName   string `json:"alertName"`
	FingerPrint string `json:"fingerPrint,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:告警unique id eventId"`
	Status      string `json:"status" gorm:"comment:告警状态： 告警中|已屏蔽|已认领|已恢复"`
	RuleId      uint   `json:"ruleId"`
	SendGroupId uint   `json:"sendGroupId"`
	EventTimes  int    `json:"eventTimes" gorm:"comment:触发次数"`

	Key       string                        `json:"key" gorm:"-"` // 前端表格使用
	Alert     template.Alert                `json:"alert" gorm:"-"`
	SendGroup *MonitorAlertManagerSendGroup `json:"sendGroup" gorm:"-"`
	Rule      *MonitorPromAlertRule         `json:"rule" gorm:"-"`

	LabelsM      map[string]string `json:"labelsM" gorm:"-"`
	AnnotationsM map[string]string `json:"annotationsM" gorm:"-"`
}

func (mae *MonitorAlertEvent) Create() error {
	return Db.Create(mae).Error
}

func (mae *MonitorAlertEvent) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(mae).Error
}

func (mae *MonitorAlertEvent) CreateOne() error {
	return Db.Create(mae).Error
}
func (mae *MonitorAlertEvent) UpdateOrCreateOne() error {
	dbMae, err := GetMonitorAlertEventByFingerPrintId(mae.FingerPrint)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			mae.EventTimes += 1
			return mae.CreateOne()
		}
		return err
	}
	mae.Status = dbMae.Status
	dbMae.EventTimes++

	return dbMae.UpdateOne()
}

func (mae *MonitorAlertEvent) UpdateOne() error {
	return Db.Where("id = ?", mae.ID).Updates(mae).Error
}

func GetMonitorAlertEventById(id int) (*MonitorAlertEvent, error) {
	var dbMonitorAlertEvent MonitorAlertEvent
	err := Db.Where("id = ? ", id).First(&dbMonitorAlertEvent).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorAlertEvent不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMonitorAlertEvent, nil
}

func GetMonitorAlertEventByFingerPrintId(fingerPrint string) (mae *MonitorAlertEvent, err error) {
	err = Db.Where("finger_print = ?", fingerPrint).First(&mae).Error
	return
}

func GetMonitorAlertEventAll() (mae []*MonitorAlertEvent, err error) {
	err = Db.Find(&mae).Error
	return
}

func (mae *MonitorAlertEvent) GenMapFromKvs(kvs []string) map[string]string {
	labelsM := map[string]string{}
	for _, i := range kvs {
		kvs := strings.Split(i, "=")
		if len(kvs) != 2 {
			continue
		}
		k := kvs[0]
		v := kvs[1]
		labelsM[k] = v
	}
	return labelsM
}

func (mae *MonitorAlertEvent) FillFrontAllData() {
	//dbUser, _ := GetUserById(int(obj.UserID))
	//if dbUser != nil {
	//	obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	//}
	//dbPool, _ := GetMonitorScrapePoolById(int(obj.PoolId))
	//if dbPool != nil {
	//	obj.PoolName = dbPool.Name
	//}
	mae.Key = fmt.Sprintf("%d", mae.ID)
	//obj.LabelsM = obj.GenMapFromKvs(obj.Labels)
	// 绑定发送组标签
	//obj.LabelsM[common.MONITOR_ALERT_MATCH_KEY] = fmt.Sprintf("%d", obj.SendGroupId)
	//obj.AnnotationsM = obj.GenMapFromKvs(obj.Annotations)
}

func GetMonitorAlertEventByIdsWithLimitOffset(ids []int, limit, offset int) (maes []*MonitorAlertEvent, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&maes).Error
	return

}

// UpdateEnable 更新采集任务的开关状态
func (mae *MonitorAlertEvent) UpdateEnable() error {
	// 推荐使用 Select 显式指定更新 enable 字段，这样既安全又能避免潜在的零值过滤问题
	return Db.Model(mae).Select("Enable").Updates(mae).Error
}

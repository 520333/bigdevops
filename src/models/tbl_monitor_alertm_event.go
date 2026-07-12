package models

import (
	"bigdevops/src/common"
	"errors"
	"fmt"
	"strings"

	"github.com/prometheus/alertmanager/template"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorAlertEvent 采集任务Job对象

type MonitorAlertEvent struct {
	Model
	AlertName   string      `json:"alertName"`
	FingerPrint string      `json:"fingerPrint,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:告警unique id eventId"`
	Status      string      `json:"status" gorm:"comment:告警状态： 告警中|已屏蔽|已认领|已恢复"`
	RuleId      uint        `json:"ruleId"`
	SendGroupId uint        `json:"sendGroupId"`
	EventTimes  int         `json:"eventTimes" gorm:"comment:触发次数"`
	SilenceID   string      `json:"silenceID" gorm:"comment:alertmanager返回的静默id"`
	Labels      StringArray `json:"labels" gorm:"comment: 标签组 k=v"`

	Key           string                        `json:"key" gorm:"-"` // 前端表格使用
	AlertRuleName string                        `json:"alertRuleName" gorm:"-"`
	SendGroupName string                        `json:"sendGroupName" gorm:"-"`
	Alert         template.Alert                `json:"alert" gorm:"-"`
	SendGroup     *MonitorAlertManagerSendGroup `json:"sendGroup" gorm:"-"`
	Rule          *MonitorAlertRule             `json:"rule" gorm:"-"`

	LabelsM      map[string]string `json:"labelsM" gorm:"-"`
	AnnotationsM map[string]string `json:"annotationsM" gorm:"-"`
}

func (obj *MonitorAlertEvent) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorAlertEvent) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorAlertEvent) CreateOne() error {
	return Db.Create(obj).Error
}
func (obj *MonitorAlertEvent) UpdateOrCreateOne() error {
	//dbobj, err := GetMonitorAlertEventByFingerPrintId(obj.FingerPrint)
	//if err != nil {
	//	if errors.Is(err, gorm.ErrRecordNotFound) {
	//		obj.EventTimes += 1
	//		return obj.CreateOne()
	//	}
	//	return err
	//}
	//obj.Status = dbobj.Status
	//dbobj.EventTimes++
	//
	//return dbobj.UpdateOne()

	dbobj, err := GetMonitorAlertEventByFingerPrintId(obj.FingerPrint)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 第一次触发
			obj.EventTimes = 1
			return obj.CreateOne()
		}
		return err
	}

	// 💡 核心修复 1：把数据库里的历史次数 + 1，赋值给当前正在处理的 obj 对象
	obj.EventTimes = dbobj.EventTimes + 1

	// 💡 核心修复 2：继承数据库里的主键 ID，这样 GORM 执行 UpdateOne 才知道更新哪一行
	obj.ID = dbobj.ID

	// 注意：去掉了 obj.Status = dbobj.Status，保留外部传进来的最新状态

	// 使用当前对象更新数据库
	return obj.UpdateOne()
}

func (obj *MonitorAlertEvent) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
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

func GetMonitorAlertEventByFingerPrintId(fingerPrint string) (obj *MonitorAlertEvent, err error) {
	err = Db.Where("finger_print = ?", fingerPrint).First(&obj).Error
	return
}

func GetMonitorAlertEventAll() (obj []*MonitorAlertEvent, err error) {
	err = Db.Find(&obj).Error
	return
}

func (obj *MonitorAlertEvent) GenMapFromKvs() map[string]string {
	labelsM := map[string]string{}
	for _, i := range obj.Labels {
		kvs := strings.Split(i, "=")
		if len(kvs) != 2 {
			continue
		}
		k := kvs[0]
		v := kvs[1]
		labelsM[k] = v
	}
	obj.LabelsM = labelsM
	return labelsM
}

func (obj *MonitorAlertEvent) FillFrontAllData() {

	obj.CreatedTime = common.TimeFormat(obj.CreatedAt)
	obj.UpdatedTime = common.TimeFormat(obj.UpdatedAt)

	obj.Key = fmt.Sprintf("%d", obj.ID)
	alertRule, _ := GetMonitorAlertRuleById(int(obj.RuleId))
	sendGroup, _ := GetMonitorAlertManagerSendGroupById(int(obj.SendGroupId))
	obj.SendGroup = sendGroup
	obj.AlertRuleName = alertRule.Name
	obj.SendGroupName = sendGroup.Name

}

func GetMonitorAlertEventByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorAlertEvent, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

// UpdateEnable 更新采集任务的开关状态
func (obj *MonitorAlertEvent) UpdateEnable() error {
	// 推荐使用 Select 显式指定更新 enable 字段，这样既安全又能避免潜在的零值过滤问题
	return Db.Model(obj).Select("Enable").Updates(obj).Error
}

func (obj *MonitorAlertEvent) SendImMessageToQunLiaoByEvent(msg, url string, logger *zap.Logger, tw int) {
	obj.FillFrontAllData()
	if obj.SendGroup == nil {
		return
	}
	feishuTextJsonString := fmt.Sprintf(
		`
	{
		"msg_type": "text",
		"content": {
			"text": "%s"
		}
	}`, msg)
	url = fmt.Sprintf("%s/%s/", url, obj.SendGroup.FeiShuQunRobotToken)
	emptyMap := map[string]string{}
	respBytes, err := common.PostWithJsonString(logger, "SendImMessageToQunLiaoByEvent", tw, url, feishuTextJsonString, emptyMap, emptyMap)
	if err != nil {
		logger.Error("发送飞书群聊消息失败", zap.Error(err), zap.Any("结果", string(respBytes)))
	}
	logger.Info("发送飞书群聊消息成功", zap.Any("结果", string(respBytes)))

}

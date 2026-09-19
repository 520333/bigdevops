package models

import (
	"bigdevops/src/common"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/alertmanager/template"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorAlertManagerEvent 采集任务Job对象

type MonitorAlertManagerEvent struct {
	Model
	AlertName    string      `json:"alertName"`
	FingerPrint  string      `json:"fingerPrint,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:告警unique id eventId"`
	Status       string      `json:"status" gorm:"comment:告警状态： 告警中|已屏蔽|已认领|已恢复"`
	RuleId       uint        `json:"ruleId"`
	SendGroupId  uint        `json:"sendGroupId"`
	EventTimes   int         `json:"eventTimes" gorm:"comment:触发次数"`
	SilenceID    string      `json:"silenceID" gorm:"comment:alertmanager返回的静默id"`
	UnsilencedAt *time.Time  `json:"unsilencedAt" gorm:"comment:最近一次解除屏蔽的时间"`
	ReLingUserId uint        `json:"reLingUserId" gorm:"comment:是谁认领了告警"`
	Labels       StringArray `json:"labels" gorm:"comment: 标签组 k=v"`

	Key           string                        `json:"key" gorm:"-"` // 前端表格使用
	AlertRuleName string                        `json:"alertRuleName" gorm:"-"`
	SendGroupName string                        `json:"sendGroupName" gorm:"-"`
	Alert         template.Alert                `json:"alert,omitempty" gorm:"-"`
	ReLingUser    *SystemUser                   `json:"reLingUser,omitempty" gorm:"-"`
	SendGroup     *MonitorAlertManagerSendGroup `json:"sendGroup,omitempty" gorm:"-"`
	Rule          *MonitorPromAlertRule         `json:"rule,omitempty" gorm:"-"`

	LabelsM      map[string]string `json:"labelsM,omitempty" gorm:"-"`
	AnnotationsM map[string]string `json:"annotationsM,omitempty" gorm:"-"`
}

func (obj *MonitorAlertManagerEvent) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorAlertManagerEvent) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorAlertManagerEvent) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *MonitorAlertManagerEvent) UpdateOrCreateOne() error {
	dbobj, err := GetMonitorAlertManagerEventByFingerPrintId(obj.FingerPrint)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 第一次触发
			obj.EventTimes = 1
			return obj.CreateOne()
		}
		return err
	}

	if obj.Status != common.MONITOR_ALERT_STATUS_RESOLVED {
		if dbobj.Status == common.MONITOR_ALERT_STATUS_RENLING || dbobj.Status == common.MONITOR_ALERT_STATUS_SILIENCED {
			// 如果之前是认领中或静默中，且告警仍然是 firing，维持原认领/静默状态
			obj.Status = dbobj.Status
			obj.ReLingUserId = dbobj.ReLingUserId
			obj.SilenceID = dbobj.SilenceID
			obj.UnsilencedAt = dbobj.UnsilencedAt
		} else if dbobj.Status == common.MONITOR_ALERT_STATUS_RESOLVED {
			// 上次已恢复，本次重新触发 Firing，重置历史认领人和已失效静默
			obj.ReLingUserId = 0
			obj.SilenceID = ""
			obj.UnsilencedAt = nil
		}
	} else {
		// 当前已恢复，继承历史认领人和静默信息便于事后复盘追溯
		obj.ReLingUserId = dbobj.ReLingUserId
		obj.SilenceID = dbobj.SilenceID
		obj.UnsilencedAt = dbobj.UnsilencedAt
	}

	obj.ID = dbobj.ID
	obj.EventTimes = dbobj.EventTimes + 1

	updates := map[string]interface{}{
		"status":          obj.Status,
		"event_times":     obj.EventTimes,
		"re_ling_user_id": obj.ReLingUserId,
		"silence_id":      obj.SilenceID,
		"unsilenced_at":   obj.UnsilencedAt,
		"labels":          obj.Labels,
		"alert_name":      obj.AlertName,
		"rule_id":         obj.RuleId,
		"send_group_id":   obj.SendGroupId,
	}
	return Db.Model(&MonitorAlertManagerEvent{}).Where("id = ?", obj.ID).Updates(updates).Error
}

func (obj *MonitorAlertManagerEvent) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func GetMonitorAlertManagerEventById(id int) (*MonitorAlertManagerEvent, error) {
	var dbMonitorAlertEvent MonitorAlertManagerEvent
	err := Db.Where("id = ? ", id).First(&dbMonitorAlertEvent).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorAlertEvent不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMonitorAlertEvent, nil
}

func GetMonitorAlertManagerEventByFingerPrintId(fingerPrint string) (obj *MonitorAlertManagerEvent, err error) {
	err = Db.Where("finger_print = ?", fingerPrint).First(&obj).Error
	return
}

func GetMonitorAlertManagerEventAll() (obj []*MonitorAlertManagerEvent, err error) {
	err = Db.Find(&obj).Error
	return
}

func GetMonitorAlertManagerEventPage(name string, limit, offset int) (objs []*MonitorAlertManagerEvent, total int64, err error) {
	tx := Db.Model(&MonitorAlertManagerEvent{})
	if name != "" {
		tx = tx.Where("alert_name LIKE ?", "%"+name+"%")
	}
	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("id DESC").Limit(limit).Offset(offset).Find(&objs).Error
	return
}

func (obj *MonitorAlertManagerEvent) GenMapFromKvs() map[string]string {
	labelsM := map[string]string{}
	for _, i := range obj.Labels {
		kvs := strings.SplitN(i, "=", 2)
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

func (obj *MonitorAlertManagerEvent) FillFrontAllData() {
	obj.CreatedTime = common.TimeFormat(obj.CreatedAt)
	obj.UpdatedTime = common.TimeFormat(obj.UpdatedAt)
	obj.Key = fmt.Sprintf("%d", obj.ID)

	alertRule, _ := GetMonitorPromAlertById(int(obj.RuleId))
	sendGroup, _ := GetMonitorAlertManagerSendGroupById(int(obj.SendGroupId))

	// 🚀 核心修复 1：把之前注释掉的认领人查询打开，并赋值给虚拟字段 ReLingUser！
	if obj.ReLingUserId > 0 {
		renLingUser, _ := GetUserById(int(obj.ReLingUserId))
		if renLingUser != nil {
			obj.ReLingUser = renLingUser
		}
	}

	if alertRule != nil {
		obj.AlertRuleName = alertRule.Name
	}
	if sendGroup != nil {
		obj.SendGroupName = sendGroup.Name
		obj.SendGroup = sendGroup
	}
}

func GetMonitorAlertManagerEventByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorAlertManagerEvent, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

// UpdateEnable 更新采集任务的开关状态
func (obj *MonitorAlertManagerEvent) UpdateEnable() error {
	// 推荐使用 Select 显式指定更新 enable 字段，这样既安全又能避免潜在的零值过滤问题
	return Db.Model(obj).Select("Enable").Updates(obj).Error
}

func (obj *MonitorAlertManagerEvent) SendImMessageToQunLiaoByEvent(msg, url string, logger *zap.Logger, tw int) {
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

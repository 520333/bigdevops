package models

import (
	"bigdevops/src/common"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorPromAlertRule 采集任务Job对象

type MonitorPromAlertRule struct {
	Model
	Name string `json:"name,omitempty" gorm:"type:varchar(100);comment:告警规则名称"`

	UserID      uint
	PoolId      uint `json:"poolId,omitempty" gorm:"comment:关联哪个prometheus实例"`
	SendGroupId int  `json:"sendGroupId"`
	TreeNodeId  uint `json:"treeNodeId" gorm:"comment:绑定到哪个节点"`
	Enable      int  `json:"enable" gorm:"comment:是否被开启 1正常 2禁用"`

	GrafanaLink string `json:"grafanaLink" gorm:"type:text;comment:grafana面板地址"`

	Expr        string      `json:"expr" gorm:"type:text;comment:规则PQL"`
	Severity    string      `json:"severity" gorm:"comment:告警级别：critical|warning"`
	ForTime     string      `json:"forTime" gorm:"comment:持续时间 到这个时间才触发"`
	Labels      StringArray `json:"labels"  gorm:"comment:标签组 k=v ,severity=critical"`
	Annotations StringArray `json:"annotations"  gorm:"comment:注解 k=v ,summary=xxx,description=xxx"`

	NodePath    string      `json:"nodePath" gorm:"-"`
	TreeNodeIds StringArray `json:"treeNodeIds,omitempty" gorm:"comment:如果使用了服务树接口 通过树id获取ip列表"`
	Key         string      `json:"key" gorm:"-"` // 前端表格使用

	PoolName       string `json:"poolName" gorm:"-"`
	SendGroupName  string `json:"sendGroupName" gorm:"-"`
	CreateUserName string `json:"createUserName" gorm:"-"`

	LabelsFront      string            `json:"labelsFront" gorm:"-"`
	AnnotationsFront string            `json:"annotationsFront" gorm:"-"`
	LabelsM          map[string]string `json:"labelsM" gorm:"-"`
	AnnotationsM     map[string]string `json:"annotationsM" gorm:"-"`
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

func GetMonitorPromAlertById(id int) (*MonitorPromAlertRule, error) {
	var dbMonitorAlertRule MonitorPromAlertRule
	err := Db.Where("id = ? ", id).First(&dbMonitorAlertRule).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorScrapePool不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMonitorAlertRule, nil
}

func GetMonitorPromAlertRuleByPoolId(poolId uint) (ps []*MonitorPromAlertRule, err error) {
	err = Db.Where("enable = 1 AND pool_id = ? ", poolId).Find(&ps).Error
	return
}

func GetMonitorPromAlertRuleBySendGroupId(sendGroupId uint) (ps []*MonitorPromAlertRule, err error) {
	err = Db.Where("send_group_id = ? ", sendGroupId).Find(&ps).Error
	return
}

func GetMonitorPromAlertRuleAll() (ps []*MonitorPromAlertRule, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *MonitorPromAlertRule) GenMapFromKvs(kvs []string) map[string]string {
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

func (obj *MonitorPromAlertRule) FillDefaultData() {
	if obj.ForTime == "" {
		obj.ForTime = "1m"
	}

	obj.Labels = common.GentStringArrayByChangeLine(obj.LabelsFront)
	obj.Annotations = common.GentStringArrayByChangeLine(obj.AnnotationsFront)
	//found := false
	//for _, ann := range obj.Annotations {
	//	if strings.HasPrefix(ann, common.MONITOR_ALERT_RULE_ANNO_VALUE) {
	//		found = true
	//		break
	//	}
	//}
	//if !found {
	//	obj.Annotations = append(obj.Annotations, fmt.Sprintf("%s=%s",
	//		common.MONITOR_ALERT_RULE_ANNO_VALUE,
	//		"{{ $value }}",
	//	))
	//}

}

func (obj *MonitorPromAlertRule) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}

	dbPool, _ := GetMonitorPromScrapePoolById(int(obj.PoolId))
	if dbPool != nil {
		obj.PoolName = dbPool.Name
	}
	sengGroup, _ := GetMonitorAlertManagerSendGroupById(int(obj.SendGroupId))
	if sengGroup != nil {
		obj.SendGroupName = sengGroup.Name
	}

	node, _ := GetStreeNodeById(int(obj.TreeNodeId))
	if node != nil {
		node.FillFrontAllData()
		obj.NodePath = node.NodePath
	}

	obj.LabelsFront = strings.Join(obj.Labels, "\n")
	obj.AnnotationsFront = strings.Join(obj.Annotations, "\n")
	obj.Key = fmt.Sprintf("%d", obj.ID)
	obj.LabelsM = obj.GenMapFromKvs(obj.Labels)
	// 绑定发送组标签
	obj.LabelsM[common.MONITOR_ALERT_MATCH_KEY] = fmt.Sprintf("%d", obj.SendGroupId)
	obj.LabelsM[common.MONITOR_ALERT_RULE_KEY] = fmt.Sprintf("%d", obj.ID)
	obj.LabelsM[common.MONITOR_ALERT_SEVERITY_KEY] = obj.Severity
	obj.LabelsM[common.MONITOR_ALERT_BIND_NODE_KEY] = obj.NodePath

	obj.AnnotationsM = obj.GenMapFromKvs(obj.Annotations)
	obj.AnnotationsM[common.MONITOR_ALERT_RULE_ANNO_VALUE] = "{{ $value }}"

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

// UpdateMonitorPromAlertRuleEnableBatch 批量更新告警规则的开关状态
func UpdateMonitorPromAlertRuleEnableBatch(ids []int, enable int) error {
	// 使用 GORM 的 IN 查询和批量 Update
	// .Update("enable", enable) 会忽略结构体的零值限制，直接强制更新对应字段
	err := Db.Model(&MonitorPromAlertRule{}).Where("id IN ?", ids).Update("enable", enable).Error
	return err
}

// DeleteMonitorPromAlertRuleBatch 批量删除告警规则
func DeleteMonitorPromAlertRuleBatch(ids []uint) error {
	// 使用 Unscoped() 进行硬删除（如果你的逻辑是软删除，去掉 Unscoped() 即可）
	return Db.Unscoped().Where("id IN ?", ids).Delete(&MonitorPromAlertRule{}).Error
}

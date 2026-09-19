package models

import (
	"bigdevops/src/common"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorPromRecordRule 采集任务Job对象

type MonitorPromRecordRule struct {
	Model
	Name       string `json:"name,omitempty" gorm:"type:varchar(100);comment:预聚合规则名称"`
	RecordName string `json:"recordName,omitempty" gorm:"type:varchar(100);comment:预聚合名称"`

	UserID     uint
	PoolId     uint `json:"poolId,omitempty" gorm:"comment:关联哪个prometheus实例"`
	TreeNodeId uint `json:"treeNodeId" gorm:"comment:绑定到哪个节点"`
	Enable     int  `json:"enable" gorm:"comment:是否被开启 1正常 2禁用"`

	Expr string `json:"expr" gorm:"type:text;comment:规则PQL"`
	//ForTime string `json:"forTime" gorm:"comment:持续时间 到这个时间才触发"`

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

func (obj *MonitorPromRecordRule) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorPromRecordRule) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorPromRecordRule) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *MonitorPromRecordRule) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func GetMonitorPromRecordRuleById(id int) (*MonitorPromRecordRule, error) {
	var dbMonitorRecordRule MonitorPromRecordRule
	err := Db.Where("id = ? ", id).First(&dbMonitorRecordRule).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorScrapePool不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMonitorRecordRule, nil
}

func GetMonitorPromRecordRuleByPoolId(poolId uint) (ps []*MonitorPromRecordRule, err error) {
	err = Db.Where("enable = 1 AND pool_id = ? ", poolId).Find(&ps).Error
	return
}

func GetMonitorPromRecordRuleAll() (ps []*MonitorPromRecordRule, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *MonitorPromRecordRule) GenMapFromKvs(kvs []string) map[string]string {
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

func (obj *MonitorPromRecordRule) FillDefaultData() {
	//if obj.ForTime == "" {
	//	obj.ForTime = "1m"
	//}

	obj.Labels = common.GentStringArrayByChangeLine(obj.LabelsFront)
	obj.Annotations = common.GentStringArrayByChangeLine(obj.AnnotationsFront)

}

func (obj *MonitorPromRecordRule) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}

	dbPool, _ := GetMonitorPromScrapePoolById(int(obj.PoolId))
	if dbPool != nil {
		obj.PoolName = dbPool.Name
	}
	//sengGroup, _ := GetMonitorAlertManagerSendGroupById(int(obj.SendGroupId))
	//if sengGroup != nil {
	//	obj.SendGroupName = sengGroup.Name
	//}

	node, _ := GetStreeNodeById(int(obj.TreeNodeId))
	if node != nil {
		node.FillFrontAllData()
		obj.NodePath = node.NodePath
	}

	obj.LabelsFront = strings.Join(obj.Labels, "\n")
	obj.AnnotationsFront = strings.Join(obj.Annotations, "\n")
	obj.Key = fmt.Sprintf("%d", obj.ID)
	obj.LabelsM = obj.GenMapFromKvs(obj.Labels)
	obj.AnnotationsM = obj.GenMapFromKvs(obj.Annotations)

	//// 绑定发送组标签
	//obj.LabelsM[common.MONITOR_ALERT_MATCH_KEY] = fmt.Sprintf("%d", obj.SendGroupId)
	//obj.LabelsM[common.MONITOR_ALERT_RULE_KEY] = fmt.Sprintf("%d", obj.ID)
	//obj.LabelsM[common.MONITOR_ALERT_SEVERITY_KEY] = obj.Severity
	//obj.LabelsM[common.MONITOR_ALERT_BIND_NODE_KEY] = obj.NodePath
	//
	//obj.AnnotationsM = obj.GenMapFromKvs(obj.Annotations)
	//obj.AnnotationsM[common.MONITOR_ALERT_RULE_ANNO_VALUE] = "{{ $value }}"

}

func GetMonitorPromRecordRuleByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorPromRecordRule, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

// UpdateEnable 更新采集任务的开关状态
func (obj *MonitorPromRecordRule) UpdateEnable() error {
	// 推荐使用 Select 显式指定更新 enable 字段，这样既安全又能避免潜在的零值过滤问题
	return Db.Model(obj).Select("Enable").Updates(obj).Error
}

// UpdateMonitorPromRecordRuleEnableBatch 批量更新告警规则的开关状态
func UpdateMonitorPromRecordRuleEnableBatch(ids []int, enable int) error {
	err := Db.Model(&MonitorPromRecordRule{}).Where("id IN ?", ids).Update("enable", enable).Error
	return err
}

// DeleteMonitorPromRecordRuleBatch 批量删除告警规则
func DeleteMonitorPromRecordRuleBatch(ids []uint) error {
	return Db.Unscoped().Where("id IN ?", ids).Delete(&MonitorPromRecordRule{}).Error
}

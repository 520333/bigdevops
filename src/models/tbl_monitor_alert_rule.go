package models

import (
	"bigdevops/src/common"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorAlertRule 采集任务Job对象

type MonitorAlertRule struct {
	Model
	Name string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:告警规则名称"`

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

func (obj *MonitorAlertRule) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorAlertRule) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorAlertRule) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *MonitorAlertRule) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

//func (obj *MonitorAlertRule) ValidateRelabelConfigsYamlString() error {
//	var relabelConfigsObj []*relabel.Config
//	return yaml.Unmarshal([]byte(obj.RelabelConfigsYamlString), &relabelConfigsObj)
//}

func GetMonitorAlertRuleById(id int) (*MonitorAlertRule, error) {
	var dbMonitorAlertRule MonitorAlertRule
	err := Db.Where("id = ? ", id).First(&dbMonitorAlertRule).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorScrapePool不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMonitorAlertRule, nil
}

func GetMonitorAlertRuleByPoolId(poolId uint) (ps []*MonitorAlertRule, err error) {
	err = Db.Where("enable = 1 AND pool_id = ? ", poolId).Find(&ps).Error
	return
}
func GetMonitorAlertRuleBySendGroupId(sendGroupId uint) (ps []*MonitorAlertRule, err error) {
	err = Db.Where("send_group_id = ? ", sendGroupId).Find(&ps).Error
	return
}

func GetMonitorAlertRuleAll() (ps []*MonitorAlertRule, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *MonitorAlertRule) GenMapFromKvs(kvs []string) map[string]string {
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

func (obj *MonitorAlertRule) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	//dbPool, _ := GetMonitorScrapePoolById(int(obj.PoolId))
	//if dbPool != nil {
	//	obj.PoolName = dbPool.Name
	//}
	promM, _ := GetMonitorScrapePoolById(int(obj.PoolId))
	if promM != nil {
		obj.PoolName = promM.Name
	}
	sengGroup, _ := GetMonitorAlertManagerSendGroupById(int(obj.PoolId))
	if sengGroup != nil {
		obj.SendGroupName = sengGroup.Name
	}
	node, _ := GetStreeNodeById(int(obj.TreeNodeId))
	if node != nil {
		node.FillFrontAllData()
		obj.NodePath = node.NodePath
	}

	obj.Key = fmt.Sprintf("%d", obj.ID)
	obj.LabelsM = obj.GenMapFromKvs(obj.Labels)
	// 绑定发送组标签
	obj.LabelsM[common.MONITOR_ALERT_MATCH_KEY] = fmt.Sprintf("%d", obj.SendGroupId)
	obj.LabelsM[common.MONITOR_ALERT_RULE_KEY] = fmt.Sprintf("%d", obj.ID)
	obj.LabelsM[common.MONITOR_ALERT_SEVERITY_KEY] = obj.Severity
	obj.LabelsM[common.MONITOR_ALERT_BIND_NODE_KEY] = obj.NodePath

	obj.AnnotationsM = obj.GenMapFromKvs(obj.Annotations)
}

func GetMonitorAlertRuleByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorAlertRule, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

// UpdateEnable 更新采集任务的开关状态
func (obj *MonitorAlertRule) UpdateEnable() error {
	// 推荐使用 Select 显式指定更新 enable 字段，这样既安全又能避免潜在的零值过滤问题
	return Db.Model(obj).Select("Enable").Updates(obj).Error
}

// SetAlertManagerRuleStatus 快捷更新开启状态
func SetAlertManagerRuleStatus(id uint, enable int) error {
	// 假设你的全局数据库对象是 global.DB 或 common.DB，请根据你的项目实际情况调整
	err := Db.Model(&MonitorAlertRule{}).Where("id = ?", id).Update("enable", enable).Error
	return err
}

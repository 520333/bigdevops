package models

import (
	"bigdevops/src/common"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkOrderInstanceComment struct {
	UserNameTime string `json:"userNameTime" gorm:"-"`
	Comment      string `json:"comment" gorm:"-" validate:"required,min=1"`
}

// WorkOrderInstance 工作审批流
type WorkOrderInstance struct {
	Model
	// 前端用户填写字段
	Title             string     `json:"title,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:工单实例名称"`
	ActualApiJsonData string     `json:"actualApiJsonData" gorm:"type:text;comment:用户填写的动态表单数据(JSON串)"`
	DesireFinishTime  *time.Time `json:"desireFinishTime" gorm:"type:datetime;default:null;comment:期望完成时间"`
	TemplateId        uint       `json:"templateId"`
	UserID            uint

	// 流转执行字段
	Status          string `json:"status" gorm:"type:varchar(50);default:'审批中';comment:工单状态(审批中/已通过/已驳回/已取消)"`
	CurrentFlowNode string `json:"currentFlowNode,omitempty" gorm:"comment:当前执行到哪个节点"`

	ActualFlowData string `json:"actualFlowData" gorm:"type:text;comment:真实执行的历史记录 审批结果(JSON串)"`
	FinalRunData   string `json:"finalRunData" gorm:"type:text;comment:最终的执行结果"`
	Comments       string `json:"comments" gorm:"type:text;comment:用户评论(JSON串)"`
	// 前端字段
	IsRelatedWithMe bool               `json:"isRelatedWithMe" gorm:"-"`
	Template        *WorkOrderTemplate `json:"template" gorm:"-"`
	Key             string             `json:"key" gorm:"-"` // 前端表格使用
	CreateUserName  string             `json:"createUserName" gorm:"-"`
}

func (obj *WorkOrderInstance) Create() error {
	return Db.Create(obj).Error
}

func (obj *WorkOrderInstance) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *WorkOrderInstance) CreateOne() error {
	dbTemplate, err := GetWorkOrderTemplateById(int(obj.TemplateId))
	if err != nil {
		return err
	}
	obj.Status = common.WORKORDER_INSTANCE_PENDINGAPPROVAL
	dbTemplate.FillFrontAllData()
	actualFlowData, _ := json.Marshal(dbTemplate.Process.FlowNodes)
	obj.ActualFlowData = string(actualFlowData)
	obj.CurrentFlowNode = dbTemplate.Process.FlowNodes[0].DefineUserOrGroup
	return Db.Create(obj).Error
}

func (obj *WorkOrderInstance) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func GetWorkOrderInstanceById(id int) (*WorkOrderInstance, error) {
	var dbWorkOrderInstance WorkOrderInstance
	err := Db.Where("id = ? ", id).First(&dbWorkOrderInstance).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("WorkOrderInstance不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbWorkOrderInstance, nil
}

func GetWorkOrderInstanceAll() (ps []*WorkOrderInstance, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *WorkOrderInstance) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}

	dbTemplate, _ := GetWorkOrderTemplateById(int(obj.TemplateId))

	if dbTemplate != nil {
		dbTemplate.FillFrontAllData()
		obj.Template = dbTemplate
	}

	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetWorkOrderInstanceByStatusAndCurrentFlowNodes(status string, currentFlowNodes []string, limit, offset int) (objs []*WorkOrderInstance, err error) {
	err = Db.Where("status = ? and current_flow_node IN ?", status, currentFlowNodes).Limit(limit).Offset(offset).Find(&objs).Error
	return
}

func GetWorkOrderInstanceByStatusAndCurrentNode(status string, currentNode string) (objs []*WorkOrderInstance, count int64, err error) {
	query := Db.Model(&WorkOrderInstance{}).Where("status = ? AND current_flow_node = ?", status, currentNode)

	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if count > 0 {
		err = query.Find(&objs).Error
	}
	return
}

func GetWorkOrderInstanceByStatusAndCurrentCount(status string, currentFlowNodes []string) int {
	var count int64
	//Db.Table("work_order_instances").Where("status = ? and current_flow_node IN", status, currentFlowNodes).Count(&count)
	Db.Table("work_order_instances").Where("status = ? and current_flow_node IN ?", status, currentFlowNodes).Count(&count)
	return int(count)
}

func GetWorkOrderInstanceByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*WorkOrderInstance, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return
}

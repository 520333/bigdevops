package models

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type K8sYamlTask struct {
	Model
	Name           string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:yaml任务名称"`
	UserID         uint
	TemplateId     uint        // yaml模板id
	ClusterName    string      `json:"clusterName"` // 集群名称
	Variables      StringArray `json:"variables,omitempty" gorm:"type:text;comment:yaml环境变量 k=v"`
	Status         string      `json:"status" gorm:"comment:当前状态"`
	ApplyResult    string      `json:"applyResult" gorm:"comment:apply后的结果返回值"`
	Key            string      `json:"key" gorm:"-"`            // 前端表格使用
	VariablesFront string      `json:"variablesFront" gorm:"-"` // 前端表格使用
	TemplateName   string      `json:"templateName" gorm:"-"`   // 前端表格使用
	CreateUserName string      `json:"createUserName" gorm:"-"` // 前端表格使用
}

func (obj *K8sYamlTask) Create() error {
	return Db.Create(obj).Error
}

func (obj *K8sYamlTask) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *K8sYamlTask) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *K8sYamlTask) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func GetK8sYamlTaskById(id int) (*K8sYamlTask, error) {
	var dbK8sYamlTask K8sYamlTask
	err := Db.Where("id = ? ", id).First(&dbK8sYamlTask).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("K8sYamlTask不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbK8sYamlTask, nil
}

func GetK8sYamlTaskAll() (ps []*K8sYamlTask, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *K8sYamlTask) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	dbTemp, _ := GetK8sYamlTemplateById(int(obj.TemplateId))
	if dbTemp != nil {
		obj.TemplateName = dbTemp.Name
	}
	if len(obj.Variables) > 0 {
		obj.VariablesFront = strings.Join(obj.Variables, "; ")
	} else {
		obj.VariablesFront = "-"
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetK8sYamlTaskByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*K8sYamlTask, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}
func GetK8sYamlTaskByTemplateId(templateId uint) (objs []*K8sYamlTask, err error) {
	err = Db.Where("template_id = ? ", templateId).Find(&objs).Error
	return
}

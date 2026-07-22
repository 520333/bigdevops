package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type K8sYamlTemplate struct {
	Model
	Name           string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:yaml模板名称"`
	UserID         uint
	Content        string `json:"content,omitempty" gorm:"type:text;comment:yaml原始内容"`
	Key            string `json:"key" gorm:"-"` // 前端表格使用
	CreateUserName string `json:"createUserName" gorm:"-"`
}

func (obj *K8sYamlTemplate) Create() error {
	return Db.Create(obj).Error
}

func (obj *K8sYamlTemplate) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *K8sYamlTemplate) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *K8sYamlTemplate) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func GetK8sYamlTemplateById(id int) (*K8sYamlTemplate, error) {
	var dbK8sYamlTemplate K8sYamlTemplate
	err := Db.Where("id = ? ", id).First(&dbK8sYamlTemplate).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("K8sYamlTemplate不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbK8sYamlTemplate, nil
}

func GetK8sYamlTemplateAll() (ps []*K8sYamlTemplate, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *K8sYamlTemplate) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetK8sYamlTemplateByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*K8sYamlTemplate, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

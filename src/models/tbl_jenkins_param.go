package models

import (
	"gorm.io/gorm/clause"
)

// JenkinsBuildParam 构建参数实体表
type JenkinsBuildParam struct {
	Model
	Name         string   `json:"name" gorm:"type:varchar(128);not null;comment:参数名称"`
	Type         string   `json:"type" gorm:"type:varchar(32);default:'string';comment:类型(string/boolean/choice/reactiveChoice)"`
	DefaultValue string   `json:"defaultValue" gorm:"type:text;comment:默认值"`
	Choices      []string `json:"choices" gorm:"serializer:json;comment:静态下拉选项"`
	Script       string   `json:"script" gorm:"type:text;comment:Groovy动态脚本(reactiveChoice使用)"`
	Description  string   `json:"description" gorm:"type:varchar(255);comment:说明"`
}

func (JenkinsBuildParam) TableName() string {
	return "jenkins_build_param"
}

func (obj *JenkinsBuildParam) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *JenkinsBuildParam) UpdateOne() error {
	var old JenkinsBuildParam
	if err := Db.Where("id = ?", obj.ID).First(&old).Error; err != nil {
		return err
	}
	old.Name = obj.Name
	old.Type = obj.Type
	old.DefaultValue = obj.DefaultValue
	old.Choices = obj.Choices
	old.Script = obj.Script
	old.Description = obj.Description
	return Db.Save(&old).Error
}

func (obj *JenkinsBuildParam) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func GetJenkinsBuildParamList(paramType string, keyword string) ([]*JenkinsBuildParam, error) {
	var objs []*JenkinsBuildParam
	query := Db.Model(&JenkinsBuildParam{})
	if paramType != "" {
		query = query.Where("type = ?", paramType)
	}
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	err := query.Order("id desc").Find(&objs).Error
	return objs, err
}

package models

import (
	"gorm.io/gorm/clause"
)

// JenkinsEnvVar 环境变量实体表
type JenkinsEnvVar struct {
	Model
	EnvGroup    string `json:"envGroup" gorm:"type:varchar(64);default:'global';comment:分组/环境(global/dev/prod)"`
	Key         string `json:"key" gorm:"type:varchar(128);not null;comment:环境变量Key"`
	Value       string `json:"value" gorm:"type:varchar(255);not null;comment:环境变量Value"`
	Description string `json:"description" gorm:"type:varchar(255);comment:说明"`
}

func (JenkinsEnvVar) TableName() string {
	return "jenkins_env_var"
}

func (obj *JenkinsEnvVar) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *JenkinsEnvVar) UpdateOne() error {
	var old JenkinsEnvVar
	if err := Db.Where("id = ?", obj.ID).First(&old).Error; err != nil {
		return err
	}
	old.EnvGroup = obj.EnvGroup
	old.Key = obj.Key
	old.Value = obj.Value
	old.Description = obj.Description
	return Db.Save(&old).Error
}

func (obj *JenkinsEnvVar) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func GetJenkinsEnvVarList(group string, keyword string) ([]*JenkinsEnvVar, error) {
	var objs []*JenkinsEnvVar
	query := Db.Model(&JenkinsEnvVar{})
	if group != "" {
		query = query.Where("env_group = ?", group)
	}
	if keyword != "" {
		query = query.Where("`key` LIKE ? OR `value` LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	err := query.Order("id desc").Find(&objs).Error
	return objs, err
}

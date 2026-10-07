package models

import (
	"gorm.io/gorm/clause"
)

// JenkinsPipelineConfig Jenkins 流水线模版主表
type JenkinsPipelineConfig struct {
	Model
	Name string `json:"name" gorm:"type:varchar(128);not null;comment:流水线配置名称"`
	Lang string `json:"lang" gorm:"type:varchar(32);default:'Vue/TS';comment:适用语言(Vue/TS/Java/Go/Python/PHP)"`
	//AgentNode      string `json:"agentNode" gorm:"type:varchar(64);default:'any';comment:构建节点"`
	Description    string `json:"description" gorm:"type:varchar(255);comment:描述说明"`
	PipelineScript string `json:"pipelineScript" gorm:"type:text;comment:Jenkinsfile(Groovy DSL流水线定义)"`
	CreatedBy      string `json:"createdBy" gorm:"type:varchar(64);comment:创建人"`
}

func (JenkinsPipelineConfig) TableName() string {
	return "jenkins_pipeline_config"
}

func (obj *JenkinsPipelineConfig) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *JenkinsPipelineConfig) UpdateOne() error {
	var old JenkinsPipelineConfig
	if err := Db.Where("id = ?", obj.ID).First(&old).Error; err != nil {
		return err
	}
	old.Name = obj.Name
	old.Lang = obj.Lang
	//old.AgentNode = obj.AgentNode
	old.Description = obj.Description
	old.PipelineScript = obj.PipelineScript
	return Db.Save(&old).Error
}

func (obj *JenkinsPipelineConfig) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func GetJenkinsPipelineConfigById(id uint) (*JenkinsPipelineConfig, error) {
	var dbObj JenkinsPipelineConfig
	err := Db.Where("id = ?", id).First(&dbObj).Error
	if err != nil {
		return nil, err
	}
	return &dbObj, nil
}

func GetJenkinsPipelineConfigList(lang string, keyword string) ([]*JenkinsPipelineConfig, error) {
	var objs []*JenkinsPipelineConfig
	query := Db.Model(&JenkinsPipelineConfig{})
	if lang != "" {
		query = query.Where("lang = ?", lang)
	}
	if keyword != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	err := query.Order("id desc").Find(&objs).Error
	return objs, err
}

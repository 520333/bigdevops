package models

import (
	"gorm.io/gorm/clause"
)

// JenkinsPipelineConfig Jenkins 流水线配置关联主表
type JenkinsPipelineConfig struct {
	Model
	Name           string               `json:"name" gorm:"type:varchar(128);not null;comment:流水线配置名称"`
	Lang           string               `json:"lang" gorm:"type:varchar(32);default:'Vue/TS';comment:适用语言(Vue/TS/Java/Go/Python)"`
	AgentNode      string               `json:"agentNode" gorm:"type:varchar(64);default:'master';comment:构建节点"`
	Description    string               `json:"description" gorm:"type:varchar(255);comment:描述说明"`
	StageIDs       []uint               `json:"stageIds" gorm:"serializer:json;comment:关联Stage的ID列表与顺序"`
	EnvIDs         []uint               `json:"envIds" gorm:"serializer:json;comment:关联环境变量ID列表"`
	ParamIDs       []uint               `json:"paramIds" gorm:"serializer:json;comment:关联构建参数ID列表"`
	Stages         []*JenkinsStage      `json:"stages" gorm:"-"`
	Environments   []*JenkinsEnvVar     `json:"environments" gorm:"-"`
	Parameters     []*JenkinsBuildParam `json:"parameters" gorm:"-"`
	PipelineScript string               `json:"pipelineScript" gorm:"type:text;comment:拼接生成的Groovy脚本"`
	CreatedBy      string               `json:"createdBy" gorm:"type:varchar(64);comment:创建人"`
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
	old.AgentNode = obj.AgentNode
	old.Description = obj.Description
	old.StageIDs = obj.StageIDs
	old.EnvIDs = obj.EnvIDs
	old.ParamIDs = obj.ParamIDs
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
	_ = dbObj.LoadRelations()
	return &dbObj, nil
}

func (obj *JenkinsPipelineConfig) LoadRelations() error {
	if len(obj.StageIDs) > 0 {
		var stages []*JenkinsStage
		_ = Db.Where("id IN ?", obj.StageIDs).Find(&stages).Error
		// 按照 StageIDs 的顺序排序
		stageMap := make(map[uint]*JenkinsStage)
		for _, s := range stages {
			stageMap[s.ID] = s
		}
		orderedStages := make([]*JenkinsStage, 0)
		for _, sid := range obj.StageIDs {
			if s, exists := stageMap[sid]; exists {
				orderedStages = append(orderedStages, s)
			}
		}
		obj.Stages = orderedStages
	}

	if len(obj.EnvIDs) > 0 {
		var envs []*JenkinsEnvVar
		_ = Db.Where("id IN ?", obj.EnvIDs).Find(&envs).Error
		obj.Environments = envs
	}

	if len(obj.ParamIDs) > 0 {
		var params []*JenkinsBuildParam
		_ = Db.Where("id IN ?", obj.ParamIDs).Find(&params).Error
		obj.Parameters = params
	}

	return nil
}

func GetJenkinsPipelineConfigList(lang string, keyword string) ([]*JenkinsPipelineConfig, error) {
	var objs []*JenkinsPipelineConfig
	query := Db.Model(&JenkinsPipelineConfig{})
	if lang != "" {
		query = query.Where("lang = ?", lang)
	}
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	err := query.Order("id desc").Find(&objs).Error
	if err == nil {
		for _, obj := range objs {
			_ = obj.LoadRelations()
		}
	}
	return objs, err
}

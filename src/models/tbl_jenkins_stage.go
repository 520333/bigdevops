package models

import (
	"gorm.io/gorm/clause"
)

// JenkinsStage 独立 Stage 阶段实体表
type JenkinsStage struct {
	Model
	Name        string `json:"name" gorm:"type:varchar(128);not null;comment:Stage名称"`
	CodeKey     string `json:"codeKey" gorm:"type:varchar(64);uniqueIndex;comment:阶段唯一标识"`
	Category    string `json:"category" gorm:"type:varchar(32);default:'general';comment:分类(frontend/backend/general)"`
	AgentType   string `json:"agentType" gorm:"type:varchar(32);default:'none';comment:Agent类型(none/label/docker)"`
	DockerImage string `json:"dockerImage" gorm:"type:varchar(255);comment:Docker镜像"`
	WhenExpr    string `json:"whenExpr" gorm:"type:varchar(255);comment:触发条件表达式"`
	Steps       string `json:"steps" gorm:"type:text;comment:Groovy 步骤脚本指令"`
	Description string `json:"description" gorm:"type:varchar(255);comment:描述说明"`
	OrderNo     int    `json:"orderNo" gorm:"default:0;comment:排序"`
	Enabled     bool   `json:"enabled" gorm:"default:true;comment:默认是否启用"`
}

func (JenkinsStage) TableName() string {
	return "jenkins_stage"
}

func (obj *JenkinsStage) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *JenkinsStage) UpdateOne() error {
	var old JenkinsStage
	if err := Db.Where("id = ?", obj.ID).First(&old).Error; err != nil {
		return err
	}
	old.Name = obj.Name
	old.CodeKey = obj.CodeKey
	old.Category = obj.Category
	old.AgentType = obj.AgentType
	old.DockerImage = obj.DockerImage
	old.WhenExpr = obj.WhenExpr
	old.Steps = obj.Steps
	old.Description = obj.Description
	old.OrderNo = obj.OrderNo
	old.Enabled = obj.Enabled
	return Db.Save(&old).Error
}

func (obj *JenkinsStage) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func GetJenkinsStageById(id uint) (*JenkinsStage, error) {
	var dbObj JenkinsStage
	err := Db.Where("id = ?", id).First(&dbObj).Error
	return &dbObj, err
}

func GetJenkinsStageList(category string, keyword string) ([]*JenkinsStage, error) {
	var objs []*JenkinsStage
	query := Db.Model(&JenkinsStage{})
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if keyword != "" {
		query = query.Where("name LIKE ? OR code_key LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	err := query.Order("order_no asc, id asc").Find(&objs).Error
	return objs, err
}

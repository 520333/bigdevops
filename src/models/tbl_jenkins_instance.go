package models

import (
	"gorm.io/gorm/clause"
)

// JenkinsInstance Jenkins多实例对象
type JenkinsInstance struct {
	Model
	Name                 string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:Jenkins实例名称"`
	URL                  string `json:"url,omitempty" gorm:"type:varchar(255);comment:Jenkins URL地址"`
	Username             string `json:"username,omitempty" gorm:"type:varchar(100);comment:用户名"`
	ApiToken             string `json:"apiToken,omitempty" gorm:"type:text;comment:API Token或密码"`
	Env                  string `json:"env,omitempty" gorm:"comment:环境信息 prod|stage|test"`
	ActionTimeoutSeconds int    `json:"actionTimeoutSeconds,omitempty" gorm:"comment:超时秒数"`

	// 前端探活状态展示字段 (不存库)
	LastProbSuccess bool   `json:"lastProbSuccess" gorm:"-"`
	LastProbErrMsg  string `json:"lastProbErrMsg" gorm:"-"`
}

func (obj *JenkinsInstance) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *JenkinsInstance) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func (obj *JenkinsInstance) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func GetJenkinsInstanceById(id uint) (*JenkinsInstance, error) {
	var dbObj JenkinsInstance
	err := Db.Where("id = ?", id).First(&dbObj).Error
	if err != nil {
		return nil, err
	}
	return &dbObj, nil
}

func GetJenkinsInstanceAll() ([]*JenkinsInstance, error) {
	var objs []*JenkinsInstance
	err := Db.Order("id desc").Find(&objs).Error
	if err != nil {
		return nil, err
	}
	return objs, nil
}

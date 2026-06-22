package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type JobScript struct {
	Model
	Name           string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:任务脚本名称"`
	Lang           string `json:"lang"`
	UserID         uint
	Content        string `json:"content,omitempty" gorm:"comment:脚本原始内容"`
	Key            string `json:"key" gorm:"-"` // 前端表格使用
	CreateUserName string `json:"createUserName" gorm:"-"`
}

func (obj *JobScript) Create() error {
	return Db.Create(obj).Error
}

func (obj *JobScript) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *JobScript) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *JobScript) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}
func GetJobScriptById(id int) (*JobScript, error) {
	var dbJobScript JobScript
	err := Db.Where("id = ? ", id).First(&dbJobScript).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("JobScript不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbJobScript, nil
}

func GetJobScriptAll() (ps []*JobScript, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *JobScript) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetJobScriptByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*JobScript, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

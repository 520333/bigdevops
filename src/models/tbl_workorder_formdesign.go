package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WorkOrderFormDesign 工作审批流
type WorkOrderFormDesign struct {
	Model
	Name           string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:表单设计名称"`
	UserID         uint
	CurrentNodeId  uint   `json:"currentNodeId"`
	FormConfig     string `json:"formConfig,omitempty" gorm:"comment:表单设计大json"`
	Key            string `json:"key" gorm:"-"` // 前端表格使用
	CreateUserName string `json:"createUserName" gorm:"-"`
}

func (obj *WorkOrderFormDesign) Create() error {
	return Db.Create(obj).Error
}

func (obj *WorkOrderFormDesign) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *WorkOrderFormDesign) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *WorkOrderFormDesign) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}
func GetFormDesignById(id int) (*WorkOrderFormDesign, error) {
	var dbWorkOrderFormDesign WorkOrderFormDesign
	err := Db.Where("id = ? ", id).First(&dbWorkOrderFormDesign).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("WorkOrderFormDesign不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbWorkOrderFormDesign, nil
}

func GetFormDesignAll() (ps []*WorkOrderFormDesign, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *WorkOrderFormDesign) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetFormDesignByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*WorkOrderFormDesign, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

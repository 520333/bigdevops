package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WorkOrderTemplate 工作审批流
type WorkOrderTemplate struct {
	Model
	Name         string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:工单模板名称"`
	UserID       uint
	FormDesignID uint
	ProcessID    uint

	Key            string `json:"key" gorm:"-"` // 前端表格使用
	CreateUserName string `json:"createUserName" gorm:"-"`

	ProcessName    string `json:"processName" gorm:"-"`
	FormDesignName string `json:"formDesignName" gorm:"-"`
}

func (obj *WorkOrderTemplate) Create() error {
	return Db.Create(obj).Error
}

func (obj *WorkOrderTemplate) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *WorkOrderTemplate) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *WorkOrderTemplate) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func GetWorkOrderTemplateById(id int) (*WorkOrderTemplate, error) {
	var dbWorkOrderTemplate WorkOrderTemplate
	err := Db.Where("id = ? ", id).First(&dbWorkOrderTemplate).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("WorkOrderTemplate不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbWorkOrderTemplate, nil
}

func GetWorkOrderTemplateAll() (ps []*WorkOrderTemplate, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *WorkOrderTemplate) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}

	dbProcess, _ := GetProcessById(int(obj.ProcessID))
	if dbUser != nil {
		obj.ProcessName = dbProcess.Name
	}

	dbFormDesign, _ := GetFormDesignById(int(obj.FormDesignID))
	if dbUser != nil {
		obj.FormDesignName = dbFormDesign.Name
	}

	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetWorkOrderTemplateByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*WorkOrderTemplate, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

func GetWorkOrderTemplateByFormDesignId(formDesignId int) (*WorkOrderTemplate, error) {
	var dbObj WorkOrderTemplate
	err := Db.Where("form_design_id = ? ", formDesignId).First(&dbObj).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("WorkOrderTemplate不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbObj, nil
}

func GetWorkOrderTemplateByProcessId(process int) (*WorkOrderTemplate, error) {
	var dbObj WorkOrderTemplate
	err := Db.Where("process_id = ? ", process).First(&dbObj).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("WorkOrderTemplate不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbObj, nil
}

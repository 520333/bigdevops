package models

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SystemRole struct {
	Model
	OrderNo   int           `json:"orderNo" gorm:"comment:排序"`
	RoleName  string        `json:"roleName" gorm:"type:varchar(100);uniqueIndex;comment:角色中文名称"`
	RoleValue string        `json:"roleValue" gorm:"type:varchar(100);uniqueIndex;comment:角色值"`
	Remark    string        `json:"remark" gorm:"comment:用户描述"`
	Status    string        `json:"status" gorm:"default:1;comment:角色是否开启 1正常 2冻结"`
	Users     []*SystemUser `json:"users" gorm:"many2many:system_user_roles"`
	Menus     []*SystemMenu `json:"menus" gorm:"many2many:system_role_menus"`
	Apis      []*SystemApi  `json:"apis" gorm:"many2many:system_role_apis"`
	MenuIds   []int         `json:"menuIds" gorm:"-"`
	ApiIds    []int         `json:"apiIds" gorm:"-"`
}

func GetRoleAll() (roles []*SystemRole, err error) {
	err = Db.Preload("Apis").Preload("Menus").Preload("Users").Find(&roles).Error
	return
}

func GetRoleByRoleValue(roleValue string) (*SystemRole, error) {
	var dbRole SystemRole
	err := Db.Where("role_value = ?", roleValue).Preload("Apis").Preload("Menus").First(&dbRole).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("角色不存在：%w", err)
		}
		return nil, fmt.Errorf("数据库错误：%w", err)
	}
	return &dbRole, nil
}

func (obj *SystemRole) CreateOne() error {
	return Db.Create(obj).Error
}

func GetRoleById(id int) (*SystemRole, error) {
	var dbRole SystemRole
	err := Db.Where("id = ? ", id).Preload("Menus").First(&dbRole).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("角色不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbRole, nil
}

func (obj *SystemRole) UpdateMenus(menus []*SystemMenu) error {
	return Db.Transaction(func(tx *gorm.DB) error {
		// 1. 更新角色基本信息
		if err := tx.Model(obj).Updates(obj).Error; err != nil {
			return err
		}
		// 2. 更新中间表
		return tx.Model(obj).Association("Menus").Replace(menus)
	})
}

func (obj *SystemRole) UpdateApis(apis []*SystemApi) error {
	return Db.Transaction(func(tx *gorm.DB) error {
		// 1. 更新角色基本信息
		if err := tx.Model(obj).Updates(obj).Error; err != nil {
			return err
		}
		// 2. 更新中间表
		return tx.Model(obj).Association("Apis").Replace(apis)
	})
}

func (obj *SystemRole) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

package models

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Menu struct {
	Model
	Name  string `json:"name" gorm:"type:varchar(100);uniqueIndex;comment:英文名称"`
	Title string `json:"title" gorm:"comment:中文名称"`
	//Title      string    `json:"title" gorm:"comment:中文名称" validate:"required,min=10,max=20"`
	Pid        int       `json:"pId" gorm:"comment:父级的ID"`
	ParentMenu string    `json:"parentMenu" gorm:"type:varchar(5);comment:父级的菜单"`
	Meta       *MenuMeta `json:"meta" gorm:"-"`
	Icon       string    `json:"icon" gorm:"comment:图标"`
	//DbId       uint      `json:"dbId" gorm:"-"`
	//Id        string  `json:"id" gorm:"-"`
	Type      string  `json:"type" gorm:"type:varchar(5);comment:类型 0=目录 1=子菜单"`
	Show      string  `json:"show" gorm:"type:varchar(5);comment:是否显示 0=禁用 1=启用"`
	OrderNo   int     `json:"orderNo" gorm:"comment:排序"`
	Component string  `json:"component" gorm:"type:varchar(50);comment:前端组件 菜单就是LAYOUT"`
	Redirect  string  `json:"redirect" gorm:"type:varchar(50);comment:显示路径"`
	Path      string  `json:"path" gorm:"type:varchar(50);comment:路由路径"`
	Remark    string  `json:"remark" gorm:"comment:备注"`
	HomePath  string  `json:"homePath" gorm:"comment:登录后的默认首页"`
	Status    string  `json:"status" gorm:"default:1;comment:菜单是否开启 0禁用 1启用"`
	Roles     []*Role `json:"roles" gorm:"many2many:role_menus"`
	Children  []*Menu `json:"children" gorm:"-"`
	Key       uint    `json:"value" gorm:"-"`
	Value     uint    `json:"key" gorm:"-"`
}

type MenuMeta struct {
	Title           string `json:"title" gorm:"-"`
	Icon            string `json:"icon" gorm:"-"`
	ShowMenu        bool   `json:"showMenu" gorm:"-"`
	HideMenu        bool   `json:"hideMenu" gorm:"-"`
	IgnoreKeepAlive bool   `json:"ignoreKeepAlive" gorm:"-"`
}

func GetMenuById(id int) (*Menu, error) {
	var dbMenu Menu
	err := Db.Where("id = ? ", id).Preload("Roles").First(&dbMenu).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("菜单不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMenu, nil
}

func (obj *Menu) UpdateOne() error {
	return Db.Updates(obj).Error
	//return Db.Model(obj).
	//	Select("*").
	//	Omit("created_at", "updated_at", "deleted_at").
	//	Updates(obj).Error
}

func (obj *Menu) CreateOne() error {
	return Db.Create(obj).Error
}
func GetMenuAll() (menus []*Menu, err error) {
	err = Db.Find(&menus).Error
	return
}

func (obj *Menu) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

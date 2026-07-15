package models

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Api struct {
	Model
	Type   string `json:"type" gorm:"type:varchar(5);comment:类型 0=父级 1=子级"`
	Path   string `json:"path" gorm:"type:varchar(100);comment:路由路径"`
	Method string `json:"method" gorm:"type:varchar(50);comment:http请求方法"`
	Pid    int    `json:"pId" gorm:"comment:父级ID 为了给树用的"`
	//Name   string `json:"name" gorm:"type:varchar(100);uniqueIndex;comment:名称"`
	Title    string  `json:"title" gorm:"type:varchar(50);uniqueIndex;comment:名称"`
	Roles    []*Role `json:"roles" gorm:"many2many:role_apis"`
	Key      uint    `json:"key" gorm:"-"`
	Value    uint    `json:"value" gorm:"-"`
	Children []*Api  `json:"children" gorm:"-"`
}

func (obj *Api) Create() error {
	return Db.Create(obj).Error
}

func GetApiAll() (apis []*Api, err error) {
	err = Db.Find(&apis).Error
	return
}

func GetApiById(id int) (*Api, error) {
	var dbApi Api
	err := Db.Where("id = ? ", id).Preload("Roles").First(&dbApi).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("api不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbApi, nil
}

func (obj *Api) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *Api) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *Api) UpdateOne() error {
	return Db.Updates(obj).Error
}

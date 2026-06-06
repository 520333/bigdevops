package models

import (
	"bigdevops/src/common"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// User 基于前端依赖的user 字段
type User struct {
	Model
	UserId   int    `json:"userId" gorm:"comment:用户id"`
	Username string `json:"userName" gorm:"type:varchar(100);uniqueIndex;comment:用户登录名"`
	Password string `json:"password" gorm:"comment:用户登录密码"`
	RealName string `json:"realName" gorm:"comment:用户昵称"`
	//Avatar   string  `json:"avatar" gorm:"comment:头像"`
	Desc     string `json:"desc" gorm:"comment:用户描述"`
	HomePath string `json:"homePath" gorm:"comment:登录后跳转地址"`
	Enable   int    `json:"enable" gorm:"default:1;comment:用户是否被冻结 1正常 2冻结"`
	//Roles    []*Role `json:"roles" gorm:"many2many:user_roles"`
	Roles    []*Role      `json:"roles" gorm:"many2many:user_roles"`
	OpsNodes []*StreeNode `json:"ops_nodes" gorm:"many2many:ops_admins;comment:人员服务树节点"`

	Processes []Process

	RolesFront []string `json:"rolesFront" gorm:"-"`
}

func CheckUserPassword(ru *UserLoginRequest) (*User, error) {
	var dbUser User
	err := Db.Where("username = ? ", ru.Username).Preload("Roles").First(&dbUser).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("用户名不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	ok := common.BcryptCheck(ru.Password, dbUser.Password)
	if ok {
		return &dbUser, nil
	}
	return nil, fmt.Errorf("密码错误")
}

func GetUserByUsername(userName string) (*User, error) {
	var dbUser User
	err := Db.Where("username = ? ", userName).Preload("Roles").Preload("Roles.Menus").First(&dbUser).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("用户名不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbUser, nil
}

func (obj *User) CreateOne() error {
	return Db.Create(obj).Error
}

//	func (obj *User) UpdateMenus(roles []*Role) error {
//		err1 := Db.Updates(obj).Error
//
//		err2 := Db.Model(obj).Association("Roles").Replace(roles)
//		if err1 == nil && err2 == nil {
//			return nil
//		} else {
//			return fmt.Errorf("更新本体%w 更新关联%w", err1, err2)
//		}
//	}

func (obj *User) UpdateOne(roles []*Role) error {
	// 使用事务确保两步操作“同生共死”
	return Db.Transaction(func(tx *gorm.DB) error {
		// 1. 更新用户表基本字段
		// 使用 Omit("password") 确保不小心传空的密码不会覆盖数据库原密码
		db := tx.Model(obj).Where("id = ?", obj.ID)
		if obj.Password == "" {
			db = db.Omit("password")
		}

		if err := db.Updates(obj).Error; err != nil {
			return err // 返回错误，事务会自动回滚
		}

		// 2. 更新多对多关联 (中间表)
		// Replace 会自动计算差异并同步 user_roles 表
		if err := tx.Model(obj).Association("Roles").Replace(roles); err != nil {
			return err
		}

		return nil // 返回 nil，事务会自动提交
	})
}

func GetUserAll() (users []*User, err error) {
	err = Db.Preload("Roles").Find(&users).Error
	return
}

func GetUserById(id int) (*User, error) {
	var dbUser User
	err := Db.Where("id = ? ", id).Preload("Roles").First(&dbUser).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbUser, nil
}

func GetUserByName(name string) (*User, error) {
	var dbUser User
	err := Db.Where("username = ? ", name).Preload("Roles").First(&dbUser).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbUser, nil
}

func (obj *User) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

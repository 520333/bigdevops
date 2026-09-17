package models

import (
	"bigdevops/src/common"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SystemUser 基于前端依赖的user 字段
type SystemUser struct {
	Model
	// UserId      int    `json:"userId,omitempty" gorm:"comment:用户id"`
	Username    string `json:"userName" gorm:"type:varchar(100);uniqueIndex;comment:用户登录名"`
	Password    string `json:"-" gorm:"comment:用户登录密码"`
	ReqPassword string `json:"password,omitempty" gorm:"-"` //仅用于接收前端 JSON 中的 password 传参，不涉及数据库存取

	Email  string `json:"email" gorm:"comment:用户邮箱"`
	Mobile string `json:"mobile,omitempty" gorm:"type:varchar(20);comment:用户手机号"`

	RealName           string                          `json:"realName" gorm:"comment:用户昵称"`
	Avatar             string                          `json:"avatar,omitempty" gorm:"type:varchar(500);comment:用户头像URL"`
	Desc               string                          `json:"desc,omitempty" gorm:"comment:用户描述"`
	FeiShuUserId       string                          `json:"feiShuUserId,omitempty" gorm:"comment:飞书userid"`
	HomePath           string                          `json:"homePath" gorm:"comment:登录后跳转地址"`
	Enable             int                             `json:"enable" gorm:"default:1;comment:用户是否被冻结 1正常 2冻结"`
	Roles              []*SystemRole                   `json:"roles,omitempty" gorm:"many2many:system_user_roles"`
	OpsNodes           []*StreeNode                    `json:"ops_nodes,omitempty" gorm:"many2many:resource_stree_ops_admins;comment:人员服务树节点"`
	StaticReceiveUsers []*MonitorAlertManagerSendGroup `json:"staticReceiveUsers,omitempty" gorm:"many2many:monitor_alert_static_receive_users;comment:人员告警组节点"`
	FirstUpgradeUsers  []*MonitorAlertManagerSendGroup `json:"firstUpgradeUsers,omitempty" gorm:"many2many:monitor_alert_first_upgrade_users;comment:人员第一告警组节点"`
	MonitorOnDutyGroup []*MonitorOndutyGroup           `json:"monitorOnDutyGroup,omitempty" gorm:"many2many:monitor_onduty_users;comment:值班人列表"`

	Processes   []WorkOrderProcess    `json:"-" gorm:"foreignKey:UserID"`
	FormDesigns []WorkOrderFormDesign `json:"-" gorm:"foreignKey:UserID"`

	RolesFront []string `json:"rolesFront,omitempty" gorm:"-"`
}

// UserCreateRequest 新增用户专用的请求结构体
type UserCreateRequest struct {
	Username   string   `json:"userName" binding:"required"`
	Password   string   `json:"password" binding:"required"` // 这里正常接收密码
	RealName   string   `json:"realName"`
	Desc       string   `json:"desc"`
	RolesFront []string `json:"rolesFront"`
}

func CheckUserPassword(ru *UserLoginRequest) (*SystemUser, error) {
	var dbUser SystemUser
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

func GetUserByUsername(userName string) (*SystemUser, error) {
	var dbUser SystemUser
	err := Db.Where("username = ? ", userName).Preload("Roles").Preload("Roles.Menus").First(&dbUser).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("用户名不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbUser, nil
}

func (obj *SystemUser) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *SystemUser) UpdateOne(roles []*SystemRole) error {
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
		// Replace 会自动计算差异并同步 system_user_roles 表
		if err := tx.Model(obj).Association("Roles").Replace(roles); err != nil {
			return err
		}

		return nil // 返回 nil，事务会自动提交
	})
}

func GetUserAll() (users []*SystemUser, err error) {
	err = Db.Preload("Roles").Find(&users).Error
	return
}

func GetUserById(id int) (*SystemUser, error) {
	var dbUser SystemUser
	err := Db.Where("id = ? ", id).Preload("Roles").First(&dbUser).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbUser, nil
}

func GetUserByName(name string) (*SystemUser, error) {
	var dbUser SystemUser
	err := Db.Where("username = ? ", name).Preload("Roles").First(&dbUser).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbUser, nil
}

func (obj *SystemUser) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *SystemUser) UpdateEnable() error {
	return Db.Model(obj).Select("Enable").Updates(obj).Error
}

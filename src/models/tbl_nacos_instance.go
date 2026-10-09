package models

import (
	"gorm.io/gorm/clause"
)

// NacosInstance Nacos 服务实例数据模型
type NacosInstance struct {
	Model
	Name        string `json:"name,omitempty" gorm:"type:varchar(100);not null;comment:实例名称"`
	EnvKey      string `json:"envKey,omitempty" gorm:"type:varchar(50);not null;default:'dev';comment:环境标识(dev/test/uat/pre/prod)"`
	ServerAddr  string `json:"serverAddr,omitempty" gorm:"type:varchar(255);not null;comment:服务器地址(IP或域名)"`
	Port        uint64 `json:"port,omitempty" gorm:"default:8848;comment:Nacos端口"`
	NamespaceId string `json:"namespaceId,omitempty" gorm:"type:varchar(128);default:'';comment:默认命名空间ID"`
	Username    string `json:"-" gorm:"type:varchar(100);default:'';comment:访问账号"`
	Password    string `json:"-" gorm:"type:varchar(255);default:'';comment:访问密码"`
	ReqPassword string `json:"password,omitempty" gorm:"-"` // 仅用于接收前端 JSON 中的 password 传参，列表响应时自动忽略

	// 状态与探活字段
	Status     string `json:"status,omitempty" gorm:"type:varchar(30);default:'untested';comment:连通状态(online/offline/untested)"`
	LastTestAt string `json:"lastTestAt,omitempty" gorm:"type:varchar(50);comment:最后测试连通时间"`
	Remark     string `json:"remark,omitempty" gorm:"type:varchar(255);default:'';comment:备注说明"`

	// 前端探活状态展示字段 (对齐 Jenkins 实例规范)
	LastProbSuccess bool   `json:"lastProbSuccess" gorm:"-"`
	LastProbErrMsg  string `json:"lastProbErrMsg,omitempty" gorm:"-"`
}

// TableName 表名
func (NacosInstance) TableName() string {
	return "nacos_instances"
}

func (obj *NacosInstance) CreateOne() error {
	if obj.Password == "" && obj.ReqPassword != "" {
		obj.Password = obj.ReqPassword
	}
	return Db.Create(obj).Error
}

func (obj *NacosInstance) UpdateOne() error {
	if obj.Password == "" && obj.ReqPassword != "" {
		obj.Password = obj.ReqPassword
	}
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func (obj *NacosInstance) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

// NacosInstanceQueryParam 查询过滤参数
type NacosInstanceQueryParam struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	EnvKey   string `form:"envKey"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
}

// GetNacosInstanceById 根据 ID 获取实例
func GetNacosInstanceById(id uint) (*NacosInstance, error) {
	var dbObj NacosInstance
	err := Db.Where("id = ?", id).First(&dbObj).Error
	if err != nil {
		return nil, err
	}
	dbObj.LastProbSuccess = (dbObj.Status == "online")
	return &dbObj, nil
}

// GetNacosInstanceList 分页查询 Nacos 实例列表
func GetNacosInstanceList(p *NacosInstanceQueryParam) ([]*NacosInstance, int64, error) {
	var list []*NacosInstance
	var total int64

	db := Db.Model(&NacosInstance{})
	if p != nil {
		if p.EnvKey != "" {
			db = db.Where("env_key = ?", p.EnvKey)
		}
		if p.Status != "" {
			db = db.Where("status = ?", p.Status)
		}
		if p.Keyword != "" {
			key := "%" + p.Keyword + "%"
			db = db.Where("name LIKE ? OR server_addr LIKE ? OR remark LIKE ?", key, key, key)
		}
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := 1
	pageSize := 10
	if p != nil {
		if p.Page > 0 {
			page = p.Page
		}
		if p.PageSize > 0 {
			pageSize = p.PageSize
		}
	}
	offset := (page - 1) * pageSize

	err := db.Order("id desc").Offset(offset).Limit(pageSize).Find(&list).Error
	if err == nil {
		for _, item := range list {
			item.LastProbSuccess = (item.Status == "online")
		}
	}
	return list, total, err
}

// DeleteNacosInstance 删除实例
func DeleteNacosInstance(id uint) error {
	obj := &NacosInstance{Model: Model{ID: id}}
	return obj.DeleteOne()
}

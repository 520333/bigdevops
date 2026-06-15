package models

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ResourceRds 数据库资源模型
type ResourceRds struct {
	Model
	ResourceCommon
	Engine            string `json:"engine,omitempty" gorm:"comment:数据库类型 MySQL mariaDB postgresql redis"`
	DBInstanceId      string `json:"DBInstanceId,omitempty" gorm:"comment:实例ID"`
	Name              string `json:"name,omitempty" gorm:"comment:实例名称/别名"`
	DBInstanceNetType string `json:"DBInstanceNetType,omitempty" gorm:"comment:实例网络类型, Internet:外网 Intranet:内网连接"`
	DBInstanceClass   string `json:"DBInstanceClass,omitempty" gorm:"comment:实例规格,rds.mys2.small"`
	DBInstanceType    string `json:"DBInstanceType,omitempty" gorm:"comment:实例类型是否主备。Primary|Readonly|Guard|Temp"`
	EngineVersion     string `json:"EngineVersion,omitempty" gorm:"comment:数据库版本"`
	PayType           string `json:"PayType,omitempty" gorm:"comment:付费类型 包年包月 | 按量付费"`
	DBInstanceStatus  string `json:"DBInstanceStatus,omitempty" gorm:"comment:实例状态，running | stop"`

	VpcId string `json:"VpcId,omitempty" gorm:"comment:专有网络VPC ID"`

	CreationTime *time.Time `json:"CreationTime,omitempty" gorm:"comment:实例创建时间。以 ISO 8601 为标准，并使用 UTC+0 时间，格式为 yyyy-MM-ddTHH:mmZ"`
	ExpiredTime  *time.Time `json:"ExpiredTime,omitempty" gorm:"comment:过期时间。以 ISO 8601 为标准，并使用 UTC+0 时间，格式为 yyyy-MM-ddTHH:mmZ"`

	Host string `json:"host,omitempty" gorm:"comment:连接地址/IP"`
	Port int    `json:"port,omitempty" gorm:"comment:连接端口"`

	// 💡 如果你的 LB 也需要挂载到服务树上（像 ECS 一样），可以加上多对多关联
	BindNodes []*StreeNode `json:"bind_nodes,omitempty" gorm:"many2many:bind_rdss;comment:绑定的服务树节点"`
}

func (obj *ResourceRds) GenHash() string {
	h := md5.New()

	// Corrected format string and simplified array formatting
	hashStr := fmt.Sprintf("%s_%s_%s_%s_%s",
		obj.Vendor,
		obj.DBInstanceId,
		obj.DBInstanceClass,
		obj.EngineVersion,
		obj.DBInstanceStatus,
	)

	h.Write([]byte(hashStr))
	return hex.EncodeToString(h.Sum(nil))
}

func (obj *ResourceRds) Create() error {
	return Db.Create(obj).Error
}

func (obj *ResourceRds) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *ResourceRds) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *ResourceRds) UpdateOne() error {
	return Db.Select("*").
		Omit("id", "created_at").
		// FIXED: Changed from load_balancer_id to db_instance_id
		Where("db_instance_id = ?", obj.DBInstanceId).
		Updates(obj).Error
}

func (obj *ResourceRds) UpdateBindNodes(nodes []*StreeNode) error {
	return Db.Model(obj).Association("BindNodes").Replace(nodes)
	//return Db.Where("id = ?", obj.ID).Updates(obj).Error

}

func GetResourceRdsAll() (re []*ResourceRds, err error) {
	// 💡 修复：1. 必须使用结构体字段名 BindNodes  2. Preload 最好放在 Find 前面
	err = Db.Preload("BindNodes").Find(&re).Error
	return
}

func GetResourceRdsByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*ResourceRds, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

func GetResourceRdsById(id string) (*ResourceRds, error) {
	var dbResourceRds ResourceRds

	// 💡 修复：将 load_balancer_id = ? 改为 id = ?
	err := Db.Where("id = ? ", id).Preload("BindNodes").First(&dbResourceRds).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("ResourceLb不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbResourceRds, nil
}

func GetResourceRdsByInstanceId(instanceId string) (*ResourceRds, error) {
	var dbResourceRds ResourceRds
	// FIXED: Changed from load_balancer_id to db_instance_id
	err := Db.Where("db_instance_id = ? ", instanceId).Preload("BindNodes").First(&dbResourceRds).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("ResourceRds不存在") // Fixed error message
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbResourceRds, nil
}

func GetResourceRdsUidAndHash() (map[string]string, error) {
	var objs []*ResourceRds
	err := Db.Find(&objs).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, h := range objs {
		// FIXED: Changed from h.LoadBalancerId to h.DBInstanceId
		m[h.DBInstanceId] = h.Hash
	}
	return m, nil
}

func GetResourceRdsByHostOrIp(val string) (*ResourceRds, error) {
	var rds ResourceRds
	// RDS 的地址存在 Host 字段里
	err := Db.Where("host LIKE ?", "%"+val+"%").First(&rds).Error
	if err != nil {
		return nil, err
	}
	return &rds, nil
}

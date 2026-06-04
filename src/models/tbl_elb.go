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

// ResourceElb 负载均衡器资源模型
type ResourceElb struct {
	Model
	ResourceCommon
	// --- 核心标识 ---
	LoadBalancerId    string `json:"loadBalancerId" gorm:"uniqueIndex;type:varchar(100);comment:实例id"`
	LoadBalancerName  string `json:"loadBalancerName"`
	LoadBalancerType  string `json:"loadBalancerType" gorm:"comment:LB实例类型：alb、nlb、clb"`
	BandwidthCapacity int    `json:"bandwidthCapacity,omitempty" gorm:"comment:带宽包上限"`
	VpcId             string `json:"VpcId,omitempty" gorm:"comment:专有网络VPC ID"`
	AddressType       string `json:"addressType,omitempty" gorm:"comment:网络类型(internet公网/intranet私网)"`
	Status            string `json:"status,omitempty" gorm:"type:varchar(50);comment:实例状态(如active/inactive)"`

	PrivateIpAddress  StringArray `json:"PrivateIpAddress,omitempty" gorm:"comment:私有 IP 地址列表：[172.17.**.**]"`
	PublicIpAddresses StringArray `json:"PublicIpAddresses,omitempty" gorm:"comment:公网 IP 地址列表：[1.1.1.1]"`
	SecurityGroupIds  StringArray `json:"SecurityGroupIds,omitempty" gorm:"comment:安全组id"`
	DNSName           string      `json:"DNSName,omitempty" gorm:"comment:DNS解析地址"`
	CreationTime      *time.Time  `json:"CreationTime,omitempty" gorm:"comment:实例创建时间。以 ISO 8601 为标准，并使用 UTC+0 时间，格式为 yyyy-MM-ddTHH:mmZ"`

	BandwidthPackageId int `json:"bandwidthPackageId,omitempty" gorm:"comment:绑定的带宽包"`

	CrossZoneEnabled bool `json:"crossZoneEnabled"`

	// 💡 如果你的 LB 也需要挂载到服务树上（像 ECS 一样），可以加上多对多关联
	BindNodes []*StreeNode `json:"bind_nodes,omitempty" gorm:"many2many:bind_elbs;comment:绑定的服务树节点"`
}

func (obj *ResourceElb) GenHash() string {
	h := md5.New()

	// Corrected format string and simplified array formatting
	hashStr := fmt.Sprintf("%s_%s_%d_%s_%s_%s_%v_%v",
		obj.LoadBalancerId,
		obj.LoadBalancerType,
		obj.BandwidthCapacity,
		obj.LoadBalancerName,
		obj.Status,
		obj.AccountName,
		obj.PublicIpAddresses,
		obj.PrivateIpAddress,
	)

	h.Write([]byte(hashStr))
	return hex.EncodeToString(h.Sum(nil))
}

func (obj *ResourceElb) Create() error {
	return Db.Create(obj).Error
}

func (obj *ResourceElb) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}
func DeleteResourceOneByLbInstanceId(iid string) error {
	return Db.Select(clause.Associations).Unscoped().Where("load_balancer_id = ?", iid).Delete(&ResourceElb{}).Error

}
func (obj *ResourceElb) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *ResourceElb) UpdateOne() error {
	//return Db.Updates(obj).Error
	//return Db.Where("id = ?", obj.ID).Updates(obj).Error
	//return Db.Where("instance_id = ?", obj.InstanceId).Updates(obj).Error
	return Db.Select("*").
		Omit("id", "created_at").
		Where("load_balancer_id = ?", obj.LoadBalancerId).
		Updates(obj).Error

}

func (obj *ResourceElb) UpdateBindNodes(nodes []*StreeNode) error {
	return Db.Model(obj).Association("BindNodes").Replace(nodes)
	//return Db.Where("id = ?", obj.ID).Updates(obj).Error

}

func GetResourceELbAll() (re []*ResourceElb, err error) {
	// 💡 修复：1. 必须使用结构体字段名 BindNodes  2. Preload 最好放在 Find 前面
	err = Db.Preload("BindNodes").Find(&re).Error
	return
}

func GetResourceLbByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*ResourceElb, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

//func GetResourceELBById(id string) (*ResourceElb, error) {
//	var dbResourceLb ResourceElb
//	err := Db.Where("load_balancer_id = ? ", id).Preload("BindNodes").First(&dbResourceLb).Error
//	if err != nil {
//		if err == gorm.ErrRecordNotFound {
//			return nil, fmt.Errorf("ResourceLb不存在")
//		}
//		return nil, fmt.Errorf("数据库错误%v", err)
//	}
//	return &dbResourceLb, nil
//}

func GetResourceELBById(id string) (*ResourceElb, error) {
	var dbResourceLb ResourceElb

	// 💡 修复：将 load_balancer_id = ? 改为 id = ?
	err := Db.Where("id = ? ", id).Preload("BindNodes").First(&dbResourceLb).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("ResourceLb不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbResourceLb, nil
}

func GetResourceLbByInstanceId(instanceId string) (*ResourceElb, error) {
	var dbResourceLb ResourceElb
	// 注意这里改成了 load_balancer_id
	err := Db.Where("load_balancer_id = ? ", instanceId).Preload("BindNodes").First(&dbResourceLb).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("ResourceLb不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbResourceLb, nil
}

func GetResourceLbUidAndHash() (map[string]string, error) {
	var objs []*ResourceElb
	err := Db.Find(&objs).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, h := range objs {
		m[h.LoadBalancerId] = h.Hash
	}
	return m, nil

}
func GetResourceElbByDnsName(dnsName string) (*ResourceElb, error) {
	var elb ResourceElb
	// 使用精确匹配，因为 ELB 的 dns_name 是唯一的
	err := Db.Where("dns_name = ?", dnsName).First(&elb).Error
	if err != nil {
		return nil, err
	}
	return &elb, nil
}

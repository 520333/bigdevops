package models

import (
	"bigdevops/src/common"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ResourceEcs 参考阿里云ecs表结构
type ResourceEcs struct {
	Model
	ResourceCommon
	// 核心字段
	InstanceId string `json:"InstanceId,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:实例id"`
	//InstanceName string `json:"InstanceName,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:实例名称，支持通配符*进行模糊搜索"`
	InstanceName string `json:"title,omitempty" gorm:"type:varchar(100);comment:实例名称，支持通配符*进行模糊搜索"`
	InstanceType string `json:"InstanceType,omitempty" gorm:"comment:实例规格"`

	// 绑定的叶子节点 多对多
	BindNodes []*StreeNode `json:"bind_nodes,omitempty" gorm:"many2many:bind_ecss;"`
	// 常见字段
	VpcId  string `json:"VpcId,omitempty" gorm:"comment:专有网络VPC ID"`
	VmType int    `json:"VmType" gorm:"default:1;comment:资源种类: 1=云虚拟机, 2=物理机, 3=容器"`
	OSType string `json:"OSType,omitempty" gorm:"comment:操作系统类型"`
	//ZoneId      string `json:"ZoneId,omitempty" gorm:"comment:实例可用区"`
	Status      string `json:"Status,omitempty" gorm:"comment:实例状态。取值范围：Pending创建中| Running运行中 |Starting启动中 |Stopping停止中|Stopped已停止。"`
	Cpu         int    `json:"Cpu,omitempty" gorm:"comment:vCPU数"`
	Memory      int    `json:"Memory,omitempty" gorm:"comment:内存大小 单位为MiB"`
	OSName      string `json:"OSName,omitempty" gorm:"comment:实例操作系统名称"`
	Description string `json:"Description,omitempty" gorm:"comment:实例操作系统名称"`
	ImageId     string `json:"ImageId,omitempty" gorm:"comment:镜像模板"`
	HostName    string `json:"HostName,omitempty" gorm:"type:varchar(100);comment:主机名"`
	Key         string `json:"key" gorm:"-"`

	// 字符串数组类型
	SecurityGroupIds  StringArray `json:"SecurityGroupIds,omitempty" gorm:"comment:安全组id"`
	PrivateIpAddress  StringArray `json:"PrivateIpAddress,omitempty" gorm:"comment:私有 IP 地址列表：[172.17.**.**]"`
	PublicIpAddresses StringArray `json:"PublicIpAddresses,omitempty" gorm:"comment:公网 IP 地址列表：[1.1.1.1]"`
	NetworkInterfaces StringArray `json:"NetworkInterfaces,omitempty" gorm:"comment:弹性网卡id集合[cni-xx1,cni-xx2]"`
	DiskIds           StringArray `json:"DiskIds,omitempty" gorm:"comment:云硬盘或本地盘id"`

	// 时间字段
	StartTime       *time.Time `json:"StartTime,omitempty" gorm:"comment:实例最近一次的启动时间。以 ISO 8601 为标准，并使用 UTC+0 时间，格式为 yyyy-MM-ddTHH:mmZ"`
	CreationTime    *time.Time `json:"CreationTime,omitempty" gorm:"comment:实例创建时间。以 ISO 8601 为标准，并使用 UTC+0 时间，格式为 yyyy-MM-ddTHH:mmZ"`
	ExpiredTime     *time.Time `json:"ExpiredTime,omitempty" gorm:"comment:过期时间。以 ISO 8601 为标准，并使用 UTC+0 时间，格式为 yyyy-MM-ddTHH:mmZ"`
	AutoReleaseTime *time.Time `json:"AutoReleaseTime,omitempty"`
	LastInvokedTime *time.Time `json:"LastInvokedTime,omitempty"`
}
type EcsBuyWorkOrder struct {
	Vendor       string `json:"vendor"`
	Num          int    `json:"num"`
	BindNode     string `json:"bindNode"`
	InstanceType string `json:"instance_type"`
	HostNames    string `json:"hostnames"`
}

func (obj *ResourceEcs) GenHash() string {
	h := md5.New()
	hashStr := fmt.Sprintf("%d_%d_%s_%s_%s_%s_%s_%s_%s_%s",
		obj.Cpu,
		obj.Memory,
		obj.Status,
		obj.InstanceName,
		obj.InstanceType,
		obj.AccountName,
		fmt.Sprintf("%v", obj.PublicIpAddresses), // 加上公网IP
		fmt.Sprintf("%v", obj.PrivateIpAddress),  // 加上内网IP
		fmt.Sprintf("%v", obj.NetworkInterfaces),
		fmt.Sprintf("%v", obj.DiskIds),
	)
	h.Write([]byte(hashStr))
	//h.Write([]byte(strconv.Itoa(rh.Cpu)))
	//h.Write([]byte(strconv.Itoa(rh.Memory)))
	return hex.EncodeToString(h.Sum(nil))

}

func (obj *ResourceEcs) Create() error {
	return Db.Create(obj).Error
}

func (obj *ResourceEcs) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}
func DeleteResourceOneByEcsInstanceId(iid string) error {
	return Db.Select(clause.Associations).Unscoped().Where("instance_id = ?", iid).Delete(&ResourceEcs{}).Error

}
func (obj *ResourceEcs) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *ResourceEcs) UpdateOne() error {
	//return Db.Updates(obj).Error
	//return Db.Where("id = ?", obj.ID).Updates(obj).Error
	//return Db.Where("instance_id = ?", obj.InstanceId).Updates(obj).Error
	return Db.Select("*").
		Omit("id", "created_at").
		Where("instance_id = ?", obj.InstanceId).
		Updates(obj).Error

}

func (obj *ResourceEcs) UpdateBindNodes(nodes []*StreeNode) error {
	return Db.Model(obj).Association("BindNodes").Replace(nodes)
	//return Db.Where("id = ?", obj.ID).Updates(obj).Error

}

func GetResourceEcsAll() (re []*ResourceEcs, err error) {
	err = Db.Find(&re).Preload("BindNodes").Error
	return
}

func GetResourceEcsByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*ResourceEcs, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}
func GetResourceEcsById(id int) (*ResourceEcs, error) {
	var dbResourceEcs ResourceEcs
	err := Db.Where("id = ? ", id).Preload("BindNodes").First(&dbResourceEcs).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("ResourceEcs不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbResourceEcs, nil
}
func GetResourceEcsByInstanceId(instanceId string) (*ResourceEcs, error) {
	var dbResourceEcs ResourceEcs
	err := Db.Where("instance_id = ? ", instanceId).Preload("BindNodes").First(&dbResourceEcs).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf(common.ERR_ECS_NOT_FOUND)
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbResourceEcs, nil
}

func GetResourceEcsUidAndHash() (map[string]string, error) {
	var objs []*ResourceEcs
	err := Db.Where("vm_type = 1").Find(&objs).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, h := range objs {
		m[h.InstanceId] = h.Hash
	}
	return m, nil

}

func GetResourceEcsByIp(ip string) (*ResourceEcs, error) {
	var ecs ResourceEcs
	// 使用 LIKE 匹配，确保不管 IP 是单独存在还是在列表中都能被查到
	// 同时也搜索私网 IP 和公网 IP
	err := Db.Where("public_ip_addresses LIKE ? OR private_ip_address LIKE ?", "%"+ip+"%", "%"+ip+"%").First(&ecs).Error
	if err != nil {
		return nil, err
	}
	return &ecs, nil
}

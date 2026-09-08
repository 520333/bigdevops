package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type ResourceDns struct {
	Model

	// 基础资产信息
	Vendor string `json:"vendor" gorm:"index"` // godaddy, dynadot
	Domain string `json:"domain" gorm:"index"` // 主域名
	Name   string `json:"name" gorm:"index"`   // 主机记录 (如 www, @)
	Type   string `json:"type" gorm:"index"`   // 记录类型 (A, CNAME, TXT)
	Value  string `json:"value"`               // 记录值 (IP 或 别名)
	TTL    int    `json:"ttl"`                 // 生存时间
	Hash   string `json:"hash" gorm:"index"`   // 用于快速判断变更的 Hash

	// 关联信息
	AssociatedInstanceId string       `json:"associated_instance_id" gorm:"index"` // 关联的 ECS/ELB ID
	EcsInstanceId        string       `json:"ecs_instance_id" gorm:"index"`        // 关联的 ECS 实例 ID (修改后) // 关联的 ECS/ELB ID
	BindNodes            []*StreeNode `json:"bind_nodes,omitempty" gorm:"many2many:resource_stree_bind_dnss;comment:绑定的服务树节点"`
}

func (r *ResourceDns) GenHash() string {
	raw := fmt.Sprintf("%s-%s-%s-%s-%d", r.Domain, r.Name, r.Type, r.Value, r.TTL)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// CreateOne 入库
func (r *ResourceDns) CreateOne() error {
	return Db.Create(r).Error
}

// UpdateOne 更新
func (r *ResourceDns) UpdateOne() error {
	return Db.Save(r).Error
}

// DeleteOne 删除
func (r *ResourceDns) DeleteOne() error {
	return Db.Delete(r).Error
}

// GetResourceDnsUidAndHash 获取当前所有 DNS 记录的 Hash 映射
func GetResourceDnsUidAndHash() (map[string]string, error) {
	var list []ResourceDns
	err := Db.Find(&list).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, item := range list {
		// UID 确保唯一性: 域名 + 主机记录 + 类型
		uid := fmt.Sprintf("%s-%s-%s", item.Domain, item.Name, item.Type)
		result[uid] = item.Hash
	}
	return result, nil
}

// GetResourceDnsByUid 根据 UID (Domain-Name-Type) 获取记录
func GetResourceDnsByUid(domain, name, rtype string) (*ResourceDns, error) {
	var res ResourceDns
	err := Db.Where("domain = ? AND name = ? AND type = ?", domain, name, rtype).First(&res).Error
	return &res, err
}

func (r *ResourceDns) UpdateBindNodes(nodes []*StreeNode) error {
	return Db.Model(r).Association("BindNodes").Replace(nodes)
}

func GetResourceDnsById(id string) (*ResourceDns, error) {
	var res ResourceDns
	err := Db.Where("id = ?", id).Preload("BindNodes").First(&res).Error
	return &res, err
}

func GetResourceDnsAll() ([]*ResourceDns, error) {
	var list []*ResourceDns
	err := Db.Preload("BindNodes").Find(&list).Error
	return list, err
}

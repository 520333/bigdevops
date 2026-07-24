package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type K8sProject struct {
	Model
	Name   string `json:"name,omitempty" gorm:"uniqueIndex:idx_cluster_name;type:varchar(100);comment:项目英文名称"`
	NameZh string `json:"nameZh,omitempty" gorm:"uniqueIndex:idx_cluster_name_zh;type:varchar(100);comment:项目中文名称"`

	Cluster    string `json:"cluster" gorm:"uniqueIndex:idx_cluster_name;uniqueIndex:idx_cluster_name_zh;type:varchar(100);comment:项目绑定的集群名称"`
	TreeNodeId uint   `json:"treeNodeId" gorm:"comment:绑定到哪个节点 面向第三层级"` // 服务树节点
	UserID     uint

	K8sApps        []K8sApp `json:"k8sApps"`
	CreateUserName string   `json:"createUserName" gorm:"-"`
	NodePath       string   `json:"nodePath" gorm:"-"`
	Key            string   `json:"key" gorm:"-"` // 前端表格使用
	AppNum         int      `json:"appNum" gorm:"-"`
}

func (obj *K8sProject) Create() error {
	return Db.Create(obj).Error
}

func (obj *K8sProject) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *K8sProject) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *K8sProject) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func GetK8sProjectById(id int) (*K8sProject, error) {
	var dbProject K8sProject

	err := Db.Where("id = ? ", id).Preload("K8sApps").First(&dbProject).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("project不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbProject, nil
}

func (obj *K8sProject) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)
	obj.AppNum = len(obj.K8sApps)
	for i := range obj.K8sApps {
		_ = obj.K8sApps[i].FillFrontAllData()
	}
	if obj.TreeNodeId > 0 {
		node, err := GetStreeNodeById(int(obj.TreeNodeId))
		if err == nil && node != nil {
			_ = node.GetFullNodePath()
			obj.NodePath = node.NodePath
		}
	}
}

func GetK8sProjectAll() (obj []*K8sProject, err error) {
	err = Db.Model(&K8sProject{}).Preload("K8sApps").Find(&obj).Error
	return
}

func GetK8sProjectByIdsWithLimitOffset(ids []int, limit, offset int) (obj []*K8sProject, err error) {
	if len(ids) == 0 {
		return nil, nil
	}
	err = Db.Where("id IN ?", ids).Limit(limit).Offset(offset).Preload("K8sApps").Find(&obj).Error
	return
}

func DeleteK8sProjectById(id int) error {
	return Db.Unscoped().Delete(&K8sProject{}, id).Error
}

// GetK8sProjectListByNameAndCreator 分页查询，支持按名称和创建人模糊查询
func GetK8sProjectListByNameAndCreator(name, creator string, limit, offset int) (obj []*K8sProject, err error) {
	query := Db.Model(&K8sProject{}).Preload("K8sApps")

	// 1. 按名称模糊查询
	if name != "" {
		query = query.Where("k8s_projects.name LIKE ?", "%"+name+"%")
	}

	// 2. 按创建人模糊查询 (关联 User 表)
	if creator != "" {
		query = query.Joins("left join users on users.id = k8s_projects.user_id").
			Where("users.username LIKE ? OR users.real_name LIKE ?", "%"+creator+"%", "%"+creator+"%")
	}

	err = query.Limit(limit).Offset(offset).Find(&obj).Error
	return
}

// GetK8sProjectCountByNameAndCreator 对应统计总数
func GetK8sProjectCountByNameAndCreator(name, creator string) (int64, error) {
	var count int64
	query := Db.Model(&K8sProject{}).Joins("left join users on users.id = k8s_projects.user_id")

	if name != "" {
		query = query.Where("k8s_projects.name LIKE ?", "%"+name+"%")
	}
	if creator != "" {
		query = query.Where("users.username LIKE ? OR users.real_name LIKE ?", "%"+creator+"%", "%"+creator+"%")
	}

	err := query.Count(&count).Error
	return count, err
}

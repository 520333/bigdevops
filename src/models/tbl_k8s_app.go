package models

import (
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	corev1 "k8s.io/api/core/v1"
)

// K8sApp 面向运维的k8s项目
type K8sApp struct {
	Model

	Name         string `json:"name" gorm:"uniqueIndex:idx_project_app_name;type:varchar(100);comment:应用英文名称"`
	K8sProjectId uint   `json:"k8sProjectId" gorm:"uniqueIndex:idx_project_app_name;comment:属于哪个项目 一对多外键"`
	TreeNodeId   uint   `json:"treeNodeId" gorm:"comment:绑定到哪个节点 面向第四层级"`
	UserID       uint

	//Cluster string `json:"cluster" gorm:"uniqueIndex:name_cluster;type:varchar(100);comment:项目绑定的集群名称"`

	K8sInstances []*K8sInstance
	Namespace    string `json:"namespace"`

	// 容器字段 用户填写
	ContainerCore

	Key              string `json:"key" gorm:"-"` // 前端表格使用
	CreateUserName   string `json:"createUserName" gorm:"-"`
	NodePath         string `json:"nodePath" gorm:"-"`
	ProjectName      string `json:"projectName" gorm:"-"`
	InstanceNum      int    `json:"instanceNum" gorm:"-"`
	ClusterNamespace string `json:"clusterNamespace" gorm:"-"`
	ClusterName      string `json:"clusterName" gorm:"-"`
}

// ContainerCore app和instance都要使用
type ContainerCore struct {
	Envs     StringArray `json:"envs" gorm:"comment:环境变量组 k=v"`
	Labels   StringArray `json:"labels" gorm:"comment:标签组 k=v"`
	Commands string      `json:"commands" gorm:"comment:启动命令组"`

	CpuRequest    string `json:"cpuRequest"`
	CpuLimit      string `json:"cpuLimit"`
	MemoryRequest string `json:"memoryRequest"`
	MemoryLimit   string `json:"memoryLimit"`

	Args            string      `json:"args" gorm:"comment:启动命令参数组 空格分隔"`
	VolumeJson      string      `json:"volumeJson" gorm:"type:text;comment:卷和挂载配置"`
	VolumeJsonFront []OneVolume `json:"volumeJsonFront" gorm:"-"` // 前端使用

	PortJson      string               `json:"portJson" gorm:"type:text;comment:容器和svc端口配置"` // 端口直接用[]corev1.servicePort
	PortJsonFront []corev1.ServicePort `json:"PortJsonFront" gorm:"-"`                       // 前端使用
}
type OneVolume struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	SubPath   string `json:"subPath"`
	PvcName   string `json:"pvcName"`
}

type OneResource struct {
	Request string `json:"request"`
}

func (obj *K8sApp) Create() error {
	return Db.Create(obj).Error
}

func (obj *K8sApp) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *K8sApp) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *K8sApp) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func GetK8sAppById(id int) (*K8sApp, error) {
	var dbApp K8sApp

	err := Db.Where("id = ? ", id).Preload("K8sInstances").First(&dbApp).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("app不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbApp, nil
}

func (obj *K8sApp) FillFrontAllData() error {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}

	var vj []OneVolume
	if obj.VolumeJson != "" {
		err := json.Unmarshal([]byte(obj.VolumeJson), &vj)
		if err != nil {
			return err
		}
		obj.VolumeJsonFront = vj
	}
	var sp []corev1.ServicePort
	if obj.PortJson != "" {
		err := json.Unmarshal([]byte(obj.PortJson), &sp)
		if err != nil {
			return err
		}
		obj.PortJsonFront = sp
	}
	obj.ClusterNamespace = obj.Namespace
	if obj.K8sProjectId > 0 {
		proj, err := GetK8sProjectById(int(obj.K8sProjectId))
		if err == nil && proj != nil {
			obj.ProjectName = fmt.Sprintf("%s (%s)", proj.NameZh, proj.Name)
			if proj.Cluster != "" {
				obj.ClusterName = proj.Cluster
				obj.ClusterNamespace = fmt.Sprintf("%s / %s", proj.Cluster, obj.Namespace)
			}
		}
	}
	if obj.TreeNodeId > 0 {
		node, err := GetStreeNodeById(int(obj.TreeNodeId))
		if err == nil && node != nil {
			_ = node.GetFullNodePath()
			obj.NodePath = node.NodePath
		}
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)
	obj.InstanceNum = len(obj.K8sInstances)
	return nil
}

func GetK8sAppAll() (obj []*K8sApp, err error) {
	err = Db.Model(&K8sApp{}).Preload("K8sInstances").Find(&obj).Error
	return
}

func GetK8sAppByIdsWithLimitOffset(ids []int, limit, offset int) (obj []*K8sApp, err error) {
	if len(ids) == 0 {
		return nil, nil
	}
	err = Db.Where("id IN ?", ids).Limit(limit).Offset(offset).Preload("K8sInstances").Find(&obj).Error
	return
}

func DeleteK8sAppById(id int) error {
	return Db.Unscoped().Delete(&K8sApp{}, id).Error
}

// GetK8sAppListByNameAndCreator 分页查询，支持按名称和创建人模糊查询
func GetK8sAppListByNameAndCreator(name, creator string, limit, offset int) (obj []*K8sApp, err error) {
	query := Db.Model(&K8sApp{}).Preload("K8sInstances")

	// 1. 按名称模糊查询
	if name != "" {
		query = query.Where("k8s_apps.name LIKE ?", "%"+name+"%")
	}

	// 2. 按创建人模糊查询 (关联 User 表)
	if creator != "" {
		query = query.Joins("left join users on users.id = k8s_apps.user_id").
			Where("users.username LIKE ? OR users.real_name LIKE ?", "%"+creator+"%", "%"+creator+"%")
	}

	err = query.Limit(limit).Offset(offset).Find(&obj).Error
	return
}

// GetK8sAppCountByNameAndCreator 对应统计总数
func GetK8sAppCountByNameAndCreator(name, creator string) (int64, error) {
	var count int64
	query := Db.Model(&K8sApp{}).Joins("left join users on users.id = k8s_apps.user_id")

	if name != "" {
		query = query.Where("k8s_apps.name LIKE ?", "%"+name+"%")
	}
	if creator != "" {
		query = query.Where("users.username LIKE ? OR users.real_name LIKE ?", "%"+creator+"%", "%"+creator+"%")
	}

	err := query.Count(&count).Error
	return count, err
}

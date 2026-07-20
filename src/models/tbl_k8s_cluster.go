package models

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// K8sCluster 采集任务Job对象

type K8sCluster struct {
	Model
	Name string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:k8s集群名称"`

	UserID            uint
	Env               string `json:"env,omitempty" gorm:"comment:集群环境信息 prod|stage|test"`
	KubeConfigContent string `json:"kubeConfigContent" gorm:"comment:kubeconfig配置文件"`

	Key string `json:"key" gorm:"-"` // 前端表格使用

	CreateUserName string `json:"createUserName" gorm:"-"`

	LabelsFront      string            `json:"labelsFront" gorm:"-"`
	AnnotationsFront string            `json:"annotationsFront" gorm:"-"`
	LabelsM          map[string]string `json:"labelsM" gorm:"-"`
	AnnotationsM     map[string]string `json:"annotationsM" gorm:"-"`
}

func (obj *K8sCluster) Create() error {
	return Db.Create(obj).Error
}

func (obj *K8sCluster) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *K8sCluster) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *K8sCluster) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func GetK8sClusterById(id int) (*K8sCluster, error) {
	var dbMonitorRecordRule K8sCluster
	err := Db.Where("id = ? ", id).First(&dbMonitorRecordRule).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorScrapePool不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMonitorRecordRule, nil
}

func GetK8sClusterByPoolId(poolId uint) (ps []*K8sCluster, err error) {
	err = Db.Where("enable = 1 AND pool_id = ? ", poolId).Find(&ps).Error
	return
}

func GetK8sClusterAll() (ps []*K8sCluster, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *K8sCluster) GenMapFromKvs(kvs []string) map[string]string {
	labelsM := map[string]string{}
	for _, i := range kvs {
		kvs := strings.Split(i, "=")
		if len(kvs) != 2 {
			continue
		}
		k := kvs[0]
		v := kvs[1]
		labelsM[k] = v
	}
	return labelsM
}

func (obj *K8sCluster) FillDefaultData() {

}

func (obj *K8sCluster) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}

}

func GetK8sClusterByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*K8sCluster, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

// UpdateEnable 更新采集任务的开关状态
func (obj *K8sCluster) UpdateEnable() error {
	// 推荐使用 Select 显式指定更新 enable 字段，这样既安全又能避免潜在的零值过滤问题
	return Db.Model(obj).Select("Enable").Updates(obj).Error
}

// UpdateK8sClusterEnableBatch 批量更新k8s集群的开关状态
func UpdateK8sClusterEnableBatch(ids []int, enable int) error {
	err := Db.Model(&K8sCluster{}).Where("id IN ?", ids).Update("enable", enable).Error
	return err
}

// DeleteK8sClusterBatch 批量删除k8s集群
func DeleteK8sClusterBatch(ids []uint) error {
	return Db.Unscoped().Where("id IN ?", ids).Delete(&K8sCluster{}).Error
}

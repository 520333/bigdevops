package models

import (
	"bigdevops/src/common"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// K8sCluster 采集任务Job对象

type K8sCluster struct {
	Model
	Name   string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:k8s集群英文名称"`
	NameZh string `json:"nameZh,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:k8s集群中文名称"`

	UserID               uint
	Env                  string `json:"env,omitempty" gorm:"comment:集群环境信息 prod|stage|test"`
	Version              string `json:"version,omitempty" gorm:"comment:集群版本"`              // 不需要用户填入 解析kubeconfig发起serverVersion请求
	ApiServerAddr        string `json:"apiServerAddr,omitempty" gorm:"comment:apiServer地址"` // 不需要用户填入 解析kubeconfig
	KubeConfigContent    string `json:"kubeConfigContent" gorm:"comment:kubeconfig配置文件"`
	ActionTimeoutSeconds int    `json:"actionTimeoutSeconds" gorm:"comment:超时时间秒数"`

	Key string `json:"key" gorm:"-"` // 前端表格使用

	CreateUserName string `json:"createUserName" gorm:"-"`

	LastProbSuccess  bool              `json:"lastProbSuccess" gorm:"-"`
	LastProbErrMsg   string            `json:"LastProbErrMsg" gorm:"-"`
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
	var dbObj K8sCluster
	err := Db.Where("id = ?", id).First(&dbObj).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("K8sCluster不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbObj, nil
}

func GetK8sClusterByName(name string) (*K8sCluster, error) {
	var dbObj K8sCluster
	err := Db.Where("name = ?", name).First(&dbObj).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("K8sCluster不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbObj, nil
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

func (obj *K8sCluster) FillDefaultData() error {
	if obj.ActionTimeoutSeconds <= 0 {
		obj.ActionTimeoutSeconds = 3
	}

	if obj.KubeConfigContent == "" {
		return errors.New("KubeConfig内容不能为空")
	}

	// 1. 解析 kubeconfig 得到 rest.Config
	kConfig, kClientSet, _, err := common.GenK8sClientSetByKubeconfigContent(obj.KubeConfigContent, obj.ActionTimeoutSeconds)
	if err != nil {
		return fmt.Errorf("解析kubeconfig内容失败: %v", err)
	}
	obj.ApiServerAddr = kConfig.Host

	// 2. 🚀 关键修复：必须在 NewForConfig 之前设置 Timeout！
	// 否则 ClientSet 的底层 HTTP Client 拿不到超时配置，导致连不上的 IP 挂起 30 秒引起前端 HTTP 请求超时！
	kConfig.Timeout = time.Duration(obj.ActionTimeoutSeconds) * time.Second

	// 3. 创建 ClientSet
	//kClientSet, err := kubernetes.NewForConfig(kConfig)
	//if err != nil {
	//	return fmt.Errorf("创建Kubernetes客户端失败: %v", err)
	//}

	// 4. 请求 ServerVersion (已包含 ActionTimeoutSeconds 超时控制)
	v, err := kClientSet.ServerVersion()
	if err == nil && v != nil && v.GitVersion != "" {
		obj.Version = v.GitVersion
	}
	return nil
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

package models

import (
	"errors"
	"fmt"

	"github.com/prometheus/prometheus/model/relabel"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MonitorScrapeJob 采集任务Job对象

type MonitorScrapeJob struct {
	Model
	Name string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:采集任务名称"`

	UserID uint

	Enable                   int    `json:"enable" gorm:"comment:是否被开启 1正常 2禁用"`
	ServiceDiscoveryType     string `json:"serviceDiscoveryType" gorm:"comment:k8s or tree-http"`
	MetricsPath              string `json:"metricsPath"`
	Scheme                   string `json:"scheme"`
	ScrapeInterval           int    `json:"scrapeInterval" gorm:"comment:采集间隔"`
	ScrapeTimeout            int    `json:"scrapeTimeout" gorm:"comment:采集超时时间"`
	PoolId                   uint   `json:"poolId"`
	RelabelConfigsYamlString string `json:"relabelConfigsYamlString,omitempty" gorm:"type:text;comment:yaml字符串"`

	// 服务发现 http
	Port            int         `json:"port,omitempty" gorm:"comment:用虚拟机类型的时候 服务树服务发现接口类型 需要传port"`
	RefreshInterval int         `json:"refreshInterval,omitempty" gorm:"comment:sd发现刷新间隔"`
	TreeNodeIds     StringArray `json:"treeNodeIds,omitempty" gorm:"comment:如果使用了服务树接口 通过树id获取ip列表"`

	// 服务发现 k8s
	APIServer          string `json:"apiServer,omitempty" gorm:"comment: apiServer地址"`
	KubeConfigFilePath string `json:"kubeConfigFilePath,omitempty" gorm:"comment: kubeconfig文件路径"`
	TlsCaFilePath      string `json:"tlsCaFilePath,omitempty" gorm:"comment: kubelet client CA文件路径"`
	TlsCaContent       string `json:"tlsCaContent,omitempty" gorm:"comment: kubelet client CA文件内容"`

	BearerToken      string `json:"bearerToken,omitempty" gorm:"comment: SA鉴权token"`
	BearerTokenFile  string `json:"bearerTokenFile,omitempty" gorm:"comment: SA鉴权token文件"`
	KubernetesSdRole string `json:"kubernetesSdRole,omitempty"`

	//KubeletClientCert string
	//KubeletClientKey  string

	Key            string `json:"key" gorm:"-"` // 前端表格使用
	PoolName       string `json:"poolName" gorm:"-"`
	CreateUserName string `json:"createUserName" gorm:"-"`
}

func (obj *MonitorScrapeJob) Create() error {
	return Db.Create(obj).Error
}

func (obj *MonitorScrapeJob) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *MonitorScrapeJob) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *MonitorScrapeJob) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func (obj *MonitorScrapeJob) IdsConvert() {
	strIds := []string{}
	for _, id := range obj.TreeNodeIds {
		strIds = append(strIds, fmt.Sprintf("%s", id))
	}
}
func (obj *MonitorScrapeJob) ValidateRelabelConfigsYamlString() error {
	var relabelConfigsObj []*relabel.Config
	return yaml.Unmarshal([]byte(obj.RelabelConfigsYamlString), &relabelConfigsObj)
}

func GetMonitorScrapeJobById(id int) (*MonitorScrapeJob, error) {
	var dbMonitorScrapeJob MonitorScrapeJob
	err := Db.Where("id = ? ", id).First(&dbMonitorScrapeJob).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("MonitorScrapePool不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbMonitorScrapeJob, nil
}

func GetMonitorScrapeJobByPoolId(poolId uint) (ps []*MonitorScrapeJob, err error) {
	err = Db.Where("enable = 1 AND pool_id = ? ", poolId).Find(&ps).Error
	return
}

func GetMonitorScrapeJobAll() (ps []*MonitorScrapeJob, err error) {
	err = Db.Find(&ps).Error
	return
}

func (obj *MonitorScrapeJob) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	dbPool, _ := GetMonitorScrapePoolById(int(obj.PoolId))
	if dbPool != nil {
		obj.PoolName = dbPool.Name
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetMonitorScrapeJobByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorScrapeJob, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

// UpdateEnable 更新采集任务的开关状态
func (obj *MonitorScrapeJob) UpdateEnable() error {
	// 推荐使用 Select 显式指定更新 enable 字段，这样既安全又能避免潜在的零值过滤问题
	return Db.Model(obj).Select("Enable").Updates(obj).Error
}

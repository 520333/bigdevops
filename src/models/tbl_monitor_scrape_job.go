package models

import (
	"fmt"

	"gorm.io/gorm/clause"
)

// MonitorScrapeJob 采集任务Job对象

type MonitorScrapeJob struct {
	Model
	Name string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:采集任务名称"`

	UserID uint

	ServiceDiscoveryType string `json:"serviceDiscoveryType" gorm:"comment:k8s or tree-http"`
	MetricsPath          string `json:"metricsPath"`
	Scheme               string `json:"scheme"`
	ScrapeInterval       int    `json:"scrapeInterval" gorm:"comment:采集间隔"`
	ScrapeTimeout        int    `json:"scrapeTimeout" gorm:"comment:采集超时时间"`
	RefreshInterval      int    `json:"refreshInterval" gorm:"comment:sd发现刷新间隔"`

	// 服务发现 k8s http
	Port        int         `json:"port" gorm:"comment:用虚拟机类型的时候 服务树服务发现接口类型 需要传port"`
	TreeNodeIds StringArray `json:"treeNodeIds" gorm:"comment:如果使用了服务树接口 通过树id获取ip列表"`
	PoolId      uint        `json:"poolId"`

	APIServer          string `json:"apiServer" gorm:"comment: apiServer地址"`
	KubeConfigFilePath string `json:"kubeConfigFilePath" gorm:"comment: kubeconfig文件路径"`
	TlsCaFilePath      string `json:"tlsCaFilePath" gorm:"comment: kubelet client CA文件路径"`
	TlsCaContent       string `json:"tlsCaContent" gorm:"comment: kubelet client CA文件内容"`

	BearerToken              string `json:"bearerToken" gorm:"comment: SA鉴权token"`
	BearerTokenFile          string `json:"bearerTokenFile" gorm:"comment: SA鉴权token文件"`
	KubernetesSdRole         string `json:"kubernetesSdRole"`
	RelabelConfigsYamlString string `json:"relabelConfigsYamlString" gorm:"comment:yaml字符串"`

	//KubeletClientCert string
	//KubeletClientKey  string

	Key            string `json:"key" gorm:"-"` // 前端表格使用
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

func GetMonitorScrapeJobByPoolId(poolId uint) (ps []*MonitorScrapeJob, err error) {
	err = Db.Where("pool_id = ? ", poolId).Find(&ps).Error
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
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

func GetMonitorScrapeJobByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*MonitorScrapeJob, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

package models

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JenkinsJob Jenkins服务Job基线表
// 严密贴合重构需求，仅保留规范明确规定的关键字段，所有无效旧字段彻底清除。
type JenkinsJob struct {
	Model
	InstanceID     uint   `json:"instanceId" gorm:"uniqueIndex:uk_inst_job_proj;comment:关联实例ID"`
	DeployType     string `json:"deployType" gorm:"type:varchar(64);comment:部署方式(主机IP、容器集群)(构建时传参)"`
	DeployEnv      string `json:"deployEnv" gorm:"type:varchar(32);comment:部署环境(dev|test|stage|uat|pre|prod)"`
	Name           string `json:"name" gorm:"uniqueIndex:uk_inst_job_proj;type:varchar(128);comment:服务名(与Jenkins中的名称一致,创建Job时用)"`
	ProjectName    string `json:"projectName" gorm:"type:varchar(128);comment:项目名称(对应的是GIT仓库的group名称,创建Job时用)"`
	Folder         string `json:"folder" gorm:"-"` // 支持文件夹传参 不入库
	GitRepo        string `json:"gitRepo" gorm:"column:git_repo;type:varchar(255);comment:GIT仓库克隆地址(构建时传参)"`
	GitBranch      string `json:"gitBranch" gorm:"column:git_branch;type:varchar(64);default:'main';comment:GIT分支(构建时传参)"`
	URL            string `json:"url" gorm:"type:varchar(255);comment:该job的jenkins地址"`
	Lang           string `json:"lang" gorm:"type:varchar(32);default:'Java';comment:该job是什么技术栈"`
	Count          int64  `json:"count" gorm:"column:count;default:0;comment:最后构建号"`
	Status         string `json:"status" gorm:"column:status;type:varchar(32);default:'NOT_BUILT';comment:同步jenkins真实的job状态"`
	CreateUserName string `json:"createUserName" gorm:"column:create_user_name;type:varchar(64);comment:创建人"`
	EnableDelete   bool   `json:"enableDelete" gorm:"column:enable_delete;default:false;comment:创建job后锁定 开关控制 开启后删除按钮可以使用 关闭时删除按钮禁用"`
}

func (obj *JenkinsJob) AfterFind(tx *gorm.DB) (err error) {
	// 如果 Name 包含文件夹路径 (如 web/test1)，自动剥离出项目名与纯服务名
	if strings.Contains(obj.Name, "/") {
		parts := strings.Split(obj.Name, "/")
		if obj.ProjectName == "" {
			obj.ProjectName = strings.Join(parts[:len(parts)-1], "/")
		}
		obj.Name = parts[len(parts)-1]
	}
	if obj.ProjectName != "" {
		obj.Folder = obj.ProjectName
	}
	return nil
}

func (obj *JenkinsJob) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *JenkinsJob) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func (obj *JenkinsJob) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func GetJenkinsJobById(id uint) (*JenkinsJob, error) {
	var dbObj JenkinsJob
	err := Db.Where("id = ?", id).First(&dbObj).Error
	if err != nil {
		return nil, err
	}
	return &dbObj, nil
}

// JenkinsJobQueryParam 搜索过滤传参定义
// 支持：1.服务名、2.项目名称、3.构建状态、4.git地址/仓库地址、5.部署环境、6.语言 等的模糊与全维度查询
type JenkinsJobQueryParam struct {
	InstanceID  uint   `json:"instanceId" form:"instanceId"`
	Name        string `json:"name" form:"name"`
	ProjectName string `json:"projectName" form:"projectName"`
	Status      string `json:"status" form:"status"`
	GitRepo     string `json:"gitRepo" form:"gitRepo"`
	DeployEnv   string `json:"deployEnv" form:"deployEnv"`
	Lang        string `json:"lang" form:"lang"`
	Keyword     string `json:"keyword" form:"keyword"`
}

func GetJenkinsJobListByParam(param *JenkinsJobQueryParam) ([]*JenkinsJob, error) {
	var objs []*JenkinsJob
	query := Db.Where("instance_id = ?", param.InstanceID)

	if param.Name != "" {
		query = query.Where("name LIKE ?", "%"+param.Name+"%")
	}
	if param.ProjectName != "" {
		query = query.Where("project_name LIKE ?", "%"+param.ProjectName+"%")
	}
	if param.Status != "" {
		query = query.Where("status = ? OR status LIKE ?", param.Status, "%"+param.Status+"%")
	}
	if param.GitRepo != "" {
		query = query.Where("git_repo LIKE ?", "%"+param.GitRepo+"%")
	}
	if param.DeployEnv != "" {
		query = query.Where("deploy_env = ? OR deploy_env LIKE ?", param.DeployEnv, "%"+param.DeployEnv+"%")
	}
	if param.Lang != "" {
		query = query.Where("lang = ? OR lang LIKE ?", param.Lang, "%"+param.Lang+"%")
	}
	if param.Keyword != "" {
		kw := "%" + param.Keyword + "%"
		query = query.Where(
			"name LIKE ? OR project_name LIKE ? OR git_repo LIKE ? OR status LIKE ? OR deploy_env LIKE ? OR lang LIKE ? OR create_user_name LIKE ?",
			kw, kw, kw, kw, kw, kw, kw,
		)
	}

	err := query.Order("id desc").Find(&objs).Error
	if err != nil {
		return nil, err
	}
	return objs, nil
}

func (JenkinsJob) TableName() string {
	return "jenkins_job"
}

// SaveOrUpdateJenkinsJob 全量基线新增或覆写保存方法
func SaveOrUpdateJenkinsJob(job *JenkinsJob) error {
	var existing JenkinsJob
	var err error

	if job.ID > 0 {
		err = Db.Where("id = ?", job.ID).First(&existing).Error
	}
	if err != nil || job.ID == 0 {
		err = Db.Where("instance_id = ? AND name = ?", job.InstanceID, job.Name).First(&existing).Error
	}

	if err == nil && existing.ID > 0 {
		job.ID = existing.ID
		// 保护在途构建状态：如果原有记录在构建中且未接收到新终止消息
		if existing.Status == "BUILDING" && job.Status != "SUCCESS" && job.Status != "FAILURE" && job.Status != "ABORTED" {
			job.Status = "BUILDING"
		}
		if job.Count == 0 && existing.Count > 0 {
			job.Count = existing.Count
		}
		// 默认保持原有防删除开关状态，未显式改变不重置
		if !job.EnableDelete && existing.EnableDelete {
			job.EnableDelete = existing.EnableDelete
		}
		return Db.Model(&existing).Updates(job).Error
	}
	return Db.Create(job).Error
}

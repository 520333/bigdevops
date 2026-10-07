package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JenkinsJob Jenkins服务Job基线表
// 严密贴合重构需求，仅保留规范明确规定的关键字段，所有无效旧字段彻底清除。
type JenkinsJob struct {
	Model
	InstanceID     uint       `json:"instanceId" gorm:"uniqueIndex:uk_inst_job_proj;comment:关联实例ID"`
	DeployType     string     `json:"deployType" gorm:"type:varchar(64);comment:部署方式(主机IP、容器集群)(构建时传参)"`
	DeployEnv      string     `json:"deployEnv" gorm:"type:varchar(32);comment:部署环境(dev|test|stage|uat|pre|prod)"`
	Name           string     `json:"name" gorm:"uniqueIndex:uk_inst_job_proj;type:varchar(128);comment:服务名(与Jenkins中的名称一致,创建Job时用)"`
	ProjectName    string     `json:"projectName" gorm:"uniqueIndex:uk_inst_job_proj;type:varchar(128);comment:项目名称(对应的是GIT仓库的group名称,创建Job时用)"`
	Folder         string     `json:"folder" gorm:"-"` // 支持文件夹传参 不入库
	GitRepo        string     `json:"gitRepo" gorm:"column:git_repo;type:varchar(255);comment:GIT仓库克隆地址(构建时传参)"`
	GitBranch      string     `json:"gitBranch" gorm:"column:git_branch;type:varchar(64);default:'main';comment:最后GIT分支(构建时传参)"`
	URL            string     `json:"url" gorm:"type:varchar(255);comment:该job的jenkins地址"`
	Lang           string     `json:"lang" gorm:"type:varchar(32);default:'Java';comment:该job是什么技术栈"`
	Count          int64      `json:"count" gorm:"column:count;default:0;comment:最后构建号"`
	Status         string     `json:"status" gorm:"column:status;type:varchar(32);default:'NOT_BUILT';comment:同步jenkins真实的job状态"`
	CreateUserName string     `json:"createUserName" gorm:"column:create_user_name;type:varchar(64);comment:创建人"`
	EnableDelete   int        `json:"enableDelete" gorm:"column:enable_delete;type:tinyint;default:2;comment:删除控制: 1-开启删除 2-禁止删除 默认为2"`
	LastBuildTime  *time.Time `json:"lastBuildTime" gorm:"column:last_build_time;type:datetime;comment:最后构建时间"`
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

	// 1. 优先按唯一索引检查库中是否已有此唯一记录 (instance_id, name, project_name)
	q := Db.Where("instance_id = ? AND name = ?", job.InstanceID, job.Name)
	if job.ProjectName != "" {
		q = q.Where("project_name = ?", job.ProjectName)
	} else {
		q = q.Where("project_name = '' OR project_name IS NULL")
	}
	err = q.First(&existing).Error

	// 2. 如果唯一索引未命中，但指定了有效 ID，则通过 ID 查找原记录 (处理用户重命名场景)
	if err != nil && job.ID > 0 {
		err = Db.Where("id = ?", job.ID).First(&existing).Error
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

		enableDelete := job.EnableDelete
		if enableDelete != 1 && enableDelete != 2 {
			if existing.EnableDelete == 1 || existing.EnableDelete == 2 {
				enableDelete = existing.EnableDelete
			} else {
				enableDelete = 2
			}
		}

		// 使用 map 更新确保所有字段均能准确持久化
		updates := map[string]interface{}{
			"instance_id":   job.InstanceID,
			"deploy_type":   job.DeployType,
			"deploy_env":    job.DeployEnv,
			"name":          job.Name,
			"project_name":  job.ProjectName,
			"git_repo":      job.GitRepo,
			"git_branch":    job.GitBranch,
			"lang":          job.Lang,
			"enable_delete": enableDelete,
		}
		if job.CreateUserName != "" {
			updates["create_user_name"] = job.CreateUserName
		}
		if job.URL != "" {
			updates["url"] = job.URL
		}
		if job.Status != "" {
			updates["status"] = job.Status
		}
		if job.Count > 0 {
			updates["count"] = job.Count
		}
		if job.LastBuildTime != nil {
			updates["last_build_time"] = job.LastBuildTime
		}
		return Db.Model(&existing).Updates(updates).Error
	}
	if job.EnableDelete != 1 && job.EnableDelete != 2 {
		job.EnableDelete = 2
	}
	return Db.Create(job).Error
}

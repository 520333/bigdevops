package models

import (
	"bigdevops/src/common"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JobResult 工作审批流
type JobResult struct {
	Model
	// 需要创建一个JobId和HostIP的联合唯一索引：一个任务给一个机器下发的任务只能有1条
	JobId    uint   `json:"jobId" gorm:"uniqueIndex:jid_hostip;comment:ip"`
	Status   string `json:"status" gorm:"comment:当前状态"`
	Stdout   string `json:"stdout" gorm:"comment:标准输出"`
	Stderr   string `json:"stderr" gorm:"comment:标准错误"`
	HostIP   string `json:"hostIP"  gorm:"type:varchar(100);uniqueIndex:jid_hostip;comment:主机IP"`
	HostName string `json:"hostName"  gorm:"comment:主机名"`

	Key string `json:"key" gorm:"-"` // 前端表格使用
}

func (obj *JobResult) Create() error {
	return Db.Create(obj).Error
}

func (obj *JobResult) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *JobResult) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *JobResult) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

// JudgeByOnErrorStrategy 根据一个机器的状态结合错误策略去更新这个任务的状态
func (obj *JobResult) JudgeByOnErrorStrategy() error {
	dbJobTask, err := GetJobTaskById(int(obj.JobId))
	if err != nil {
		return err
	}

	switch obj.Status {
	case common.AGENT_TASK_STATUS_SUCCESS, common.AGENT_TASK_STATUS_RUNNING:
		return nil
	}

	switch dbJobTask.OnErrorStrategy {
	case common.JOB_ONERROR_STRATEGY_IGNORE:
		return nil
	case common.JOB_ONERROR_STRATEGY_PAUSE:
		if dbJobTask.Action == common.AGENT_TASK_ACTION_KILL {
			return nil
		}
		dbJobTask.Status = common.JOB_STATUS_PAUSED
		return dbJobTask.UpdateOne()
	case common.JOB_ONERROR_STRATEGY_STOP:
		dbJobTask.Status = common.JOB_STATUS_FINISHED
		return dbJobTask.UpdateOne()
	}
	return nil
}

func (obj *JobResult) GetOrCreate() error {
	var dbObj *JobResult
	Db.Where("job_id = ? and host_ip = ? ", obj.JobId, obj.HostIP).Find(&dbObj)
	if dbObj.ID != 0 {
		return nil
	}
	return obj.Create()
}

func (obj *JobResult) UpdateFlowNodes(nodes []WorkOrderFlowNode) error {
	return Db.Model(obj).Association("FlowNodes").Replace(nodes)
}

func GetJobResultTotal() (obj []*JobResult, err error) {
	err = Db.Preload("FlowNodes").Find(&obj).Error
	return
}

func GetJobResultAllWithLimitOffset(limit, offset int) (obj []*JobResult, err error) {
	err = Db.Preload("FlowNodes").Limit(limit).Offset(offset).Find(&obj).Error
	return
}

func GetJobResultByJobId(jobId int) (obj []*JobResult, err error) {
	err = Db.Where("job_id = ?", jobId).Find(&obj).Error
	return
}

func GetJobTaskResultByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*JobTask, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return
}

func GetJobResultByJobIdAndHostIp(jobId int, hostIp string) (obj *JobResult, err error) {
	err = Db.Where("job_id = ? and host_ip = ?", jobId, hostIp).First(&obj).Error
	return
}

func GetJobResultById(id int) (*JobResult, error) {
	var dbJobResult JobResult

	// 💡 修复：将 load_balancer_id = ? 改为 id = ?
	err := Db.Where("id = ? ", id).Preload("FlowNodes").First(&dbJobResult).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("JobResult不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbJobResult, nil
}

func GetJobExecResultByJobId(jobId int) (obj *JobResult, err error) {
	err = Db.Where("job_id = ? ", jobId).First(&obj).Error
	return
}

func GetJobResultsByFilters(jobId int, status, ip string, limit, offset int) (objs []*JobResult, total int64, err error) {
	// 强制绑定当前 jobId
	query := Db.Model(&JobResult{}).Where("job_id = ?", jobId)

	// 状态精确匹配
	if status != "" {
		query = query.Where("status = ?", status)
	}
	// IP 或主机名模糊搜索
	if ip != "" {
		query = query.Where("host_ip LIKE ? OR host_name LIKE ?", "%"+ip+"%", "%"+ip+"%")
	}

	// 先查总数
	err = query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 再查分页数据
	err = query.Limit(limit).Offset(offset).Find(&objs).Error
	return
}
func GetJobResultByInstanceId(instanceId string) (*JobResult, error) {
	var dbJobResult JobResult
	// FIXED: Changed from load_balancer_id to db_instance_id
	err := Db.Where("db_instance_id = ? ", instanceId).Preload("BindNodes").First(&dbJobResult).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("JobResult不存在") // Fixed error message
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbJobResult, nil
}

// GetJobResultCountByName 查询总数
func GetJobResultCountByName(name string) (int64, error) {
	var count int64
	query := Db.Model(&JobResult{})
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	err := query.Count(&count).Error
	return count, err
}

// GetJobResultListByName 分页查询
func GetJobResultListByName(name string, limit, offset int) (obj []*JobResult, err error) {
	query := Db.Preload("FlowNodes")
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	err = query.Limit(limit).Offset(offset).Find(&obj).Error
	return
}

// GetJobResultListByNameAndCreator 分页查询，支持按名称和创建人模糊查询
func GetJobResultListByNameAndCreator(name, creator string, limit, offset int) (obj []*JobResult, err error) {
	query := Db.Model(&JobResult{}).Preload("FlowNodes")

	// 1. 按名称模糊查询
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	// 2. 按创建人模糊查询 (关联 User 表)
	if creator != "" {
		// 使用 LEFT JOIN 关联 users 表进行模糊搜索
		query = query.Joins("left join users on users.id = work_order_JobResultes.user_id").
			Where("users.username LIKE ? OR users.real_name LIKE ?", "%"+creator+"%", "%"+creator+"%")
	}

	err = query.Limit(limit).Offset(offset).Find(&obj).Error
	return
}

// GetJobResultCountByNameAndCreator 对应统计总数
func GetJobResultCountByNameAndCreator(name, creator string) (int64, error) {
	var count int64
	query := Db.Model(&JobResult{}).Joins("left join users on users.id = work_order_JobResultes.user_id")

	if name != "" {
		query = query.Where("work_order_JobResultes.name LIKE ?", "%"+name+"%")
	}
	if creator != "" {
		query = query.Where("users.username LIKE ? OR users.real_name LIKE ?", "%"+creator+"%", "%"+creator+"%")
	}

	err := query.Count(&count).Error
	return count, err
}

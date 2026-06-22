package models

import (
	"bigdevops/src/common"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type JobTask struct {
	Model
	Title              string `json:"title" gorm:"comment:中文名称"`
	Account            string `json:"account" gorm:"comment:执行任务的账号"`
	Args               string `json:"args" gorm:"type:varchar(255);comment:执行任务的参数"`
	ScriptContent      string `json:"scriptContent" gorm:"comment:脚本内容 来源2种：01当场写的 02从script表中获取"`
	ExecTimeoutSeconds int    `json:"execTimeoutSeconds"  gorm:"comment:任务执行超时时间"`
	BatchSize          int    `json:"batchSize" gorm:"comment:机器并发执行数量"`
	OnErrorStrategy    string `json:"OnErrorStrategy" gorm:"comment:遇到某些机器出错后的策略，忽略执行完 | 遇到错误停止 | 遇到错误暂停"`
	TreeNodeId         uint   `json:"treeNodeId" gorm:"comment:关联的服务树节点"`
	HostsRaw           string `json:"hostsRaw" gorm:"type:text;comment:机器的IP列表"`
	ScheduledHosts     string `json:"scheduledHosts" gorm:"type:text;comment:已经下发的机器IP列表 并发或者暂停使用"`
	Status             string `json:"status" gorm:"comment:当前执行状态"`
	Action             string `json:"action" gorm:"comment:当前执行动作"`
	HostsIdsRaw        string `json:"hostsIdsRaw" gorm:"-"`
	UserID             uint
	ActualFlowData     string `json:"actualFlowData" gorm:"type:text;comment:任务执行的状态记录 结果(JSON串)"`

	Lang string `json:"lang" gorm:"comment:脚本语言(shell/python/ansible)"`

	TotalNum int `json:"totalNum" gorm:"-"`

	CreateUserName string `json:"createUserName" gorm:"-"`
	Key            uint   `json:"value" gorm:"-"`
	Value          uint   `json:"key" gorm:"-"`
}

func GetJobTaskById(id int) (*JobTask, error) {
	var dbJobTask JobTask
	err := Db.Where("id = ? ", id).First(&dbJobTask).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("任务 不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbJobTask, nil
}

func (obj *JobTask) UpdateOne() error {
	return Db.Updates(obj).Error
}

func (obj *JobTask) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *JobTask) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	obj.Key = obj.ID
	obj.Value = obj.ID
}

func GetJobTaskAll() (JobTasks []*JobTask, err error) {
	err = Db.Find(&JobTasks).Error
	return
}

func GetJobTaskUnDone(statusArr []string) (JobTasks []*JobTask, err error) {
	err = Db.Where("status IN ?", statusArr).Find(&JobTasks).Error
	return
}

func (obj *JobTask) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func GetJobTaskByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*JobTask, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return
}

// CheckAndComplete 检查任务是否全部节点都执行完毕，并更新主状态
func (obj *JobTask) CheckAndComplete() error {
	// 1. 解析目标机器总数
	var allIps []string
	_ = json.Unmarshal([]byte(obj.HostsRaw), &allIps)
	totalTarget := len(allIps)

	if totalTarget == 0 {
		return nil
	}

	// 2. 获取该任务在结果表中的所有执行明细
	results, err := GetJobResultByJobId(int(obj.ID))
	if err != nil {
		return err
	}

	// 3. 如果结果条数还不等于目标总数，说明还有机器尚未被调度下发，主任务肯定没结束
	if len(results) < totalTarget {
		return nil
	}

	// 4. 遍历所有结果，判断是否全部结束
	hasFailed := false
	for _, r := range results {
		// 只要还有一台机器正在运行，主任务就继续保持 running
		if r.Status == common.AGENT_TASK_STATUS_RUNNING || r.Status == "pending" {
			return nil
		}
		// 记录是否有失败或被杀死的节点
		if r.Status == common.AGENT_TASK_STATUS_FAILED || r.Status == common.AGENT_TASK_STATUS_KILLED {
			hasFailed = true
		}
	}

	// 5. 走到这里，说明所有机器的任务都已经跑完，进行状态收敛
	if hasFailed {
		obj.Status = common.AGENT_TASK_STATUS_FAILED
	} else {
		obj.Status = common.AGENT_TASK_STATUS_SUCCESS
	}

	// 更新主任务状态
	return obj.UpdateOne()
}

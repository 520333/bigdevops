package models

import (
	"fmt"
)

type K8sYamlTaskLog struct {
	Model
	TaskId         uint   `json:"taskId" gorm:"comment:YAML任务ID"`
	TaskName       string `json:"taskName" gorm:"type:varchar(100);comment:任务名称"`
	ClusterName    string `json:"clusterName" gorm:"type:varchar(100);comment:目标集群"`
	TemplateId     uint   `json:"templateId" gorm:"comment:模板ID"`
	YamlContent    string `json:"yamlContent" gorm:"type:text;comment:应用时的实际YAML内容"`
	Status         string `json:"status" gorm:"type:varchar(30);comment:执行状态 SUCCESS/FAILED"`
	ErrMsg         string `json:"errMsg" gorm:"type:text;comment:错误堆栈信息"`
	UserID         uint   `json:"userId"`
	CreateUserName string `json:"createUserName" gorm:"-"`
}

func (obj *K8sYamlTaskLog) CreateOne() error {
	return Db.Create(obj).Error
}

func GetK8sYamlTaskLogListByTaskId(taskId int, page, pageSize int) (objs []*K8sYamlTaskLog, total int64, err error) {
	db := Db.Model(&K8sYamlTaskLog{})
	if taskId > 0 {
		db = db.Where("task_id = ?", taskId)
	}
	err = db.Count(&total).Error
	if err != nil {
		return
	}
	offset := (page - 1) * pageSize
	err = db.Order("id desc").Limit(pageSize).Offset(offset).Find(&objs).Error
	return
}

func (obj *K8sYamlTaskLog) FillFrontAllData() {
	if obj.UserID > 0 {
		dbUser, _ := GetUserById(int(obj.UserID))
		if dbUser != nil {
			obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
		}
	}
}

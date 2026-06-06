package models

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Process 工作审批流
type Process struct {
	Model
	Name string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:流程名称"`
	//Creator       *User      `json:"creator,omitempty"`
	UserID         uint
	FlowNodes      []FlowNode `json:"flowNodes,omitempty"`
	CurrentNodeId  uint       `json:"currentNodeId"`
	FlowNodeStr    string     `json:"flowNodeStr,omitempty" gorm:"-"`
	Status         string     `json:"status" gorm:"comment:执行中 | 已完成 | 已拒绝 | 已驳回"`
	Key            string     `json:"key" gorm:"-"` // 前端表格使用
	CreateUserName string     `json:"createUserName" gorm:"-"`
}

// FlowNode 节点
type FlowNode struct {
	Model
	ProcessID         uint
	Type              string `json:"type" gorm:"comment: 起始节点 审批节点 执行节点 结束节点"`
	DefineUserOrGroup string `json:"defineUserOrGroup" gorm:"comment:期望的人或组 userName or RoleName"`
	ActualUser        string `json:"actualUser" gorm:"comment:真实执行或认领的人"`
	StartTime         string `json:"startTime"`
	EndTime           string `json:"endTime"`
	IsPass            bool   `json:"isPass"`
}

func (obj *Process) Create() error {
	return Db.Create(obj).Error
}

func (obj *Process) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *Process) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *Process) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func (obj *Process) UpdateFlowNodes(nodes []FlowNode) error {
	return Db.Model(obj).Association("FlowNodes").Replace(nodes)
}
func (obj *Process) UpdateWithNodes() error {
	return Db.Transaction(func(tx *gorm.DB) error {
		// 1. 找出前端保留下来的旧节点 ID
		var keepNodeIDs []uint
		for _, node := range obj.FlowNodes {
			if node.ID != 0 {
				keepNodeIDs = append(keepNodeIDs, node.ID)
			}
		}

		// 2. 物理删除已经被前端移除的旧节点
		if len(keepNodeIDs) > 0 {
			if err := tx.Where("process_id = ? AND id NOT IN ?", obj.ID, keepNodeIDs).Delete(&FlowNode{}).Error; err != nil {
				return err
			}
		} else {
			// 如果前端把所有节点都删了
			if err := tx.Where("process_id = ?", obj.ID).Delete(&FlowNode{}).Error; err != nil {
				return err
			}
		}

		// 3. 开启级联保存 (非常强大)
		// 由于 obj 是从数据库查出来后被我们修改过的，所以使用 Save 非常安全
		// FullSaveAssociations 会自动把 obj.FlowNodes 里：
		// - 带有 ID 的执行 UPDATE 语句更新字段
		// - 没有 ID 的执行 INSERT 语句插入新数据
		if err := tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(obj).Error; err != nil {
			return err
		}

		return nil
	})
}
func GetProcessAll() (ps []*Process, err error) {
	err = Db.Preload("FlowNodes").Find(&ps).Error
	return
}

func GetProcessByIdsWithLimitOffset(ids []int, limit, offset int) (objs []*Process, err error) {
	err = Db.Where("id in ?", ids).Limit(limit).Offset(offset).Find(&objs).Error
	return

}

func GetProcessById(id int) (*Process, error) {
	var dbProcess Process

	// 💡 修复：将 load_balancer_id = ? 改为 id = ?
	err := Db.Where("id = ? ", id).Preload("FlowNodes").First(&dbProcess).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("process不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbProcess, nil
}

func GetProcessByInstanceId(instanceId string) (*Process, error) {
	var dbProcess Process
	// FIXED: Changed from load_balancer_id to db_instance_id
	err := Db.Where("db_instance_id = ? ", instanceId).Preload("BindNodes").First(&dbProcess).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("Process不存在") // Fixed error message
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbProcess, nil
}

func (obj *Process) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)
	//for _, node := range obj.FlowNodes {
	//	obj.FlowNodeStr = fmt.Sprintf("%s->%s", obj.FlowNodeStr, node.DefineUserOrGroup)
	//}
	var nodeNames []string
	for _, node := range obj.FlowNodes {
		nodeNames = append(nodeNames, node.DefineUserOrGroup)
	}
	obj.FlowNodeStr = strings.Join(nodeNames, " -> ")
}

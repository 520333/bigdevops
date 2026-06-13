package models

import (
	"bigdevops/src/common"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WorkOrderProcess 工作审批流
type WorkOrderProcess struct {
	Model
	Name string `json:"name,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:流程名称"`
	//Creator       *User      `json:"creator,omitempty"`
	UserID uint
	//FlowNodes []WorkOrderFlowNode `json:"flowNodes,omitempty"`
	FlowNodes      []WorkOrderFlowNode `json:"flowNodes,omitempty" gorm:"foreignKey:ProcessID"`
	FlowNodeStr    string              `json:"flowNodeStr,omitempty" gorm:"-"`
	Status         string              `json:"status" gorm:"comment:执行中 | 已完成 | 已拒绝 | 已驳回"`
	Key            string              `json:"key" gorm:"-"` // 前端表格使用
	CreateUserName string              `json:"createUserName" gorm:"-"`
}

// WorkOrderFlowNode 节点
type WorkOrderFlowNode struct {
	Model
	ProcessID         uint
	Type              string `json:"type" gorm:"comment: 起始节点 审批节点 执行节点 结束节点"`
	DefineUserOrGroup string `json:"defineUserOrGroup" gorm:"comment:期望的人或组 userName or RoleName"`

	ActualUser        string `json:"actualUser" gorm:"-"`
	OutPut            string `json:"outPut" gorm:"-"` // 审批结果 执行的输出
	EndTime           string `json:"endTime" gorm:"-"`
	IsPassOrIsSuccess bool   `json:"isPassOrIsSuccess" gorm:"-"` // 审批结果或执行结果
}

func (obj *WorkOrderProcess) Create() error {
	return Db.Create(obj).Error
}

func (obj *WorkOrderProcess) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *WorkOrderProcess) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *WorkOrderProcess) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func (obj *WorkOrderProcess) UpdateFlowNodes(nodes []WorkOrderFlowNode) error {
	return Db.Model(obj).Association("FlowNodes").Replace(nodes)
}
func (obj *WorkOrderProcess) UpdateWithNodes() error {
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
			if err := tx.Where("process_id = ? AND id NOT IN ?", obj.ID, keepNodeIDs).Delete(&WorkOrderFlowNode{}).Error; err != nil {
				return err
			}
		} else {
			// 如果前端把所有节点都删了
			if err := tx.Where("process_id = ?", obj.ID).Delete(&WorkOrderFlowNode{}).Error; err != nil {
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

func GetProcessTotal() (obj []*WorkOrderProcess, err error) {
	err = Db.Preload("FlowNodes").Find(&obj).Error
	return
}

func GetProcessAllWithLimitOffset(limit, offset int) (obj []*WorkOrderProcess, err error) {
	err = Db.Preload("FlowNodes").Limit(limit).Offset(offset).Find(&obj).Error
	return
}

func GetProcessById(id int) (*WorkOrderProcess, error) {
	var dbProcess WorkOrderProcess

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

func GetProcessByInstanceId(instanceId string) (*WorkOrderProcess, error) {
	var dbProcess WorkOrderProcess
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

func (obj *WorkOrderProcess) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)

	var nodeNames []string
	for _, node := range obj.FlowNodes {
		oneNode := fmt.Sprintf("[%s:%s]", common.FLOW_TYPE_MAP[node.Type], node.DefineUserOrGroup)
		nodeNames = append(nodeNames, oneNode)
	}
	obj.FlowNodeStr = strings.Join(nodeNames, " -> ")
}

// GetProcessByName 查询总数
func GetProcessCountByName(name string) (int64, error) {
	var count int64
	query := Db.Model(&WorkOrderProcess{})
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	err := query.Count(&count).Error
	return count, err
}

// GetProcessListByName 分页查询
func GetProcessListByName(name string, limit, offset int) (obj []*WorkOrderProcess, err error) {
	query := Db.Preload("FlowNodes")
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	err = query.Limit(limit).Offset(offset).Find(&obj).Error
	return
}

// GetProcessListByNameAndCreator 分页查询，支持按名称和创建人模糊查询
func GetProcessListByNameAndCreator(name, creator string, limit, offset int) (obj []*WorkOrderProcess, err error) {
	query := Db.Model(&WorkOrderProcess{}).Preload("FlowNodes")

	// 1. 按名称模糊查询
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	// 2. 按创建人模糊查询 (关联 User 表)
	if creator != "" {
		// 使用 LEFT JOIN 关联 users 表进行模糊搜索
		query = query.Joins("left join users on users.id = work_order_processes.user_id").
			Where("users.username LIKE ? OR users.real_name LIKE ?", "%"+creator+"%", "%"+creator+"%")
	}

	err = query.Limit(limit).Offset(offset).Find(&obj).Error
	return
}

// GetProcessCountByNameAndCreator 对应统计总数
func GetProcessCountByNameAndCreator(name, creator string) (int64, error) {
	var count int64
	query := Db.Model(&WorkOrderProcess{}).Joins("left join users on users.id = work_order_processes.user_id")

	if name != "" {
		query = query.Where("work_order_processes.name LIKE ?", "%"+name+"%")
	}
	if creator != "" {
		query = query.Where("users.username LIKE ? OR users.real_name LIKE ?", "%"+creator+"%", "%"+creator+"%")
	}

	err := query.Count(&count).Error
	return count, err
}

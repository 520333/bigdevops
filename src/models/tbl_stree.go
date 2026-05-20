package models

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StreeNode struct {
	Model
	Title  string `json:"title" gorm:"type:varchar(50);uniqueIndex;comment:名称"`
	Pid    uint   `json:"pId" gorm:"comment:父级ID 树用的"`
	Level  int    `json:"level" gorm:"comment:层级"`
	IsLeaf bool   `json:"isLeaf" gorm:";comment:类型 0=否 1=是"`
	Desc   string `json:"desc" gorm:"comment:描述"`

	OpsAdmins                []*User           `json:"ops_admins" gorm:"many2many:ops_admins;comment:运维负责人列表"`
	BindEcss                 []*ResourceEcs    `json:"bind_ecss,omitempty" gorm:"many2many:bind_ecss;"`
	BindElbs                 []*ResourceElb    `json:"bind_elbs,omitempty" gorm:"many2many:bind_elbs;comment:绑定的服务树节点"`
	EcsNum                   int               `json:"ecsNum" gorm:"-"`
	NodeNum                  int               `json:"nodeNum" gorm:"-"`     // 子节点数量
	LeafNodeNum              int               `json:"leafNodeNum" gorm:"-"` // 叶子节点数量
	EcsCpuTotal              int               `json:"ecsCpuTotal" gorm:"-"`
	EcsMemoryTotal           int               `json:"ecsMemoryTotal" gorm:"-"`
	EcsDiskTotal             int               `json:"ecsDiskTotal" gorm:"-"`
	GroupByVendor            []*EchartsOneItem `json:"groupByVendor,omitempty" gorm:"-"`
	GroupByZoneId            []*EchartsOneItem `json:"groupByZoneId,omitempty" gorm:"-"`
	GroupByOSNameOrderKeys   []string          `json:"groupByOSNameOrderKeys,omitempty" gorm:"-"`
	GroupByOSNameOrderValues []int             `json:"groupByOSNameOrderValues,omitempty" gorm:"-"`
	OpsAdminUsers            []string          `json:"ops_admin_users" gorm:"-"`

	Children []*StreeNode `json:"children" gorm:"-"`
	Key      uint         `json:"key" gorm:"-"`
	NodePath string       `json:"nodePath" gorm:"-"`
}

func (obj *StreeNode) Create() error {
	return Db.Create(obj).Error
}

func (obj *StreeNode) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *StreeNode) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *StreeNode) UpdateOne() error {
	return Db.Updates(obj).Error
}

func (obj *StreeNode) FillFrontAllData() {
	obj.Key = obj.ID
	obj.GetFullNodePath()
	obj.FillFrontResource()
	//obj.SetEcsNum()
	obj.BindEcsData()
}

func (obj *StreeNode) SetEcsNum() {
	num, _ := obj.GetBindEcsNum()
	obj.EcsNum = num
}

func (obj *StreeNode) FillFrontResource() {
	for _, ecs := range obj.BindEcss {
		ecs := ecs
		ecs.Key = fmt.Sprintf("%d", ecs.ID)
	}
}

func (obj *StreeNode) GetFullNodePath() error {
	fatherTitles := []string{obj.Title}
	node := obj
	for node.Pid > 0 {
		father, err := GetStreeNodeById(int(node.Pid))
		if err != nil {
			return err
		}
		fatherTitles = append(fatherTitles, father.Title)
		node = father
	}
	nodePath := ""
	num := len(fatherTitles)
	for i := num - 1; i >= 0; i-- {
		title := fatherTitles[i]
		if nodePath == "" {
			nodePath = title
			continue
		}
		nodePath = fmt.Sprintf("%s.%s", nodePath, title)
	}
	//if nodePath != "" {
	//	nodePath = fmt.Sprintf("%s.%s", nodePath, obj.Title)
	//
	//} else {
	//	nodePath = obj.Title
	//}

	obj.NodePath = nodePath
	return nil
}

// GetAllLeafNodes 获取所有子孙节点（为了不改动其他地方的名字，方法名保持不变，但逻辑改为获取所有节点）
func GetAllLeafNodes(pid int) (objs []*StreeNode, err error) {
	children, err := GetStreeNodesByPId(pid)
	if err != nil {
		return nil, err
	}

	for _, child := range children {
		// 🌟 核心修复：无论它是目录还是叶子，先把自己装入结果集！
		objs = append(objs, child)

		// 如果不是叶子，说明下面可能还有子节点，继续往下掏
		if !child.IsLeaf {
			leaves, err := GetAllLeafNodes(int(child.ID))
			if err != nil {
				return nil, err
			}
			objs = append(objs, leaves...)
		}
	}

	return objs, nil
}

func (obj *StreeNode) GetBindEcsNum() (allNum int, err error) {
	allNodes := []*StreeNode{obj}
	childrens, err := GetAllLeafNodes(int(obj.ID))
	if err != nil {
		return
	}

	allResourceIdsMap := map[uint]struct{}{}
	if childrens != nil {
		allNodes = append(allNodes, childrens...)
	}
	for _, node := range allNodes {
		node := node
		if node.BindEcss == nil {
			continue
		}
		for _, obj := range node.BindEcss {
			obj := obj
			allResourceIdsMap[obj.ID] = struct{}{}
		}

	}
	return len(allResourceIdsMap), nil
}

//func (obj *StreeNode) BindEcsData() {
//	allNum := 0
//	groupByVendor := make(map[string]int)
//	groupByZoneId := make(map[string]int)
//	groupByOSName := make(map[string]int)
//	allNodes := []*StreeNode{obj}
//	//allNodes = append(allNodes, GetAllLeafNodes(int(obj.ID))...)
//	// 去掉自身算子节点数量
//	obj.NodeNum = len(allNodes) - 1
//	allResourceIdsMap := map[uint]struct{}{}
//	//childrens, err := GetAllLeafNodes(int(obj.ID))
//	//if err != nil {
//	//	return
//	//}
//	//
//	//if childrens != nil {
//	//	allNodes = append(allNodes, childrens...)
//	//}
//	for _, node := range allNodes {
//		if node.IsLeaf {
//			obj.LeafNodeNum++
//		}
//		node := node
//		if node.BindEcss == nil {
//			continue
//		}
//		for _, obj := range node.BindEcss {
//			obj := obj
//			groupByVendor[obj.Vendor]++
//			groupByOSName[obj.OSName]++
//			groupByZoneId[obj.ZoneId]++
//			allResourceIdsMap[obj.ID] = struct{}{}
//		}
//
//	}
//	allNum = len(allResourceIdsMap)
//	obj.EcsNum = allNum
//	arrGroupByVendor := make([]*EchartsOneItem, 0)
//	arrGroupByZoneId := make([]*EchartsOneItem, 0)
//	for name, value := range groupByVendor {
//		arrGroupByVendor = append(arrGroupByVendor, &EchartsOneItem{
//			Name:  name,
//			Value: value,
//		})
//	}
//	for name, value := range groupByZoneId {
//		arrGroupByZoneId = append(arrGroupByZoneId, &EchartsOneItem{
//			Name:  name,
//			Value: value,
//		})
//	}
//	var osNameKeys []string
//	var osNameValues []int
//	for name := range groupByOSName {
//		osNameKeys = append(osNameKeys, name)
//	}
//	sort.Strings(osNameKeys)
//
//	for _, name := range osNameKeys {
//		osNameValues = append(osNameValues, groupByOSName[name])
//	}
//	obj.GroupByVendor = arrGroupByVendor
//	obj.GroupByZoneId = arrGroupByZoneId
//	obj.GroupByOSNameOrderKeys = osNameKeys
//	obj.GroupByOSNameOrderValues = osNameValues
//	return
//}

// BindEcsData 填充统计数据
func (obj *StreeNode) BindEcsData() {
	allNum := 0
	groupByVendor := make(map[string]int)
	groupByZoneId := make(map[string]int)
	groupByOSName := make(map[string]int)

	allNodes := []*StreeNode{obj}

	childrens, err := GetAllLeafNodes(int(obj.ID))
	if err != nil {
		return
	}

	if childrens != nil {
		allNodes = append(allNodes, childrens...)
	}

	allResourceIdsMap := map[uint]struct{}{}

	// 🌟 新增：专门用两个变量分开统计目录和叶子
	dirCount := 0
	leafCount := 0

	// 🌟 预编译正则表达式：匹配括号里的纯数字，例如 "(100G)" 或 "(100)" 提取出 "100"
	diskRegexp := regexp.MustCompile(`\((\d+)[^)]*\)`)

	for _, node := range allNodes {
		// 🌟 核心修复：排除当前节点自身，分别统计目录和叶子
		if node.ID != obj.ID {
			if node.IsLeaf {
				leafCount++
			} else {
				dirCount++
			}
		}

		if node.BindEcss == nil {
			continue
		}

		for _, ecsObj := range node.BindEcss {
			// 资源去重统计
			if _, exists := allResourceIdsMap[ecsObj.ID]; !exists {
				groupByVendor[ecsObj.Vendor]++
				groupByOSName[ecsObj.OSName]++
				groupByZoneId[ecsObj.ZoneId]++
				obj.EcsCpuTotal += ecsObj.Cpu
				obj.EcsMemoryTotal += (ecsObj.Memory + 512) / 1024
				diskStr := fmt.Sprintf("%v", ecsObj.DiskIds)
				matches := diskRegexp.FindAllStringSubmatch(diskStr, -1)
				//obj.EcsDiskTotal += ecsObj.DiskIds
				for _, match := range matches {
					if len(match) > 1 {
						// 将提取出的字符串数字（如 "100"）转为整型 int
						size, err := strconv.Atoi(match[1])
						if err == nil {
							obj.EcsDiskTotal += size
						}
					}
				}
				allResourceIdsMap[ecsObj.ID] = struct{}{}
			}
		}
	}

	// 🌟 将分开统计的结果赋值
	obj.LeafNodeNum = leafCount
	obj.NodeNum = dirCount // 这里的 NodeNum 现在只代表“目录节点”的数量了

	allNum = len(allResourceIdsMap)
	obj.EcsNum = allNum

	// ----- 下面的图表数据组装逻辑保持不变 -----
	arrGroupByVendor := make([]*EchartsOneItem, 0)
	arrGroupByZoneId := make([]*EchartsOneItem, 0)

	for name, value := range groupByVendor {
		arrGroupByVendor = append(arrGroupByVendor, &EchartsOneItem{
			Name:  name,
			Value: value,
		})
	}
	for name, value := range groupByZoneId {
		arrGroupByZoneId = append(arrGroupByZoneId, &EchartsOneItem{
			Name:  name,
			Value: value,
		})
	}

	var osNameKeys []string
	var osNameValues []int
	for name := range groupByOSName {
		osNameKeys = append(osNameKeys, name)
	}
	sort.Strings(osNameKeys)

	for _, name := range osNameKeys {
		osNameValues = append(osNameValues, groupByOSName[name])
	}

	obj.GroupByVendor = arrGroupByVendor
	obj.GroupByZoneId = arrGroupByZoneId
	obj.GroupByOSNameOrderKeys = osNameKeys
	obj.GroupByOSNameOrderValues = osNameValues

	return
}

func GetStreeNodeAll() (sn []*StreeNode, err error) {
	err = Db.Find(&sn).Error
	return
}

func GetStreeNodeByLevel(level int) (sn []*StreeNode, err error) {
	err = Db.Where("level = ?", level).Preload("OpsAdmins").Preload("BindEcss").Find(&sn).Error
	return
}

func GetStreeNodeById(id int) (*StreeNode, error) {
	var dbStreeNode StreeNode
	err := Db.Where("id = ? ", id).Preload("OpsAdmins").Preload("BindEcss").First(&dbStreeNode).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("StreeNode不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbStreeNode, nil
}

func GetStreeNodesByPId(pid int) (dbObjs []*StreeNode, err error) {
	err = Db.Where("pid = ? ", pid).Preload("OpsAdmins").Preload("BindEcss").Find(&dbObjs).Error
	return
}

func (obj *StreeNode) UpdateStreeNode() error {
	return Db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(obj).Updates(obj).Error; err != nil {
			return err
		}
		return tx.Model(obj).Association("OpsAdmins").Replace(obj.OpsAdmins)
	})
}

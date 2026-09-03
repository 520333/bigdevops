package models

import (
	"bigdevops/src/config"

	"go.uber.org/zap"
)

func mockWorkOrderData(sc *config.ServerConfig, adminUser *SystemUser) {
	// 1. Mock 动态表单表
	forms := []*WorkOrderFormDesign{
		{
			Name:       "基础资源申请表单",
			UserID:     adminUser.ID,
			FormConfig: `{"schemas":[{"field":"resourceName","label":"资源名称","component":"Input","required":true},{"field":"reason","label":"申请原因","component":"InputTextArea","required":true}]}`,
		},
		{
			Name:       "权限开通申请表单",
			UserID:     adminUser.ID,
			FormConfig: `{"schemas":[{"field":"systemName","label":"系统名称","component":"Input","required":true},{"field":"roleName","label":"需要开通的角色","component":"Input","required":true},{"field":"expireTime","label":"过期时间","component":"DatePicker","required":false}]}`,
		},
	}

	for _, form := range forms {
		if err := Db.Create(form).Error; err != nil {
			sc.Logger.Error("Mock FormDesign 失败", zap.Error(err))
		}
	}

	// 2. Mock 审批流程链
	processes := []*WorkOrderProcess{
		{
			Name:   "基础直线审批",
			UserID: adminUser.ID,
			FlowNodes: []WorkOrderFlowNode{
				{Type: "起始节点", DefineUserOrGroup: "test"},
				{Type: "审批节点", DefineUserOrGroup: "admin"},
				{Type: "结束节点", DefineUserOrGroup: "test"},
			},
		},
		{
			Name:   "带执行的标准流程",
			UserID: adminUser.ID,
			FlowNodes: []WorkOrderFlowNode{
				{Type: "起始节点", DefineUserOrGroup: "test"},
				{Type: "审批节点", DefineUserOrGroup: "组@超级管理员"},
				{Type: "执行节点", DefineUserOrGroup: "admin"},
				{Type: "结束节点", DefineUserOrGroup: "bot_super"},
			},
		},
	}

	for _, p := range processes {
		if err := Db.Create(p).Error; err != nil {
			sc.Logger.Error("Mock Process 失败", zap.Error(err))
		}
	}

	// 3. Mock 工单复合模板关联
	templates := []*WorkOrderTemplate{
		{Name: "【测试】通用资源申请", UserID: adminUser.ID, FormDesignID: forms[0].ID, ProcessID: processes[0].ID},
		{Name: "【生产】核心系统权限申请", UserID: adminUser.ID, FormDesignID: forms[1].ID, ProcessID: processes[1].ID},
	}

	for _, tmpl := range templates {
		if err := Db.Create(tmpl).Error; err != nil {
			sc.Logger.Error("Mock WorkOrderTemplate 失败", zap.Error(err))
		}
	}

	// 4. 机器人自动化闭环专有工单 Mock
	autoForm := &WorkOrderFormDesign{
		Name:       "自动购买ECS资源表单",
		UserID:     adminUser.ID,
		FormConfig: `{"schemas":[{"field":"HostNames","label":"主机名(多行换行)","component":"InputTextArea","required":true}]}`,
	}
	_ = Db.Create(autoForm)

	autoProcess := &WorkOrderProcess{
		Name:   "全自动ECS交付流程",
		UserID: adminUser.ID,
		FlowNodes: []WorkOrderFlowNode{
			{Type: "起始节点", DefineUserOrGroup: "test"},
			{Type: "执行节点", DefineUserOrGroup: sc.WorkOrderAutoActionC.ServiceAccount},
			{Type: "结束节点", DefineUserOrGroup: "test"},
		},
	}
	_ = Db.Create(autoProcess)

	autoTemplate := &WorkOrderTemplate{
		Name:         sc.WorkOrderAutoActionC.AutoTemplateNameBuyEcs,
		UserID:       adminUser.ID,
		FormDesignID: autoForm.ID,
		ProcessID:    autoProcess.ID,
	}
	_ = Db.Create(autoTemplate)

	sc.Logger.Info("工单模块 Mock 数据注入成功")
}

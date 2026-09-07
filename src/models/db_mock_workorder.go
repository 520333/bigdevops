package models

import (
	"bigdevops/src/config"

	"go.uber.org/zap"
)

func mockWorkOrderData(sc *config.ServerConfig, adminUser *SystemUser) {
	// 1. Mock 动态表单表
	pipelineFormConfig := `{"schemas":[{"component":"Divider","label":"1. 代码仓库配置","icon":"radix-icons:divider-horizontal","field":"_divider_1","colProps":{"span":24},"componentProps":{"orientation":"center","dashed":true,"plain":false},"itemProps":{"labelCol":{},"wrapperCol":{}},"key":"_divider_1"},{"component":"ApiSelect","label":"GIT仓库","icon":"ant-design:api-outlined","field":"git_repo","required":true,"colProps":{"span":24},"componentProps":{"api":"/api/code/getCodeGitRepoList","resultField":"items","labelField":"fullName","valueField":"cloneUrlSsh","optionLabelProp":"value","placeholder":"下拉选择 GIT 仓库地址..."},"key":"_api_select_2","itemProps":{"labelCol":{},"wrapperCol":{}},"link":[]},{"component":"ApiSelect","label":"GIT分支","icon":"ant-design:branches-outlined","field":"git_branch","required":true,"colProps":{"span":24},"componentProps":{"api":"/api/code/getRepoBranches","labelField":"name","valueField":"name","params":{"fullName":"$git_repo"},"placeholder":"选定 GIT 仓库后自动获取真实分支..."},"link":["git_repo"],"key":"_api_select_1","itemProps":{"labelCol":{},"wrapperCol":{}}},{"component":"Divider","label":"2. 部署与构建环境","icon":"radix-icons:divider-horizontal","field":"_divider_2","colProps":{"span":24},"componentProps":{"orientation":"center","dashed":true},"itemProps":{"labelCol":{},"wrapperCol":{}},"key":"_divider_2"},{"component":"RadioGroup","label":"部署类型","icon":"carbon:radio-button-checked","field":"deploy_type","required":true,"colProps":{"span":24},"componentProps":{"defaultValue":"binary","options":[{"label":"二进制 / 主机发布","value":"binary"},{"label":"Docker 容器部署","value":"docker"},{"label":"K8S 容器集群","value":"k8s"}]},"key":"_radio_group_7","itemProps":{"labelCol":{},"wrapperCol":{}}},{"component":"ApiSelect","label":"目标集群","icon":"bi:input-cursor-text","field":"k8s","required":true,"colProps":{"span":24},"componentProps":{"type":"text","api":"/api/k8s/getK8sClusterList","resultField":"items","labelField":"nameZh","valueField":"name","optionLabelProp":"name"},"key":"_input_1","itemProps":{"labelCol":{},"wrapperCol":{}},"vShow":"values.deploy_type === 'k8s'"},{"component":"ApiSelect","label":"目标主机","icon":"bi:input-cursor-text","field":"host","required":true,"colProps":{"span":24},"componentProps":{"mode":"multiple","type":"text","api":"/api/stree/getResourceEcsList","resultField":"items","labelField":"label","valueField":"value","optionLabelProp":"value","placeholder":"请选择目标主机 (支持多选)..."},"key":"_api_select_2","itemProps":{"labelCol":{},"wrapperCol":{}},"vShow":"values.deploy_type === 'binary'||values.deploy_type === 'docker'"},{"component":"Select","label":"项目类型","icon":"gg:select","field":"project_type","required":true,"colProps":{"span":24},"componentProps":{"options":[{"label":"前端 (Frontend)","value":"frontend"},{"label":"后端 (Backend)","value":"backend"}]},"key":"_select_7","itemProps":{"labelCol":{},"wrapperCol":{}}},{"component":"Select","label":"JDK版本","icon":"gg:select","field":"jdk_version","colProps":{"span":24},"componentProps":{"options":[{"label":"jdk8","value":"8"},{"label":"jdk11","value":"11"},{"label":"jdk17","value":"17"},{"label":"jdk21","value":"21"}],"defaultOpen":false,"defaultValue":"17"},"key":"_select_9","itemProps":{"labelCol":{},"wrapperCol":{},"required":false,"help":"不选默认jdk17"},"vShow":"values.project_type === 'backend'"},{"component":"Select","label":"NODE版本","icon":"gg:select","field":"node_version","colProps":{"span":24},"componentProps":{"options":[{"label":"node16","value":"16"},{"label":"node18","value":"18"},{"label":"node20","value":"20"},{"label":"node22","value":"22"},{"label":"node24","value":"24"}],"defaultValue":"16"},"key":"_select_8","itemProps":{"labelCol":{},"wrapperCol":{},"help":"不选默认node16"},"vShow":"values.project_type === 'frontend'"},{"component":"InputTextArea","label":"构建命令","icon":"ant-design:file-text-filled","field":"frontend_build_command","colProps":{"span":24},"componentProps":{"rows":3,"defaultValue":"npm install --prefer-offline --registry=https://registry.npmmirror.com/ --loglevel=error && npm run build:stage -- --silent"},"key":"_input_text_area_16","itemProps":{"labelCol":{},"wrapperCol":{}},"vShow":"values.project_type === 'frontend'"},{"component":"InputTextArea","label":"构建命令","icon":"ant-design:file-text-filled","field":"backend_build_command","colProps":{"span":24},"componentProps":{"rows":3,"defaultValue":"mvn clean package -Dmaven.test.skip=true -T 1C -q"},"key":"_input_text_area_17","itemProps":{"labelCol":{},"wrapperCol":{}},"vShow":"values.project_type === 'backend'"},{"component":"Input","label":"产出目录","icon":"bi:input-cursor-text","field":"frontend_output_dir","colProps":{"span":24},"componentProps":{"defaultValue":"dist","placeholder":"例如 dist 或 build"},"key":"_input_frontend_dir","itemProps":{"labelCol":{},"wrapperCol":{},"help":"前端编译产物目录，不填默认 dist"},"vShow":"values.project_type === 'frontend'"},{"field":"_grid_23","component":"Grid","label":"后端参数配置分组","icon":"icon-grid","componentProps":{},"columns":[{"span":8,"children":[{"component":"Input","label":"模块路径","icon":"bi:input-cursor-text","field":"module_path","colProps":{"span":24,"offset":0,"order":0,"pull":0,"push":0},"componentProps":{"defaultValue":"."},"key":"_input_20","itemProps":{"labelCol":{},"wrapperCol":{}},"vShow":"values.project_type === 'backend'"}]},{"span":8,"children":[{"component":"Select","label":"配置文件","icon":"gg:select","field":"use_config","colProps":{"span":24},"componentProps":{"defaultValue":"dev","options":[{"label":"dev","value":"dev"},{"label":"test","value":"test"},{"label":"stage","value":"stage"},{"label":"pre","value":"pre"},{"label":"prod","value":"prod"}]},"key":"_select_22","itemProps":{"labelCol":{},"wrapperCol":{}},"vShow":"values.project_type === 'backend'"}]},{"span":8,"children":[{"component":"Input","label":"监听端口","icon":"bi:input-cursor-text","field":"listen_port","colProps":{"span":24,"offset":0},"componentProps":{"defaultValue":"8080"},"key":"_input_18","itemProps":{"labelCol":{},"wrapperCol":{}},"vShow":"values.project_type === 'backend'"}]}],"colProps":{"span":24},"options":{"gutter":0},"key":"_grid_23","itemProps":{"labelCol":{},"wrapperCol":{}},"vShow":"values.project_type === 'backend'"}],"layout":"horizontal","labelLayout":"flex","labelWidth":100,"labelCol":{"span":4},"wrapperCol":{"span":20},"currentItem":{"component":"Divider","label":"2. 部署与构建环境","icon":"radix-icons:divider-horizontal","field":"_divider_2","colProps":{"span":24},"componentProps":{"orientation":"center","dashed":true},"itemProps":{"labelCol":{},"wrapperCol":{}},"key":"_divider_2"},"activeKey":4}`

	forms := []*WorkOrderFormDesign{
		{
			Name:       "流水线开通表单",
			UserID:     adminUser.ID,
			FormConfig: pipelineFormConfig,
		},
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
			Name:   "流水线自动化开通与构建流程",
			UserID: adminUser.ID,
			FlowNodes: []WorkOrderFlowNode{
				{Type: "审批节点", DefineUserOrGroup: "admin"},
				{Type: "执行节点", DefineUserOrGroup: sc.WorkOrderAutoActionC.ServiceAccount},
			},
		},
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
		{Name: "流水线开通申请", UserID: adminUser.ID, FormDesignID: forms[0].ID, ProcessID: processes[0].ID},
		{Name: "【测试】通用资源申请", UserID: adminUser.ID, FormDesignID: forms[1].ID, ProcessID: processes[1].ID},
		{Name: "【生产】核心系统权限申请", UserID: adminUser.ID, FormDesignID: forms[2].ID, ProcessID: processes[2].ID},
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

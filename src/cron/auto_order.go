package cron

import (
	"bigdevops/src/common"
	"bigdevops/src/models"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gammazero/workerpool"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

// AuthOrderManager 启动定时扫描任务
func (cm *CronManager) AuthOrderManager(ctx context.Context) error {
	go wait.UntilWithContext(ctx, cm.RunAutoOrder, time.Duration(cm.Sc.WorkOrderAutoActionC.RunIntervalSeconds)*time.Second)
	<-ctx.Done()
	cm.Sc.Logger.Info("AuthManager收到其他任务退出信号")
	return nil
}

// RunAutoOrder 扫描待执行工单
func (cm *CronManager) RunAutoOrder(ctx context.Context) {
	start := time.Now()
	serviceAccountName := cm.Sc.WorkOrderAutoActionC.ServiceAccount
	if serviceAccountName == "" {
		cm.Sc.Logger.Error("自动执行账号没有配置，退出工单执行")
		return
	}

	// 使用精准的节点名称匹配查询
	pendingOrders, count, err := models.GetWorkOrderInstanceByStatusAndCurrentNode(common.WORKORDER_INSTANCE_PENDING_ACTION, serviceAccountName)
	if err != nil {
		cm.Sc.Logger.Error("[工单自动执行]扫描数据库中待执行的工单失败", zap.Error(err))
		return
	}

	cm.Sc.Logger.Info("[工单自动执行]扫描到待执行工单", zap.Int64("总数", count))
	if count == 0 {
		return
	}

	wp := workerpool.New(cm.Sc.WorkOrderAutoActionC.BatchNum)
	for _, pendingOrder := range pendingOrders {
		pendingOrder := pendingOrder // 防止闭包变量逃逸
		wp.Submit(func() {
			cm.RunAutoOrderOne(pendingOrder)
		})
	}
	wp.StopWait()

	tookSeconds := time.Since(start).Seconds()
	cm.Sc.Logger.Info("[工单自动执行]本轮执行结束",
		zap.Float64("执行耗时(秒)", tookSeconds),
		zap.Int64("执行总数", count),
	)
}

func (cm *CronManager) RunAutoOrderOne(pendingOrder *models.WorkOrderInstance) {
	cm.Sc.Logger.Info("[工单自动执行] ⚡ 开始处理单条", zap.String("标题", pendingOrder.Title))
	pendingOrder.FillFrontAllData()

	template := pendingOrder.Template
	// 补全 nil 模板
	if template == nil {
		dbTemplate, err := models.GetWorkOrderTemplateById(int(pendingOrder.TemplateId))
		if err == nil {
			template = dbTemplate
			pendingOrder.Template = dbTemplate
		} else {
			cm.Sc.Logger.Error("[工单自动执行] ❌ 数据库查不到对应模板，中止", zap.Uint("TID", pendingOrder.TemplateId))
			return
		}
	}

	switch template.Name {
	case cm.Sc.WorkOrderAutoActionC.AutoTemplateNameBuyEcs:
		cm.RunAutoOrderOneByEcs(pendingOrder)
	default:
		cm.Sc.Logger.Warn("[工单自动执行] ⚠️ 模板名未匹配，跳过", zap.String("模板名", template.Name))
	}
}

// RunAutoOrderOneByEcs 执行购买 ECS 逻辑
func (cm *CronManager) RunAutoOrderOneByEcs(pendingOrder *models.WorkOrderInstance) {
	var ecsBuyWorkOrderReq models.EcsBuyWorkOrder
	err := json.Unmarshal([]byte(pendingOrder.ActualApiJsonData), &ecsBuyWorkOrderReq)
	if err != nil {
		cm.Sc.Logger.Error("[工单自动执行][购买虚拟机] json解析失败", zap.Error(err))
		cm.finishAutoAction(pendingOrder, false, fmt.Sprintf("参数解析失败: %v", err))
		return
	}

	hostNames := strings.Split(ecsBuyWorkOrderReq.HostNames, "\n")
	var successMsgs []string
	var errMsgs []string
	now := time.Now()

	for index, hostName := range hostNames {
		hostName = strings.TrimSpace(hostName)
		if hostName == "" {
			continue
		}

		objOne := &models.ResourceEcs{
			InstanceId:        fmt.Sprintf("i-mock-%d-%d", now.Unix(), index),
			InstanceName:      hostName,
			HostName:          hostName,
			InstanceType:      "ecs.g6.large",
			Status:            "Running",
			Cpu:               2,
			Memory:            8,
			OSName:            "Ubuntu 22.04 64位",
			VmType:            "1",
			PrivateIpAddress:  models.StringArray{fmt.Sprintf("172.16.100.%d", 10+index)},
			PublicIpAddresses: models.StringArray{fmt.Sprintf("47.100.20.%d", 10+index)},
			CreationTime:      &now,
		}

		// 🚨 必须生成 Hash，防止唯一索引冲突
		objOne.Hash = objOne.GenHash()

		err = objOne.CreateOne()
		if err != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("机器 [%s] 创建失败: %v", hostName, err))
		} else {
			successMsgs = append(successMsgs, fmt.Sprintf("机器 [%s] 创建成功 (IP: %v)", hostName, objOne.PrivateIpAddress[0]))
			cm.autoBindToStreeNode(objOne, ecsBuyWorkOrderReq.BindNode)
		}
	}

	if len(errMsgs) > 0 {
		cm.finishAutoAction(pendingOrder, false, strings.Join(errMsgs, "\n")+"\n"+strings.Join(successMsgs, "\n"))
	} else {
		cm.finishAutoAction(pendingOrder, true, strings.Join(successMsgs, "\n"))
	}
}

// finishAutoAction 负责修改流转记录，并推进工单状态，防止死循环
func (cm *CronManager) finishAutoAction(order *models.WorkOrderInstance, isSuccess bool, output string) {
	var flowNodes []map[string]interface{}
	_ = json.Unmarshal([]byte(order.ActualFlowData), &flowNodes)

	for i, node := range flowNodes {
		if node["actualUser"] == "" && (node["type"] == "执行节点" || strings.Contains(fmt.Sprintf("%v", node["type"]), "执行")) {
			flowNodes[i]["actualUser"] = cm.Sc.WorkOrderAutoActionC.ServiceAccount
			flowNodes[i]["outPut"] = output
			flowNodes[i]["isPassOrIsSuccess"] = isSuccess
			flowNodes[i]["endTime"] = time.Now().Format("2006-01-02 15:04:05")
			break
		}
	}

	newFlowData, _ := json.Marshal(flowNodes)
	order.ActualFlowData = string(newFlowData)
	order.CurrentFlowNode = ""

	if isSuccess {
		order.Status = common.WORKORDER_INSTANCE_FINISHED
	} else {
		order.Status = common.WORKORDER_INSTANCE_PENDING_ACTION
	}

	if err := order.UpdateOne(); err != nil {
		cm.Sc.Logger.Error("自动执行后更新数据库状态失败", zap.Error(err))
	} else {
		cm.Sc.Logger.Info("🤖 自动执行工单已收尾", zap.String("标题", order.Title), zap.Bool("成功", isSuccess))
	}
}

func (cm *CronManager) autoBindToStreeNode(ecs *models.ResourceEcs, bindNodeTitle string) {
	if bindNodeTitle == "" {
		cm.Sc.Logger.Warn("[工单自动执行] 工单未指定绑定节点，跳过绑定")
		return
	}

	// 1. 根据节点名称查找到该 StreeNode 对象
	var streeNode models.StreeNode
	err := models.Db.Where("title = ?", bindNodeTitle).First(&streeNode).Error
	if err != nil {
		cm.Sc.Logger.Error("[工单自动执行] 绑定失败，找不到对应服务树节点",
			zap.String("节点名称", bindNodeTitle), zap.Error(err))
		return
	}

	// 2. 使用 GORM 的 Association 进行绑定
	// 这里使用 Replace，确保该机器只关联这个节点（也可以用 Append）
	err = models.Db.Model(ecs).Association("BindNodes").Append(&streeNode)
	if err != nil {
		cm.Sc.Logger.Error("[工单自动执行] 绑定 ECS 到服务树失败", zap.Error(err))
	} else {
		cm.Sc.Logger.Info("[工单自动执行] ✅ 机器已成功自动绑定到服务树节点",
			zap.String("ECS", ecs.InstanceName),
			zap.String("节点", bindNodeTitle))
	}
}

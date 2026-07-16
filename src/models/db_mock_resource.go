package models

import (
	"bigdevops/src/config"

	"go.uber.org/zap"
)

func mockResourceData(sc *config.ServerConfig, adminUser *User) {
	ecss := []*ResourceEcs{
		{InstanceId: "7aed4d56-71d7-3307-7cee-919822564f6b", InstanceName: "k8s-master", VmType: 2, Cpu: 8, Memory: 15954, OSName: "ubuntu 22.04", HostName: "k8s-master", PrivateIpAddress: []string{"192.168.50.200"}, DiskIds: []string{"98"}},
		{InstanceId: "ea8b4d56-a119-f7ff-81e9-f3ea18c9df36", InstanceName: "k8s-node01", VmType: 2, Cpu: 8, Memory: 7902, OSName: "ubuntu 22.04", HostName: "k8s-node01", PrivateIpAddress: []string{"192.168.50.201"}, DiskIds: []string{"98"}},
	}
	for _, ecs := range ecss {
		// 1. 生成唯一 Hash
		ecs.Hash = ecs.GenHash()
		_ = ecs.CreateOne()
	}

	streeNodes := []*StreeNode{
		{Title: "TORKEY", Desc: "拓基时代-国际事业", Pid: 0, Level: 1, OpsAdmins: []*User{adminUser}},
		{Title: "研发一组", Pid: 1, Level: 2, Desc: "umipay、binance、upipay项目"},
		{Title: "umipay项目", Pid: 2, Level: 2, IsLeaf: true, Desc: "umipay项目"},
		{Title: "binance项目", Pid: 2, Level: 2, IsLeaf: true, Desc: "binance项目"},
		{Title: "upipay项目", Pid: 2, Level: 2, IsLeaf: true, Desc: "upipay项目"},
		{Title: "研发二组", Pid: 1, Level: 2, Desc: "发卡项目"},
		{Title: "全球发卡项目", Pid: 6, Level: 2, IsLeaf: true, Desc: "虚拟卡/物理卡全球发卡业务项目"},
		{Title: "研发三组", Pid: 1, Level: 2, Desc: "资管项目"},
		{Title: "资管项目", Pid: 8, Level: 2, IsLeaf: true, Desc: "资管项目"},
	}

	for _, node := range streeNodes {
		_ = node.CreateOne()
		if node.IsLeaf && node.ID == 3 {
			err := Db.Model(node).Association("BindEcss").Append(ecss[0])
			if err != nil {
				sc.Logger.Error("绑定ECS到叶子节点失败", zap.Error(err))
			}
		}
	}

	sc.Logger.Info("CMDB服务树模块 Mock 数据注入成功")
}

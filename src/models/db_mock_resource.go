package models

import (
	"bigdevops/src/config"
)

func mockResourceData(sc *config.ServerConfig, adminUser *User) {
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
		node.CreateOne()
	}
	sc.Logger.Info("CMDB服务树模块 Mock 数据注入成功")
}

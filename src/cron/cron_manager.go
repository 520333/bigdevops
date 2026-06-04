package cron

import (
	"bigdevops/src/config"
	"sync"
)

// CronManager 定义执行计划任务对象
type CronManager struct {
	Sc                  *config.ServerConfig
	EcsLastSyncFinished bool // =true代表上次已经同步完了
	ElbLastSyncFinished bool
	RdsLastSyncFinished bool
	DnsLastSyncFinished bool
	sync.RWMutex
}

func (cm *CronManager) SetEcsSynced(fin bool) {
	cm.Lock()
	defer cm.Unlock()
	cm.EcsLastSyncFinished = fin
}

func (cm *CronManager) GetEcsSynced() bool {
	cm.RLock()
	defer cm.RUnlock()
	return cm.EcsLastSyncFinished
}

func (cm *CronManager) SetElbSynced(fin bool) {
	cm.Lock()
	defer cm.Unlock()
	cm.ElbLastSyncFinished = fin
}

func (cm *CronManager) GetElbSynced() bool {
	cm.RLock()
	defer cm.RUnlock()
	return cm.ElbLastSyncFinished
}

func (cm *CronManager) SetRdsSynced(fin bool) {
	cm.Lock()
	defer cm.Unlock()
	cm.RdsLastSyncFinished = fin
}

func (cm *CronManager) GetRdsSynced() bool {
	cm.RLock()
	defer cm.RUnlock()
	return cm.RdsLastSyncFinished
}

func (cm *CronManager) SetDnsSynced(fin bool) {
	cm.Lock()
	defer cm.Unlock()
	cm.DnsLastSyncFinished = fin
}

func (cm *CronManager) GetDnsSynced() bool {
	cm.RLock()
	defer cm.RUnlock()
	return cm.DnsLastSyncFinished
}

func NewCronManager(sc *config.ServerConfig) *CronManager {
	return &CronManager{Sc: sc,
		EcsLastSyncFinished: true,
		ElbLastSyncFinished: true,
		RdsLastSyncFinished: true,
		DnsLastSyncFinished: true}
}

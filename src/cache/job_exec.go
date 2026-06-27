package cache

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

type TaskCache struct {
	sync.RWMutex
	M           map[string][]*models.JobTask
	Sc          *config.ServerConfig
	UnderUpdate bool // 在全量更新中的标志位
}

func (tc *TaskCache) ReNewCache() {
	tc.SyncCache(context.TODO())

}

func NewTaskCache(sc *config.ServerConfig) *TaskCache {
	tc := &TaskCache{
		RWMutex: sync.RWMutex{},
		M:       make(map[string][]*models.JobTask),
		Sc:      sc,
	}
	return tc
}
func (tc *TaskCache) TaskCacheManager(ctx context.Context) error {
	go wait.UntilWithContext(ctx, tc.SyncCache, time.Duration(tc.Sc.JobExec.RunIntervalSeconds)*time.Second)
	<-ctx.Done()
	tc.Sc.Logger.Info("SyncCache 收到其他任务退出信号")
	return nil
}

func (tc *TaskCache) SyncCache(ctx context.Context) {
	nowTime := time.Now()
	tasks, err := models.GetJobTaskUnDone([]string{common.JOB_STATUS_RUNNING, common.JOB_STATUS_KILLING})
	if err != nil {
		tc.Sc.Logger.Error("[任务执行模块]扫描数据库中待处理的任务失败", zap.Error(err))
		return
	}

	resM := make(map[string][]*models.JobTask)
	if len(tasks) == 0 {
		return
	}

	for _, task := range tasks {
		toKill := false

		task := task
		toKillIps := []string{}
		//if task.Status == common.JOB_STATUS_KILLING && time.Now().Sub(task.UpdatedAt) > time.Minute {
		if task.Status == common.JOB_STATUS_KILLING && time.Now().Sub(task.UpdatedAt) > time.Minute {
			task.Status = common.JOB_STATUS_KILLED
			task.UpdateOne()
			toKill = true
		}

		scheduledHosts, err := models.GetJobResultByJobId(int(task.ID))
		if err != nil {
			tc.Sc.Logger.Error("[任务执行模块]查找这个任务已经下发的IP列表错误",
				zap.Error(err),
				zap.Any("任务id", task.ID),
				zap.Any("任务名称", task.Title),
			)
			continue
		}
		var allIps []string
		//err = json.Unmarshal([]byte(task.HostsRaw), &toKillIps)
		err = json.Unmarshal([]byte(task.HostsRaw), &allIps)
		if err != nil {
			tc.Sc.Logger.Error("[任务执行模块]json解析这个任务ip列表错误",
				zap.Error(err),
				zap.Any("任务id", task.ID),
				zap.Any("任务名称", task.Title),
			)
			continue
		}

		allIpsMap := make(map[string]struct{})
		for _, ip := range allIps {
			allIpsMap[ip] = struct{}{}
		}

		for _, scheduledHost := range scheduledHosts {
			scheduledHost := scheduledHost
			delete(allIpsMap, scheduledHost.HostIP)
		}

		if toKill {
			for _, ip := range toKillIps {
				allIpsMap[ip] = struct{}{}
			}
		}
		// 并发度
		sortIpArr := []string{}
		for ip := range allIpsMap {
			sortIpArr = append(sortIpArr, ip)
		}
		sort.Strings(sortIpArr)
		scheduledNum := 0

		for _, ip := range sortIpArr {

			if task.BatchSize > 0 && scheduledNum >= task.BatchSize {
				break
			}

			taskArr, loaded := resM[ip]
			if !loaded {
				taskArr = make([]*models.JobTask, 0)
			}
			taskArr = append(taskArr, task)
			resM[ip] = taskArr
			scheduledNum++
		}

	}

	for ip, tasks := range resM {
		taskIds := []uint{}
		for _, task := range tasks {
			task := task
			taskIds = append(taskIds, task.ID)
		}
		tc.Sc.Logger.Info("[任务执行模块]打印这个resM",
			zap.Any("总数量", len(tasks)),
			zap.Any("这个ip的任务数量", len(tasks)),
			zap.Any("ip", ip),
			zap.Any("任务", taskIds),
		)
	}

	tc.Lock()
	tc.M = resM
	tc.Unlock()
	tc.Sc.Logger.Info("[任务执行模块]扫描数据库中待处理的任务结果",
		zap.Any("数量", len(tasks)),
		zap.Duration("耗时", time.Since(nowTime)),
	)

}

func (tc *TaskCache) GetTaskByIp(ip string) []*models.JobTask {
	tc.RLock()
	defer tc.RUnlock()
	return tc.M[ip]
}

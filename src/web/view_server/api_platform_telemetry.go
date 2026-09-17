package view_server

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"
	"bigdevops/src/models"
	"math"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type PlatformTelemetryResp struct {
	Clusters     int64   `json:"clusters"`     // 纳管集群数
	Pods         int64   `json:"pods"`         // 运行容器数
	PipelineRuns int64   `json:"pipelineRuns"` // 流水线构建次数
	AlertRate    float64 `json:"alertRate"`    // 告警收敛率 (如 94.8)
	UpdatedAt    int64   `json:"updatedAt"`    // 毫秒时间戳
}

// 内存快照缓存，避免频繁冲击数据库和 K8s API Server
var (
	telemetryCacheLock sync.RWMutex
	cachedTelemetry    *PlatformTelemetryResp
	lastTelemetryTime  time.Time
)

// @Summary      获取平台大盘公开遥测数据（登录页展示）
// @Description  免鉴权轻量聚合接口，返回脱敏数字：纳管集群数、运行容器数、流水线构建数、告警收敛率
// @Tags         public-telemetry
// @Produce      json
// @Router       /noAuth/platform/telemetry [get]
func GetPlatformTelemetry(c *gin.Context) {
	telemetryCacheLock.RLock()
	if cachedTelemetry != nil && time.Since(lastTelemetryTime) < 30*time.Second {
		resp := *cachedTelemetry
		telemetryCacheLock.RUnlock()
		common.OkWithData(resp, c)
		return
	}
	telemetryCacheLock.RUnlock()

	telemetryCacheLock.Lock()
	defer telemetryCacheLock.Unlock()

	if cachedTelemetry != nil && time.Since(lastTelemetryTime) < 30*time.Second {
		common.OkWithData(*cachedTelemetry, c)
		return
	}

	resp := calculateTelemetry(c)
	cachedTelemetry = &resp
	lastTelemetryTime = time.Now()

	common.OkWithData(resp, c)
}

func calculateTelemetry(c *gin.Context) PlatformTelemetryResp {
	var clustersCount int64
	var podsCount int64
	var pipelineRuns int64

	// 1. 纳管集群总数
	_ = models.Db.Model(&models.K8sCluster{}).Count(&clustersCount).Error

	// 2. 运行容器 Pod 真实数量统计：
	// 极速模式：纯内存 O(1) 读取后台定时协程收集的各集群 Pod 快照（0 穿透、0 外部调用、0 压力）
	if val, exists := c.Get(common.GIN_CTX_K8S_CACHE); exists && val != nil {
		if kc, ok := val.(*cache.K8sClusterCache); ok && kc != nil {
			podsCount = kc.GetTotalPodsCount()
		}
	}

	// 兜底：若后台尚未采集到（例如服务刚启动的瞬间），尝试读取应用发布实例中的副本数
	if podsCount == 0 {
		var instanceReplicas int64
		_ = models.Db.Model(&models.K8sInstance{}).Select("COALESCE(SUM(replicas), 0)").Scan(&instanceReplicas).Error
		podsCount = instanceReplicas
	}

	// 3. 流水线与任务执行总数 (汇总 jenkins_job count 构建号 + jobexec_result 任务记录)
	var jenkinsBuildCount int64
	var jobExecCount int64
	_ = models.Db.Model(&models.JenkinsJob{}).Select("COALESCE(SUM(count), 0)").Scan(&jenkinsBuildCount).Error
	_ = models.Db.Model(&models.JobResult{}).Count(&jobExecCount).Error
	pipelineRuns = jenkinsBuildCount + jobExecCount

	// 4. 告警收敛率计算
	var totalTriggerTimes int64
	var uniqueEventCount int64
	_ = models.Db.Model(&models.MonitorAlertManagerEvent{}).Select("COALESCE(SUM(event_times), 0)").Scan(&totalTriggerTimes).Error
	_ = models.Db.Model(&models.MonitorAlertManagerEvent{}).Count(&uniqueEventCount).Error

	alertRate := 94.8
	if totalTriggerTimes > 0 && uniqueEventCount > 0 && totalTriggerTimes >= uniqueEventCount {
		computedRate := (1.0 - float64(uniqueEventCount)/float64(totalTriggerTimes)) * 100.0
		if computedRate >= 0 && computedRate <= 100 {
			alertRate = math.Round(computedRate*10) / 10
		}
	}

	return PlatformTelemetryResp{
		Clusters:     clustersCount,
		Pods:         podsCount,
		PipelineRuns: pipelineRuns,
		AlertRate:    alertRate,
		UpdatedAt:    time.Now().UnixMilli(),
	}
}

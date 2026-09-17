package cache

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	metricsClientSet "k8s.io/metrics/pkg/client/clientset/versioned"
)

type K8sClusterCache struct {
	sync.RWMutex
	KubeClientsMap        map[uint]*kubernetes.Clientset
	MetricsClientSetMap   map[uint]*metricsClientSet.Clientset
	DynamicClientMap      map[uint]*dynamic.DynamicClient
	KubeClientsProbErrMsg map[uint]string
	ClusterPodsCount      map[uint]int64 // 后台定期采集的各集群 Pod 数量快照
	Sc                    *config.ServerConfig
}

func NewK8sClusterCache(sc *config.ServerConfig) *K8sClusterCache {
	kc := &K8sClusterCache{
		KubeClientsMap:        make(map[uint]*kubernetes.Clientset),
		MetricsClientSetMap:   make(map[uint]*metricsClientSet.Clientset),
		DynamicClientMap:      make(map[uint]*dynamic.DynamicClient),
		KubeClientsProbErrMsg: make(map[uint]string),
		ClusterPodsCount:      make(map[uint]int64),
		Sc:                    sc,
	}
	return kc
}

func (obj *K8sClusterCache) K8sClusterCacheManager(ctx context.Context) error {
	go wait.UntilWithContext(ctx, obj.ReNewClientsMap, time.Duration(obj.Sc.K8sClusterC.RunIntervalSeconds)*time.Second)
	<-ctx.Done()
	obj.Sc.Logger.Info("ReNewClientsMap 收到其他任务退出信号")
	return nil
}

func (obj *K8sClusterCache) ReNewClientsMap(ctx context.Context) {
	kcs, err := models.GetK8sClusterAll()
	if err != nil {
		obj.Sc.Logger.Error("[k8s模块]扫描数据中的k8s集群失败", zap.Error(err))
		return
	}
	if len(kcs) == 0 {
		return
	}

	m := make(map[uint]*kubernetes.Clientset)
	sm := make(map[uint]*metricsClientSet.Clientset)
	dm := make(map[uint]*dynamic.DynamicClient)
	lastErrM := make(map[uint]string)
	podsCountM := make(map[uint]int64)

	for _, kc := range kcs {
		kc := kc
		// 1. 生成 Clientset、MetricsClientSet 及 DynamicClient
		restConfig, kClientSet, mClientSet, err := common.GenK8sClientSetByKubeconfigContent(kc.KubeConfigContent, kc.ActionTimeoutSeconds)
		if err != nil {
			obj.Sc.Logger.Error("[k8s模块]解析KubeConfig内存内容失败", zap.Error(err), zap.Any("集群名称", kc.Name))
			lastErrM[kc.ID] = "解析KubeConfig失败: " + err.Error()
			continue
		}
		m[kc.ID] = kClientSet
		if mClientSet != nil {
			sm[kc.ID] = mClientSet
		}
		dyClient, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			obj.Sc.Logger.Error("[k8s模块]生成DynamicClient失败", zap.Error(err), zap.Any("集群名称", kc.Name))
		} else {
			dm[kc.ID] = dyClient
		}

		// 2. 发起探活
		version, err := kClientSet.ServerVersion()
		if err != nil {
			obj.Sc.Logger.Error("[k8s模块]获取集群版本(探活)失败", zap.Error(err), zap.Any("集群名称", kc.Name))
			lastErrM[kc.ID] = "连接Kubernetes失败: " + err.Error()
		} else if version != nil && version.GitVersion != "" {
			lastErrM[kc.ID] = ""
			// 3. 探活成功：后台异步安全统计 Pod 数量快照（带 3 秒严格超时，绝不阻塞主流程）
			ctxPods, cancelPods := context.WithTimeout(ctx, 3*time.Second)
			podList, err := kClientSet.CoreV1().Pods("").List(ctxPods, metav1.ListOptions{})
			cancelPods()
			if err == nil {
				podsCountM[kc.ID] = int64(len(podList.Items))
			} else {
				obj.Sc.Logger.Warn("[k8s模块]后台快照统计Pod数量失败", zap.Error(err), zap.Any("集群名称", kc.Name))
			}
		} else {
			lastErrM[kc.ID] = "未知错误: 服务端GitVersion为空"
		}
	}

	obj.Lock()
	obj.KubeClientsMap = m
	obj.MetricsClientSetMap = sm
	obj.DynamicClientMap = dm
	obj.KubeClientsProbErrMsg = lastErrM
	obj.ClusterPodsCount = podsCountM
	obj.Unlock()
}

func (obj *K8sClusterCache) GetClusterProbeErrMsgById(id uint) string {
	obj.RLock()
	defer obj.RUnlock()
	return obj.KubeClientsProbErrMsg[id]
}

func (obj *K8sClusterCache) GetClusterProbeResultById(id uint) bool {
	obj.RLock()
	defer obj.RUnlock()
	errMsg, ok := obj.KubeClientsProbErrMsg[id]
	return ok && errMsg == ""
}

func (obj *K8sClusterCache) GetClusterClientSetById(id uint) *kubernetes.Clientset {
	obj.RLock()
	defer obj.RUnlock()
	return obj.KubeClientsMap[id]
}

func (obj *K8sClusterCache) GetClusterMetricsSetById(id uint) *metricsClientSet.Clientset {
	obj.RLock()
	defer obj.RUnlock()
	return obj.MetricsClientSetMap[id]
}

func (obj *K8sClusterCache) GetClusterDynamicClientById(id uint) *dynamic.DynamicClient {
	obj.RLock()
	defer obj.RUnlock()
	return obj.DynamicClientMap[id]
}

// GetTotalPodsCount 纯内存读取所有集群的 Pod 总数（0 毫秒、0 穿透、0 压力）
func (obj *K8sClusterCache) GetTotalPodsCount() int64 {
	obj.RLock()
	defer obj.RUnlock()
	var total int64
	for _, count := range obj.ClusterPodsCount {
		total += count
	}
	return total
}

// GetClusterPodsCountById 纯内存读取单个集群的 Pod 总数
func (obj *K8sClusterCache) GetClusterPodsCountById(id uint) int64 {
	obj.RLock()
	defer obj.RUnlock()
	return obj.ClusterPodsCount[id]
}

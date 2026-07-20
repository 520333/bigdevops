package cache

import (
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sClusterCache struct {
	sync.RWMutex
	KubeClientsMap   map[uint]*kubernetes.Clientset
	KubeClientsAlive map[uint]bool
	Sc               *config.ServerConfig
}

func NewK8sClusterCache(sc *config.ServerConfig) *K8sClusterCache {
	kc := &K8sClusterCache{
		KubeClientsMap:   make(map[uint]*kubernetes.Clientset),
		KubeClientsAlive: make(map[uint]bool),
		Sc:               sc,
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
	//obj.Sc.Logger.Info("[k8s模块] 开始执行 ReNewClientsMap...")
	kcs, err := models.GetK8sClusterAll()
	if err != nil {
		obj.Sc.Logger.Error("[k8s模块]扫描数据中的k8s集群失败", zap.Error(err))
		return
	}
	if len(kcs) == 0 {
		return
	}

	m := make(map[uint]*kubernetes.Clientset)
	aliveM := make(map[uint]bool)
	for _, kc := range kcs {
		kc := kc
		kConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kc.KubeConfigContent))
		if err != nil {
			obj.Sc.Logger.Error("[k8s模块]解析KubeConfig内存内容失败", zap.Error(err), zap.Any("集群名称", kc.Name))
			continue
		}

		clientSet, err := kubernetes.NewForConfig(kConfig)
		if err != nil {
			obj.Sc.Logger.Error("[k8s模块]生成NewForConfig错误", zap.Error(err), zap.Any("集群名称", kc.Name))
			continue
		}
		m[kc.ID] = clientSet

		version, err := clientSet.ServerVersion()
		if err != nil {
			obj.Sc.Logger.Error("[k8s模块]获取集群版本(探活)失败", zap.Error(err), zap.Any("集群名称", kc.Name))
			aliveM[kc.ID] = false
		} else if version != nil && version.GitVersion != "" {
			aliveM[kc.ID] = true
		} else {
			aliveM[kc.ID] = false
		}

		//ctx1, cancel1 := common.GenTimeoutContext(kc.ActionTimeoutSeconds)
		//nodes, err := clientSet.CoreV1().Nodes().List(ctx1, metav1.ListOptions{})
		//cancel1()

		//if err != nil {
		//	continue
		//}

		//for index, node := range nodes.Items {
		//	node := node
		//	obj.Sc.Logger.Info("[k8s模块]测试获取节点",
		//		zap.Any("集群", kc.Name),
		//		zap.Any("序号", index),
		//		zap.Any("节点名称", node.Name),
		//		zap.Any("内核版本", node.Status.NodeInfo.KernelVersion),
		//		zap.Any("内核版本", node.Status.NodeInfo.OSImage),
		//		zap.Any("内核版本", node.Status.NodeInfo.ContainerRuntimeVersion),
		//	)
		//}

	}

	//obj.Sc.Logger.Info("[k8s模块] 探活结果汇总", zap.Any("aliveM", aliveM))
	obj.Lock()
	obj.KubeClientsMap = m
	obj.KubeClientsAlive = aliveM
	obj.Unlock()
}

func (obj *K8sClusterCache) GetClusterProbeResultById(id uint) bool {
	obj.RLock()
	defer obj.RUnlock()
	return obj.KubeClientsAlive[id]
}

package cache

import (
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sClusterCache struct {
	sync.RWMutex
	KubeClientsMap map[uint]*kubernetes.Clientset
	Sc             *config.ServerConfig
}

func NewK8sClusterCache(sc *config.ServerConfig) *K8sClusterCache {
	kc := &K8sClusterCache{
		KubeClientsMap: make(map[uint]*kubernetes.Clientset),
		Sc:             sc,
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
	for _, kc := range kcs {
		kc := kc
		// 【彻底修复】：直接在内存中解析 KubeConfig 内容，不生成任何临时文件
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
		//obj.Sc.Logger.Info("[k8s模块]读取kubeconfig生成NewForConfig成功", zap.Any("集群名称", kc.Name))
		nodes, err := clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})

		if err != nil {
			continue
		}
		for index, node := range nodes.Items {
			node := node
			obj.Sc.Logger.Info("[k8s模块]测试获取节点",
				zap.Any("集群", kc.Name),
				zap.Any("序号", index),
				zap.Any("节点名称", node.Name),
				zap.Any("内核版本", node.Status.NodeInfo.KernelVersion),
				zap.Any("内核版本", node.Status.NodeInfo.OSImage),
				zap.Any("内核版本", node.Status.NodeInfo.ContainerRuntimeVersion),
			)
		}

	}

	obj.Lock()
	obj.KubeClientsMap = m
	obj.Unlock()
}

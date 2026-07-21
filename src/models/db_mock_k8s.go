package models

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func mockK8sData(sc *config.ServerConfig, adminUser *User) {
	var kubeConfigContents []string
	var kubeConfigNames []string

	configs, err := os.ReadDir("./kubeconfig")
	if err != nil {
		sc.Logger.Error("[k8s模块] 未找到 ./kubeconfig 目录，停止注入 Mock 数据", zap.Error(err))
		return
	}

	for _, info := range configs {
		if info.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join("./kubeconfig", info.Name()))
		if err != nil {
			sc.Logger.Error("[k8s模块] 读取 kubeconfig 配置文件失败", zap.String("file", info.Name()), zap.Error(err))
			continue
		}
		kubeConfigContents = append(kubeConfigContents, string(content))
		kubeConfigNames = append(kubeConfigNames, info.Name())
	}

	if len(kubeConfigContents) == 0 {
		sc.Logger.Error("[k8s模块] ./kubeconfig 目录下没有找到任何有效的配置文件，停止注入 Mock 数据")
		return
	}

	num := 4
	for i := 0; i < num; i++ {
		mIndex := i
		if mIndex >= len(common.RUN_ENV_TYPE_ARRAY) {
			mIndex = len(common.RUN_ENV_TYPE_ARRAY) - 1
		}
		env := common.RUN_ENV_TYPE_ARRAY[mIndex]

		configContent := kubeConfigContents[i%len(kubeConfigContents)]

		baseName := kubeConfigNames[i%len(kubeConfigNames)]
		tmp := K8sCluster{
			Name:                 fmt.Sprintf("%s-%s", baseName, env),
			NameZh:               fmt.Sprintf("集群-%s", env),
			UserID:               1,
			Env:                  env,
			KubeConfigContent:    configContent,
			ActionTimeoutSeconds: i + 1,
		}
		_ = tmp.FillDefaultData()
		_ = tmp.CreateOne()
	}

	// mock节点
	num = 5
	clusters, _ := GetK8sClusterAll()
	for _, cluster := range clusters {
		cluster := cluster

		// 每个集群仅解析生成一次 Clientset
		_, kClientSet, _, err := common.GenK8sClientSetByKubeconfigContent(cluster.KubeConfigContent, cluster.ActionTimeoutSeconds)
		if err != nil || kClientSet == nil {
			continue
		}

		var nodes []corev1.Node
		for i := 0; i < num; i++ {
			nodeName := fmt.Sprintf("%s-node-%d", strings.ReplaceAll(cluster.Name, "_", "-"), i+1)
			node := corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
					Labels: map[string]string{
						"kubernetes.io/hostname":         nodeName,
						"kubernetes.io/os":               "linux",
						"kubernetes.io/arch":             "amd64",
						"node-role.kubernetes.io/worker": "",
					},
				},
				Spec: corev1.NodeSpec{
					Unschedulable: false, // 允许调度
				},
				Status: corev1.NodeStatus{
					Phase: corev1.NodeRunning,
					// 设置节点容量资源 (4核 8G)
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
						corev1.ResourcePods:   resource.MustParse("110"),
					},
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
						corev1.ResourcePods:   resource.MustParse("110"),
					},
					// 设置节点健康状态 (Ready)
					Conditions: []corev1.NodeCondition{
						{
							Type:               corev1.NodeReady,
							Status:             corev1.ConditionTrue,
							LastHeartbeatTime:  metav1.Now(),
							LastTransitionTime: metav1.Now(),
							Reason:             "KubeletReady",
							Message:            "kubelet is posting ready status",
						},
					},
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion:          cluster.Version,
						OSImage:                 "Ubuntu 22.04 LTS",
						KernelVersion:           "5.15.0-100-generic",
						ContainerRuntimeVersion: "containerd://1.6.20",
					},
				},
			}

			// 使用匿名函数隔离 defer cancel1() 作用域，避免在 for 循环中堆积
			func() {
				ctx1, cancel1 := common.GenTimeoutContext(cluster.ActionTimeoutSeconds)
				defer cancel1()
				_, _ = kClientSet.CoreV1().Nodes().Create(ctx1, &node, metav1.CreateOptions{})
			}()
			nodes = append(nodes, node)
		}
	}

	sc.Logger.Info("k8s集群模块 Mock 数据注入成功")
}

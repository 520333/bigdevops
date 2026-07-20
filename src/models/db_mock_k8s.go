package models

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
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

	num := 1
	for i := 0; i < num; i++ {
		mIndex := i
		if mIndex >= len(common.RUN_ENV_TYPE_ARRAY) {
			mIndex = len(common.RUN_ENV_TYPE_ARRAY) - 1
		}
		env := common.RUN_ENV_TYPE_ARRAY[mIndex]

		configContent := kubeConfigContents[i%len(kubeConfigContents)]

		tmp := K8sCluster{
			Name:                 kubeConfigNames[mIndex],
			NameZh:               fmt.Sprintf("集群-%s", env),
			UserID:               1,
			Env:                  env,
			KubeConfigContent:    configContent,
			ActionTimeoutSeconds: i + 1,
		}
		_ = tmp.CreateOne()
	}

	sc.Logger.Info("k8s集群模块 Mock 数据注入成功")
}

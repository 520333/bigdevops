package cache

import (
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"sync"
	"time"

	"github.com/bndr/gojenkins"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

type JenkinsCache struct {
	sync.RWMutex
	JenkinsClientMap   map[uint]*gojenkins.Jenkins
	JenkinsProbeErrMsg map[uint]string
	Sc                 *config.ServerConfig
}

func NewJenkinsCache(sc *config.ServerConfig) *JenkinsCache {
	return &JenkinsCache{
		JenkinsClientMap:   make(map[uint]*gojenkins.Jenkins),
		JenkinsProbeErrMsg: make(map[uint]string),
		Sc:                 sc,
	}
}

func (jc *JenkinsCache) JenkinsCacheManager(ctx context.Context) error {
	interval := 30 * time.Second
	if jc.Sc.K8sClusterC != nil && jc.Sc.K8sClusterC.RunIntervalSeconds > 0 {
		interval = time.Duration(jc.Sc.K8sClusterC.RunIntervalSeconds) * time.Second
	}
	go wait.UntilWithContext(ctx, jc.ReNewJenkinsClientsMap, interval)
	<-ctx.Done()
	jc.Sc.Logger.Info("JenkinsCache 收到退出信号")
	return nil
}

func (jc *JenkinsCache) ReNewJenkinsClientsMap(ctx context.Context) {
	servers, err := models.GetJenkinsInstanceAll()
	if err != nil {
		jc.Sc.Logger.Error("[Jenkins模块] 扫描数据库中的 Jenkins 实例配置失败", zap.Error(err))
		return
	}
	if len(servers) == 0 {
		return
	}

	clientMap := make(map[uint]*gojenkins.Jenkins)
	errMsgMap := make(map[uint]string)

	for _, s := range servers {
		s := s
		client := gojenkins.CreateJenkins(nil, s.URL, s.Username, s.ApiToken)
		_, err := client.Init(ctx)
		if err != nil {
			jc.Sc.Logger.Error("[Jenkins模块] 连接/探活 Jenkins 实例失败",
				zap.Error(err),
				zap.Uint("ID", s.ID),
				zap.String("Name", s.Name),
			)
			errMsgMap[s.ID] = "连接失败: " + err.Error()
			continue
		}

		clientMap[s.ID] = client
		errMsgMap[s.ID] = ""
	}

	jc.Lock()
	jc.JenkinsClientMap = clientMap
	jc.JenkinsProbeErrMsg = errMsgMap
	jc.Unlock()
}

func (jc *JenkinsCache) GetJenkinsClientById(id uint) *gojenkins.Jenkins {
	jc.RLock()
	defer jc.RUnlock()
	return jc.JenkinsClientMap[id]
}

func (jc *JenkinsCache) GetJenkinsProbeResultById(id uint) bool {
	jc.RLock()
	defer jc.RUnlock()
	errMsg, ok := jc.JenkinsProbeErrMsg[id]
	return ok && errMsg == ""
}

func (jc *JenkinsCache) GetJenkinsProbeErrMsgById(id uint) string {
	jc.RLock()
	defer jc.RUnlock()
	return jc.JenkinsProbeErrMsg[id]
}

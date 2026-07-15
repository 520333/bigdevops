package cache

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	pcc "github.com/prometheus/common/config"
	pmodel "github.com/prometheus/common/model"
	ppc "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/http"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	hashTmpKey = "__tmp_hash"
)

type MonitorCache struct {
	PrometheusMainConfigMap   map[string]string
	AlertManagerMainConfigMap map[string]string
	AlertRuleMap              map[string]string
	RecordRuleMap             map[string]string
	Sc                        *config.ServerConfig
	sync.RWMutex
	MainLock   sync.RWMutex
	RecordLock sync.RWMutex
	AlertLock  sync.RWMutex

	AlertManagerLock sync.RWMutex
}

func NewMonitorCache(sc *config.ServerConfig) *MonitorCache {
	mc := &MonitorCache{
		PrometheusMainConfigMap: map[string]string{},
		AlertRuleMap:            map[string]string{},
		RecordRuleMap:           map[string]string{},
		Sc:                      sc,
		RWMutex:                 sync.RWMutex{},
		MainLock:                sync.RWMutex{},
		RecordLock:              sync.RWMutex{},
		AlertLock:               sync.RWMutex{},
	}
	return mc
}

func (mc *MonitorCache) MonitorCacheManager(ctx context.Context) error {
	go wait.UntilWithContext(ctx, mc.GeneratePrometheusMainConfigYaml, time.Duration(mc.Sc.MonitorComputeC.RunIntervalSeconds)*time.Second)
	go wait.UntilWithContext(ctx, mc.GeneratePrometheusAlertRuleConfigYaml, time.Duration(mc.Sc.MonitorComputeC.RunIntervalSeconds)*time.Second)
	go wait.UntilWithContext(ctx, mc.GeneratePrometheusRecordRuleConfigYaml, time.Duration(mc.Sc.MonitorComputeC.RunIntervalSeconds)*time.Second)
	go wait.UntilWithContext(ctx, mc.GenerateAlertManagerMainConfigYaml, time.Duration(mc.Sc.MonitorComputeC.RunIntervalSeconds)*time.Second)
	<-ctx.Done()
	mc.Sc.Logger.Info("SyncCache 收到其他任务退出信号")
	return nil
}

func (mc *MonitorCache) GetPrometheusMainConfigYamlByIp(ip string) string {
	mc.MainLock.RLock()
	defer mc.MainLock.RUnlock()
	return mc.PrometheusMainConfigMap[ip]
}

// GeneratePrometheusMainConfigYaml 生成主配置文件
func (mc *MonitorCache) GeneratePrometheusMainConfigYaml(ctx context.Context) {
	pools, err := models.GetMonitorPromScrapePoolAll()
	if err != nil {
		mc.Sc.Logger.Error("[监控模块]扫描数据库中的采集池失败", zap.Error(err))
		return
	}
	if len(pools) == 0 {
		return
	}
	mainConfigMap := map[string]string{}
	for _, pool := range pools {
		pool := pool
		// 1. 生成基础配置 (global, remote_write)
		allConfig := mc.GeneratePrometheusMainConfigYamlOnePool(pool)

		// 2. 生成并附加采集段 (scrape_configs) - 必须在 Marshal 之前！
		baseScrapeConfigs := mc.GeneratePrometheusScrapeConfigYamlOnePool(pool)
		if baseScrapeConfigs == nil {
			continue
		}
		//scrapeConfigs := mc.GeneratePrometheusScrapeConfigYamlOnePool(pool)
		//if scrapeConfigs == nil {
		//	continue
		//}

		ipNum := len(pool.PrometheusInstances)

		for index, ip := range pool.PrometheusInstances {
			ip := ip
			// 根据数量做hashmod的判断 大于0需要分片
			//if ipNum > 0 {
			//	scrapeConfigs = mc.HashModScrapeConfig(scrapeConfigs, ipNum, index)
			//}
			//allConfig.ScrapeConfigs = scrapeConfigs
			//// 3. 所有内容组装完毕，执行序列化转为 YAML
			//out, err := yaml.Marshal(allConfig)
			//if err != nil {
			//	mc.Sc.Logger.Error("[监控模块]根据采集池配置生成prometheus主配置文件错误", zap.Error(err), zap.String("采集池", pool.Name))
			//	continue
			//}
			var currentScrapeConfigs []*ppc.ScrapeConfig
			if ipNum > 1 {
				currentScrapeConfigs = mc.HashModScrapeConfig(baseScrapeConfigs, ipNum, index)
			} else {
				currentScrapeConfigs = baseScrapeConfigs
			}

			allConfig.ScrapeConfigs = currentScrapeConfigs

			// 3. 所有内容组装完毕，执行序列化转为 YAML
			out, err := yaml.Marshal(allConfig)

			// ====删除一些没用的字段====
			var m map[string]any
			_ = yaml.Unmarshal(out, &m)

			if remoteWrites, ok := m["remote_write"].([]any); ok {
				for _, rw := range remoteWrites {
					if rwMap, ok := rw.(map[string]any); ok {
						delete(rwMap, "follow_redirects")
						delete(rwMap, "enable_http2")
					}
				}
			}
			if remoteRead, ok := m["remote_read"].([]any); ok {
				for _, rw := range remoteRead {
					if rwMap, ok := rw.(map[string]any); ok {
						delete(rwMap, "follow_redirects")
						delete(rwMap, "enable_http2")
					}
				}
			}
			if alertingMap, ok := m["alerting"].(map[string]any); ok {

				// 2. alertmanagers 是字典里的一个列表 ([]any)
				if alertmanagers, ok := alertingMap["alertmanagers"].([]any); ok {

					// 3. 遍历这个列表
					for _, am := range alertmanagers {

						// 4. 列表里的每个元素是一个字典 (map[string]any)
						if amMap, ok := am.(map[string]any); ok {
							// 执行删除
							delete(amMap, "api_version")
							delete(amMap, "enable_http2")
							delete(amMap, "follow_redirects")
						}
					}
				}
			}

			if scrapeConfigs, ok := m["scrape_configs"].([]any); ok {
				for _, rw := range scrapeConfigs {
					if scMap, ok := rw.(map[string]any); ok {
						// 1. 清理 http_sd_configs
						if httpSds, ok := scMap["http_sd_configs"].([]any); ok {
							for _, hsd := range httpSds {
								if hsdMap, ok := hsd.(map[string]any); ok {
									delete(hsdMap, "enable_http2")
									delete(hsdMap, "follow_redirects")
								}
							}
						}
						delete(scMap, "enable_compression")
						delete(scMap, "enable_http2")
						delete(scMap, "follow_redirects")
						delete(scMap, "honor_timestamps")
						delete(scMap, "track_timestamps_staleness")
					}
				}
			}

			out, _ = yaml.Marshal(m)

			// ======================

			if err != nil {
				mc.Sc.Logger.Error("[监控模块]根据采集池配置生成prometheus主配置文件错误", zap.Error(err), zap.String("采集池", pool.Name))
				continue
			}
			outStr := string(out)
			// 重新查一遍当前 pool 下的任务（因为 allConfig.ScrapeConfigs 里的顺序和查询出的顺序一致）
			scrapeJobs, _ := models.GetMonitorPromScrapeJobByPoolId(pool.ID)
			for _, job := range scrapeJobs {
				if job.BearerToken != "" {
					// Kubernetes SD 配置中通常有两处 token (外部 HTTPClientConfig 和 SD 内部的 HTTPClientConfig)
					replaceCount := 1
					if job.ServiceDiscoveryType == common.MONITOR_SCRAPE_JOB_SD_TYPE_K8S {
						replaceCount = 2
					}

					// 将被安全屏蔽的 <secret> 按顺序替换回真实的 Token，并加上双引号防止特殊字符导致 YAML 解析报错
					outStr = strings.Replace(outStr, "bearer_token: <secret>", fmt.Sprintf(`bearer_token: "%s"`, job.BearerToken), replaceCount)
				}
			}
			mc.Sc.Logger.Info("[监控模块]根据采集池配置生成prometheus主配置文件成功", zap.String("采集池", pool.Name), zap.Any("ip", ip))

			//fileName := fmt.Sprintf("pool_%v.yml", pool.Name)
			//_ = os.WriteFile(fileName, out, 0666)
			//mainConfigMap[ip] = string(out)
			mainConfigMap[ip] = outStr
		}

	}

	mc.MainLock.Lock()
	mc.PrometheusMainConfigMap = mainConfigMap
	mc.MainLock.Unlock()

}

func GenPromModeDuration(ts int) pmodel.Duration {
	return pmodel.Duration(time.Duration(ts) * time.Second)

}

func (mc *MonitorCache) HashModScrapeConfig(scrapeConfigs []*ppc.ScrapeConfig, modNul, index int) []*ppc.ScrapeConfig {
	res := []*ppc.ScrapeConfig{}
	for _, scrapeConfig := range scrapeConfigs {
		// 【最小改动点】：必须解引用创建一个新结构体，防止循环污染原始数据
		newScrapeConfig := *scrapeConfig

		hasModes := []*relabel.Config{
			{
				SourceLabels: pmodel.LabelNames{pmodel.AddressLabel},
				Regex:        relabel.MustNewRegexp("(.*)"),
				Modulus:      uint64(modNul),
				TargetLabel:  hashTmpKey,
				Replacement:  "$1",
				Action:       relabel.HashMod,
			},
			{
				SourceLabels: pmodel.LabelNames{hashTmpKey},
				Regex:        relabel.MustNewRegexp(fmt.Sprintf("^%d$", index)),
				Replacement:  "$1",
				Action:       relabel.Keep,
			},
		}

		// 【最小改动点】：分配新切片内存，防止底层数组互相覆盖
		newRelabels := make([]*relabel.Config, 0, len(newScrapeConfig.RelabelConfigs)+len(hasModes))
		newRelabels = append(newRelabels, newScrapeConfig.RelabelConfigs...)
		newRelabels = append(newRelabels, hasModes...)

		newScrapeConfig.RelabelConfigs = newRelabels
		res = append(res, &newScrapeConfig)
	}
	return res
}

func (mc *MonitorCache) GeneratePrometheusMainConfigYamlOnePool(pool *models.MonitorPromScrapePool) ppc.Config {
	// 拼接主global
	gc := ppc.GlobalConfig{
		ScrapeInterval: GenPromModeDuration(pool.ScrapeInterval),
		ScrapeTimeout:  GenPromModeDuration(pool.ScrapeTimeout),
	}
	// 拼接externalLabels
	externalLabels := []string{}
	for _, i := range externalLabels {
		kvs := strings.Split(i, "=")
		if len(kvs) != 2 {
			continue
		}
		k := kvs[0]
		v := kvs[1]
		externalLabels = append(externalLabels, k, v)
	}
	if len(externalLabels) > 0 && len(externalLabels)%2 == 0 {
		gc.ExternalLabels = labels.FromStrings(externalLabels...)
	}
	// 拼接remote_write
	remoteWriteC := &ppc.RemoteWriteConfig{
		URL:           mustParseURL(pool.RemoteWriteUrl),
		RemoteTimeout: GenPromModeDuration(pool.ScrapeTimeout),
	}
	all := ppc.Config{}
	all.GlobalConfig = gc
	all.RemoteWriteConfigs = []*ppc.RemoteWriteConfig{remoteWriteC}

	// 拼接rule_files
	all.RuleFiles = []string{pool.RuleFilePath}

	// 判断 如果是支持alert池子 需要添加alert段
	switch pool.SupperAlert {
	case common.GORM_ENABLE_RES_YES:
		remoteReadC := &ppc.RemoteReadConfig{
			URL:           mustParseURL(pool.RemoteReadUrl),
			RemoteTimeout: GenPromModeDuration(pool.RemoteTimeoutSeconds),
		}
		all.RemoteReadConfigs = []*ppc.RemoteReadConfig{remoteReadC}

		// 拼接alert告警段
		alt := &ppc.AlertmanagerConfig{}
		alt.APIVersion = "v2"
		alt.ServiceDiscoveryConfigs = []discovery.Config{
			discovery.StaticConfig{
				{
					Targets: []pmodel.LabelSet{{
						pmodel.AddressLabel: pmodel.LabelValue(pool.AlertManagerUrl),
					}},
				},
			},
		}

		all.AlertingConfig = ppc.AlertingConfig{
			AlertmanagerConfigs: []*ppc.AlertmanagerConfig{
				alt,
			},
		}
		//all.RuleFiles = append(all.RuleFiles, pool.RuleFilePath)

	}
	switch pool.SupperRecord {
	case common.GORM_ENABLE_RES_YES:

		all.RuleFiles = append(all.RuleFiles, pool.RecordFilePath)
	}
	return all
}

func (mc *MonitorCache) GeneratePrometheusScrapeConfigYamlOnePool(pool *models.MonitorPromScrapePool) []*ppc.ScrapeConfig {
	var scrapeConfigs []*ppc.ScrapeConfig
	scrapeJobs, err := models.GetMonitorPromScrapeJobByPoolId(pool.ID)
	if err != nil {
		mc.Sc.Logger.Error("[监控模块]根据采集池poolId查找所有采集任务错误", zap.Error(err), zap.String("池子", pool.Name))
		return nil
	}

	for _, scrapeJob := range scrapeJobs {
		scrapeJob := scrapeJob

		// 【核心修复 1】：使用官方默认模板初始化，防止 Go 的零值(false)被意外序列化！
		defaultJob := ppc.DefaultScrapeConfig
		oneJob := &defaultJob

		if scrapeJob.RelabelConfigsYamlString != "" {
			var relabelConfigObj []*relabel.Config
			err := yaml.Unmarshal([]byte(scrapeJob.RelabelConfigsYamlString), &relabelConfigObj)
			if err != nil {
				mc.Sc.Logger.Error("[监控模块]解析Relabel YAML字符串失败", zap.Error(err), zap.String("job", scrapeJob.Name))
			} else if relabelConfigObj != nil {
				oneJob.RelabelConfigs = relabelConfigObj
			}
		}

		switch scrapeJob.ServiceDiscoveryType {
		case common.MONITOR_SCRAPE_JOB_SD_TYPE_HTTP:
			oneJob.JobName = scrapeJob.Name
			oneJob.ScrapeInterval = GenPromModeDuration(scrapeJob.ScrapeInterval)
			oneJob.ScrapeTimeout = GenPromModeDuration(scrapeJob.ScrapeTimeout)
			oneJob.Scheme = scrapeJob.Scheme
			oneJob.MetricsPath = scrapeJob.MetricsPath

			sdUrl := fmt.Sprintf("%s?port=%v&leafNodeIds=%v", mc.Sc.MonitorComputeC.HttpSdApi, scrapeJob.Port, strings.Join(scrapeJob.TreeNodeIds, ","))

			// HTTP SD 也赋予官方默认的 HTTP 客户端配置
			httpSdConfig := &http.SDConfig{
				URL:              sdUrl,
				RefreshInterval:  GenPromModeDuration(scrapeJob.RefreshInterval),
				HTTPClientConfig: pcc.DefaultHTTPClientConfig,
			}
			oneJob.ServiceDiscoveryConfigs = discovery.Configs{httpSdConfig}
			scrapeConfigs = append(scrapeConfigs, oneJob)

		case common.MONITOR_SCRAPE_JOB_SD_TYPE_K8S:
			oneJob.JobName = scrapeJob.Name
			oneJob.Scheme = scrapeJob.Scheme
			oneJob.MetricsPath = scrapeJob.MetricsPath

			// 【核心修复 2】：Job 级别的 HTTP 客户端也要基于官方 Default 初始化
			oneJob.HTTPClientConfig = pcc.DefaultHTTPClientConfig
			oneJob.HTTPClientConfig.TLSConfig = pcc.TLSConfig{
				InsecureSkipVerify: true,
				CAFile:             scrapeJob.TlsCaFilePath,
			}
			if scrapeJob.BearerTokenFile != "" {
				oneJob.HTTPClientConfig.BearerTokenFile = scrapeJob.BearerTokenFile
			} else if scrapeJob.BearerToken != "" {
				oneJob.HTTPClientConfig.BearerToken = pcc.Secret(scrapeJob.BearerToken)
			}

			// 【核心修复 3】：K8s SD 必须赋予 DefaultHTTPClientConfig，否则会被判定为 Custom Client 从而报错
			k8sSdConfig := &kubernetes.SDConfig{
				Role:             kubernetes.Role(scrapeJob.KubernetesSdRole),
				HTTPClientConfig: pcc.DefaultHTTPClientConfig,
			}

			if scrapeJob.KubeConfigFilePath != "" {
				// 认证方式 A: 使用本地 Kubeconfig 证书
				k8sSdConfig.KubeConfig = scrapeJob.KubeConfigFilePath
				// 注意：这里绝对不能再对 k8sSdConfig.HTTPClientConfig 做任何赋值操作！
			} else if scrapeJob.APIServer != "" {
				// 认证方式 B: 使用 APIServer + Token
				parsedUrl, err := url.Parse(scrapeJob.APIServer)
				if err == nil {
					k8sSdConfig.APIServer = pcc.URL{URL: parsedUrl}
				}

				// 只有在使用 APIServer 方式时，才允许配置 Custom TLS/Token
				k8sSdConfig.HTTPClientConfig.TLSConfig = pcc.TLSConfig{
					CA:                 scrapeJob.TlsCaContent,
					CAFile:             scrapeJob.TlsCaFilePath,
					InsecureSkipVerify: true,
				}
				if scrapeJob.BearerTokenFile != "" {
					k8sSdConfig.HTTPClientConfig.BearerTokenFile = scrapeJob.BearerTokenFile
				} else if scrapeJob.BearerToken != "" {
					k8sSdConfig.HTTPClientConfig.BearerToken = pcc.Secret(scrapeJob.BearerToken)
				}
			}

			oneJob.ServiceDiscoveryConfigs = discovery.Configs{k8sSdConfig}
			scrapeConfigs = append(scrapeConfigs, oneJob)
		}
	}
	return scrapeConfigs
}

func mustParseURL(u string) *pcc.URL {
	parsed, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	return &pcc.URL{URL: parsed}
}

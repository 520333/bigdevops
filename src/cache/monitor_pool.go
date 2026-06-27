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
	PrometheusMainConfigMap map[string]string
	AlertRuleMap            map[string]string
	RecordRuleMap           map[string]string
	Sc                      *config.ServerConfig
	sync.RWMutex
}

func NewMonitorCache(sc *config.ServerConfig) *MonitorCache {
	mc := &MonitorCache{
		PrometheusMainConfigMap: map[string]string{},
		AlertRuleMap:            map[string]string{},
		RecordRuleMap:           map[string]string{},
		Sc:                      sc,
		RWMutex:                 sync.RWMutex{},
	}
	return mc
}

func (mc *MonitorCache) MonitorCacheManager(ctx context.Context) error {
	go wait.UntilWithContext(ctx, mc.GeneratePrometheusMainConfigYaml, time.Duration(mc.Sc.MonitorComputeC.RunIntervalSeconds)*time.Second)
	<-ctx.Done()
	mc.Sc.Logger.Info("SyncCache 收到其他任务退出信号")
	return nil
}

func (mc *MonitorCache) GetPrometheusMainConfigYamlByIp(ip string) string {
	mc.RLock()
	defer mc.RUnlock()
	return mc.PrometheusMainConfigMap[ip]
}

// GeneratePrometheusMainConfigYaml 生成主配置文件
func (mc *MonitorCache) GeneratePrometheusMainConfigYaml(ctx context.Context) {
	pools, err := models.GetMonitorScrapePoolAll()
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
		scrapeConfigs := mc.GeneratePrometheusScrapeConfigYamlOnePool(pool)
		if scrapeConfigs == nil {
			continue
		}
		//allConfig.ScrapeConfigs = scrapeConfigs

		ipNum := len(pool.PrometheusInstances)

		for index, ip := range pool.PrometheusInstances {
			ip := ip
			// 根据数量做hashmod的判断 大于0需要分片
			if ipNum > 0 {
				scrapeConfigs = mc.HashModScrapeConfig(scrapeConfigs, ipNum, index)
			}
			allConfig.ScrapeConfigs = scrapeConfigs
			// 3. 所有内容组装完毕，执行序列化转为 YAML
			out, err := yaml.Marshal(allConfig)
			if err != nil {
				mc.Sc.Logger.Error("[监控模块]根据采集池配置生成prometheus主配置文件错误", zap.Error(err), zap.String("采集池", pool.Name))
				continue
			}

			outStr := string(out)
			// 重新查一遍当前 pool 下的任务（因为 allConfig.ScrapeConfigs 里的顺序和查询出的顺序一致）
			scrapeJobs, _ := models.GetMonitorScrapeJobByPoolId(pool.ID)
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

			mc.Sc.Logger.Info("[监控模块]根据采集池配置生成prometheus主配置文件成功", zap.String("采集池", pool.Name), zap.Any("配置", outStr))
			//fileName := fmt.Sprintf("pool_%v.yml", pool.Name)
			//_ = os.WriteFile(fileName, out, 0666)
			//mainConfigMap[ip] = string(out)
			mainConfigMap[ip] = outStr
		}
	}

	mc.Lock()
	mc.PrometheusMainConfigMap = mainConfigMap
	mc.Unlock()

}

func GenPromModeDuration(ts int) pmodel.Duration {
	return pmodel.Duration(time.Duration(ts) * time.Second)

}

// HashModScrapeConfig 给单独一个ip的scrape数组添加relabelConfigs
func (mc *MonitorCache) HashModScrapeConfig(scrapeConfigs []*ppc.ScrapeConfig, modNul, index int) []*ppc.ScrapeConfig {
	res := []*ppc.ScrapeConfig{}
	for _, scrapeConfig := range scrapeConfigs {
		scrapeConfig := scrapeConfig
		scrapeConfig.RelabelConfigs = []*relabel.Config{
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
		res = append(res, scrapeConfig)
	}
	return res

}

func (mc *MonitorCache) GeneratePrometheusMainConfigYamlOnePool(pool *models.MonitorScrapePool) ppc.Config {
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

	return all
}

// GeneratePrometheusScrapeConfigYamlOnePool 生成采集段
func (mc *MonitorCache) GeneratePrometheusScrapeConfigYamlOnePool(pool *models.MonitorScrapePool) []*ppc.ScrapeConfig {
	var scrapeConfigs []*ppc.ScrapeConfig
	scrapeJobs, err := models.GetMonitorScrapeJobByPoolId(pool.ID)
	if err != nil {
		mc.Sc.Logger.Error("[监控模块]根据采集池poolId查找所有采集任务错误", zap.Error(err), zap.String("池子", pool.Name))
		return nil
	}

	for _, scrapeJob := range scrapeJobs {
		scrapeJob := scrapeJob
		oneJob := &ppc.ScrapeConfig{}

		switch scrapeJob.ServiceDiscoveryType {
		case common.MONITOR_SCRAPE_JOB_SD_TYPE_HTTP:
			oneJob.JobName = scrapeJob.Name
			oneJob.ScrapeInterval = GenPromModeDuration(scrapeJob.ScrapeInterval)
			oneJob.ScrapeTimeout = GenPromModeDuration(scrapeJob.ScrapeTimeout)
			oneJob.Scheme = scrapeJob.Scheme
			oneJob.MetricsPath = scrapeJob.MetricsPath
			sdUrl := fmt.Sprintf("%s?port=%v&leafNodeIds=%v", mc.Sc.MonitorComputeC.HttpSdApi, scrapeJob.Port, strings.Join(scrapeJob.TreeNodeIds, ","))
			oneJob.ServiceDiscoveryConfigs = discovery.Configs{
				&http.SDConfig{
					URL:             sdUrl,
					RefreshInterval: GenPromModeDuration(scrapeJob.RefreshInterval),
				},
			}
			scrapeConfigs = append(scrapeConfigs, oneJob)

		case common.MONITOR_SCRAPE_JOB_SD_TYPE_K8S:
			oneJob.JobName = scrapeJob.Name
			oneJob.Scheme = scrapeJob.Scheme
			oneJob.MetricsPath = scrapeJob.MetricsPath
			//oneJob.HTTPClientConfig.BearerTokenFile = scrapeJob.BearerTokenFile
			oneJob.HTTPClientConfig.TLSConfig = pcc.TLSConfig{
				InsecureSkipVerify: true,
			}
			// ================= 新增：写入 K8s 专属的 Relabel 规则 =================
			oneJob.RelabelConfigs = []*relabel.Config{
				{
					Action:       relabel.Replace,
					SourceLabels: pmodel.LabelNames{"__meta_kubernetes_node_label_kubernetes_io_hostname"},
					Regex:        relabel.MustNewRegexp("(.+)"),
					TargetLabel:  "node",
				},
				{
					Action:      relabel.LabelMap,
					Regex:       relabel.MustNewRegexp("__meta_kubernetes_node_label_(.+)"),
					Replacement: "$1",
				},
				{
					Action:      relabel.Replace,
					Regex:       relabel.MustNewRegexp("(.*)"),
					TargetLabel: "__metrics_path__",
					Replacement: "/metrics/cadvisor", // 注意：这会硬性覆盖上面 oneJob.MetricsPath 的值
				},
			}
			// ==================================================================
			oneJob.HTTPClientConfig.BearerToken = pcc.Secret(scrapeJob.BearerToken)
			oneJob.HTTPClientConfig.BearerTokenFile = scrapeJob.BearerTokenFile
			oneJob.ServiceDiscoveryConfigs = discovery.Configs{
				&kubernetes.SDConfig{
					APIServer:  *mustParseURL(scrapeJob.APIServer),
					Role:       kubernetes.Role(scrapeJob.KubernetesSdRole),
					KubeConfig: scrapeJob.KubeConfigFilePath,
					HTTPClientConfig: pcc.HTTPClientConfig{
						BearerToken: pcc.Secret(scrapeJob.BearerToken),
						//BearerTokenFile: scrapeJob.BearerTokenFile,
						TLSConfig: pcc.TLSConfig{
							CA:                 scrapeJob.TlsCaContent,
							CAFile:             scrapeJob.TlsCaFilePath,
							InsecureSkipVerify: true,
						},
					},
				},
			}
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

package models

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"fmt"
	"math/rand"
)

var (
	caContext = `-----BEGIN CERTIFICATE-----
MIIDBTCCAe2gAwIBAgIIaRoSLz27SfIwDQYJKoZIhvcNAQELBQAwFTETMBEGA1UE
AxMKa3ViZXJuZXRlczAeFw0yNjA1MjgwODAzNDdaFw0zNjA1MjUwODA4NDdaMBUx
EzARBgNVBAMTCmt1YmVybmV0ZXMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
AoIBAQDECKJ8V7j9btyCVOB1y/GF/+fuJEWnpjZ8Hy0Ymc92/af2aIFA7Vjo956J
KsyMLwEJGaUtv8HOs72DGZfElnEKac8JXIhVWsvphHxeAmbBf/wrEP6x5Bao2gwf
VeDITqB4N2MoK8/W+p2g+LxWPcmR/sKoWK7hCi1ImzbYXggLb/cTM7iD+aeAnCqF
D0V9PB0/f+q7/n3Egw89ougYAS1YMkGiHRiat4IeVJ7rKL1TYKxK4yZO/NsYDsd5
wTSUuCE+Fmh6HCMO3A63ftohIPfXpMdhE4w4W3hVAs7itloQkGbkWpuS85Vg3+1n
91McWrr8mOZko1H8YVrpndlibnJrAgMBAAGjWTBXMA4GA1UdDwEB/wQEAwICpDAP
BgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSbtEMoHX1cv0IsqU80R40Den+hjzAV
BgNVHREEDjAMggprdWJlcm5ldGVzMA0GCSqGSIb3DQEBCwUAA4IBAQAIKddVUZXf
pYoeKyDS1EmJWOHY5On/lw1hMJlkvGcKq1CXXSnve6Ne+O1prw3V8r3aKMjjwcTz
Ml5jjUrCkfaM0I/38A3EOMA7LU7PeEU+MER4EAkhKULwTvp5qB22vFd/SEMkvELS
Swyv+Y+oMuxS36Z0HLjmMFU45o6LDUYelhmQRDWoHoDdeD9Ol8HkRLuUB0t5KY3/
NjSAcXGDMjljXqmBpDpfM0xaetzt4knhekk1LUuKCwtv6vdGb1lgSSlcz7PDB9bo
Hd0zU25yukcmCk4Po9Mdzm+Bs70LuXRwrQI1FWOPmz5MlNweqtxy+lWMWGpX+lYf
PeO73tYJhHTq
-----END CERTIFICATE-----`
	tokenContext        = "eyJhbGciOiJSUzI1NiIsImtpZCI6IjdFV09MMTZnRnlxVlBkczIzTmVMTU5yMG1Wa29Rb3FNYTdNVE5TQXZYN0UifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJrdWJlLXN5c3RlbSIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VjcmV0Lm5hbWUiOiJwcm9tZXRoZXVzIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQubmFtZSI6InByb21ldGhldXMiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiJhMDNkZTFhZS04NTBjLTRjMzUtOWIzNC00MzI4YjY5MzU2OWYiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6a3ViZS1zeXN0ZW06cHJvbWV0aGV1cyJ9.fvm5rJ85Zl__E2kVhNe4fjdDRrO0ibWAD0UpKRooS2GZjy45fcJiURC5jxw5n_46zdqxeoEZhfn3tkV3rb0uYe7xrT6TqC8Jg-I146KTGk6PzI5lMXAKKnbQlcPXlOHX9xDRQCudZSiTL8c13ekUM4IvRDq6qugllvcDL7c35m3P0lQw3ndqhrDoFSWnVkHISbut-d1LeJlYgh6xsS2sl343T2wpwlXR7LRnPvmO1tGZLl6ry-6Q5JHwE2bMNCQaRBmrW1ZvhouNoKOG2H1ntwSD--ibCN7L2Z2gXKHnUL4treMdA_cNoe2zZX9uenZ0eOzaG30GirLSBNRwQl3qBg"
	k8sCadVisorRelabels = `
- action: replace
  regex: (.+)
  source_labels:
    - __meta_kubernetes_node_label_kubernetes_io_hostname
  target_label: node
- separator:
  regex: __meta_kubernetes_node_label_(.+)
  replacement: $1
  action: labelmap
- separator:
  regex: (.*)
  target_label: __metrics_path__
  replacement: /metrics/cadvisor
  action: replace
`
)

func mockMonitorData(sc *config.ServerConfig, adminUser *User) {
	// prometheus 实例池
	randTagKeys := []string{"dev", "test", "pre", "prod"}
	randTagValues := []string{"project1", "project2", "project3", "project4", "project5"}
	num := 1
	for i := 0; i < num; i++ {
		tags := []string{}
		for _, tagKey := range randTagKeys {
			taValue := randTagValues[rand.Intn(len(randTagValues))]
			onTag := fmt.Sprintf("%s=%s", tagKey, taValue)
			tags = append(tags, onTag)
		}
		p := MonitorScrapePool{
			Name:                 fmt.Sprintf("pool-%v", i+1),
			PrometheusInstances:  []string{"192.168.50.200", "192.168.50.201"},
			ScrapeInterval:       15,
			ScrapeTimeout:        10,
			ExternalLabels:       tags,
			RemoteWriteUrl:       fmt.Sprintf("http://192.168.50.%v:8428/api/v1/write", 200),
			RemoteTimeoutSeconds: 5,
			UserID:               1,
			SupperAlert:          common.GORM_ENABLE_RES_YES,
			RemoteReadUrl:        "http://192.168.50.200:8428/api/v1/read",
			AlertManagerUrl:      "192.168.50.200:9093",
			RuleFilePath:         "/opt/app/prometheus/rule.yml",
		}
		_ = p.CreateOne()
	}
	sc.Logger.Info("监控采集池数据 Mock 数据注入成功")

	// prometheus 采集任务
	treeNodes, _ := GetStreeNodeAllLeaf()
	treeNodeIds := []string{}
	for _, treeNode := range treeNodes {
		treeNode := treeNode
		treeNodeIds = append(treeNodeIds, fmt.Sprintf("%d", treeNode.ID))
	}

	exporterNames := []string{"node", "redis", "mysql"}

	num = 3
	for i := 0; i < num; i++ {
		j := i
		if j >= len(treeNodeIds) {
			j = len(treeNodeIds) - 1
		}
		k := i % len(exporterNames)
		http := MonitorScrapeJob{
			Name:                 fmt.Sprintf("%v_exporter_%v", exporterNames[k], i+1),
			UserID:               1,
			Enable:               1,
			ServiceDiscoveryType: common.MONITOR_SCRAPE_JOB_SD_TYPE_HTTP,
			MetricsPath:          "/metrics",
			Scheme:               "http",
			ScrapeInterval:       15,
			ScrapeTimeout:        5,
			RefreshInterval:      30,
			Port:                 9200,
			TreeNodeIds:          []string{treeNodeIds[0]},
			PoolId:               1,
		}
		_ = http.CreateOne()
	}
	// k8s采集job
	for i := 0; i < 1; i++ {
		//k8s := MonitorScrapeJob{
		//	Name:                 fmt.Sprintf("k8s-%v", i+1),
		//	UserID:               1,
		//	ServiceDiscoveryType: common.MONITOR_SCRAPE_JOB_SD_TYPE_K8S,
		//	MetricsPath:          "/metrics",
		//	Scheme:               "https",
		//	ScrapeInterval:       15,
		//	ScrapeTimeout:        5,
		//	RefreshInterval:      30,
		//	Port:                 6443,
		//	PoolId:               1,
		//
		//	APIServer: "https://192.168.50.200:6443",
		//	//KubeConfigFilePath: "/root/.kube/config",
		//	//TlsCaFilePath:      "",
		//	TlsCaContent: caContext,
		//	BearerToken:  tokenContext,
		//	//BearerTokenFile:  "",
		//	KubernetesSdRole: "node",
		//
		//	Key:            "",
		//	CreateUserName: "",
		//}

		// 使用K8S集群SA  token证书方式
		k8s := MonitorScrapeJob{
			Name:                 "k8s-pod-monitor",
			UserID:               1,
			Enable:               1,
			ServiceDiscoveryType: common.MONITOR_SCRAPE_JOB_SD_TYPE_K8S,
			MetricsPath:          "/metrics",
			Scheme:               "https",
			ScrapeInterval:       15,
			ScrapeTimeout:        5,
			RefreshInterval:      30,
			Port:                 6443,
			PoolId:               1,

			KubeConfigFilePath:       "/root/.kube/config",
			BearerTokenFile:          "/opt/app/prometheus/k8s-cluster-token",
			KubernetesSdRole:         "pod",
			RelabelConfigsYamlString: k8sCadVisorRelabels,
		}
		k8s.CreateOne()
	}

	// alertManager 实例池
	num = 1
	ips := []string{"192.168.50.200", "192.168.50.201"}
	for i := 0; i < num; i++ {
		r := MonitorAlertManagerPool{
			Name:                   fmt.Sprintf("online-%v", i+1),
			AlertManagerInstanceId: ips,
			UserID:                 1,
			ResolveTimeout:         "30m",
			GroupWait:              "15s",
			GroupInterval:          "20s",
			RepeatInterval:         "30s",
			Receiver:               "default",
			GroupBy:                []string{"alertname"},
		}
		_ = r.CreateOne()
	}

	// 创建发送组
	for i := 0; i < num; i++ {
		sg := MonitorAlertManagerSendGroup{
			Name:           fmt.Sprintf("sre-%v", i+1),
			NameZh:         fmt.Sprintf("运维组-%v", i+1),
			Enable:         1,
			UserID:         1,
			PoolId:         uint(1),
			ImRobotToken:   "aa",
			RepeatInterval: "30s",
			SendResolved:   1,
		}
		_ = sg.CreateOne()

	}

	// rule规则
	metricsNames := []string{
		`node_memory_Active_bytes`,
		`node_boot_time_seconds`,
		`node_uname_info`,
	}
	num = 3
	for i := 0; i < num; i++ {
		mIndex := i
		if mIndex >= len(metricsNames) {
			mIndex = len(metricsNames) - 1
		}
		rule := MonitorPromAlertRule{
			Name:        fmt.Sprintf("rule-%v", i+1),
			UserID:      1,
			PoolId:      1,
			Enable:      1,
			SendGroupId: 1,
			Expr:        fmt.Sprintf(`%s{job="node_exporter_1"} > 0`, metricsNames[mIndex]),
			ForTime:     "30s",
			Labels:      []string{"l1=v1", "l2=v2"},
			Annotations: []string{"a3=v3", "a4=v4"},
		}
		_ = rule.CreateOne()
	}

	// 值班组
	users, _ := GetUserAll()
	for i := 0; i < num; i++ {
		dutyGroup := &MonitorOndutyGroup{
			Name:    fmt.Sprintf("mock-dutyGroup-%v", i+1),
			UserID:  1,
			Members: users,

			Key:            "",
			PoolName:       "",
			CreateUserName: "",
		}
		_ = dutyGroup.CreateOne()
	}
}

package view_server

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"

	"github.com/gin-gonic/gin"
)

// ==========  prometheus配置 ===========
func fetchPrometheusMainConfigYaml(c *gin.Context) string {
	mc := c.MustGet(common.GIN_CTX_MONITOR_CACHE).(*cache.MonitorCache)
	ip := c.Query("ip")

	// 从缓存中根据ip 获取这个节点的主配置文件
	mainConfigYaml := mc.GetPrometheusMainConfigYamlByIp(ip)
	return mainConfigYaml
}

func downloadPrometheusMainConfigYaml(c *gin.Context) {
	mainConfigYaml := fetchPrometheusMainConfigYaml(c)
	c.String(200, mainConfigYaml)
}

func getMonitorPrometheusYamlOne(c *gin.Context) {
	mainConfigYaml := fetchPrometheusMainConfigYaml(c)
	common.OkWithData(mainConfigYaml, c)
}

// ==========  alertmanager 配置 ===========
func fetchAlertManagerMainConfigYaml(c *gin.Context) string {
	mc := c.MustGet(common.GIN_CTX_MONITOR_CACHE).(*cache.MonitorCache)
	ip := c.Query("ip")

	// 从缓存中根据ip 获取这个节点的主配置文件
	mainConfigYaml := mc.GetAlertManagerMainConfigYamlByIp(ip)
	return mainConfigYaml
}

func downloadAlertManagerMainConfigYaml(c *gin.Context) {
	mainConfigYaml := fetchAlertManagerMainConfigYaml(c)
	c.String(200, mainConfigYaml)
}

func getMonitorAlertManagerYamlOne(c *gin.Context) {
	mainConfigYaml := fetchAlertManagerMainConfigYaml(c)
	common.OkWithData(mainConfigYaml, c)
}

// ==========  rule告警规则 ===========
func fetchPrometheusAlertRuleMainConfigYaml(c *gin.Context) string {
	mc := c.MustGet(common.GIN_CTX_MONITOR_CACHE).(*cache.MonitorCache)
	ip := c.Query("ip")
	mainConfigYaml := mc.GetPrometheusAlertRuleConfigYamlByIp(ip)
	return mainConfigYaml
}

func downloadPrometheusAlertRuleMainConfigYaml(c *gin.Context) {
	mainConfigYaml := fetchPrometheusAlertRuleMainConfigYaml(c)
	c.String(200, mainConfigYaml)
}

func getMonitorPrometheusAlertRuleYamlOne(c *gin.Context) {
	mainConfigYaml := fetchPrometheusAlertRuleMainConfigYaml(c)
	common.OkWithData(mainConfigYaml, c)
}

// ==========  record 预聚合规则 ===========
func fetchPrometheusRecordRuleMainConfigYaml(c *gin.Context) string {
	mc := c.MustGet(common.GIN_CTX_MONITOR_CACHE).(*cache.MonitorCache)
	ip := c.Query("ip")
	mainConfigYaml := mc.GetPrometheusRecordRuleConfigYamlByIp(ip)
	return mainConfigYaml
}

func downloadPrometheusRecordRuleMainConfigYaml(c *gin.Context) {
	mainConfigYaml := fetchPrometheusRecordRuleMainConfigYaml(c)
	c.String(200, mainConfigYaml)
}

func getMonitorPrometheusRecordRuleYamlOne(c *gin.Context) {
	mainConfigYaml := fetchPrometheusRecordRuleMainConfigYaml(c)
	common.OkWithData(mainConfigYaml, c)
}

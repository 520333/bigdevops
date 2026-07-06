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

// ==========  alertmanager配置 ===========
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
func fetchPrometheusRuleMainConfigYaml(c *gin.Context) string {
	mc := c.MustGet(common.GIN_CTX_MONITOR_CACHE).(*cache.MonitorCache)
	ip := c.Query("ip")
	mainConfigYaml := mc.GetPrometheusRuleConfigYamlByIp(ip)
	return mainConfigYaml
}

func downloadPrometheusRuleMainConfigYaml(c *gin.Context) {
	mainConfigYaml := fetchPrometheusRuleMainConfigYaml(c)
	c.String(200, mainConfigYaml)
}

func getMonitorPrometheusRuleYamlOne(c *gin.Context) {
	mainConfigYaml := fetchPrometheusRuleMainConfigYaml(c)
	common.OkWithData(mainConfigYaml, c)
}

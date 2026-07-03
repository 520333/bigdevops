package view

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"

	"github.com/gin-gonic/gin"
)

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

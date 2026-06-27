package view

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"

	"github.com/gin-gonic/gin"
)

func downloadPrometheusMainConfigYaml(c *gin.Context) {
	mc := c.MustGet(common.GIN_CTX_MONITOR_CACHE).(*cache.MonitorCache)
	ip := c.Query("ip")

	// 从缓存中根据ip 获取这个节点的主配置文件
	mainConfigYaml := mc.GetPrometheusMainConfigYamlByIp(ip)
	if mainConfigYaml == "" {
		c.String(200, "未找到该机器")
	}
	c.String(200, mainConfigYaml)

}

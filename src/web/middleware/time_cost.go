package middleware

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TimeCost() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
		sc.Logger.Info("耗时中间件打印结果:",
			zap.String("url", c.Request.URL.String()),
			zap.Duration("耗时", time.Since(start)))
		log.Printf("the request URL %s cost %v\n", c.Request.RequestURI, time.Since(start))
	}
}

package web

import (
	"bigdevops/src/cache"
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/web/middleware"
	"bigdevops/src/web/view_server"
	"net/http"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/zsais/go-gin-prometheus"

	"github.com/gin-gonic/gin"
)

// ServerStartGin 启动gin
// view_server 路由放专门目录下
func ServerStartGin(sc *config.ServerConfig, mc *cache.MonitorCache) error {
	// 初始化引擎
	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()
	//r := gin.Default()
	r := gin.New()
	r.Use(gin.Recovery())

	varMap := map[string]interface{}{}
	//varMap[common.GIN_CTX_CONFIG_LOGGER] = sc.Logger
	varMap[common.GIN_CTX_CONFIG_CONFIG] = sc
	varMap[common.GIN_CTX_MONITOR_CACHE] = mc
	r.Use(middleware.ConfigMiddleware(varMap))
	//r.Use(middleware.TimeCost())
	r.Use(requestid.New())
	// 记录request-id body header中的token
	r.Use(middleware.NewGinZapLogger(sc.Logger))
	//r.Use(ginzap.Ginzap(sc.Logger, time.RFC3339, false))

	// 暴露metrics
	p := ginprometheus.NewPrometheus("bigdevops")
	p.Use(r)

	// 配置路由
	view_server.ConfigRouter(r)
	s := &http.Server{
		Addr:           sc.HttpAddr,
		Handler:        r,
		ReadTimeout:    time.Second * 5,
		WriteTimeout:   time.Second * 30,
		MaxHeaderBytes: 1 << 20,
	}

	return s.ListenAndServe()
}

package web

import (
	_ "bigdevops/docs"
	"bigdevops/src/cache"
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/web/middleware"
	"bigdevops/src/web/view_server"
	"net/http"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/zsais/go-gin-prometheus"
)

// ServerStartGin 启动gin
// view_server 路由放专门目录下
func ServerStartGin(sc *config.ServerConfig, mc *cache.MonitorCache, kc *cache.K8sClusterCache, jc *cache.JenkinsCache) error {
	// 初始化引擎
	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()
	//r := gin.Default()
	r := gin.New()
	r.Use(gin.Recovery())

	// 注册 Swagger UI 路由（增加 HTTP Basic Auth 账号密码认证）
	swaggerGroup := r.Group("/swagger", gin.BasicAuth(gin.Accounts{
		"admin":  "devops666", // 账号 : 密码 (可添加多组)
		"devops": "devops666",
	}))
	swaggerGroup.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	varMap := map[string]interface{}{}
	//varMap[common.GIN_CTX_CONFIG_LOGGER] = sc.Logger
	varMap[common.GIN_CTX_CONFIG_CONFIG] = sc
	varMap[common.GIN_CTX_MONITOR_CACHE] = mc
	varMap[common.GIN_CTX_K8S_CACHE] = kc
	varMap[common.GIN_CTX_JENKINS_CACHE] = jc
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

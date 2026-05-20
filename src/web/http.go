package web

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/web/middleware"
	"bigdevops/src/web/view"
	"net/http"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/zsais/go-gin-prometheus"

	"github.com/gin-gonic/gin"
)

// StartGin 启动gin
// view 路由放专门目录下
func StartGin(sc *config.ServerConfig) error {
	// 初始化引擎
	//gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()
	//r := gin.Default()
	r := gin.New()
	r.Use(gin.Recovery())

	varMap := map[string]interface{}{}
	//varMap[common.GIN_CTX_CONFIG_LOGGER] = sc.Logger
	varMap[common.GIN_CTX_CONFIG_CONFIG] = sc
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
	view.ConfigRouter(r)
	s := &http.Server{
		Addr:           sc.HttpAddr,
		Handler:        r,
		ReadTimeout:    time.Second * 5,
		WriteTimeout:   time.Second * 30,
		MaxHeaderBytes: 1 << 20,
	}

	return s.ListenAndServe()
}

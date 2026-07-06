package web

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/web/middleware"
	"net/http"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	ginprometheus "github.com/zsais/go-gin-prometheus"
)

func AlertWebhookStartGin(sc *config.AlertWebhookConfig) error {
	gin.DisableConsoleColor()
	r := gin.New()
	varMap := map[string]interface{}{}
	varMap[common.GIN_CTX_CONFIG_CONFIG] = sc
	r.Use(middleware.ConfigMiddleware(varMap))
	r.Use(requestid.New())
	r.Use(middleware.NewGinZapLogger(sc.Logger))

	// 暴露metrics
	p := ginprometheus.NewPrometheus("bigdevops-webhook")
	p.Use(r)
	AlertWebhookConfigRouter(r)
	s := &http.Server{
		Addr:           sc.HttpAddr,
		Handler:        r,
		ReadTimeout:    time.Second * 5,
		WriteTimeout:   time.Second * 30,
		MaxHeaderBytes: 1 << 20,
	}

	return s.ListenAndServe()
}

func AlertWebhookConfigRouter(r *gin.Engine) {
	base := r.Group("/")
	{
		base.GET("/ping", ping)
	}
}

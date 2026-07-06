package web

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/web/middleware"
	"bigdevops/src/web/view_alertwebhook"
	"net/http"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/alertmanager/template"
	ginprometheus "github.com/zsais/go-gin-prometheus"
)

func AlertWebhookStartGin(sc *config.AlertWebhookConfig, alertReceiveQ chan template.Alert) error {
	gin.DisableConsoleColor()
	r := gin.New()
	varMap := map[string]interface{}{}
	varMap[common.GIN_CTX_CONFIG_CONFIG] = sc
	varMap[common.GIN_CTX_CONFIG_ALERTRECEIVEQ] = alertReceiveQ
	r.Use(middleware.ConfigMiddleware(varMap))
	r.Use(requestid.New())
	r.Use(middleware.NewGinZapLogger(sc.Logger))

	// 暴露metrics
	p := ginprometheus.NewPrometheus("bigdevops-webhook")
	p.Use(r)
	view_alertwebhook.ConfigRouter(r)
	s := &http.Server{
		Addr:           sc.HttpAddr,
		Handler:        r,
		ReadTimeout:    time.Second * 5,
		WriteTimeout:   time.Second * 30,
		MaxHeaderBytes: 1 << 20,
	}

	return s.ListenAndServe()
}

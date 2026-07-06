package view_alertwebhook

import (
	"time"

	"github.com/gin-gonic/gin"
)

func ConfigRouter(r *gin.Engine) {
	base := r.Group("/")
	base.GET("/now", getNowTs)
	base.POST("/receive", AlertReceive)

}
func getNowTs(c *gin.Context) {
	c.String(200, time.Now().Format("2006-01-02 15:04:05"))
}

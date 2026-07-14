package middleware

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func UserStatusMiddleware() gin.HandlerFunc {

	return func(c *gin.Context) {
		userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
		sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
		dbUser, err := models.GetUserByUsername(userName)
		if err != nil || dbUser.Enable == 2 {
			sc.Logger.Info("登录拦截：账号已被禁用", zap.String("userName", userName))
			common.Req403WithMessage("您的账号已被禁用，请联系管理员", c)
			c.Abort()
			return
		}
		c.Next()
	}
}

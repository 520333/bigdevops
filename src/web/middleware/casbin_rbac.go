package middleware

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func CasBinRbacMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
		sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
		dbUser, err := models.GetUserByUsername(userName)
		if err != nil {
			sc.Logger.Error("[casbin]通过token解析到的userName去数据库中找User", zap.Error(err))
			common.ReqBadFailWithMessage(fmt.Sprintf("[casbin]通过token解析到的userName去数据库中找User失败 %v", err.Error()), c)
			c.Abort()
			return
		}

		path := c.Request.URL.Path
		method := c.Request.Method

		pass := false
		for _, role := range dbUser.Roles {
			role := role
			ok, err := models.CasBinCheckPermission(role.RoleValue, path, method)
			if err != nil {
				sc.Logger.Error("casbin校验出错",
					zap.Error(err),
					zap.String("userName", userName),
					zap.String("RoleValue", role.RoleValue),
					zap.String("path", path),
					zap.String("method", method),
				)
				common.ReqBadFailWithMessage(fmt.Sprintf("[casbin]校验出错：%v", err.Error()), c)
				c.Abort()
				return
			}
			sc.Logger.Debug("[casbin]单个role-casbin校验结果",
				zap.Bool("通过状态", ok),
				zap.String("userName", userName),
				zap.String("RoleValue", role.RoleValue),
				zap.String("path", path),
				zap.String("method", method),
			)
			if ok {
				pass = true
				break
			}
		}
		if !pass {
			sc.Logger.Warn("casbin校验未通过(403)",
				zap.String("userName", userName),
				zap.String("path", path),
				zap.String("method", method),
			)
			common.Req403WithMessage("[casbin]校验未通过", c)
			c.Abort()
			return
		}
		sc.Logger.Info("casbin校验通过",
			zap.String("userName", userName),
			zap.String("path", path),
			zap.String("method", method),
		)
		c.Next()
	}
}

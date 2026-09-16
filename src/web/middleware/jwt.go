package middleware

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func JWTAuthMiddleWare() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 01.去Header里面找Authorization 没有报401
		authHeaderString := c.Request.Header.Get("Authorization")
		if authHeaderString == "" && c.Query("token") != "" {
			authHeaderString = "Bearer " + c.Query("token")
		}
		if authHeaderString == "" {
			common.Req401WithDetailed(gin.H{"reload": true}, "未登录或非法访问 header没有Authorization", c)
			c.Abort()
			return
		}
		// Bearer <Token>
		parts := strings.SplitN(authHeaderString, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			common.Req401WithDetailed(gin.H{"reload": true}, "请求头中的auth格式错误", c)
			c.Abort()
			return
		}
		// 02.拿到jwt token字符串解析
		sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
		userClaims, err := models.ParseToken(parts[1], sc)
		if err != nil {
			common.Req401WithDetailed(gin.H{"reload": true}, fmt.Sprintf("ParseToken 解析token包含的信息错误：%v", err.Error()), c)
			c.Abort()
			return
		}

		// 03. 单设备互斥登录校验：如果当前 Token 不是该用户最新活跃的 Token，判定为被顶号下线
		if !models.IsLatestUserToken(userClaims.Username, parts[1]) {
			sc.Logger.Error("用户账号在其他设备登录，强制当前设备下线",
				zap.String("user", userClaims.Username),
				zap.Uint("userId", userClaims.SystemUser.ID),
			)
			common.Req401WithDetailed(gin.H{"reload": true, "kicked": true}, "该账号已在其他设备登录，您已被强制下线！", c)
			c.Abort()
			return
		}

		// 04.传递给业务处理函数使用

		// 05.续期 前端需要获取Header中的new-token
		// 如果 (过期时间 - 当前时间) < 缓冲时间，说明快过期了
		if userClaims.RegisteredClaims.ExpiresAt.Unix()-time.Now().Unix() < int64(sc.JWTC.BufferDuration/time.Second) {
			sc.Logger.Info("jwt临期，自动刷新jwt续签",
				zap.String("user", userClaims.Username),
				zap.Int64("剩余秒数", userClaims.RegisteredClaims.ExpiresAt.Unix()-time.Now().Unix()),
			)
			newToken, err := models.GenJWTToken(userClaims.SystemUser, sc)
			if err != nil {
				common.Result5xx(0, gin.H{}, fmt.Sprintf("ParseToken 解析token包含的信息错误：%v", err.Error()), c)
				c.Abort()
			}
			// 同步刷新该用户最新活跃 Token，防止自身续签后误判为被顶号
			models.SetUserActiveToken(userClaims.Username, newToken)

			// 将新 Token 放入响应头，约定 Key 为 "new-token"
			// 前端拦截器检测到这个 Header 时，自动更新本地存储
			c.Header("new-token", newToken)
			c.Header("Access-Control-Expose-Headers", "new-token")
		}
		//c.Set(common.GIN_CTX_JWT_CLAIM, userClaims)
		c.Set(common.GIN_CTX_JWT_USER_NAME, userClaims.Username)
		c.Next()
	}
}

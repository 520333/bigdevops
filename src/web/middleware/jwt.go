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
			msg := fmt.Sprintf("登录凭据校验失败：%v", err.Error())
			if strings.Contains(err.Error(), "token is expired") {
				msg = "登录已过期，请重新登录"
			}
			common.Req401WithDetailed(gin.H{"reload": true, "expired": true}, msg, c)
			c.Abort()
			return
		}

		// 03. 单设备互斥登录与强退校验 (支持续期过渡宽限期与管理员强退精准识别)
		valid, isKickedOut, latestToken := models.CheckUserTokenStatus(userClaims.Username, parts[1])
		if !valid {
			if isKickedOut {
				sc.Logger.Warn("用户已被管理员强制下线",
					zap.String("user", userClaims.Username),
					zap.Uint("userId", userClaims.SystemUser.ID),
				)
				common.Req401WithDetailed(gin.H{"reload": true, "kicked": true}, "该账号已被管理员强制下线，请重新登录！", c)
			} else {
				sc.Logger.Warn("用户账号在其他设备登录，强制当前设备下线",
					zap.String("user", userClaims.Username),
					zap.Uint("userId", userClaims.SystemUser.ID),
				)
				common.Req401WithDetailed(gin.H{"reload": true, "kicked": true}, "该账号已在其他设备登录，您已被强制下线！", c)
			}
			c.Abort()
			return
		}

		// 若当前请求使用的是处于平滑过渡宽限期内的上一版旧 Token，主动在响应头回传最新 Token 加快前端同步
		if latestToken != "" && latestToken != parts[1] {
			c.Header("new-token", latestToken)
			c.Header("Access-Control-Expose-Headers", "new-token")
		}

		// 04.传递给业务处理函数使用

		// 05.续期 前端需要获取Header中的new-token
		// 如果 (过期时间 - 当前时间) < 缓冲时间，说明快过期了
		if userClaims.RegisteredClaims.ExpiresAt.Unix()-time.Now().Unix() < int64(sc.JWTC.BufferDuration/time.Second) {
			newToken, isNew, err := models.RenewUserToken(userClaims.SystemUser, parts[1], sc)
			if err != nil {
				common.Result5xx(0, gin.H{}, fmt.Sprintf("自动续签token失败：%v", err.Error()), c)
				c.Abort()
				return
			}
			if isNew {
				sc.Logger.Info("jwt临期，完成平滑自动续签",
					zap.String("user", userClaims.Username),
					zap.Int64("剩余秒数", userClaims.RegisteredClaims.ExpiresAt.Unix()-time.Now().Unix()),
				)
			}
			// 将新 Token 放入响应头，约定 Key 为 "new-token"
			// 前端拦截器检测到这个 Header 时，自动更新本地存储
			c.Header("new-token", newToken)
			c.Header("Access-Control-Expose-Headers", "new-token")
		}
		// 确保在线用户会话存在：即使服务重启，也能自动从有效 JWT 中自愈恢复在线会话并刷新最近活跃时间、真实IP与浏览器环境
		models.EnsureOnlineSession(userClaims, parts[1], common.GetRealClientIP(c), c.Request.UserAgent())

		//c.Set(common.GIN_CTX_JWT_CLAIM, userClaims)
		c.Set(common.GIN_CTX_JWT_USER_NAME, userClaims.Username)
		c.Next()
	}
}

package models

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UserActiveTokens 全局维护：username -> 当前合法的最新 Token (用于单设备登录/顶号互斥控制)
var UserActiveTokens sync.Map

// SetUserActiveToken 记录用户的最新活跃 Token
func SetUserActiveToken(username string, token string) {
	UserActiveTokens.Store(username, token)
}

// IsLatestUserToken 校验当前 Token 是否为该用户最新活跃 Token
func IsLatestUserToken(username string, currentToken string) bool {
	val, ok := UserActiveTokens.Load(username)
	if !ok {
		// 服务刚重启或该用户尚无记录时，将当前有效 Token 作为最新 Token
		UserActiveTokens.Store(username, currentToken)
		return true
	}
	return val.(string) == currentToken
}

// ClearUserActiveToken 用户退出登录时清理
func ClearUserActiveToken(username string) {
	UserActiveTokens.Delete(username)
}

func TokenNext(dbUser *SystemUser, c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	token, err := GenJWTToken(dbUser, sc)
	if err != nil {
		sc.Logger.Error("生成token失败", zap.Error(err))
		common.FailWithMessage("生成token失败", c)
		return
	}

	// 记录最新有效 Token（互斥登录：顶掉之前的登录会话）
	SetUserActiveToken(dbUser.Username, token)

	userRsp := UserLoginResponse{
		SystemUser: dbUser,
		Token:      token,
	}
	common.OkWithDetailed(userRsp, "登录成功", c)
}

func GenJWTToken(dbUser *SystemUser, sc *config.ServerConfig) (string, error) {
	c := UserCustomClaims{
		SystemUser: dbUser,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(), // 唯一JWT标识(JTI)，确保每次签发的Token完全独立唯一，防止同一秒内/高频登录生成相同Token
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    sc.JWTC.Issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(sc.JWTC.ExpiresDuration)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString([]byte(sc.JWTC.SigningKey))
}

func ParseToken(jwtLongToken string, sc *config.ServerConfig) (*UserCustomClaims, error) {
	tokenClaims, err := jwt.ParseWithClaims(
		jwtLongToken,
		&UserCustomClaims{},
		func(token *jwt.Token) (i interface{}, e error) {
			return []byte(sc.JWTC.SigningKey), nil
		},
	)
	if err != nil {
		sc.Logger.Error("根据长tokenString解析错误", zap.Error(err))
		return nil, err
	}
	if claims, ok := tokenClaims.Claims.(*UserCustomClaims); ok && tokenClaims.Valid {
		return claims, nil
	}
	return nil, err
}

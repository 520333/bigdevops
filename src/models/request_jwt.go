package models

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

func TokenNext(dbUser *User, c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	token, err := GenJWTToken(dbUser, sc)
	if err != nil {
		sc.Logger.Error("生成token失败", zap.Error(err))
		common.FailWithMessage("生成token失败", c)
		return
	}
	userRsp := UserLoginResponse{
		User:  dbUser,
		Token: token,
	}
	common.OkWithDetailed(userRsp, "登录成功", c)
}

func GenJWTToken(dbUser *User, sc *config.ServerConfig) (string, error) {
	c := UserCustomClaims{
		User: dbUser,
		RegisteredClaims: jwt.RegisteredClaims{
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

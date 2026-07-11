package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/models"
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func commonGetUsersByNames(userNames []string, logger *zap.Logger, c *gin.Context) (res []*models.User) {
	for _, userName := range userNames {
		userName := userName
		dbUser, err := models.GetUserByName(userName)
		if err != nil {
			logger.Error("通过token解析到userName去数据库中找user失败", zap.Error(err))
			common.ReqBadFailWithMessage(fmt.Sprintf("通过token解析到userName去数据库中找user失败：%v", err.Error()), c)
			return
		}
		res = append(res, dbUser)
	}
	return
}

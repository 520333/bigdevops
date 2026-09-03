package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/models"
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type setAlertRuleEnableBatchReq struct {
	Ids    []int `json:"ids" validate:"required,min=1"`        // 接收一个 ID 数组，要求至少有 1 个元素
	Enable int   `json:"enable" validate:"required,oneof=1 2"` // 1=启用 2=禁用}
}

type setRecordRuleEnableBatchReq struct {
	Ids    []int `json:"ids" validate:"required,min=1"`        // 接收一个 ID 数组，要求至少有 1 个元素
	Enable int   `json:"enable" validate:"required,oneof=1 2"` // 1=启用 2=禁用}
}

func commonGetUsersByNames(userNames []string, logger *zap.Logger, c *gin.Context) (res []*models.SystemUser) {
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

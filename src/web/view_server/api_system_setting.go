package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/models"
	"github.com/gin-gonic/gin"
)

// GetSystemSetting 获取系统全局设置
func GetSystemSetting(c *gin.Context) {
	var setting models.SystemSetting
	err := models.Db.First(&setting).Error
	if err != nil {
		// 如果不存在，创建一条默认记录
		setting = models.SystemSetting{
			WatermarkEnabled: false,
			WatermarkText:    "",
		}
		models.Db.Create(&setting)
	}

	common.OkWithData(setting, c)
}

// UpdateSystemSetting 更新系统全局设置
func UpdateSystemSetting(c *gin.Context) {
	var req models.SystemSetting
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage("参数解析错误", c)
		return
	}

	var setting models.SystemSetting
	err := models.Db.First(&setting).Error
	if err != nil {
		// 不存在则创建
		models.Db.Create(&req)
	} else {
		// 更新
		setting.WatermarkEnabled = req.WatermarkEnabled
		setting.WatermarkText = req.WatermarkText
		models.Db.Save(&setting)
	}

	common.OkWithMessage("更新成功", c)
}

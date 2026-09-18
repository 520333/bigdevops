package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/models"
	"github.com/gin-gonic/gin"
)

// @Summary      获取系统全局设置
// @Description  获取系统全局设置 接口
// @Tags         system-setting
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取系统全局设置 响应结果"
// @Router       /system/setting/get [get]
// @Security     Bearer
func GetSystemSetting(c *gin.Context) {
	var setting models.SystemSetting
	err := models.Db.First(&setting).Error
	if err != nil {
		// 如果不存在，创建一条默认记录
		setting = models.SystemSetting{
			WatermarkEnabled:     false,
			WatermarkText:        "",
			UpgradePromptEnabled: false,
			UpgradePromptTiming:  "version_once",
			UpgradePromptTitle:   "新版本发布",
			UpgradePromptVersion: "v1.0.0",
			UpgradePromptContent: "系统已升级至最新版本，优化了部分功能并提升了运行稳定性。",
		}
		models.Db.Create(&setting)
	} else {
		// 针对老数据补齐默认值
		if setting.UpgradePromptTiming == "" {
			setting.UpgradePromptTiming = "version_once"
		}
		if setting.UpgradePromptTitle == "" {
			setting.UpgradePromptTitle = "新版本发布"
		}
	}

	common.OkWithData(setting, c)
}

// @Summary      更新系统全局设置
// @Description  更新系统全局设置 接口
// @Tags         system-setting
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "更新系统全局设置 响应结果"
// @Router       /system/setting/update [put]
// @Security     Bearer
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
		setting.UpgradePromptEnabled = req.UpgradePromptEnabled
		setting.UpgradePromptTiming = req.UpgradePromptTiming
		setting.UpgradePromptTitle = req.UpgradePromptTitle
		setting.UpgradePromptVersion = req.UpgradePromptVersion
		setting.UpgradePromptContent = req.UpgradePromptContent
		models.Db.Save(&setting)
	}

	common.OkWithMessage("更新成功", c)
}

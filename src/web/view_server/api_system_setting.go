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
			WatermarkEnabled: false,
			WatermarkText:    "",
		}
		models.Db.Create(&setting)
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
		models.Db.Save(&setting)
	}

	common.OkWithMessage("更新成功", c)
}

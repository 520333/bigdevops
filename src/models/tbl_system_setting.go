package models

import "gorm.io/gorm"

// SystemSetting 系统设置模型
type SystemSetting struct {
	gorm.Model
	WatermarkEnabled     bool   `json:"watermarkEnabled"`
	WatermarkText        string `json:"watermarkText"`
	UpgradePromptEnabled bool   `json:"upgradePromptEnabled"`
	UpgradePromptTiming  string `json:"upgradePromptTiming"` // "never", "every_login", "version_once", "day_once"
	UpgradePromptTitle   string `json:"upgradePromptTitle"`
	UpgradePromptVersion string `json:"upgradePromptVersion"`
	UpgradePromptContent string `json:"upgradePromptContent"`
}

// TableName 设置表名
func (SystemSetting) TableName() string {
	return "system_setting"
}

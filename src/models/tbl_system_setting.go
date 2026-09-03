package models

import "gorm.io/gorm"

// SystemSetting 系统设置模型
type SystemSetting struct {
	gorm.Model
	WatermarkEnabled bool   `json:"watermarkEnabled"`
	WatermarkText    string `json:"watermarkText"`
}

// TableName 设置表名
func (SystemSetting) TableName() string {
	return "system_setting"
}

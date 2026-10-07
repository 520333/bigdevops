package models

import (
	"gorm.io/gorm"
)

// JenkinsUserToken 存储各用户在各 Jenkins 实例的专属 API Token (方案2: 自动代管代生成，实现真实用户构建审计)
type JenkinsUserToken struct {
	Model
	InstanceID uint   `json:"instanceId" gorm:"uniqueIndex:uk_inst_user;comment:关联实例ID"`
	Username   string `json:"username" gorm:"uniqueIndex:uk_inst_user;type:varchar(100);comment:Keycloak/本地用户名"`
	TokenValue string `json:"tokenValue" gorm:"type:varchar(255);comment:该用户专属的 Jenkins API Token"`
}

func (JenkinsUserToken) TableName() string {
	return "jenkins_user_token"
}

// GetJenkinsUserToken 查询用户在指定实例下的 API Token
func GetJenkinsUserToken(instanceId uint, username string) (string, error) {
	var record JenkinsUserToken
	err := Db.Where("instance_id = ? AND username = ?", instanceId, username).First(&record).Error
	if err != nil {
		return "", err
	}
	return record.TokenValue, nil
}

// SaveJenkinsUserToken 保存或更新用户在指定实例下的 API Token
func SaveJenkinsUserToken(instanceId uint, username string, tokenValue string) error {
	var record JenkinsUserToken
	err := Db.Where("instance_id = ? AND username = ?", instanceId, username).First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			newRecord := JenkinsUserToken{
				InstanceID: instanceId,
				Username:   username,
				TokenValue: tokenValue,
			}
			return Db.Create(&newRecord).Error
		}
		return err
	}
	return Db.Model(&record).Update("token_value", tokenValue).Error
}

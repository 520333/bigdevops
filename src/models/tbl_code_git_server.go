package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CodeGitServer struct {
	Model
	Name        string `json:"name" gorm:"type:varchar(100);uniqueIndex;comment:配置名称,例如'内部GitLab'"`
	Platform    string `json:"platform" gorm:"type:varchar(50);comment:平台类型: gitlab / gitea"`
	Endpoint    string `json:"endpoint" gorm:"type:varchar(255);comment:服务API地址,例如 https://gitlab.company.com"`
	Description string `json:"description" gorm:"type:varchar(255);comment:描述"`

	UserID uint `json:"userId" gorm:"comment:创建人ID(关联User表)"`

	// 鉴权相关
	AuthType string `json:"authType" gorm:"type:varchar(50);comment:认证方式: token / password"`
	Username string `json:"username" gorm:"type:varchar(100);comment:用户名(密码认证时使用)"`

	Token string `json:"token" gorm:"type:varchar(500);comment:访问令牌(PAT),建议加密存储"`
	// 网络与状态
	SkipVerify bool `json:"skipVerify" gorm:"comment:是否跳过SSL证书验证(针对自签证书)"`

	Status string `json:"status" gorm:"type:varchar(50);comment:连接状态: connected / disconnected"`

	ExternalLabelsFront string `json:"externalLabelsFront" gorm:"-"`
	Key                 string `json:"key" gorm:"-"` // 前端表格使用
	CreateUserName      string `json:"createUserName" gorm:"-"`
}

// CreateOne 新增
func (obj *CodeGitServer) CreateOne() error {
	return Db.Create(obj).Error
}

// UpdateOne 更新
func (obj *CodeGitServer) UpdateOne() error {
	// 使用 Updates 可以只更新非零值字段；如果需要强制更新某个状态，使用 Select
	return Db.Updates(obj).Error
}

// DeleteOne 删除 (硬删除或软删除取决于项目中 gorm.Model 的配置)
func (obj *CodeGitServer) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

// GetCodeGitServerById 根据ID查询
func GetCodeGitServerById(id int) (*CodeGitServer, error) {
	var dbObj CodeGitServer
	err := Db.Where("id = ?", id).First(&dbObj).Error
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			return nil, fmt.Errorf("git配置不存在")
		}
		return nil, fmt.Errorf("数据库错误: %v", err)
	}
	return &dbObj, nil
}

// GetCodeGitServerList 获取列表及总数
func GetCodeGitServerList(limit, offset int, name, platform, endpoint string) (objs []*CodeGitServer, total int64, err error) {
	query := Db.Model(&CodeGitServer{})
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if platform != "" {
		query = query.Where("platform = ?", platform)
	}
	if endpoint != "" {
		query = query.Where("endpoint LIKE ?", "%"+endpoint+"%")
	}

	err = query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = query.Limit(limit).Offset(offset).Order("id desc").Find(&objs).Error
	return objs, total, err
}
func (obj *CodeGitServer) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)
}

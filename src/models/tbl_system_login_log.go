package models

import (
	"bigdevops/src/common"
	"fmt"

	"github.com/gin-gonic/gin"
)

// SystemLoginLog 系统用户登录审计日志模型
type SystemLoginLog struct {
	Model
	UserID    uint   `json:"userId" gorm:"index;comment:用户ID"`
	Username  string `json:"userName" gorm:"type:varchar(100);index;comment:登录账号"`
	RealName  string `json:"realName" gorm:"type:varchar(100);comment:用户姓名"`
	LoginType string `json:"loginType" gorm:"type:varchar(50);index;comment:登录类型(密码登录/OIDC单点/钉钉扫码/退出登录/强退)"`
	Ip        string `json:"ip" gorm:"type:varchar(100);comment:登录IP"`
	OS        string `json:"os" gorm:"type:varchar(100);comment:操作系统"`
	Browser   string `json:"browser" gorm:"type:varchar(100);comment:浏览器"`
	Status    int    `json:"status" gorm:"comment:登录状态(1:成功, 0:失败)"`
	Message   string `json:"message" gorm:"type:varchar(255);comment:提示信息"`

	Key string `json:"key" gorm:"-"` // 前端表格使用
}

func (l *SystemLoginLog) Create() error {
	return Db.Create(l).Error
}

func (l *SystemLoginLog) FillFrontData() {
	l.CreatedTime = common.TimeFormat(l.CreatedAt)
	l.UpdatedTime = common.TimeFormat(l.UpdatedAt)
	l.Key = fmt.Sprintf("%d", l.ID)
}

// RecordLoginLog 异步记录系统登录/登出日志
func RecordLoginLog(c *gin.Context, username, realName, loginType string, status int, message string) {
	ip := common.GetRealClientIP(c)
	ua := ""
	if c != nil && c.Request != nil {
		ua = c.Request.UserAgent()
	}
	browser, os := common.ParseUserAgent(ua)

	go func(uName, rName, lType, uIp, uOs, uBrowser, msg string, st int) {
		userID := uint(0)
		if uName != "" && rName == "" {
			user, err := GetUserByUsername(uName)
			if err == nil && user != nil {
				userID = user.ID
				rName = user.RealName
			}
		} else if uName != "" {
			user, err := GetUserByUsername(uName)
			if err == nil && user != nil {
				userID = user.ID
			}
		}

		log := &SystemLoginLog{
			UserID:    userID,
			Username:  uName,
			RealName:  rName,
			LoginType: lType,
			Ip:        uIp,
			OS:        uOs,
			Browser:   uBrowser,
			Status:    st,
			Message:   msg,
		}
		_ = log.Create()
	}(username, realName, loginType, ip, os, browser, message, status)
}

// RecordLoginLogDirect 直接记录登录日志（无需gin.Context，用于后台踢下线等异步触发场景）
func RecordLoginLogDirect(username, realName, ip, os, browser, loginType string, status int, message string) {
	go func() {
		userID := uint(0)
		if username != "" {
			user, err := GetUserByUsername(username)
			if err == nil && user != nil {
				userID = user.ID
				if realName == "" {
					realName = user.RealName
				}
			}
		}

		log := &SystemLoginLog{
			UserID:    userID,
			Username:  username,
			RealName:  realName,
			LoginType: loginType,
			Ip:        common.NormalizeIP(ip),
			OS:        os,
			Browser:   browser,
			Status:    status,
			Message:   message,
		}
		_ = log.Create()
	}()
}

// LoginLogFilter 查询过滤条件
type LoginLogFilter struct {
	Username  string `form:"userName"`
	LoginType string `form:"loginType"`
	Status    *int   `form:"status"`
	StartDate string `form:"startDate"`
	EndDate   string `form:"endDate"`
}

// GetLoginLogList 分页过滤查询登录日志
func GetLoginLogList(filter LoginLogFilter, page, pageSize int) ([]*SystemLoginLog, int64, error) {
	var logs []*SystemLoginLog
	var total int64

	db := Db.Model(&SystemLoginLog{})

	if filter.Username != "" {
		db = db.Where("username LIKE ? OR real_name LIKE ?", "%"+filter.Username+"%", "%"+filter.Username+"%")
	}
	if filter.LoginType != "" {
		db = db.Where("login_type = ?", filter.LoginType)
	}
	if filter.Status != nil && *filter.Status >= 0 {
		db = db.Where("status = ?", *filter.Status)
	}
	if filter.StartDate != "" {
		db = db.Where("created_at >= ?", filter.StartDate+" 00:00:00")
	}
	if filter.EndDate != "" {
		db = db.Where("created_at <= ?", filter.EndDate+" 23:59:59")
	}

	err := db.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err = db.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	for _, log := range logs {
		log.FillFrontData()
	}

	return logs, total, nil
}

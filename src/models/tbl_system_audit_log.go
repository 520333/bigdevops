package models

import (
	"bigdevops/src/common"
	"fmt"
)

// SystemAuditLog 系统操作审计日志模型
type SystemAuditLog struct {
	Model
	UserID      uint   `json:"userId" gorm:"index;comment:操作人ID"`
	Username    string `json:"userName" gorm:"type:varchar(100);index;comment:操作人账号"`
	RealName    string `json:"realName" gorm:"type:varchar(100);comment:操作人昵称"`
	Module      string `json:"module" gorm:"type:varchar(100);index;comment:功能模块"`
	Action      string `json:"action" gorm:"type:varchar(100);comment:操作类型"`
	Method      string `json:"method" gorm:"type:varchar(20);comment:请求方法"`
	Path        string `json:"path" gorm:"type:varchar(255);comment:请求路径"`
	Ip          string `json:"ip" gorm:"type:varchar(100);comment:操作者IP"`
	Status      int    `json:"status" gorm:"comment:HTTP状态码"`
	Latency     int64  `json:"latency" gorm:"comment:耗时(毫秒)"`
	ReqBody     string `json:"reqBody" gorm:"type:longtext;comment:请求报文"`
	RespMessage string `json:"respMessage" gorm:"type:text;comment:响应摘要"`

	Key string `json:"key" gorm:"-"` // 前端表格使用
}

func (l *SystemAuditLog) Create() error {
	return Db.Create(l).Error
}

func (l *SystemAuditLog) FillFrontData() {
	l.CreatedTime = common.TimeFormat(l.CreatedAt)
	l.UpdatedTime = common.TimeFormat(l.UpdatedAt)
	l.Key = fmt.Sprintf("%d", l.ID)
}

// AuditLogFilter 查询过滤条件
type AuditLogFilter struct {
	Username  string `form:"userName"`
	Module    string `form:"module"`
	Status    int    `form:"status"`
	StartDate string `form:"startDate"`
	EndDate   string `form:"endDate"`
}

// GetAuditLogList 分页过滤查询审计日志
func GetAuditLogList(filter AuditLogFilter, page, pageSize int) ([]*SystemAuditLog, int64, error) {
	var logs []*SystemAuditLog
	var total int64

	db := Db.Model(&SystemAuditLog{})

	if filter.Username != "" {
		db = db.Where("username LIKE ?", "%"+filter.Username+"%")
	}
	if filter.Module != "" {
		db = db.Where("module = ?", filter.Module)
	} else {
		// 默认排除“用户认证”，保证操作日志纯粹展示业务增删改查
		db = db.Where("module != ?", "用户认证")
	}
	if filter.Status > 0 {
		db = db.Where("status = ?", filter.Status)
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

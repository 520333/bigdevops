package models

import "time"

const (
	NOTIFY_STATUS_READ    = 1
	NOTIFY_STATUS_CLEARED = 2
)

type WorkOrderNotifyStatus struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserName  string    `gorm:"type:varchar(64);uniqueIndex:uk_user_notice,priority:1;not null" json:"userName"`
	NoticeID  string    `gorm:"type:varchar(128);uniqueIndex:uk_user_notice,priority:2;not null" json:"noticeId"`
	Status    int       `gorm:"type:tinyint;not null;comment:'1:已读, 2:已清空'" json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (WorkOrderNotifyStatus) TableName() string {
	return "workorder_notify_status"
}

// CleanDuplicateNotifyStatus 清理历史重复记录并保留状态最高（清空优先于已读）的一条
func CleanDuplicateNotifyStatus() {
	if Db == nil {
		return
	}
	// 1. 先将同一 (user_name, notice_id) 下所有重复记录的状态更新为其中最大值（若有已清空2则全置为2）
	_ = Db.Exec(`
		UPDATE workorder_notify_status t1
		INNER JOIN (
			SELECT user_name, notice_id, MAX(status) as max_status
			FROM workorder_notify_status
			GROUP BY user_name, notice_id
			HAVING COUNT(*) > 1
		) t2 ON t1.user_name = t2.user_name AND t1.notice_id = t2.notice_id
		SET t1.status = t2.max_status
	`).Error

	// 2. 删除重复记录中较旧的行
	_ = Db.Exec(`
		DELETE t1 FROM workorder_notify_status t1
		INNER JOIN workorder_notify_status t2 
		ON t1.user_name = t2.user_name AND t1.notice_id = t2.notice_id
		WHERE t1.id < t2.id
	`).Error
}

// MarkNotifyStatus 标记单条通知/消息状态（已读或已清空）
func MarkNotifyStatus(userName string, noticeID string, status int) error {
	if userName == "" || noticeID == "" {
		return nil
	}
	// 更新所有匹配该用户和该通知的记录（防止历史重复导致状态不一致）
	res := Db.Model(&WorkOrderNotifyStatus{}).
		Where("user_name = ? AND notice_id = ?", userName, noticeID).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}

	newObj := WorkOrderNotifyStatus{
		UserName: userName,
		NoticeID: noticeID,
		Status:   status,
	}
	return Db.Create(&newObj).Error
}

// BatchMarkNotifyStatus 批量标记通知/消息状态（如整页清空）
func BatchMarkNotifyStatus(userName string, noticeIDs []string, status int) error {
	if userName == "" || len(noticeIDs) == 0 {
		return nil
	}
	for _, nid := range noticeIDs {
		_ = MarkNotifyStatus(userName, nid, status)
	}
	return nil
}

// GetUserNotifyStatusMap 获取用户的所有通知状态 map[notice_id]status
func GetUserNotifyStatusMap(userName string) (map[string]int, error) {
	var list []WorkOrderNotifyStatus
	err := Db.Where("user_name = ?", userName).Find(&list).Error
	if err != nil {
		return nil, err
	}
	res := make(map[string]int)
	for _, item := range list {
		// 优先保留 status 更大值（2: CLEARED 绝对优先于 1: READ，防止被旧的已读状态覆盖）
		if cur, ok := res[item.NoticeID]; !ok || item.Status > cur {
			res[item.NoticeID] = item.Status
		}
	}
	return res, nil
}

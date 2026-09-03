package models

import "time"

type Model struct {
	ID        uint `json:"id" gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	CreatedTime string `json:"createdTime,omitempty" gorm:"-"`
	UpdatedTime string `json:"updatedTime,omitempty" gorm:"-"`
}
type EchartsOneItem struct {
	Name  string `json:"name" gorm:"-"`
	Value int    `json:"value" gorm:"-"`
}

func commonGetUserNamesByUsers(users []*SystemUser) []string {
	var userNames []string
	for _, user := range users {
		user := user
		userNames = append(userNames, user.Username)
	}
	return userNames

}

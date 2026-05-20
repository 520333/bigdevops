package models

import "time"

type Model struct {
	ID        uint `json:"id" gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
type EchartsOneItem struct {
	Name  string `json:"name" gorm:"-"`
	Value int    `json:"value" gorm:"-"`
}

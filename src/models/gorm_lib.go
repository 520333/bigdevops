package models

import (
	"database/sql/driver"
	"strings"
)

type StringArray []string

func (m StringArray) Value() (driver.Value, error) {
	str := strings.Join(m, "|")
	return str, nil
}

func (a *StringArray) Scan(value interface{}) error {
	s := value.([]uint8)
	ss := strings.Split(string(s), "|")
	*a = StringArray(ss)
	return nil
}

package models

import "time"

// AppSetting 系统设置（键值对）。
type AppSetting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Remark    string    `json:"remark"`
	UpdatedAt time.Time `json:"updated_at"`
}

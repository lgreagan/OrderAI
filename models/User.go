package models

import "time"

// User 登录账号。密码只保存 bcrypt 哈希，永不出现在接口返回里。
type User struct {
	ID           uint64     `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	DisplayName  string     `json:"display_name"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	SessionCount int        `json:"session_count"`
}

// IsAdmin 管理员可维护账号与系统设置。
func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

// Name 显示名，为空时退回登录名。
func (u *User) Name() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}

// RoleName 角色中文名。
func (u *User) RoleName() string {
	switch u.Role {
	case UserRoleAdmin:
		return "管理员"
	case UserRoleStaff:
		return "业务员"
	}
	return u.Role
}

// StatusName 状态中文名。
func (u *User) StatusName() string {
	if u.Status == UserStatusDisabled {
		return "已停用"
	}
	return "启用中"
}

// Initial 头像用的首字（按字符取，中文不会被截断）。
func (u *User) Initial() string {
	r := []rune(u.Name())
	if len(r) == 0 {
		return "?"
	}
	return string(r[0])
}

// Session 登录会话。token 存库，Cookie 里只放这个随机串。
type Session struct {
	Token     string    `json:"-"`
	UserID    uint64    `json:"user_id"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

const (
	UserRoleAdmin = "admin"
	UserRoleStaff = "staff"

	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)

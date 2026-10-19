package services

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"OrderAI/models"
	"OrderAI/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound      = errors.New("账号不存在")
	ErrDuplicateUsername = errors.New("该登录名已被使用")
	ErrInvalidUsername   = errors.New("登录名 2-32 位，不能包含空格")
	ErrInvalidRole       = errors.New("角色只能是管理员或业务员")
	ErrSelfDisable       = errors.New("不能停用当前登录的账号")
	ErrLastAdmin         = errors.New("至少要保留一个启用状态的管理员")
	ErrPasswordTooShort  = errors.New("密码至少 6 位")
)

// CreateUserInput 新增账号的入参。
type CreateUserInput struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

// UpdateUserInput 修改账号的入参（不含密码）。
type UpdateUserInput struct {
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

// UserService 管理员维护账号：新增、改资料、停用 / 启用、重置密码。
type UserService struct {
	Repo *repository.UserRepository
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{Repo: repo}
}

// List 全部账号（含在线会话数）。
func (s *UserService) List(ctx context.Context) ([]models.User, error) {
	return s.Repo.List(ctx)
}

// Create 新建账号。
func (s *UserService) Create(ctx context.Context, in CreateUserInput) (*models.User, error) {
	username := strings.TrimSpace(in.Username)
	if !validUsername(username) {
		return nil, ErrInvalidUsername
	}
	if !validPassword(in.Password) {
		return nil, ErrPasswordTooShort
	}

	role := strings.TrimSpace(in.Role)
	if role == "" {
		role = models.UserRoleStaff
	}
	if role != models.UserRoleAdmin && role != models.UserRoleStaff {
		return nil, ErrInvalidRole
	}

	status := strings.TrimSpace(in.Status)
	if status != models.UserStatusDisabled {
		status = models.UserStatusActive
	}

	exist, err := s.Repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if exist != nil {
		return nil, ErrDuplicateUsername
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u := &models.User{
		Username:     username,
		PasswordHash: string(hash),
		DisplayName:  strings.TrimSpace(in.DisplayName),
		Role:         role,
		Status:       status,
	}
	if err := s.Repo.Create(ctx, u); err != nil {
		// 并发下唯一键可能先报错，转成友好提示。
		if strings.Contains(err.Error(), "Duplicate") {
			return nil, ErrDuplicateUsername
		}
		return nil, err
	}

	u.PasswordHash = ""

	// 补上创建时间等库侧默认值，返回给前端的数据才完整。
	if saved, err := s.Repo.FindByID(ctx, u.ID); err == nil && saved != nil {
		return saved, nil
	}
	return u, nil
}

// Update 改姓名 / 角色 / 状态。operatorID 是当前登录账号，用于自我保护。
func (s *UserService) Update(
	ctx context.Context,
	id uint64,
	in UpdateUserInput,
	operatorID uint64,
) (*models.User, error) {

	target, err := s.Repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, ErrUserNotFound
	}

	role := strings.TrimSpace(in.Role)
	if role == "" {
		role = target.Role
	}
	if role != models.UserRoleAdmin && role != models.UserRoleStaff {
		return nil, ErrInvalidRole
	}

	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = target.Status
	}
	if status != models.UserStatusActive && status != models.UserStatusDisabled {
		status = target.Status
	}

	if id == operatorID && status == models.UserStatusDisabled {
		return nil, ErrSelfDisable
	}

	// 从「启用中的管理员」变成非管理员或停用，且只剩这一个，拦住。
	losingAdmin := target.Role == models.UserRoleAdmin &&
		target.Status == models.UserStatusActive &&
		(role != models.UserRoleAdmin || status != models.UserStatusActive)
	if losingAdmin {
		n, err := s.Repo.CountActiveAdmins(ctx)
		if err != nil {
			return nil, err
		}
		if n <= 1 {
			return nil, ErrLastAdmin
		}
	}

	displayName := strings.TrimSpace(in.DisplayName)
	if err := s.Repo.UpdateProfile(ctx, id, displayName, role, status); err != nil {
		return nil, err
	}

	// 停用后立即踢下线，避免已登录的旧 Cookie 继续用。
	if status == models.UserStatusDisabled {
		if err := s.Repo.DeleteSessions(ctx, id, ""); err != nil {
			return nil, err
		}
	}

	return s.Repo.FindByID(ctx, id)
}

// ResetPassword 管理员重置指定账号密码（不需要原密码），并踢掉该账号其它会话。
// keepToken 为当前操作者自己的会话（重置自己时保持在线）。
func (s *UserService) ResetPassword(
	ctx context.Context,
	id uint64,
	newPassword, keepToken string,
) error {

	if !validPassword(newPassword) {
		return ErrPasswordTooShort
	}

	target, err := s.Repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if target == nil {
		return ErrUserNotFound
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.Repo.UpdatePassword(ctx, id, string(hash)); err != nil {
		return err
	}

	return s.Repo.DeleteSessions(ctx, id, keepToken)
}

// validUsername 登录名：2-32 个字符，不含空白与控制符。
func validUsername(name string) bool {
	n := utf8.RuneCountInString(name)
	if n < 2 || n > 32 {
		return false
	}
	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validPassword(pwd string) bool {
	return len(strings.TrimSpace(pwd)) >= minPasswordLength
}

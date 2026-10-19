package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"OrderAI/models"
	"OrderAI/repository"

	"golang.org/x/crypto/bcrypt"
)

const (
	// SessionCookie 登录态 Cookie 名。
	SessionCookie = "orderai_session"

	// 记住我 7 天，否则 12 小时（一个工作日够用）。
	sessionTTLRemember = 7 * 24 * time.Hour
	sessionTTLDefault  = 12 * time.Hour

	// DefaultAdminUsername / DefaultAdminPassword 首次启动创建的管理员。
	DefaultAdminUsername = "admin"
	DefaultAdminPassword = "admin123"

	minPasswordLength = 6
)

var (
	ErrBadCredentials = errors.New("用户名或密码不正确")
	ErrUserDisabled   = errors.New("账号已停用，请联系管理员")
	ErrOldPassword    = errors.New("原密码不正确")
	ErrWeakPassword   = errors.New("新密码至少 6 位")
)

// AuthService 负责登录、会话与密码。
type AuthService struct {
	Repo *repository.UserRepository
}

func NewAuthService(repo *repository.UserRepository) *AuthService {
	return &AuthService{Repo: repo}
}

// EnsureDefaultAdmin 没有任何账号时创建一个管理员，返回是否新建。
func (s *AuthService) EnsureDefaultAdmin(ctx context.Context) (bool, error) {
	n, err := s.Repo.Count(ctx)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(DefaultAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}

	u := &models.User{
		Username:     DefaultAdminUsername,
		PasswordHash: string(hash),
		DisplayName:  "管理员",
		Role:         models.UserRoleAdmin,
		Status:       models.UserStatusActive,
	}
	if err := s.Repo.Create(ctx, u); err != nil {
		return false, err
	}
	return true, nil
}

// Login 校验密码并建会话。remember 为真时有效期 7 天。
func (s *AuthService) Login(
	ctx context.Context,
	username, password, ip, userAgent string,
	remember bool,
) (*models.User, *models.Session, error) {

	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, nil, ErrBadCredentials
	}

	u, err := s.Repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, nil, err
	}
	if u == nil {
		return nil, nil, ErrBadCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrBadCredentials
	}
	if u.Status != models.UserStatusActive {
		return nil, nil, ErrUserDisabled
	}

	token, err := newSessionToken()
	if err != nil {
		return nil, nil, err
	}

	ttl := sessionTTLDefault
	if remember {
		ttl = sessionTTLRemember
	}
	sess := &models.Session{
		Token:     token,
		UserID:    u.ID,
		IP:        ip,
		UserAgent: truncate(userAgent, 255),
		ExpiresAt: time.Now().Add(ttl),
	}

	// 过期会话没人清会越堆越多，登录时顺手清一次（失败不影响登录）。
	_ = s.Repo.CleanExpiredSessions(ctx)

	if err := s.Repo.CreateSession(ctx, sess); err != nil {
		return nil, nil, err
	}
	if err := s.Repo.TouchLogin(ctx, u.ID); err != nil {
		return nil, nil, err
	}

	now := time.Now()
	u.LastLoginAt = &now
	return u, sess, nil
}

// UserByToken 会话有效时返回账号。
func (s *AuthService) UserByToken(ctx context.Context, token string) (*models.User, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil
	}
	u, err := s.Repo.SessionUser(ctx, token)
	if err != nil {
		return nil, err
	}
	if u == nil || u.Status != models.UserStatusActive {
		return nil, nil
	}
	return u, nil
}

// Logout 删除会话。
func (s *AuthService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.Repo.DeleteSession(ctx, token)
}

// ChangePassword 校验原密码后改密，并踢掉该账号的其它会话。
func (s *AuthService) ChangePassword(
	ctx context.Context,
	userID uint64,
	oldPassword, newPassword, keepToken string,
) error {

	if len(strings.TrimSpace(newPassword)) < minPasswordLength {
		return ErrWeakPassword
	}

	u, err := s.Repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if u == nil {
		return ErrBadCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrOldPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.Repo.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return err
	}

	// 改密后其它设备上的登录态立即失效，当前这台保持在线。
	return s.Repo.DeleteSessions(ctx, userID, keepToken)
}

// SessionTTL 按「记住我」返回会话时长，控制器据此设置 Cookie 有效期。
func SessionTTL(remember bool) time.Duration {
	if remember {
		return sessionTTLRemember
	}
	return sessionTTLDefault
}

func newSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

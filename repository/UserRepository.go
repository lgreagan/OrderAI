package repository

import (
	"context"
	"database/sql"

	"OrderAI/models"
)

// UserRepository 读写账号与登录会话。
type UserRepository struct {
	DB *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{DB: db}
}

const userColumns = `id, username, password_hash, display_name, role, status, last_login_at, created_at, updated_at`

func scanUser(row interface{ Scan(...interface{}) error }) (*models.User, error) {
	var u models.User
	var last sql.NullTime
	err := row.Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName,
		&u.Role, &u.Status, &last, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if last.Valid {
		t := last.Time
		u.LastLoginAt = &t
	}
	return &u, nil
}

// Count 账号总数（用于首次启动创建默认管理员）。
func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var n int
	err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// FindByUsername 按登录名查账号。
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE username = ?`, username)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// FindByID 按主键查账号。
func (r *UserRepository) FindByID(ctx context.Context, id uint64) (*models.User, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// List 账号列表（不含密码哈希），顺带带出有效会话数用于显示在线状态。
func (r *UserRepository) List(ctx context.Context) ([]models.User, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT u.id, u.username, u.display_name, u.role, u.status,
		       u.last_login_at, u.created_at, u.updated_at,
		       (SELECT COUNT(*) FROM user_sessions s
		         WHERE s.user_id = u.id AND s.expires_at > NOW()) AS sessions
		FROM users u ORDER BY u.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]models.User, 0)
	for rows.Next() {
		var u models.User
		var last sql.NullTime
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role,
			&u.Status, &last, &u.CreatedAt, &u.UpdatedAt, &u.SessionCount); err != nil {
			return nil, err
		}
		if last.Valid {
			t := last.Time
			u.LastLoginAt = &t
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

// Create 新建账号，返回主键。
func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	res, err := r.DB.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, display_name, role, status)
		VALUES (?, ?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.DisplayName, u.Role, u.Status,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	u.ID = uint64(id)
	return nil
}

// UpdatePassword 改密码哈希。
func (r *UserRepository) UpdatePassword(ctx context.Context, id uint64, hash string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

// UpdateProfile 更新姓名、角色与启用状态（不含密码）。
func (r *UserRepository) UpdateProfile(
	ctx context.Context,
	id uint64,
	displayName, role, status string,
) error {
	_, err := r.DB.ExecContext(ctx, `
		UPDATE users SET display_name = ?, role = ?, status = ? WHERE id = ?`,
		displayName, role, status, id,
	)
	return err
}

// CountActiveAdmins 启用状态的管理员数量，用于拦住「把最后一个管理员停用/降级」。
func (r *UserRepository) CountActiveAdmins(ctx context.Context) (int, error) {
	var n int
	err := r.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active'`).Scan(&n)
	return n, err
}

// TouchLogin 记录最近登录时间。
func (r *UserRepository) TouchLogin(ctx context.Context, id uint64) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = ?`, id)
	return err
}

// CreateSession 写入登录会话。
func (r *UserRepository) CreateSession(ctx context.Context, s *models.Session) error {
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO user_sessions (token, user_id, ip, user_agent, expires_at)
		VALUES (?, ?, ?, ?, ?)`,
		s.Token, s.UserID, s.IP, s.UserAgent, s.ExpiresAt,
	)
	return err
}

// SessionUser 用 token 取用户；顺带返回过期时间，方便判断。
// 会话过期或账号被停用时返回 nil。
func (r *UserRepository) SessionUser(ctx context.Context, token string) (*models.User, error) {
	row := r.DB.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.password_hash, u.display_name, u.role, u.status,
		       u.last_login_at, u.created_at, u.updated_at
		FROM user_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token = ? AND s.expires_at > NOW()`, token)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// DeleteSession 退出登录。
func (r *UserRepository) DeleteSession(ctx context.Context, token string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM user_sessions WHERE token = ?`, token)
	return err
}

// DeleteSessions 踢掉某账号的全部会话（可在 keepToken 里留一个，如改密码的当前会话）。
func (r *UserRepository) DeleteSessions(ctx context.Context, userID uint64, keepToken string) error {
	_, err := r.DB.ExecContext(ctx,
		`DELETE FROM user_sessions WHERE user_id = ? AND token <> ?`, userID, keepToken)
	return err
}

// CleanExpiredSessions 清理过期会话，登录时顺手调用。
func (r *UserRepository) CleanExpiredSessions(ctx context.Context) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM user_sessions WHERE expires_at <= NOW()`)
	return err
}

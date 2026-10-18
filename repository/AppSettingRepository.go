package repository

import (
	"context"
	"database/sql"

	"OrderAI/models"
)

// AppSettingRepository 读写系统设置（键值对）。
type AppSettingRepository struct {
	DB *sql.DB
}

func NewAppSettingRepository(db *sql.DB) *AppSettingRepository {
	return &AppSettingRepository{DB: db}
}

// All 返回全部设置。
func (r *AppSettingRepository) All(ctx context.Context) ([]models.AppSetting, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT setting_key, COALESCE(setting_value, ''), remark, updated_at
		FROM app_settings
		ORDER BY setting_key ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.AppSetting, 0)
	for rows.Next() {
		var s models.AppSetting
		if err := rows.Scan(&s.Key, &s.Value, &s.Remark, &s.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, s)
	}

	return result, rows.Err()
}

// Get 读取单个设置值；不存在时返回空字符串。
func (r *AppSettingRepository) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.DB.QueryRowContext(
		ctx,
		`SELECT COALESCE(setting_value, '') FROM app_settings WHERE setting_key = ?`,
		key,
	).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

// Delete 删除设置（不存在时静默返回）。
func (r *AppSettingRepository) Delete(ctx context.Context, key string) error {
	_, err := r.DB.ExecContext(
		ctx,
		`DELETE FROM app_settings WHERE setting_key = ?`,
		key,
	)
	return err
}

// Upsert 写入设置（不存在则新增）。
func (r *AppSettingRepository) Upsert(
	ctx context.Context,
	key, value string,
) error {
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO app_settings (setting_key, setting_value)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value)`,
		key, value,
	)
	return err
}

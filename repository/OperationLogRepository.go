package repository

import (
	"context"
	"database/sql"
	"time"

	"OrderAI/models"
)

// OperationLogRepository 读写关键操作日志。
type OperationLogRepository struct {
	DB *sql.DB
}

func NewOperationLogRepository(db *sql.DB) *OperationLogRepository {
	return &OperationLogRepository{DB: db}
}

const operationLogColumns = `
	id, module, module_name, action, action_name, description,
	method, path, uri, target_id, operator, ip,
	status, success, error_message, duration_ms,
	COALESCE(detail, '') AS detail, created_at
`

func scanOperationLog(row interface {
	Scan(...any) error
}) (*models.OperationLog, error) {
	l := &models.OperationLog{}
	err := row.Scan(
		&l.ID, &l.Module, &l.ModuleName, &l.Action, &l.ActionName, &l.Description,
		&l.Method, &l.Path, &l.URI, &l.TargetID, &l.Operator, &l.IP,
		&l.Status, &l.Success, &l.ErrorMsg, &l.DurationMs,
		&l.Detail, &l.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return l, nil
}

// operationLogWhere 按查询条件拼装 WHERE 子句与参数。
func operationLogWhere(f models.OperationLogFilter) (string, []any) {
	where := " WHERE 1 = 1"
	args := []any{}

	if f.Module != "" {
		where += " AND module = ?"
		args = append(args, f.Module)
	}
	if f.Action != "" {
		where += " AND action = ?"
		args = append(args, f.Action)
	}
	if f.Operator != "" {
		where += " AND operator LIKE ?"
		args = append(args, "%"+f.Operator+"%")
	}
	switch f.Success {
	case "1":
		where += " AND success = 1"
	case "0":
		where += " AND success = 0"
	}
	if f.DateFrom != "" {
		where += " AND created_at >= ?"
		args = append(args, f.DateFrom+" 00:00:00")
	}
	if f.DateTo != "" {
		where += " AND created_at <= ?"
		args = append(args, f.DateTo+" 23:59:59")
	}
	if f.Keyword != "" {
		k := "%" + f.Keyword + "%"
		where += ` AND (
			description LIKE ? OR module_name LIKE ? OR action_name LIKE ?
			OR path LIKE ? OR target_id LIKE ? OR ip LIKE ? OR error_message LIKE ?
		)`
		args = append(args, k, k, k, k, k, k, k)
	}

	return where, args
}

// Create 写入一条操作日志。
func (r *OperationLogRepository) Create(
	ctx context.Context,
	l *models.OperationLog,
) error {
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO operation_logs (
			module, module_name, action, action_name, description,
			method, path, uri, target_id, operator, ip,
			status, success, error_message, duration_ms, detail
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.Module, l.ModuleName, l.Action, l.ActionName, l.Description,
		l.Method, l.Path, l.URI, l.TargetID, l.Operator, l.IP,
		l.Status, l.Success, l.ErrorMsg, l.DurationMs, l.Detail,
	)
	return err
}

// List 分页查询操作日志，最近的在前，同时返回总条数。
func (r *OperationLogRepository) List(
	ctx context.Context,
	f models.OperationLogFilter,
) ([]models.OperationLog, int64, error) {
	f.Normalize()

	where, args := operationLogWhere(f)

	var total int64
	if err := r.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM operation_logs`+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	result := make([]models.OperationLog, 0)
	if total == 0 {
		return result, 0, nil
	}

	query := `SELECT ` + operationLogColumns + ` FROM operation_logs` + where +
		` ORDER BY id DESC LIMIT ? OFFSET ?`
	pageArgs := append(append([]any{}, args...), f.PageSize, f.Offset())

	rows, err := r.DB.QueryContext(ctx, query, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		l, err := scanOperationLog(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *l)
	}

	return result, total, rows.Err()
}

// Export 按查询条件取日志（供导出用），不分页。
func (r *OperationLogRepository) Export(
	ctx context.Context,
	f models.OperationLogFilter,
	limit int,
) ([]models.OperationLog, error) {
	where, args := operationLogWhere(f)

	query := `SELECT ` + operationLogColumns + ` FROM operation_logs` + where +
		` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.OperationLog, 0)
	for rows.Next() {
		l, err := scanOperationLog(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *l)
	}

	return result, rows.Err()
}

// GetByID 按主键查询单条日志。
func (r *OperationLogRepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.OperationLog, error) {
	row := r.DB.QueryRowContext(ctx,
		`SELECT `+operationLogColumns+` FROM operation_logs WHERE id = ?`, id)
	return scanOperationLog(row)
}

// Summary 概览统计：总数、今日、失败数。
func (r *OperationLogRepository) Summary(
	ctx context.Context,
) (*models.OperationLogSummary, error) {
	s := &models.OperationLogSummary{}
	err := r.DB.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(created_at >= CURDATE()), 0),
			COALESCE(SUM(created_at >= CURDATE() AND success = 0), 0),
			COALESCE(SUM(success = 0), 0)
		FROM operation_logs`,
	).Scan(&s.Total, &s.Today, &s.TodayFailed, &s.Failed)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Modules 日志中出现过的模块（下拉筛选用）。
func (r *OperationLogRepository) Modules(
	ctx context.Context,
) ([]models.OperationLogOption, error) {
	return r.options(ctx, `
		SELECT module, MAX(module_name), COUNT(*)
		FROM operation_logs
		GROUP BY module
		ORDER BY COUNT(*) DESC, module ASC`)
}

// Actions 日志中出现过的动作（下拉筛选用）。
func (r *OperationLogRepository) Actions(
	ctx context.Context,
) ([]models.OperationLogOption, error) {
	return r.options(ctx, `
		SELECT action, MAX(action_name), COUNT(*)
		FROM operation_logs
		GROUP BY action
		ORDER BY COUNT(*) DESC, action ASC`)
}

func (r *OperationLogRepository) options(
	ctx context.Context,
	query string,
) ([]models.OperationLogOption, error) {
	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.OperationLogOption, 0)
	for rows.Next() {
		var o models.OperationLogOption
		if err := rows.Scan(&o.Value, &o.Label, &o.Count); err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

// DeleteBefore 删除 before 之前（不含）的日志，返回删除条数。
func (r *OperationLogRepository) DeleteBefore(
	ctx context.Context,
	before time.Time,
) (int64, error) {
	result, err := r.DB.ExecContext(
		ctx,
		`DELETE FROM operation_logs WHERE created_at < ?`,
		before,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

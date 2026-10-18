package repository

import (
	"context"
	"database/sql"
	"strings"

	"OrderAI/models"
)

// MaterialRepository 读写材料主数据（包材/原材料）。
type MaterialRepository struct {
	DB *sql.DB
}

func NewMaterialRepository(db *sql.DB) *MaterialRepository {
	return &MaterialRepository{DB: db}
}

const materialColumns = `
	id, code, name, name_zh, short_name, category, spec, unit,
	price, currency, supplier_name, safety_stock, status, remark,
	created_at, updated_at
`

func scanMaterial(row interface {
	Scan(...any) error
}) (*models.Material, error) {
	m := &models.Material{}
	err := row.Scan(
		&m.ID, &m.Code, &m.Name, &m.NameZH, &m.ShortName,
		&m.Category, &m.Spec, &m.Unit,
		&m.Price, &m.Currency, &m.SupplierName, &m.SafetyStock,
		&m.Status, &m.Remark, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// List 查询材料，支持关键字与分类过滤。
func (r *MaterialRepository) List(
	ctx context.Context,
	keyword, category string,
) ([]models.Material, error) {
	query := `SELECT ` + materialColumns + ` FROM materials WHERE 1=1`
	args := []any{}

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query += ` AND (
			code LIKE ? OR name LIKE ? OR name_zh LIKE ? OR
			short_name LIKE ? OR supplier_name LIKE ?
		)`
		k := "%" + keyword + "%"
		args = append(args, k, k, k, k, k)
	}

	if category = strings.TrimSpace(category); category != "" {
		query += ` AND category = ?`
		args = append(args, category)
	}

	query += ` ORDER BY category ASC, id DESC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.Material, 0)
	for rows.Next() {
		m, err := scanMaterial(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *m)
	}

	return result, rows.Err()
}

// PricesByIDs 批量取材料的 PI 箱费单价（materials.price），返回 id -> 单价。
func (r *MaterialRepository) PricesByIDs(
	ctx context.Context,
	ids []uint64,
) (map[uint64]float64, error) {
	result := make(map[uint64]float64, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, price FROM materials
		WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uint64
		var price float64
		if err := rows.Scan(&id, &price); err != nil {
			return nil, err
		}
		result[id] = price
	}
	return result, rows.Err()
}

func (r *MaterialRepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.Material, error) {
	m, err := scanMaterial(r.DB.QueryRowContext(
		ctx,
		`SELECT `+materialColumns+` FROM materials WHERE id = ?`,
		id,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrMaterialNotFound
		}
		return nil, err
	}
	return m, nil
}

// GetByCode 按编码查询材料（用于下推时复用已自动建好的纸箱等材料）。
func (r *MaterialRepository) GetByCode(
	ctx context.Context,
	code string,
) (*models.Material, error) {
	m, err := scanMaterial(r.DB.QueryRowContext(
		ctx,
		`SELECT `+materialColumns+` FROM materials WHERE code = ?`,
		code,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrMaterialNotFound
		}
		return nil, err
	}
	return m, nil
}

func normalizeMaterial(m *models.Material) {
	m.Code = strings.TrimSpace(m.Code)
	m.Name = strings.TrimSpace(m.Name)
	m.NameZH = strings.TrimSpace(m.NameZH)
	m.ShortName = strings.TrimSpace(m.ShortName)
	m.Spec = strings.TrimSpace(m.Spec)
	m.SupplierName = strings.TrimSpace(m.SupplierName)

	if m.Category == "" {
		m.Category = models.MaterialCategoryPackaging
	}
	if m.Unit == "" {
		m.Unit = "pcs"
	}
	if m.Currency == "" {
		m.Currency = "CNY"
	}
	if m.Status == "" {
		m.Status = "active"
	}
}

func (r *MaterialRepository) Create(
	ctx context.Context,
	m *models.Material,
) error {
	normalizeMaterial(m)

	result, err := r.DB.ExecContext(ctx, `
		INSERT INTO materials (
			code, name, name_zh, short_name, category, spec, unit,
			price, currency, supplier_name, safety_stock, status, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		m.Code, m.Name, m.NameZH, m.ShortName, m.Category, m.Spec, m.Unit,
		m.Price, m.Currency, m.SupplierName, m.SafetyStock, m.Status, m.Remark,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	m.ID = uint64(id)
	return nil
}

func (r *MaterialRepository) Update(
	ctx context.Context,
	m *models.Material,
) error {
	normalizeMaterial(m)

	result, err := r.DB.ExecContext(ctx, `
		UPDATE materials SET
			code = ?, name = ?, name_zh = ?, short_name = ?,
			category = ?, spec = ?, unit = ?, price = ?, currency = ?,
			supplier_name = ?, safety_stock = ?, status = ?, remark = ?
		WHERE id = ?
	`,
		m.Code, m.Name, m.NameZH, m.ShortName,
		m.Category, m.Spec, m.Unit, m.Price, m.Currency,
		m.SupplierName, m.SafetyStock, m.Status, m.Remark,
		m.ID,
	)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		_, err = r.GetByID(ctx, m.ID)
		return err
	}
	return nil
}

func (r *MaterialRepository) Delete(
	ctx context.Context,
	id uint64,
) error {
	result, err := r.DB.ExecContext(
		ctx, `DELETE FROM materials WHERE id = ?`, id,
	)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return models.ErrMaterialNotFound
	}
	return nil
}

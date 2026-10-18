package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"OrderAI/models"
)

// ProductCategoryRepository 读写报关品类（申报要素主数据）。
type ProductCategoryRepository struct {
	DB *sql.DB
}

func NewProductCategoryRepository(db *sql.DB) *ProductCategoryRepository {
	return &ProductCategoryRepository{DB: db}
}

const productCategoryColumns = `
	c.id, c.code, c.name_en, c.name_zh, c.material, c.hs_code, c.purpose,
	c.brand, c.model, c.product_url, c.declare_price, c.image, c.remark,
	c.status, c.created_at, c.updated_at
`

func scanProductCategory(row interface {
	Scan(...any) error
}) (*models.ProductCategory, error) {
	c := &models.ProductCategory{}
	err := row.Scan(
		&c.ID, &c.Code, &c.NameEN, &c.NameZH, &c.Material, &c.HSCode, &c.Purpose,
		&c.Brand, &c.Model, &c.ProductURL, &c.DeclarePrice, &c.Image, &c.Remark,
		&c.Status, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// List 查询报关品类（附关联 SKU 数）。
func (r *ProductCategoryRepository) List(
	ctx context.Context,
	keyword, status string,
) ([]models.ProductCategory, error) {
	query := `SELECT ` + productCategoryColumns + `,
		(SELECT COUNT(*) FROM skus s WHERE s.product_category_id = c.id)
		FROM product_categories c WHERE 1 = 1`
	args := []any{}

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		k := "%" + keyword + "%"
		query += ` AND (c.code LIKE ? OR c.name_en LIKE ? OR c.name_zh LIKE ? OR c.hs_code LIKE ?)`
		args = append(args, k, k, k, k)
	}
	if status = strings.TrimSpace(status); status != "" {
		query += ` AND c.status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY c.id DESC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ProductCategory, 0)
	for rows.Next() {
		c := &models.ProductCategory{}
		if err := rows.Scan(
			&c.ID, &c.Code, &c.NameEN, &c.NameZH, &c.Material, &c.HSCode, &c.Purpose,
			&c.Brand, &c.Model, &c.ProductURL, &c.DeclarePrice, &c.Image, &c.Remark,
			&c.Status, &c.CreatedAt, &c.UpdatedAt, &c.SKUCount,
		); err != nil {
			return nil, err
		}
		result = append(result, *c)
	}
	return result, rows.Err()
}

// GetByID 按主键查询。
func (r *ProductCategoryRepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.ProductCategory, error) {
	row := r.DB.QueryRowContext(ctx,
		`SELECT `+productCategoryColumns+` FROM product_categories c WHERE c.id = ?`, id)
	c, err := scanProductCategory(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("报关品类不存在")
		}
		return nil, err
	}
	return c, nil
}

// Create 新建报关品类。
func (r *ProductCategoryRepository) Create(
	ctx context.Context,
	c *models.ProductCategory,
) error {
	normalizeProductCategory(c)

	result, err := r.DB.ExecContext(ctx, `
		INSERT INTO product_categories (
			code, name_en, name_zh, material, hs_code, purpose,
			brand, model, product_url, declare_price, image, remark, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Code, c.NameEN, c.NameZH, c.Material, c.HSCode, c.Purpose,
		c.Brand, c.Model, c.ProductURL, c.DeclarePrice, c.Image, c.Remark, c.Status,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	c.ID = uint64(id)
	return nil
}

// Update 更新报关品类。
func (r *ProductCategoryRepository) Update(
	ctx context.Context,
	c *models.ProductCategory,
) error {
	normalizeProductCategory(c)

	result, err := r.DB.ExecContext(ctx, `
		UPDATE product_categories SET
			code = ?, name_en = ?, name_zh = ?, material = ?, hs_code = ?, purpose = ?,
			brand = ?, model = ?, product_url = ?, declare_price = ?, image = ?,
			remark = ?, status = ?
		WHERE id = ?`,
		c.Code, c.NameEN, c.NameZH, c.Material, c.HSCode, c.Purpose,
		c.Brand, c.Model, c.ProductURL, c.DeclarePrice, c.Image, c.Remark, c.Status,
		c.ID,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		if _, err := r.GetByID(ctx, c.ID); err != nil {
			return err
		}
	}
	return nil
}

// Delete 删除报关品类；仍被 SKU 引用时拒绝删除。
func (r *ProductCategoryRepository) Delete(ctx context.Context, id uint64) error {
	var used int
	if err := r.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM skus WHERE product_category_id = ?`, id,
	).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return fmt.Errorf("该品类已被 %d 个 SKU 引用，请先解除引用", used)
	}

	result, err := r.DB.ExecContext(ctx, `DELETE FROM product_categories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("报关品类不存在")
	}
	return nil
}

func normalizeProductCategory(c *models.ProductCategory) {
	c.Code = strings.TrimSpace(c.Code)
	c.NameEN = strings.TrimSpace(c.NameEN)
	c.NameZH = strings.TrimSpace(c.NameZH)
	c.Material = strings.TrimSpace(c.Material)
	c.HSCode = strings.TrimSpace(c.HSCode)
	c.Purpose = strings.TrimSpace(c.Purpose)
	c.Brand = strings.TrimSpace(c.Brand)
	c.Model = strings.TrimSpace(c.Model)
	c.ProductURL = strings.TrimSpace(c.ProductURL)
	c.Image = strings.TrimSpace(c.Image)
	c.Remark = strings.TrimSpace(c.Remark)
	if c.Brand == "" {
		c.Brand = "无"
	}
	if c.Model == "" {
		c.Model = "无"
	}
	if c.Status == "" {
		c.Status = "active"
	}
}

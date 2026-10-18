package repository

import (
	"context"
	"database/sql"
	"strings"

	"OrderAI/models"
)

// ProductBomRepository 读写产品用料清单（BOM）。
type ProductBomRepository struct {
	DB *sql.DB
}

func NewProductBomRepository(db *sql.DB) *ProductBomRepository {
	return &ProductBomRepository{DB: db}
}

// List 查询用料清单，附带产品名与材料信息。
func (r *ProductBomRepository) List(
	ctx context.Context,
	productType, keyword string,
) ([]models.ProductBom, error) {
	query := `
		SELECT
			b.id, b.product_type, b.product_ref_id, b.material_id,
			b.quantity_per_unit, b.unit, b.remark, b.status,
			b.created_at, b.updated_at,
			COALESCE(NULLIF(r.product_name_zh, ''), NULLIF(r.customer_product_name, ''), NULLIF(s.product_name_zh, ''), NULLIF(s.product_name, ''), '') AS product_name,
			COALESCE(m.code, '') AS material_code,
			COALESCE(NULLIF(m.short_name, ''), NULLIF(m.name_zh, ''), m.name, '') AS material_name,
			COALESCE(m.spec, '') AS material_spec
		FROM product_bom b
		LEFT JOIN materials m ON m.id = b.material_id
		LEFT JOIN product_match_rules r
			ON b.product_type = 'rule' AND r.id = b.product_ref_id
		LEFT JOIN skus s
			ON b.product_type = 'sku' AND s.id = b.product_ref_id
		WHERE 1 = 1`
	args := []any{}

	if productType = strings.TrimSpace(productType); productType != "" {
		query += ` AND b.product_type = ?`
		args = append(args, productType)
	}

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		k := "%" + keyword + "%"
		query += ` AND (
			m.code LIKE ? OR m.name_zh LIKE ? OR m.name LIKE ? OR
			r.product_name_zh LIKE ? OR r.rule_code LIKE ? OR
			s.product_name_zh LIKE ? OR s.sku_code LIKE ?
		)`
		args = append(args, k, k, k, k, k, k, k)
	}

	query += ` ORDER BY b.product_type ASC, b.product_ref_id ASC, b.id ASC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ProductBom, 0)
	for rows.Next() {
		var b models.ProductBom
		if err := rows.Scan(
			&b.ID, &b.ProductType, &b.ProductRefID, &b.MaterialID,
			&b.QuantityPerUnit, &b.Unit, &b.Remark, &b.Status,
			&b.CreatedAt, &b.UpdatedAt,
			&b.ProductName, &b.MaterialCode, &b.MaterialName, &b.MaterialSpec,
		); err != nil {
			return nil, err
		}
		result = append(result, b)
	}

	return result, rows.Err()
}

// GetByID 查询单条用料记录（含展示信息）。
func (r *ProductBomRepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.ProductBom, error) {
	items, err := r.queryOne(ctx, id)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *ProductBomRepository) queryOne(
	ctx context.Context,
	id uint64,
) (*models.ProductBom, error) {
	var b models.ProductBom
	err := r.DB.QueryRowContext(ctx, `
		SELECT
			b.id, b.product_type, b.product_ref_id, b.material_id,
			b.quantity_per_unit, b.unit, b.remark, b.status,
			b.created_at, b.updated_at,
			COALESCE(NULLIF(r.product_name_zh, ''), NULLIF(r.customer_product_name, ''), NULLIF(s.product_name_zh, ''), NULLIF(s.product_name, ''), '') AS product_name,
			COALESCE(m.code, '') AS material_code,
			COALESCE(NULLIF(m.short_name, ''), NULLIF(m.name_zh, ''), m.name, '') AS material_name,
			COALESCE(m.spec, '') AS material_spec
		FROM product_bom b
		LEFT JOIN materials m ON m.id = b.material_id
		LEFT JOIN product_match_rules r
			ON b.product_type = 'rule' AND r.id = b.product_ref_id
		LEFT JOIN skus s
			ON b.product_type = 'sku' AND s.id = b.product_ref_id
		WHERE b.id = ?`, id,
	).Scan(
		&b.ID, &b.ProductType, &b.ProductRefID, &b.MaterialID,
		&b.QuantityPerUnit, &b.Unit, &b.Remark, &b.Status,
		&b.CreatedAt, &b.UpdatedAt,
		&b.ProductName, &b.MaterialCode, &b.MaterialName, &b.MaterialSpec,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrProductBomNotFound
		}
		return nil, err
	}
	return &b, nil
}

// ListByProducts 批量查询指定产品的用料清单，供生产完工时计算材料消耗。
func (r *ProductBomRepository) ListByProducts(
	ctx context.Context,
	pairs []ProductRef,
) (map[ProductRef][]models.ProductBom, error) {
	return r.listByProducts(ctx, pairs, "")
}

// ListPackagingByProducts 批量查询指定产品的**包装类**用料（category = packaging）。
//
// 用于发货时自动核销包装材料：纸箱等包材随货发出即被消耗，
// 不应再依赖人工去领料页面点一次，否则漏点就会长期挂账。
func (r *ProductBomRepository) ListPackagingByProducts(
	ctx context.Context,
	pairs []ProductRef,
) (map[ProductRef][]models.ProductBom, error) {
	return r.listByProducts(ctx, pairs, models.MaterialCategoryPackaging)
}

// listByProducts 批量查询用料清单，category 非空时只返回该分类的材料。
func (r *ProductBomRepository) listByProducts(
	ctx context.Context,
	pairs []ProductRef,
	category string,
) (map[ProductRef][]models.ProductBom, error) {
	result := make(map[ProductRef][]models.ProductBom)
	if len(pairs) == 0 {
		return result, nil
	}

	conditions := make([]string, 0, len(pairs))
	args := make([]any, 0, len(pairs)*2+1)
	for _, p := range pairs {
		conditions = append(conditions, "(b.product_type = ? AND b.product_ref_id = ?)")
		args = append(args, p.Type, p.RefID)
	}

	query := `
		SELECT
			b.id, b.product_type, b.product_ref_id, b.material_id,
			b.quantity_per_unit, b.unit, b.remark, b.status,
			b.created_at, b.updated_at,
			'' AS product_name,
			COALESCE(m.code, ''),
			COALESCE(NULLIF(m.short_name, ''), NULLIF(m.name_zh, ''), m.name, ''),
			COALESCE(m.spec, ''),
			COALESCE(m.category, '')
		FROM product_bom b
		LEFT JOIN materials m ON m.id = b.material_id
		WHERE b.status = 'active' AND (` + strings.Join(conditions, " OR ") + `)`
	if category != "" {
		query += ` AND m.category = ?`
		args = append(args, category)
	}
	query += ` ORDER BY b.id ASC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var b models.ProductBom
		if err := rows.Scan(
			&b.ID, &b.ProductType, &b.ProductRefID, &b.MaterialID,
			&b.QuantityPerUnit, &b.Unit, &b.Remark, &b.Status,
			&b.CreatedAt, &b.UpdatedAt,
			&b.ProductName, &b.MaterialCode, &b.MaterialName, &b.MaterialSpec,
			&b.MaterialCategory,
		); err != nil {
			return nil, err
		}
		key := ProductRef{Type: b.ProductType, RefID: b.ProductRefID}
		result[key] = append(result[key], b)
	}

	return result, rows.Err()
}

// ProductRef 产品标识（类型 + 引用 ID）。
type ProductRef struct {
	Type  string
	RefID uint64
}

func normalizeBom(b *models.ProductBom) {
	b.ProductType = strings.TrimSpace(b.ProductType)
	b.Unit = strings.TrimSpace(b.Unit)
	b.Remark = strings.TrimSpace(b.Remark)

	if b.ProductType == "" {
		b.ProductType = models.ProductionProductRule
	}
	if b.Status == "" {
		b.Status = "active"
	}
}

// Upsert 按 (产品, 材料) 唯一键写入或更新用料记录（用于下推时补全用料）。
func (r *ProductBomRepository) Upsert(
	ctx context.Context,
	b *models.ProductBom,
) error {
	normalizeBom(b)

	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO product_bom (
			product_type, product_ref_id, material_id,
			quantity_per_unit, unit, remark, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			quantity_per_unit = VALUES(quantity_per_unit),
			unit = VALUES(unit),
			remark = VALUES(remark),
			status = VALUES(status)`,
		b.ProductType, b.ProductRefID, b.MaterialID,
		b.QuantityPerUnit, b.Unit, b.Remark, b.Status,
	)
	return err
}

// Create 新增用料记录。
func (r *ProductBomRepository) Create(
	ctx context.Context,
	b *models.ProductBom,
) error {
	normalizeBom(b)

	result, err := r.DB.ExecContext(ctx, `
		INSERT INTO product_bom (
			product_type, product_ref_id, material_id,
			quantity_per_unit, unit, remark, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b.ProductType, b.ProductRefID, b.MaterialID,
		b.QuantityPerUnit, b.Unit, b.Remark, b.Status,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	b.ID = uint64(id)
	return nil
}

// Update 更新用料记录。
func (r *ProductBomRepository) Update(
	ctx context.Context,
	b *models.ProductBom,
) error {
	normalizeBom(b)

	result, err := r.DB.ExecContext(ctx, `
		UPDATE product_bom SET
			product_type = ?, product_ref_id = ?, material_id = ?,
			quantity_per_unit = ?, unit = ?, remark = ?, status = ?
		WHERE id = ?`,
		b.ProductType, b.ProductRefID, b.MaterialID,
		b.QuantityPerUnit, b.Unit, b.Remark, b.Status,
		b.ID,
	)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		if _, err := r.queryOne(ctx, b.ID); err != nil {
			return err
		}
	}
	return nil
}

// Delete 删除用料记录。
func (r *ProductBomRepository) Delete(ctx context.Context, id uint64) error {
	result, err := r.DB.ExecContext(ctx, `DELETE FROM product_bom WHERE id = ?`, id)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return models.ErrProductBomNotFound
	}
	return nil
}

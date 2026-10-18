package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"OrderAI/models"
)

type SKURepository struct {
	DB *sql.DB
}

func NewSKURepository(db *sql.DB) *SKURepository {
	return &SKURepository{DB: db}
}

const skuColumns = `
	id, sort_order, sku_code, product_name, product_name_zh, short_name, series,
	size_spec, pi_color, model,
	price, price_usd, declare_price, currency, image, color, pack_quantity, unit,
	box_length, box_width, box_height, box_unit,
	box_weight, weight_unit, hs_code, category, asin, fnsku, product_category_id, status, description, remark,
	created_at, updated_at
`

func scanSKU(row interface {
	Scan(...any) error
}) (*models.SKU, error) {
	s := &models.SKU{}
	err := row.Scan(
		&s.ID, &s.SortOrder, &s.SKUCode, &s.ProductName, &s.ProductNameZH,
		&s.ShortName, &s.Series, &s.SizeSpec, &s.PiColor, &s.Model,
		&s.Price, &s.PriceUSD, &s.DeclarePrice, &s.Currency, &s.Image, &s.Color,
		&s.PackQuantity, &s.Unit,
		&s.BoxLength, &s.BoxWidth, &s.BoxHeight,
		&s.BoxUnit, &s.BoxWeight, &s.WeightUnit,
		&s.HSCode, &s.Category, &s.ASIN, &s.FNSKU, &s.ProductCategoryID, &s.Status,
		&s.Description, &s.Remark, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *SKURepository) List(
	ctx context.Context,
	keyword, status string,
) ([]models.SKU, error) {
	query := `SELECT ` + skuColumns + ` FROM skus WHERE 1=1`
	args := []any{}

	if keyword != "" {
		query += ` AND (
			sku_code LIKE ? OR
			product_name LIKE ? OR
			product_name_zh LIKE ? OR
			short_name LIKE ? OR
			model LIKE ? OR color LIKE ?
		)`
		k := "%" + keyword + "%"
		args = append(args, k, k, k, k, k, k)
	}

	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}

	// 产品列表统一按 sort_order 升序（同值再按 id 保证顺序稳定）。
	query += ` ORDER BY sort_order ASC, id ASC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.SKU, 0)
	for rows.Next() {
		s, err := scanSKU(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *s)
	}

	return result, rows.Err()
}

// FindByProductNameKeys 按 skus.product_name 批量精确匹配。
//
// 用于订单行与单品 SKU 的直配：
// CONCAT(order_items.product_name, ' ', order_items.specification) = skus.product_name
func (r *SKURepository) FindByProductNameKeys(
	ctx context.Context,
	keys []string,
) ([]models.SKU, error) {
	if len(keys) == 0 {
		return []models.SKU{}, nil
	}

	placeholders := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys))

	for _, key := range keys {
		placeholders = append(placeholders, "?")
		args = append(args, key)
	}

	query := `SELECT ` + skuColumns + ` FROM skus
		WHERE product_name IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY id ASC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.SKU, 0, len(keys))
	for rows.Next() {
		s, err := scanSKU(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *s)
	}

	return result, rows.Err()
}

// ProductNamesByIDs 批量取 SKU 的英文产品名（skus.product_name），返回 id -> name。
func (r *SKURepository) ProductNamesByIDs(
	ctx context.Context,
	ids []uint64,
) (map[uint64]string, error) {
	result := make(map[uint64]string, len(ids))
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
		SELECT id, product_name FROM skus
		WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uint64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		result[id] = name
	}
	return result, rows.Err()
}

// PriceUSDByIDs 批量取 SKU 的 PI 美金售价（skus.price_usd），返回 id -> 单价。
func (r *SKURepository) PriceUSDByIDs(
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
		SELECT id, price_usd FROM skus
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

// ImagesByIDs 批量取 SKU 的产品图片路径（skus.image），返回 id -> 路径。
func (r *SKURepository) ImagesByIDs(
	ctx context.Context,
	ids []uint64,
) (map[uint64]string, error) {
	result := make(map[uint64]string, len(ids))
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
		SELECT id, image FROM skus
		WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uint64
		var image string
		if err := rows.Scan(&id, &image); err != nil {
			return nil, err
		}
		result[id] = image
	}
	return result, rows.Err()
}

// ProductNamesByCodes 批量按 SKU 编码取英文产品名（skus.product_name），返回 code -> name。
func (r *SKURepository) ProductNamesByCodes(
	ctx context.Context,
	codes []string,
) (map[string]string, error) {
	result := make(map[string]string, len(codes))
	if len(codes) == 0 {
		return result, nil
	}

	placeholders := make([]string, 0, len(codes))
	args := make([]any, 0, len(codes))
	for _, code := range codes {
		placeholders = append(placeholders, "?")
		args = append(args, code)
	}

	rows, err := r.DB.QueryContext(ctx, `
		SELECT sku_code, product_name FROM skus
		WHERE sku_code IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var code, name string
		if err := rows.Scan(&code, &name); err != nil {
			return nil, err
		}
		result[code] = name
	}
	return result, rows.Err()
}

func (r *SKURepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.SKU, error) {
	s, err := scanSKU(r.DB.QueryRowContext(
		ctx,
		`SELECT `+skuColumns+` FROM skus WHERE id = ?`,
		id,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("SKU不存在")
		}
		return nil, err
	}
	return s, nil
}

func normalizeSKU(s *models.SKU) {
	s.SKUCode = strings.TrimSpace(s.SKUCode)
	s.ProductName = strings.TrimSpace(s.ProductName)
	s.ProductNameZH = strings.TrimSpace(s.ProductNameZH)
	s.ShortName = strings.TrimSpace(s.ShortName)
	s.Series = strings.TrimSpace(s.Series)
	s.SizeSpec = strings.TrimSpace(s.SizeSpec)
	s.PiColor = strings.TrimSpace(s.PiColor)
	s.Model = strings.TrimSpace(s.Model)
	s.Color = strings.TrimSpace(s.Color)
	s.HSCode = strings.TrimSpace(s.HSCode)

	if s.Currency == "" {
		s.Currency = "CNY"
	}
	if s.Unit == "" {
		s.Unit = "pcs"
	}
	if s.Category == "" {
		s.Category = "finished"
	}
	if s.BoxUnit == "" {
		s.BoxUnit = "cm"
	}
	if s.WeightUnit == "" {
		s.WeightUnit = "kg"
	}
	if s.Status == "" {
		s.Status = "active"
	}
	if s.PackQuantity <= 0 {
		s.PackQuantity = 1
	}
}

func (r *SKURepository) Create(
	ctx context.Context,
	s *models.SKU,
) error {
	normalizeSKU(s)

	result, err := r.DB.ExecContext(ctx, `
		INSERT INTO skus (
			sort_order, sku_code, product_name, product_name_zh, short_name, series,
			size_spec, pi_color, model,
			price, price_usd, declare_price, currency, image, color, pack_quantity, unit,
			box_length, box_width, box_height, box_unit,
			box_weight, weight_unit, hs_code, category, asin, fnsku, product_category_id, status, description, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		s.SortOrder, s.SKUCode, s.ProductName, s.ProductNameZH, s.ShortName, s.Series,
		s.SizeSpec, s.PiColor, s.Model,
		s.Price, s.PriceUSD, s.DeclarePrice, s.Currency, s.Image, s.Color, s.PackQuantity, s.Unit,
		s.BoxLength, s.BoxWidth, s.BoxHeight, s.BoxUnit,
		s.BoxWeight, s.WeightUnit, s.HSCode, s.Category, s.ASIN, s.FNSKU, s.ProductCategoryID, s.Status,
		s.Description, s.Remark,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	s.ID = uint64(id)
	return nil
}

func (r *SKURepository) Update(
	ctx context.Context,
	s *models.SKU,
) error {
	normalizeSKU(s)

	result, err := r.DB.ExecContext(ctx, `
		UPDATE skus SET
			sort_order = ?,
			sku_code = ?, product_name = ?, product_name_zh = ?,
			short_name = ?, series = ?, size_spec = ?, pi_color = ?,
			model = ?, price = ?, price_usd = ?, declare_price = ?, currency = ?, image = ?, color = ?,
			pack_quantity = ?, unit = ?, box_length = ?, box_width = ?,
			box_height = ?, box_unit = ?, box_weight = ?,
			weight_unit = ?, hs_code = ?, category = ?, asin = ?, fnsku = ?, product_category_id = ?,
			status = ?, description = ?, remark = ?
		WHERE id = ?
	`,
		s.SortOrder, s.SKUCode, s.ProductName, s.ProductNameZH,
		s.ShortName, s.Series, s.SizeSpec, s.PiColor,
		s.Model, s.Price, s.PriceUSD, s.DeclarePrice, s.Currency, s.Image, s.Color,
		s.PackQuantity, s.Unit, s.BoxLength, s.BoxWidth,
		s.BoxHeight, s.BoxUnit, s.BoxWeight,
		s.WeightUnit, s.HSCode, s.Category, s.ASIN, s.FNSKU, s.ProductCategoryID,
		s.Status, s.Description, s.Remark,
		s.ID,
	)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		_, err = r.GetByID(ctx, s.ID)
		return err
	}
	return nil
}

func (r *SKURepository) Delete(
	ctx context.Context,
	id uint64,
) error {
	result, err := r.DB.ExecContext(
		ctx, `DELETE FROM skus WHERE id = ?`, id,
	)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("SKU不存在")
	}
	return nil
}

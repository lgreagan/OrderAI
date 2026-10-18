package repository

import (
	"OrderAI/models"
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type ProductMatchRepository struct {
	DB *sql.DB
}

func NewProductMatchRepository(db *sql.DB) *ProductMatchRepository {
	return &ProductMatchRepository{DB: db}
}

const matchRuleColumns = `
	id, rule_code, customer_product_name, customer_specification,
	product_name_zh, order_unit, sets_per_carton, carton_weight, box_length, box_width, box_height,
	status, remark, created_at, updated_at
`

const matchRuleItemColumns = `
	id, rule_id, sku_id, quantity_per_set, sort_order, remark, created_at, updated_at
`

const matchRuleItemDetailColumns = `
	i.id, i.rule_id, i.sku_id, i.quantity_per_set,
	i.sort_order, i.remark, i.created_at, i.updated_at,
	s.sku_code, s.product_name, s.product_name_zh, s.model, s.color, s.status
`

func scanProductMatchRule(row interface {
	Scan(...any) error
}) (*models.ProductMatchRule, error) {
	m := &models.ProductMatchRule{}
	err := row.Scan(
		&m.ID, &m.RuleCode, &m.CustomerProductName, &m.CustomerSpecification,
		&m.ProductNameZH, &m.OrderUnit, &m.SetsPerCarton,
		&m.CartonWeight, &m.BoxLength, &m.BoxWidth, &m.BoxHeight,
		&m.Status, &m.Remark,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func scanProductMatchRuleItem(row interface {
	Scan(...any) error
}) (*models.ProductMatchRuleItem, error) {
	mi := &models.ProductMatchRuleItem{}
	err := row.Scan(
		&mi.ID, &mi.RuleID, &mi.SkuID, &mi.QuantityPerSet,
		&mi.SortOrder, &mi.Remark, &mi.CreatedAt, &mi.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return mi, nil
}

func scanProductMatchRuleItemDetail(row interface {
	Scan(...any) error
}) (*models.ProductMatchRuleItemDetail, error) {
	d := &models.ProductMatchRuleItemDetail{}
	err := row.Scan(
		&d.ID, &d.RuleID, &d.SkuID, &d.QuantityPerSet,
		&d.SortOrder, &d.Remark, &d.CreatedAt, &d.UpdatedAt,
		&d.SKUCode, &d.ProductName, &d.ProductNameZH, &d.Model, &d.Color,
		&d.Status,
	)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// ListItemsByRuleIDs 批量查询多条规则的组成明细（含单品 SKU 信息）。
func (r *ProductMatchRepository) ListItemsByRuleIDs(
	ctx context.Context,
	ruleIDs []uint64,
) (map[uint64][]models.ProductMatchRuleItemDetail, error) {
	result := make(map[uint64][]models.ProductMatchRuleItemDetail, len(ruleIDs))
	if len(ruleIDs) == 0 {
		return result, nil
	}

	placeholders := make([]string, 0, len(ruleIDs))
	args := make([]any, 0, len(ruleIDs))
	for _, id := range ruleIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	query := `SELECT ` + matchRuleItemDetailColumns + `
		FROM product_match_rule_items i
		JOIN skus s ON s.id = i.sku_id
		WHERE i.rule_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY i.rule_id ASC, i.sort_order ASC, i.id ASC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		d, err := scanProductMatchRuleItemDetail(rows)
		if err != nil {
			return nil, err
		}
		result[d.RuleID] = append(result[d.RuleID], *d)
	}
	return result, rows.Err()
}

// ListRule 主规则列表，keyword检索 rule_code/customer_product_name/customer_specification
func (r *ProductMatchRepository) ListRule(
	ctx context.Context,
	keyword, status string,
) ([]models.ProductMatchRule, error) {
	query := `SELECT ` + matchRuleColumns + ` FROM product_match_rules WHERE 1=1`
	args := []any{}
	if keyword != "" {
		query += ` AND (
			rule_code LIKE ? OR
			customer_product_name LIKE ? OR
			customer_specification LIKE ?
		)`
		k := "%" + keyword + "%"
		args = append(args, k, k, k)
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY id DESC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ProductMatchRule, 0)
	for rows.Next() {
		m, err := scanProductMatchRule(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *m)
	}
	return result, rows.Err()
}

// FindActiveRulesByFullNames 按「客户商品名称 + 空格 + 客户规格」批量精确匹配启用中的组合规则。
//
// 订单行必须与该拼接结果完全一致才算命中组合规则，不做按名称降级匹配。
func (r *ProductMatchRepository) FindActiveRulesByFullNames(
	ctx context.Context,
	keys []string,
) ([]models.ProductMatchRule, error) {
	if len(keys) == 0 {
		return []models.ProductMatchRule{}, nil
	}

	placeholders := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys))

	for _, key := range keys {
		placeholders = append(placeholders, "?")
		args = append(args, key)
	}

	query := `SELECT ` + matchRuleColumns + ` FROM product_match_rules
		WHERE status = 'active'
		AND CONCAT(customer_product_name, ' ', customer_specification)
			IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY id ASC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ProductMatchRule, 0, len(keys))
	for rows.Next() {
		m, err := scanProductMatchRule(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *m)
	}
	return result, rows.Err()
}

func (r *ProductMatchRepository) GetRuleByID(
	ctx context.Context,
	id uint64,
) (*models.ProductMatchRule, error) {
	m, err := scanProductMatchRule(r.DB.QueryRowContext(
		ctx,
		`SELECT `+matchRuleColumns+` FROM product_match_rules WHERE id = ?`,
		id,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrProductMatchRuleNotFound
		}
		return nil, err
	}
	return m, nil
}

func normalizeProductMatchRule(m *models.ProductMatchRule) {
	m.RuleCode = strings.TrimSpace(m.RuleCode)
	m.CustomerProductName = strings.TrimSpace(m.CustomerProductName)
	m.CustomerSpecification = strings.TrimSpace(m.CustomerSpecification)
	m.ProductNameZH = strings.TrimSpace(m.ProductNameZH)
	m.OrderUnit = strings.TrimSpace(m.OrderUnit)
	if m.OrderUnit == "" {
		m.OrderUnit = "套"
	}
	if m.SetsPerCarton <= 0 {
		m.SetsPerCarton = 40
	}
	if m.CartonWeight < 0 {
		m.CartonWeight = 0
	}
	if m.Status == "" {
		m.Status = "active"
	}
}

// CreateRule 创建主规则
func (r *ProductMatchRepository) CreateRule(
	ctx context.Context,
	m *models.ProductMatchRule,
) error {
	normalizeProductMatchRule(m)
	result, err := r.DB.ExecContext(ctx, `
		INSERT INTO product_match_rules (
			rule_code, customer_product_name, customer_specification,
			product_name_zh, order_unit, sets_per_carton,
			carton_weight, box_length, box_width, box_height, status, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		m.RuleCode, m.CustomerProductName, m.CustomerSpecification,
		m.ProductNameZH, m.OrderUnit, m.SetsPerCarton,
		m.CartonWeight, m.BoxLength, m.BoxWidth, m.BoxHeight, m.Status, m.Remark,
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

// CreateRuleWithItems 在同一个事务中创建主规则与组成明细，
// 任一明细写入失败时整体回滚。
func (r *ProductMatchRepository) CreateRuleWithItems(
	ctx context.Context,
	m *models.ProductMatchRule,
	items []models.ProductMatchRuleItem,
) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	normalizeProductMatchRule(m)

	result, err := tx.ExecContext(ctx, `
		INSERT INTO product_match_rules (
			rule_code, customer_product_name, customer_specification,
			product_name_zh, order_unit, sets_per_carton,
			carton_weight, box_length, box_width, box_height, status, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		m.RuleCode, m.CustomerProductName, m.CustomerSpecification,
		m.ProductNameZH, m.OrderUnit, m.SetsPerCarton,
		m.CartonWeight, m.BoxLength, m.BoxWidth, m.BoxHeight, m.Status, m.Remark,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	m.ID = uint64(id)

	for i := range items {
		items[i].RuleID = m.ID
		normalizeProductMatchRuleItem(&items[i])
		if items[i].SortOrder <= 0 {
			items[i].SortOrder = i + 1
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO product_match_rule_items (
				rule_id, sku_id, quantity_per_set, sort_order, remark
			) VALUES (?, ?, ?, ?, ?)
		`,
			items[i].RuleID, items[i].SkuID, items[i].QuantityPerSet,
			items[i].SortOrder, items[i].Remark,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateRule 更新主规则
func (r *ProductMatchRepository) UpdateRule(
	ctx context.Context,
	m *models.ProductMatchRule,
) error {
	normalizeProductMatchRule(m)
	result, err := r.DB.ExecContext(ctx, `
		UPDATE product_match_rules SET
			rule_code = ?, customer_product_name = ?, customer_specification = ?,
			product_name_zh = ?, order_unit = ?, sets_per_carton = ?,
			carton_weight = ?, box_length = ?, box_width = ?, box_height = ?, status = ?, remark = ?
		WHERE id = ?
	`,
		m.RuleCode, m.CustomerProductName, m.CustomerSpecification,
		m.ProductNameZH, m.OrderUnit, m.SetsPerCarton,
		m.CartonWeight, m.BoxLength, m.BoxWidth, m.BoxHeight, m.Status, m.Remark,
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
		_, err = r.GetRuleByID(ctx, m.ID)
		return err
	}
	return nil
}

// UpdateRuleWithItems 在同一个事务中更新主规则与组成明细。
//
// 明细按提交内容整体替换：先删除该规则原有明细，再按顺序写入新明细，
// 任一步失败整体回滚。明细表没有外部引用，整体替换比逐条比对更安全。
func (r *ProductMatchRepository) UpdateRuleWithItems(
	ctx context.Context,
	m *models.ProductMatchRule,
	items []models.ProductMatchRuleItem,
) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	normalizeProductMatchRule(m)

	result, err := tx.ExecContext(ctx, `
		UPDATE product_match_rules SET
			rule_code = ?, customer_product_name = ?, customer_specification = ?,
			product_name_zh = ?, order_unit = ?, sets_per_carton = ?,
			carton_weight = ?, box_length = ?, box_width = ?, box_height = ?, status = ?, remark = ?
		WHERE id = ?
	`,
		m.RuleCode, m.CustomerProductName, m.CustomerSpecification,
		m.ProductNameZH, m.OrderUnit, m.SetsPerCarton,
		m.CartonWeight, m.BoxLength, m.BoxWidth, m.BoxHeight, m.Status, m.Remark,
		m.ID,
	)
	if err != nil {
		return err
	}

	// 提交内容与库中一致时影响行数为 0，需要确认规则确实存在。
	if _, err := result.RowsAffected(); err != nil {
		return err
	}

	var exists int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM product_match_rules WHERE id = ?`,
		m.ID,
	).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return models.ErrProductMatchRuleNotFound
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM product_match_rule_items WHERE rule_id = ?`,
		m.ID,
	); err != nil {
		return err
	}

	for i := range items {
		items[i].RuleID = m.ID
		normalizeProductMatchRuleItem(&items[i])
		if items[i].SortOrder <= 0 {
			items[i].SortOrder = i + 1
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO product_match_rule_items (
				rule_id, sku_id, quantity_per_set, sort_order, remark
			) VALUES (?, ?, ?, ?, ?)
		`,
			items[i].RuleID, items[i].SkuID, items[i].QuantityPerSet,
			items[i].SortOrder, items[i].Remark,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateRuleStatus 修改规则状态
func (r *ProductMatchRepository) UpdateRuleStatus(
	ctx context.Context,
	id uint64,
	status string,
) error {
	result, err := r.DB.ExecContext(ctx, `
		UPDATE product_match_rules SET status = ? WHERE id = ?
	`, status, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		_, err = r.GetRuleByID(ctx, id)
		return err
	}
	return nil
}

// CountRuleReferences 统计该规则被订单行匹配结果引用的数量。
//
// order_item_matches.rule_id 为 ON DELETE SET NULL，删除规则只会解除关联，
// 历史订单的快照字段（rule_code_snapshot 等）不受影响。
func (r *ProductMatchRepository) CountRuleReferences(
	ctx context.Context,
	id uint64,
) (int64, error) {
	var total int64
	err := r.DB.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM order_item_matches WHERE rule_id = ?`,
		id,
	).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// DeleteRule 删除主规则，数据库设置ON DELETE CASCADE，子项会自动删除
func (r *ProductMatchRepository) DeleteRule(
	ctx context.Context,
	id uint64,
) error {
	result, err := r.DB.ExecContext(
		ctx, `DELETE FROM product_match_rules WHERE id = ?`, id,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return models.ErrProductMatchRuleNotFound
	}
	return nil
}

// -------- Item 子项相关 --------

// ListItemByRuleID 根据rule_id查询所有子项
func (r *ProductMatchRepository) ListItemByRuleID(
	ctx context.Context,
	ruleID uint64,
) ([]models.ProductMatchRuleItem, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT `+matchRuleItemColumns+` FROM product_match_rule_items WHERE rule_id = ? ORDER BY sort_order ASC`,
		ruleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]models.ProductMatchRuleItem, 0)
	for rows.Next() {
		item, err := scanProductMatchRuleItem(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, rows.Err()
}

func (r *ProductMatchRepository) GetItemByID(
	ctx context.Context,
	id uint64,
) (*models.ProductMatchRuleItem, error) {
	item, err := scanProductMatchRuleItem(r.DB.QueryRowContext(
		ctx,
		`SELECT `+matchRuleItemColumns+` FROM product_match_rule_items WHERE id = ?`,
		id,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("规则子项不存在")
		}
		return nil, err
	}
	return item, nil
}

func normalizeProductMatchRuleItem(mi *models.ProductMatchRuleItem) {
	mi.Remark = strings.TrimSpace(mi.Remark)
	if mi.QuantityPerSet <= 0 {
		mi.QuantityPerSet = 1
	}
}

func (r *ProductMatchRepository) CreateItem(
	ctx context.Context,
	mi *models.ProductMatchRuleItem,
) error {
	normalizeProductMatchRuleItem(mi)
	result, err := r.DB.ExecContext(ctx, `
		INSERT INTO product_match_rule_items (
			rule_id, sku_id, quantity_per_set, sort_order, remark
		) VALUES (?, ?, ?, ?, ?)
	`,
		mi.RuleID, mi.SkuID, mi.QuantityPerSet, mi.SortOrder, mi.Remark,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	mi.ID = uint64(id)
	return nil
}

func (r *ProductMatchRepository) UpdateItem(
	ctx context.Context,
	mi *models.ProductMatchRuleItem,
) error {
	normalizeProductMatchRuleItem(mi)
	result, err := r.DB.ExecContext(ctx, `
		UPDATE product_match_rule_items SET
			rule_id = ?, sku_id = ?, quantity_per_set = ?, sort_order = ?, remark = ?
		WHERE id = ?
	`,
		mi.RuleID, mi.SkuID, mi.QuantityPerSet, mi.SortOrder, mi.Remark,
		mi.ID,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		_, err = r.GetItemByID(ctx, mi.ID)
		return err
	}
	return nil
}

func (r *ProductMatchRepository) DeleteItem(
	ctx context.Context,
	id uint64,
) error {
	result, err := r.DB.ExecContext(
		ctx, `DELETE FROM product_match_rule_items WHERE id = ?`, id,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("规则子项不存在")
	}
	return nil
}

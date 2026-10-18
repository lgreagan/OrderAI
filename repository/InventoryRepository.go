package repository

import (
	"context"
	"database/sql"
	"strings"

	"OrderAI/models"
)

// InventoryRepository 读写库存台账（流水）与结存。
type InventoryRepository struct {
	DB *sql.DB
}

func NewInventoryRepository(db *sql.DB) *InventoryRepository {
	return &InventoryRepository{DB: db}
}

// ListStock 查询库存结存，附带物品编码与名称。
func (r *InventoryRepository) ListStock(
	ctx context.Context,
	itemType, keyword string,
) ([]models.InventoryStock, error) {
	query := `
		SELECT
			v.item_type, v.item_id, v.warehouse, v.stock_qty,
			COALESCE(NULLIF(s.sku_code, ''), m.code, '') AS item_code,
			COALESCE(
				NULLIF(s.short_name, ''),
				NULLIF(s.product_name_zh, ''),
				m.name_zh, m.name, ''
			) AS item_name,
			COALESCE(NULLIF(s.unit, ''), m.unit, '') AS unit
		FROM v_inventory_stock v
		LEFT JOIN skus s
			ON v.item_type = 'sku' AND s.id = v.item_id
		LEFT JOIN materials m
			ON v.item_type = 'material' AND m.id = v.item_id
		WHERE 1 = 1`
	args := []any{}

	if itemType = strings.TrimSpace(itemType); itemType != "" {
		query += ` AND v.item_type = ?`
		args = append(args, itemType)
	}

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		k := "%" + keyword + "%"
		query += ` AND (
			s.sku_code LIKE ? OR s.product_name_zh LIKE ? OR s.short_name LIKE ?
			OR m.code LIKE ? OR m.name_zh LIKE ? OR m.name LIKE ?
		)`
		args = append(args, k, k, k, k, k, k)
	}

	query += ` ORDER BY v.item_type ASC, v.item_id ASC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.InventoryStock, 0)
	for rows.Next() {
		var it models.InventoryStock
		if err := rows.Scan(
			&it.ItemType, &it.ItemID, &it.Warehouse, &it.StockQuantity,
			&it.ItemCode, &it.ItemName, &it.Unit,
		); err != nil {
			return nil, err
		}
		result = append(result, it)
	}

	return result, rows.Err()
}

// ListTransactions 查询库存流水，最近的在前。
func (r *InventoryRepository) ListTransactions(
	ctx context.Context,
	itemType, keyword, bizType string,
	limit int,
) ([]models.InventoryTransaction, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	query := `
		SELECT
			t.id, t.item_type, t.item_id, t.direction, t.biz_type,
			t.quantity, t.unit_cost, t.currency, t.warehouse, t.batch_no,
			t.ref_type, t.ref_id, t.occurred_at, t.operator, t.remark,
			t.created_at,
			COALESCE(NULLIF(s.sku_code, ''), m.code, '') AS item_code,
			COALESCE(
				NULLIF(s.short_name, ''),
				NULLIF(s.product_name_zh, ''),
				m.name_zh, m.name, ''
			) AS item_name
		FROM inventory_transactions t
		LEFT JOIN skus s
			ON t.item_type = 'sku' AND s.id = t.item_id
		LEFT JOIN materials m
			ON t.item_type = 'material' AND m.id = t.item_id
		WHERE 1 = 1`
	args := []any{}

	if itemType = strings.TrimSpace(itemType); itemType != "" {
		query += ` AND t.item_type = ?`
		args = append(args, itemType)
	}

	if bizType = strings.TrimSpace(bizType); bizType != "" {
		query += ` AND t.biz_type = ?`
		args = append(args, bizType)
	}

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		k := "%" + keyword + "%"
		query += ` AND (
			s.sku_code LIKE ? OR s.product_name_zh LIKE ? OR s.short_name LIKE ?
			OR m.code LIKE ? OR m.name_zh LIKE ? OR m.name LIKE ?
		)`
		args = append(args, k, k, k, k, k, k)
	}

	query += ` ORDER BY t.id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.InventoryTransaction, 0)
	for rows.Next() {
		var t models.InventoryTransaction
		if err := rows.Scan(
			&t.ID, &t.ItemType, &t.ItemID, &t.Direction, &t.BizType,
			&t.Quantity, &t.UnitCost, &t.Currency, &t.Warehouse, &t.BatchNo,
			&t.RefType, &t.RefID, &t.OccurredAt, &t.Operator, &t.Remark,
			&t.CreatedAt, &t.ItemCode, &t.ItemName,
		); err != nil {
			return nil, err
		}
		result = append(result, t)
	}

	return result, rows.Err()
}

// StockByIDs 返回指定物品的结存（含编码与名称），按物品汇总。
func (r *InventoryRepository) StockByIDs(
	ctx context.Context,
	itemType string,
	ids []uint64,
) ([]models.InventoryStock, error) {
	result := make([]models.InventoryStock, 0)
	if len(ids) == 0 {
		return result, nil
	}

	ph := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, itemType)
	for _, id := range ids {
		ph = append(ph, "?")
		args = append(args, id)
	}

	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			v.item_type, v.item_id, MAX(v.warehouse), SUM(v.stock_qty),
			COALESCE(NULLIF(MAX(s.sku_code), ''), MAX(m.code), ''),
			COALESCE(
				NULLIF(MAX(s.short_name), ''),
				NULLIF(MAX(s.product_name_zh), ''),
				MAX(m.name_zh), MAX(m.name), ''
			)
		FROM v_inventory_stock v
		LEFT JOIN skus s ON v.item_type = 'sku' AND s.id = v.item_id
		LEFT JOIN materials m ON v.item_type = 'material' AND m.id = v.item_id
		WHERE v.item_type = ? AND v.item_id IN (`+strings.Join(ph, ",")+`)
		GROUP BY v.item_type, v.item_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var it models.InventoryStock
		if err := rows.Scan(
			&it.ItemType, &it.ItemID, &it.Warehouse, &it.StockQuantity,
			&it.ItemCode, &it.ItemName,
		); err != nil {
			return nil, err
		}
		result = append(result, it)
	}

	return result, rows.Err()
}

// DeleteByRef 删除某单据产生的库存流水（用于发货单重新保存时重算扣减）。
func (r *InventoryRepository) DeleteByRef(
	ctx context.Context,
	refType string,
	refID uint64,
) error {
	_, err := r.DB.ExecContext(
		ctx,
		`DELETE FROM inventory_transactions WHERE ref_type = ? AND ref_id = ?`,
		refType, refID,
	)
	return err
}

// DeleteByRefBiz 删除某单据产生的、指定业务类型的库存流水。
//
// 与 DeleteByRef 的区别：只清理单据自动生成的流水，保留人工产生的流水。
// 例如生产单的领料出库是人工操作的，重算或删单时不应被抹掉，否则材料库存会凭空回涨。
func (r *InventoryRepository) DeleteByRefBiz(
	ctx context.Context,
	refType string,
	refID uint64,
	bizTypes ...string,
) error {
	if len(bizTypes) == 0 {
		return nil
	}

	ph := make([]string, 0, len(bizTypes))
	args := make([]any, 0, len(bizTypes)+2)
	args = append(args, refType, refID)
	for _, b := range bizTypes {
		ph = append(ph, "?")
		args = append(args, b)
	}

	_, err := r.DB.ExecContext(
		ctx,
		`DELETE FROM inventory_transactions
		  WHERE ref_type = ? AND ref_id = ?
		    AND biz_type IN (`+strings.Join(ph, ",")+`)`,
		args...,
	)
	return err
}

// SumByRefs 按物品汇总若干单据在某业务类型下已出库的数量。
//
// 用于包材按「整单累计」核销时求差额：先算出整单应核销的总量，
// 再减去同一订单其它发货单已经核销的部分，只补差额。
func (r *InventoryRepository) SumByRefs(
	ctx context.Context,
	itemType, bizType, refType string,
	refIDs []uint64,
) (map[uint64]float64, error) {
	result := make(map[uint64]float64)
	if len(refIDs) == 0 {
		return result, nil
	}

	ph := make([]string, 0, len(refIDs))
	args := make([]any, 0, len(refIDs)+3)
	args = append(args, itemType, bizType, refType)
	for _, id := range refIDs {
		ph = append(ph, "?")
		args = append(args, id)
	}

	rows, err := r.DB.QueryContext(ctx, `
		SELECT item_id, COALESCE(SUM(quantity), 0)
		FROM inventory_transactions
		WHERE item_type = ? AND biz_type = ? AND ref_type = ?
		  AND ref_id IN (`+strings.Join(ph, ",")+`)
		GROUP BY item_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id  uint64
			qty float64
		)
		if err := rows.Scan(&id, &qty); err != nil {
			return nil, err
		}
		result[id] += qty
	}

	return result, rows.Err()
}

// Create 写入一条库存流水。
func (r *InventoryRepository) Create(
	ctx context.Context,
	t *models.InventoryTransaction,
) error {
	result, err := r.DB.ExecContext(ctx, `
		INSERT INTO inventory_transactions (
			item_type, item_id, direction, biz_type, quantity,
			unit_cost, currency, warehouse, batch_no,
			ref_type, ref_id, operator, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		t.ItemType, t.ItemID, t.Direction, t.BizType, t.Quantity,
		t.UnitCost, t.Currency, t.Warehouse, t.BatchNo,
		t.RefType, t.RefID, t.Operator, t.Remark,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	t.ID = uint64(id)
	return nil
}

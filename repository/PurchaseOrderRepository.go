package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"OrderAI/models"
)

// PurchaseOrderRepository 读写持久化的采购订单（头 + 明细）。
type PurchaseOrderRepository struct {
	DB *sql.DB
}

func NewPurchaseOrderRepository(db *sql.DB) *PurchaseOrderRepository {
	return &PurchaseOrderRepository{DB: db}
}

// List 查询采购单列表（含来源订单号与明细/到货统计）。
func (r *PurchaseOrderRepository) List(
	ctx context.Context,
	keyword, status string,
) ([]models.PurchaseOrderListItem, error) {
	query := `
		SELECT
			po.id, po.po_number, po.supplier_name, po.source_order_id,
			COALESCE(o.order_number, ''), po.order_date, po.expected_date,
			po.currency, po.status,
			po.goods_amount, po.freight_amount, po.total_amount,
			(SELECT COUNT(*) FROM purchase_order_items i
				WHERE i.purchase_order_id = po.id),
			(SELECT COUNT(*) FROM purchase_order_items i
				WHERE i.purchase_order_id = po.id
				  AND i.quantity > 0 AND i.received_quantity >= i.quantity),
			po.created_at
		FROM purchase_orders po
		LEFT JOIN orders o ON o.id = po.source_order_id
		WHERE 1 = 1`
	args := []any{}

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		k := "%" + keyword + "%"
		query += ` AND (
			po.po_number LIKE ? OR po.supplier_name LIKE ? OR
			o.order_number LIKE ? OR po.remark LIKE ?
		)`
		args = append(args, k, k, k, k)
	}

	if status = strings.TrimSpace(status); status != "" {
		query += ` AND po.status = ?`
		args = append(args, status)
	}

	query += ` ORDER BY po.id DESC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.PurchaseOrderListItem, 0)
	for rows.Next() {
		var it models.PurchaseOrderListItem
		if err := rows.Scan(
			&it.ID, &it.PONumber, &it.SupplierName, &it.SourceOrderID,
			&it.SourceOrderNumber, &it.OrderDate, &it.ExpectedDate,
			&it.Currency, &it.Status,
			&it.GoodsAmount, &it.FreightAmount, &it.TotalAmount,
			&it.ItemCount, &it.ReceivedCount,
			&it.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, it)
	}

	return result, rows.Err()
}

// ItemSummary 采购报表行：按物品汇总多张采购单的采购量、到货量与金额。
//
// 过滤条件与 List 保持一致（单号/供应商/订单号/备注 + 状态）。
func (r *PurchaseOrderRepository) ItemSummary(
	ctx context.Context,
	keyword, status string,
) ([]models.PurchaseItemSummary, error) {
	query := `
		SELECT
			i.item_type,
			i.item_code,
			MAX(i.item_name),
			MAX(i.spec),
			MAX(i.unit),
			SUM(i.required_quantity),
			SUM(i.quantity),
			SUM(i.received_quantity),
			SUM(i.amount)
		FROM purchase_order_items i
		JOIN purchase_orders po ON po.id = i.purchase_order_id
		LEFT JOIN orders o ON o.id = po.source_order_id
		WHERE 1 = 1`
	args := []any{}

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		k := "%" + keyword + "%"
		query += ` AND (
			po.po_number LIKE ? OR po.supplier_name LIKE ? OR
			o.order_number LIKE ? OR po.remark LIKE ?
		)`
		args = append(args, k, k, k, k)
	}

	if status = strings.TrimSpace(status); status != "" {
		query += ` AND po.status = ?`
		args = append(args, status)
	}

	query += `
		GROUP BY i.item_type, i.item_code
		ORDER BY i.item_type ASC, i.item_code ASC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.PurchaseItemSummary, 0)
	for rows.Next() {
		var it models.PurchaseItemSummary
		if err := rows.Scan(
			&it.ItemType, &it.ItemCode, &it.ItemName, &it.Spec, &it.Unit,
			&it.RequiredQuantity, &it.Quantity, &it.ReceivedQuantity, &it.Amount,
		); err != nil {
			return nil, err
		}
		result = append(result, it)
	}

	return result, rows.Err()
}

const purchaseItemColumns = `
	id, purchase_order_id, item_type, item_id, item_code, item_name,
	short_name, spec, unit, required_quantity, quantity, unit_price,
	amount, received_quantity, remark
`

func scanPurchaseItem(row interface {
	Scan(...any) error
}) (*models.PurchaseOrderItem, error) {
	it := &models.PurchaseOrderItem{}
	err := row.Scan(
		&it.ID, &it.PurchaseOrderID, &it.ItemType, &it.ItemID,
		&it.ItemCode, &it.ItemName, &it.ShortName, &it.Spec, &it.Unit,
		&it.RequiredQuantity, &it.Quantity, &it.UnitPrice,
		&it.Amount, &it.ReceivedQuantity, &it.Remark,
	)
	if err != nil {
		return nil, err
	}
	return it, nil
}

// ListItems 查询某采购单的明细行。
func (r *PurchaseOrderRepository) ListItems(
	ctx context.Context,
	poID uint64,
) ([]models.PurchaseOrderItem, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT `+purchaseItemColumns+`
		FROM purchase_order_items
		WHERE purchase_order_id = ?
		ORDER BY id ASC`, poID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.PurchaseOrderItem, 0)
	for rows.Next() {
		it, err := scanPurchaseItem(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *it)
	}

	return result, rows.Err()
}

// GetByID 查询采购单头 + 明细。
func (r *PurchaseOrderRepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.PurchaseOrder, error) {
	po := &models.PurchaseOrder{}
	err := r.DB.QueryRowContext(ctx, `
		SELECT
			po.id, po.po_number, po.supplier_name, po.source_order_id,
			po.order_date, po.expected_date, po.currency, po.status,
			po.goods_amount, po.freight_amount, po.total_amount,
			po.remark, po.created_by, po.created_at, po.updated_at,
			COALESCE(o.order_number, '')
		FROM purchase_orders po
		LEFT JOIN orders o ON o.id = po.source_order_id
		WHERE po.id = ?`, id,
	).Scan(
		&po.ID, &po.PONumber, &po.SupplierName, &po.SourceOrderID,
		&po.OrderDate, &po.ExpectedDate, &po.Currency, &po.Status,
		&po.GoodsAmount, &po.FreightAmount, &po.TotalAmount,
		&po.Remark, &po.CreatedBy, &po.CreatedAt, &po.UpdatedAt,
		&po.SourceOrderNumber,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrPurchaseOrderNotFound
		}
		return nil, err
	}

	items, err := r.ListItems(ctx, id)
	if err != nil {
		return nil, err
	}
	po.Items = items

	return po, nil
}

// ListReceivedMaterialItems 查询已到货的材料采购明细（用于 PI 的 Box orders 自动带出）。
//
// 只统计关联到指定客户订单、且已入库数量大于 0 的材料行。
func (r *PurchaseOrderRepository) ListReceivedMaterialItems(
	ctx context.Context,
	orderIDs []uint64,
) ([]models.PurchaseOrderItem, error) {
	if len(orderIDs) == 0 {
		return []models.PurchaseOrderItem{}, nil
	}

	ph := make([]string, 0, len(orderIDs))
	args := make([]any, 0, len(orderIDs)+1)
	for _, id := range orderIDs {
		ph = append(ph, "?")
		args = append(args, id)
	}
	args = append(args, models.InventoryItemMaterial)

	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			i.id, i.purchase_order_id, i.item_type, i.item_id, i.item_code,
			i.item_name, i.short_name, i.spec, i.unit,
			i.required_quantity, i.quantity, i.unit_price, i.amount,
			i.received_quantity, i.remark
		FROM purchase_order_items i
		JOIN purchase_orders po ON po.id = i.purchase_order_id
		WHERE po.source_order_id IN (`+strings.Join(ph, ",")+`)
		  AND i.item_type = ?
		  AND i.received_quantity > 0
		  AND po.status IN ('partial', 'received')
		ORDER BY i.id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.PurchaseOrderItem, 0)
	for rows.Next() {
		it, err := scanPurchaseItem(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *it)
	}

	return result, rows.Err()
}

// FindDraftBySourceOrder 查找某订单已下推的草稿采购单 ID（无则返回 0）。
func (r *PurchaseOrderRepository) FindDraftBySourceOrder(
	ctx context.Context,
	orderID uint64,
) (uint64, error) {
	var id uint64
	err := r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(id), 0)
		FROM purchase_orders
		WHERE source_order_id = ? AND status = ?`,
		orderID, models.PurchaseStatusDraft,
	).Scan(&id)
	return id, err
}

// NextNumber 生成采购单号：PO-yyyyMMdd[-NN]。
func (r *PurchaseOrderRepository) NextNumber(ctx context.Context) (string, error) {
	prefix := "PO-" + time.Now().Format("20060102")

	var count int
	if err := r.DB.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM purchase_orders WHERE po_number LIKE ?`,
		prefix+"%",
	).Scan(&count); err != nil {
		return "", err
	}

	if count == 0 {
		return prefix, nil
	}
	return fmt.Sprintf("%s-%02d", prefix, count+1), nil
}

func insertPurchaseItems(
	ctx context.Context,
	tx *sql.Tx,
	poID uint64,
	items []models.PurchaseOrderItem,
	preserveReceived map[uint64]float64,
) error {
	for i := range items {
		it := &items[i]

		// 明细被重写时，保留已入库数量，避免库存与单据脱节。
		received := 0.0
		if it.ID != 0 {
			if v, ok := preserveReceived[it.ID]; ok {
				received = v
			}
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO purchase_order_items (
				purchase_order_id, item_type, item_id, item_code, item_name,
				short_name, spec, unit, required_quantity, quantity,
				unit_price, amount, received_quantity, remark
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			poID, it.ItemType, it.ItemID, it.ItemCode, it.ItemName,
			it.ShortName, it.Spec, it.Unit, it.RequiredQuantity, it.Quantity,
			it.UnitPrice, it.Amount, received, it.Remark,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *PurchaseOrderRepository) insertHeader(
	ctx context.Context,
	tx *sql.Tx,
	po *models.PurchaseOrder,
) error {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO purchase_orders (
			po_number, supplier_name, source_order_id, order_date, expected_date,
			currency, status, goods_amount, freight_amount, total_amount,
			remark, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		po.PONumber, po.SupplierName, po.SourceOrderID, po.OrderDate, po.ExpectedDate,
		po.Currency, po.Status, po.GoodsAmount, po.FreightAmount, po.TotalAmount,
		po.Remark, po.CreatedBy,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	po.ID = uint64(id)
	return nil
}

// Create 新增采购单（头 + 明细，事务）。
func (r *PurchaseOrderRepository) Create(
	ctx context.Context,
	po *models.PurchaseOrder,
) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := r.insertHeader(ctx, tx, po); err != nil {
		return err
	}
	if err := insertPurchaseItems(ctx, tx, po.ID, po.Items, nil); err != nil {
		return err
	}

	return tx.Commit()
}

// Update 更新采购单（重写明细，事务；保留已入库数量）。
func (r *PurchaseOrderRepository) Update(
	ctx context.Context,
	po *models.PurchaseOrder,
) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		UPDATE purchase_orders SET
			po_number = ?, supplier_name = ?, source_order_id = ?,
			order_date = ?, expected_date = ?, currency = ?, status = ?,
			goods_amount = ?, freight_amount = ?, total_amount = ?, remark = ?
		WHERE id = ?`,
		po.PONumber, po.SupplierName, po.SourceOrderID,
		po.OrderDate, po.ExpectedDate, po.Currency, po.Status,
		po.GoodsAmount, po.FreightAmount, po.TotalAmount, po.Remark,
		po.ID,
	)
	if err != nil {
		return err
	}

	// 先取出已入库数量，重写明细时按行保留。
	preserve := make(map[uint64]float64)
	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, received_quantity FROM purchase_order_items WHERE purchase_order_id = ?`,
		po.ID,
	)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uint64
		var received float64
		if err := rows.Scan(&id, &received); err != nil {
			rows.Close()
			return err
		}
		preserve[id] = received
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if _, err = tx.ExecContext(
		ctx, `DELETE FROM purchase_order_items WHERE purchase_order_id = ?`, po.ID,
	); err != nil {
		return err
	}

	if err := insertPurchaseItems(ctx, tx, po.ID, po.Items, preserve); err != nil {
		return err
	}

	return tx.Commit()
}

// SetItemReceived 更新某明细行的已入库数量。
func (r *PurchaseOrderRepository) SetItemReceived(
	ctx context.Context,
	itemID uint64,
	received float64,
) error {
	_, err := r.DB.ExecContext(
		ctx,
		`UPDATE purchase_order_items SET received_quantity = ? WHERE id = ?`,
		received, itemID,
	)
	return err
}

// UpdateStatus 更新采购单状态。
func (r *PurchaseOrderRepository) UpdateStatus(
	ctx context.Context,
	id uint64,
	status string,
) error {
	result, err := r.DB.ExecContext(
		ctx, `UPDATE purchase_orders SET status = ? WHERE id = ?`, status, id,
	)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// Delete 删除采购单（明细级联删除）。
func (r *PurchaseOrderRepository) Delete(ctx context.Context, id uint64) error {
	result, err := r.DB.ExecContext(ctx, `DELETE FROM purchase_orders WHERE id = ?`, id)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return models.ErrPurchaseOrderNotFound
	}
	return nil
}

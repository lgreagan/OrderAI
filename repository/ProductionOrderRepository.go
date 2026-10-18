package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"OrderAI/models"
)

// ProductionOrderRepository 读写持久化的生产订单（头 + 明细）。
type ProductionOrderRepository struct {
	DB *sql.DB
}

func NewProductionOrderRepository(db *sql.DB) *ProductionOrderRepository {
	return &ProductionOrderRepository{DB: db}
}

// List 查询生产单列表（含来源订单号、明细/完工统计与合计箱数）。
func (r *ProductionOrderRepository) List(
	ctx context.Context,
	keyword, status string,
) ([]models.ProductionOrderListItem, error) {
	query := `
		SELECT
			po.id, po.po_number, po.source_order_id,
			COALESCE(o.order_number, ''),
			po.source_purchase_order_id,
			COALESCE(pur.po_number, ''),
			po.plan_date, po.start_date, po.finish_date, po.status,
			po.work_type,
			(SELECT COUNT(*) FROM production_order_items i
				WHERE i.production_order_id = po.id),
			(SELECT COUNT(*) FROM production_order_items i
				WHERE i.production_order_id = po.id
				  AND i.planned_quantity > 0 AND i.produced_quantity >= i.planned_quantity),
			(SELECT COALESCE(SUM(i.carton_count), 0) FROM production_order_items i
				WHERE i.production_order_id = po.id),
			po.created_at
		FROM production_orders po
		LEFT JOIN orders o ON o.id = po.source_order_id
		LEFT JOIN purchase_orders pur ON pur.id = po.source_purchase_order_id
		WHERE 1 = 1`
	args := []any{}

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		k := "%" + keyword + "%"
		query += ` AND (
			po.po_number LIKE ? OR o.order_number LIKE ? OR
			pur.po_number LIKE ? OR o.customer LIKE ? OR po.remark LIKE ?
		)`
		args = append(args, k, k, k, k, k)
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

	result := make([]models.ProductionOrderListItem, 0)
	for rows.Next() {
		var it models.ProductionOrderListItem
		if err := rows.Scan(
			&it.ID, &it.PONumber, &it.SourceOrderID, &it.SourceOrderNumber,
			&it.SourcePurchaseOrderID, &it.SourcePurchaseNumber,
			&it.PlanDate, &it.StartDate, &it.FinishDate, &it.Status,
			&it.WorkType,
			&it.ItemCount, &it.CompletedCount, &it.TotalCartons,
			&it.CreatedAt,
		); err != nil {
			return nil, err
		}
		it.WorkType = models.NormalizeWorkType(it.WorkType)
		it.WorkTypeLabel = models.WorkTypeLabel(it.WorkType)
		result = append(result, it)
	}

	return result, rows.Err()
}

// MaterialSummary 汇总生产单所需材料（计划量 × 单位用量），供生产报表备料查看。
//
// 过滤条件与 List 保持一致（单号/订单号/客户/备注 + 状态），
// 只统计仍为 active 的用料记录，与完工领料的口径一致。
func (r *ProductionOrderRepository) MaterialSummary(
	ctx context.Context,
	keyword, status string,
) ([]models.ProductionMaterialSummary, error) {
	query := `
		SELECT
			b.material_id,
			COALESCE(m.code, ''),
			COALESCE(NULLIF(m.short_name, ''), NULLIF(m.name_zh, ''), m.name, ''),
			COALESCE(m.spec, ''),
			COALESCE(NULLIF(b.unit, ''), m.unit, ''),
			COALESCE(m.category, ''),
			CASE WHEN COALESCE(m.category, '') = 'packaging'
				THEN CEIL(SUM(i.planned_quantity * b.quantity_per_unit))
				ELSE SUM(i.planned_quantity * b.quantity_per_unit) END,
			COUNT(DISTINCT CONCAT(i.product_type, ':', COALESCE(i.product_ref_id, 0)))
		FROM production_order_items i
		JOIN production_orders po ON po.id = i.production_order_id
		LEFT JOIN orders o ON o.id = po.source_order_id
		JOIN product_bom b
			ON b.product_type = i.product_type
		   AND b.product_ref_id = i.product_ref_id
		   AND b.status = 'active'
		LEFT JOIN materials m ON m.id = b.material_id
		WHERE i.planned_quantity > 0`
	args := []any{}

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		k := "%" + keyword + "%"
		query += ` AND (
			po.po_number LIKE ? OR o.order_number LIKE ? OR
			o.customer LIKE ? OR po.remark LIKE ?
		)`
		args = append(args, k, k, k, k)
	}

	if status = strings.TrimSpace(status); status != "" {
		query += ` AND po.status = ?`
		args = append(args, status)
	}

	query += `
		GROUP BY b.material_id, m.code, m.short_name, m.name_zh, m.name, m.spec,
			b.unit, m.unit, m.category
		ORDER BY m.code ASC, b.material_id ASC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ProductionMaterialSummary, 0)
	for rows.Next() {
		var it models.ProductionMaterialSummary
		if err := rows.Scan(
			&it.MaterialID, &it.MaterialCode, &it.MaterialName, &it.Spec,
			&it.Unit, &it.Category, &it.RequiredQuantity, &it.ProductCount,
		); err != nil {
			return nil, err
		}
		it.IsPackaging = it.Category == models.MaterialCategoryPackaging
		result = append(result, it)
	}

	return result, rows.Err()
}

// MaterialNeeds 汇总某生产单所需的材料（计划量 × 单位用量），用于领料时预填。
func (r *ProductionOrderRepository) MaterialNeeds(
	ctx context.Context,
	poID uint64,
) ([]models.ProductionMaterialSummary, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			b.material_id,
			COALESCE(m.code, ''),
			COALESCE(NULLIF(m.short_name, ''), NULLIF(m.name_zh, ''), m.name, ''),
			COALESCE(m.spec, ''),
			COALESCE(NULLIF(b.unit, ''), m.unit, ''),
			COALESCE(m.category, ''),
			CASE WHEN COALESCE(m.category, '') = 'packaging'
				THEN CEIL(SUM(i.planned_quantity * b.quantity_per_unit))
				ELSE SUM(i.planned_quantity * b.quantity_per_unit) END,
			COUNT(DISTINCT CONCAT(i.product_type, ':', COALESCE(i.product_ref_id, 0)))
		FROM production_order_items i
		JOIN product_bom b
			ON b.product_type = i.product_type
		   AND b.product_ref_id = i.product_ref_id
		   AND b.status = 'active'
		LEFT JOIN materials m ON m.id = b.material_id
		WHERE i.production_order_id = ? AND i.planned_quantity > 0
		GROUP BY b.material_id, m.code, m.short_name, m.name_zh, m.name, m.spec,
			b.unit, m.unit, m.category
		ORDER BY m.code ASC, b.material_id ASC`, poID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ProductionMaterialSummary, 0)
	for rows.Next() {
		var it models.ProductionMaterialSummary
		if err := rows.Scan(
			&it.MaterialID, &it.MaterialCode, &it.MaterialName, &it.Spec,
			&it.Unit, &it.Category, &it.RequiredQuantity, &it.ProductCount,
		); err != nil {
			return nil, err
		}
		it.IsPackaging = it.Category == models.MaterialCategoryPackaging
		result = append(result, it)
	}

	return result, rows.Err()
}

const productionItemColumns = `
	id, production_order_id, product_type, product_ref_id, product_name,
	short_name, spec, planned_quantity, required_quantity, produced_quantity,
	sets_per_carton, carton_count, unit, remark
`

func scanProductionItem(row interface {
	Scan(...any) error
}) (*models.ProductionOrderItem, error) {
	it := &models.ProductionOrderItem{}
	err := row.Scan(
		&it.ID, &it.ProductionOrderID, &it.ProductType, &it.ProductRefID,
		&it.ProductName, &it.ShortName, &it.Spec,
		&it.PlannedQuantity, &it.RequiredQuantity, &it.ProducedQuantity,
		&it.SetsPerCarton, &it.CartonCount, &it.Unit, &it.Remark,
	)
	if err != nil {
		return nil, err
	}
	return it, nil
}

// ListItems 查询某生产单的明细行。
func (r *ProductionOrderRepository) ListItems(
	ctx context.Context,
	poID uint64,
) ([]models.ProductionOrderItem, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT `+productionItemColumns+`
		FROM production_order_items
		WHERE production_order_id = ?
		ORDER BY id ASC`, poID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ProductionOrderItem, 0)
	for rows.Next() {
		it, err := scanProductionItem(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *it)
	}

	return result, rows.Err()
}

// GetByID 查询生产单头 + 明细。
func (r *ProductionOrderRepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.ProductionOrder, error) {
	po := &models.ProductionOrder{}
	err := r.DB.QueryRowContext(ctx, `
		SELECT
			po.id, po.po_number, po.source_order_id,
			po.source_purchase_order_id,
			po.plan_date, po.start_date, po.finish_date, po.status,
			po.work_type,
			po.remark, po.created_by, po.created_at, po.updated_at,
			COALESCE(o.order_number, ''),
			COALESCE(pur.po_number, '')
		FROM production_orders po
		LEFT JOIN orders o ON o.id = po.source_order_id
		LEFT JOIN purchase_orders pur ON pur.id = po.source_purchase_order_id
		WHERE po.id = ?`, id,
	).Scan(
		&po.ID, &po.PONumber, &po.SourceOrderID,
		&po.SourcePurchaseOrderID,
		&po.PlanDate, &po.StartDate, &po.FinishDate, &po.Status,
		&po.WorkType,
		&po.Remark, &po.CreatedBy, &po.CreatedAt, &po.UpdatedAt,
		&po.SourceOrderNumber,
		&po.SourcePurchaseNumber,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrProductionOrderNotFound
		}
		return nil, err
	}
	po.WorkType = models.NormalizeWorkType(po.WorkType)

	items, err := r.ListItems(ctx, id)
	if err != nil {
		return nil, err
	}
	po.Items = items

	return po, nil
}

// FindDraftBySourceOrder 查找某订单已下推的草稿生产单 ID（无则返回 0）。
func (r *ProductionOrderRepository) FindDraftBySourceOrder(
	ctx context.Context,
	orderID uint64,
) (uint64, error) {
	var id uint64
	err := r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(id), 0)
		FROM production_orders
		WHERE source_order_id = ? AND status = ?`,
		orderID, models.ProductionStatusDraft,
	).Scan(&id)
	return id, err
}

// FindDraftBySourcePurchaseOrder 查找某采购单已下推的草稿生产单 ID（无则返回 0）。
func (r *ProductionOrderRepository) FindDraftBySourcePurchaseOrder(
	ctx context.Context,
	purchaseOrderID uint64,
) (uint64, error) {
	var id uint64
	err := r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(id), 0)
		FROM production_orders
		WHERE source_purchase_order_id = ? AND status = ?`,
		purchaseOrderID, models.ProductionStatusDraft,
	).Scan(&id)
	return id, err
}

// NextNumber 生成生产单号：MO-yyyyMMdd[-NN]。
func (r *ProductionOrderRepository) NextNumber(ctx context.Context) (string, error) {
	prefix := "MO-" + time.Now().Format("20060102")

	var count int
	if err := r.DB.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM production_orders WHERE po_number LIKE ?`,
		prefix+"%",
	).Scan(&count); err != nil {
		return "", err
	}

	if count == 0 {
		return prefix, nil
	}
	return fmt.Sprintf("%s-%02d", prefix, count+1), nil
}

// ListRuleComponents 批量查询组合规则的组成单品。
func (r *ProductionOrderRepository) ListRuleComponents(
	ctx context.Context,
	ruleIDs []uint64,
) (map[uint64][]models.RuleComponent, error) {
	result := make(map[uint64][]models.RuleComponent)
	if len(ruleIDs) == 0 {
		return result, nil
	}

	ph := make([]string, 0, len(ruleIDs))
	args := make([]any, 0, len(ruleIDs))
	for _, id := range ruleIDs {
		ph = append(ph, "?")
		args = append(args, id)
	}

	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			i.rule_id, i.sku_id,
			COALESCE(s.sku_code, ''),
			COALESCE(s.product_name, ''),
			COALESCE(s.product_name_zh, ''),
			i.quantity_per_set
		FROM product_match_rule_items i
		LEFT JOIN skus s ON s.id = i.sku_id
		WHERE i.rule_id IN (`+strings.Join(ph, ",")+`)
		ORDER BY i.rule_id ASC, i.sort_order ASC, i.id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var c models.RuleComponent
		if err := rows.Scan(
			&c.RuleID, &c.SkuID, &c.SkuCode,
			&c.ProductName, &c.ProductNameZH, &c.QuantityPerSet,
		); err != nil {
			return nil, err
		}
		result[c.RuleID] = append(result[c.RuleID], c)
	}

	return result, rows.Err()
}

func insertProductionItems(
	ctx context.Context,
	tx *sql.Tx,
	poID uint64,
	items []models.ProductionOrderItem,
	preserveProduced map[uint64]float64,
) error {
	for i := range items {
		it := &items[i]

		// 明细重写时保留已生产数量，避免库存与单据脱节。
		produced := 0.0
		if it.ID != 0 {
			if v, ok := preserveProduced[it.ID]; ok {
				produced = v
			}
		}

		cartons := 0.0
		if it.SetsPerCarton > 0 {
			cartons = it.PlannedQuantity / it.SetsPerCarton
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO production_order_items (
				production_order_id, product_type, product_ref_id, product_name,
				short_name, spec, planned_quantity, required_quantity,
				produced_quantity, sets_per_carton, carton_count, unit, remark
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			poID, it.ProductType, it.ProductRefID, it.ProductName,
			it.ShortName, it.Spec, it.PlannedQuantity, it.RequiredQuantity,
			produced, it.SetsPerCarton, cartons, it.Unit, it.Remark,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ProductionOrderRepository) insertHeader(
	ctx context.Context,
	tx *sql.Tx,
	po *models.ProductionOrder,
) error {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO production_orders (
			po_number, source_order_id, source_purchase_order_id,
			plan_date, start_date, finish_date,
			status, work_type, remark, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		po.PONumber, po.SourceOrderID, po.SourcePurchaseOrderID,
		po.PlanDate, po.StartDate, po.FinishDate,
		po.Status, models.NormalizeWorkType(po.WorkType), po.Remark, po.CreatedBy,
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

// Create 新增生产单（头 + 明细，事务）。
func (r *ProductionOrderRepository) Create(
	ctx context.Context,
	po *models.ProductionOrder,
) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := r.insertHeader(ctx, tx, po); err != nil {
		return err
	}
	if err := insertProductionItems(ctx, tx, po.ID, po.Items, nil); err != nil {
		return err
	}

	return tx.Commit()
}

// Update 更新生产单（重写明细，事务；保留已生产数量）。
func (r *ProductionOrderRepository) Update(
	ctx context.Context,
	po *models.ProductionOrder,
) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		UPDATE production_orders SET
			po_number = ?, source_order_id = ?, source_purchase_order_id = ?,
			plan_date = ?, start_date = ?,
			finish_date = ?, status = ?, work_type = ?, remark = ?
		WHERE id = ?`,
		po.PONumber, po.SourceOrderID, po.SourcePurchaseOrderID,
		po.PlanDate, po.StartDate,
		po.FinishDate, po.Status, models.NormalizeWorkType(po.WorkType), po.Remark,
		po.ID,
	)
	if err != nil {
		return err
	}

	preserve := make(map[uint64]float64)
	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, produced_quantity FROM production_order_items WHERE production_order_id = ?`,
		po.ID,
	)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uint64
		var produced float64
		if err := rows.Scan(&id, &produced); err != nil {
			rows.Close()
			return err
		}
		preserve[id] = produced
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if _, err = tx.ExecContext(
		ctx, `DELETE FROM production_order_items WHERE production_order_id = ?`, po.ID,
	); err != nil {
		return err
	}

	if err := insertProductionItems(ctx, tx, po.ID, po.Items, preserve); err != nil {
		return err
	}

	return tx.Commit()
}

// SetItemProduced 更新某明细行的已生产数量。
func (r *ProductionOrderRepository) SetItemProduced(
	ctx context.Context,
	itemID uint64,
	produced float64,
) error {
	_, err := r.DB.ExecContext(
		ctx,
		`UPDATE production_order_items SET produced_quantity = ? WHERE id = ?`,
		produced, itemID,
	)
	return err
}

// UpdateStatus 更新生产单状态。
func (r *ProductionOrderRepository) UpdateStatus(
	ctx context.Context,
	id uint64,
	status string,
) error {
	result, err := r.DB.ExecContext(
		ctx, `UPDATE production_orders SET status = ? WHERE id = ?`, status, id,
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

// Delete 删除生产单（明细级联删除）。
func (r *ProductionOrderRepository) Delete(ctx context.Context, id uint64) error {
	result, err := r.DB.ExecContext(ctx, `DELETE FROM production_orders WHERE id = ?`, id)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return models.ErrProductionOrderNotFound
	}
	return nil
}

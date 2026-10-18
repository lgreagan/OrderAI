package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"OrderAI/models"
)

// ShipmentRepository 读写发货单与发货明细。
type ShipmentRepository struct {
	DB *sql.DB
}

func NewShipmentRepository(db *sql.DB) *ShipmentRepository {
	return &ShipmentRepository{DB: db}
}

// List 查询发货单列表（含关联订单信息与明细行数）。
func (r *ShipmentRepository) List(
	ctx context.Context,
	keyword, status string,
) ([]models.ShipmentListItem, error) {
	query := `
		SELECT
			s.id, s.shipment_number, s.order_id, s.batch_no, s.ship_date,
			s.shipping_method, s.carton_count, s.freight_cost, s.insurance_cost,
			s.currency, s.status,
			COALESCE(o.order_number, ''),
			COALESCE(o.customer, ''),
			COALESCE(o.country, ''),
			(SELECT COUNT(*) FROM shipment_items si WHERE si.shipment_id = s.id)
		FROM shipments s
		LEFT JOIN orders o ON o.id = s.order_id
		WHERE 1 = 1`
	args := []any{}

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		k := "%" + keyword + "%"
		query += ` AND (
			s.shipment_number LIKE ? OR o.order_number LIKE ? OR
			o.customer LIKE ? OR s.tracking_no LIKE ?
		)`
		args = append(args, k, k, k, k)
	}

	if status = strings.TrimSpace(status); status != "" {
		query += ` AND s.status = ?`
		args = append(args, status)
	}

	query += ` ORDER BY s.id DESC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ShipmentListItem, 0)
	for rows.Next() {
		var it models.ShipmentListItem
		if err := rows.Scan(
			&it.ID, &it.ShipmentNumber, &it.OrderID, &it.BatchNo, &it.ShipDate,
			&it.ShippingMethod, &it.CartonCount, &it.FreightCost, &it.InsuranceCost,
			&it.Currency, &it.Status,
			&it.OrderNumber, &it.Customer, &it.Country, &it.ItemCount,
		); err != nil {
			return nil, err
		}
		result = append(result, it)
	}

	return result, rows.Err()
}

const shipmentItemColumns = `
	id, shipment_id, order_item_id, item_type, sku_id, material_id,
	short_name, description, quantity, unit, pack_quantity, carton_count,
	carton_no, po_number, carton_weight, carton_length, carton_width, carton_height,
	source, unit_price, amount, is_paid, remark
`

func scanShipmentItem(row interface {
	Scan(...any) error
}) (*models.ShipmentItem, error) {
	it := &models.ShipmentItem{}
	err := row.Scan(
		&it.ID, &it.ShipmentID, &it.OrderItemID, &it.ItemType, &it.SkuID, &it.MaterialID,
		&it.ShortName, &it.Description, &it.Quantity, &it.Unit, &it.PackQuantity, &it.CartonCount,
		&it.CartonNo, &it.PONumber, &it.CartonWeight, &it.CartonLength, &it.CartonWidth, &it.CartonHeight,
		&it.Source, &it.UnitPrice, &it.Amount, &it.IsPaid, &it.Remark,
	)
	if err != nil {
		return nil, err
	}
	return it, nil
}

// ListItems 查询某发货单的明细行。
func (r *ShipmentRepository) ListItems(
	ctx context.Context,
	shipmentID uint64,
) ([]models.ShipmentItem, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT `+shipmentItemColumns+`
		FROM shipment_items
		WHERE shipment_id = ?
		ORDER BY id ASC`, shipmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ShipmentItem, 0)
	for rows.Next() {
		it, err := scanShipmentItem(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *it)
	}

	return result, rows.Err()
}

// GetByID 查询发货单头（含关联订单信息）。
func (r *ShipmentRepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.Shipment, error) {
	s := &models.Shipment{}
	err := r.DB.QueryRowContext(ctx, `
		SELECT
			s.id, s.shipment_number, s.order_id, s.batch_no, s.ship_date,
			s.shipping_method, s.carrier, s.tracking_no, s.carton_count,
			s.freight_cost, s.insurance_cost, s.currency, s.status, s.remark,
			s.created_at, s.updated_at,
			s.warehouse_code, s.po_number, s.customs_mode, s.tax_mode, s.declare_currency,
			s.is_electric, s.is_magnetic, s.is_liquid, s.is_powder, s.is_danger,
			s.need_insurance, s.template_id,
			COALESCE(o.order_number, ''), COALESCE(o.customer, ''),
			COALESCE(o.country, ''), COALESCE(o.shipping_method, '')
		FROM shipments s
		LEFT JOIN orders o ON o.id = s.order_id
		WHERE s.id = ?`, id,
	).Scan(
		&s.ID, &s.ShipmentNumber, &s.OrderID, &s.BatchNo, &s.ShipDate,
		&s.ShippingMethod, &s.Carrier, &s.TrackingNo, &s.CartonCount,
		&s.FreightCost, &s.InsuranceCost, &s.Currency, &s.Status, &s.Remark,
		&s.CreatedAt, &s.UpdatedAt,
		&s.WarehouseCode, &s.PONumber, &s.CustomsMode, &s.TaxMode, &s.DeclareCurrency,
		&s.IsElectric, &s.IsMagnetic, &s.IsLiquid, &s.IsPowder, &s.IsDanger,
		&s.NeedInsurance, &s.TemplateID,
		&s.OrderNumber, &s.Customer, &s.Country, &s.OrderShipping,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrShipmentNotFound
		}
		return nil, err
	}

	items, err := r.ListItems(ctx, id)
	if err != nil {
		return nil, err
	}
	s.Items = items

	return s, nil
}

// ListByOrderIDs 返回指定订单的发货单（含明细）；orderIDs 为空时返回全部。
func (r *ShipmentRepository) ListByOrderIDs(
	ctx context.Context,
	orderIDs []uint64,
) ([]models.Shipment, error) {
	query := `
		SELECT
			s.id, s.shipment_number, s.order_id, s.batch_no, s.ship_date,
			s.shipping_method, s.carrier, s.tracking_no, s.carton_count,
			s.freight_cost, s.insurance_cost, s.currency, s.status, s.remark,
			COALESCE(o.order_number, ''), COALESCE(o.customer, ''),
			COALESCE(o.country, '')
		FROM shipments s
		LEFT JOIN orders o ON o.id = s.order_id
		WHERE 1 = 1`
	args := []any{}

	if len(orderIDs) > 0 {
		ph := make([]string, 0, len(orderIDs))
		for _, id := range orderIDs {
			ph = append(ph, "?")
			args = append(args, id)
		}
		query += ` AND s.order_id IN (` + strings.Join(ph, ",") + `)`
	}

	query += ` ORDER BY s.order_id ASC, s.batch_no ASC, s.id ASC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.Shipment, 0)
	ids := make([]uint64, 0)

	for rows.Next() {
		var s models.Shipment
		if err := rows.Scan(
			&s.ID, &s.ShipmentNumber, &s.OrderID, &s.BatchNo, &s.ShipDate,
			&s.ShippingMethod, &s.Carrier, &s.TrackingNo, &s.CartonCount,
			&s.FreightCost, &s.InsuranceCost, &s.Currency, &s.Status, &s.Remark,
			&s.OrderNumber, &s.Customer, &s.Country,
		); err != nil {
			return nil, err
		}
		result = append(result, s)
		ids = append(ids, s.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return result, nil
	}

	// 批量加载明细。
	ph := make([]string, 0, len(ids))
	itemArgs := make([]any, 0, len(ids))
	for _, id := range ids {
		ph = append(ph, "?")
		itemArgs = append(itemArgs, id)
	}

	itemRows, err := r.DB.QueryContext(ctx, `
		SELECT `+shipmentItemColumns+`
		FROM shipment_items
		WHERE shipment_id IN (`+strings.Join(ph, ",")+`)
		ORDER BY id ASC`, itemArgs...)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	itemsByShipment := make(map[uint64][]models.ShipmentItem)
	for itemRows.Next() {
		it, err := scanShipmentItem(itemRows)
		if err != nil {
			return nil, err
		}
		itemsByShipment[it.ShipmentID] = append(itemsByShipment[it.ShipmentID], *it)
	}
	if err := itemRows.Err(); err != nil {
		return nil, err
	}

	for i := range result {
		result[i].Items = itemsByShipment[result[i].ID]
	}

	return result, nil
}

// NextBatchNo 返回某订单下一个批次号。
func (r *ShipmentRepository) NextBatchNo(
	ctx context.Context,
	orderID uint64,
) (int, error) {
	var n int
	err := r.DB.QueryRowContext(
		ctx,
		`SELECT COALESCE(MAX(batch_no), 0) + 1 FROM shipments WHERE order_id = ?`,
		orderID,
	).Scan(&n)
	return n, err
}

// FindDraftByOrder 查找某订单已下推的草稿发货单 ID（无则返回 0）。
//
// 重复下推会重复计算"已发货量"，因此下推前先复用已有草稿。
func (r *ShipmentRepository) FindDraftByOrder(
	ctx context.Context,
	orderID uint64,
) (uint64, error) {
	var id uint64
	err := r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(id), 0)
		FROM shipments
		WHERE order_id = ? AND status = ?`,
		orderID, models.ShipmentStatusDraft,
	).Scan(&id)
	return id, err
}

// GetOrderHeader 查询订单表头（用于发货单上下文）。
func (r *ShipmentRepository) GetOrderHeader(
	ctx context.Context,
	orderID uint64,
) (*models.OrderShipmentContext, error) {
	oc := &models.OrderShipmentContext{OrderID: orderID}
	err := r.DB.QueryRowContext(ctx, `
		SELECT order_number, customer, country, shipping_method
		FROM orders WHERE id = ?`, orderID,
	).Scan(&oc.OrderNumber, &oc.Customer, &oc.Country, &oc.ShippingMethod)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrOrderNotFound
		}
		return nil, err
	}
	return oc, nil
}

// ListOrderLines 查询某订单的可发货行：按匹配 SKU 展开，并带出已发货量。
//
// 一行 = 订单行 × 匹配到的 SKU。组合套装（rule_id 非空）会展开出多行，
// 带出规则编号、每套用量与组合箱规，供前端按「套装父行 + 组成单品子行」折叠展示；
// 单品直配每单行只有一行。
//
// 箱数口径：组合套装行的每箱数量 = sets_per_carton × quantity_per_set
// （每箱装 N 套 × 每套 M 个 = 每箱 M×N 个），单品直配行沿用 SKU 的 pack_quantity。
//
// 排序：先按订单行分组（组内最小产品排序号），组内按产品排序号 → SKU，保证同一组的行连续。
func (r *ShipmentRepository) ListOrderLines(
	ctx context.Context,
	orderID uint64,
) ([]models.OrderShipmentLine, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			oi.id,
			oi.line_number,
			CASE WHEN m.rule_id IS NULL THEN 'sku' ELSE 'rule' END,
			m.rule_id,
			COALESCE(r.rule_code, ''),
			COALESCE(r.product_name_zh, ''),
			m.sku_id,
			COALESCE(s.sku_code, ''),
			COALESCE(s.short_name, ''),
			COALESCE(s.product_name_zh, ''),
			oi.product_name,
			oi.specification,
			oi.quantity,
			COALESCE(NULLIF(r.order_unit, ''), NULLIF(oi.unit, ''), ''),
			m.quantity_per_set,
			COALESCE(r.sets_per_carton, 0),
			COALESCE(m.required_quantity, 0),
			COALESCE(sh.shipped, 0),
			CASE
				WHEN m.rule_id IS NOT NULL AND COALESCE(r.sets_per_carton, 0) > 0
					THEN r.sets_per_carton * m.quantity_per_set
				ELSE COALESCE(s.pack_quantity, 0)
			END,
			COALESCE(s.price_usd, 0),
			COALESCE(NULLIF(s.unit, ''), oi.unit)
		FROM order_items oi
		JOIN order_item_matches m ON m.order_item_id = oi.id
		LEFT JOIN skus s ON s.id = m.sku_id
		LEFT JOIN product_match_rules r ON r.id = m.rule_id
		LEFT JOIN (
			SELECT order_item_id, sku_id, SUM(quantity) AS shipped
			FROM shipment_items
			GROUP BY order_item_id, sku_id
		) sh ON sh.order_item_id = oi.id AND sh.sku_id = m.sku_id
		WHERE oi.order_id = ?
		ORDER BY
			(SELECT MIN(COALESCE(s2.sort_order, 0))
			   FROM order_item_matches m2
			   LEFT JOIN skus s2 ON s2.id = m2.sku_id
			  WHERE m2.order_item_id = oi.id) ASC,
			oi.line_number ASC,
			COALESCE(s.sort_order, 0) ASC,
			m.sku_id ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.OrderShipmentLine, 0)
	for rows.Next() {
		var l models.OrderShipmentLine
		if err := rows.Scan(
			&l.OrderItemID, &l.LineNumber, &l.MatchType,
			&l.RuleID, &l.RuleCode, &l.RuleNameZH,
			&l.SkuID, &l.SkuCode, &l.ShortName, &l.ProductNameZH,
			&l.CustomerProductName, &l.Specification,
			&l.OrderQuantity, &l.OrderUnit,
			&l.QuantityPerSet, &l.SetsPerCarton,
			&l.RequiredQuantity, &l.ShippedQuantity,
			&l.PackQuantity, &l.UnitPrice, &l.Unit,
		); err != nil {
			return nil, err
		}
		l.RemainingQuantity = l.RequiredQuantity - l.ShippedQuantity
		result = append(result, l)
	}

	return result, rows.Err()
}

// GenerateNumber 生成发货单号：SH-yyyyMMdd-NN。
func (r *ShipmentRepository) GenerateNumber(ctx context.Context) (string, error) {
	var count int
	err := r.DB.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM shipments WHERE DATE(created_at) = CURDATE()`,
	).Scan(&count)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("SH-%s-%02d", time.Now().Format("20060102"), count+1), nil
}

func insertShipmentItems(
	ctx context.Context,
	tx *sql.Tx,
	shipmentID uint64,
	items []models.ShipmentItem,
) error {
	for i := range items {
		it := &items[i]
		_, err := tx.ExecContext(ctx, `
			INSERT INTO shipment_items (
				shipment_id, order_item_id, item_type, sku_id, material_id,
				short_name, description, quantity, unit, pack_quantity, carton_count,
				carton_no, po_number, carton_weight, carton_length, carton_width, carton_height,
				source, unit_price, amount, is_paid, remark
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			shipmentID, it.OrderItemID, it.ItemType, it.SkuID, it.MaterialID,
			it.ShortName, it.Description, it.Quantity, it.Unit, it.PackQuantity, it.CartonCount,
			it.CartonNo, it.PONumber, it.CartonWeight, it.CartonLength, it.CartonWidth, it.CartonHeight,
			it.Source, it.UnitPrice, it.Amount, it.IsPaid, it.Remark,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// Create 新增发货单（头 + 明细，事务）。
func (r *ShipmentRepository) Create(
	ctx context.Context,
	s *models.Shipment,
) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO shipments (
			shipment_number, order_id, batch_no, ship_date, shipping_method,
			carrier, tracking_no, carton_count, freight_cost, insurance_cost,
			currency, status, remark,
			warehouse_code, po_number, customs_mode, tax_mode, declare_currency,
			is_electric, is_magnetic, is_liquid, is_powder, is_danger,
			need_insurance, template_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ShipmentNumber, s.OrderID, s.BatchNo, s.ShipDate, s.ShippingMethod,
		s.Carrier, s.TrackingNo, s.CartonCount, s.FreightCost, s.InsuranceCost,
		s.Currency, s.Status, s.Remark,
		s.WarehouseCode, s.PONumber, s.CustomsMode, s.TaxMode, s.DeclareCurrency,
		s.IsElectric, s.IsMagnetic, s.IsLiquid, s.IsPowder, s.IsDanger,
		s.NeedInsurance, s.TemplateID,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	s.ID = uint64(id)

	if err := insertShipmentItems(ctx, tx, s.ID, s.Items); err != nil {
		return err
	}

	return tx.Commit()
}

// Update 更新发货单（重写明细，事务）。
func (r *ShipmentRepository) Update(
	ctx context.Context,
	s *models.Shipment,
) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		UPDATE shipments SET
			shipment_number = ?, order_id = ?, batch_no = ?, ship_date = ?,
			shipping_method = ?, carrier = ?, tracking_no = ?, carton_count = ?,
			freight_cost = ?, insurance_cost = ?, currency = ?, status = ?, remark = ?,
			warehouse_code = ?, po_number = ?, customs_mode = ?, tax_mode = ?,
			declare_currency = ?, is_electric = ?, is_magnetic = ?, is_liquid = ?,
			is_powder = ?, is_danger = ?, need_insurance = ?, template_id = ?
		WHERE id = ?`,
		s.ShipmentNumber, s.OrderID, s.BatchNo, s.ShipDate,
		s.ShippingMethod, s.Carrier, s.TrackingNo, s.CartonCount,
		s.FreightCost, s.InsuranceCost, s.Currency, s.Status, s.Remark,
		s.WarehouseCode, s.PONumber, s.CustomsMode, s.TaxMode,
		s.DeclareCurrency, s.IsElectric, s.IsMagnetic, s.IsLiquid,
		s.IsPowder, s.IsDanger, s.NeedInsurance, s.TemplateID,
		s.ID,
	)
	if err != nil {
		return err
	}

	if _, err = tx.ExecContext(
		ctx, `DELETE FROM shipment_items WHERE shipment_id = ?`, s.ID,
	); err != nil {
		return err
	}

	if err := insertShipmentItems(ctx, tx, s.ID, s.Items); err != nil {
		return err
	}

	return tx.Commit()
}

// Delete 删除发货单（明细级联删除）。
func (r *ShipmentRepository) Delete(
	ctx context.Context,
	id uint64,
) error {
	result, err := r.DB.ExecContext(ctx, `DELETE FROM shipments WHERE id = ?`, id)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return models.ErrShipmentNotFound
	}
	return nil
}

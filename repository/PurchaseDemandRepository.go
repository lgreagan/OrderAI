package repository

import (
	"context"
	"database/sql"

	"OrderAI/models"
)

// PurchaseDemandRepository 以 orders 表为准，读取订单的采购需求（按单品 SKU 汇总）。
type PurchaseDemandRepository struct {
	DB *sql.DB
}

func NewPurchaseDemandRepository(db *sql.DB) *PurchaseDemandRepository {
	return &PurchaseDemandRepository{DB: db}
}

// GetOrder 查询来源订单表头。
func (r *PurchaseDemandRepository) GetOrder(
	ctx context.Context,
	orderID uint64,
) (*models.PurchaseDemandSourceOrder, error) {
	order := &models.PurchaseDemandSourceOrder{}

	err := r.DB.QueryRowContext(ctx, `
		SELECT
			id, order_number, country, shipping_method,
			customer, status, created_at
		FROM orders
		WHERE id = ?`,
		orderID,
	).Scan(
		&order.ID, &order.OrderNumber, &order.Country, &order.ShippingMethod,
		&order.Customer, &order.Status, &order.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrOrderNotFound
		}
		return nil, err
	}

	return order, nil
}

// UpdateOrderStatus 更新订单状态，用于推进订单履约流程。
func (r *PurchaseDemandRepository) UpdateOrderStatus(
	ctx context.Context,
	orderID uint64,
	status string,
) error {
	result, err := r.DB.ExecContext(ctx, `
		UPDATE orders
		SET status = ?
		WHERE id = ?`,
		status, orderID,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// 状态未变化时影响行数为 0，用订单是否存在区分。
	if affected == 0 {
		if _, err := r.GetOrder(ctx, orderID); err != nil {
			return err
		}
	}

	return nil
}

// ListOrderItems 查询某订单的采购需求：按单品 SKU 汇总需求数量，
// 并带出当前库存与在途采购量，便于计算建议采购量。
func (r *PurchaseDemandRepository) ListOrderItems(
	ctx context.Context,
	orderID uint64,
) ([]models.PurchaseDemandItem, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			m.sku_id,
			COALESCE(MAX(s.sku_code), MAX(m.sku_code_snapshot), ''),
			COALESCE(MAX(s.product_name), MAX(m.product_name_snapshot), ''),
			COALESCE(MAX(s.product_name_zh), ''),
			COALESCE(MAX(s.short_name), ''),
			COALESCE(MAX(s.model), MAX(m.model_snapshot), ''),
			COALESCE(MAX(s.color), MAX(m.color_snapshot), ''),
			COALESCE(MAX(s.status), ''),
			COALESCE(MAX(s.price), 0),
			COALESCE(MAX(s.price_usd), 0),
			COALESCE(MAX(s.currency), ''),
			COALESCE(MAX(NULLIF(s.unit, '')), ''),
			COALESCE(SUM(m.required_quantity), 0),
			COUNT(*),
			COALESCE(MAX(st.qty), 0),
			COALESCE(MAX(ti.qty), 0)
		FROM order_item_matches m
		JOIN order_items oi ON oi.id = m.order_item_id
		LEFT JOIN skus s ON s.id = m.sku_id
		LEFT JOIN (
			SELECT item_id, SUM(stock_qty) AS qty
			FROM v_inventory_stock
			WHERE item_type = 'sku'
			GROUP BY item_id
		) st ON st.item_id = m.sku_id
		LEFT JOIN (
			SELECT poi.item_id, SUM(poi.quantity - poi.received_quantity) AS qty
			FROM purchase_order_items poi
			JOIN purchase_orders po ON po.id = poi.purchase_order_id
			WHERE poi.item_type = 'sku'
			  AND po.status IN ('draft', 'ordered', 'partial')
			GROUP BY poi.item_id
		) ti ON ti.item_id = m.sku_id
		WHERE oi.order_id = ?
		GROUP BY m.sku_id
		ORDER BY MAX(s.model) ASC, MAX(s.color) ASC, m.sku_id ASC`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.PurchaseDemandItem, 0)

	for rows.Next() {
		var item models.PurchaseDemandItem

		err := rows.Scan(
			&item.SkuID,
			&item.SkuCode,
			&item.ProductName,
			&item.ProductNameZH,
			&item.ShortName,
			&item.Model,
			&item.Color,
			&item.Status,
			&item.Price,
			&item.PriceUSD,
			&item.Currency,
			&item.Unit,
			&item.RequiredQuantity,
			&item.LineCount,
			&item.StockQuantity,
			&item.InTransitQuantity,
		)
		if err != nil {
			return nil, err
		}

		item.SuggestedQuantity = suggestedQuantity(
			item.RequiredQuantity, item.StockQuantity, item.InTransitQuantity,
		)

		result = append(result, item)
	}

	return result, rows.Err()
}

// ListOrderLineDemand 查询某订单的需求明细，**按订单行一对一返回**。
//
// 与 ListOrderItems 的区别：后者把需求量按 SKU 汇总（采购按单品买），
// 这里保留「订单行」这个粒度，供「采购材料」直接按订单需求量折算纸箱
// （箱数与采购量无关，只跟订单要多少有关）。
//
// 注意 order_items.quantity 是**下单口径**（如 480 × "2 Pack"、321 × "Bag"），
// 不等于单品个数，所以一律以 order_item_matches.required_quantity 为准：
//   - 单品行：required_quantity 就是该单品的需求个数
//   - 组合行：required_quantity ÷ quantity_per_set = 套数
//
// 组合行按成员单品展开成多条，调用方需按 OrderItemID 去重。
func (r *PurchaseDemandRepository) ListOrderLineDemand(
	ctx context.Context,
	orderID uint64,
) ([]models.PurchaseDemandLine, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			oi.id,
			COALESCE(m.sku_id, 0),
			COALESCE(m.rule_id, 0),
			COALESCE(r.rule_code, ''),
			COALESCE(NULLIF(r.product_name_zh, ''), r.customer_product_name, ''),
			COALESCE(r.sets_per_carton, 0),
			COALESCE(m.required_quantity, 0),
			COALESCE(NULLIF(m.quantity_per_set, 0), 1),
			COALESCE(oi.quantity, 0)
		FROM order_items oi
		LEFT JOIN order_item_matches m ON m.order_item_id = oi.id
		LEFT JOIN product_match_rules r ON r.id = m.rule_id
		WHERE oi.order_id = ?
		ORDER BY oi.id ASC, m.id ASC`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.PurchaseDemandLine, 0)

	for rows.Next() {
		var item models.PurchaseDemandLine
		if err := rows.Scan(
			&item.OrderItemID,
			&item.SkuID,
			&item.RuleID,
			&item.RuleCode,
			&item.RuleNameZH,
			&item.SetsPerCarton,
			&item.RequiredQuantity,
			&item.QuantityPerSet,
			&item.Quantity,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, rows.Err()
}

// suggestedQuantity 建议采购量 = max(0, 需求 − 库存 − 在途)。
func suggestedQuantity(required, stock, inTransit float64) float64 {
	value := required - stock - inTransit
	if value < 0 {
		return 0
	}
	return value
}

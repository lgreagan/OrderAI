package repository

import (
	"context"
	"database/sql"

	"OrderAI/models"
)

// ProductionDemandRepository 以 orders 表为准，读取订单的生产/装箱需求。
type ProductionDemandRepository struct {
	DB *sql.DB
}

func NewProductionDemandRepository(db *sql.DB) *ProductionDemandRepository {
	return &ProductionDemandRepository{DB: db}
}

// productionTaskSelect 是组合规则任务的公共查询。
//
// 按 (订单行, 组合规则) 分组：套数 = 需求数量 ÷ 每套用量，箱数 = 套数 ÷ 每箱套数。
const productionTaskSelect = `
		SELECT
			oi.id,
			o.id,
			o.order_number,
			o.customer,
			o.country,
			o.created_at,
			oi.line_number,
			oi.product_name,
			oi.specification,
			oi.quantity,
			oi.unit,
			m.rule_id,
			COALESCE(MAX(m.rule_code_snapshot), ''),
			COALESCE(MAX(r.customer_product_name), ''),
			COALESCE(MAX(r.product_name_zh), ''),
			COALESCE(MAX(r.sets_per_carton), 0),
			COALESCE(MAX(r.order_unit), ''),
			COALESCE(MAX(m.required_quantity / NULLIF(m.quantity_per_set, 0)), 0),
			COUNT(*)
		FROM order_item_matches m
		JOIN order_items oi ON oi.id = m.order_item_id
		JOIN orders o ON o.id = oi.order_id
		LEFT JOIN product_match_rules r ON r.id = m.rule_id
		WHERE m.rule_id IS NOT NULL`

const productionTaskGroupBy = `
		GROUP BY
			oi.id, o.id, o.order_number, o.customer, o.country, o.created_at,
			oi.line_number, oi.product_name, oi.specification, oi.quantity, oi.unit,
			m.rule_id`

// GetOrder 查询来源订单表头。
func (r *ProductionDemandRepository) GetOrder(
	ctx context.Context,
	orderID uint64,
) (*models.ProductionDemandSourceOrder, error) {
	order := &models.ProductionDemandSourceOrder{}

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
func (r *ProductionDemandRepository) UpdateOrderStatus(
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

	if affected == 0 {
		if _, err := r.GetOrder(ctx, orderID); err != nil {
			return err
		}
	}

	return nil
}

// ListOrderTasks 列出某个订单命中组合规则的订单行任务。
func (r *ProductionDemandRepository) ListOrderTasks(
	ctx context.Context,
	orderID uint64,
) ([]models.ProductionDemandTask, error) {
	rows, err := r.DB.QueryContext(ctx,
		productionTaskSelect+`
			AND oi.order_id = ?`+productionTaskGroupBy+`
		ORDER BY oi.line_number ASC`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ProductionDemandTask, 0)

	for rows.Next() {
		var task models.ProductionDemandTask

		err := rows.Scan(
			&task.OrderItemID, &task.OrderID, &task.OrderNumber,
			&task.Customer, &task.Country, &task.OrderCreatedAt,
			&task.LineNumber, &task.ProductName, &task.Specification,
			&task.Quantity, &task.Unit,
			&task.RuleID, &task.RuleCode, &task.RuleProductName,
			&task.RuleProductNameZH, &task.SetsPerCarton, &task.OrderUnit,
			&task.Sets, &task.SKUCount,
		)
		if err != nil {
			return nil, err
		}

		if task.SetsPerCarton > 0 {
			task.CartonCount = task.Sets / task.SetsPerCarton
		}

		result = append(result, task)
	}

	return result, rows.Err()
}

// ListDirectItems 查询某个订单中 SKU 直配（未命中组合规则）的组成单品。
func (r *ProductionDemandRepository) ListDirectItems(
	ctx context.Context,
	orderID uint64,
) ([]models.ProductionDemandTaskItem, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			m.sku_id,
			COALESCE(s.sku_code, m.sku_code_snapshot, ''),
			COALESCE(s.product_name, m.product_name_snapshot, ''),
			COALESCE(s.product_name_zh, ''),
			COALESCE(s.short_name, ''),
			COALESCE(s.model, m.model_snapshot, ''),
			COALESCE(s.color, m.color_snapshot, ''),
			COALESCE(s.pack_quantity, 0),
			m.quantity_per_set,
			m.required_quantity
		FROM order_item_matches m
		JOIN order_items oi ON oi.id = m.order_item_id
		LEFT JOIN skus s ON s.id = m.sku_id
		WHERE oi.order_id = ?
			AND m.rule_id IS NULL
			AND NOT EXISTS (
				SELECT 1
				FROM order_item_matches x
				WHERE x.order_item_id = oi.id
					AND x.rule_id IS NOT NULL
			)
		ORDER BY m.order_item_id ASC, m.id ASC`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ProductionDemandTaskItem, 0)

	for rows.Next() {
		var item models.ProductionDemandTaskItem
		err := rows.Scan(
			&item.SkuID, &item.SkuCode, &item.ProductName, &item.ProductNameZH,
			&item.ShortName, &item.Model, &item.Color, &item.PackQuantity,
			&item.QuantityPerSet, &item.RequiredQuantity,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, rows.Err()
}

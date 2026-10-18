package repository

import (
	"context"
	"database/sql"

	"OrderAI/models"
)

// StatisticsRepository 汇总各业务单据的统计数据。
type StatisticsRepository struct {
	DB *sql.DB
}

func NewStatisticsRepository(db *sql.DB) *StatisticsRepository {
	return &StatisticsRepository{DB: db}
}

// groupCounts 执行 "SELECT label, count ... GROUP BY" 查询。
func (r *StatisticsRepository) groupCounts(
	ctx context.Context,
	query string,
) ([]models.StatCount, error) {
	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.StatCount, 0)
	for rows.Next() {
		var item models.StatCount
		if err := rows.Scan(&item.Label, &item.Count); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, rows.Err()
}

// stockAlerts 按条件返回库存预警行。
func (r *StatisticsRepository) stockAlerts(
	ctx context.Context,
	condition string,
	args ...any,
) ([]models.StockAlert, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT t.item_type, t.item_id, t.item_code, t.item_name, t.stock
		FROM (
			SELECT
				v.item_type, v.item_id,
				SUM(v.stock_qty) AS stock,
				COALESCE(NULLIF(s.sku_code, ''), m.code, '') AS item_code,
				COALESCE(
					NULLIF(s.short_name, ''),
					NULLIF(s.product_name_zh, ''),
					m.name_zh, m.name, ''
				) AS item_name
			FROM v_inventory_stock v
			LEFT JOIN skus s ON v.item_type = 'sku' AND s.id = v.item_id
			LEFT JOIN materials m ON v.item_type = 'material' AND m.id = v.item_id
			GROUP BY v.item_type, v.item_id
		) t
		WHERE `+condition+`
		ORDER BY t.stock ASC
		LIMIT 100`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.StockAlert, 0)
	for rows.Next() {
		var a models.StockAlert
		if err := rows.Scan(
			&a.ItemType, &a.ItemID, &a.ItemCode, &a.ItemName, &a.Stock,
		); err != nil {
			return nil, err
		}
		result = append(result, a)
	}

	return result, rows.Err()
}

// Overview 汇总统计总览。
func (r *StatisticsRepository) Overview(
	ctx context.Context,
	lowStockThreshold float64,
) (*models.StatisticsOverview, error) {
	ov := &models.StatisticsOverview{LowStockThreshold: lowStockThreshold}

	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders`).
		Scan(&ov.OrderTotal); err != nil {
		return nil, err
	}

	orderByStatus, err := r.groupCounts(ctx,
		`SELECT status, COUNT(*) FROM orders GROUP BY status ORDER BY status`)
	if err != nil {
		return nil, err
	}
	ov.OrderByStatus = orderByStatus

	if err := r.DB.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(total_amount), 0)
		FROM purchase_orders`).
		Scan(&ov.PurchaseCount, &ov.PurchaseAmount); err != nil {
		return nil, err
	}

	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM production_orders`).
		Scan(&ov.ProductionCount); err != nil {
		return nil, err
	}

	productionByStatus, err := r.groupCounts(ctx,
		`SELECT status, COUNT(*) FROM production_orders GROUP BY status ORDER BY status`)
	if err != nil {
		return nil, err
	}
	ov.ProductionByStatus = productionByStatus

	if err := r.DB.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(freight_cost), 0), COALESCE(SUM(insurance_cost), 0)
		FROM shipments`).
		Scan(&ov.ShipmentCount, &ov.FreightTotal, &ov.InsuranceTotal); err != nil {
		return nil, err
	}

	if err := r.DB.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount), 0),
		       COALESCE(SUM(amount + carryover_amount - paid_amount), 0),
		       COALESCE(SUM(paid_amount), 0)
		FROM proforma_invoices`).
		Scan(&ov.PICount, &ov.PIAmount, &ov.PIUnpaid, &ov.PIPaid); err != nil {
		return nil, err
	}

	if err := r.DB.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM payments`).
		Scan(&ov.PaymentCount, &ov.PaymentAmount); err != nil {
		return nil, err
	}

	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM skus`).
		Scan(&ov.SkuCount); err != nil {
		return nil, err
	}

	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM materials`).
		Scan(&ov.MaterialCount); err != nil {
		return nil, err
	}

	negative, err := r.stockAlerts(ctx, `t.stock < 0`)
	if err != nil {
		return nil, err
	}
	ov.NegativeStock = negative

	if lowStockThreshold > 0 {
		low, err := r.stockAlerts(ctx, `t.stock > 0 AND t.stock <= ?`, lowStockThreshold)
		if err != nil {
			return nil, err
		}
		ov.LowStock = low
	} else {
		ov.LowStock = make([]models.StockAlert, 0)
	}

	return ov, nil
}

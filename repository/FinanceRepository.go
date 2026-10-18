package repository

import (
	"context"
	"database/sql"
	"strings"

	"OrderAI/models"
)

// FinanceRepository 汇总财务/利润数据。
type FinanceRepository struct {
	DB *sql.DB
}

func NewFinanceRepository(db *sql.DB) *FinanceRepository {
	return &FinanceRepository{DB: db}
}

// financeWhere 拼装利润明细的过滤条件（两种分组方式共用）。
//
// 已标记 is_paid 的明细行不计入 PI 金额，这里同样排除，
// 保证报表收入与 PI 单据上的合计一致。
func financeWhere(f models.FinanceFilter) (string, []any) {
	clauses := []string{"i.is_paid = 0"}
	args := []any{}

	if v := strings.TrimSpace(f.DateFrom); v != "" {
		clauses = append(clauses, "p.issued_date >= ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(f.DateTo); v != "" {
		clauses = append(clauses, "p.issued_date <= ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(f.Customer); v != "" {
		clauses = append(clauses, "p.customer_name LIKE ?")
		args = append(args, "%"+v+"%")
	}
	if v := strings.TrimSpace(f.Keyword); v != "" {
		clauses = append(clauses, `(
			o.order_number LIKE ? OR p.pi_number LIKE ? OR p.customer_name LIKE ?
		)`)
		k := "%" + v + "%"
		args = append(args, k, k, k)
	}

	return strings.Join(clauses, " AND "), args
}

// financeAmountColumns 是收入与标准成本的公共聚合列。
//
// 货款/箱费构成销售收入；运费与保险是代收代付，单独列出不进毛利。
// 标准成本取主数据成本价：产品 skus.price、材料 materials.price。
const financeAmountColumns = `
	MIN(p.issued_date) AS issued_date,
	GROUP_CONCAT(DISTINCT p.pi_number ORDER BY p.pi_number SEPARATOR '|') AS pi_numbers,
	COALESCE(SUM(CASE WHEN i.item_type = 'goods' THEN i.amount ELSE 0 END), 0) AS goods_usd,
	COALESCE(SUM(CASE WHEN i.item_type = 'box' THEN i.amount ELSE 0 END), 0) AS box_usd,
	COALESCE(SUM(CASE WHEN i.item_type = 'charge' AND i.charge_type = 'shipping' THEN i.amount ELSE 0 END), 0) AS freight_usd,
	COALESCE(SUM(CASE WHEN i.item_type = 'charge' AND i.charge_type = 'insurance' THEN i.amount ELSE 0 END), 0) AS insurance_usd,
	COALESCE(SUM(CASE WHEN i.item_type = 'goods' THEN i.quantity * COALESCE(s.price, 0) ELSE 0 END), 0)
		+ COALESCE(SUM(CASE WHEN i.item_type = 'box' THEN i.quantity * COALESCE(m.price, 0) ELSE 0 END), 0) AS std_cost_cny,
	COALESCE(SUM(CASE WHEN i.item_type = 'goods' AND i.sku_id IS NOT NULL AND COALESCE(s.price, 0) <= 0 THEN 1 ELSE 0 END), 0) AS missing_goods_cost,
	COALESCE(SUM(CASE WHEN i.item_type = 'box' AND (i.material_id IS NULL OR COALESCE(m.price, 0) <= 0) THEN 1 ELSE 0 END), 0) AS missing_box_cost
`

const financeItemJoins = `
	FROM proforma_invoice_items i
	JOIN proforma_invoices p ON p.id = i.pi_id
	LEFT JOIN orders o ON o.id = i.order_id
	LEFT JOIN skus s ON s.id = i.sku_id
	LEFT JOIN materials m ON m.id = i.material_id
`

// scanFinanceRow 扫描一行：分组键（订单 ID 或 PI ID）+ 收入与标准成本列。
// UnlinkedRows 用局部 key 接收 PI ID，行本身的 OrderID 保持 0。
func scanFinanceRow(
	rows *sql.Rows,
	it *models.FinanceOrderRow,
	key *uint64,
) error {
	return rows.Scan(
		key,
		&it.OrderNumber, &it.Customer, &it.Country,
		&it.IssuedDate, &it.PINumbers,
		&it.GoodsUSD, &it.BoxUSD, &it.FreightUSD, &it.InsuranceUSD,
		&it.StdCostCNY, &it.MissingGoodsCost, &it.MissingBoxCost,
	)
}

// OrderRows 按客户订单汇总收入与标准成本。
func (r *FinanceRepository) OrderRows(
	ctx context.Context,
	f models.FinanceFilter,
) ([]models.FinanceOrderRow, error) {
	where, args := financeWhere(f)

	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			i.order_id,
			COALESCE(o.order_number, ''),
			COALESCE(o.customer, ''),
			COALESCE(o.country, ''),
			`+financeAmountColumns+financeItemJoins+`
		WHERE i.order_id IS NOT NULL AND `+where+`
		GROUP BY i.order_id, o.order_number, o.customer, o.country
		ORDER BY i.order_id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.FinanceOrderRow, 0)
	for rows.Next() {
		var it models.FinanceOrderRow
		if err := scanFinanceRow(rows, &it, &it.OrderID); err != nil {
			return nil, err
		}
		result = append(result, it)
	}

	return result, rows.Err()
}

// UnlinkedRows 汇总未关联客户订单的 PI 明细，按 PI 单独成行。
func (r *FinanceRepository) UnlinkedRows(
	ctx context.Context,
	f models.FinanceFilter,
) ([]models.FinanceOrderRow, error) {
	where, args := financeWhere(f)

	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			i.pi_id,
			'',
			COALESCE(MAX(p.customer_name), ''),
			'',
			`+financeAmountColumns+financeItemJoins+`
		WHERE i.order_id IS NULL AND `+where+`
		GROUP BY i.pi_id
		ORDER BY i.pi_id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.FinanceOrderRow, 0)
	for rows.Next() {
		var key uint64
		var it models.FinanceOrderRow
		if err := scanFinanceRow(rows, &it, &key); err != nil {
			return nil, err
		}
		result = append(result, it)
	}

	return result, rows.Err()
}

// PurchaseCosts 按来源订单汇总实际采购成本，已取消的采购单不计。
func (r *FinanceRepository) PurchaseCosts(
	ctx context.Context,
) (map[uint64]models.FinancePurchaseCost, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			source_order_id,
			COUNT(*),
			COALESCE(SUM(total_amount), 0),
			GROUP_CONCAT(po_number ORDER BY po_number SEPARATOR '|')
		FROM purchase_orders
		WHERE source_order_id IS NOT NULL AND status <> 'cancelled'
		GROUP BY source_order_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[uint64]models.FinancePurchaseCost{}
	for rows.Next() {
		var it models.FinancePurchaseCost
		if err := rows.Scan(
			&it.OrderID, &it.Count, &it.AmountCNY, &it.PONumbers,
		); err != nil {
			return nil, err
		}
		result[it.OrderID] = it
	}

	return result, rows.Err()
}

// ShippedItemByOrder 按订单 + SKU 汇总 PI 出货数量（含产品信息），
// 用于判断采购单是否覆盖出货、并列出采购缺口。
func (r *FinanceRepository) ShippedItemByOrder(
	ctx context.Context,
	f models.FinanceFilter,
) (map[uint64]map[uint64]models.FinanceShippedItem, error) {
	where, args := financeWhere(f)

	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			i.order_id, i.sku_id, COALESCE(SUM(i.quantity), 0),
			COALESCE(MAX(s.sku_code), ''),
			COALESCE(MAX(NULLIF(s.short_name, '')), MAX(s.product_name_zh), ''),
			COALESCE(MAX(s.model), ''),
			COALESCE(MAX(i.unit), '')
		FROM proforma_invoice_items i
		JOIN proforma_invoices p ON p.id = i.pi_id
		LEFT JOIN orders o ON o.id = i.order_id
		LEFT JOIN skus s ON s.id = i.sku_id
		WHERE i.order_id IS NOT NULL
			AND i.item_type = 'goods'
			AND i.sku_id IS NOT NULL
			AND `+where+`
		GROUP BY i.order_id, i.sku_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[uint64]map[uint64]models.FinanceShippedItem{}
	for rows.Next() {
		var it models.FinanceShippedItem
		if err := rows.Scan(
			&it.OrderID, &it.SkuID, &it.Quantity,
			&it.SkuCode, &it.SkuName, &it.Spec, &it.Unit,
		); err != nil {
			return nil, err
		}

		if result[it.OrderID] == nil {
			result[it.OrderID] = map[uint64]models.FinanceShippedItem{}
		}
		item := result[it.OrderID][it.SkuID]
		item.OrderID = it.OrderID
		item.SkuID = it.SkuID
		item.Quantity += it.Quantity
		item.SkuCode = it.SkuCode
		item.SkuName = it.SkuName
		item.Spec = it.Spec
		item.Unit = it.Unit
		result[it.OrderID][it.SkuID] = item
	}

	return result, rows.Err()
}

// PurchasedItemByOrder 按订单 + SKU 汇总采购数量与涉及的单号（不含已取消采购单）。
func (r *FinanceRepository) PurchasedItemByOrder(
	ctx context.Context,
) (map[uint64]map[uint64]models.FinancePurchasedItem, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			po.source_order_id, poi.item_id,
			COALESCE(SUM(poi.quantity), 0),
			GROUP_CONCAT(DISTINCT po.po_number ORDER BY po.po_number SEPARATOR '|')
		FROM purchase_order_items poi
		JOIN purchase_orders po ON po.id = poi.purchase_order_id
		WHERE po.source_order_id IS NOT NULL
			AND po.status <> 'cancelled'
			AND poi.item_type = 'sku'
			AND poi.item_id IS NOT NULL
		GROUP BY po.source_order_id, poi.item_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[uint64]map[uint64]models.FinancePurchasedItem{}
	for rows.Next() {
		var it models.FinancePurchasedItem
		if err := rows.Scan(&it.OrderID, &it.SkuID, &it.Quantity, &it.PONumbers); err != nil {
			return nil, err
		}

		if result[it.OrderID] == nil {
			result[it.OrderID] = map[uint64]models.FinancePurchasedItem{}
		}
		result[it.OrderID][it.SkuID] = it
	}

	return result, rows.Err()
}

// ShipmentCharges 逐单取出发货单录入的运费与保险（保留币种与发货日期）。
//
// 不在这里 SUM：不同发货单的币种可能不同，人民币单要先按当月汇率折算成美元，
// 再和 PI 的「向客户收取」比较，否则会出现假的代收代付差额。
func (r *FinanceRepository) ShipmentCharges(
	ctx context.Context,
) ([]models.FinanceShipmentCharge, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			order_id,
			COALESCE(currency, ''),
			ship_date,
			COALESCE(freight_cost, 0),
			COALESCE(insurance_cost, 0)
		FROM shipments
		WHERE order_id IS NOT NULL
		ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.FinanceShipmentCharge, 0)
	for rows.Next() {
		var it models.FinanceShipmentCharge
		if err := rows.Scan(
			&it.OrderID, &it.Currency, &it.ShipDate, &it.FreightCost, &it.InsuranceCost,
		); err != nil {
			return nil, err
		}
		result = append(result, it)
	}

	return result, rows.Err()
}

// PISummary 汇总 PI 的开票、已收与未收金额（未收含历史欠款）。
func (r *FinanceRepository) PISummary(
	ctx context.Context,
	f models.FinanceFilter,
) (*models.FinancePISummary, error) {
	clauses := []string{"1 = 1"}
	args := []any{}

	if v := strings.TrimSpace(f.DateFrom); v != "" {
		clauses = append(clauses, "issued_date >= ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(f.DateTo); v != "" {
		clauses = append(clauses, "issued_date <= ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(f.Customer); v != "" {
		clauses = append(clauses, "customer_name LIKE ?")
		args = append(args, "%"+v+"%")
	}
	if v := strings.TrimSpace(f.Keyword); v != "" {
		k := "%" + v + "%"
		clauses = append(clauses, "(pi_number LIKE ? OR customer_name LIKE ?)")
		args = append(args, k, k)
	}

	summary := &models.FinancePISummary{}
	err := r.DB.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(amount), 0),
			COALESCE(SUM(paid_amount), 0),
			COALESCE(SUM(amount + carryover_amount - paid_amount), 0)
		FROM proforma_invoices
		WHERE `+strings.Join(clauses, " AND "), args...).
		Scan(&summary.Count, &summary.AmountUSD, &summary.PaidUSD, &summary.UnpaidUSD)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

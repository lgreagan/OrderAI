package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"OrderAI/models"
)

// ProformaInvoiceRepository 读写形式发票（头 + 明细）。
type ProformaInvoiceRepository struct {
	DB *sql.DB
}

func NewProformaInvoiceRepository(db *sql.DB) *ProformaInvoiceRepository {
	return &ProformaInvoiceRepository{DB: db}
}

const piItemColumns = `
	id, pi_id, item_type, group_no, group_title, order_id, shipment_id,
	sku_id, material_id, model, short_name, description, quantity, unit,
	unit_price, amount, is_paid, charge_type, title, sort_order, remark
`

// piTotalUnpaidExpr 累计未收是派生值：本期合计 + 历史欠款 − 已收款。
// 已收款由收款管理回写，读取时重算可避免历史数据偏离。
const piTotalUnpaidExpr = `amount + carryover_amount - paid_amount`

func scanPIItem(row interface {
	Scan(...any) error
}) (*models.ProformaInvoiceItem, error) {
	it := &models.ProformaInvoiceItem{}
	err := row.Scan(
		&it.ID, &it.PIID, &it.ItemType, &it.GroupNo, &it.GroupTitle,
		&it.OrderID, &it.ShipmentID, &it.SkuID, &it.MaterialID,
		&it.Model, &it.ShortName, &it.Description, &it.Quantity, &it.Unit,
		&it.UnitPrice, &it.Amount, &it.IsPaid, &it.ChargeType, &it.Title,
		&it.SortOrder, &it.Remark,
	)
	if err != nil {
		return nil, err
	}
	return it, nil
}

// List 查询 PI 列表。
func (r *ProformaInvoiceRepository) List(
	ctx context.Context,
) ([]models.ProformaInvoiceListItem, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			id, pi_number, customer_name, issued_date, currency, status,
			item_amount, box_amount, insurance_amount, freight_amount,
			amount, carryover_amount, `+piTotalUnpaidExpr+` AS total_unpaid,
			paid_amount, created_at
		FROM proforma_invoices
		ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ProformaInvoiceListItem, 0)
	for rows.Next() {
		var it models.ProformaInvoiceListItem
		if err := rows.Scan(
			&it.ID, &it.PINumber, &it.CustomerName, &it.IssuedDate, &it.Currency,
			&it.Status, &it.ItemAmount, &it.BoxAmount, &it.InsuranceAmount,
			&it.FreightAmount, &it.Amount, &it.CarryoverAmount, &it.TotalUnpaid,
			&it.PaidAmount, &it.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, it)
	}

	return result, rows.Err()
}

// SumAmountByCustomer 统计某客户的历史 PI 本期合计（用于计算历史欠款）。
func (r *ProformaInvoiceRepository) SumAmountByCustomer(
	ctx context.Context,
	customerName string,
	excludeID uint64,
) (float64, error) {
	var total float64
	err := r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM proforma_invoices
		WHERE customer_name = ? AND id <> ?`,
		customerName, excludeID,
	).Scan(&total)
	return total, err
}

// SetStockNote 更新某 PI 的库存说明文本。
func (r *ProformaInvoiceRepository) SetStockNote(
	ctx context.Context,
	id uint64,
	note string,
) error {
	_, err := r.DB.ExecContext(
		ctx, `UPDATE proforma_invoices SET stock_note = ? WHERE id = ?`, note, id,
	)
	return err
}

// SetPaidAmount 更新某 PI 的已收款金额，并同步累计未收
// （累计未收 = 本期合计 + 历史欠款 − 已收款）。
func (r *ProformaInvoiceRepository) SetPaidAmount(
	ctx context.Context,
	id uint64,
	paid float64,
) error {
	_, err := r.DB.ExecContext(
		ctx,
		`UPDATE proforma_invoices
		 SET paid_amount = ?, total_unpaid = amount + carryover_amount - ?
		 WHERE id = ?`,
		paid, paid, id,
	)
	return err
}

// ListItems 查询某 PI 的全部明细。
func (r *ProformaInvoiceRepository) ListItems(
	ctx context.Context,
	piID uint64,
) ([]models.ProformaInvoiceItem, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT `+piItemColumns+`
		FROM proforma_invoice_items
		WHERE pi_id = ?
		ORDER BY sort_order ASC, id ASC`, piID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ProformaInvoiceItem, 0)
	for rows.Next() {
		it, err := scanPIItem(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *it)
	}

	return result, rows.Err()
}

// GetByID 查询 PI 头 + 明细。
func (r *ProformaInvoiceRepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.ProformaInvoice, error) {
	p := &models.ProformaInvoice{}
	err := r.DB.QueryRowContext(ctx, `
		SELECT
			id, pi_number, customer_name, issued_date, currency, status,
			item_amount, box_amount, insurance_amount, freight_amount,
			amount, carryover_amount, `+piTotalUnpaidExpr+` AS total_unpaid,
			paid_amount, stock_note, remark, created_at, updated_at
		FROM proforma_invoices
		WHERE id = ?`, id,
	).Scan(
		&p.ID, &p.PINumber, &p.CustomerName, &p.IssuedDate, &p.Currency,
		&p.Status, &p.ItemAmount, &p.BoxAmount, &p.InsuranceAmount,
		&p.FreightAmount, &p.Amount, &p.CarryoverAmount, &p.TotalUnpaid,
		&p.PaidAmount, &p.StockNote, &p.Remark,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrProformaInvoiceNotFound
		}
		return nil, err
	}

	items, err := r.ListItems(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Items = items

	return p, nil
}

// NextNumber 生成 PI 单号：PI-yyyyMMdd[-NN]。
func (r *ProformaInvoiceRepository) NextNumber(
	ctx context.Context,
) (string, error) {
	today := time.Now().Format("20060102")
	prefix := "PI-" + today

	var count int
	err := r.DB.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM proforma_invoices WHERE pi_number LIKE ?`,
		prefix+"%",
	).Scan(&count)
	if err != nil {
		return "", err
	}

	if count == 0 {
		return prefix, nil
	}
	return fmt.Sprintf("%s-%02d", prefix, count+1), nil
}

func insertPIItems(
	ctx context.Context,
	tx *sql.Tx,
	piID uint64,
	items []models.ProformaInvoiceItem,
) error {
	for i := range items {
		it := &items[i]
		_, err := tx.ExecContext(ctx, `
			INSERT INTO proforma_invoice_items (
				pi_id, item_type, group_no, group_title, order_id, shipment_id,
				sku_id, material_id, model, short_name, description, quantity,
				unit, unit_price, amount, is_paid, charge_type, title, sort_order, remark
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			piID, it.ItemType, it.GroupNo, it.GroupTitle, it.OrderID, it.ShipmentID,
			it.SkuID, it.MaterialID, it.Model, it.ShortName, it.Description, it.Quantity,
			it.Unit, it.UnitPrice, it.Amount, it.IsPaid, it.ChargeType, it.Title,
			it.SortOrder, it.Remark,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ProformaInvoiceRepository) insertHeader(
	ctx context.Context,
	tx *sql.Tx,
	p *models.ProformaInvoice,
) error {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO proforma_invoices (
			pi_number, customer_name, issued_date, currency, status,
			item_amount, box_amount, insurance_amount, freight_amount,
			amount, carryover_amount, total_unpaid, paid_amount,
			stock_note, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.PINumber, p.CustomerName, p.IssuedDate, p.Currency, p.Status,
		p.ItemAmount, p.BoxAmount, p.InsuranceAmount, p.FreightAmount,
		p.Amount, p.CarryoverAmount, p.TotalUnpaid, p.PaidAmount,
		p.StockNote, p.Remark,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = uint64(id)
	return nil
}

// Create 新增 PI（头 + 明细，事务）。
func (r *ProformaInvoiceRepository) Create(
	ctx context.Context,
	p *models.ProformaInvoice,
) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := r.insertHeader(ctx, tx, p); err != nil {
		return err
	}
	if err := insertPIItems(ctx, tx, p.ID, p.Items); err != nil {
		return err
	}

	return tx.Commit()
}

// Update 更新 PI（重写明细，事务）。
func (r *ProformaInvoiceRepository) Update(
	ctx context.Context,
	p *models.ProformaInvoice,
) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		UPDATE proforma_invoices SET
			pi_number = ?, customer_name = ?, issued_date = ?, currency = ?,
			status = ?, item_amount = ?, box_amount = ?, insurance_amount = ?,
			freight_amount = ?, amount = ?, carryover_amount = ?,
			total_unpaid = ?, paid_amount = ?, stock_note = ?, remark = ?
		WHERE id = ?`,
		p.PINumber, p.CustomerName, p.IssuedDate, p.Currency,
		p.Status, p.ItemAmount, p.BoxAmount, p.InsuranceAmount,
		p.FreightAmount, p.Amount, p.CarryoverAmount,
		p.TotalUnpaid, p.PaidAmount, p.StockNote, p.Remark,
		p.ID,
	)
	if err != nil {
		return err
	}

	if _, err = tx.ExecContext(
		ctx, `DELETE FROM proforma_invoice_items WHERE pi_id = ?`, p.ID,
	); err != nil {
		return err
	}

	if err := insertPIItems(ctx, tx, p.ID, p.Items); err != nil {
		return err
	}

	return tx.Commit()
}

// Delete 删除 PI（明细级联删除）。
func (r *ProformaInvoiceRepository) Delete(
	ctx context.Context,
	id uint64,
) error {
	result, err := r.DB.ExecContext(
		ctx, `DELETE FROM proforma_invoices WHERE id = ?`, id,
	)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return models.ErrProformaInvoiceNotFound
	}
	return nil
}

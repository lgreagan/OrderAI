package repository

import (
	"context"
	"database/sql"
	"strings"

	"OrderAI/models"
)

// PaymentRepository 读写收款记录。
type PaymentRepository struct {
	DB *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{DB: db}
}

const paymentColumns = `
	p.id, p.customer_name, p.pi_id, COALESCE(pi.pi_number, ''),
	p.amount, p.currency, p.paid_date, p.method, p.reference, p.remark,
	p.created_at
`

func scanPayment(row interface {
	Scan(...any) error
}) (*models.Payment, error) {
	p := &models.Payment{}
	err := row.Scan(
		&p.ID, &p.CustomerName, &p.PIID, &p.PINumber,
		&p.Amount, &p.Currency, &p.PaidDate, &p.Method, &p.Reference,
		&p.Remark, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// List 查询收款记录（可按客户/关键字过滤）。
func (r *PaymentRepository) List(
	ctx context.Context,
	customerName, keyword string,
) ([]models.Payment, error) {
	query := `SELECT ` + paymentColumns + `
		FROM payments p
		LEFT JOIN proforma_invoices pi ON pi.id = p.pi_id
		WHERE 1 = 1`
	args := []any{}

	if customerName = strings.TrimSpace(customerName); customerName != "" {
		query += ` AND p.customer_name = ?`
		args = append(args, customerName)
	}

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		k := "%" + keyword + "%"
		query += ` AND (
			p.customer_name LIKE ? OR pi.pi_number LIKE ? OR
			p.method LIKE ? OR p.reference LIKE ? OR p.remark LIKE ?
		)`
		args = append(args, k, k, k, k, k)
	}

	query += ` ORDER BY p.id DESC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.Payment, 0)
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *p)
	}

	return result, rows.Err()
}

func (r *PaymentRepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.Payment, error) {
	p, err := scanPayment(r.DB.QueryRowContext(ctx, `
		SELECT `+paymentColumns+`
		FROM payments p
		LEFT JOIN proforma_invoices pi ON pi.id = p.pi_id
		WHERE p.id = ?`, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrPaymentNotFound
		}
		return nil, err
	}
	return p, nil
}

// Create 新增收款。
func (r *PaymentRepository) Create(
	ctx context.Context,
	p *models.Payment,
) error {
	result, err := r.DB.ExecContext(ctx, `
		INSERT INTO payments (
			customer_name, pi_id, amount, currency, paid_date,
			method, reference, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.CustomerName, p.PIID, p.Amount, p.Currency, p.PaidDate,
		p.Method, p.Reference, p.Remark,
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

// Update 更新收款。
func (r *PaymentRepository) Update(
	ctx context.Context,
	p *models.Payment,
) error {
	result, err := r.DB.ExecContext(ctx, `
		UPDATE payments SET
			customer_name = ?, pi_id = ?, amount = ?, currency = ?,
			paid_date = ?, method = ?, reference = ?, remark = ?
		WHERE id = ?`,
		p.CustomerName, p.PIID, p.Amount, p.Currency, p.PaidDate,
		p.Method, p.Reference, p.Remark, p.ID,
	)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		if _, err := r.GetByID(ctx, p.ID); err != nil {
			return err
		}
	}
	return nil
}

// Delete 删除收款。
func (r *PaymentRepository) Delete(ctx context.Context, id uint64) error {
	result, err := r.DB.ExecContext(ctx, `DELETE FROM payments WHERE id = ?`, id)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return models.ErrPaymentNotFound
	}
	return nil
}

// SumByCustomer 统计某客户的收款合计（用于计算历史欠款）。
func (r *PaymentRepository) SumByCustomer(
	ctx context.Context,
	customerName string,
) (float64, error) {
	var total float64
	err := r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM payments WHERE customer_name = ?`,
		customerName,
	).Scan(&total)
	return total, err
}

// SumByPI 统计某 PI 的收款合计。
func (r *PaymentRepository) SumByPI(
	ctx context.Context,
	piID uint64,
) (float64, error) {
	var total float64
	err := r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM payments WHERE pi_id = ?`,
		piID,
	).Scan(&total)
	return total, err
}

// DistinctCustomers 返回有收款记录或 PI 的客户名，供前端下拉。
func (r *PaymentRepository) DistinctCustomers(ctx context.Context) ([]string, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT DISTINCT customer_name FROM (
			SELECT customer_name FROM payments WHERE customer_name <> ''
			UNION
			SELECT customer_name FROM proforma_invoices WHERE customer_name <> ''
		) t ORDER BY customer_name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}

	return result, rows.Err()
}

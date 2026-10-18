package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"OrderAI/models"
)

// WarehouseAddressRepository 读写仓库地址库（FBA 仓库地址）。
type WarehouseAddressRepository struct {
	DB *sql.DB
}

func NewWarehouseAddressRepository(db *sql.DB) *WarehouseAddressRepository {
	return &WarehouseAddressRepository{DB: db}
}

const warehouseAddressColumns = `
	id, short_name, code, fba_code, contact, company, tel, mobile,
	address1, address2, address3, city, state, country, zip,
	COALESCE(declare_extra, ''), vat_no, status, created_at, updated_at
`

func scanWarehouseAddress(row interface {
	Scan(...any) error
}) (*models.WarehouseAddress, error) {
	a := &models.WarehouseAddress{}
	err := row.Scan(
		&a.ID, &a.ShortName, &a.Code, &a.FBACode, &a.Contact, &a.Company,
		&a.Tel, &a.Mobile,
		&a.Address1, &a.Address2, &a.Address3, &a.City, &a.State, &a.Country, &a.Zip,
		&a.DeclareExtra, &a.VatNo, &a.Status, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// List 查询仓库地址；country 为空表示全部，limit<=0 时默认 200 条。
func (r *WarehouseAddressRepository) List(
	ctx context.Context,
	keyword, country string,
	limit int,
) ([]models.WarehouseAddress, error) {
	if limit <= 0 {
		limit = 200
	}

	query := `SELECT ` + warehouseAddressColumns + ` FROM warehouse_addresses WHERE status = 'active'`
	args := []any{}

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		k := "%" + keyword + "%"
		query += ` AND (short_name LIKE ? OR code LIKE ? OR city LIKE ? OR address1 LIKE ?)`
		args = append(args, k, k, k, k)
	}
	if country = strings.TrimSpace(country); country != "" {
		query += ` AND country = ?`
		args = append(args, country)
	}
	query += ` ORDER BY country ASC, short_name ASC LIMIT ?`
	args = append(args, limit)

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.WarehouseAddress, 0)
	for rows.Next() {
		a, err := scanWarehouseAddress(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *a)
	}
	return result, rows.Err()
}

// GetByShortName 按地址简称查询（报关填「地址库编码」即按简称带出）。
func (r *WarehouseAddressRepository) GetByShortName(
	ctx context.Context,
	shortName string,
) (*models.WarehouseAddress, error) {
	name := strings.TrimSpace(shortName)
	if name == "" {
		return nil, nil
	}
	row := r.DB.QueryRowContext(ctx,
		`SELECT `+warehouseAddressColumns+` FROM warehouse_addresses WHERE short_name = ? LIMIT 1`, name)
	a, err := scanWarehouseAddress(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

// GetByID 按主键查询。
func (r *WarehouseAddressRepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.WarehouseAddress, error) {
	row := r.DB.QueryRowContext(ctx,
		`SELECT `+warehouseAddressColumns+` FROM warehouse_addresses WHERE id = ?`, id)
	a, err := scanWarehouseAddress(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("仓库地址不存在")
		}
		return nil, err
	}
	return a, nil
}

// Countries 返回地址库里已有的国家代码（用于筛选）。
func (r *WarehouseAddressRepository) Countries(ctx context.Context) ([]string, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT country FROM warehouse_addresses WHERE country <> '' GROUP BY country ORDER BY country ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// Create 新增仓库地址。
func (r *WarehouseAddressRepository) Create(
	ctx context.Context,
	a *models.WarehouseAddress,
) error {
	normalizeWarehouseAddress(a)

	result, err := r.DB.ExecContext(ctx, `
		INSERT INTO warehouse_addresses (
			short_name, code, fba_code, contact, company, tel, mobile,
			address1, address2, address3, city, state, country, zip,
			declare_extra, vat_no, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ShortName, a.Code, a.FBACode, a.Contact, a.Company, a.Tel, a.Mobile,
		a.Address1, a.Address2, a.Address3, a.City, a.State, a.Country, a.Zip,
		a.DeclareExtra, a.VatNo, a.Status,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	a.ID = uint64(id)
	return nil
}

// Update 更新仓库地址。
func (r *WarehouseAddressRepository) Update(
	ctx context.Context,
	a *models.WarehouseAddress,
) error {
	normalizeWarehouseAddress(a)

	result, err := r.DB.ExecContext(ctx, `
		UPDATE warehouse_addresses SET
			short_name = ?, code = ?, fba_code = ?, contact = ?, company = ?,
			tel = ?, mobile = ?,
			address1 = ?, address2 = ?, address3 = ?, city = ?, state = ?,
			country = ?, zip = ?, declare_extra = ?, vat_no = ?, status = ?
		WHERE id = ?`,
		a.ShortName, a.Code, a.FBACode, a.Contact, a.Company, a.Tel, a.Mobile,
		a.Address1, a.Address2, a.Address3, a.City, a.State, a.Country, a.Zip,
		a.DeclareExtra, a.VatNo, a.Status,
		a.ID,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		if _, err := r.GetByID(ctx, a.ID); err != nil {
			return err
		}
	}
	return nil
}

// Delete 删除仓库地址。
func (r *WarehouseAddressRepository) Delete(ctx context.Context, id uint64) error {
	result, err := r.DB.ExecContext(ctx, `DELETE FROM warehouse_addresses WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("仓库地址不存在")
	}
	return nil
}

func normalizeWarehouseAddress(a *models.WarehouseAddress) {
	a.ShortName = strings.TrimSpace(a.ShortName)
	a.Code = strings.TrimSpace(a.Code)
	a.FBACode = strings.TrimSpace(a.FBACode)
	a.Contact = strings.TrimSpace(a.Contact)
	a.Company = strings.TrimSpace(a.Company)
	a.Tel = strings.TrimSpace(a.Tel)
	a.Mobile = strings.TrimSpace(a.Mobile)
	a.Address1 = strings.TrimSpace(a.Address1)
	a.Address2 = strings.TrimSpace(a.Address2)
	a.Address3 = strings.TrimSpace(a.Address3)
	a.City = strings.TrimSpace(a.City)
	a.State = strings.TrimSpace(a.State)
	a.Country = strings.TrimSpace(a.Country)
	a.Zip = strings.TrimSpace(a.Zip)
	a.VatNo = strings.TrimSpace(a.VatNo)
	if a.Status == "" {
		a.Status = "active"
	}
}

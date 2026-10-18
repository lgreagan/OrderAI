package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"OrderAI/models"
)

// DeclarationTemplateRepository 读写报关模板登记（含单元格映射 JSON）。
type DeclarationTemplateRepository struct {
	DB *sql.DB
}

func NewDeclarationTemplateRepository(db *sql.DB) *DeclarationTemplateRepository {
	return &DeclarationTemplateRepository{DB: db}
}

const declarationTemplateColumns = `
	id, name, country, carrier, ship_type, file_path, source_file,
	COALESCE(mapping, ''), remark, status, created_at, updated_at
`

func scanDeclarationTemplate(row interface {
	Scan(...any) error
}) (*models.DeclarationTemplate, error) {
	t := &models.DeclarationTemplate{}
	err := row.Scan(
		&t.ID, &t.Name, &t.Country, &t.Carrier, &t.ShipType, &t.FilePath, &t.SourceFile,
		&t.Mapping, &t.Remark, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// List 查询全部启用的报关模板。
func (r *DeclarationTemplateRepository) List(ctx context.Context) ([]models.DeclarationTemplate, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT `+declarationTemplateColumns+` FROM declaration_templates
		 WHERE status = 'active' ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.DeclarationTemplate, 0)
	for rows.Next() {
		t, err := scanDeclarationTemplate(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *t)
	}
	return result, rows.Err()
}

// GetByID 按主键查询。
func (r *DeclarationTemplateRepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.DeclarationTemplate, error) {
	row := r.DB.QueryRowContext(ctx,
		`SELECT `+declarationTemplateColumns+` FROM declaration_templates WHERE id = ?`, id)
	t, err := scanDeclarationTemplate(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("报关模板不存在")
		}
		return nil, err
	}
	return t, nil
}

// GetByCountry 按国家取第一个启用模板（发货单未指定模板时的兜底）。
func (r *DeclarationTemplateRepository) GetByCountry(
	ctx context.Context,
	country string,
) (*models.DeclarationTemplate, error) {
	name := strings.ToUpper(strings.TrimSpace(country))
	if name == "" {
		return nil, nil
	}
	row := r.DB.QueryRowContext(ctx,
		`SELECT `+declarationTemplateColumns+` FROM declaration_templates
		 WHERE status = 'active' AND country = ? ORDER BY id ASC LIMIT 1`, name)
	t, err := scanDeclarationTemplate(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

// UpdateMapping 只更新映射（模板管理页保存映射时用）。
func (r *DeclarationTemplateRepository) UpdateMapping(
	ctx context.Context,
	id uint64,
	mapping string,
) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE declaration_templates SET mapping = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		mapping, id)
	return err
}

package repository

import (
	"context"
	"database/sql"

	"OrderAI/models"
)

// DeclarationItemRow 报关装配用的发货明细行（已联出 SKU 与报关品类）。
type DeclarationItemRow struct {
	ItemID       uint64
	ShortName    string
	Description  string
	Quantity     float64
	CartonCount  float64
	PackQuantity float64

	CartonNo     string
	PONumber     string
	CartonWeight float64
	CartonLength float64
	CartonWidth  float64
	CartonHeight float64

	UnitPrice float64
	SkuID     sql.NullInt64

	SKUCode      string
	ProductName  string
	ProductNameZ string
	SKUImage     string
	SKUPriceUSD  float64
	SKUDeclare   float64 // skus.declare_price：SKU 自己的报关申报单价，0 表示用品类默认价
	SKUHSCode    string
	SKUASIN      string
	SKUFNSKU     string
	BoxWeight    float64
	BoxLength    float64
	BoxWidth     float64
	BoxHeight    float64

	CategoryID       uint64
	CategoryNameEN   string
	CategoryNameZH   string
	CategoryMaterial string
	CategoryHSCode   string
	CategoryPurpose  string
	CategoryBrand    string
	CategoryModel    string
	CategoryURL      string
	CategoryPrice    float64
	CategoryImage    string
}

// DeclarationRepository 为报关资料装配读取数据。
type DeclarationRepository struct {
	DB *sql.DB
}

func NewDeclarationRepository(db *sql.DB) *DeclarationRepository {
	return &DeclarationRepository{DB: db}
}

// ListSKUItems 返回某发货单的产品类明细（已联出 SKU 与报关品类）。
// 材料类明细（item_type = 'material'）不进报关资料，直接排除。
func (r *DeclarationRepository) ListSKUItems(
	ctx context.Context,
	shipmentID uint64,
) ([]DeclarationItemRow, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT
			si.id, COALESCE(si.short_name, ''), COALESCE(si.description, ''),
			si.quantity, si.carton_count, si.pack_quantity,
			COALESCE(si.carton_no, ''), COALESCE(si.po_number, ''),
			si.carton_weight, si.carton_length, si.carton_width, si.carton_height,
			si.unit_price, si.sku_id,
			COALESCE(s.sku_code, ''), COALESCE(s.product_name, ''),
			COALESCE(s.product_name_zh, ''), COALESCE(s.image, ''),
			COALESCE(s.price_usd, 0), COALESCE(s.declare_price, 0), COALESCE(s.hs_code, ''),
			COALESCE(s.asin, ''), COALESCE(s.fnsku, ''),
			COALESCE(s.box_weight, 0), COALESCE(s.box_length, 0),
			COALESCE(s.box_width, 0), COALESCE(s.box_height, 0),
			COALESCE(s.product_category_id, 0),
			COALESCE(c.name_en, ''), COALESCE(c.name_zh, ''), COALESCE(c.material, ''),
			COALESCE(c.hs_code, ''), COALESCE(c.purpose, ''), COALESCE(c.brand, ''),
			COALESCE(c.model, ''), COALESCE(c.product_url, ''),
			COALESCE(c.declare_price, 0), COALESCE(c.image, '')
		FROM shipment_items si
		LEFT JOIN skus s ON s.id = si.sku_id
		LEFT JOIN product_categories c ON c.id = s.product_category_id
		WHERE si.shipment_id = ? AND si.item_type = 'sku'
		ORDER BY si.id ASC`, shipmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]DeclarationItemRow, 0)
	for rows.Next() {
		var it DeclarationItemRow
		if err := rows.Scan(
			&it.ItemID, &it.ShortName, &it.Description,
			&it.Quantity, &it.CartonCount, &it.PackQuantity,
			&it.CartonNo, &it.PONumber,
			&it.CartonWeight, &it.CartonLength, &it.CartonWidth, &it.CartonHeight,
			&it.UnitPrice, &it.SkuID,
			&it.SKUCode, &it.ProductName,
			&it.ProductNameZ, &it.SKUImage,
			&it.SKUPriceUSD, &it.SKUDeclare, &it.SKUHSCode,
			&it.SKUASIN, &it.SKUFNSKU,
			&it.BoxWeight, &it.BoxLength,
			&it.BoxWidth, &it.BoxHeight,
			&it.CategoryID,
			&it.CategoryNameEN, &it.CategoryNameZH, &it.CategoryMaterial,
			&it.CategoryHSCode, &it.CategoryPurpose, &it.CategoryBrand,
			&it.CategoryModel, &it.CategoryURL,
			&it.CategoryPrice, &it.CategoryImage,
		); err != nil {
			return nil, err
		}
		result = append(result, it)
	}
	return result, rows.Err()
}

const declarationRecordColumns = `
	id, shipment_id, template_id, template_name, order_no, file_name,
	line_count, carton_sum, COALESCE(doc, ''), created_at
`

// InsertRecord 留档一次导出快照。
func (r *DeclarationRepository) InsertRecord(
	ctx context.Context,
	rec *models.DeclarationRecord,
) error {
	result, err := r.DB.ExecContext(ctx, `
		INSERT INTO declaration_records (
			shipment_id, template_id, template_name, order_no, file_name,
			line_count, carton_sum, doc
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		rec.ShipmentID, rec.TemplateID, rec.TemplateName, rec.OrderNo, rec.FileName,
		rec.LineCount, rec.CartonSum, rec.Doc,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	rec.ID = uint64(id)
	return nil
}

// ListRecords 取某发货单的导出留档（时间倒序）。
func (r *DeclarationRepository) ListRecords(
	ctx context.Context,
	shipmentID uint64,
	limit int,
) ([]models.DeclarationRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	rows, err := r.DB.QueryContext(ctx, `
		SELECT `+declarationRecordColumns+`
		FROM declaration_records
		WHERE shipment_id = ?
		ORDER BY id DESC
		LIMIT ?`, shipmentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.DeclarationRecord, 0)
	for rows.Next() {
		var rec models.DeclarationRecord
		if err := rows.Scan(
			&rec.ID, &rec.ShipmentID, &rec.TemplateID, &rec.TemplateName, &rec.OrderNo,
			&rec.FileName, &rec.LineCount, &rec.CartonSum, &rec.Doc, &rec.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, rec)
	}
	return result, rows.Err()
}

// CategoriesByIDs 按 ID 批量取品类（备用）。
func (r *DeclarationRepository) CategoriesByIDs(
	ctx context.Context,
	ids []uint64,
) (map[uint64]models.ProductCategory, error) {
	result := make(map[uint64]models.ProductCategory)
	if len(ids) == 0 {
		return result, nil
	}

	query := `SELECT ` + productCategoryColumns + ` FROM product_categories c WHERE c.id IN (`
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		if i > 0 {
			query += ","
		}
		query += "?"
		args = append(args, id)
	}
	query += ")"

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		c, err := scanProductCategory(rows)
		if err != nil {
			return nil, err
		}
		result[c.ID] = *c
	}
	return result, rows.Err()
}

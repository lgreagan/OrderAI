package repository

import (
	"context"
	"database/sql"
	"strings"

	"OrderAI/models"
)

// PackInfoRepository 查询产品装箱信息（组合套取规则箱规，单品取 SKU 箱规）。
type PackInfoRepository struct {
	DB *sql.DB
}

func NewPackInfoRepository(db *sql.DB) *PackInfoRepository {
	return &PackInfoRepository{DB: db}
}

// ListPackInfo 批量查询产品装箱信息（组合套取规则箱规，单品取 SKU 箱规）。
func (r *PackInfoRepository) ListPackInfo(
	ctx context.Context,
	ruleIDs, skuIDs []uint64,
) (map[string]models.PackInfo, error) {
	result := make(map[string]models.PackInfo)

	if len(ruleIDs) > 0 {
		ph := make([]string, 0, len(ruleIDs))
		args := make([]any, 0, len(ruleIDs))
		for _, id := range ruleIDs {
			ph = append(ph, "?")
			args = append(args, id)
		}

		rows, err := r.DB.QueryContext(ctx, `
			SELECT
				id,
				COALESCE(NULLIF(rule_code, ''), ''),
				COALESCE(NULLIF(product_name_zh, ''), customer_product_name, ''),
				COALESCE(sets_per_carton, 0),
				COALESCE(box_length, 0), COALESCE(box_width, 0), COALESCE(box_height, 0)
			FROM product_match_rules
			WHERE id IN (`+strings.Join(ph, ",")+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var info models.PackInfo
			info.ProductType = models.ProductionProductRule
			if err := rows.Scan(
				&info.ProductRefID, &info.Code, &info.Name, &info.PackPerCarton,
				&info.Length, &info.Width, &info.Height,
			); err != nil {
				rows.Close()
				return nil, err
			}
			info.Unit = "cm"
			result[models.PackKey(info.ProductType, info.ProductRefID)] = info
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	if len(skuIDs) > 0 {
		ph := make([]string, 0, len(skuIDs))
		args := make([]any, 0, len(skuIDs))
		for _, id := range skuIDs {
			ph = append(ph, "?")
			args = append(args, id)
		}

		rows, err := r.DB.QueryContext(ctx, `
			SELECT
				id,
				COALESCE(NULLIF(sku_code, ''), ''),
				COALESCE(NULLIF(product_name_zh, ''), product_name, ''),
				COALESCE(pack_quantity, 0),
				COALESCE(box_length, 0), COALESCE(box_width, 0), COALESCE(box_height, 0),
				COALESCE(NULLIF(box_unit, ''), 'cm')
			FROM skus
			WHERE id IN (`+strings.Join(ph, ",")+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var info models.PackInfo
			info.ProductType = models.ProductionProductSku
			if err := rows.Scan(
				&info.ProductRefID, &info.Code, &info.Name, &info.PackPerCarton,
				&info.Length, &info.Width, &info.Height, &info.Unit,
			); err != nil {
				rows.Close()
				return nil, err
			}
			result[models.PackKey(info.ProductType, info.ProductRefID)] = info
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	return result, nil
}

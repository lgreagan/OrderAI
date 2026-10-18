package repository

import (
	"context"
	"database/sql"

	"OrderAI/models"
)

// ShippingChannelRepository 读写货代渠道字典。
type ShippingChannelRepository struct {
	DB *sql.DB
}

func NewShippingChannelRepository(db *sql.DB) *ShippingChannelRepository {
	return &ShippingChannelRepository{DB: db}
}

// List 查询启用的渠道（可按国家/关键词过滤，country 为空表示全部）。
func (r *ShippingChannelRepository) List(
	ctx context.Context,
	country string,
) ([]models.ShippingChannel, error) {
	query := `SELECT id, name, country, sort_order, status FROM shipping_channels WHERE status = 'active'`
	args := []any{}
	if country != "" {
		// 未标注国家的通用渠道也一并返回，避免漏选。
		query += ` AND (country = ? OR country = '')`
		args = append(args, country)
	}
	query += ` ORDER BY sort_order ASC, id ASC`

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.ShippingChannel, 0)
	for rows.Next() {
		var c models.ShippingChannel
		if err := rows.Scan(&c.ID, &c.Name, &c.Country, &c.SortOrder, &c.Status); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

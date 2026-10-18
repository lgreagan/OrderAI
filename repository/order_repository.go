package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"OrderAI/models"
)

type OrderRepository struct {
	DB *sql.DB
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{
		DB: db,
	}
}

// ========================================
// 新增订单
// ========================================

func (r *OrderRepository) Create(
	ctx context.Context,
	order *models.Order,
) error {

	tx, err := r.DB.BeginTx(
		ctx,
		nil,
	)

	if err != nil {
		return err
	}

	defer func() {

		if err != nil {
			_ = tx.Rollback()
		}

	}()

	result, err := tx.ExecContext(
		ctx,
		`
		INSERT INTO orders (
			order_number,
			country,
			shipping_method,
			customer,
			status,
			source_image
		)
		VALUES (?, ?, ?, ?, ?, ?)
		`,
		order.OrderNumber,
		order.Country,
		order.ShippingMethod,
		order.Customer,
		order.Status,
		order.SourceImage,
	)

	if err != nil {
		return fmt.Errorf(
			"新增订单失败: %w",
			err,
		)
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取订单ID失败: %w", err)
	}

	order.ID = uint64(lastID)

	if err != nil {
		return err
	}

	if err = insertOrderItemsWithMatches(ctx, tx, order); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf(
			"提交订单事务失败: %w",
			err,
		)
	}

	return nil
}

// insertOrderItemsWithMatches 写入订单明细及其匹配结果。
//
// 订单明细与 order_item_matches 必须在同一个事务中写入，
// 订单行匹配结果依赖订单明细自增出来的 id。
func insertOrderItemsWithMatches(
	ctx context.Context,
	tx *sql.Tx,
	order *models.Order,
) error {
	for i := range order.Items {
		item := &order.Items[i]

		item.OrderID = order.ID

		result, err := tx.ExecContext(
			ctx,
			`
			INSERT INTO order_items (
				order_id,
				line_number,
				product_name,
				description,
				specification,
				quantity,
				unit,
				pack_size,
				actual_quantity,
				declared_quantity
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`,
			item.OrderID,
			item.LineNumber,
			item.ProductName,
			item.Description,
			item.Specification,
			item.Quantity,
			item.Unit,
			item.PackSize,
			item.ActualQuantity,
			item.DeclaredQuantity,
		)

		if err != nil {
			return fmt.Errorf(
				"保存订单明细失败: %w",
				err,
			)
		}

		itemID, err := result.LastInsertId()

		if err != nil {
			return fmt.Errorf(
				"获取订单明细ID失败: %w",
				err,
			)
		}

		item.ID = uint64(itemID)

		if err = insertOrderItemMatches(ctx, tx, item); err != nil {
			return err
		}
	}

	return nil
}

// insertOrderItemMatches 写入单个订单行的匹配结果。
func insertOrderItemMatches(
	ctx context.Context,
	tx *sql.Tx,
	item *models.OrderItem,
) error {
	for j := range item.Matches {
		match := &item.Matches[j]

		match.OrderItemID = item.ID

		_, err := tx.ExecContext(
			ctx,
			`
			INSERT INTO order_item_matches (
				order_item_id,
				sku_id,
				rule_id,
				quantity_per_set,
				ordered_quantity,
				required_quantity,
				sku_code_snapshot,
				product_name_snapshot,
				model_snapshot,
				color_snapshot,
				rule_code_snapshot
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`,
			match.OrderItemID,
			match.SkuID,
			match.RuleID,
			match.QuantityPerSet,
			match.OrderedQuantity,
			match.RequiredQuantity,
			match.SkuCodeSnapshot,
			match.ProductNameSnapshot,
			match.ModelSnapshot,
			match.ColorSnapshot,
			match.RuleCodeSnapshot,
		)

		if err != nil {
			return fmt.Errorf(
				"保存订单行匹配结果失败: %w",
				err,
			)
		}
	}

	return nil
}

func (r *OrderRepository) CreateWithAI(
	ctx context.Context,
	order *models.Order,
	rawJSON string,
	imagePath string,
	provider string,
	model string,
) error {

	tx, err := r.DB.BeginTx(
		ctx,
		nil,
	)

	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// ========================================
	// 1. 保存订单主表
	// ========================================

	result, err := tx.ExecContext(
		ctx,
		`
		INSERT INTO orders (
			order_number,
			country,
			shipping_method,
			customer,
			status,
			source_image
		)
		VALUES (?, ?, ?, ?, ?, ?)
		`,
		order.OrderNumber,
		order.Country,
		order.ShippingMethod,
		order.Customer,
		"confirmed",
		imagePath,
	)

	if err != nil {
		return fmt.Errorf(
			"保存订单失败: %w",
			err,
		)
	}

	orderid, err := result.LastInsertId()
	order.ID = uint64(orderid)

	if err != nil {
		return fmt.Errorf(
			"获取订单ID失败: %w",
			err,
		)
	}

	order.Status = "confirmed"

	// ========================================
	// 2. 保存订单明细与匹配结果
	// ========================================

	if err = insertOrderItemsWithMatches(ctx, tx, order); err != nil {
		return err
	}

	// ========================================
	// 3. 保存 AI 原始结果
	// ========================================

	_, err = tx.ExecContext(
		ctx,
		`
		INSERT INTO ai_results (
			order_id,
			provider,
			model,
			image_path,
			raw_json
		)
		VALUES (?, ?, ?, ?, ?)
		`,
		order.ID,
		provider,
		model,
		imagePath,
		rawJSON,
	)

	if err != nil {
		return fmt.Errorf(
			"保存AI识别记录失败: %w",
			err,
		)
	}

	// ========================================
	// 4. 提交事务
	// ========================================

	if err = tx.Commit(); err != nil {
		return fmt.Errorf(
			"订单事务提交失败: %w",
			err,
		)
	}

	return nil
}

// ========================================
// 查询订单列表
// ========================================

func (r *OrderRepository) List(
	ctx context.Context,
) ([]models.Order, error) {

	rows, err := r.DB.QueryContext(
		ctx,
		`
		SELECT
			id,
			order_number,
			country,
			shipping_method,
			customer,
			status,
			source_image,
			created_at,
			updated_at
		FROM orders
		ORDER BY id DESC
		`,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var orders []models.Order

	for rows.Next() {

		var order models.Order

		err := rows.Scan(
			&order.ID,
			&order.OrderNumber,
			&order.Country,
			&order.ShippingMethod,
			&order.Customer,
			&order.Status,
			&order.SourceImage,
			&order.CreatedAt,
			&order.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		orders = append(
			orders,
			order,
		)
	}

	return orders, rows.Err()
}

// ========================================
// 查询订单详情
// ========================================

func (r *OrderRepository) GetByID(
	ctx context.Context,
	id uint64,
) (*models.Order, error) {

	var order models.Order

	err := r.DB.QueryRowContext(
		ctx,
		`
		SELECT
			id,
			order_number,
			country,
			shipping_method,
			customer,
			status,
			source_image,
			created_at,
			updated_at
		FROM orders
		WHERE id = ?
		`,
		id,
	).Scan(
		&order.ID,
		&order.OrderNumber,
		&order.Country,
		&order.ShippingMethod,
		&order.Customer,
		&order.Status,
		&order.SourceImage,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrOrderNotFound
	}

	if err != nil {
		return nil, err
	}

	rows, err := r.DB.QueryContext(
		ctx,
		`
		SELECT
			id,
			order_id,
			line_number,
			product_name,
			description,
			specification,
			quantity,
			unit,
			pack_size,
			actual_quantity,
			declared_quantity,
			created_at
		FROM order_items
		WHERE order_id = ?
		ORDER BY line_number ASC, id ASC
		`,
		id,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {

		var item models.OrderItem

		err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.LineNumber,
			&item.ProductName,
			&item.Description,
			&item.Specification,
			&item.Quantity,
			&item.Unit,
			&item.PackSize,
			&item.ActualQuantity,
			&item.DeclaredQuantity,
			&item.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		order.Items = append(
			order.Items,
			item,
		)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	// 订单行的匹配结果供编辑订单时回填所选的匹配产品。
	matches, err := r.listOrderItemMatches(ctx, id)
	if err != nil {
		return nil, err
	}

	for i := range order.Items {
		order.Items[i].Matches = matches[order.Items[i].ID]
	}

	return &order, nil
}

// listOrderItemMatches 查询某个订单下所有订单行的匹配结果，
// 按订单明细 ID 分组。
func (r *OrderRepository) listOrderItemMatches(
	ctx context.Context,
	orderID uint64,
) (map[uint64][]models.OrderItemMatch, error) {
	rows, err := r.DB.QueryContext(
		ctx,
		`
		SELECT
			m.id,
			m.order_item_id,
			m.sku_id,
			m.rule_id,
			m.quantity_per_set,
			m.ordered_quantity,
			m.required_quantity,
			m.sku_code_snapshot,
			m.product_name_snapshot,
			m.model_snapshot,
			m.color_snapshot,
			m.rule_code_snapshot,
			m.created_at
		FROM order_item_matches m
		JOIN order_items i ON i.id = m.order_item_id
		WHERE i.order_id = ?
		ORDER BY m.id ASC
		`,
		orderID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	result := make(map[uint64][]models.OrderItemMatch)

	for rows.Next() {

		var match models.OrderItemMatch

		err := rows.Scan(
			&match.ID,
			&match.OrderItemID,
			&match.SkuID,
			&match.RuleID,
			&match.QuantityPerSet,
			&match.OrderedQuantity,
			&match.RequiredQuantity,
			&match.SkuCodeSnapshot,
			&match.ProductNameSnapshot,
			&match.ModelSnapshot,
			&match.ColorSnapshot,
			&match.RuleCodeSnapshot,
			&match.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		result[match.OrderItemID] = append(
			result[match.OrderItemID],
			match,
		)
	}

	return result, rows.Err()
}

// ========================================
// 修改订单
// ========================================

func (r *OrderRepository) Update(
	ctx context.Context,
	order *models.Order,
) error {

	tx, err := r.DB.BeginTx(
		ctx,
		nil,
	)

	if err != nil {
		return err
	}

	defer func() {

		if err != nil {
			_ = tx.Rollback()
		}

	}()

	// 订单必须存在：UPDATE 的 RowsAffected 只统计被修改的列，
	// 订单表头字段没变化时会是 0，所以这里单独判断存在性。
	var exists uint64

	err = tx.QueryRowContext(
		ctx,
		`SELECT id FROM orders WHERE id = ?`,
		order.ID,
	).Scan(&exists)

	if err == sql.ErrNoRows {
		err = models.ErrOrderNotFound
		return err
	}

	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`
		UPDATE orders
		SET
			order_number = ?,
			country = ?,
			shipping_method = ?,
			customer = ?,
			status = ?
		WHERE id = ?
		`,
		order.OrderNumber,
		order.Country,
		order.ShippingMethod,
		order.Customer,
		order.Status,
		order.ID,
	)

	if err != nil {
		return fmt.Errorf(
			"修改订单失败: %w",
			err,
		)
	}

	// 重新保存明细：order_item_matches 由外键级联删除
	_, err = tx.ExecContext(
		ctx,
		`
		DELETE FROM order_items
		WHERE order_id = ?
		`,
		order.ID,
	)

	if err != nil {
		return fmt.Errorf(
			"删除旧订单明细失败: %w",
			err,
		)
	}

	if err = insertOrderItemsWithMatches(ctx, tx, order); err != nil {
		return err
	}

	err = tx.Commit()

	if err != nil {
		return fmt.Errorf(
			"提交修改失败: %w",
			err,
		)
	}

	return nil
}

// ========================================
// 删除订单
// ========================================

func (r *OrderRepository) Delete(
	ctx context.Context,
	id uint64,
) error {

	result, err := r.DB.ExecContext(
		ctx,
		`
		DELETE FROM orders
		WHERE id = ?
		`,
		id,
	)

	if err != nil {
		return fmt.Errorf(
			"删除订单失败: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()

	if err != nil {
		return err
	}

	if affected == 0 {
		return models.ErrOrderNotFound
	}

	return nil
}

// ========================================
// PI 产品名来源
// ========================================

// OrderItemProductName 取某订单下某 SKU 对应的 order_items 产品名与规格。
//
// PI 产品名按 order_items.product_name + specification 生成，因此这里通过
// order_item_matches 把 (order_id, sku_id) 反查到订单行。找不到返回空串。
func (r *OrderRepository) OrderItemProductName(
	ctx context.Context,
	orderID, skuID uint64,
) (productName string, specification string, err error) {

	row := r.DB.QueryRowContext(
		ctx,
		`
		SELECT oi.product_name, oi.specification
		FROM order_items oi
		JOIN order_item_matches m ON m.order_item_id = oi.id
		WHERE oi.order_id = ? AND m.sku_id = ?
		ORDER BY oi.line_number ASC
		LIMIT 1
		`,
		orderID,
		skuID,
	)

	err = row.Scan(&productName, &specification)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	return productName, specification, nil
}

// ========================================
// 客户原始订单截图
// ========================================

// SourceImagesByIDs 批量取订单的客户原始订单截图路径（order_id -> 路径）。
//
// 优先取 ai_results.image_path（上传给 AI 识别的那张图，多张时取最新一条），
// 为空时回退 orders.source_image；两者都没有的订单不出现在结果里。
func (r *OrderRepository) SourceImagesByIDs(
	ctx context.Context,
	ids []uint64,
) (map[uint64]string, error) {

	images := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return images, nil
	}

	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	rows, err := r.DB.QueryContext(
		ctx,
		`
		SELECT
			o.id,
			COALESCE(
				(
					SELECT ar.image_path
					FROM ai_results ar
					WHERE ar.order_id = o.id AND ar.image_path <> ''
					ORDER BY ar.id DESC
					LIMIT 1
				),
				o.source_image,
				''
			) AS image_path
		FROM orders o
		WHERE o.id IN (`+strings.Join(placeholders, ",")+`)
		`,
		args...,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var id uint64
		var path string

		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}

		if path = strings.TrimSpace(path); path != "" {
			images[id] = path
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return images, nil
}

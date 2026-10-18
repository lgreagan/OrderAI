package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

type OrderController struct {
	Service *services.OrderService
}

func NewOrderController(
	service *services.OrderService,
) *OrderController {

	return &OrderController{
		Service: service,
	}
}

// orderWriteError 把下单 / 改单错误映射为合适的响应状态码。
func orderWriteError(
	ctx echo.Context,
	err error,
) error {
	status := http.StatusInternalServerError

	var invalid *services.OrderValidationError

	switch {
	case errors.As(err, &invalid):
		status = http.StatusBadRequest
	case errors.Is(err, models.ErrOrderNotFound):
		status = http.StatusNotFound
	}

	return ctx.JSON(
		status,
		map[string]interface{}{
			"success": false,
			"message": err.Error(),
		},
	)
}

func (c *OrderController) Confirm(
	ctx echo.Context,
) error {

	var req models.ConfirmOrderRequest

	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(
			http.StatusBadRequest,
			map[string]interface{}{
				"success": false,
				"message": "请求数据格式错误",
			},
		)
	}

	if req.Order.OrderNumber == "" {
		return ctx.JSON(
			http.StatusBadRequest,
			map[string]interface{}{
				"success": false,
				"message": "订单号不能为空",
			},
		)
	}

	if req.Order.Customer == "" {
		req.Order.Customer = "Polat"
	}

	if len(req.Order.Items) == 0 {
		return ctx.JSON(
			http.StatusBadRequest,
			map[string]interface{}{
				"success": false,
				"message": "订单至少需要一个产品",
			},
		)
	}

	if req.Provider == "" {
		req.Provider = "deepseek"
	}

	if req.Model == "" {
		req.Model = "deepseek-flash"
	}

	err := c.Service.CreateWithAI(
		ctx.Request().Context(),
		&req.Order,
		req.RawJSON,
		req.ImagePath,
		req.Provider,
		req.Model,
	)

	if err != nil {
		return ctx.JSON(
			http.StatusInternalServerError,
			map[string]interface{}{
				"success": false,
				"message": err.Error(),
			},
		)
	}

	unmatchedLines := 0

	for i := range req.Order.Items {
		if len(req.Order.Items[i].Matches) == 0 {
			unmatchedLines++
		}
	}

	return ctx.JSON(
		http.StatusOK,
		map[string]interface{}{
			"success": true,
			"message": "订单确认并入库成功",
			"data": map[string]interface{}{
				"id":                   req.Order.ID,
				"unmatched_line_count": unmatchedLines,
			},
		},
	)
}

// ========================================
// GET /api/orders
// ========================================

func (c *OrderController) List(
	ctx echo.Context,
) error {

	orders, err := c.Service.List(
		ctx.Request().Context(),
	)

	if err != nil {
		return ctx.JSON(
			http.StatusInternalServerError,
			map[string]interface{}{
				"success": false,
				"message": err.Error(),
			},
		)
	}

	return ctx.JSON(
		http.StatusOK,
		map[string]interface{}{
			"success": true,
			"data":    orders,
		},
	)
}

// ========================================
// GET /api/orders/:id
// ========================================

func (c *OrderController) Get(
	ctx echo.Context,
) error {

	id, err := strconv.ParseUint(
		ctx.Param("id"),
		10,
		64,
	)

	if err != nil {
		return ctx.JSON(
			http.StatusBadRequest,
			map[string]interface{}{
				"success": false,
				"message": "订单ID无效",
			},
		)
	}

	order, err := c.Service.Get(
		ctx.Request().Context(),
		id,
	)

	if err != nil {
		return orderWriteError(ctx, err)
	}

	return ctx.JSON(
		http.StatusOK,
		map[string]interface{}{
			"success": true,
			"data":    order,
		},
	)
}

// ========================================
// POST /api/orders
// ========================================

func (c *OrderController) Create(
	ctx echo.Context,
) error {

	var order models.Order

	if err := ctx.Bind(&order); err != nil {
		return ctx.JSON(
			http.StatusBadRequest,
			map[string]interface{}{
				"success": false,
				"message": "请求数据格式错误",
			},
		)
	}

	err := c.Service.Create(
		ctx.Request().Context(),
		&order,
	)

	if err != nil {
		return orderWriteError(ctx, err)
	}

	return ctx.JSON(
		http.StatusOK,
		map[string]interface{}{
			"success": true,
			"data":    order,
		},
	)
}

// ========================================
// PUT /api/orders/:id
// ========================================

func (c *OrderController) Update(
	ctx echo.Context,
) error {

	id, err := strconv.ParseUint(
		ctx.Param("id"),
		10,
		64,
	)

	if err != nil {
		return ctx.JSON(
			http.StatusBadRequest,
			map[string]interface{}{
				"success": false,
				"message": "订单ID无效",
			},
		)
	}

	var order models.Order

	if err := ctx.Bind(&order); err != nil {
		return ctx.JSON(
			http.StatusBadRequest,
			map[string]interface{}{
				"success": false,
				"message": "请求数据格式错误",
			},
		)
	}

	order.ID = id

	err = c.Service.Update(
		ctx.Request().Context(),
		&order,
	)

	if err != nil {
		return orderWriteError(ctx, err)
	}

	return ctx.JSON(
		http.StatusOK,
		map[string]interface{}{
			"success": true,
			"data":    order,
		},
	)
}

// ========================================
// DELETE /api/orders/:id
// ========================================

func (c *OrderController) Delete(
	ctx echo.Context,
) error {

	id, err := strconv.ParseUint(
		ctx.Param("id"),
		10,
		64,
	)

	if err != nil {
		return ctx.JSON(
			http.StatusBadRequest,
			map[string]interface{}{
				"success": false,
				"message": "订单ID无效",
			},
		)
	}

	err = c.Service.Delete(
		ctx.Request().Context(),
		id,
	)

	if err != nil {
		return orderWriteError(ctx, err)
	}

	return ctx.JSON(
		http.StatusOK,
		map[string]interface{}{
			"success": true,
			"message": "订单删除成功",
		},
	)
}

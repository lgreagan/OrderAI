package controllers

import (
	"net/http"
	"strconv"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// PaymentController 提供收款登记接口。
type PaymentController struct {
	Service *services.PaymentService
}

func NewPaymentController(service *services.PaymentService) *PaymentController {
	return &PaymentController{Service: service}
}

func paymentError(c echo.Context, err error, code int) error {
	return c.JSON(code, map[string]any{
		"success": false,
		"message": err.Error(),
	})
}

func (ctl *PaymentController) List(c echo.Context) error {
	items, err := ctl.Service.List(
		c.Request().Context(),
		c.QueryParam("customer_name"),
		c.QueryParam("keyword"),
	)
	if err != nil {
		return paymentError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

// Customers 返回客户名列表，供筛选与选择。
func (ctl *PaymentController) Customers(c echo.Context) error {
	items, err := ctl.Service.Customers(c.Request().Context())
	if err != nil {
		return paymentError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

func (ctl *PaymentController) Create(c echo.Context) error {
	var p models.Payment
	if err := c.Bind(&p); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}
	p.ID = 0

	if err := ctl.Service.Create(c.Request().Context(), &p); err != nil {
		return paymentError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"success": true, "message": "收款登记成功", "data": p,
	})
}

func (ctl *PaymentController) Update(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "收款记录 ID 无效",
		})
	}

	var p models.Payment
	if err := c.Bind(&p); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}
	p.ID = id

	if err := ctl.Service.Update(c.Request().Context(), &p); err != nil {
		return paymentError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "收款记录更新成功",
	})
}

func (ctl *PaymentController) Delete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "收款记录 ID 无效",
		})
	}

	if err := ctl.Service.Delete(c.Request().Context(), id); err != nil {
		return paymentError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "收款记录已删除",
	})
}

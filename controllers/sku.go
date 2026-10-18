package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

type SKUController struct {
	Service *services.SKUService
}

func NewSKUController(service *services.SKUService) *SKUController {
	return &SKUController{Service: service}
}

func skuError(c echo.Context, err error, code int) error {
	return c.JSON(code, map[string]any{
		"success": false,
		"message": err.Error(),
	})
}

func (ctl *SKUController) List(c echo.Context) error {
	items, err := ctl.Service.List(
		c.Request().Context(),
		strings.TrimSpace(c.QueryParam("keyword")),
		strings.TrimSpace(c.QueryParam("status")),
	)
	if err != nil {
		return skuError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

func (ctl *SKUController) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "SKU ID无效",
		})
	}

	item, err := ctl.Service.Get(c.Request().Context(), id)
	if err != nil {
		return skuError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    item,
	})
}

func (ctl *SKUController) Create(c echo.Context) error {
	var sku models.SKU
	if err := c.Bind(&sku); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	if err := ctl.Service.Create(c.Request().Context(), &sku); err != nil {
		code := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			code = http.StatusConflict
		}
		return skuError(c, err, code)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"success": true,
		"message": "SKU创建成功",
		"data":    sku,
	})
}

func (ctl *SKUController) Update(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "SKU ID无效",
		})
	}

	var sku models.SKU
	if err := c.Bind(&sku); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}
	sku.ID = id

	if err := ctl.Service.Update(c.Request().Context(), &sku); err != nil {
		code := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			code = http.StatusConflict
		}
		return skuError(c, err, code)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": "SKU更新成功",
	})
}

func (ctl *SKUController) Delete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "SKU ID无效",
		})
	}

	if err := ctl.Service.Delete(c.Request().Context(), id); err != nil {
		return skuError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": "SKU删除成功",
	})
}

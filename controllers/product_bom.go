package controllers

import (
	"net/http"
	"strconv"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// ProductBomController 提供产品用料清单（BOM）接口。
type ProductBomController struct {
	Service *services.ProductBomService
}

func NewProductBomController(service *services.ProductBomService) *ProductBomController {
	return &ProductBomController{Service: service}
}

func bomError(c echo.Context, err error, code int) error {
	return c.JSON(code, map[string]any{
		"success": false,
		"message": err.Error(),
	})
}

func (ctl *ProductBomController) List(c echo.Context) error {
	items, err := ctl.Service.List(
		c.Request().Context(),
		c.QueryParam("product_type"),
		c.QueryParam("keyword"),
	)
	if err != nil {
		return bomError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

func (ctl *ProductBomController) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "用料记录 ID 无效",
		})
	}

	item, err := ctl.Service.Get(c.Request().Context(), id)
	if err != nil {
		return bomError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    item,
	})
}

func (ctl *ProductBomController) Create(c echo.Context) error {
	var b models.ProductBom
	if err := c.Bind(&b); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}
	b.ID = 0

	if err := ctl.Service.Create(c.Request().Context(), &b); err != nil {
		return bomError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"success": true, "message": "用料记录创建成功", "data": b,
	})
}

func (ctl *ProductBomController) Update(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "用料记录 ID 无效",
		})
	}

	var b models.ProductBom
	if err := c.Bind(&b); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}
	b.ID = id

	if err := ctl.Service.Update(c.Request().Context(), &b); err != nil {
		return bomError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "用料记录更新成功",
	})
}

func (ctl *ProductBomController) Delete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "用料记录 ID 无效",
		})
	}

	if err := ctl.Service.Delete(c.Request().Context(), id); err != nil {
		return bomError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "用料记录已删除",
	})
}

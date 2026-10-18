package controllers

import (
	"net/http"
	"strconv"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// InventoryController 提供库存结存、流水与盘点调整接口。
type InventoryController struct {
	Service *services.InventoryService
}

func NewInventoryController(service *services.InventoryService) *InventoryController {
	return &InventoryController{Service: service}
}

func (ctl *InventoryController) Stock(c echo.Context) error {
	items, err := ctl.Service.ListStock(
		c.Request().Context(),
		c.QueryParam("item_type"),
		c.QueryParam("keyword"),
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false, "message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

func (ctl *InventoryController) Transactions(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	items, err := ctl.Service.ListTransactions(
		c.Request().Context(),
		c.QueryParam("item_type"),
		c.QueryParam("keyword"),
		c.QueryParam("biz_type"),
		limit,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false, "message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

// In 手工入库。
//
// POST /api/inventory/in
func (ctl *InventoryController) In(c echo.Context) error {
	var req models.InventoryInRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	if err := ctl.Service.ManualIn(c.Request().Context(), &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "入库成功",
	})
}

// Issue 领料出库。
//
// POST /api/inventory/issue
func (ctl *InventoryController) Issue(c echo.Context) error {
	var req models.MaterialIssueRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	if err := ctl.Service.IssueMaterials(c.Request().Context(), &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "领料出库成功",
	})
}

func (ctl *InventoryController) Adjust(c echo.Context) error {
	var req models.InventoryAdjustRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	if err := ctl.Service.Adjust(c.Request().Context(), &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "库存已调整",
	})
}

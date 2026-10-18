package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// ProductionOrderController 提供持久化生产订单接口。
type ProductionOrderController struct {
	Service *services.ProductionOrderService
}

func NewProductionOrderController(service *services.ProductionOrderService) *ProductionOrderController {
	return &ProductionOrderController{Service: service}
}

func productionError(c echo.Context, err error, code int) error {
	return c.JSON(code, map[string]any{
		"success": false,
		"message": err.Error(),
	})
}

func (ctl *ProductionOrderController) List(c echo.Context) error {
	items, err := ctl.Service.List(
		c.Request().Context(),
		c.QueryParam("keyword"),
		c.QueryParam("status"),
	)
	if err != nil {
		return productionError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

// MaterialSummary 生产报表数据：汇总生产单所需材料。
//
// GET /api/production-orders/report/materials
func (ctl *ProductionOrderController) MaterialSummary(c echo.Context) error {
	items, err := ctl.Service.MaterialSummary(
		c.Request().Context(),
		c.QueryParam("keyword"),
		c.QueryParam("status"),
	)
	if err != nil {
		return productionError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

func (ctl *ProductionOrderController) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "生产单 ID 无效",
		})
	}

	po, err := ctl.Service.Get(c.Request().Context(), id)
	if err != nil {
		return productionError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    po,
	})
}

// Materials 某生产单的材料需求（BOM 理论用量），用于领料页面预填。
//
// GET /api/production-orders/:id/materials
func (ctl *ProductionOrderController) Materials(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "生产单 ID 无效",
		})
	}

	items, err := ctl.Service.MaterialNeeds(c.Request().Context(), id)
	if err != nil {
		return productionError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

func (ctl *ProductionOrderController) Create(c echo.Context) error {
	var po models.ProductionOrder
	if err := c.Bind(&po); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}
	po.ID = 0

	if err := ctl.Service.Save(c.Request().Context(), &po); err != nil {
		return productionError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"success": true, "message": "生产单创建成功", "data": po,
	})
}

func (ctl *ProductionOrderController) Update(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "生产单 ID 无效",
		})
	}

	var po models.ProductionOrder
	if err := c.Bind(&po); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}
	po.ID = id

	if err := ctl.Service.Save(c.Request().Context(), &po); err != nil {
		return productionError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "生产单更新成功", "data": po,
	})
}

func (ctl *ProductionOrderController) Delete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "生产单 ID 无效",
		})
	}

	if err := ctl.Service.Delete(c.Request().Context(), id); err != nil {
		return productionError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "生产单已删除",
	})
}

// Complete 完工登记：成品入库（材料消耗改由领料出库页面处理）。
//
// POST /api/production-orders/:id/complete
func (ctl *ProductionOrderController) Complete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "生产单 ID 无效",
		})
	}

	var req models.ProductionCompleteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	if err := ctl.Service.Complete(c.Request().Context(), id, &req); err != nil {
		return productionError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "完工登记成功",
	})
}

// Suggestion 返回按订单生成生产单的建议。
//
// GET /api/production-orders/order/:orderId/suggestion
func (ctl *ProductionOrderController) Suggestion(c echo.Context) error {
	orderID, err := strconv.ParseUint(c.Param("orderId"), 10, 64)
	if err != nil || orderID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "订单 ID 无效",
		})
	}

	result, err := ctl.Service.Suggestion(c.Request().Context(), orderID)
	if err != nil {
		return productionError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    result,
	})
}

// PushDown 由客户订单下推生成生产单草稿（保存原始需求快照 + 纸箱用料）。
//
// POST /api/production-orders/order/:orderId/push
func (ctl *ProductionOrderController) PushDown(c echo.Context) error {
	orderID, err := strconv.ParseUint(c.Param("orderId"), 10, 64)
	if err != nil || orderID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "订单 ID 无效",
		})
	}

	po, cartons, created, err := ctl.Service.PushDown(c.Request().Context(), orderID)
	if err != nil {
		return productionError(c, err, http.StatusBadRequest)
	}

	message := fmt.Sprintf("生产单已下推并保存，新增/复用纸箱用料 %d 项", cartons)
	if !created {
		message = "该订单已有下推草稿，已为你打开"
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": message,
		"created": created,
		"cartons": cartons,
		"data":    po,
	})
}

// MarkProduced 把客户订单标记为已生产。
//
// POST /api/production-orders/order/:orderId/produced
func (ctl *ProductionOrderController) MarkProduced(c echo.Context) error {
	orderID, err := strconv.ParseUint(c.Param("orderId"), 10, 64)
	if err != nil || orderID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "订单 ID 无效",
		})
	}

	if err := ctl.Service.MarkOrderProduced(c.Request().Context(), orderID); err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, models.ErrOrderNotFound) {
			code = http.StatusNotFound
		}
		return productionError(c, err, code)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "订单已标记为已生产",
	})
}

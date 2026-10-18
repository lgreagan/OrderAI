package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// PurchaseOrderController 提供持久化采购订单接口。
type PurchaseOrderController struct {
	Service *services.PurchaseOrderService
}

func NewPurchaseOrderController(service *services.PurchaseOrderService) *PurchaseOrderController {
	return &PurchaseOrderController{Service: service}
}

func purchaseError(c echo.Context, err error, code int) error {
	return c.JSON(code, map[string]any{
		"success": false,
		"message": err.Error(),
	})
}

func (ctl *PurchaseOrderController) List(c echo.Context) error {
	items, err := ctl.Service.List(
		c.Request().Context(),
		c.QueryParam("keyword"),
		c.QueryParam("status"),
	)
	if err != nil {
		return purchaseError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

// ItemSummary 采购报表数据：按物品汇总采购量、到货量与金额。
//
// GET /api/purchase-orders/report/items
func (ctl *PurchaseOrderController) ItemSummary(c echo.Context) error {
	items, err := ctl.Service.ItemSummary(
		c.Request().Context(),
		c.QueryParam("keyword"),
		c.QueryParam("status"),
	)
	if err != nil {
		return purchaseError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

func (ctl *PurchaseOrderController) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "采购单 ID 无效",
		})
	}

	po, err := ctl.Service.Get(c.Request().Context(), id)
	if err != nil {
		return purchaseError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    po,
	})
}

func (ctl *PurchaseOrderController) Create(c echo.Context) error {
	var po models.PurchaseOrder
	if err := c.Bind(&po); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}
	po.ID = 0

	if err := ctl.Service.Save(c.Request().Context(), &po); err != nil {
		return purchaseError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"success": true, "message": "采购单创建成功", "data": po,
	})
}

func (ctl *PurchaseOrderController) Update(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "采购单 ID 无效",
		})
	}

	var po models.PurchaseOrder
	if err := c.Bind(&po); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}
	po.ID = id

	if err := ctl.Service.Save(c.Request().Context(), &po); err != nil {
		return purchaseError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "采购单更新成功", "data": po,
	})
}

func (ctl *PurchaseOrderController) Delete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "采购单 ID 无效",
		})
	}

	if err := ctl.Service.Delete(c.Request().Context(), id); err != nil {
		return purchaseError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "采购单已删除",
	})
}

// Receive 到货登记：写入入库流水并刷新状态。
//
// POST /api/purchase-orders/:id/receive
func (ctl *PurchaseOrderController) Receive(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "采购单 ID 无效",
		})
	}

	var req models.PurchaseOrderReceiveRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	if err := ctl.Service.Receive(c.Request().Context(), id, &req); err != nil {
		return purchaseError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "到货登记成功",
	})
}

// MaterialNeeds 「采购材料」：按产品数量带出纸箱等材料用量。
//
// POST /api/purchase-orders/material-needs
func (ctl *PurchaseOrderController) MaterialNeeds(c echo.Context) error {
	var req models.PurchaseMaterialRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	items, err := ctl.Service.MaterialNeeds(c.Request().Context(), &req)
	if err != nil {
		return purchaseError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

// Suggestion 返回按订单生成采购单的建议。
//
// GET /api/purchase-orders/order/:orderId/suggestion
func (ctl *PurchaseOrderController) Suggestion(c echo.Context) error {
	orderID, err := strconv.ParseUint(c.Param("orderId"), 10, 64)
	if err != nil || orderID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "订单 ID 无效",
		})
	}

	result, err := ctl.Service.Suggestion(c.Request().Context(), orderID)
	if err != nil {
		return purchaseError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    result,
	})
}

// PushDown 由客户订单下推生成采购单草稿（保存原始需求快照）。
//
// POST /api/purchase-orders/order/:orderId/push
func (ctl *PurchaseOrderController) PushDown(c echo.Context) error {
	orderID, err := strconv.ParseUint(c.Param("orderId"), 10, 64)
	if err != nil || orderID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "订单 ID 无效",
		})
	}

	po, created, err := ctl.Service.PushDown(c.Request().Context(), orderID)
	if err != nil {
		return purchaseError(c, err, http.StatusBadRequest)
	}

	message := "采购单已下推并保存"
	if !created {
		message = "该订单已有下推草稿，已为你打开"
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": message,
		"created": created,
		"data":    po,
	})
}

// MarkPurchased 把客户订单标记为已采购。
//
// POST /api/purchase-orders/order/:orderId/purchased
func (ctl *PurchaseOrderController) MarkPurchased(c echo.Context) error {
	orderID, err := strconv.ParseUint(c.Param("orderId"), 10, 64)
	if err != nil || orderID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "订单 ID 无效",
		})
	}

	if err := ctl.Service.MarkOrderPurchased(c.Request().Context(), orderID); err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, models.ErrOrderNotFound) {
			code = http.StatusNotFound
		}
		return purchaseError(c, err, code)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "订单已标记为已采购",
	})
}

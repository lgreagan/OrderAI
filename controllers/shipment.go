package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// ShipmentController 提供发货单接口。
type ShipmentController struct {
	Service *services.ShipmentService
}

func NewShipmentController(service *services.ShipmentService) *ShipmentController {
	return &ShipmentController{Service: service}
}

func shipmentError(c echo.Context, err error, code int) error {
	return c.JSON(code, map[string]any{
		"success": false,
		"message": err.Error(),
	})
}

// PackagingNeeds 核算本次发货将随货消耗的包装材料（只读预览，不写库存）。
//
// POST /api/shipments/packaging-needs
func (ctl *ShipmentController) PackagingNeeds(c echo.Context) error {
	var req models.ShipmentPackagingRequest
	if err := c.Bind(&req); err != nil {
		return shipmentError(c, err, http.StatusBadRequest)
	}

	items, err := ctl.Service.PackagingNeeds(c.Request().Context(), &req)
	if err != nil {
		return shipmentError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

func (ctl *ShipmentController) List(c echo.Context) error {
	items, err := ctl.Service.List(
		c.Request().Context(),
		c.QueryParam("keyword"),
		c.QueryParam("status"),
	)
	if err != nil {
		return shipmentError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

func (ctl *ShipmentController) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "发货单 ID 无效",
		})
	}

	item, err := ctl.Service.Get(c.Request().Context(), id)
	if err != nil {
		return shipmentError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    item,
	})
}

// OrderLines 返回某订单的可发货行，用于新建发货单。
//
// GET /api/shipments/order/:orderId/lines
func (ctl *ShipmentController) OrderLines(c echo.Context) error {
	orderID, err := strconv.ParseUint(c.Param("orderId"), 10, 64)
	if err != nil || orderID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "订单 ID 无效",
		})
	}

	ctx, err := ctl.Service.OrderContext(c.Request().Context(), orderID)
	if err != nil {
		return shipmentError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    ctx,
	})
}

// PushDown 由客户订单下推发货单草稿：默认把该订单剩余未发数量一次性全部发完（一票发完）。
//
// POST /api/shipments/order/:orderId/push
func (ctl *ShipmentController) PushDown(c echo.Context) error {
	orderID, err := strconv.ParseUint(c.Param("orderId"), 10, 64)
	if err != nil || orderID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "订单 ID 无效",
		})
	}

	ship, created, err := ctl.Service.PushDown(c.Request().Context(), orderID)
	if err != nil {
		return shipmentError(c, err, http.StatusBadRequest)
	}

	message := fmt.Sprintf(
		"发货单 %s 已下推：%d 行，默认全部一票发完", ship.ShipmentNumber, len(ship.Items),
	)
	if !created {
		message = fmt.Sprintf("该订单已有草稿发货单 %s，已为你打开（避免重复下推）", ship.ShipmentNumber)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": message,
		"created": created,
		"data":    ship,
	})
}

func (ctl *ShipmentController) Create(c echo.Context) error {
	var ship models.Shipment
	if err := c.Bind(&ship); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}
	ship.ID = 0

	if err := ctl.Service.Save(c.Request().Context(), &ship); err != nil {
		return shipmentError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"success": true, "message": "发货单创建成功", "data": ship,
	})
}

func (ctl *ShipmentController) Update(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "发货单 ID 无效",
		})
	}

	var ship models.Shipment
	if err := c.Bind(&ship); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}
	ship.ID = id

	if err := ctl.Service.Save(c.Request().Context(), &ship); err != nil {
		return shipmentError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "发货单更新成功", "data": ship,
	})
}

func (ctl *ShipmentController) Delete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "发货单 ID 无效",
		})
	}

	if err := ctl.Service.Delete(c.Request().Context(), id); err != nil {
		return shipmentError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "发货单已删除",
	})
}

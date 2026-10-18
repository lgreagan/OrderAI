package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

var (
	errBadWarehouseID      = errors.New("仓库地址 ID 无效")
	errBadWarehousePayload = errors.New("请求数据格式错误")
)

// WarehouseAddressController 仓库地址库（FBA）维护。
type WarehouseAddressController struct {
	Service *services.WarehouseAddressService
}

func NewWarehouseAddressController(service *services.WarehouseAddressService) *WarehouseAddressController {
	return &WarehouseAddressController{Service: service}
}

func warehouseError(c echo.Context, err error, code int) error {
	return c.JSON(code, map[string]any{"success": false, "message": err.Error()})
}

func warehouseStatusCode(err error) int {
	if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		return http.StatusConflict
	}
	return http.StatusBadRequest
}

func (ctl *WarehouseAddressController) List(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	items, err := ctl.Service.List(
		c.Request().Context(),
		strings.TrimSpace(c.QueryParam("keyword")),
		strings.ToUpper(strings.TrimSpace(c.QueryParam("country"))),
		limit,
	)
	if err != nil {
		return warehouseError(c, err, http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "data": items})
}

func (ctl *WarehouseAddressController) Countries(c echo.Context) error {
	items, err := ctl.Service.Countries(c.Request().Context())
	if err != nil {
		return warehouseError(c, err, http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "data": items})
}

func (ctl *WarehouseAddressController) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return warehouseError(c, errBadWarehouseID, http.StatusBadRequest)
	}
	item, err := ctl.Service.Get(c.Request().Context(), id)
	if err != nil {
		return warehouseError(c, err, http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "data": item})
}

func (ctl *WarehouseAddressController) Create(c echo.Context) error {
	var item models.WarehouseAddress
	if err := c.Bind(&item); err != nil {
		return warehouseError(c, errBadWarehousePayload, http.StatusBadRequest)
	}
	if err := ctl.Service.Create(c.Request().Context(), &item); err != nil {
		return warehouseError(c, err, warehouseStatusCode(err))
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"success": true,
		"message": "仓库地址创建成功",
		"data":    item,
	})
}

func (ctl *WarehouseAddressController) Update(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return warehouseError(c, errBadWarehouseID, http.StatusBadRequest)
	}
	var item models.WarehouseAddress
	if err := c.Bind(&item); err != nil {
		return warehouseError(c, errBadWarehousePayload, http.StatusBadRequest)
	}
	item.ID = id
	if err := ctl.Service.Update(c.Request().Context(), &item); err != nil {
		return warehouseError(c, err, warehouseStatusCode(err))
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "message": "仓库地址更新成功"})
}

func (ctl *WarehouseAddressController) Delete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return warehouseError(c, errBadWarehouseID, http.StatusBadRequest)
	}
	if err := ctl.Service.Delete(c.Request().Context(), id); err != nil {
		return warehouseError(c, err, http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "message": "仓库地址删除成功"})
}

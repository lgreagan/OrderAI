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

// MaterialController 提供材料（包材/原材料）维护接口。
type MaterialController struct {
	Service *services.MaterialService
}

func NewMaterialController(service *services.MaterialService) *MaterialController {
	return &MaterialController{Service: service}
}

func materialError(c echo.Context, err error, code int) error {
	return c.JSON(code, map[string]any{
		"success": false,
		"message": err.Error(),
	})
}

func (ctl *MaterialController) List(c echo.Context) error {
	items, err := ctl.Service.List(
		c.Request().Context(),
		c.QueryParam("keyword"),
		c.QueryParam("category"),
	)
	if err != nil {
		return materialError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

func (ctl *MaterialController) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "材料 ID 无效",
		})
	}

	item, err := ctl.Service.Get(c.Request().Context(), id)
	if err != nil {
		return materialError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    item,
	})
}

func (ctl *MaterialController) Create(c echo.Context) error {
	var m models.Material
	if err := c.Bind(&m); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	if err := ctl.Service.Create(c.Request().Context(), &m); err != nil {
		code := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			code = http.StatusConflict
		}
		return materialError(c, err, code)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"success": true, "message": "材料创建成功", "data": m,
	})
}

func (ctl *MaterialController) Update(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "材料 ID 无效",
		})
	}

	var m models.Material
	if err := c.Bind(&m); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}
	m.ID = id

	if err := ctl.Service.Update(c.Request().Context(), &m); err != nil {
		code := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			code = http.StatusConflict
		}
		return materialError(c, err, code)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "材料更新成功",
	})
}

func (ctl *MaterialController) Delete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "材料 ID 无效",
		})
	}

	if err := ctl.Service.Delete(c.Request().Context(), id); err != nil {
		code := http.StatusNotFound
		if errors.Is(err, models.ErrMaterialNotFound) {
			code = http.StatusNotFound
		}
		return materialError(c, err, code)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "材料删除成功",
	})
}

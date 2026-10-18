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
	errBadCategoryID      = errors.New("报关品类 ID 无效")
	errBadCategoryPayload = errors.New("请求数据格式错误")
)

// ProductCategoryController 报关品类维护。
type ProductCategoryController struct {
	Service *services.ProductCategoryService
}

func NewProductCategoryController(service *services.ProductCategoryService) *ProductCategoryController {
	return &ProductCategoryController{Service: service}
}

func categoryError(c echo.Context, err error, code int) error {
	return c.JSON(code, map[string]any{
		"success": false,
		"message": err.Error(),
	})
}

// categoryStatusCode 唯一键冲突返回 409，其余按 400。
func categoryStatusCode(err error) int {
	if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		return http.StatusConflict
	}
	return http.StatusBadRequest
}

func (ctl *ProductCategoryController) List(c echo.Context) error {
	items, err := ctl.Service.List(
		c.Request().Context(),
		strings.TrimSpace(c.QueryParam("keyword")),
		strings.TrimSpace(c.QueryParam("status")),
	)
	if err != nil {
		return categoryError(c, err, http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "data": items})
}

func (ctl *ProductCategoryController) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return categoryError(c, errBadCategoryID, http.StatusBadRequest)
	}
	item, err := ctl.Service.Get(c.Request().Context(), id)
	if err != nil {
		return categoryError(c, err, http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "data": item})
}

func (ctl *ProductCategoryController) Create(c echo.Context) error {
	var item models.ProductCategory
	if err := c.Bind(&item); err != nil {
		return categoryError(c, errBadCategoryPayload, http.StatusBadRequest)
	}
	if err := ctl.Service.Create(c.Request().Context(), &item); err != nil {
		return categoryError(c, err, categoryStatusCode(err))
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"success": true,
		"message": "报关品类创建成功",
		"data":    item,
	})
}

func (ctl *ProductCategoryController) Update(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return categoryError(c, errBadCategoryID, http.StatusBadRequest)
	}
	var item models.ProductCategory
	if err := c.Bind(&item); err != nil {
		return categoryError(c, errBadCategoryPayload, http.StatusBadRequest)
	}
	item.ID = id
	if err := ctl.Service.Update(c.Request().Context(), &item); err != nil {
		return categoryError(c, err, categoryStatusCode(err))
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "message": "报关品类更新成功"})
}

func (ctl *ProductCategoryController) Delete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return categoryError(c, errBadCategoryID, http.StatusBadRequest)
	}
	if err := ctl.Service.Delete(c.Request().Context(), id); err != nil {
		return categoryError(c, err, categoryStatusCode(err))
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "message": "报关品类删除成功"})
}

package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// ProformaInvoiceController 提供形式发票（PI）接口。
type ProformaInvoiceController struct {
	Service *services.ProformaInvoiceService
	Excel   *services.PiExcelService
}

func NewProformaInvoiceController(
	service *services.ProformaInvoiceService,
	excel *services.PiExcelService,
) *ProformaInvoiceController {
	return &ProformaInvoiceController{Service: service, Excel: excel}
}

func piError(c echo.Context, err error, code int) error {
	return c.JSON(code, map[string]any{
		"success": false,
		"message": err.Error(),
	})
}

func (ctl *ProformaInvoiceController) List(c echo.Context) error {
	items, err := ctl.Service.List(c.Request().Context())
	if err != nil {
		return piError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

// Generate 按订单/时间装配一张 PI 草稿（不落库）。
//
// POST /api/proforma-invoices/generate
func (ctl *ProformaInvoiceController) Generate(c echo.Context) error {
	var req models.ProformaInvoiceGenerateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	pi, err := ctl.Service.Generate(c.Request().Context(), &req)
	if err != nil {
		return piError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    pi,
	})
}

func (ctl *ProformaInvoiceController) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "PI ID 无效",
		})
	}

	pi, err := ctl.Service.Get(c.Request().Context(), id)
	if err != nil {
		return piError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    pi,
	})
}

// Save 新增(POST)或更新(PUT) PI。
func (ctl *ProformaInvoiceController) Save(c echo.Context) error {
	var pi models.ProformaInvoice
	if err := c.Bind(&pi); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	if id, err := strconv.ParseUint(c.Param("id"), 10, 64); err == nil && id > 0 {
		pi.ID = id
	}

	if err := ctl.Service.Save(c.Request().Context(), &pi); err != nil {
		return piError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "PI 已保存", "data": pi,
	})
}

func (ctl *ProformaInvoiceController) Delete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "PI ID 无效",
		})
	}

	if err := ctl.Service.Delete(c.Request().Context(), id); err != nil {
		return piError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "PI 已删除",
	})
}

// Export 导出 Excel（以 excel/PI.xlsx 为模板；产品名取 order_items 英文名，缺失数据留空）。
//
// GET /api/proforma-invoices/:id/export
func (ctl *ProformaInvoiceController) Export(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "PI ID 无效",
		})
	}

	ctx := c.Request().Context()

	pi, err := ctl.Service.GetForExport(ctx, id)
	if err != nil {
		return piError(c, err, http.StatusNotFound)
	}

	content, err := ctl.Excel.Build(
		ctx, pi,
		ctl.Service.SKUImages(ctx, pi),
		ctl.Service.OrderImages(ctx, pi),
	)
	if err != nil {
		return piError(c, err, http.StatusInternalServerError)
	}

	filename := pi.PINumber
	if filename == "" {
		filename = fmt.Sprintf("PI-%d", id)
	}

	c.Response().Header().Set(
		echo.HeaderContentType,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	)
	c.Response().Header().Set(
		"Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s.xlsx"`, filename),
	)

	return c.Blob(http.StatusOK, "application/octet-stream", content)
}

// RefreshStockNote 重新生成某 PI 的库存说明。
//
// POST /api/proforma-invoices/:id/refresh-stock-note
func (ctl *ProformaInvoiceController) RefreshStockNote(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "PI ID 无效",
		})
	}

	note, err := ctl.Service.RefreshStockNote(c.Request().Context(), id)
	if err != nil {
		return piError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": "库存说明已刷新",
		"data":    note,
	})
}

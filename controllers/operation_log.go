package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// 清理日志时默认保留的天数。
const defaultOperationLogKeepDays = 90

// OperationLogController 操作日志查询与清理。
type OperationLogController struct {
	Service *services.OperationLogService
}

func NewOperationLogController(service *services.OperationLogService) *OperationLogController {
	return &OperationLogController{Service: service}
}

func operationLogError(c echo.Context, err error, code int) error {
	return c.JSON(code, map[string]any{
		"success": false,
		"message": err.Error(),
	})
}

// operationLogFilter 从查询参数解析筛选条件。
func operationLogFilter(c echo.Context) models.OperationLogFilter {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))

	filter := models.OperationLogFilter{
		Module:   c.QueryParam("module"),
		Action:   c.QueryParam("action"),
		Keyword:  c.QueryParam("keyword"),
		Operator: c.QueryParam("operator"),
		Success:  c.QueryParam("success"),
		DateFrom: c.QueryParam("date_from"),
		DateTo:   c.QueryParam("date_to"),
		Page:     page,
		PageSize: pageSize,
	}
	filter.Normalize()
	return filter
}

// List 分页查询操作日志。
//
// GET /api/operation-logs
func (ctl *OperationLogController) List(c echo.Context) error {
	ctx := c.Request().Context()
	filter := operationLogFilter(c)

	items, total, err := ctl.Service.List(ctx, filter)
	if err != nil {
		return operationLogError(c, err, http.StatusInternalServerError)
	}

	summary, err := ctl.Service.Summary(ctx)
	if err != nil {
		return operationLogError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"items":     items,
			"total":     total,
			"page":      filter.Page,
			"page_size": filter.PageSize,
			"summary":   summary,
		},
	})
}

// Get 查询单条日志（含完整请求摘要）。
//
// GET /api/operation-logs/:id
func (ctl *OperationLogController) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return operationLogError(c, fmt.Errorf("日志 ID 无效"), http.StatusBadRequest)
	}

	item, err := ctl.Service.Get(c.Request().Context(), id)
	if err != nil {
		return operationLogError(c, err, http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{"success": true, "data": item})
}

// Options 返回模块 / 动作筛选下拉项。
//
// GET /api/operation-logs/options
func (ctl *OperationLogController) Options(c echo.Context) error {
	ctx := c.Request().Context()

	modules, err := ctl.Service.Modules(ctx)
	if err != nil {
		return operationLogError(c, err, http.StatusInternalServerError)
	}

	actions, err := ctl.Service.Actions(ctx)
	if err != nil {
		return operationLogError(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"modules": modules,
			"actions": actions,
		},
	})
}

// Export 导出操作日志 CSV。
//
// GET /api/operation-logs/export
func (ctl *OperationLogController) Export(c echo.Context) error {
	data, err := ctl.Service.ExportCSV(c.Request().Context(), operationLogFilter(c))
	if err != nil {
		return operationLogError(c, err, http.StatusInternalServerError)
	}

	filename := "operation-logs-" + time.Now().Format("20060102150405") + ".csv"
	c.Response().Header().Set(
		echo.HeaderContentDisposition,
		"attachment; filename="+filename,
	)

	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", data)
}

// Cleanup 清理 before_days 天之前的日志。
//
// DELETE /api/operation-logs?before_days=90
func (ctl *OperationLogController) Cleanup(c echo.Context) error {
	keepDays := defaultOperationLogKeepDays
	if raw := strings.TrimSpace(c.QueryParam("before_days")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return operationLogError(c, fmt.Errorf("保留天数无效"), http.StatusBadRequest)
		}
		keepDays = value
	}

	deleted, err := ctl.Service.Cleanup(c.Request().Context(), keepDays)
	if err != nil {
		return operationLogError(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("已清理 %d 条 %d 天前的日志", deleted, keepDays),
		"data":    map[string]any{"deleted": deleted},
	})
}

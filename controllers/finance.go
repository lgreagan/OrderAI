package controllers

import (
	"net/http"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// FinanceController 提供财务报表接口。
type FinanceController struct {
	Service *services.FinanceService
}

func NewFinanceController(service *services.FinanceService) *FinanceController {
	return &FinanceController{Service: service}
}

// Report 返回利润报表数据。
//
// GET /api/finance/report?date_from=&date_to=&customer=&keyword=
func (ctl *FinanceController) Report(c echo.Context) error {
	report, err := ctl.Service.Report(
		c.Request().Context(),
		models.FinanceFilter{
			DateFrom: c.QueryParam("date_from"),
			DateTo:   c.QueryParam("date_to"),
			Customer: c.QueryParam("customer"),
			Keyword:  c.QueryParam("keyword"),
		},
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false, "message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    report,
	})
}

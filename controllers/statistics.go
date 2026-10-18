package controllers

import (
	"net/http"

	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// StatisticsController 提供数据统计接口。
type StatisticsController struct {
	Service *services.StatisticsService
}

func NewStatisticsController(service *services.StatisticsService) *StatisticsController {
	return &StatisticsController{Service: service}
}

func (ctl *StatisticsController) Overview(c echo.Context) error {
	ov, err := ctl.Service.Overview(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false, "message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    ov,
	})
}

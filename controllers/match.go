package controllers

import (
	"net/http"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// MatchController 负责订单行与单品 SKU 的匹配预览。
type MatchController struct {
	Service *services.SKUMatchService
}

func NewMatchController(
	service *services.SKUMatchService,
) *MatchController {
	return &MatchController{
		Service: service,
	}
}

// Preview 计算 AI 识别结果的 SKU 匹配预览，不写库。
//
// POST /api/order/match
func (ctl *MatchController) Preview(c echo.Context) error {
	var req models.OrderMatchRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "请求数据格式错误",
		})
	}

	if len(req.Items) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "订单明细不能为空",
		})
	}

	previews, err := ctl.Service.Match(
		c.Request().Context(),
		req.Items,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false,
			"message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"summary": services.SummarizeMatches(previews),
		"data":    previews,
	})
}

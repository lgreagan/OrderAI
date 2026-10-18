package controllers

import (
	"errors"
	"net/http"
	"path/filepath"

	"OrderAI/config"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

type AIController struct {
	Service services.AIService
}

func NewAIController(cfg *config.Config) *AIController {

	service := services.NewDeepSeekService(
		cfg.DeepSeekAPIKey,
		cfg.DeepSeekBaseURL,
		cfg.DeepSeekModel,
	)

	return &AIController{
		Service: service,
	}
}

func (a *AIController) Recognize(c echo.Context) error {

	filename := c.FormValue("filename")

	if filename == "" {
		return c.JSON(
			http.StatusBadRequest,
			map[string]interface{}{
				"success": false,
				"message": "缺少图片文件名",
			},
		)
	}

	// 防止通过 filename 访问 uploads 之外的路径
	filename = filepath.Base(filename)

	imagePath := filepath.Join(
		"uploads",
		filename,
	)

	result, err := a.Service.RecognizeOrder(
		imagePath,
	)

	if err != nil {
		status := http.StatusInternalServerError
		var apiErr *services.DeepSeekAPIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusPaymentRequired {
			status = http.StatusPaymentRequired
		}

		return c.JSON(
			status,
			map[string]interface{}{
				"success": false,
				"message": err.Error(),
			},
		)
	}

	return c.JSON(
		http.StatusOK,
		map[string]interface{}{
			"success": true,
			"data":    result,
		},
	)
}

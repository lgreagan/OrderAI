package controllers

import (
	"net/http"
	"runtime"

	"OrderAI/config"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// SettingsController 提供系统设置与运行时信息接口。
type SettingsController struct {
	Service *services.SettingsService
	Config  *config.Config
}

func NewSettingsController(
	service *services.SettingsService,
	cfg *config.Config,
) *SettingsController {
	return &SettingsController{Service: service, Config: cfg}
}

func (ctl *SettingsController) List(c echo.Context) error {
	items, err := ctl.Service.All(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false, "message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    items,
	})
}

// Save 批量保存设置，请求体为 { "key": "value", ... }。
//
// PUT /api/settings
func (ctl *SettingsController) Save(c echo.Context) error {
	payload := map[string]string{}
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	if err := ctl.Service.Save(c.Request().Context(), payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "设置已保存",
	})
}

// Delete 删除单个设置。
//
// DELETE /api/settings/:key
func (ctl *SettingsController) Delete(c echo.Context) error {
	key := c.Param("key")
	if key == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "设置项名称不能为空",
		})
	}

	if err := ctl.Service.Delete(c.Request().Context(), key); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true, "message": "设置项已删除",
	})
}

// SystemInfo 返回只读的运行时信息。
//
// GET /api/system-info
func (ctl *SettingsController) SystemInfo(c echo.Context) error {
	cfg := ctl.Config

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"app_name":    "OrderAI",
			"app_port":    cfg.AppPort,
			"go_version":  runtime.Version(),
			"db_host":     cfg.DBHost,
			"db_port":     cfg.DBPort,
			"db_name":     cfg.DBName,
			"db_user":     cfg.DBUser,
			"server_os":   runtime.GOOS,
			"server_arch": runtime.GOARCH,
		},
	})
}

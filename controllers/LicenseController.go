package controllers

import (
	"errors"
	"net/http"
	"strings"

	"OrderAI/license"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// LicenseController 激活页与激活接口。未激活时全站都被拦到这里。
type LicenseController struct {
	Service *services.LicenseService
}

func NewLicenseController(service *services.LicenseService) *LicenseController {
	return &LicenseController{Service: service}
}

// Page 渲染激活页（已激活时显示授权状态，可从这里回首页）。
//
// GET /activate
func (ctl *LicenseController) Page(c echo.Context) error {
	return c.Render(http.StatusOK, "activate.html", ctl.viewData())
}

// Status 授权状态（激活页轮询 / 前端刷新用）。
//
// GET /api/license/status
func (ctl *LicenseController) Status(c echo.Context) error {
	view := ctl.viewData()
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    view,
	})
}

// Activate 在线激活：PIN 换授权文件。
//
// POST /api/license/activate
func (ctl *LicenseController) Activate(c echo.Context) error {
	var payload struct {
		PIN string `json:"pin"`
	}
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	_, msg, err := ctl.Service.Activate(payload.PIN)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": err.Error(),
		})
	}
	if msg == "" {
		msg = "激活成功"
	}
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": msg,
		"data":    ctl.viewData(),
	})
}

// Offline 离线激活码激活。
//
// POST /api/license/offline
func (ctl *LicenseController) Offline(c echo.Context) error {
	var payload struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	l, err := ctl.Service.ActivateOffline(payload.Code)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": "激活成功，有效期至 " + l.ExpiresText(),
		"data":    ctl.viewData(),
	})
}

// viewData 激活页需要的全部数据。
func (ctl *LicenseController) viewData() map[string]any {
	view := map[string]any{
		"title":     "系统激活",
		"activated": false,
		"server":    ctl.Service.Server(),
		"file":      ctl.Service.File(),
	}

	machine, cpu, mac, machineErr := ctl.Service.Machine()
	if machineErr != nil {
		view["machine_error"] = machineErr.Error()
		view["machine"] = ""
	} else {
		view["machine"] = machine
		view["cpu"] = cpu
		view["mac"] = mac
	}

	l, err := ctl.Service.Current()
	switch {
	case machineErr != nil:
		view["status_text"] = "读取机器码失败"
	case l != nil:
		valid := l.VerifyLocal(machine)
		view["license"] = l
		view["customer"] = l.Customer
		view["expires"] = l.ExpiresText()
		view["days_left"] = l.DaysLeft()
		view["activated"] = valid == nil
		if valid != nil {
			view["status_text"] = statusText(valid)
		} else {
			view["status_text"] = "已激活"
		}
	case err != nil:
		view["status_text"] = statusText(err)
	default:
		view["status_text"] = "未激活"
	}
	return view
}

func statusText(err error) string {
	switch {
	case errors.Is(err, license.ErrNotFound):
		return "未激活"
	case errors.Is(err, license.ErrExpired):
		return "授权已过期"
	case errors.Is(err, license.ErrMachineMismatch):
		return "授权不属于本机"
	case errors.Is(err, license.ErrTampered):
		return "授权文件已损坏"
	}
	msg := err.Error()
	if strings.Contains(msg, "CPU") || strings.Contains(msg, "网卡") {
		return "读取机器码失败"
	}
	return "未激活"
}

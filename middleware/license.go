package middleware

import (
	"net/http"
	"strings"

	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// licenseExemptPrefixes 未激活也要放行的路径：激活页本身、激活接口、静态资源。
var licenseExemptPrefixes = []string{
	"/activate",
	"/api/license",
	"/web/",
	"/uploads/",
	"/favicon.ico",
}

// LicenseGate 未激活时整站拦到激活页。
//
// 页面请求 303 跳 /activate；接口请求返回 403 JSON，前端可据此提示。
func LicenseGate(svc *services.LicenseService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			for _, p := range licenseExemptPrefixes {
				if strings.HasPrefix(path, p) {
					return next(c)
				}
			}

			if err := svc.Ready(); err != nil {
				if strings.HasPrefix(path, "/api/") {
					return c.JSON(http.StatusForbidden, map[string]interface{}{
						"success":  false,
						"message":  "系统未激活：" + err.Error(),
						"activate": "/activate",
					})
				}
				return c.Redirect(http.StatusSeeOther, "/activate")
			}
			return next(c)
		}
	}
}

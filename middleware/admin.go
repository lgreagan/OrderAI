package middleware

import (
	"net/http"
	"strings"

	"OrderAI/models"

	"github.com/labstack/echo/v4"
)

// RequireAdmin 仅允许管理员访问。
//
// 需放在 Auth 之后：Auth 会把登录账号放进 context。
// 非管理员访问接口返回 403 JSON，访问页面直接回工作台。
func RequireAdmin() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, _ := c.Get(ContextUserKey).(*models.User)
			if user == nil || !user.IsAdmin() {
				if strings.HasPrefix(c.Request().URL.Path, "/api/") {
					return c.JSON(http.StatusForbidden, map[string]any{
						"success": false, "message": "仅管理员可执行该操作",
					})
				}
				return c.Redirect(http.StatusSeeOther, "/")
			}
			return next(c)
		}
	}
}

package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// ContextUserKey 登录用户在 echo.Context 里的键名，页面模板也用这个名字取当前用户。
const ContextUserKey = "user"

// authSkipPrefixes 静态资源与登录页本身不需要登录态。
// 激活页/激活接口也要放行：系统还没激活时，客户连账号都没有。
var authSkipPrefixes = []string{
	"/web/",
	"/uploads/",
	"/login",
	"/api/auth/login",
	"/activate",
	"/api/license/",
	"/favicon.ico",
	"/robots.txt",
}

func skipAuth(path string) bool {
	for _, p := range authSkipPrefixes {
		if path == p || strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// Auth 校验登录会话。
//
// 未登录时：接口请求返回 401 JSON，页面请求跳转登录页并带上 next 便于登录后回到原处。
// 登录成功时把账号放进 context，供控制器与 layout 使用。
func Auth(auth *services.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if skipAuth(path) {
				return next(c)
			}

			token := ""
			if cookie, err := c.Cookie(services.SessionCookie); err == nil {
				token = cookie.Value
			}

			var user *models.User
			if token != "" {
				u, err := auth.UserByToken(c.Request().Context(), token)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]any{
						"success": false, "message": "会话校验失败",
					})
				}
				user = u
			}

			if user == nil {
				// 会话失效顺手清掉 Cookie，避免浏览器一直带着废 token。
				if token != "" {
					clearSessionCookie(c)
				}
				if strings.HasPrefix(path, "/api/") {
					return c.JSON(http.StatusUnauthorized, map[string]any{
						"success": false, "message": "登录已过期，请重新登录",
					})
				}
				next := c.Request().URL.RequestURI()
				return c.Redirect(http.StatusSeeOther, "/login?next="+url.QueryEscape(next))
			}

			c.Set(ContextUserKey, user)
			c.Set("session_token", token)
			return next(c)
		}
	}
}

// clearSessionCookie 过期登录 Cookie。
func clearSessionCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     services.SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

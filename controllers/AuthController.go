package controllers

import (
	"net/http"
	"strings"

	"OrderAI/middleware"
	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// AuthController 登录页与登录 / 退出 / 改密接口。
type AuthController struct {
	Service *services.AuthService
}

func NewAuthController(service *services.AuthService) *AuthController {
	return &AuthController{Service: service}
}

// LoginPage 渲染登录页；已登录直接回到目标页。
//
// GET /login
func (ctl *AuthController) LoginPage(c echo.Context) error {
	next := safeNext(c.QueryParam("next"))

	if token := sessionToken(c); token != "" {
		if u, err := ctl.Service.UserByToken(c.Request().Context(), token); err == nil && u != nil {
			return c.Redirect(http.StatusSeeOther, defaultNext(next))
		}
	}

	return c.Render(http.StatusOK, "login.html", map[string]interface{}{
		"title": "登录",
		"next":  next,
	})
}

// Login 校验账号密码并下发会话 Cookie。
//
// POST /api/auth/login
func (ctl *AuthController) Login(c echo.Context) error {
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	user, sess, err := ctl.Service.Login(
		c.Request().Context(),
		payload.Username, payload.Password,
		c.RealIP(), c.Request().UserAgent(),
		payload.Remember,
	)
	if err != nil {
		status := http.StatusUnauthorized
		message := err.Error()
		switch err {
		case services.ErrUserDisabled:
			status = http.StatusForbidden
		case services.ErrBadCredentials:
			message = services.ErrBadCredentials.Error()
		}
		return c.JSON(status, map[string]any{"success": false, "message": message})
	}

	ttl := services.SessionTTL(payload.Remember)
	c.SetCookie(&http.Cookie{
		Name:     services.SessionCookie,
		Value:    sess.Token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    user,
	})
}

// Logout 退出登录。
//
// POST /api/auth/logout
func (ctl *AuthController) Logout(c echo.Context) error {
	if token := sessionToken(c); token != "" {
		_ = ctl.Service.Logout(c.Request().Context(), token)
	}
	c.SetCookie(&http.Cookie{
		Name:     services.SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

// Me 当前登录账号。
//
// GET /api/auth/me
func (ctl *AuthController) Me(c echo.Context) error {
	u, ok := c.Get(middleware.ContextUserKey).(*models.User)
	if !ok || u == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"success": false})
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "data": u})
}

// ChangePassword 修改当前账号密码。
//
// POST /api/auth/password
func (ctl *AuthController) ChangePassword(c echo.Context) error {
	user, ok := c.Get(middleware.ContextUserKey).(*models.User)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"success": false, "message": "请先登录",
		})
	}

	var payload struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": "请求数据格式错误",
		})
	}

	err := ctl.Service.ChangePassword(
		c.Request().Context(),
		user.ID,
		payload.OldPassword, payload.NewPassword, sessionToken(c),
	)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false, "message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{"success": true, "message": "密码已更新"})
}

// sessionToken 当前请求携带的会话 token。
func sessionToken(c echo.Context) string {
	if v, ok := c.Get("session_token").(string); ok && v != "" {
		return v
	}
	if cookie, err := c.Cookie(services.SessionCookie); err == nil {
		return cookie.Value
	}
	return ""
}

// safeNext 只接受站内路径，避免登录后被跳到外站。
func safeNext(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}
	if strings.HasPrefix(raw, "/login") {
		return ""
	}
	return raw
}

func defaultNext(next string) string {
	if next == "" {
		return "/"
	}
	return next
}

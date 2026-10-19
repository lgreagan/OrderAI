package controllers

import (
	"net/http"
	"strconv"

	"OrderAI/middleware"
	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// UserController 账号管理（仅管理员）。
type UserController struct {
	Service *services.UserService
}

func NewUserController(service *services.UserService) *UserController {
	return &UserController{Service: service}
}

// Page 渲染账号管理页。
//
// GET /users
func (ctl *UserController) Page(c echo.Context) error {
	return c.Render(http.StatusOK, "users.html", map[string]interface{}{
		"title": "用户管理",
	})
}

// List 账号列表。
//
// GET /api/users
func (ctl *UserController) List(c echo.Context) error {
	list, err := ctl.Service.List(c.Request().Context())
	if err != nil {
		return fail(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"data":    list,
	})
}

// Create 新增账号。
//
// POST /api/users
func (ctl *UserController) Create(c echo.Context) error {
	var in services.CreateUserInput
	if err := c.Bind(&in); err != nil {
		return fail(c, http.StatusBadRequest, "请求数据格式错误")
	}

	u, err := ctl.Service.Create(c.Request().Context(), in)
	if err != nil {
		return fail(c, http.StatusBadRequest, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": "账号已创建",
		"data":    u,
	})
}

// Update 改姓名 / 角色 / 状态（停用即在这里改 status）。
//
// PUT /api/users/:id
func (ctl *UserController) Update(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return fail(c, http.StatusBadRequest, "账号 ID 不正确")
	}

	var in services.UpdateUserInput
	if err := c.Bind(&in); err != nil {
		return fail(c, http.StatusBadRequest, "请求数据格式错误")
	}

	operator, _ := c.Get(middleware.ContextUserKey).(*models.User)
	var operatorID uint64
	if operator != nil {
		operatorID = operator.ID
	}

	u, err := ctl.Service.Update(c.Request().Context(), id, in, operatorID)
	if err != nil {
		return fail(c, http.StatusBadRequest, err.Error())
	}

	msg := "账号已保存"
	if u != nil && u.Status == models.UserStatusDisabled {
		msg = "账号已停用"
	} else if u != nil && u.Status == models.UserStatusActive {
		msg = "账号已启用"
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": msg,
		"data":    u,
	})
}

// ResetPassword 重置指定账号的密码。
//
// POST /api/users/:id/password
func (ctl *UserController) ResetPassword(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return fail(c, http.StatusBadRequest, "账号 ID 不正确")
	}

	var payload struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&payload); err != nil {
		return fail(c, http.StatusBadRequest, "请求数据格式错误")
	}

	keep := ""
	if operator, ok := c.Get(middleware.ContextUserKey).(*models.User); ok && operator != nil && operator.ID == id {
		keep = sessionToken(c)
	}

	if err := ctl.Service.ResetPassword(c.Request().Context(), id, payload.Password, keep); err != nil {
		return fail(c, http.StatusBadRequest, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": "密码已重置，该账号需用新密码重新登录",
	})
}

func fail(c echo.Context, status int, message string) error {
	return c.JSON(status, map[string]any{"success": false, "message": message})
}

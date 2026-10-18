package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

type ProductMatchController struct {
	Service *services.ProductMatchService
}

func NewProductMatchController(
	service *services.ProductMatchService,
) *ProductMatchController {
	return &ProductMatchController{
		Service: service,
	}
}

func productMatchError(
	c echo.Context,
	err error,
	code int,
) error {
	return c.JSON(code, map[string]any{
		"success": false,
		"message": err.Error(),
	})
}

// ProductMatchRulesPage 显示组合 SKU 匹配规则列表页面。
func ProductMatchRulesPage(c echo.Context) error {
	return c.Render(200, "product_match_rules.html", map[string]any{
		"title": "组合 SKU 匹配规则",
	})
}

func ProductMatchRulesAPI(c echo.Context) error {
	ctl := c.Get("productMatchController").(*ProductMatchController)
	return ctl.List(c)
}
func ProductMatchRuleStatusAPI(c echo.Context) error {
	ctl := c.Get("productMatchController").(*ProductMatchController)
	return ctl.UpdateStatus(c)
}

// productMatchRuleItemPayload 是新建规则时提交的组成单品。
type productMatchRuleItemPayload struct {
	SkuID          uint64  `json:"sku_id"`
	QuantityPerSet float64 `json:"quantity_per_set"`
	SortOrder      int     `json:"sort_order"`
	Remark         string  `json:"remark"`
}

// productMatchRulePayload 是新建 / 编辑组合规则的请求体。
type productMatchRulePayload struct {
	RuleCode              string                        `json:"rule_code"`
	CustomerProductName   string                        `json:"customer_product_name"`
	CustomerSpecification string                        `json:"customer_specification"`
	ProductNameZH         string                        `json:"product_name_zh"`
	OrderUnit             string                        `json:"order_unit"`
	SetsPerCarton         float64                       `json:"sets_per_carton"`
	CartonWeight          float64                       `json:"carton_weight"`
	BoxLength             float64                       `json:"box_length"`
	BoxWidth              float64                       `json:"box_width"`
	BoxHeight             float64                       `json:"box_height"`
	Status                string                        `json:"status"`
	Remark                string                        `json:"remark"`
	Items                 []productMatchRuleItemPayload `json:"items"`
}

// toModel 把请求体转换为规则模型。
func (payload productMatchRulePayload) toModel() *models.ProductMatchRule {
	return &models.ProductMatchRule{
		RuleCode:              payload.RuleCode,
		CustomerProductName:   payload.CustomerProductName,
		CustomerSpecification: payload.CustomerSpecification,
		ProductNameZH:         payload.ProductNameZH,
		OrderUnit:             payload.OrderUnit,
		SetsPerCarton:         payload.SetsPerCarton,
		CartonWeight:          payload.CartonWeight,
		BoxLength:             payload.BoxLength,
		BoxWidth:              payload.BoxWidth,
		BoxHeight:             payload.BoxHeight,
		Status:                payload.Status,
		Remark:                payload.Remark,
	}
}

// toItems 把请求体中的组成单品转换为模型。
func (payload productMatchRulePayload) toItems() []models.ProductMatchRuleItem {
	items := make([]models.ProductMatchRuleItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, models.ProductMatchRuleItem{
			SkuID:          item.SkuID,
			QuantityPerSet: item.QuantityPerSet,
			SortOrder:      item.SortOrder,
			Remark:         item.Remark,
		})
	}

	return items
}

// productMatchRuleID 解析路径中的规则 ID。
func productMatchRuleID(c echo.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("组合规则 ID 无效")
	}

	return id, nil
}

// productMatchWriteError 把写入规则时的错误映射为合适的响应。
func productMatchWriteError(c echo.Context, err error) error {
	errText := strings.ToLower(err.Error())

	switch {
	case errors.Is(err, models.ErrProductMatchRuleNotFound):
		return productMatchError(c, err, http.StatusNotFound)
	case strings.Contains(errText, "1452"),
		strings.Contains(errText, "foreign key"):
		return productMatchError(
			c,
			errors.New("组成单品不存在，请重新选择 SKU"),
			http.StatusBadRequest,
		)
	case strings.Contains(errText, "duplicate"),
		strings.Contains(errText, "uk_rule_code"):
		return productMatchError(c, err, http.StatusConflict)
	default:
		return productMatchError(c, err, http.StatusBadRequest)
	}
}

// List 查询组合规则列表。
func (ctl *ProductMatchController) List(c echo.Context) error {
	items, err := ctl.Service.List(
		c.Request().Context(),
		strings.TrimSpace(c.QueryParam("keyword")),
		strings.TrimSpace(c.QueryParam("status")),
	)
	if err != nil {
		return productMatchError(
			c,
			err,
			http.StatusInternalServerError,
		)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"total":   len(items),
		"data":    items,
	})
}

// Create 新建组合规则及其组成明细。
func (ctl *ProductMatchController) Create(c echo.Context) error {
	var payload productMatchRulePayload
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "请求数据格式错误",
		})
	}

	created, err := ctl.Service.Create(
		c.Request().Context(),
		payload.toModel(),
		payload.toItems(),
	)
	if err != nil {
		return productMatchWriteError(c, err)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"success": true,
		"message": "组合规则创建成功",
		"data":    created,
	})
}

// Update 编辑组合规则及其组成明细。
//
// PUT /api/product-match-rules/:id
func (ctl *ProductMatchController) Update(c echo.Context) error {
	id, err := productMatchRuleID(c)
	if err != nil {
		return productMatchError(c, err, http.StatusBadRequest)
	}

	var payload productMatchRulePayload
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "请求数据格式错误",
		})
	}

	updated, err := ctl.Service.Update(
		c.Request().Context(),
		id,
		payload.toModel(),
		payload.toItems(),
	)
	if err != nil {
		return productMatchWriteError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": "组合规则已更新",
		"data":    updated,
	})
}

// Delete 删除组合规则。
//
// DELETE /api/product-match-rules/:id?force=1
//
// 规则已被订单行引用时默认拒绝，需带 force=1 二次确认后才会删除；
// 删除后历史订单匹配记录仅解除关联（rule_id 置空），快照字段保留。
func (ctl *ProductMatchController) Delete(c echo.Context) error {
	id, err := productMatchRuleID(c)
	if err != nil {
		return productMatchError(c, err, http.StatusBadRequest)
	}

	references, err := ctl.Service.CountReferences(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrProductMatchRuleNotFound) {
			return productMatchError(c, err, http.StatusNotFound)
		}
		return productMatchError(c, err, http.StatusInternalServerError)
	}

	force := strings.TrimSpace(c.QueryParam("force")) == "1"
	if references > 0 && !force {
		return c.JSON(http.StatusConflict, map[string]any{
			"success": false,
			"message": fmt.Sprintf(
				"该规则已被 %d 条订单明细引用。删除后历史订单将解除与该规则的关联"+
					"（已快照的规则编号保留）。确认删除请再次确认。",
				references,
			),
			"references": references,
		})
	}

	deleted, err := ctl.Service.Delete(c.Request().Context(), id)
	if err != nil {
		return productMatchWriteError(c, err)
	}

	message := "组合规则已删除"
	if deleted > 0 {
		message = fmt.Sprintf(
			"组合规则已删除，已解除 %d 条订单明细的规则关联",
			deleted,
		)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": message,
	})
}

// UpdateStatus 启用或停用组合匹配规则。
func (ctl *ProductMatchController) UpdateStatus(
	c echo.Context,
) error {
	id, err := productMatchRuleID(c)
	if err != nil {
		return productMatchError(c, err, http.StatusBadRequest)
	}

	var payload struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "请求数据格式错误",
		})
	}

	err = ctl.Service.UpdateStatus(
		c.Request().Context(),
		id,
		payload.Status,
	)
	if err != nil {
		status := http.StatusBadRequest

		if errors.Is(err, models.ErrProductMatchRuleNotFound) {
			status = http.StatusNotFound
		}

		return productMatchError(c, err, status)
	}

	action := "启用"
	if strings.TrimSpace(payload.Status) == "inactive" {
		action = "停用"
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": "组合规则已" + action,
	})
}

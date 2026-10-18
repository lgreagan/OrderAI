package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"OrderAI/models"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// ============================================================
// 关键操作日志
//
// 记录会改动业务数据的接口调用（POST/PUT/PATCH/DELETE，以及少量导出类 GET）：
// 模块与操作描述由 operationRules 给出，未登记的写接口走通用规则兜底，
// 上传图片、匹配预览等不落业务数据的请求忽略。
// ============================================================

// operationRule 一条关键操作的定义。
type operationRule struct {
	Module      string
	ModuleName  string
	Action      string
	Description string
}

// rule 构造操作规则。
func rule(module, moduleName, action, description string) operationRule {
	return operationRule{
		Module:      module,
		ModuleName:  moduleName,
		Action:      action,
		Description: description,
	}
}

// 模块标识。
const (
	moduleOrders     = "orders"
	moduleProducts   = "products"
	moduleMatch      = "product_match"
	modulePurchase   = "purchase"
	moduleProduction = "production"
	moduleBOM        = "bom"
	moduleMaterials  = "materials"
	moduleInventory  = "inventory"
	moduleShipments  = "shipments"
	moduleDeclare    = "declaration"
	moduleWarehouse  = "warehouse"
	moduleCategory   = "category"
	modulePI         = "proforma_invoice"
	modulePayments   = "payments"
	moduleSettings   = "settings"
	moduleAI         = "ai"
	moduleAudit      = "audit"
)

// operationRules 关键操作表，键为「请求方法 + 路由模式」。
var operationRules = map[string]operationRule{
	// AI 订单识别
	"POST /api/order/recognize": rule(moduleAI, "AI订单识别", models.OperationActionExecute, "AI识别订单"),

	// 订单
	"POST /api/orders":         rule(moduleOrders, "订单管理", models.OperationActionCreate, "新增订单"),
	"PUT /api/orders/:id":      rule(moduleOrders, "订单管理", models.OperationActionUpdate, "修改订单"),
	"DELETE /api/orders/:id":   rule(moduleOrders, "订单管理", models.OperationActionDelete, "删除订单"),
	"POST /api/orders/confirm": rule(moduleOrders, "订单管理", models.OperationActionExecute, "确认订单入库"),

	// 产品(SKU)
	"POST /api/skus":       rule(moduleProducts, "产品管理", models.OperationActionCreate, "新增产品"),
	"PUT /api/skus/:id":    rule(moduleProducts, "产品管理", models.OperationActionUpdate, "修改产品"),
	"DELETE /api/skus/:id": rule(moduleProducts, "产品管理", models.OperationActionDelete, "删除产品"),

	// 产品匹配规则
	"POST /api/product-match-rules":            rule(moduleMatch, "产品匹配规则", models.OperationActionCreate, "新增产品匹配规则"),
	"PUT /api/product-match-rules/:id":         rule(moduleMatch, "产品匹配规则", models.OperationActionUpdate, "修改产品匹配规则"),
	"POST /api/product-match-rules/:id/status": rule(moduleMatch, "产品匹配规则", models.OperationActionExecute, "变更匹配规则状态"),

	// 采购订单
	"POST /api/purchase-orders":                          rule(modulePurchase, "采购订单", models.OperationActionCreate, "新增采购订单"),
	"PUT /api/purchase-orders/:id":                       rule(modulePurchase, "采购订单", models.OperationActionUpdate, "修改采购订单"),
	"DELETE /api/purchase-orders/:id":                    rule(modulePurchase, "采购订单", models.OperationActionDelete, "删除采购订单"),
	"POST /api/purchase-orders/:id/receive":              rule(modulePurchase, "采购订单", models.OperationActionExecute, "采购订单到货入库"),
	"POST /api/purchase-orders/order/:orderId/push":      rule(modulePurchase, "采购订单", models.OperationActionExecute, "按订单生成采购订单"),
	"POST /api/purchase-orders/order/:orderId/purchased": rule(modulePurchase, "采购订单", models.OperationActionExecute, "标记订单已采购"),

	// 生产订单
	"POST /api/production-orders":                        rule(moduleProduction, "生产订单", models.OperationActionCreate, "新增生产订单"),
	"PUT /api/production-orders/:id":                     rule(moduleProduction, "生产订单", models.OperationActionUpdate, "修改生产订单"),
	"DELETE /api/production-orders/:id":                  rule(moduleProduction, "生产订单", models.OperationActionDelete, "删除生产订单"),
	"POST /api/production-orders/:id/complete":           rule(moduleProduction, "生产订单", models.OperationActionExecute, "生产订单完工入库"),
	"POST /api/production-orders/order/:orderId/push":    rule(moduleProduction, "生产订单", models.OperationActionExecute, "按订单生成生产订单"),
	"POST /api/production-orders/order/:orderId/produced": rule(moduleProduction, "生产订单", models.OperationActionExecute, "标记订单已生产"),

	// 产品用料(BOM)
	"POST /api/product-bom":       rule(moduleBOM, "产品用料", models.OperationActionCreate, "新增产品用料"),
	"PUT /api/product-bom/:id":    rule(moduleBOM, "产品用料", models.OperationActionUpdate, "修改产品用料"),
	"DELETE /api/product-bom/:id": rule(moduleBOM, "产品用料", models.OperationActionDelete, "删除产品用料"),

	// 材料
	"POST /api/materials":       rule(moduleMaterials, "材料管理", models.OperationActionCreate, "新增材料"),
	"PUT /api/materials/:id":    rule(moduleMaterials, "材料管理", models.OperationActionUpdate, "修改材料"),
	"DELETE /api/materials/:id": rule(moduleMaterials, "材料管理", models.OperationActionDelete, "删除材料"),

	// 库存
	"POST /api/inventory/in":     rule(moduleInventory, "库存管理", models.OperationActionExecute, "库存入库"),
	"POST /api/inventory/issue":  rule(moduleInventory, "库存管理", models.OperationActionExecute, "领料出库"),
	"POST /api/inventory/adjust": rule(moduleInventory, "库存管理", models.OperationActionExecute, "库存盘点调整"),

	// 发货
	"POST /api/shipments":                        rule(moduleShipments, "发货管理", models.OperationActionCreate, "新增发货单"),
	"PUT /api/shipments/:id":                     rule(moduleShipments, "发货管理", models.OperationActionUpdate, "修改发货单"),
	"DELETE /api/shipments/:id":                  rule(moduleShipments, "发货管理", models.OperationActionDelete, "删除发货单"),
	"POST /api/shipments/order/:orderId/push":    rule(moduleShipments, "发货管理", models.OperationActionExecute, "按订单生成发货单"),
	"POST /api/shipments/:id/declaration/export": rule(moduleDeclare, "报关管理", models.OperationActionExport, "生成报关资料"),

	// 报关模板 / 仓库地址
	"PUT /api/declaration-templates/:id/mapping": rule(moduleDeclare, "报关管理", models.OperationActionUpdate, "保存报关模板映射"),
	"POST /api/warehouse-addresses":              rule(moduleWarehouse, "仓库地址", models.OperationActionCreate, "新增仓库地址"),
	"PUT /api/warehouse-addresses/:id":           rule(moduleWarehouse, "仓库地址", models.OperationActionUpdate, "修改仓库地址"),
	"DELETE /api/warehouse-addresses/:id":        rule(moduleWarehouse, "仓库地址", models.OperationActionDelete, "删除仓库地址"),

	// 报关品类
	"POST /api/product-categories":       rule(moduleCategory, "报关品类", models.OperationActionCreate, "新增报关品类"),
	"PUT /api/product-categories/:id":    rule(moduleCategory, "报关品类", models.OperationActionUpdate, "修改报关品类"),
	"DELETE /api/product-categories/:id": rule(moduleCategory, "报关品类", models.OperationActionDelete, "删除报关品类"),

	// 形式发票(PI)
	"POST /api/proforma-invoices":                        rule(modulePI, "形式发票(PI)", models.OperationActionCreate, "新增形式发票"),
	"PUT /api/proforma-invoices/:id":                     rule(modulePI, "形式发票(PI)", models.OperationActionUpdate, "修改形式发票"),
	"DELETE /api/proforma-invoices/:id":                  rule(modulePI, "形式发票(PI)", models.OperationActionDelete, "删除形式发票"),
	"POST /api/proforma-invoices/generate":               rule(modulePI, "形式发票(PI)", models.OperationActionExecute, "生成形式发票(PI)"),
	"POST /api/proforma-invoices/:id/refresh-stock-note": rule(modulePI, "形式发票(PI)", models.OperationActionExecute, "刷新PI库存说明"),
	"GET /api/proforma-invoices/:id/export":              rule(modulePI, "形式发票(PI)", models.OperationActionExport, "导出形式发票(PI)"),

	// 收款
	"POST /api/payments":       rule(modulePayments, "收款管理", models.OperationActionCreate, "登记收款"),
	"PUT /api/payments/:id":    rule(modulePayments, "收款管理", models.OperationActionUpdate, "修改收款记录"),
	"DELETE /api/payments/:id": rule(modulePayments, "收款管理", models.OperationActionDelete, "删除收款记录"),

	// 系统设置
	"PUT /api/settings":         rule(moduleSettings, "系统设置", models.OperationActionUpdate, "保存系统设置"),
	"DELETE /api/settings/:key": rule(moduleSettings, "系统设置", models.OperationActionDelete, "删除系统配置"),

	// 操作日志自身
	"GET /api/operation-logs/export": rule(moduleAudit, "操作日志", models.OperationActionExport, "导出操作日志"),
	"DELETE /api/operation-logs":     rule(moduleAudit, "操作日志", models.OperationActionExecute, "清理过期操作日志"),
}

// ignoredRoutes 不落业务数据、不进日志的接口。
var ignoredRoutes = map[string]bool{
	"POST /api/order/upload":      true,
	"POST /api/skus/upload-image": true,
	"POST /api/order/match":       true,
}

// operationModules 兜底规则使用的路径前缀 → 模块。
var operationModules = []struct {
	Prefix string
	Module string
	Name   string
}{
	{"/api/inventory", moduleInventory, "库存管理"},
	{"/api/material", moduleMaterials, "材料管理"},
	{"/api/product-match", moduleMatch, "产品匹配规则"},
	{"/api/product-bom", moduleBOM, "产品用料"},
	{"/api/product-categories", moduleCategory, "报关品类"},
	{"/api/skus", moduleProducts, "产品管理"},
	{"/api/purchase", modulePurchase, "采购订单"},
	{"/api/production", moduleProduction, "生产订单"},
	{"/api/shipment", moduleShipments, "发货管理"},
	{"/api/declaration", moduleDeclare, "报关管理"},
	{"/api/warehouse", moduleWarehouse, "仓库地址"},
	{"/api/proforma", modulePI, "形式发票(PI)"},
	{"/api/payment", modulePayments, "收款管理"},
	{"/api/settings", moduleSettings, "系统设置"},
	{"/api/operation-logs", moduleAudit, "操作日志"},
	{"/api/order", moduleOrders, "订单管理"},
	{"/api", "system", "系统"},
}

// 请求体与响应体的采集上限，避免日志表被大报文撑大。
const (
	operationLogMaxRequestBytes = 1 << 20 // 请求体超过该大小则不读取
	operationLogBodyLimit       = 2000    // detail 中保留的请求体字符数
	operationLogDetailLimit     = 4000    // detail 字段总长度上限
	operationLogResponseCapture = 8 << 10 // 响应体采集上限
)

// OperationLog 返回记录关键操作的中间件。
func OperationLog(service *services.OperationLogService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			opRule, ok := matchOperationRule(c)
			if !ok {
				return next(c)
			}

			start := time.Now()
			body := captureRequestBody(c)

			recorder := newResponseRecorder(c)
			err := next(c)
			// 采集完成立刻换回原writer：后续（如 Echo 的错误处理）的写操作
			// 直接落到客户端，不再进入采集缓冲区。
			recorder.restore()

			status := c.Response().Status
			if err != nil {
				status = statusFromError(err)
			}

			payload := recorder.payload()
			success := err == nil && status >= http.StatusOK && status < http.StatusMultipleChoices
			if payload != nil && !payload.Success && status < http.StatusMultipleChoices {
				success = false
			}

			item := &models.OperationLog{
				Module:      opRule.Module,
				ModuleName:  opRule.ModuleName,
				Action:      opRule.Action,
				ActionName:  models.OperationActionName(opRule.Action),
				Description: opRule.Description,
				Method:      c.Request().Method,
				Path:        c.Path(),
				URI:         truncate(c.Request().URL.RequestURI(), 500),
				TargetID:    targetID(c, payload),
				Operator:    models.DefaultOperator,
				IP:          c.RealIP(),
				Status:      status,
				Success:     success,
				DurationMs:  time.Since(start).Milliseconds(),
			}

			if !success {
				item.ErrorMsg = failureMessage(err, payload, status)
			}

			item.Detail = buildDetail(c, body, payload)

			service.Record(item)

			return err
		}
	}
}

// matchOperationRule 判断当前请求是否属于关键操作。
func matchOperationRule(c echo.Context) (operationRule, bool) {
	method := strings.ToUpper(c.Request().Method)
	key := method + " " + c.Path()

	if opRule, ok := operationRules[key]; ok {
		return opRule, true
	}

	// GET 只有导出类需要记录，未登记的直接跳过。
	if method != http.MethodPost && method != http.MethodPut &&
		method != http.MethodPatch && method != http.MethodDelete {
		return operationRule{}, false
	}

	if ignoredRoutes[key] || !strings.HasPrefix(c.Path(), "/api/") {
		return operationRule{}, false
	}

	// 未登记的写接口走通用规则，避免新增接口漏记。
	module, moduleName := moduleByPath(c.Path())
	action := actionByMethod(method, c.Path())
	return rule(
		module,
		moduleName,
		action,
		moduleName+models.OperationActionName(action)+"："+c.Path(),
	), true
}

// moduleByPath 按路径前缀推断模块。
func moduleByPath(path string) (string, string) {
	for _, m := range operationModules {
		if strings.HasPrefix(path, m.Prefix) {
			return m.Module, m.Name
		}
	}
	return "system", "系统"
}

func actionByMethod(method, path string) string {
	switch method {
	case http.MethodPost:
		// POST 落在「资源 + 动作」的地址上（如 /api/orders/confirm）按执行处理。
		if isSubActionPath(path) {
			return models.OperationActionExecute
		}
		return models.OperationActionCreate
	case http.MethodPut, http.MethodPatch:
		return models.OperationActionUpdate
	case http.MethodDelete:
		return models.OperationActionDelete
	}
	return models.OperationActionOther
}

// isSubActionPath 判断路径是否为「资源 + 动作」形式。
func isSubActionPath(path string) bool {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) < 3 {
		return false
	}
	// 末段是路由参数（/api/orders/:id）说明仍是资源本身，其余按动作处理。
	return !strings.HasPrefix(segments[len(segments)-1], ":")
}

// statusFromError 从处理函数返回的错误里取 HTTP 状态码。
func statusFromError(err error) int {
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Code
	}
	return http.StatusInternalServerError
}

// failureMessage 组装失败原因。
func failureMessage(err error, payload *responsePayload, status int) string {
	if payload != nil && payload.Message != "" {
		return truncate(payload.Message, 500)
	}
	if err != nil {
		return truncate(err.Error(), 500)
	}
	return "HTTP " + strconv.Itoa(status)
}

// targetID 优先取路由参数（id 优先，其次其他命名参数），最后取响应里的 data.id。
func targetID(c echo.Context, payload *responsePayload) string {
	if id := c.Param("id"); id != "" {
		return truncate(id, 100)
	}
	for _, name := range c.ParamNames() {
		if value := c.Param(name); value != "" {
			return truncate(value, 100)
		}
	}
	if payload != nil {
		return truncate(payload.DataID(), 100)
	}
	return ""
}

// buildDetail 组装请求摘要 JSON。
func buildDetail(c echo.Context, body string, payload *responsePayload) string {
	detail := map[string]any{}

	if params := c.ParamNames(); len(params) > 0 {
		values := map[string]string{}
		for _, name := range params {
			values[name] = c.Param(name)
		}
		detail["params"] = values
	}

	if query := c.QueryParams(); len(query) > 0 {
		values := map[string]string{}
		for key, list := range query {
			values[key] = strings.Join(list, ",")
		}
		detail["query"] = values
	}

	if body != "" {
		detail["body"] = body
	}
	if payload != nil && payload.Message != "" {
		detail["message"] = payload.Message
	}

	if len(detail) == 0 {
		return ""
	}

	data, err := json.Marshal(detail)
	if err != nil {
		return ""
	}
	return truncate(string(data), operationLogDetailLimit)
}

// captureRequestBody 读取 JSON 请求体并原样还原，返回截断后的文本。
func captureRequestBody(c echo.Context) string {
	req := c.Request()
	if req.Body == nil {
		return ""
	}
	if !strings.HasPrefix(req.Header.Get(echo.HeaderContentType), echo.MIMEApplicationJSON) {
		return ""
	}
	if req.ContentLength > operationLogMaxRequestBytes {
		return ""
	}

	data, err := io.ReadAll(io.LimitReader(req.Body, operationLogMaxRequestBytes+1))
	// 已读部分与剩余部分拼回请求体，保证后续解析不受影响。
	req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(data), req.Body))
	if err != nil {
		return ""
	}

	return truncate(string(data), operationLogBodyLimit)
}

// responsePayload 业务接口统一返回的 JSON 结构。
type responsePayload struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// DataID 从 data 中取新建/更新对象的主键。
func (p *responsePayload) DataID() string {
	if len(p.Data) == 0 {
		return ""
	}

	var object map[string]any
	if err := json.Unmarshal(p.Data, &object); err != nil {
		return ""
	}

	switch value := object["id"].(type) {
	case float64:
		return strconv.FormatInt(int64(value), 10)
	case string:
		return value
	}
	return ""
}

// responseRecorder 旁路采集响应体，用于取业务返回的提示语与主键。
type responseRecorder struct {
	http.ResponseWriter
	original http.ResponseWriter
	response *echo.Response
	buf      bytes.Buffer
}

func newResponseRecorder(c echo.Context) *responseRecorder {
	response := c.Response()
	recorder := &responseRecorder{
		ResponseWriter: response.Writer,
		original:       response.Writer,
		response:       response,
	}
	response.Writer = recorder
	return recorder
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if remaining := operationLogResponseCapture - r.buf.Len(); remaining > 0 {
		if len(b) <= remaining {
			r.buf.Write(b)
		} else {
			r.buf.Write(b[:remaining])
		}
	}
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// restore 把响应writer换回原来的实现，handler返回后立即调用，
// 避免采集缓冲区继续接收后续写出的内容。
func (r *responseRecorder) restore() {
	if r.response == nil {
		return
	}
	r.response.Writer = r.original
}

// payload 解析采集到的响应体。
func (r *responseRecorder) payload() *responsePayload {
	if r.buf.Len() == 0 {
		return nil
	}

	payload := &responsePayload{}
	if err := json.Unmarshal(r.buf.Bytes(), payload); err != nil {
		return nil
	}
	return payload
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

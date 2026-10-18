package routes

import (
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"OrderAI/config"
	"OrderAI/controllers"
	"OrderAI/middleware"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// layoutPages 是使用公共 layout 的页面文件，文件内只定义 "content" 区块。
var layoutPages = []string{
	"index.html",
	"orders.html",
	"order_form.html",
	"order_detail.html",
	"products.html",
	"product_match_rules.html",
	"order_upload.html",
	"purchase_orders.html",
	"production_orders.html",
	"product_bom.html",
	"materials.html",
	"inventory.html",
	"material_issues.html",
	"shipments.html",
	"proforma_invoices.html",
	"payments.html",
	"statistics.html",
	"finance.html",
	"settings.html",
	"product_categories.html",
	"declaration_templates.html",
	"operation_logs.html",
}

// standalonePages 是独立完整页面，文件本身就是完整 HTML。
var standalonePages = []string{
	"order_upload.html",
	"purchase_order_print.html",
	"production_order_print.html",
	"purchase_order_screen.html",
	"production_order_screen.html",
	"purchase_report.html",
	"production_report.html",
}

func Register(
	e *echo.Echo,
	orderService *services.OrderService, skuService *services.SKUService, productMatchService *services.ProductMatchService, matchService *services.SKUMatchService,
	purchaseOrderService *services.PurchaseOrderService, productionOrderService *services.ProductionOrderService,
	materialService *services.MaterialService,
	inventoryService *services.InventoryService,
	shipmentService *services.ShipmentService,
	proformaInvoiceService *services.ProformaInvoiceService,
	productBomService *services.ProductBomService,
	paymentService *services.PaymentService,
	settingsService *services.SettingsService,
	statisticsService *services.StatisticsService,
	financeService *services.FinanceService,
	declarationService *services.DeclarationService,
	productCategoryService *services.ProductCategoryService,
	warehouseAddressService *services.WarehouseAddressService,
	shippingChannelService *services.ShippingChannelService,
	operationLogService *services.OperationLogService,
) {

	e.Renderer = NewTemplateRenderer("templates")

	// 关键操作日志：记录改动业务数据的接口调用，供事后追溯。
	e.Use(middleware.OperationLog(operationLogService))

	// 页面与前端资源不做强缓存：改动模板/JS/CSS 后刷新页面即可生效。
	// 上传的订单、产品图片保持可缓存。
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !strings.HasPrefix(c.Request().URL.Path, "/uploads/") {
				c.Response().Header().Set(
					echo.HeaderCacheControl,
					"no-cache, must-revalidate",
				)
			}
			return next(c)
		}
	})

	e.Static(
		"/web",
		"web",
	)

	e.Static(
		"/uploads",
		"uploads",
	)

	cfg := config.Load()

	aiController := controllers.NewAIController(
		cfg,
	)

	orderController := controllers.NewOrderController(
		orderService,
	)

	// 首页
	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "index.html", map[string]interface{}{
			"title": "工作台",
		})
	})
	e.GET("/orderupload", func(c echo.Context) error {
		return c.Render(http.StatusOK, "order_upload.html", map[string]interface{}{
			"title": "上传订单",
		})
	})

	// 图片上传
	e.POST(
		"/api/order/upload",
		controllers.UploadOrderImage,
	)

	// AI识别
	e.POST(
		"/api/order/recognize",
		aiController.Recognize,
	)

	// 订单行 SKU 匹配预览（识别之后、确认入库之前）
	matchController := controllers.NewMatchController(matchService)

	e.POST(
		"/api/order/match",
		matchController.Preview,
	)

	// ========================================
	// Order CRUD API
	// ========================================
	// ========================================
	// Order API
	// ========================================
	e.GET("/api/orders", orderController.List)
	e.GET("/api/orders/:id", orderController.Get)
	e.POST("/api/orders", orderController.Create)
	e.PUT("/api/orders/:id", orderController.Update)
	e.DELETE("/api/orders/:id", orderController.Delete)
	e.POST("/api/orders/confirm", orderController.Confirm)
	e.GET("/orders", func(c echo.Context) error {
		return c.Render(200, "orders.html", map[string]interface{}{
			"title": "订单管理",
		})
	})
	e.GET("/orders/new", func(c echo.Context) error {
		return c.Render(http.StatusOK, "order_form.html", map[string]interface{}{
			"title": "新建订单",
			"id":    "",
		})
	})
	e.GET("/orders/:id/edit", func(c echo.Context) error {
		return c.Render(http.StatusOK, "order_form.html", map[string]interface{}{
			"title": "编辑订单",
			"id":    c.Param("id"),
		})
	})
	e.GET("/orders/:id", func(c echo.Context) error {
		return c.Render(http.StatusOK, "order_detail.html", map[string]interface{}{
			"id": c.Param("id"),
		})
	})

	skuController := controllers.NewSKUController(skuService)

	e.GET("/products", func(c echo.Context) error {
		return c.Render(200, "products.html", map[string]any{
			"title": "SKU 管理",
		})
	})

	e.GET("/api/skus", skuController.List)
	e.GET("/api/skus/:id", skuController.Get)
	e.POST("/api/skus/upload-image", controllers.UploadSKUImage)
	e.POST("/api/skus", skuController.Create)
	e.PUT("/api/skus/:id", skuController.Update)
	e.DELETE("/api/skus/:id", skuController.Delete)

	productMatchController := controllers.NewProductMatchController(productMatchService)
	e.GET("/product-match-rules", controllers.ProductMatchRulesPage)
	e.GET("/api/product-match-rules", productMatchController.List)
	e.POST("/api/product-match-rules", productMatchController.Create)
	e.PUT("/api/product-match-rules/:id", productMatchController.Update)
	e.DELETE("/api/product-match-rules/:id", productMatchController.Delete)
	e.POST("/api/product-match-rules/:id/status", productMatchController.UpdateStatus)

	// ========================================
	// 采购订单（持久化，可人工维护，可超量采购 → 入库形成库存）
	// ========================================

	purchaseOrderController := controllers.NewPurchaseOrderController(purchaseOrderService)

	e.GET("/purchase-orders", func(c echo.Context) error {
		return c.Render(http.StatusOK, "purchase_orders.html", map[string]any{
			"title": "采购订单",
		})
	})
	e.GET("/purchase-orders/report", func(c echo.Context) error {
		return c.Render(http.StatusOK, "purchase_report.html", map[string]any{
			"title": "采购报表",
		})
	})
	e.GET("/purchase-orders/:id/print", func(c echo.Context) error {
		return c.Render(http.StatusOK, "purchase_order_print.html", map[string]any{
			"title": "采购订单 · 打印",
			"id":    c.Param("id"),
		})
	})
	e.GET("/purchase-orders/:id/screen", func(c echo.Context) error {
		return c.Render(http.StatusOK, "purchase_order_screen.html", map[string]any{
			"title": "采购批次汇总 · 截图",
			"id":    c.Param("id"),
		})
	})
	e.GET("/api/purchase-orders", purchaseOrderController.List)
	e.POST("/api/purchase-orders", purchaseOrderController.Create)
	e.GET("/api/purchase-orders/report/items", purchaseOrderController.ItemSummary)
	e.POST("/api/purchase-orders/material-needs", purchaseOrderController.MaterialNeeds)
	e.GET("/api/purchase-orders/order/:orderId/suggestion", purchaseOrderController.Suggestion)
	e.POST("/api/purchase-orders/order/:orderId/push", purchaseOrderController.PushDown)
	e.POST("/api/purchase-orders/order/:orderId/purchased", purchaseOrderController.MarkPurchased)
	e.GET("/api/purchase-orders/:id", purchaseOrderController.Get)
	e.PUT("/api/purchase-orders/:id", purchaseOrderController.Update)
	e.DELETE("/api/purchase-orders/:id", purchaseOrderController.Delete)
	e.POST("/api/purchase-orders/:id/receive", purchaseOrderController.Receive)

	// ========================================
	// 生产订单（持久化）+ 产品用料(BOM)
	// ========================================

	productionOrderController := controllers.NewProductionOrderController(productionOrderService)

	e.GET("/production-orders", func(c echo.Context) error {
		return c.Render(http.StatusOK, "production_orders.html", map[string]any{
			"title": "生产订单",
		})
	})
	e.GET("/production-orders/report", func(c echo.Context) error {
		return c.Render(http.StatusOK, "production_report.html", map[string]any{
			"title": "生产报表",
		})
	})
	e.GET("/production-orders/:id/print", func(c echo.Context) error {
		return c.Render(http.StatusOK, "production_order_print.html", map[string]any{
			"title": "生产订单 · 打印",
			"id":    c.Param("id"),
		})
	})
	e.GET("/production-orders/:id/screen", func(c echo.Context) error {
		return c.Render(http.StatusOK, "production_order_screen.html", map[string]any{
			"title": "生产订单 · 截图",
			"id":    c.Param("id"),
		})
	})
	e.GET("/api/production-orders", productionOrderController.List)
	e.POST("/api/production-orders", productionOrderController.Create)
	e.GET("/api/production-orders/report/materials", productionOrderController.MaterialSummary)
	e.GET("/api/production-orders/order/:orderId/suggestion", productionOrderController.Suggestion)
	e.POST("/api/production-orders/order/:orderId/push", productionOrderController.PushDown)
	e.POST("/api/production-orders/order/:orderId/produced", productionOrderController.MarkProduced)
	e.GET("/api/production-orders/:id", productionOrderController.Get)
	e.GET("/api/production-orders/:id/materials", productionOrderController.Materials)
	e.PUT("/api/production-orders/:id", productionOrderController.Update)
	e.DELETE("/api/production-orders/:id", productionOrderController.Delete)
	e.POST("/api/production-orders/:id/complete", productionOrderController.Complete)

	productBomController := controllers.NewProductBomController(productBomService)

	e.GET("/product-bom", func(c echo.Context) error {
		return c.Render(http.StatusOK, "product_bom.html", map[string]any{
			"title": "产品用料(BOM)",
		})
	})
	e.GET("/api/product-bom", productBomController.List)
	e.POST("/api/product-bom", productBomController.Create)
	e.GET("/api/product-bom/:id", productBomController.Get)
	e.PUT("/api/product-bom/:id", productBomController.Update)
	e.DELETE("/api/product-bom/:id", productBomController.Delete)

	// ========================================
	// 材料（包材/原材料）
	// ========================================

	materialController := controllers.NewMaterialController(materialService)

	e.GET("/materials", func(c echo.Context) error {
		return c.Render(http.StatusOK, "materials.html", map[string]any{
			"title": "材料管理",
		})
	})
	e.GET("/api/materials", materialController.List)
	e.GET("/api/materials/:id", materialController.Get)
	e.POST("/api/materials", materialController.Create)
	e.PUT("/api/materials/:id", materialController.Update)
	e.DELETE("/api/materials/:id", materialController.Delete)

	// ========================================
	// 库存
	// ========================================

	inventoryController := controllers.NewInventoryController(inventoryService)

	e.GET("/inventory", func(c echo.Context) error {
		return c.Render(http.StatusOK, "inventory.html", map[string]any{
			"title": "库存管理",
		})
	})
	e.GET("/api/inventory/stock", inventoryController.Stock)
	e.GET("/api/inventory/transactions", inventoryController.Transactions)
	e.POST("/api/inventory/in", inventoryController.In)
	e.POST("/api/inventory/issue", inventoryController.Issue)
	e.POST("/api/inventory/adjust", inventoryController.Adjust)

	// 领料出库（单独页面，材料按实际领用量出库，可含损耗）
	e.GET("/material-issues", func(c echo.Context) error {
		return c.Render(http.StatusOK, "material_issues.html", map[string]any{
			"title": "领料出库",
		})
	})

	// ========================================
	// 发货
	// ========================================

	shipmentController := controllers.NewShipmentController(shipmentService)

	e.GET("/shipments", func(c echo.Context) error {
		return c.Render(http.StatusOK, "shipments.html", map[string]any{
			"title": "发货管理",
		})
	})
	e.GET("/api/shipments", shipmentController.List)
	e.GET("/api/shipments/:id", shipmentController.Get)
	e.POST("/api/shipments", shipmentController.Create)
	e.PUT("/api/shipments/:id", shipmentController.Update)
	e.DELETE("/api/shipments/:id", shipmentController.Delete)
	e.GET("/api/shipments/order/:orderId/lines", shipmentController.OrderLines)
	e.POST("/api/shipments/order/:orderId/push", shipmentController.PushDown)
	e.POST("/api/shipments/packaging-needs", shipmentController.PackagingNeeds)

	// ========================================
	// 出口报关资料（货代 xls 模板）
	// ========================================

	declarationController := controllers.NewDeclarationController(
		declarationService,
		services.NewDeclarationExcelService(declarationService.Templates),
		shippingChannelService,
	)
	productCategoryController := controllers.NewProductCategoryController(productCategoryService)
	warehouseAddressController := controllers.NewWarehouseAddressController(warehouseAddressService)

	e.GET("/product-categories", func(c echo.Context) error {
		return c.Render(http.StatusOK, "product_categories.html", map[string]any{
			"title": "报关品类",
		})
	})
	e.GET("/api/product-categories", productCategoryController.List)
	e.GET("/api/product-categories/:id", productCategoryController.Get)
	e.POST("/api/product-categories", productCategoryController.Create)
	e.PUT("/api/product-categories/:id", productCategoryController.Update)
	e.DELETE("/api/product-categories/:id", productCategoryController.Delete)

	e.GET("/declaration-templates", func(c echo.Context) error {
		return c.Render(http.StatusOK, "declaration_templates.html", map[string]any{
			"title": "报关模板",
		})
	})
	e.GET("/api/declaration-templates", declarationController.ListTemplates)
	e.PUT("/api/declaration-templates/:id/mapping", declarationController.SaveMapping)

	e.GET("/api/shipping-channels", declarationController.ListChannels)

	e.GET("/api/warehouse-addresses", warehouseAddressController.List)
	e.GET("/api/warehouse-addresses/countries", warehouseAddressController.Countries)
	e.GET("/api/warehouse-addresses/:id", warehouseAddressController.Get)
	e.POST("/api/warehouse-addresses", warehouseAddressController.Create)
	e.PUT("/api/warehouse-addresses/:id", warehouseAddressController.Update)
	e.DELETE("/api/warehouse-addresses/:id", warehouseAddressController.Delete)

	e.GET("/api/shipments/:id/declaration/preview", declarationController.Preview)
	e.POST("/api/shipments/:id/declaration/export", declarationController.Export)
	e.GET("/api/shipments/:id/declaration/records", declarationController.Records)

	// ========================================
	// 形式发票(PI)
	// ========================================

	piController := controllers.NewProformaInvoiceController(
		proformaInvoiceService,
		services.NewPiExcelService(settingsService),
	)

	e.GET("/proforma-invoices", func(c echo.Context) error {
		return c.Render(http.StatusOK, "proforma_invoices.html", map[string]any{
			"title": "形式发票(PI)",
		})
	})
	e.GET("/api/proforma-invoices", piController.List)
	e.POST("/api/proforma-invoices/generate", piController.Generate)
	e.GET("/api/proforma-invoices/:id", piController.Get)
	e.POST("/api/proforma-invoices", piController.Save)
	e.PUT("/api/proforma-invoices/:id", piController.Save)
	e.DELETE("/api/proforma-invoices/:id", piController.Delete)
	e.GET("/api/proforma-invoices/:id/export", piController.Export)
	e.POST("/api/proforma-invoices/:id/refresh-stock-note", piController.RefreshStockNote)

	// ========================================
	// 收款
	// ========================================

	paymentController := controllers.NewPaymentController(paymentService)

	e.GET("/payments", func(c echo.Context) error {
		return c.Render(http.StatusOK, "payments.html", map[string]any{
			"title": "收款管理",
		})
	})
	e.GET("/api/payments", paymentController.List)
	e.POST("/api/payments", paymentController.Create)
	e.PUT("/api/payments/:id", paymentController.Update)
	e.DELETE("/api/payments/:id", paymentController.Delete)
	e.GET("/api/payments/customers", paymentController.Customers)

	// ========================================
	// 数据统计
	// ========================================

	statisticsController := controllers.NewStatisticsController(statisticsService)

	e.GET("/statistics", func(c echo.Context) error {
		return c.Render(http.StatusOK, "statistics.html", map[string]any{
			"title": "数据统计",
		})
	})
	e.GET("/api/statistics/overview", statisticsController.Overview)

	// ========================================
	// 财务报表（收入 / 成本 / 利润）
	// ========================================

	financeController := controllers.NewFinanceController(financeService)

	e.GET("/finance", func(c echo.Context) error {
		return c.Render(http.StatusOK, "finance.html", map[string]any{
			"title": "财务报表",
		})
	})
	e.GET("/api/finance/report", financeController.Report)

	// ========================================
	// 系统设置
	// ========================================

	settingsController := controllers.NewSettingsController(settingsService, cfg)

	e.GET("/settings", func(c echo.Context) error {
		return c.Render(http.StatusOK, "settings.html", map[string]any{
			"title": "系统设置",
		})
	})
	e.GET("/api/settings", settingsController.List)
	e.PUT("/api/settings", settingsController.Save)
	e.DELETE("/api/settings/:key", settingsController.Delete)
	e.GET("/api/system-info", settingsController.SystemInfo)

	// ========================================
	// 操作日志
	// ========================================

	operationLogController := controllers.NewOperationLogController(operationLogService)

	e.GET("/operation-logs", func(c echo.Context) error {
		return c.Render(http.StatusOK, "operation_logs.html", map[string]any{
			"title": "操作日志",
		})
	})
	e.GET("/api/operation-logs", operationLogController.List)
	e.GET("/api/operation-logs/options", operationLogController.Options)
	e.GET("/api/operation-logs/export", operationLogController.Export)
	e.GET("/api/operation-logs/:id", operationLogController.Get)
	e.DELETE("/api/operation-logs", operationLogController.Cleanup)
}

// TemplateRenderer 渲染 templates 目录下的页面。
//
// 所有模板共用一个命名空间，如果多个页面同时定义 "content"，
// 后解析的会把先解析的覆盖掉，所以这里按页面分别解析：
// layoutPages 与 layout.html 组成一套模板，渲染 "layout"。
type TemplateRenderer struct {
	layouts    map[string]*template.Template
	standalone *template.Template
}

func NewTemplateRenderer(dir string) *TemplateRenderer {

	layouts := make(map[string]*template.Template, len(layoutPages))

	for _, page := range layoutPages {
		layouts[page] = template.Must(
			template.ParseFiles(
				filepath.Join(dir, "layout.html"),
				filepath.Join(dir, page),
			),
		)
	}

	standalone := template.New("standalone")

	for _, page := range standalonePages {
		standalone = template.Must(
			standalone.ParseFiles(
				filepath.Join(dir, page),
			),
		)
	}

	return &TemplateRenderer{
		layouts:    layouts,
		standalone: standalone,
	}
}

func (t *TemplateRenderer) Render(
	w io.Writer,
	name string,
	data interface{},
	c echo.Context,
) error {

	if page, ok := t.layouts[name]; ok {
		return page.ExecuteTemplate(
			w,
			"layout",
			data,
		)
	}

	return t.standalone.ExecuteTemplate(
		w,
		name,
		data,
	)
}
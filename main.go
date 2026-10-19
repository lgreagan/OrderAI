package main

import (
	"context"
	"log"

	"OrderAI/config"
	"OrderAI/database"
	"OrderAI/repository"
	"OrderAI/routes"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

func main() {

	// ========================================
	// 配置
	// ========================================

	cfg := config.Load()
	log.Printf(
		"数据库配置: host=%s port=%s user=%s db=%s",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBName,
	)
	// ========================================
	// MySQL
	// ========================================

	db, err := database.Connect(cfg)

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	log.Println("MySQL 数据库连接成功")

	// ========================================
	// Repository
	// ========================================

	orderRepo := repository.NewOrderRepository(db)
	skuRepo := repository.NewSKURepository(db)
	productMatchRepo := repository.NewProductMatchRepository(db)
	purchaseOrderRepo := repository.NewPurchaseOrderRepository(db)
	purchaseDemandRepo := repository.NewPurchaseDemandRepository(db)
	productionOrderRepo := repository.NewProductionOrderRepository(db)
	productionDemandRepo := repository.NewProductionDemandRepository(db)
	productBomRepo := repository.NewProductBomRepository(db)
	materialRepo := repository.NewMaterialRepository(db)
	packInfoRepo := repository.NewPackInfoRepository(db)
	inventoryRepo := repository.NewInventoryRepository(db)
	shipmentRepo := repository.NewShipmentRepository(db)
	proformaInvoiceRepo := repository.NewProformaInvoiceRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	settingRepo := repository.NewAppSettingRepository(db)
	statisticsRepo := repository.NewStatisticsRepository(db)
	financeRepo := repository.NewFinanceRepository(db)
	declarationRepo := repository.NewDeclarationRepository(db)
	productCategoryRepo := repository.NewProductCategoryRepository(db)
	warehouseAddressRepo := repository.NewWarehouseAddressRepository(db)
	shippingChannelRepo := repository.NewShippingChannelRepository(db)
	declarationTemplateRepo := repository.NewDeclarationTemplateRepository(db)
	operationLogRepo := repository.NewOperationLogRepository(db)
	userRepo := repository.NewUserRepository(db)

	// ========================================
	// Service
	// ========================================

	matchService := services.NewSKUMatchService(
		skuRepo,
		productMatchRepo,
	)

	orderService := services.NewOrderService(
		orderRepo,
		matchService,
	)
	skuService := services.NewSKUService(skuRepo)
	productMatchService := services.NewProductMatchService(
		productMatchRepo,
	)
	settingsService := services.NewSettingsService(
		settingRepo,
	)
	inventoryService := services.NewInventoryService(
		inventoryRepo,
		settingsService,
	)
	materialRequirementService := services.NewMaterialRequirementService(
		productBomRepo,
		materialRepo,
		packInfoRepo,
	)
	purchaseOrderService := services.NewPurchaseOrderService(
		purchaseOrderRepo,
		purchaseDemandRepo,
		inventoryService,
		materialRequirementService,
	)
	productionOrderService := services.NewProductionOrderService(
		productionOrderRepo,
		productionDemandRepo,
		materialRequirementService,
		inventoryService,
	)
	productBomService := services.NewProductBomService(
		productBomRepo,
	)
	materialService := services.NewMaterialService(
		materialRepo,
	)
	shipmentService := services.NewShipmentService(
		shipmentRepo,
		inventoryService,
		settingsService,
		materialRequirementService,
		purchaseDemandRepo,
	)
	proformaInvoiceService := services.NewProformaInvoiceService(
		proformaInvoiceRepo,
		shipmentRepo,
		orderRepo,
		purchaseOrderRepo,
		paymentRepo,
		inventoryRepo,
		skuRepo,
		materialRepo,
		settingsService,
	)
	paymentService := services.NewPaymentService(
		paymentRepo,
		proformaInvoiceRepo,
		settingsService,
	)
	statisticsService := services.NewStatisticsService(
		statisticsRepo,
		settingsService,
	)
	financeService := services.NewFinanceService(
		financeRepo,
		settingsService,
	)
	declarationService := services.NewDeclarationService(
		shipmentRepo,
		declarationRepo,
		declarationTemplateRepo,
		warehouseAddressRepo,
	)
	productCategoryService := services.NewProductCategoryService(productCategoryRepo)
	warehouseAddressService := services.NewWarehouseAddressService(warehouseAddressRepo)
	shippingChannelService := services.NewShippingChannelService(shippingChannelRepo)
	operationLogService := services.NewOperationLogService(operationLogRepo)
	authService := services.NewAuthService(userRepo)
	userService := services.NewUserService(userRepo)

	// 首次启动没有任何账号时创建管理员，避免装完进不去。
	if created, err := authService.EnsureDefaultAdmin(context.Background()); err != nil {
		log.Printf("初始化默认管理员失败: %v", err)
	} else if created {
		log.Printf(
			"已创建默认管理员：%s / %s（请登录后尽快修改密码）",
			services.DefaultAdminUsername, services.DefaultAdminPassword,
		)
	}

	// 退出时把队列里剩余的操作日志落库。
	defer operationLogService.Close()

	// ========================================
	// Echo
	// ========================================

	e := echo.New()

	routes.Register(
		e,
		orderService, skuService, productMatchService, matchService,
		purchaseOrderService, productionOrderService,
		materialService,
		inventoryService,
		shipmentService,
		proformaInvoiceService,
		productBomService,
		paymentService,
		settingsService,
		statisticsService,
		financeService,
		declarationService,
		productCategoryService,
		warehouseAddressService,
		shippingChannelService,
		operationLogService,
		authService,
		userService,
	)

	port := cfg.AppPort
	if port == "" {
		port = "8080"
	}

	e.Logger.Fatal(
		e.Start(":" + port),
	)
}

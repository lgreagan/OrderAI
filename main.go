package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"OrderAI/config"
	"OrderAI/database"
	"OrderAI/license"
	"OrderAI/repository"
	"OrderAI/routes"
	"OrderAI/services"

	"github.com/labstack/echo/v4"
)

// ========================================
// 授权（绑定 CPU + 主网卡 MAC）
// ========================================

// licenseSecret 签名密钥。改动它 = 所有已发授权码立即失效。
const licenseSecret = "OrderAI!2026@根#license$7f3a9c"

// licenseCode 本机授权码：由 licenseSecret + 本机 CPU ID + 主网卡 MAC 签名得出。
// 换服务器 / 换网卡：先跑 OrderAI.exe -lic 打印新授权码，替换这里再重新编译。
const licenseCode = "ORDAI1-PDJJ-Z27F-4109-9C53"

func main() {

	// ========================================
	// 授权校验（必须在连数据库之前）
	// ========================================

	printLic := flag.Bool("lic", false, "打印本机指纹与本机授权码后退出")
	flag.Parse()

	if *printLic {
		printLicenseInfo()
		return
	}

	if err := license.Verify(licenseSecret, licenseCode); err != nil {
		log.Println("========================================")
		log.Println("授权校验失败，程序停止启动")
		var mm *license.MismatchError
		if errors.As(err, &mm) {
			log.Printf("本机指纹: CPU=%s  MAC=%s", mm.CPU, mm.MAC)
			log.Println("原因：这台机器未授权，或更换了 CPU / 网卡")
		} else {
			log.Printf("原因: %v", err)
		}
		log.Println("获取授权码：执行  OrderAI.exe -lic  会打印本机授权码，")
		log.Println("替换 main.go 里的 licenseCode 后重新编译；或把上面的指纹发给管理员出码。")
		log.Println("========================================")
		os.Exit(1)
	}
	log.Printf("授权校验通过（%s）", license.MachineLine())

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

// printLicenseInfo 打印本机指纹与应填授权码，供换机 / 首次部署时使用。
func printLicenseInfo() {
	cpu, mac, err := license.Fingerprint()
	if err != nil {
		log.Fatal("读取机器指纹失败: ", err)
	}
	code := license.Code(licenseSecret, cpu, mac)

	fmt.Println()
	fmt.Println("========== OrderAI 授权信息 ==========")
	fmt.Println("CPU     :", cpu)
	fmt.Println("MAC     :", mac)
	fmt.Println("授权码  :", code)
	fmt.Println()
	fmt.Println("其它网卡（主网卡换/停用时从这里挑一个重新出码）：")
	for _, m := range license.AllMACs() {
		fmt.Println("  ", m)
	}
	fmt.Println()
	fmt.Println("填到 main.go：")
	fmt.Printf("const licenseCode = %q\n", code)
	fmt.Println("=====================================")
	fmt.Println()
}

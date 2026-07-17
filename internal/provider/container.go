package provider

import (
	"github.com/dujiao-next/internal/authz"
	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	paymentprovider "github.com/dujiao-next/internal/payment/provider"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"
)

// Container 依赖注入容器
type Container struct {
	Config      *config.Config
	QueueClient *queue.Client

	// Repositories
	AdminRepo                  repository.AdminRepository
	UserRepo                   repository.UserRepository
	UserOAuthIdentityRepo      repository.UserOAuthIdentityRepository
	EmailVerifyCodeRepo        repository.EmailVerifyCodeRepository
	OrderRepo                  repository.OrderRepository
	PaymentRepo                repository.PaymentRepository
	PaymentChannelRepo         repository.PaymentChannelRepository
	CardSecretRepo             repository.CardSecretRepository
	CardSecretBatchRepo        repository.CardSecretBatchRepository
	GiftCardRepo               repository.GiftCardRepository
	FulfillmentRepo            repository.FulfillmentRepository
	ProductRepo                repository.ProductRepository
	ProductSKURepo             repository.ProductSKURepository
	CartRepo                   repository.CartRepository
	CouponRepo                 repository.CouponRepository
	CouponUsageRepo            repository.CouponUsageRepository
	PromotionRepo              repository.PromotionRepository
	WalletRepo                 repository.WalletRepository
	OrderRefundRecordRepo      repository.OrderRefundRecordRepository
	PostRepo                   repository.PostRepository
	CategoryRepo               repository.CategoryRepository
	BannerRepo                 repository.BannerRepository
	SettingRepo                repository.SettingRepository
	UserLoginLogRepo           repository.UserLoginLogRepository
	AuthzAuditLogRepo          repository.AuthzAuditLogRepository
	NotificationLogRepo        repository.NotificationLogRepository
	AdminLoginLogRepo          repository.AdminLoginLogRepository
	DashboardRepo              repository.DashboardRepository
	AffiliateRepo              repository.AffiliateRepository
	ResellerRepo               repository.ResellerRepository
	ResellerProductSettingRepo repository.ResellerProductSettingRepository
	ResellerOperationsRepo     repository.ResellerOperationsRepository
	ApiCredentialRepo          repository.ApiCredentialRepository
	SiteConnectionRepo         repository.SiteConnectionRepository
	ProductMappingRepo         repository.ProductMappingRepository
	SKUMappingRepo             repository.SKUMappingRepository
	ProcurementOrderRepo       repository.ProcurementOrderRepository
	DownstreamOrderRefRepo     repository.DownstreamOrderRefRepository
	ReconciliationJobRepo      repository.ReconciliationJobRepository
	ReconciliationItemRepo     repository.ReconciliationItemRepository
	ChannelClientRepo          repository.ChannelClientRepository
	TelegramBroadcastRepo      repository.TelegramBroadcastRepository
	MemberLevelRepo            repository.MemberLevelRepository
	MemberLevelPriceRepo       repository.MemberLevelPriceRepository
	MediaRepo                  repository.MediaRepository
	PostCategoryRepo           repository.PostCategoryRepository

	// Services
	AuthzService                  *authz.Service
	AuthService                   *service.AuthService
	TOTPService                   *service.TOTPService
	UserTOTPService               *service.UserTOTPService
	UserAuthService               *service.UserAuthService
	TelegramAuthService           *service.TelegramAuthService
	EmailService                  *service.EmailService
	CaptchaService                *service.CaptchaService
	UploadService                 *service.UploadService
	ProductService                *service.ProductService
	PostService                   *service.PostService
	CategoryService               *service.CategoryService
	SettingService                *service.SettingService
	SitemapService                *service.SitemapService
	CartService                   *service.CartService
	WalletService                 *service.WalletService
	OrderRefundService            *service.OrderRefundService
	OrderService                  *service.OrderService
	FulfillmentService            *service.FulfillmentService
	CouponAdminService            *service.CouponAdminService
	PromotionAdminService         *service.PromotionAdminService
	BannerService                 *service.BannerService
	PaymentService                *service.PaymentService
	CardSecretService             *service.CardSecretService
	GiftCardService               *service.GiftCardService
	UserLoginLogService           *service.UserLoginLogService
	AuthzAuditService             *service.AuthzAuditService
	NotificationLogService        *service.NotificationLogService
	DashboardService              *service.DashboardService
	NotificationService           *service.NotificationService
	AffiliateService              *service.AffiliateService
	ResellerDomainResolver        *service.ResellerDomainResolver
	ResellerPricingResolver       *service.ResellerPricingResolver
	ResellerManagementService     *service.ResellerManagementService
	ResellerSiteConfigService     *service.ResellerSiteConfigService
	ResellerProductSettingService *service.ResellerProductSettingService
	ResellerAccountingService     *service.ResellerAccountingService
	ResellerOrderService          *service.ResellerOrderService
	ResellerOperationsService     *service.ResellerOperationsService
	ApiCredentialService          *service.ApiCredentialService
	SiteConnectionService         *service.SiteConnectionService
	ProductMappingService         *service.ProductMappingService
	ProcurementOrderService       *service.ProcurementOrderService
	DownstreamCallbackService     *service.DownstreamCallbackService
	ReconciliationService         *service.ReconciliationService
	ChannelClientService          *service.ChannelClientService
	TelegramBroadcastService      *service.TelegramBroadcastService
	MemberLevelService            *service.MemberLevelService
	AdProxyService                *service.AdProxyService
	MediaService                  *service.MediaService
	PostCategoryService           *service.PostCategoryService
	OrderRiskControlService       *service.OrderRiskControlService
	ComplianceService             *service.ComplianceService

	// 支付网关 Provider 注册表(P1.2 Phase 1 引入,pilot 阶段仅注册 stripe + paypal)
	PaymentProviderRegistry *paymentprovider.Registry
}

// NewContainer 初始化容器
func NewContainer(cfg *config.Config) *Container {
	// 初始化缓存
	if err := cache.InitRedis(&cfg.Redis); err != nil {
		logger.Warnw("provider_init_redis_failed", "error", err)
	}

	// 初始化队列客户端
	var queueClient *queue.Client
	if cfg.Queue.Enabled {
		qc, err := queue.NewClient(&cfg.Queue)
		if err != nil {
			logger.Errorw("provider_init_queue_client_failed", "error", err)
		} else {
			queueClient = qc
		}
	}

	// 实例化支付 provider 注册表并注册所有 9 个 adapter。
	// P1.2a 注册 pilot: stripe + paypal
	// P1.2b Task 2-8 完成的 7 个 new adapter wrapper: wechat/alipay/epay/epusdt/bepusdt/tokenpay/okpay
	// 必须在 initServices() 之前完成,以便 PaymentService 构造时可注入。
	// 所有 adapter 注册完成后,旧 switch case 即可整体移除(Task 10)。
	paymentRegistry := paymentprovider.NewRegistry()
	paymentRegistry.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeStripe, paymentprovider.NewStripeAdapter())
	paymentRegistry.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypePaypal, paymentprovider.NewPaypalAdapter())
	// P1.2b Task 2-8: 新增 7 个 adapter (wechat/alipay/epay/epusdt/bepusdt/tokenpay/okpay)
	paymentRegistry.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeWechat, paymentprovider.NewWechatpayAdapter())
	paymentRegistry.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeAlipay, paymentprovider.NewAlipayAdapter())
	paymentRegistry.Register(constants.PaymentProviderEpay, "", paymentprovider.NewEpayAdapter())
	paymentRegistry.Register(constants.PaymentProviderEpusdt, "", paymentprovider.NewEpusdtAdapter())
	paymentRegistry.Register(constants.PaymentProviderBepusdt, "", paymentprovider.NewBepusdtAdapter())
	paymentRegistry.Register(constants.PaymentProviderDujiaoPay, "", paymentprovider.NewDujiaoPayAdapter())
	paymentRegistry.Register(constants.PaymentProviderTokenpay, "", paymentprovider.NewTokenpayAdapter())
	paymentRegistry.Register(constants.PaymentProviderOkpay, "", paymentprovider.NewOkpayAdapter())

	c := &Container{
		Config:                  cfg,
		QueueClient:             queueClient,
		PaymentProviderRegistry: paymentRegistry,
	}

	// 1. 初始化 Repositories
	c.initRepositories()

	// 2. 初始化 Services
	c.initServices()

	return c
}

func (c *Container) initRepositories() {
	db := models.DB
	c.AdminRepo = repository.NewAdminRepository(db)
	c.UserRepo = repository.NewUserRepository(db)
	c.UserOAuthIdentityRepo = repository.NewUserOAuthIdentityRepository(db)
	c.EmailVerifyCodeRepo = repository.NewEmailVerifyCodeRepository(db)
	c.OrderRepo = repository.NewOrderRepository(db)
	c.PaymentRepo = repository.NewPaymentRepository(db)
	c.PaymentChannelRepo = repository.NewPaymentChannelRepository(db)
	c.CardSecretRepo = repository.NewCardSecretRepository(db)
	c.CardSecretBatchRepo = repository.NewCardSecretBatchRepository(db)
	c.GiftCardRepo = repository.NewGiftCardRepository(db)
	c.FulfillmentRepo = repository.NewFulfillmentRepository(db)
	c.ProductRepo = repository.NewProductRepository(db)
	c.ProductSKURepo = repository.NewProductSKURepository(db)
	c.CartRepo = repository.NewCartRepository(db)
	c.CouponRepo = repository.NewCouponRepository(db)
	c.CouponUsageRepo = repository.NewCouponUsageRepository(db)
	c.PromotionRepo = repository.NewPromotionRepository(db)
	c.WalletRepo = repository.NewWalletRepository(db)
	c.OrderRefundRecordRepo = repository.NewOrderRefundRecordRepository(db)
	c.PostRepo = repository.NewPostRepository(db)
	c.CategoryRepo = repository.NewCategoryRepository(db)
	c.BannerRepo = repository.NewBannerRepository(db)
	c.SettingRepo = repository.NewSettingRepository(db)
	c.UserLoginLogRepo = repository.NewUserLoginLogRepository(db)
	c.AuthzAuditLogRepo = repository.NewAuthzAuditLogRepository(db)
	c.NotificationLogRepo = repository.NewNotificationLogRepository(db)
	c.AdminLoginLogRepo = repository.NewAdminLoginLogRepository(db)
	c.DashboardRepo = repository.NewDashboardRepository(db)
	c.AffiliateRepo = repository.NewAffiliateRepository(db)
	c.ResellerRepo = repository.NewResellerRepository(db)
	c.ResellerProductSettingRepo = repository.NewResellerProductSettingRepository(db)
	c.ResellerOperationsRepo = repository.NewResellerOperationsRepository(db)
	c.ApiCredentialRepo = repository.NewApiCredentialRepository(db)
	c.SiteConnectionRepo = repository.NewSiteConnectionRepository(db)
	c.ProductMappingRepo = repository.NewProductMappingRepository(db)
	c.SKUMappingRepo = repository.NewSKUMappingRepository(db)
	c.ProcurementOrderRepo = repository.NewProcurementOrderRepository(db)
	c.DownstreamOrderRefRepo = repository.NewDownstreamOrderRefRepository(db)
	c.ReconciliationJobRepo = repository.NewReconciliationJobRepository(db)
	c.ReconciliationItemRepo = repository.NewReconciliationItemRepository(db)
	c.ChannelClientRepo = repository.NewChannelClientRepository(db)
	c.TelegramBroadcastRepo = repository.NewTelegramBroadcastRepository(db)
	c.MemberLevelRepo = repository.NewMemberLevelRepository(db)
	c.MemberLevelPriceRepo = repository.NewMemberLevelPriceRepository(db)
	c.MediaRepo = repository.NewMediaRepository(db)
	c.PostCategoryRepo = repository.NewPostCategoryRepository(db)
}

func (c *Container) initServices() {
	authzService, err := authz.NewService(models.DB)
	if err != nil {
		logger.Errorw("provider_init_authz_failed", "error", err)
		panic(err)
	}
	c.AuthzService = authzService
	if err := c.AuthzService.BootstrapBuiltinRoles(); err != nil {
		logger.Errorw("provider_bootstrap_builtin_roles_failed", "error", err)
		panic(err)
	}

	c.SettingService = service.NewSettingService(c.SettingRepo, c.Config.Order)
	c.ResellerDomainResolver = service.NewResellerDomainResolver(c.ResellerRepo, c.Config.Reseller)
	c.ResellerPricingResolver = service.NewResellerPricingResolver(c.ResellerRepo)
	c.ResellerManagementService = service.NewResellerManagementService(c.ResellerRepo, c.Config.Reseller)
	c.ResellerSiteConfigService = service.NewResellerSiteConfigService(c.ResellerRepo)
	c.ResellerProductSettingService = service.NewResellerProductSettingService(c.ResellerProductSettingRepo, c.ResellerRepo, c.ProductRepo)
	c.ResellerAccountingService = service.NewResellerAccountingService(c.ResellerRepo, service.ResellerAccountingOptions{
		ConfirmDays: c.Config.Reseller.SettlementConfirmDays,
	})
	c.ResellerOrderService = service.NewResellerOrderService(c.ResellerRepo)
	c.ResellerOperationsService = service.NewResellerOperationsService(c.ResellerOperationsRepo)
	c.ComplianceService = service.NewComplianceService(c.SettingRepo)
	smtpSetting, err := c.SettingService.GetSMTPSetting(c.Config.Email)
	if err != nil {
		logger.Warnw("provider_load_smtp_setting_failed", "error", err)
	} else {
		c.Config.Email = service.SMTPSettingToConfig(smtpSetting)
	}

	captchaSetting, err := c.SettingService.GetCaptchaSetting(c.Config.Captcha)
	if err != nil {
		logger.Warnw("provider_load_captcha_setting_failed", "error", err)
	} else {
		c.Config.Captcha = service.CaptchaSettingToConfig(captchaSetting)
	}

	telegramAuthSetting, err := c.SettingService.GetTelegramAuthSetting(c.Config.TelegramAuth)
	if err != nil {
		logger.Warnw("provider_load_telegram_auth_setting_failed", "error", err)
	} else {
		c.Config.TelegramAuth = service.TelegramAuthSettingToConfig(telegramAuthSetting)
	}

	c.EmailService = service.NewEmailService(&c.Config.Email)
	c.CaptchaService = service.NewCaptchaService(c.SettingService, c.Config.Captcha)
	c.AuthService = service.NewAuthService(c.Config, c.AdminRepo)
	c.TOTPService = service.NewTOTPService(c.Config, c.AdminRepo, cache.Client())
	c.UserTOTPService = service.NewUserTOTPService(c.Config, c.UserRepo, cache.Client())
	c.TelegramAuthService = service.NewTelegramAuthService(c.Config.TelegramAuth)
	c.UserAuthService = service.NewUserAuthService(c.Config, c.UserRepo, c.UserOAuthIdentityRepo, c.EmailVerifyCodeRepo, c.SettingService, c.EmailService, c.TelegramAuthService)
	c.UploadService = service.NewUploadService(c.Config)
	c.AffiliateService = service.NewAffiliateService(c.AffiliateRepo, c.UserRepo, c.OrderRepo, c.ProductRepo, c.SettingService)
	c.ProductService = service.NewProductService(c.ProductRepo, c.ProductSKURepo, c.CardSecretRepo, c.CardSecretBatchRepo, c.CategoryRepo, c.MemberLevelPriceRepo, c.CartRepo, c.ProductMappingRepo, c.OrderRepo, c.PaymentChannelRepo)
	c.PostService = service.NewPostService(c.PostRepo, c.PostCategoryRepo)
	c.PostCategoryService = service.NewPostCategoryService(c.PostCategoryRepo)
	c.CategoryService = service.NewCategoryService(c.CategoryRepo)
	c.SitemapService = service.NewSitemapService(c.ProductRepo, c.CategoryRepo, c.PostRepo)
	c.CartService = service.NewCartService(c.CartRepo, c.ProductRepo, c.ProductSKURepo, c.PromotionRepo, c.SettingService)
	c.WalletService = service.NewWalletService(c.WalletRepo, c.OrderRepo, c.OrderRefundRecordRepo, c.UserRepo, c.AffiliateService, c.SettingService)
	c.OrderRefundService = service.NewOrderRefundService(c.OrderRepo, c.UserRepo, c.OrderRefundRecordRepo, c.AffiliateService, c.SettingService)
	c.MemberLevelService = service.NewMemberLevelService(c.MemberLevelRepo, c.MemberLevelPriceRepo, c.UserRepo)
	c.OrderRiskControlService = service.NewOrderRiskControlService(c.SettingService, c.OrderRepo)
	c.OrderService = service.NewOrderService(service.OrderServiceOptions{
		OrderRepo:                 c.OrderRepo,
		OrderRefundRecordRepo:     c.OrderRefundRecordRepo,
		PaymentRepo:               c.PaymentRepo,
		UserRepo:                  c.UserRepo,
		ProductRepo:               c.ProductRepo,
		ProductSKURepo:            c.ProductSKURepo,
		CardSecretRepo:            c.CardSecretRepo,
		ResellerRepo:              c.ResellerRepo,
		CouponRepo:                c.CouponRepo,
		CouponUsageRepo:           c.CouponUsageRepo,
		PromotionRepo:             c.PromotionRepo,
		QueueClient:               c.QueueClient,
		SettingService:            c.SettingService,
		DefaultEmailConfig:        c.Config.Email,
		WalletService:             c.WalletService,
		AffiliateService:          c.AffiliateService,
		MemberLevelService:        c.MemberLevelService,
		ResellerPricingResolver:   c.ResellerPricingResolver,
		ResellerAccountingService: c.ResellerAccountingService,
		RiskControlService:        c.OrderRiskControlService,
		ExpireMinutes:             c.Config.Order.PaymentExpireMinutes,
	})
	c.FulfillmentService = service.NewFulfillmentService(
		c.OrderRepo, c.FulfillmentRepo, c.CardSecretRepo, c.QueueClient,
		c.SettingService, c.Config.Email,
		c.UserOAuthIdentityRepo,
	)
	c.CardSecretService = service.NewCardSecretService(c.CardSecretRepo, c.CardSecretBatchRepo, c.ProductRepo, c.ProductSKURepo)
	c.GiftCardService = service.NewGiftCardService(c.GiftCardRepo, c.UserRepo, c.WalletService, c.SettingService)
	c.CouponAdminService = service.NewCouponAdminService(c.CouponRepo)
	c.PromotionAdminService = service.NewPromotionAdminService(c.PromotionRepo)
	c.BannerService = service.NewBannerService(c.BannerRepo)
	c.UserLoginLogService = service.NewUserLoginLogService(c.UserLoginLogRepo)
	c.AuthzAuditService = service.NewAuthzAuditService(c.AuthzAuditLogRepo)
	c.NotificationLogService = service.NewNotificationLogService(c.NotificationLogRepo)
	c.DashboardService = service.NewDashboardService(c.DashboardRepo, c.SettingService)
	c.NotificationService = service.NewNotificationService(c.SettingService, c.EmailService, c.QueueClient, c.DashboardService, c.NotificationLogService, c.Config.TelegramAuth)
	c.ApiCredentialService = service.NewApiCredentialService(c.ApiCredentialRepo)
	c.SiteConnectionService = service.NewSiteConnectionService(c.SiteConnectionRepo, c.Config.App.SecretKey, "uploads")
	c.ProductMappingService = service.NewProductMappingService(c.ProductMappingRepo, c.SKUMappingRepo, c.ProductRepo, c.ProductSKURepo, c.CategoryRepo, c.SiteConnectionService)
	c.ProductMappingService.SetCategoryService(c.CategoryService)
	c.ProductMappingService.SetSettingService(c.SettingService)
	c.SiteConnectionService.SetMarkupReapplier(c.ProductMappingService)
	c.OrderService.SetProductMappingService(c.ProductMappingService)
	c.DownstreamCallbackService = service.NewDownstreamCallbackService(c.DownstreamOrderRefRepo, c.OrderRepo, c.ApiCredentialRepo, c.QueueClient)
	c.PaymentService = service.NewPaymentService(service.PaymentServiceOptions{
		OrderRepo:                 c.OrderRepo,
		ProductRepo:               c.ProductRepo,
		ProductSKURepo:            c.ProductSKURepo,
		PaymentRepo:               c.PaymentRepo,
		ChannelRepo:               c.PaymentChannelRepo,
		WalletRepo:                c.WalletRepo,
		UserRepo:                  c.UserRepo,
		UserOAuthIdentityRepo:     c.UserOAuthIdentityRepo,
		QueueClient:               c.QueueClient,
		WalletService:             c.WalletService,
		SettingService:            c.SettingService,
		DefaultEmailConfig:        c.Config.Email,
		ExpireMinutes:             c.Config.Order.PaymentExpireMinutes,
		AffiliateService:          c.AffiliateService,
		NotificationService:       c.NotificationService,
		PaymentProviderRegistry:   c.PaymentProviderRegistry,
		ResellerAccountingService: c.ResellerAccountingService,
	})
	c.ProcurementOrderService = service.NewProcurementOrderService(
		c.ProcurementOrderRepo, c.OrderRepo, c.FulfillmentRepo, c.ProductMappingRepo, c.SKUMappingRepo,
		c.SiteConnectionService, c.QueueClient, c.SettingService, c.Config.Email, c.FulfillmentService,
	)
	c.ReconciliationService = service.NewReconciliationService(
		c.ReconciliationJobRepo, c.ReconciliationItemRepo, c.ProcurementOrderRepo,
		c.SiteConnectionService, c.QueueClient, c.NotificationService,
	)
	c.ChannelClientService = service.NewChannelClientService(c.ChannelClientRepo, c.Config.App.SecretKey)
	c.TelegramBroadcastService = service.NewTelegramBroadcastService(
		c.TelegramBroadcastRepo,
		c.UserOAuthIdentityRepo,
		c.ChannelClientRepo,
		c.ChannelClientService,
		c.QueueClient,
		service.NewTelegramNotifyService(c.SettingService, c.Config.TelegramAuth),
	)
	c.UserAuthService.SetMemberLevelService(c.MemberLevelService)
	c.WalletService.SetResellerAccountingService(c.ResellerAccountingService)
	c.OrderRefundService.SetResellerAccountingService(c.ResellerAccountingService)
	c.PaymentService.SetMemberLevelService(c.MemberLevelService)
	c.PaymentService.SetProcurementService(c.ProcurementOrderService)
	c.PaymentService.SetDownstreamCallbackService(c.DownstreamCallbackService)
	c.FulfillmentService.SetDownstreamCallbackService(c.DownstreamCallbackService)
	c.ProcurementOrderService.SetDownstreamCallbackService(c.DownstreamCallbackService)
	c.ProcurementOrderService.SetNotificationService(c.NotificationService)
	c.MediaService = service.NewMediaService(c.MediaRepo)
	c.ProductMappingService.SetMediaService(c.MediaService)
	c.AdProxyService = service.NewAdProxyService()
}

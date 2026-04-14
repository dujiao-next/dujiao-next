package provider

import (
	"github.com/dujiao-next/internal/authz"
	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"
)

// Container 依赖注入容器
type Container struct {
	Config      *config.Config
	QueueClient *queue.Client

	// Repositories
	AdminRepo                 repository.AdminRepository
	UserRepo                  repository.UserRepository
	UserOAuthIdentityRepo     repository.UserOAuthIdentityRepository
	EmailVerifyCodeRepo       repository.EmailVerifyCodeRepository
	OrderRepo                 repository.OrderRepository
	PaymentRepo               repository.PaymentRepository
	PaymentChannelRepo        repository.PaymentChannelRepository
	CardSecretRepo            repository.CardSecretRepository
	CardSecretBatchRepo       repository.CardSecretBatchRepository
	GiftCardRepo              repository.GiftCardRepository
	FulfillmentRepo           repository.FulfillmentRepository
	ProductRepo               repository.ProductRepository
	ProductSKURepo            repository.ProductSKURepository
	CartRepo                  repository.CartRepository
	CouponRepo                repository.CouponRepository
	CouponUsageRepo           repository.CouponUsageRepository
	PromotionRepo             repository.PromotionRepository
	WalletRepo                repository.WalletRepository
	OrderRefundRecordRepo     repository.OrderRefundRecordRepository
	PostRepo                  repository.PostRepository
	CategoryRepo              repository.CategoryRepository
	BannerRepo                repository.BannerRepository
	SettingRepo               repository.SettingRepository
	UserLoginLogRepo          repository.UserLoginLogRepository
	AuthzAuditLogRepo         repository.AuthzAuditLogRepository
	NotificationLogRepo       repository.NotificationLogRepository
	DashboardRepo             repository.DashboardRepository
	AffiliateRepo             repository.AffiliateRepository
	ApiCredentialRepo         repository.ApiCredentialRepository
	SiteConnectionRepo        repository.SiteConnectionRepository
	ProductMappingRepo        repository.ProductMappingRepository
	SKUMappingRepo            repository.SKUMappingRepository
	ProcurementOrderRepo      repository.ProcurementOrderRepository
	DownstreamOrderRefRepo    repository.DownstreamOrderRefRepository
	ReconciliationJobRepo     repository.ReconciliationJobRepository
	ReconciliationItemRepo    repository.ReconciliationItemRepository
	ChannelClientRepo         repository.ChannelClientRepository
	TelegramBroadcastRepo     repository.TelegramBroadcastRepository
	MemberLevelRepo           repository.MemberLevelRepository
	MemberLevelPriceRepo      repository.MemberLevelPriceRepository
	MediaRepo                 repository.MediaRepository
	PluginRepo                repository.PluginRepository
	PluginMarketPlatformRepo  repository.PluginMarketPlatformRepository
	PluginLicensePlatformRepo repository.PluginLicensePlatformRepository

	// Services
	AuthzService               *authz.Service
	AuthService                *service.AuthService
	UserAuthService            *service.UserAuthService
	TelegramAuthService        *service.TelegramAuthService
	EmailService               *service.EmailService
	CaptchaService             *service.CaptchaService
	UploadService              *service.UploadService
	ProductService             *service.ProductService
	PostService                *service.PostService
	CategoryService            *service.CategoryService
	SettingService             *service.SettingService
	CartService                *service.CartService
	WalletService              *service.WalletService
	OrderRefundService         *service.OrderRefundService
	OrderService               *service.OrderService
	FulfillmentService         *service.FulfillmentService
	CouponAdminService         *service.CouponAdminService
	PromotionAdminService      *service.PromotionAdminService
	BannerService              *service.BannerService
	PaymentService             *service.PaymentService
	CardSecretService          *service.CardSecretService
	GiftCardService            *service.GiftCardService
	UserLoginLogService        *service.UserLoginLogService
	AuthzAuditService          *service.AuthzAuditService
	NotificationLogService     *service.NotificationLogService
	DashboardService           *service.DashboardService
	NotificationService        *service.NotificationService
	AffiliateService           *service.AffiliateService
	ApiCredentialService       *service.ApiCredentialService
	SiteConnectionService      *service.SiteConnectionService
	ProductMappingService      *service.ProductMappingService
	ProcurementOrderService    *service.ProcurementOrderService
	DownstreamCallbackService  *service.DownstreamCallbackService
	ReconciliationService      *service.ReconciliationService
	ChannelClientService       *service.ChannelClientService
	TelegramBroadcastService   *service.TelegramBroadcastService
	MemberLevelService         *service.MemberLevelService
	AdProxyService             *service.AdProxyService
	MediaService               *service.MediaService
	OrderRiskControlService    *service.OrderRiskControlService
	PluginCenterService        *service.PluginCenterService
	OnlinePluginCenterService  *service.OnlinePluginCenterService
	OnlinePluginLicenseService *service.OnlinePluginLicenseService
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

	c := &Container{
		Config:      cfg,
		QueueClient: queueClient,
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
	c.DashboardRepo = repository.NewDashboardRepository(db)
	c.AffiliateRepo = repository.NewAffiliateRepository(db)
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
	c.PluginRepo = repository.NewPluginRepository(db)
	c.PluginMarketPlatformRepo = repository.NewPluginMarketPlatformRepository(db)
	c.PluginLicensePlatformRepo = repository.NewPluginLicensePlatformRepository(db)
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

	c.SettingService = service.NewSettingService(c.SettingRepo)
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
	c.TelegramAuthService = service.NewTelegramAuthService(c.Config.TelegramAuth)
	c.UserAuthService = service.NewUserAuthService(c.Config, c.UserRepo, c.UserOAuthIdentityRepo, c.EmailVerifyCodeRepo, c.SettingService, c.EmailService, c.TelegramAuthService)
	c.UploadService = service.NewUploadService(c.Config)
	c.AffiliateService = service.NewAffiliateService(c.AffiliateRepo, c.UserRepo, c.OrderRepo, c.ProductRepo, c.SettingService)
	c.ProductService = service.NewProductService(c.ProductRepo, c.ProductSKURepo, c.CardSecretRepo, c.CardSecretBatchRepo, c.CategoryRepo, c.MemberLevelPriceRepo, c.CartRepo, c.ProductMappingRepo, c.OrderRepo)
	c.PostService = service.NewPostService(c.PostRepo)
	c.CategoryService = service.NewCategoryService(c.CategoryRepo)
	c.CartService = service.NewCartService(c.CartRepo, c.ProductRepo, c.ProductSKURepo, c.PromotionRepo, c.SettingService)
	c.WalletService = service.NewWalletService(c.WalletRepo, c.OrderRepo, c.UserRepo, c.AffiliateService)
	c.OrderRefundService = service.NewOrderRefundService(c.OrderRepo, c.UserRepo, c.OrderRefundRecordRepo, c.AffiliateService)
	c.MemberLevelService = service.NewMemberLevelService(c.MemberLevelRepo, c.MemberLevelPriceRepo, c.UserRepo)
	c.OrderRiskControlService = service.NewOrderRiskControlService(c.SettingService, c.OrderRepo)
	c.OnlinePluginCenterService = service.NewOnlinePluginCenterService(c.PluginMarketPlatformRepo)
	c.OnlinePluginLicenseService = service.NewOnlinePluginLicenseService(c.PluginLicensePlatformRepo)
	c.OrderService = service.NewOrderService(service.OrderServiceOptions{
		OrderRepo:             c.OrderRepo,
		OrderRefundRecordRepo: c.OrderRefundRecordRepo,
		UserRepo:              c.UserRepo,
		ProductRepo:           c.ProductRepo,
		ProductSKURepo:        c.ProductSKURepo,
		CardSecretRepo:        c.CardSecretRepo,
		CouponRepo:            c.CouponRepo,
		CouponUsageRepo:       c.CouponUsageRepo,
		PromotionRepo:         c.PromotionRepo,
		QueueClient:           c.QueueClient,
		SettingService:        c.SettingService,
		DefaultEmailConfig:    c.Config.Email,
		WalletService:         c.WalletService,
		AffiliateService:      c.AffiliateService,
		MemberLevelService:    c.MemberLevelService,
		RiskControlService:    c.OrderRiskControlService,
		ExpireMinutes:         c.Config.Order.PaymentExpireMinutes,
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
	c.DownstreamCallbackService = service.NewDownstreamCallbackService(c.DownstreamOrderRefRepo, c.OrderRepo, c.ApiCredentialRepo, c.QueueClient)
	c.PaymentService = service.NewPaymentService(service.PaymentServiceOptions{
		OrderRepo:             c.OrderRepo,
		ProductRepo:           c.ProductRepo,
		ProductSKURepo:        c.ProductSKURepo,
		PaymentRepo:           c.PaymentRepo,
		ChannelRepo:           c.PaymentChannelRepo,
		WalletRepo:            c.WalletRepo,
		UserRepo:              c.UserRepo,
		UserOAuthIdentityRepo: c.UserOAuthIdentityRepo,
		QueueClient:           c.QueueClient,
		WalletService:         c.WalletService,
		SettingService:        c.SettingService,
		DefaultEmailConfig:    c.Config.Email,
		ExpireMinutes:         c.Config.Order.PaymentExpireMinutes,
		AffiliateService:      c.AffiliateService,
		NotificationService:   c.NotificationService,
	})
	c.ProcurementOrderService = service.NewProcurementOrderService(
		c.ProcurementOrderRepo, c.OrderRepo, c.ProductMappingRepo, c.SKUMappingRepo,
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
	c.PaymentService.SetMemberLevelService(c.MemberLevelService)
	c.PaymentService.SetProcurementService(c.ProcurementOrderService)
	c.PaymentService.SetDownstreamCallbackService(c.DownstreamCallbackService)
	c.FulfillmentService.SetDownstreamCallbackService(c.DownstreamCallbackService)
	c.ProcurementOrderService.SetDownstreamCallbackService(c.DownstreamCallbackService)
	c.ProcurementOrderService.SetNotificationService(c.NotificationService)
	c.MediaService = service.NewMediaService(c.MediaRepo)
	c.ProductMappingService.SetMediaService(c.MediaService)
	c.AdProxyService = service.NewAdProxyService()
	c.PluginCenterService = service.NewPluginCenterService(c.PluginRepo, c.SettingService)
	if c.OnlinePluginCenterService != nil {
		c.PluginCenterService.SetBuiltinFeedProvider("official", func() (*service.MarketFeed, error) {
			return c.OnlinePluginCenterService.BuildPublicFeed("official")
		})
		c.PluginCenterService.SetBuiltinFeedProvider("public", func() (*service.MarketFeed, error) {
			return c.OnlinePluginCenterService.BuildPublicFeed("public")
		})
	}
	c.PluginCenterService.SetHostExtension("main_gorm_db", models.DB)
	c.PluginCenterService.SetHostExtension("main_db_driver", models.DB.Dialector.Name())
	if err := c.PluginCenterService.ReloadRuntime(); err != nil {
		logger.Warnw("provider_reload_plugin_runtime_failed", "error", err)
	}
}

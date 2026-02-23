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
	AdminRepo             repository.AdminRepository
	UserRepo              repository.UserRepository
	UserOAuthIdentityRepo repository.UserOAuthIdentityRepository
	EmailVerifyCodeRepo   repository.EmailVerifyCodeRepository
	OrderRepo             repository.OrderRepository
	PaymentRepo           repository.PaymentRepository
	PaymentChannelRepo    repository.PaymentChannelRepository
	CardSecretRepo        repository.CardSecretRepository
	CardSecretBatchRepo   repository.CardSecretBatchRepository
	FulfillmentRepo       repository.FulfillmentRepository
	ProductRepo           repository.ProductRepository
	ProductSKURepo        repository.ProductSKURepository
	CartRepo              repository.CartRepository
	CouponRepo            repository.CouponRepository
	CouponUsageRepo       repository.CouponUsageRepository
	PromotionRepo         repository.PromotionRepository
	WalletRepo            repository.WalletRepository
	PostRepo              repository.PostRepository
	CategoryRepo          repository.CategoryRepository
	BannerRepo            repository.BannerRepository
	SettingRepo           repository.SettingRepository
	UserLoginLogRepo      repository.UserLoginLogRepository
	AuthzAuditLogRepo     repository.AuthzAuditLogRepository
	DashboardRepo         repository.DashboardRepository

	// Services
	AuthzService          *authz.Service
	AuthService           *service.AuthService
	UserAuthService       *service.UserAuthService
	TelegramAuthService   *service.TelegramAuthService
	EmailService          *service.EmailService
	CaptchaService        *service.CaptchaService
	UploadService         *service.UploadService
	ProductService        *service.ProductService
	PostService           *service.PostService
	CategoryService       *service.CategoryService
	SettingService        *service.SettingService
	CartService           *service.CartService
	WalletService         *service.WalletService
	OrderService          *service.OrderService
	FulfillmentService    *service.FulfillmentService
	CouponAdminService    *service.CouponAdminService
	PromotionAdminService *service.PromotionAdminService
	BannerService         *service.BannerService
	PaymentService        *service.PaymentService
	CardSecretService     *service.CardSecretService
	UserLoginLogService   *service.UserLoginLogService
	AuthzAuditService     *service.AuthzAuditService
	DashboardService      *service.DashboardService
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
	c.FulfillmentRepo = repository.NewFulfillmentRepository(db)
	c.ProductRepo = repository.NewProductRepository(db)
	c.ProductSKURepo = repository.NewProductSKURepository(db)
	c.CartRepo = repository.NewCartRepository(db)
	c.CouponRepo = repository.NewCouponRepository(db)
	c.CouponUsageRepo = repository.NewCouponUsageRepository(db)
	c.PromotionRepo = repository.NewPromotionRepository(db)
	c.WalletRepo = repository.NewWalletRepository(db)
	c.PostRepo = repository.NewPostRepository(db)
	c.CategoryRepo = repository.NewCategoryRepository(db)
	c.BannerRepo = repository.NewBannerRepository(db)
	c.SettingRepo = repository.NewSettingRepository(db)
	c.UserLoginLogRepo = repository.NewUserLoginLogRepository(db)
	c.AuthzAuditLogRepo = repository.NewAuthzAuditLogRepository(db)
	c.DashboardRepo = repository.NewDashboardRepository(db)
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
	c.UserAuthService = service.NewUserAuthService(c.Config, c.UserRepo, c.UserOAuthIdentityRepo, c.EmailVerifyCodeRepo, c.EmailService, c.TelegramAuthService, c.SettingService)
	c.UploadService = service.NewUploadService(c.Config)
	c.ProductService = service.NewProductService(c.ProductRepo, c.ProductSKURepo)
	c.PostService = service.NewPostService(c.PostRepo)
	c.CategoryService = service.NewCategoryService(c.CategoryRepo)
	c.CartService = service.NewCartService(c.CartRepo, c.ProductRepo, c.ProductSKURepo, c.PromotionRepo, c.SettingService)
	c.WalletService = service.NewWalletService(c.WalletRepo, c.OrderRepo, c.UserRepo)
	c.OrderService = service.NewOrderService(c.OrderRepo, c.ProductRepo, c.ProductSKURepo, c.CardSecretRepo, c.CouponRepo, c.CouponUsageRepo, c.PromotionRepo, c.QueueClient, c.SettingService, c.WalletService, c.Config.Order.PaymentExpireMinutes)
	c.FulfillmentService = service.NewFulfillmentService(c.OrderRepo, c.FulfillmentRepo, c.CardSecretRepo, c.QueueClient)
	c.CardSecretService = service.NewCardSecretService(c.CardSecretRepo, c.CardSecretBatchRepo, c.ProductRepo, c.ProductSKURepo)
	c.CouponAdminService = service.NewCouponAdminService(c.CouponRepo)
	c.PromotionAdminService = service.NewPromotionAdminService(c.PromotionRepo)
	c.BannerService = service.NewBannerService(c.BannerRepo)
	c.PaymentService = service.NewPaymentService(c.OrderRepo, c.ProductRepo, c.ProductSKURepo, c.PaymentRepo, c.PaymentChannelRepo, c.WalletRepo, c.QueueClient, c.WalletService)
	c.UserLoginLogService = service.NewUserLoginLogService(c.UserLoginLogRepo)
	c.AuthzAuditService = service.NewAuthzAuditService(c.AuthzAuditLogRepo)
	c.DashboardService = service.NewDashboardService(c.DashboardRepo, c.SettingService)
}

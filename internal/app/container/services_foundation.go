package container

import (
	"github.com/dujiao-next/internal/authz"
	catalogproductbootstrap "github.com/dujiao-next/internal/bootstrap/catalogproduct"
	mailbrandwiring "github.com/dujiao-next/internal/bootstrap/mailbrand"
	telegramauthcache "github.com/dujiao-next/internal/bootstrap/telegramauthcache"
	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/logger"
	affiliateapp "github.com/dujiao-next/internal/modules/affiliate/application"
	captchaapp "github.com/dujiao-next/internal/modules/captcha/application"
	captchaturnstile "github.com/dujiao-next/internal/modules/captcha/infrastructure/turnstile"
	complianceapp "github.com/dujiao-next/internal/modules/compliance/application"
	adminauthapp "github.com/dujiao-next/internal/modules/identity/adminauth/application"
	admintotpapp "github.com/dujiao-next/internal/modules/identity/adminauth/totp/application"
	githubauthapp "github.com/dujiao-next/internal/modules/identity/githubauth/application"
	googleauthapp "github.com/dujiao-next/internal/modules/identity/googleauth/application"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	userauthcachestore "github.com/dujiao-next/internal/modules/identity/userauth/infrastructure/cachestore"
	userauthgormstore "github.com/dujiao-next/internal/modules/identity/userauth/infrastructure/gormstore"
	usertotpapp "github.com/dujiao-next/internal/modules/identity/userauth/totp/application"
	notificationsmtp "github.com/dujiao-next/internal/modules/notification/infrastructure/smtp"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	reseller "github.com/dujiao-next/internal/modules/reseller/application"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	uploadapp "github.com/dujiao-next/internal/modules/upload/application"
	uploadlocal "github.com/dujiao-next/internal/modules/upload/infrastructure/localstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
)

// initPolicyAndSettingServices 装配授权、动态设置、分销商基础能力与合规服务。
func (c *Container) initPolicyAndSettingServices() {
	authzService, err := authz.NewService(gormdb.DB)
	if err != nil {
		logger.Errorw("provider_init_authz_failed", "error", err)
		panic(err)
	}
	c.AuthzService = authzService
	if err := c.AuthzService.BootstrapBuiltinRoles(); err != nil {
		logger.Errorw("provider_bootstrap_builtin_roles_failed", "error", err)
		panic(err)
	}

	c.SettingService = settingsapp.NewService(c.SettingRepo, c.Config.Order)
	c.EmailBrandResolver = mailbrandwiring.New(c.SettingService, c.ResellerStore)
	c.ResellerDomainResolver = reseller.NewDomainResolver(c.ResellerStore, c.Config.Reseller)
	c.ResellerPricingResolver = orderapp.NewResellerPricingResolver(c.ResellerStore)
	c.ResellerManagementService = reseller.NewManagementService(c.ResellerStore, c.Config.Reseller)
	c.ResellerSiteConfigService = reseller.NewSiteConfigService(c.ResellerStore)
	c.ResellerProductSettingService = reseller.NewProductSettingService(c.ResellerStore, c.ProductRepo)
	c.ResellerAccountingQuery = reseller.NewAccountingQueryService(c.ResellerStore)
	c.ResellerAccountingWithdraw = reseller.NewAccountingWithdrawService(c.ResellerStore)
	c.ResellerAccountingLedger = reseller.NewAccountingLedgerService(
		c.ResellerStore,
		c.Config.Reseller.SettlementConfirmDays,
	)
	c.ResellerOrderService = reseller.NewOrderQueryService(c.ResellerStore)
	c.ResellerOperationsService = reseller.NewOperationsService(c.ResellerStore)
	c.ComplianceService = complianceapp.NewService(c.SettingRepo)
}

// loadRuntimeSettings 用数据库设置覆盖启动配置中的可动态配置项。
func (c *Container) loadRuntimeSettings() {
	smtpSetting, err := c.SettingService.GetSMTPSetting(c.Config.Email)
	if err != nil {
		logger.Warnw("provider_load_smtp_setting_failed", "error", err)
	} else {
		c.Config.Email = settingsmessaging.SMTPSettingToConfig(smtpSetting)
	}

	captchaSetting, err := c.SettingService.GetCaptchaSetting(c.Config.Captcha)
	if err != nil {
		logger.Warnw("provider_load_captcha_setting_failed", "error", err)
	} else {
		c.Config.Captcha = settingssecurity.CaptchaSettingToConfig(captchaSetting)
	}

	telegramAuthSetting, err := c.SettingService.GetTelegramAuthSetting(c.Config.TelegramAuth)
	if err != nil {
		logger.Warnw("provider_load_telegram_auth_setting_failed", "error", err)
	} else {
		c.Config.TelegramAuth = settingssecurity.TelegramAuthSettingToConfig(telegramAuthSetting)
	}

	googleAuthSetting, err := c.SettingService.GetGoogleAuthSetting(c.Config.GoogleAuth)
	if err != nil {
		// Database-backed settings are the administrative source of truth. A
		// read failure must not resurrect an enabled YAML fallback after an
		// operator disabled Google login in the database.
		c.Config.GoogleAuth.Enabled = false
		logger.Warnw(
			"provider_load_google_auth_setting_failed",
			"error", err,
			"fail_closed", true,
		)
	} else {
		c.Config.GoogleAuth = settingssecurity.GoogleAuthSettingToConfig(googleAuthSetting)
	}

	gitHubAuthSetting, gitHubErr := c.SettingService.GetGitHubAuthSetting(c.Config.GitHubAuth)
	if gitHubErr != nil {
		c.Config.GitHubAuth.Enabled = false
		logger.Warnw("provider_load_github_auth_setting_failed", "error", gitHubErr, "fail_closed", true)
	} else {
		c.Config.GitHubAuth = settingssecurity.GitHubAuthSettingToConfig(gitHubAuthSetting)
	}
}

// initIdentityAndCatalogServices 装配身份认证、上传、推广与商品读取能力。
func (c *Container) initIdentityAndCatalogServices() {
	c.EmailSender = notificationsmtp.New(&c.Config.Email)
	c.CaptchaService = captchaapp.NewService(c.SettingService, c.Config.Captcha, captchaturnstile.New())
	c.AuthService = adminauthapp.NewService(c.Config, c.AdminStore)
	c.TOTPService = admintotpapp.NewService(c.Config, c.AdminStore, cache.Client())
	c.UserTOTPService = usertotpapp.NewService(c.Config, c.UserStore, cache.Client())
	c.TelegramAuthService = telegramauthapp.NewService(c.Config.TelegramAuth, telegramauthcache.Options()...)
	c.GoogleAuthService = googleauthapp.NewService(c.Config.GoogleAuth)
	c.GitHubAuthService = githubauthapp.NewService(c.Config.GitHubAuth)
	c.UserAuthService = userauthapp.NewService(c.Config, c.UserStore, c.ExternalIdentityStore, c.EmailVerificationStore, c.SettingService, c.EmailSender, c.TelegramAuthService)
	c.UserAuthService.SetGoogleAuthService(c.GoogleAuthService)
	c.UserAuthService.SetGoogleRedirectStore(userauthcachestore.NewGoogleRedirectStore())
	c.UserAuthService.SetGitHubStateStore(userauthcachestore.NewGitHubStateStore())
	c.UserAuthService.SetAuthUnitOfWork(userauthgormstore.New(gormdb.DB))
	c.UserAuthService.SetEmailBrandResolver(c.EmailBrandResolver)
	c.UploadService = uploadapp.NewService(uploadapp.Policy{
		MaxSize:           c.Config.Upload.MaxSize,
		AllowedTypes:      c.Config.Upload.AllowedTypes,
		AllowedExtensions: c.Config.Upload.AllowedExtensions,
		MaxWidth:          c.Config.Upload.MaxWidth,
		MaxHeight:         c.Config.Upload.MaxHeight,
	}, uploadlocal.New("uploads"))
	c.AffiliateService = affiliateapp.NewService(c.AffiliateRepo, c.UserStore, c.OrderStore, c.ProductRepo, c.SettingService)
	productServices := catalogproductbootstrap.New(catalogproductbootstrap.Dependencies{
		Products:          c.ProductRepo,
		SKUs:              c.ProductSKURepo,
		CardSecrets:       c.CardSecretRepo,
		CardSecretBatches: c.CardSecretBatchRepo,
		Categories:        c.CategoryRepo,
		MemberLevelPrices: c.MemberLevelPriceRepo,
		Carts:             c.CartRepo,
		ProductMappings:   c.ProductMappingRepo,
		Orders:            c.OrderStore,
		PaymentChannels:   c.PaymentChannelStore,
	})
	c.ProductReadService = productServices.Read
	c.ProductAdminService = productServices.Admin
	c.ProductWriteService = productServices.Write
}

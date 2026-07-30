package httpserver

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/app/httpserver/middleware"
	"github.com/dujiao-next/internal/authz"
	adminauthwiring "github.com/dujiao-next/internal/bootstrap/adminauth"
	adminauthzwiring "github.com/dujiao-next/internal/bootstrap/adminauthz"
	adminuserwiring "github.com/dujiao-next/internal/bootstrap/adminuser"
	affiliatebootstrap "github.com/dujiao-next/internal/bootstrap/affiliate"
	catalogproductbootstrap "github.com/dujiao-next/internal/bootstrap/catalogproduct"
	channelwiring "github.com/dujiao-next/internal/bootstrap/channelapi"
	channeluserwiring "github.com/dujiao-next/internal/bootstrap/channeluser"
	fulfillmentwiring "github.com/dujiao-next/internal/bootstrap/fulfillment"
	orderwiring "github.com/dujiao-next/internal/bootstrap/order"
	paymentwiring "github.com/dujiao-next/internal/bootstrap/payment"
	publicconfigwiring "github.com/dujiao-next/internal/bootstrap/publicconfig"
	resellerbootstrap "github.com/dujiao-next/internal/bootstrap/reseller"
	upstreamwiring "github.com/dujiao-next/internal/bootstrap/upstreamapi"
	userauthwiring "github.com/dujiao-next/internal/bootstrap/userauth"
	walletbootstrap "github.com/dujiao-next/internal/bootstrap/wallet"
	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	apicredentialtransport "github.com/dujiao-next/internal/modules/apicredential/transport/http"
	auditlogtransport "github.com/dujiao-next/internal/modules/auditlog/transport/http"
	captchahttp "github.com/dujiao-next/internal/modules/captcha/transport/http"
	cardsecrettransport "github.com/dujiao-next/internal/modules/cardsecret/transport/http"
	carttransport "github.com/dujiao-next/internal/modules/cart/transport/http"
	categoryhttp "github.com/dujiao-next/internal/modules/catalog/category/transport/http"
	mappinghttp "github.com/dujiao-next/internal/modules/catalog/mapping/transport/http"
	producthttp "github.com/dujiao-next/internal/modules/catalog/product/transport/http"
	contenttransport "github.com/dujiao-next/internal/modules/content/transport/http"
	coupontransport "github.com/dujiao-next/internal/modules/coupon/transport/http"
	dashboardtransport "github.com/dujiao-next/internal/modules/dashboard/transport/http"
	giftcardtransport "github.com/dujiao-next/internal/modules/giftcard/transport/http"
	memberleveltransport "github.com/dujiao-next/internal/modules/memberlevel/transport/http"
	notificationtransport "github.com/dujiao-next/internal/modules/notification/transport/http"
	procurementtransport "github.com/dujiao-next/internal/modules/procurement/transport/http"
	promotiontransport "github.com/dujiao-next/internal/modules/promotion/transport/http"
	settingstransport "github.com/dujiao-next/internal/modules/settings/transport/http"
	sitemapbrand "github.com/dujiao-next/internal/modules/sitemap/infrastructure/settingsbrand"
	sitemaptransport "github.com/dujiao-next/internal/modules/sitemap/transport/http"
	telegramchanneltransport "github.com/dujiao-next/internal/modules/telegram/channelbot/transport/http"
	"github.com/dujiao-next/internal/web"

	"github.com/gin-gonic/gin"
)

// SetupRouter 初始化路由。
func SetupRouter(cfg *config.Config, c *container.Container) *gin.Engine {
	log := logger.L
	if log == nil {
		log = logger.Init(cfg.Server.Mode, cfg.Log.ToLoggerOptions())
	}
	r := gin.New()
	captchaVerifier := captchahttp.NewVerifier(c.CaptchaService)

	// 初始化 Handler（按前台/后台分组）
	adminAuthHandlers := adminauthwiring.New(c)
	adminLoginHandler := adminAuthHandlers.Login
	admin2FAHandler := adminAuthHandlers.TwoFA
	adminUser2FAHandler := adminAuthHandlers.UserTwoFA
	adminUserHandler := adminuserwiring.NewHandler(c)
	adminAuthzHandler := adminauthzwiring.NewHandler(c)
	adminFulfillmentHandler := fulfillmentwiring.NewAdminHandler(c)
	orderHandlers := orderwiring.New(c)
	adminOrderHandler := orderHandlers.Admin
	adminOrderRefundHandler := orderHandlers.AdminRefund
	userOrderHandler := orderHandlers.User
	guestOrderHandler := orderHandlers.Guest
	orderPreviewHandler := orderHandlers.Preview
	orderCreateHandler := orderHandlers.Create
	paymentHandlers := paymentwiring.New(c)
	paymentLatestHandler := paymentHandlers.Latest
	paymentWriteHandler := paymentHandlers.Write
	adminPaymentHandler := paymentHandlers.Admin
	adminPaymentChannelHandler := paymentHandlers.AdminChannel
	paymentWebhookHandler := paymentHandlers.Webhook
	paymentCallbackHandler := paymentHandlers.Callback
	publicConfigHandler := publicconfigwiring.NewHandler(c)
	userCartHandler := carttransport.NewUserHandler(c.CartService)
	channelHandler := channelwiring.NewHandler(c)
	upstreamHandler := upstreamwiring.NewHandler(c)
	publicContentHandler := contenttransport.NewPublicHandler(
		c.ContentPostService,
		c.ContentPostCategoryService,
		c.ContentBannerService,
	)
	publicCatalogHandler := catalogproductbootstrap.NewPublicHTTP(catalogproductbootstrap.PublicHTTPDependencies{
		Products:     c.ProductReadService,
		Hidden:       c.ResellerStore,
		Pricer:       c.ResellerPricingResolver,
		Promotions:   c.PromotionRepo,
		MemberLevels: c.MemberLevelService,
		Mappings:     c.ProductMappingRepo,
		SKUMappings:  c.SKUMappingRepo,
		RelatedPosts: c.ContentPostService,
	})
	publicCategoryHandler := categoryhttp.NewPublicHandler(c.CategoryService)
	adminContentHandler := contenttransport.NewAdminHandler(
		c.ContentPostService,
		c.ContentPostCategoryService,
		c.ContentBannerService,
		c.ContentMediaService,
	)
	adminDashboardHandler := dashboardtransport.NewAdminHandler(c.DashboardService)
	adminMemberLevelHandler := memberleveltransport.NewAdminHandler(c.MemberLevelService)
	publicMemberLevelHandler := memberleveltransport.NewPublicHandler(c.MemberLevelService)
	userAuthHandlers := userauthwiring.New(c)
	userProfileHandler := userAuthHandlers.Profile
	userEmailHandler := userAuthHandlers.Email
	userPasswordHandler := userAuthHandlers.Password
	userVerifyHandler := userAuthHandlers.Verify
	userLoginHandler := userAuthHandlers.Login
	user2FAHandler := userAuthHandlers.TwoFA
	userTelegramOIDCHandler := userAuthHandlers.TelegramOIDC
	userTelegramHandler := userAuthHandlers.Telegram
	walletHandlers := walletbootstrap.New(c)
	userWalletHandler := walletHandlers.User
	adminWalletHandler := walletHandlers.Admin
	channelWalletHandler := walletHandlers.Channel
	channelMemberLevelHandler := memberleveltransport.NewChannelHandler(c.MemberLevelService)
	adminApiCredentialHandler := apicredentialtransport.NewAdminHandler(c.ApiCredentialService)
	userApiCredentialHandler := apicredentialtransport.NewUserHandler(c.ApiCredentialService)
	adminAuditLogHandler := auditlogtransport.NewAdminHandler(c.AuthzAuditService, c.UserLoginLogService)
	adminCardSecretHandler := cardsecrettransport.NewAdminHandler(c.CardSecretService)
	adminCatalogCategoryHandler := categoryhttp.NewAdminCategoryHandler(c.CategoryService)
	adminCatalogProductHandler := producthttp.NewAdminProductHandler(
		c.ProductReadService,
		c.ProductWriteService,
		c.ProductAdminService,
		c.SettingService,
		c.ProductMappingRepo,
		c.SKUMappingRepo,
	)
	adminCatalogProductMappingHandler := mappinghttp.NewAdminHandler(c.ProductMappingService)
	userAuditLogHandler := auditlogtransport.NewUserHandler(c.UserLoginLogService)
	adminCouponHandler := coupontransport.NewAdminHandler(c.CouponAdminService)
	adminGiftCardHandler := giftcardtransport.NewAdminHandler(c.GiftCardService)
	userGiftCardHandler := giftcardtransport.NewUserHandler(c.GiftCardService, captchaVerifier)
	channelGiftCardHandler := giftcardtransport.NewChannelHandler(
		c.GiftCardService,
		channeluserwiring.NewSimpleProvisioner(c.UserAuthService),
	)
	channelAffiliateHandler := affiliatebootstrap.NewChannelHandler(c)
	channelTelegramBotHandler := telegramchanneltransport.NewChannelBotHandler(c.SettingService, c.ChannelClientService)
	adminSettingsHandler := settingstransport.NewAdminHandler(c.SettingService)
	adminPromotionHandler := promotiontransport.NewAdminHandler(c.PromotionAdminService)
	adminNotificationHandler := notificationtransport.NewAdminHandler(c.SettingService, c.NotificationLogService, c.NotificationService)
	adminProcurementHandler := procurementtransport.NewAdminHandler(c.ProcurementOrderService)
	resellerHandlers := resellerbootstrap.New(c)
	userResellerHandler := resellerHandlers.User
	userResellerProductSettingHandler := resellerHandlers.UserProductSetting
	userResellerFinanceHandler := resellerHandlers.UserFinance
	userResellerOrderHandler := resellerHandlers.UserOrder
	adminResellerManagementHandler := resellerHandlers.AdminManagement
	adminResellerProfileDetailHandler := resellerHandlers.AdminProfileDetail
	adminResellerSiteConfigHandler := resellerHandlers.AdminSiteConfig
	adminResellerProductSettingHandler := resellerHandlers.AdminProductSetting
	adminResellerOperationsHandler := resellerHandlers.AdminOperations
	adminResellerFinanceHandler := resellerHandlers.AdminFinance

	redisPrefix := strings.TrimSpace(cfg.Redis.Prefix)
	if redisPrefix == "" {
		redisPrefix = constants.RedisPrefixDefault
	}
	redisClient := cache.Client()
	loginRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:login", redisPrefix),
		WindowSeconds: cfg.Security.LoginRateLimit.WindowSeconds,
		MaxRequests:   cfg.Security.LoginRateLimit.MaxAttempts,
		BlockSeconds:  cfg.Security.LoginRateLimit.BlockSeconds,
		MessageKey:    "error.login_too_many",
	}
	adminLoginRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:admin_login", redisPrefix),
		WindowSeconds: cfg.Security.LoginRateLimit.WindowSeconds,
		MaxRequests:   cfg.Security.LoginRateLimit.MaxAttempts,
		BlockSeconds:  cfg.Security.LoginRateLimit.BlockSeconds,
		MessageKey:    "error.login_too_many",
	}
	guestReadRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:guest_orders:read", redisPrefix),
		WindowSeconds: 60,
		MaxRequests:   120,
		BlockSeconds:  60,
		MessageKey:    "error.rate_limited",
	}
	guestWriteRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:guest_orders:write", redisPrefix),
		WindowSeconds: 60,
		MaxRequests:   20,
		BlockSeconds:  300,
		MessageKey:    "error.rate_limited",
	}
	upstreamAPIRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:upstream_api", redisPrefix),
		WindowSeconds: 60,
		MaxRequests:   60,
		BlockSeconds:  30,
		MessageKey:    "error.rate_limited",
	}

	// middleware.RequestIDMiddleware 必须前置于 middleware.RecoveryMiddleware：panic 日志与响应都依赖 request_id。
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.RecoveryMiddleware())
	r.Use(middleware.LoggerMiddleware(log))
	r.Use(middleware.CORSMiddleware(cfg.CORS))
	r.Use(middleware.CallbackRouteMiddleware(c.SettingService, paymentCallbackHandler, paymentWebhookHandler, upstreamHandler))

	// 静态文件服务（上传的图片）必须放在前面。
	r.Static("/uploads", "./uploads")

	// SEO 资源（动态生成）。
	sitemapHandler := sitemaptransport.NewHandler(c.SitemapService, sitemapbrand.New(c.SettingService), cfg.Web.UserSPADisabled)
	sitemaptransport.RegisterRoutes(r, sitemapHandler)

	apiV1 := r.Group("/api/v1")
	registerStorefrontRoutes(apiV1, cfg, c, publicContentHandler, publicCatalogHandler, publicCategoryHandler, userResellerHandler, userResellerProductSettingHandler, userResellerFinanceHandler, userResellerOrderHandler, userApiCredentialHandler, userAuditLogHandler, userGiftCardHandler, publicMemberLevelHandler, userProfileHandler, userEmailHandler, userPasswordHandler, userVerifyHandler, userTelegramOIDCHandler, userTelegramHandler, userLoginHandler, user2FAHandler, publicConfigHandler, userCartHandler, userOrderHandler, guestOrderHandler, orderPreviewHandler, orderCreateHandler, paymentLatestHandler, paymentWriteHandler, userWalletHandler, redisClient, loginRule, guestReadRule, guestWriteRule)
	registerUpstreamRoutes(apiV1, c, upstreamHandler, redisClient, upstreamAPIRule)
	registerChannelRoutes(apiV1, c, channelHandler, channelMemberLevelHandler, channelGiftCardHandler, channelAffiliateHandler, channelTelegramBotHandler, channelWalletHandler)
	registerPaymentCallbackRoutes(apiV1, paymentCallbackHandler, paymentWebhookHandler)
	registerAdminRoutes(r, apiV1, cfg, c, adminLoginHandler, admin2FAHandler, adminUser2FAHandler, adminUserHandler, adminAuthzHandler, adminFulfillmentHandler, adminOrderHandler, adminOrderRefundHandler, adminContentHandler, adminDashboardHandler, adminMemberLevelHandler, adminApiCredentialHandler, adminAuditLogHandler, adminCardSecretHandler, adminCatalogCategoryHandler, adminCatalogProductHandler, adminCatalogProductMappingHandler, adminCouponHandler, adminGiftCardHandler, adminPromotionHandler, adminNotificationHandler, adminProcurementHandler, adminResellerManagementHandler, adminResellerProfileDetailHandler, adminResellerSiteConfigHandler, adminResellerProductSettingHandler, adminResellerOperationsHandler, adminResellerFinanceHandler, adminSettingsHandler, adminWalletHandler, adminPaymentHandler, adminPaymentChannelHandler, redisClient, adminLoginRule)

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 嵌入式前端资源（仅在 -tags fullstack 构建时生效）
	if web.Enabled() {
		// cmd/server 已在数据库初始化之前校验过一次；这里保留是为了兜住其它调用方
		// （测试、未来的其它入口）——重复校验没有代价，漏校验会让 RegisterAdmin panic。
		if err := web.ValidateAdminPath(cfg.Web.AdminPath); err != nil {
			log.Sugar().Fatalf("web.admin_path 配置错误: %v", err)
		}
		if err := web.RegisterAdmin(r, cfg.Web.AdminPath, web.AdminFS()); err != nil {
			log.Sugar().Fatalf("注册 admin SPA 失败: %v", err)
		}

		// user SPA：受 user_spa_disabled 控制
		if !cfg.Web.UserSPADisabled {
			if err := web.RegisterUser(r, web.UserFS()); err != nil {
				log.Sugar().Fatalf("注册 user SPA 失败: %v", err)
			}
		}
		// user_spa_disabled=true：不注册 user SPA，NoRoute 默认返回 404
	}

	return r
}

type adminPermissionCatalogItem struct {
	Module     string `json:"module"`
	Method     string `json:"method"`
	Object     string `json:"object"`
	Permission string `json:"permission"`
}

func buildAdminPermissionCatalog(engine *gin.Engine) []adminPermissionCatalogItem {
	if engine == nil {
		return []adminPermissionCatalogItem{}
	}

	routes := engine.Routes()
	seen := make(map[string]struct{}, len(routes))
	items := make([]adminPermissionCatalogItem, 0, len(routes))

	for _, item := range routes {
		method := strings.ToUpper(strings.TrimSpace(item.Method))
		if method == "" || method == http.MethodOptions || method == http.MethodHead {
			continue
		}
		if !strings.HasPrefix(item.Path, "/api/v1/admin/") {
			continue
		}
		if item.Path == "/api/v1/admin/login" {
			continue
		}
		if item.Path == "/api/v1/admin/login/verify-2fa" {
			continue
		}
		object := authz.NormalizeObject(item.Path)
		permission := method + ":" + object
		if _, exists := seen[permission]; exists {
			continue
		}
		seen[permission] = struct{}{}
		items = append(items, adminPermissionCatalogItem{
			Module:     deriveAdminPermissionModule(object),
			Method:     method,
			Object:     object,
			Permission: permission,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Module == items[j].Module {
			if items[i].Object == items[j].Object {
				return items[i].Method < items[j].Method
			}
			return items[i].Object < items[j].Object
		}
		return items[i].Module < items[j].Module
	})

	return items
}

func deriveAdminPermissionModule(object string) string {
	normalized := strings.TrimPrefix(strings.TrimSpace(object), "/")
	if normalized == "" {
		return "system"
	}
	segments := strings.Split(normalized, "/")
	if len(segments) <= 1 {
		return segments[0]
	}
	if segments[0] != "admin" {
		return segments[0]
	}
	if segments[1] == "authz" {
		return "authz"
	}
	return segments[1]
}

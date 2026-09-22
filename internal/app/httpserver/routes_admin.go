package httpserver

import (
	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/app/httpserver/middleware"
	affiliatebootstrap "github.com/dujiao-next/internal/bootstrap/affiliate"
	settingsbootstrap "github.com/dujiao-next/internal/bootstrap/settingshttp"
	"github.com/dujiao-next/internal/config"
	adproxytransport "github.com/dujiao-next/internal/modules/adproxy/transport/http"
	affiliatetransport "github.com/dujiao-next/internal/modules/affiliate/transport/http"
	apicredentialtransport "github.com/dujiao-next/internal/modules/apicredential/transport/http"
	auditlogtransport "github.com/dujiao-next/internal/modules/auditlog/transport/http"
	cardsecrettransport "github.com/dujiao-next/internal/modules/cardsecret/transport/http"
	categoryhttp "github.com/dujiao-next/internal/modules/catalog/category/transport/http"
	mappinghttp "github.com/dujiao-next/internal/modules/catalog/mapping/transport/http"
	producthttp "github.com/dujiao-next/internal/modules/catalog/product/transport/http"
	channelclienthttp "github.com/dujiao-next/internal/modules/channelclient/transport/http"
	compliancetransport "github.com/dujiao-next/internal/modules/compliance/transport/http"
	contenttransport "github.com/dujiao-next/internal/modules/content/transport/http"
	coupontransport "github.com/dujiao-next/internal/modules/coupon/transport/http"
	dashboardtransport "github.com/dujiao-next/internal/modules/dashboard/transport/http"
	fulfillmenttransport "github.com/dujiao-next/internal/modules/fulfillment/transport/http"
	giftcardtransport "github.com/dujiao-next/internal/modules/giftcard/transport/http"
	adminauthtransport "github.com/dujiao-next/internal/modules/identity/adminauth/transport/http"
	adminauthztransport "github.com/dujiao-next/internal/modules/identity/adminauthorization/transport/http"
	adminusertransport "github.com/dujiao-next/internal/modules/identity/user/transport/http/admin"
	memberleveltransport "github.com/dujiao-next/internal/modules/memberlevel/transport/http"
	notificationtransport "github.com/dujiao-next/internal/modules/notification/transport/http"
	ordertransport "github.com/dujiao-next/internal/modules/order/transport/http"
	paymenttransport "github.com/dujiao-next/internal/modules/payment/transport/http"
	procurementtransport "github.com/dujiao-next/internal/modules/procurement/transport/http"
	promotiontransport "github.com/dujiao-next/internal/modules/promotion/transport/http"
	reconciliationtransport "github.com/dujiao-next/internal/modules/reconciliation/transport/http"
	resellertransport "github.com/dujiao-next/internal/modules/reseller/transport/http/admin"
	settingstransport "github.com/dujiao-next/internal/modules/settings/transport/http"
	siteconnectiontransport "github.com/dujiao-next/internal/modules/siteconnection/transport/http"
	broadcasthttp "github.com/dujiao-next/internal/modules/telegram/broadcast/transport/http"
	uploadtransport "github.com/dujiao-next/internal/modules/upload/transport/http"
	wallettransport "github.com/dujiao-next/internal/modules/wallet/transport/http"
	"github.com/dujiao-next/internal/platform/http/response"
	systemtransport "github.com/dujiao-next/internal/platform/http/system"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func registerAdminRoutes(
	engine *gin.Engine,
	apiV1 *gin.RouterGroup,
	cfg *config.Config,
	c *container.Container,
	adminLoginHandler *adminauthtransport.AdminLoginHandler,
	admin2FAHandler *adminauthtransport.Admin2FAHandler,
	adminUser2FAHandler *adminauthtransport.AdminUser2FAHandler,
	adminUserHandler *adminusertransport.AdminHandler,
	adminAuthzHandler *adminauthztransport.AdminHandler,
	adminFulfillmentHandler *fulfillmenttransport.AdminHandler,
	adminOrderHandler *ordertransport.AdminHandler,
	adminOrderRefundHandler *ordertransport.AdminRefundHandler,
	adminContentHandler *contenttransport.AdminHandler,
	adminDashboardHandler *dashboardtransport.AdminHandler,
	adminMemberLevelHandler *memberleveltransport.AdminHandler,
	adminApiCredentialHandler *apicredentialtransport.AdminHandler,
	adminAuditLogHandler *auditlogtransport.AdminHandler,
	adminCardSecretHandler *cardsecrettransport.AdminHandler,
	adminCatalogCategoryHandler *categoryhttp.AdminCategoryHandler,
	adminCatalogProductHandler *producthttp.AdminProductHandler,
	adminCatalogProductMappingHandler *mappinghttp.AdminHandler,
	adminCouponHandler *coupontransport.AdminHandler,
	adminGiftCardHandler *giftcardtransport.AdminHandler,
	adminPromotionHandler *promotiontransport.AdminHandler,
	adminNotificationHandler *notificationtransport.AdminHandler,
	adminProcurementHandler *procurementtransport.AdminHandler,
	adminResellerManagementHandler *resellertransport.AdminManagementHandler,
	adminResellerProfileDetailHandler *resellertransport.AdminProfileDetailHandler,
	adminResellerSiteConfigHandler *resellertransport.AdminSiteConfigHandler,
	adminResellerProductSettingHandler *resellertransport.AdminProductSettingHandler,
	adminResellerOperationsHandler *resellertransport.AdminOperationsHandler,
	adminResellerFinanceHandler *resellertransport.AdminFinanceHandler,
	adminSettingsHandler *settingstransport.AdminHandler,
	adminWalletHandler *wallettransport.AdminHandler,
	adminPaymentHandler *paymenttransport.AdminHandler,
	adminPaymentChannelHandler *paymenttransport.AdminChannelHandler,
	redisClient *redis.Client,
	adminLoginRule middleware.RateLimitRule,
) {
	admin := apiV1.Group("/admin")

	// 登录接口（无需鉴权）
	adminauthtransport.RegisterAdminLoginAuthRoutes(admin, adminLoginHandler, middleware.RateLimitMiddleware(redisClient, adminLoginRule, middleware.KeyByIP))
	adminauthtransport.RegisterAdmin2FAAuthRoutes(admin, admin2FAHandler, middleware.RateLimitMiddleware(redisClient, adminLoginRule, middleware.KeyByIP))

	// 需要鉴权的接口
	authorized := admin.Use(middleware.JWTAuthMiddleware(cfg.JWT.SecretKey, c.AdminStore), middleware.AdminRBACMiddleware(c.AuthzService))
	// 支付/财务相关受保护子组：未确认合规声明时拦截
	// 注：admin.Use(...) 已 mutate admin 自身，新 Group 继承 JWT + RBAC 中间件
	paymentProtected := admin.Group("", middleware.PaymentComplianceRequired(c.ComplianceService))

	// 合规声明
	compliancetransport.RegisterAdminRoutes(authorized, compliancetransport.NewAdminHandler(c.ComplianceService))

	// 仪表盘
	dashboardtransport.RegisterAdminRoutes(authorized, adminDashboardHandler)

	// 广告代理
	adproxytransport.RegisterAdminRoutes(authorized, adproxytransport.NewAdminHandler(c.AdProxyService))

	// 商品 / 分类管理
	producthttp.RegisterAdminRoutes(authorized, adminCatalogProductHandler)
	contenttransport.RegisterAdminRoutes(authorized, adminContentHandler)
	categoryhttp.RegisterAdminRoutes(authorized, adminCatalogCategoryHandler)

	// 设置管理
	settingstransport.RegisterAdminRoutes(authorized, adminSettingsHandler)
	settingstransport.RegisterAdminSMTPRoutes(authorized, settingsbootstrap.NewSMTPHandler(c, cfg))
	settingstransport.RegisterAdminCaptchaRoutes(authorized, settingsbootstrap.NewCaptchaHandler(c, cfg))
	settingstransport.RegisterAdminTelegramAuthRoutes(authorized, settingsbootstrap.NewTelegramAuthHandler(c, cfg))
	settingstransport.RegisterAdminGoogleAuthRoutes(authorized, settingsbootstrap.NewGoogleAuthHandler(c, cfg))
	settingstransport.RegisterAdminGitHubAuthRoutes(authorized, settingsbootstrap.NewGitHubAuthHandler(c, cfg))
	notificationtransport.RegisterAdminRoutes(authorized, adminNotificationHandler)
	settingstransport.RegisterAdminOrderEmailTemplateRoutes(authorized, settingstransport.NewOrderEmailTemplateHandler(c.SettingService))
	settingstransport.RegisterAdminAffiliateRoutes(authorized, settingstransport.NewAffiliateHandler(c.SettingService))
	settingstransport.RegisterAdminTelegramBotRoutes(authorized, settingstransport.NewTelegramBotHandler(c.SettingService))
	adminauthtransport.RegisterAdminPasswordRoutes(authorized, adminLoginHandler)

	// 系统信息与版本检测
	systemtransport.RegisterAdminRoutes(authorized, systemtransport.NewAdminHandler(nil))

	adminauthtransport.RegisterAdmin2FARoutes(authorized, admin2FAHandler)

	// 推广返利
	adminAffiliateHandler := affiliatebootstrap.NewAdminHandler(c)
	affiliatetransport.RegisterAdminRoutes(authorized, adminAffiliateHandler)
	affiliatetransport.RegisterAdminFinanceRoutes(paymentProtected, adminAffiliateHandler)
	resellertransport.RegisterOperationsOverviewRoutes(authorized, adminResellerOperationsHandler)
	resellertransport.RegisterManagementRoutes(authorized, adminResellerManagementHandler)
	resellertransport.RegisterProfileDetailRoutes(authorized, adminResellerProfileDetailHandler)
	resellertransport.RegisterSiteConfigRoutes(authorized, adminResellerSiteConfigHandler)
	resellertransport.RegisterProductSettingRoutes(authorized, adminResellerProductSettingHandler)
	resellertransport.RegisterOperationsFinanceRoutes(paymentProtected, adminResellerOperationsHandler)
	resellertransport.RegisterFinanceRoutes(paymentProtected, adminResellerFinanceHandler)

	// 权限管理
	adminauthztransport.RegisterAdminRoutes(authorized, adminAuthzHandler)
	auditlogtransport.RegisterAdminRoutes(authorized, adminAuditLogHandler)
	authorized.GET("/authz/permissions/catalog", func(ctx *gin.Context) {
		response.Success(ctx, buildAdminPermissionCatalog(engine))
	})

	// 文件上传
	uploadtransport.RegisterAdminRoutes(authorized, uploadtransport.NewAdminHandler(c.UploadService, c.ContentMediaService))

	// 订单管理
	ordertransport.RegisterAdminRoutes(authorized, adminOrderHandler)
	ordertransport.RegisterAdminRefundWriteRoutes(authorized, adminOrderRefundHandler)
	ordertransport.RegisterAdminRefundRoutes(authorized, adminOrderRefundHandler)
	fulfillmenttransport.RegisterAdminRoutes(authorized, adminFulfillmentHandler)
	cardsecrettransport.RegisterAdminRoutes(authorized, adminCardSecretHandler)
	giftcardtransport.RegisterAdminRoutes(authorized, adminGiftCardHandler)

	// 优惠券与活动价
	coupontransport.RegisterAdminRoutes(authorized, adminCouponHandler)
	promotiontransport.RegisterAdminRoutes(authorized, adminPromotionHandler)

	// 会员等级
	memberleveltransport.RegisterAdminRoutes(authorized, adminMemberLevelHandler)

	// 支付渠道与支付记录
	paymenttransport.RegisterAdminChannelRoutes(paymentProtected, adminPaymentChannelHandler)
	paymenttransport.RegisterAdminRoutes(paymentProtected, adminPaymentHandler)

	// 用户管理
	adminusertransport.RegisterAdminRoutes(authorized, adminUserHandler)
	wallettransport.RegisterAdminRoutes(paymentProtected, adminWalletHandler)
	adminauthtransport.RegisterAdminUser2FARoutes(authorized, adminUser2FAHandler)

	// API 凭证审核管理
	apicredentialtransport.RegisterAdminRoutes(authorized, adminApiCredentialHandler)

	// 站点对接连接管理
	siteconnectiontransport.RegisterAdminRoutes(authorized, siteconnectiontransport.NewAdminHandler(
		c.SiteConnectionService,
		c.ProductMappingService,
	))

	// 商品映射管理
	mappinghttp.RegisterAdminRoutes(authorized, adminCatalogProductMappingHandler)

	// 采购单管理
	procurementtransport.RegisterAdminRoutes(authorized, adminProcurementHandler)

	// 对账管理
	reconciliationtransport.RegisterAdminRoutes(paymentProtected, reconciliationtransport.NewAdminHandler(c.ReconciliationService))

	// 渠道客户端管理
	channelclienthttp.RegisterAdminRoutes(authorized, channelclienthttp.NewAdminHandler(c.ChannelClientService))

	// Telegram Bot 群发
	broadcasthttp.RegisterAdminRoutes(authorized, broadcasthttp.NewAdminHandler(c.TelegramBroadcastService))
}

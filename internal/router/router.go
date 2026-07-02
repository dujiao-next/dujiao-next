package router

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/dujiao-next/internal/authz"
	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	adminhandlers "github.com/dujiao-next/internal/http/handlers/admin"
	channelhandlers "github.com/dujiao-next/internal/http/handlers/channel"
	publichandlers "github.com/dujiao-next/internal/http/handlers/public"
	upstreamhandlers "github.com/dujiao-next/internal/http/handlers/upstream"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/provider"
	"github.com/dujiao-next/internal/web"

	"github.com/gin-gonic/gin"
)

// SetupRouter 初始化路由
func SetupRouter(cfg *config.Config, c *provider.Container) *gin.Engine {
	log := logger.L
	if log == nil {
		log = logger.Init(cfg.Server.Mode, cfg.Log.ToLoggerOptions())
	}
	r := gin.New()

	// 初始化 Handler（按前台/后台分组）
	publicHandler := publichandlers.New(c)
	adminHandler := adminhandlers.New(c)
	channelHandler := channelhandlers.New(c)
	upstreamHandler := upstreamhandlers.New(c, c.DownstreamOrderRefRepo)
	redisPrefix := strings.TrimSpace(cfg.Redis.Prefix)
	if redisPrefix == "" {
		redisPrefix = constants.RedisPrefixDefault
	}
	redisClient := cache.Client()
	loginRule := RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:login", redisPrefix),
		WindowSeconds: cfg.Security.LoginRateLimit.WindowSeconds,
		MaxRequests:   cfg.Security.LoginRateLimit.MaxAttempts,
		BlockSeconds:  cfg.Security.LoginRateLimit.BlockSeconds,
		MessageKey:    "error.login_too_many",
	}
	adminLoginRule := RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:admin_login", redisPrefix),
		WindowSeconds: cfg.Security.LoginRateLimit.WindowSeconds,
		MaxRequests:   cfg.Security.LoginRateLimit.MaxAttempts,
		BlockSeconds:  cfg.Security.LoginRateLimit.BlockSeconds,
		MessageKey:    "error.login_too_many",
	}
	upstreamAPIRule := RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:upstream_api", redisPrefix),
		WindowSeconds: 60,
		MaxRequests:   60,
		BlockSeconds:  30,
		MessageKey:    "error.rate_limited",
	}

	// 中间件
	// RequestIDMiddleware 必须前置于 RecoveryMiddleware:
	// 它本身不会 panic(仅做 uuid 生成 + c.Set + Header.Set),先注入 request_id 才能
	// 保证 RecoveryMiddleware 在 panic 时拿到的 request_id 一定不为空——既用于日志关联,
	// 也用于 response body 的 request_id 字段。
	r.Use(RequestIDMiddleware())
	r.Use(RecoveryMiddleware())
	r.Use(LoggerMiddleware(log))
	r.Use(CORSMiddleware(cfg.CORS))
	r.Use(CallbackRouteMiddleware(c.SettingService, publicHandler, upstreamHandler))

	// 静态文件服务（上传的图片）- 必须放在最前面
	r.Static("/uploads", "./uploads")

	// SEO 资源（动态生成）
	r.GET("/sitemap.xml", publicHandler.GetSitemap)
	r.GET("/robots.txt", publicHandler.GetRobots)

	// API 路由组
	apiV1 := r.Group("/api/v1")
	{
		storefront := apiV1.Group("")
		storefront.Use(ResellerTenantMiddleware(c.ResellerDomainResolver))

		// 公开接口
		public := storefront.Group("/public")
		{
			public.GET("/config", publicHandler.GetConfig)
			public.GET("/products", publicHandler.GetProducts)
			public.GET("/products/:slug", publicHandler.GetProductBySlug)
			public.GET("/posts", publicHandler.GetPosts)
			public.GET("/posts/:slug", publicHandler.GetPostBySlug)
			public.GET("/banners", publicHandler.GetPublicBanners)
			public.GET("/categories", publicHandler.GetCategories)
			public.GET("/captcha/image", publicHandler.GetImageCaptcha)
			public.POST("/affiliate/click", publicHandler.TrackAffiliateClick)
			public.GET("/member-levels", publicHandler.GetPublicMemberLevels)
		}

		// 游客接口
		guest := storefront.Group("/guest")
		{
			guest.POST("/orders", publicHandler.CreateGuestOrder)
			guest.POST("/orders/create-and-pay", publicHandler.CreateGuestOrderAndPay)
			guest.POST("/orders/preview", publicHandler.PreviewGuestOrder)
			guest.GET("/orders", publicHandler.ListGuestOrders)
			guest.GET("/orders/:order_no", publicHandler.GetGuestOrderByOrderNo)
			guest.GET("/orders/:order_no/fulfillment/download", publicHandler.DownloadGuestFulfillment)
			guest.POST("/payments", publicHandler.CreateGuestPayment)
			guest.POST("/payments/:id/capture", publicHandler.CaptureGuestPayment)
			guest.GET("/payments/latest", publicHandler.GetGuestLatestPayment)
		}

		// 用户认证接口
		auth := storefront.Group("/auth")
		{
			auth.POST("/send-verify-code", publicHandler.SendUserVerifyCode)
			auth.POST("/register", publicHandler.UserRegister)
			auth.POST("/login", RateLimitMiddleware(redisClient, loginRule, KeyByIPAndJSONField("email")), publicHandler.UserLogin)
			auth.POST("/login/verify-2fa", RateLimitMiddleware(redisClient, loginRule, KeyByIP), publicHandler.VerifyUser2FA)
			auth.POST("/telegram/login", RateLimitMiddleware(redisClient, loginRule, KeyByIP), publicHandler.UserTelegramLogin)
			auth.POST("/telegram/miniapp/login", RateLimitMiddleware(redisClient, loginRule, KeyByIP), publicHandler.UserTelegramMiniAppLogin)
			auth.GET("/telegram/oidc/start", RateLimitMiddleware(redisClient, loginRule, KeyByIP), publicHandler.StartTelegramOIDCLogin)
			auth.POST("/telegram/oidc/callback", RateLimitMiddleware(redisClient, loginRule, KeyByIP), publicHandler.TelegramOIDCLoginCallback)
			auth.POST("/forgot-password", publicHandler.UserForgotPassword)
		}

		// 用户接口（需鉴权）
		user := storefront.Group("")
		user.Use(UserJWTAuthMiddleware(cfg.UserJWT.SecretKey, c.UserRepo))
		{
			user.GET("/me", publicHandler.GetCurrentUser)
			user.GET("/me/login-logs", publicHandler.GetMyLoginLogs)
			user.PUT("/me/profile", publicHandler.UpdateUserProfile)
			user.PUT("/me/password", publicHandler.ChangeUserPassword)
			user.GET("/me/telegram", publicHandler.GetMyTelegramBinding)
			user.POST("/me/telegram/bind", publicHandler.BindMyTelegram)
			user.POST("/me/telegram/miniapp/bind", publicHandler.BindMyTelegramMiniApp)
			user.GET("/me/telegram/oidc/start", publicHandler.StartTelegramOIDCBind)
			user.POST("/me/telegram/oidc/callback", publicHandler.TelegramOIDCBindCallback)
			user.DELETE("/me/telegram/unbind", publicHandler.UnbindMyTelegram)
			user.POST("/me/email/send-verify-code", publicHandler.SendChangeEmailCode)
			user.POST("/me/email/change", publicHandler.ChangeEmail)
			user.GET("/me/2fa/status", publicHandler.GetUser2FAStatus)
			user.POST("/me/2fa/setup", publicHandler.SetupUser2FA)
			user.POST("/me/2fa/enable", publicHandler.EnableUser2FA)
			user.POST("/me/2fa/disable", publicHandler.DisableUser2FA)
			user.POST("/me/2fa/recovery-codes/regenerate", publicHandler.RegenerateUser2FARecoveryCodes)
			user.GET("/cart", publicHandler.GetCart)
			user.POST("/cart/items", publicHandler.UpsertCartItem)
			user.DELETE("/cart/items/:product_id", publicHandler.DeleteCartItem)
			user.POST("/orders", publicHandler.CreateOrder)
			user.POST("/orders/create-and-pay", publicHandler.CreateOrderAndPay)
			user.POST("/orders/preview", publicHandler.PreviewOrder)
			user.POST("/order/payment-channels", publicHandler.GetOrderPaymentChannels)
			user.GET("/orders", publicHandler.ListOrders)
			user.GET("/orders/stats", publicHandler.OrderStats)
			user.GET("/orders/:order_no", publicHandler.GetOrderByOrderNo)
			user.GET("/orders/:order_no/fulfillment/download", publicHandler.DownloadFulfillment)
			user.POST("/orders/:order_no/cancel", publicHandler.CancelOrder)
			user.POST("/payments", publicHandler.CreatePayment)
			user.POST("/payments/:id/capture", publicHandler.CapturePayment)
			user.GET("/payments/latest", publicHandler.GetLatestPayment)
			user.GET("/wallet", publicHandler.GetMyWallet)
			user.GET("/wallet/transactions", publicHandler.GetMyWalletTransactions)
			user.POST("/wallet/payment-channels", publicHandler.GetMyWalletPaymentChannels)
			user.POST("/wallet/recharge", publicHandler.RechargeWallet)
			user.GET("/wallet/recharges", publicHandler.ListMyWalletRecharges)
			user.GET("/wallet/recharges/stats", publicHandler.MyWalletRechargeStats)
			user.GET("/wallet/recharges/:recharge_no", publicHandler.GetMyWalletRecharge)
			user.POST("/wallet/recharge/payments/:id/capture", publicHandler.CaptureMyWalletRechargePayment)
			user.POST("/gift-cards/redeem", publicHandler.RedeemGiftCard)
			user.POST("/affiliate/open", publicHandler.OpenAffiliate)
			user.GET("/affiliate/dashboard", publicHandler.GetAffiliateDashboard)
			user.GET("/affiliate/commissions", publicHandler.ListAffiliateCommissions)
			user.GET("/affiliate/withdraws", publicHandler.ListAffiliateWithdraws)
			user.POST("/affiliate/withdraws", publicHandler.ApplyAffiliateWithdraw)
			resellerConsole := user.Group("/reseller")
			resellerConsole.Use(RequireMainTenantForResellerConsole())
			{
				resellerConsole.GET("/profile", publicHandler.GetResellerManagementSnapshot)
				resellerConsole.POST("/apply", publicHandler.ApplyResellerProfile)
				resellerConsole.GET("/domains", publicHandler.ListResellerDomains)
				resellerConsole.POST("/domains", publicHandler.SubmitResellerCustomDomain)
				resellerConsole.GET("/site-config", publicHandler.GetResellerSiteConfig)
				resellerConsole.PUT("/site-config", publicHandler.UpdateResellerSiteConfig)
				resellerConsole.POST("/upload", publicHandler.UploadResellerImage)
				resellerConsole.GET("/product-settings", publicHandler.ListResellerProductSettings)
				resellerConsole.GET("/product-settings/:product_id", publicHandler.GetResellerProductSetting)
				resellerConsole.POST("/product-settings/:product_id/preview", publicHandler.PreviewResellerProductSettings)
				resellerConsole.PUT("/product-settings/:product_id", publicHandler.UpdateResellerProductSettings)
				resellerConsole.DELETE("/product-settings/:product_id", publicHandler.ResetResellerProductSetting)
				resellerConsole.GET("/dashboard", publicHandler.GetResellerDashboard)
				resellerConsole.GET("/orders", publicHandler.ListResellerOrders)
				resellerConsole.GET("/orders/stats", publicHandler.GetResellerOrderStats)
				resellerConsole.GET("/orders/:order_no", publicHandler.GetResellerOrderDetail)
				resellerConsole.GET("/balance-accounts", publicHandler.ListResellerBalanceAccounts)
				resellerConsole.GET("/ledger-entries", publicHandler.ListResellerLedgerEntries)
				resellerConsole.GET("/withdraws", publicHandler.ListResellerWithdraws)
				resellerConsole.POST("/withdraws", publicHandler.ApplyResellerWithdraw)
			}

			// API 对接权限（用户中心）
			user.GET("/api-credential", publicHandler.GetMyApiCredential)
			user.POST("/api-credential/apply", publicHandler.ApplyApiCredential)
			user.POST("/api-credential/regenerate", publicHandler.RegenerateMyApiCredential)
			user.PUT("/api-credential/status", publicHandler.UpdateMyApiCredentialStatus)
		}

		// 上游 API（本站作为 B 站点，暴露给下游 A 调用）
		upstreamAPI := apiV1.Group("/upstream")
		upstreamAPI.Use(RateLimitMiddleware(redisClient, upstreamAPIRule, KeyByUpstreamApiKey))
		upstreamAPI.Use(UpstreamAPIAuthMiddleware(c.ApiCredentialRepo))
		{
			upstreamAPI.POST("/ping", upstreamHandler.Ping)
			upstreamAPI.GET("/categories", upstreamHandler.ListCategories)
			upstreamAPI.GET("/products", upstreamHandler.ListProducts)
			upstreamAPI.GET("/products/:id", upstreamHandler.GetProduct)
			upstreamAPI.POST("/orders", upstreamHandler.CreateOrder)
			upstreamAPI.GET("/orders/:id", upstreamHandler.GetOrder)
			upstreamAPI.POST("/orders/:id/cancel", upstreamHandler.CancelOrder)
		}

		// 上游回调接收（本站作为 A 站点，接收 B 的回调）
		apiV1.POST("/upstream/callback", upstreamHandler.HandleCallback)

		// 渠道 API（Telegram Bot 等外部服务调用）
		channelAPI := apiV1.Group("/channel")
		channelAPI.Use(ChannelAPIAuthMiddleware(c))
		{
			channelAPI.GET("/telegram/config", channelHandler.GetBotConfig)
			channelAPI.POST("/telegram/heartbeat", channelHandler.ReportHeartbeat)
			channelAPI.POST("/identities/telegram/resolve", channelHandler.ResolveTelegramIdentity)
			channelAPI.POST("/identities/telegram/provision", channelHandler.ProvisionTelegramIdentity)
			channelAPI.POST("/identities/telegram/bind", channelHandler.BindTelegramIdentity)
			channelAPI.GET("/me", channelHandler.GetCurrentIdentity)
			channelAPI.POST("/affiliate/click", channelHandler.TrackAffiliateClick)
			channelAPI.POST("/affiliate/open", channelHandler.OpenAffiliate)
			channelAPI.GET("/affiliate/dashboard", channelHandler.GetAffiliateDashboard)
			channelAPI.GET("/affiliate/commissions", channelHandler.ListAffiliateCommissions)
			channelAPI.GET("/affiliate/withdraws", channelHandler.ListAffiliateWithdraws)
			channelAPI.POST("/affiliate/withdraws", channelHandler.ApplyAffiliateWithdraw)

			// Catalog 端点（商品浏览）
			channelAPI.GET("/catalog/categories", channelHandler.GetCategories)
			channelAPI.GET("/catalog/products", channelHandler.GetProducts)
			channelAPI.GET("/catalog/products/:id", channelHandler.GetProductDetail)
			channelAPI.GET("/member-levels", channelHandler.GetMemberLevels)

			// Order / Payment 端点（购买流程）
			channelAPI.POST("/orders/preview", channelHandler.PreviewOrder)
			channelAPI.POST("/orders", channelHandler.CreateOrder)
			channelAPI.GET("/orders", channelHandler.ListOrders)
			channelAPI.GET("/orders/by-order-no/:order_no", channelHandler.GetOrderByOrderNo)
			channelAPI.GET("/orders/:id", channelHandler.GetOrderStatus)
			channelAPI.POST("/orders/:id/cancel", channelHandler.CancelOrder)
			channelAPI.GET("/payment-channels", channelHandler.GetPaymentChannels)
			channelAPI.GET("/payment-methods", channelHandler.GetPaymentChannels)
			channelAPI.GET("/payments/latest", channelHandler.GetLatestPayment)
			channelAPI.GET("/payments/:id", channelHandler.GetPaymentDetail)
			channelAPI.POST("/payments", channelHandler.CreatePayment)

			// Wallet 端点（钱包）
			channelAPI.GET("/wallet", channelHandler.GetWallet)
			channelAPI.GET("/wallet/transactions", channelHandler.GetWalletTransactions)
			channelAPI.POST("/wallet/gift-card/redeem", channelHandler.RedeemGiftCard)
			channelAPI.POST("/wallet/recharge", channelHandler.CreateWalletRecharge)
		}

		apiV1.POST("/payments/callback", publicHandler.PaymentCallback)
		apiV1.GET("/payments/callback", publicHandler.PaymentCallback)
		apiV1.POST("/payments/webhook/dujiaopay", publicHandler.DujiaoPayWebhook)
		apiV1.POST("/payments/webhook/paypal", publicHandler.PaypalWebhook)
		apiV1.POST("/payments/webhook/stripe", publicHandler.StripeWebhook)

		// 管理员接口
		admin := apiV1.Group("/admin")
		{
			// 登录接口（无需鉴权）
			admin.POST("/login", RateLimitMiddleware(redisClient, adminLoginRule, KeyByIP), adminHandler.AdminLogin)
			admin.POST("/login/verify-2fa", RateLimitMiddleware(redisClient, adminLoginRule, KeyByIP), adminHandler.Verify2FA)

			// 需要鉴权的接口
			authorized := admin.Use(JWTAuthMiddleware(cfg.JWT.SecretKey, c.AdminRepo), AdminRBACMiddleware(c.AuthzService))
			// 支付/财务相关受保护子组：未确认合规声明时拦截
			// 注：admin.Use(...) 已 mutate admin 自身，新 Group 继承 JWT + RBAC 中间件
			paymentProtected := admin.Group("", PaymentComplianceRequired(c.ComplianceService))
			{
				// 合规声明
				authorized.GET("/compliance/status", adminHandler.GetComplianceStatus)
				authorized.POST("/compliance/acknowledge", adminHandler.AcknowledgeCompliance)

				// 仪表盘
				authorized.GET("/dashboard/overview", adminHandler.GetDashboardOverview)
				authorized.GET("/dashboard/trends", adminHandler.GetDashboardTrends)
				authorized.GET("/dashboard/rankings", adminHandler.GetDashboardRankings)
				authorized.GET("/dashboard/inventory-alerts", adminHandler.GetDashboardInventoryAlerts)

				// 广告代理
				authorized.GET("/ads/render/:slotCode", adminHandler.GetAdRender)
				authorized.POST("/ads/impression", adminHandler.PostAdImpression)

				// 商品管理
				authorized.GET("/products", adminHandler.GetAdminProducts)
				authorized.GET("/products/:id", adminHandler.GetAdminProduct)
				authorized.POST("/products", adminHandler.CreateProduct)
				authorized.PUT("/products/:id", adminHandler.UpdateProduct)
				authorized.PATCH("/products/:id/wholesale-prices", adminHandler.UpdateProductWholesalePrices)
				authorized.PATCH("/products/:id", adminHandler.QuickUpdateProduct)
				authorized.DELETE("/products/:id", adminHandler.DeleteProduct)
				authorized.POST("/products/batch-status", adminHandler.BatchUpdateProductStatus)
				authorized.POST("/products/batch-category", adminHandler.BatchUpdateProductCategory)
				authorized.POST("/products/batch-delete", adminHandler.BatchDeleteProducts)

				// 文章管理
				authorized.GET("/posts", adminHandler.GetAdminPosts)
				authorized.POST("/posts", adminHandler.CreatePost)
				authorized.PUT("/posts/:id", adminHandler.UpdatePost)
				authorized.DELETE("/posts/:id", adminHandler.DeletePost)
				authorized.GET("/posts/:id/products", adminHandler.GetAdminPostProductIDs)

				// Banner 管理
				authorized.GET("/banners", adminHandler.GetAdminBanners)
				authorized.GET("/banners/:id", adminHandler.GetAdminBanner)
				authorized.POST("/banners", adminHandler.CreateBanner)
				authorized.PUT("/banners/:id", adminHandler.UpdateBanner)
				authorized.DELETE("/banners/:id", adminHandler.DeleteBanner)

				// 分类管理
				authorized.GET("/categories", adminHandler.GetAdminCategories)
				authorized.POST("/categories", adminHandler.CreateCategory)
				authorized.PUT("/categories/:id", adminHandler.UpdateCategory)
				authorized.PATCH("/categories/:id/active", adminHandler.PatchCategoryActive)
				authorized.DELETE("/categories/:id", adminHandler.DeleteCategory)

				// 设置管理
				authorized.GET("/settings", adminHandler.GetSettings)
				authorized.PUT("/settings", adminHandler.UpdateSettings)
				authorized.GET("/settings/smtp", adminHandler.GetSMTPSettings)
				authorized.PUT("/settings/smtp", adminHandler.UpdateSMTPSettings)
				authorized.POST("/settings/smtp/test", adminHandler.TestSMTPSettings)
				authorized.GET("/settings/captcha", adminHandler.GetCaptchaSettings)
				authorized.PUT("/settings/captcha", adminHandler.UpdateCaptchaSettings)
				authorized.GET("/settings/telegram-auth", adminHandler.GetTelegramAuthSettings)
				authorized.PUT("/settings/telegram-auth", adminHandler.UpdateTelegramAuthSettings)
				authorized.GET("/settings/notification-center", adminHandler.GetNotificationCenterSettings)
				authorized.PUT("/settings/notification-center", adminHandler.UpdateNotificationCenterSettings)
				authorized.GET("/settings/notification-center/logs", adminHandler.ListNotificationLogs)
				authorized.POST("/settings/notification-center/test", adminHandler.TestNotificationCenterSettings)
				authorized.GET("/settings/notifications", adminHandler.GetNotificationCenterSettings)
				authorized.PUT("/settings/notifications", adminHandler.UpdateNotificationCenterSettings)
				authorized.GET("/settings/notifications/logs", adminHandler.ListNotificationLogs)
				authorized.POST("/settings/notifications/test", adminHandler.TestNotificationCenterSettings)
				authorized.GET("/settings/order-email-template", adminHandler.GetOrderEmailTemplateSettings)
				authorized.PUT("/settings/order-email-template", adminHandler.UpdateOrderEmailTemplateSettings)
				authorized.POST("/settings/order-email-template/reset", adminHandler.ResetOrderEmailTemplateSettings)
				authorized.GET("/settings/affiliate", adminHandler.GetAffiliateSettings)
				authorized.PUT("/settings/affiliate", adminHandler.UpdateAffiliateSettings)
				authorized.PUT("/password", adminHandler.UpdateAdminPassword) // 修改密码

				// 系统信息与版本检测
				authorized.GET("/system/version/check", adminHandler.CheckSystemUpdate)

				authorized.GET("/2fa/status", adminHandler.Get2FAStatus)
				authorized.POST("/2fa/setup", adminHandler.Setup2FA)
				authorized.POST("/2fa/enable", adminHandler.Enable2FA)
				authorized.POST("/2fa/disable", adminHandler.Disable2FA)
				authorized.POST("/2fa/recovery-codes/regenerate", adminHandler.RegenerateRecoveryCodes)

				// 推广返利
				authorized.GET("/affiliates/users", adminHandler.ListAffiliateUsers)
				authorized.PATCH("/affiliates/users/:id/status", adminHandler.UpdateAffiliateUserStatus)
				authorized.PATCH("/affiliates/users/batch-status", adminHandler.BatchUpdateAffiliateUserStatus)
				paymentProtected.GET("/affiliates/commissions", adminHandler.ListAffiliateCommissions)
				paymentProtected.GET("/affiliates/withdraws", adminHandler.ListAffiliateWithdraws)
				paymentProtected.POST("/affiliates/withdraws/:id/reject", adminHandler.RejectAffiliateWithdraw)
				paymentProtected.POST("/affiliates/withdraws/:id/pay", adminHandler.PayAffiliateWithdraw)
				authorized.GET("/resellers/operations/overview", adminHandler.GetResellerOperationsOverview)
				authorized.GET("/resellers/profiles", adminHandler.ListResellerProfiles)
				authorized.GET("/resellers/profiles/:id", adminHandler.GetResellerProfileDetail)
				authorized.PUT("/resellers/profiles/:id", adminHandler.UpdateResellerProfile)
				authorized.PUT("/resellers/profiles/:id/system-domain", adminHandler.AssignResellerSystemDomain)
				authorized.POST("/resellers/profiles/:id/approve", adminHandler.ApproveResellerProfile)
				authorized.POST("/resellers/profiles/:id/reject", adminHandler.RejectResellerProfile)
				authorized.POST("/resellers/profiles/:id/disable", adminHandler.DisableResellerProfile)
				authorized.POST("/resellers/profiles/:id/restore", adminHandler.RestoreResellerProfile)
				authorized.GET("/resellers/domains", adminHandler.ListResellerDomains)
				authorized.POST("/resellers/domains/:id/approve", adminHandler.ApproveResellerDomain)
				authorized.POST("/resellers/domains/:id/disable", adminHandler.DisableResellerDomain)
				authorized.POST("/resellers/domains/:id/set-primary", adminHandler.SetPrimaryResellerDomain)
				authorized.GET("/resellers/site-configs", adminHandler.ListResellerSiteConfigs)
				authorized.GET("/resellers/site-configs/:reseller_id", adminHandler.GetResellerSiteConfig)
				authorized.PUT("/resellers/site-configs/:reseller_id", adminHandler.UpdateResellerSiteConfig)
				authorized.POST("/resellers/site-configs/:reseller_id/reset", adminHandler.ResetResellerSiteConfig)
				authorized.GET("/resellers/product-settings", adminHandler.ListResellerProductSettings)
				authorized.GET("/resellers/product-settings/:reseller_id/:product_id", adminHandler.GetResellerProductSetting)
				authorized.POST("/resellers/product-settings/:reseller_id/:product_id/preview", adminHandler.PreviewResellerProductSettings)
				authorized.PUT("/resellers/product-settings/:reseller_id/:product_id", adminHandler.UpdateResellerProductSettings)
				authorized.DELETE("/resellers/product-settings/:reseller_id/:product_id", adminHandler.ResetResellerProductSetting)
				paymentProtected.GET("/resellers/operations/finance", adminHandler.GetResellerOperationsFinance)
				paymentProtected.GET("/resellers/ledger-entries", adminHandler.ListResellerLedgerEntries)
				paymentProtected.GET("/resellers/balance-accounts", adminHandler.ListResellerBalanceAccounts)
				paymentProtected.GET("/resellers/withdraws", adminHandler.ListResellerWithdraws)
				paymentProtected.POST("/resellers/withdraws/:id/reject", adminHandler.RejectResellerWithdraw)
				paymentProtected.POST("/resellers/withdraws/:id/pay", adminHandler.PayResellerWithdraw)

				// 权限管理
				authorized.GET("/authz/me", adminHandler.GetAuthzMe)
				authorized.GET("/authz/roles", adminHandler.ListAuthzRoles)
				authorized.GET("/authz/admins", adminHandler.ListAuthzAdmins)
				authorized.GET("/authz/audit-logs", adminHandler.ListAuthzAuditLogs)
				authorized.POST("/authz/admins", adminHandler.CreateAuthzAdmin)
				authorized.PUT("/authz/admins/:id", adminHandler.UpdateAuthzAdmin)
				authorized.POST("/authz/admins/:id/2fa/reset", adminHandler.ResetTargetAdmin2FA)
				authorized.DELETE("/authz/admins/:id", adminHandler.DeleteAuthzAdmin)
				authorized.GET("/authz/permissions/catalog", func(ctx *gin.Context) {
					response.Success(ctx, buildAdminPermissionCatalog(r))
				})
				authorized.POST("/authz/roles", adminHandler.CreateAuthzRole)
				authorized.DELETE("/authz/roles/:role", adminHandler.DeleteAuthzRole)
				authorized.GET("/authz/roles/:role/policies", adminHandler.GetAuthzRolePolicies)
				authorized.POST("/authz/policies", adminHandler.GrantAuthzPolicy)
				authorized.DELETE("/authz/policies", adminHandler.RevokeAuthzPolicy)
				authorized.GET("/authz/admins/:id/roles", adminHandler.GetAuthzAdminRoles)
				authorized.PUT("/authz/admins/:id/roles", adminHandler.SetAuthzAdminRoles)

				// 文件上传
				authorized.POST("/upload", adminHandler.UploadFile)

				// 素材管理
				authorized.GET("/media", adminHandler.GetAdminMedia)
				authorized.POST("/media/batch-delete", adminHandler.BatchDeleteMedia)
				authorized.PUT("/media/:id", adminHandler.UpdateMedia)
				authorized.DELETE("/media/:id", adminHandler.DeleteMedia)

				// 订单管理
				authorized.GET("/orders", adminHandler.AdminListOrders)
				authorized.GET("/orders/:id", adminHandler.AdminGetOrder)
				authorized.GET("/orders/:id/fulfillment/download", adminHandler.AdminDownloadFulfillment)
				authorized.PATCH("/orders/:id", adminHandler.AdminUpdateOrderStatus)
				authorized.POST("/orders/:id/refund-to-wallet", adminHandler.AdminRefundOrderToWallet)
				authorized.POST("/orders/:id/manual-refund", adminHandler.AdminManualRefundOrder)
				authorized.GET("/order-refunds", adminHandler.GetAdminOrderRefunds)
				authorized.GET("/order-refunds/:id", adminHandler.GetAdminOrderRefund)
				authorized.POST("/fulfillments", adminHandler.AdminCreateFulfillment)
				authorized.POST("/card-secrets/batch", adminHandler.CreateCardSecretBatch)
				authorized.POST("/card-secrets/import", adminHandler.ImportCardSecretCSV)
				authorized.GET("/card-secrets", adminHandler.GetCardSecrets)
				authorized.PUT("/card-secrets/:id", adminHandler.UpdateCardSecret)
				authorized.PATCH("/card-secrets/batch-status", adminHandler.BatchUpdateCardSecretStatus)
				authorized.POST("/card-secrets/batch-delete", adminHandler.BatchDeleteCardSecrets)
				authorized.POST("/card-secrets/export", adminHandler.ExportCardSecrets)
				authorized.POST("/card-secrets/export-available", adminHandler.ExportAvailableCardSecrets)
				authorized.GET("/card-secrets/stats", adminHandler.GetCardSecretStats)
				authorized.GET("/card-secrets/batches", adminHandler.GetCardSecretBatches)
				authorized.GET("/card-secrets/template", adminHandler.GetCardSecretTemplate)
				authorized.POST("/gift-cards/generate", adminHandler.GenerateGiftCards)
				authorized.GET("/gift-cards", adminHandler.GetGiftCards)
				authorized.PUT("/gift-cards/:id", adminHandler.UpdateGiftCard)
				authorized.DELETE("/gift-cards/:id", adminHandler.DeleteGiftCard)
				authorized.PATCH("/gift-cards/batch-status", adminHandler.BatchUpdateGiftCardStatus)
				authorized.POST("/gift-cards/export", adminHandler.ExportGiftCards)

				// 优惠券与活动价
				authorized.POST("/coupons", adminHandler.CreateCoupon)
				authorized.GET("/coupons", adminHandler.GetAdminCoupons)
				authorized.PUT("/coupons/:id", adminHandler.UpdateCoupon)
				authorized.DELETE("/coupons/:id", adminHandler.DeleteCoupon)
				authorized.POST("/promotions", adminHandler.CreatePromotion)
				authorized.GET("/promotions", adminHandler.GetAdminPromotions)
				authorized.PUT("/promotions/:id", adminHandler.UpdatePromotion)
				authorized.DELETE("/promotions/:id", adminHandler.DeletePromotion)

				// 会员等级
				authorized.GET("/member-levels", adminHandler.GetAdminMemberLevels)
				authorized.POST("/member-levels", adminHandler.CreateMemberLevel)
				authorized.PUT("/member-levels/:id", adminHandler.UpdateMemberLevel)
				authorized.DELETE("/member-levels/:id", adminHandler.DeleteMemberLevel)
				authorized.GET("/member-level-prices", adminHandler.GetMemberLevelPrices)
				authorized.POST("/member-level-prices/batch", adminHandler.BatchUpsertMemberLevelPrices)
				authorized.DELETE("/member-level-prices/:id", adminHandler.DeleteMemberLevelPrice)
				authorized.POST("/member-levels/backfill", adminHandler.BackfillMemberLevels)

				// 支付渠道与支付记录
				paymentProtected.POST("/payment-channels", adminHandler.CreatePaymentChannel)
				paymentProtected.GET("/payment-channels", adminHandler.GetPaymentChannels)
				paymentProtected.GET("/payment-channels/:id", adminHandler.GetPaymentChannel)
				paymentProtected.PUT("/payment-channels/:id", adminHandler.UpdatePaymentChannel)
				paymentProtected.DELETE("/payment-channels/:id", adminHandler.DeletePaymentChannel)
				paymentProtected.GET("/payments", adminHandler.GetAdminPayments)
				paymentProtected.GET("/payments/export", adminHandler.ExportAdminPayments)
				paymentProtected.GET("/payments/:id", adminHandler.GetAdminPayment)

				// 用户管理
				authorized.GET("/users", adminHandler.GetAdminUsers)
				authorized.GET("/user-login-logs", adminHandler.GetUserLoginLogs)
				authorized.PUT("/users/batch-status", adminHandler.BatchUpdateUserStatus)
				authorized.DELETE("/users/:id/oauth/telegram", adminHandler.UnbindAdminUserTelegram)
				authorized.GET("/users/:id", adminHandler.GetAdminUser)
				authorized.PUT("/users/:id", adminHandler.UpdateAdminUser)
				authorized.GET("/users/:id/coupon-usages", adminHandler.GetAdminUserCouponUsages)
				paymentProtected.GET("/users/:id/wallet", adminHandler.GetAdminUserWallet)
				paymentProtected.GET("/users/:id/wallet/transactions", adminHandler.GetAdminUserWalletTransactions)
				paymentProtected.POST("/users/:id/wallet/adjust", adminHandler.AdjustAdminUserWallet)
				authorized.PUT("/users/:id/member-level", adminHandler.SetUserMemberLevel)
				authorized.DELETE("/users/:id/2fa", adminHandler.ResetUser2FA)
				paymentProtected.GET("/wallet/recharges", adminHandler.GetAdminWalletRecharges)

				// API 凭证审核管理
				authorized.GET("/api-credentials", adminHandler.GetApiCredentials)
				authorized.GET("/api-credentials/:id", adminHandler.GetApiCredential)
				authorized.POST("/api-credentials/:id/approve", adminHandler.ApproveApiCredential)
				authorized.POST("/api-credentials/:id/reject", adminHandler.RejectApiCredential)
				authorized.PUT("/api-credentials/:id/status", adminHandler.UpdateApiCredentialStatus)
				authorized.DELETE("/api-credentials/:id", adminHandler.DeleteApiCredential)

				// 站点对接连接管理
				authorized.GET("/site-connections", adminHandler.GetSiteConnections)
				authorized.GET("/site-connections/:id", adminHandler.GetSiteConnection)
				authorized.POST("/site-connections", adminHandler.CreateSiteConnection)
				authorized.PUT("/site-connections/:id", adminHandler.UpdateSiteConnection)
				authorized.DELETE("/site-connections/:id", adminHandler.DeleteSiteConnection)
				authorized.POST("/site-connections/:id/ping", adminHandler.PingSiteConnection)
				authorized.PUT("/site-connections/:id/status", adminHandler.UpdateSiteConnectionStatus)
				authorized.POST("/site-connections/:id/reapply-markup", adminHandler.ReapplyConnectionMarkup)

				// 商品映射管理
				authorized.GET("/product-mappings", adminHandler.GetProductMappings)
				authorized.GET("/product-mappings/:id", adminHandler.GetProductMapping)
				authorized.POST("/product-mappings/import", adminHandler.ImportUpstreamProduct)
				authorized.POST("/product-mappings/batch-import", adminHandler.BatchImportUpstreamProducts)
				authorized.POST("/product-mappings/:id/sync", adminHandler.SyncProductMapping)
				authorized.PUT("/product-mappings/:id/status", adminHandler.UpdateProductMappingStatus)
				authorized.DELETE("/product-mappings/:id", adminHandler.DeleteProductMapping)
				authorized.POST("/product-mappings/batch-sync", adminHandler.BatchSyncProductMappings)
				authorized.POST("/product-mappings/batch-status", adminHandler.BatchUpdateProductMappingStatus)
				authorized.POST("/product-mappings/batch-delete", adminHandler.BatchDeleteProductMappings)
				authorized.GET("/upstream-products", adminHandler.ListUpstreamProducts)
				authorized.GET("/upstream-categories", adminHandler.ListUpstreamCategories)
				authorized.POST("/product-mappings/batch-import-by-category", adminHandler.BatchImportByCategory)

				// 采购单管理
				authorized.GET("/procurement-orders", adminHandler.GetProcurementOrders)
				authorized.GET("/procurement-orders/stats", adminHandler.GetProcurementOrderStats)
				authorized.GET("/procurement-orders/:id", adminHandler.GetProcurementOrder)
				authorized.GET("/procurement-orders/:id/upstream-payload/download", adminHandler.DownloadProcurementUpstreamPayload)
				authorized.POST("/procurement-orders/:id/retry", adminHandler.RetryProcurementOrder)
				authorized.POST("/procurement-orders/:id/direct-submit", adminHandler.DirectSubmit)
				authorized.POST("/procurement-orders/:id/cancel", adminHandler.CancelProcurementOrder)

				// 对账管理
				paymentProtected.POST("/reconciliation/run", adminHandler.RunReconciliation)
				paymentProtected.GET("/reconciliation/jobs", adminHandler.GetReconciliationJobs)
				paymentProtected.GET("/reconciliation/jobs/:id", adminHandler.GetReconciliationJob)
				paymentProtected.PUT("/reconciliation/items/:id/resolve", adminHandler.ResolveReconciliationItem)

				// 渠道客户端管理
				authorized.GET("/channel-clients", adminHandler.ListChannelClients)
				authorized.POST("/channel-clients", adminHandler.CreateChannelClient)
				authorized.GET("/channel-clients/:id", adminHandler.GetChannelClient)
				authorized.PUT("/channel-clients/:id", adminHandler.UpdateChannelClient)
				authorized.PUT("/channel-clients/:id/status", adminHandler.UpdateChannelClientStatus)
				authorized.POST("/channel-clients/:id/reset-secret", adminHandler.ResetChannelClientSecret)
				authorized.DELETE("/channel-clients/:id", adminHandler.DeleteChannelClient)

				// Telegram Bot 群发
				authorized.GET("/telegram-bot/broadcasts", adminHandler.ListTelegramBroadcasts)
				authorized.GET("/telegram-bot/broadcasts/:id", adminHandler.GetTelegramBroadcast)
				authorized.POST("/telegram-bot/broadcasts", adminHandler.CreateTelegramBroadcast)
				authorized.GET("/telegram-bot/users", adminHandler.ListTelegramBroadcastUsers)

				// Telegram Bot 设置
				authorized.GET("/settings/telegram-bot", adminHandler.GetTelegramBotConfig)
				authorized.PUT("/settings/telegram-bot", adminHandler.UpdateTelegramBotConfig)
				authorized.GET("/settings/telegram-bot/runtime-status", adminHandler.GetTelegramBotRuntimeStatus)
			}
		}
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 嵌入式前端资源（仅在 -tags fullstack 构建时生效）
	if web.Enabled() {
		if err := web.ValidateAdminPath(cfg.Web.AdminPath); err != nil {
			log.Sugar().Fatalf("web.admin_path 配置错误: %v", err)
		}
		if err := web.RegisterAdmin(r, cfg.Web.AdminPath, web.AdminFS()); err != nil {
			log.Sugar().Fatalf("注册 admin SPA 失败: %v", err)
		}
		if err := web.RegisterUser(r, web.UserFS()); err != nil {
			log.Sugar().Fatalf("注册 user SPA 失败: %v", err)
		}
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

package httpserver

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSetupRouterDelegatesRouteDomains(t *testing.T) {
	routerDirectory := currentRouterDirectory(t)
	source := readRouterSource(t, filepath.Join(routerDirectory, "router.go"))

	for _, registration := range []string{
		"registerStorefrontRoutes(",
		"registerUpstreamRoutes(",
		"registerChannelRoutes(",
		"registerPaymentCallbackRoutes(",
		"registerAdminRoutes(",
	} {
		if !strings.Contains(source, registration) {
			t.Errorf("SetupRouter must delegate through %s", registration)
		}
	}

	for _, inlineGroup := range []string{
		`apiV1.Group("/upstream")`,
		`apiV1.Group("/channel")`,
		`apiV1.Group("/admin")`,
	} {
		if strings.Contains(source, inlineGroup) {
			t.Errorf("router.go must not retain inline domain group %s", inlineGroup)
		}
	}
}

func TestSetupRouterUserSPADisabledGuardsRegisterUser(t *testing.T) {
	source := readRouterSource(t, filepath.Join(currentRouterDirectory(t), "router.go"))

	if !strings.Contains(source, "cfg.Web.UserSPADisabled") {
		t.Error("router.go must wrap RegisterUser in cfg.Web.UserSPADisabled condition")
	}
}

func TestRouteDomainFilesPreserveTrustBoundaries(t *testing.T) {
	routerDirectory := currentRouterDirectory(t)
	tests := []struct {
		file     string
		required []string
	}{
		{
			file: "routes_storefront.go",
			required: []string{
				`storefront.Use(middleware.ResellerTenantMiddleware(`,
				`producthttp.RegisterPublicRoutes(public, publicCatalogHandler)`,
				`categoryhttp.RegisterPublicRoutes(public, publicCategoryHandler)`,
				`contenttransport.RegisterPublicRoutes(public, publicContentHandler)`,
				`captchatransport.RegisterPublicRoutes(public,`,
				`affiliatetransport.RegisterPublicRoutes(public, affiliateHandler)`,
				`affiliatetransport.RegisterUserRoutes(user, affiliateHandler)`,
				`user.Use(middleware.UserJWTAuthMiddleware(`,
				`resellerConsole.Use(middleware.RequireMainTenantForResellerConsole())`,
				`resellertransport.RegisterUserConsoleRoutes(resellerConsole, userResellerHandler)`,
				`resellertransport.RegisterUserProductSettingRoutes(resellerConsole, userResellerProductSettingHandler)`,
				`resellertransport.RegisterUserFinanceRoutes(resellerConsole, userResellerFinanceHandler)`,
				`resellertransport.RegisterUserOrderRoutes(resellerConsole, userResellerOrderHandler)`,
				`apicredentialtransport.RegisterUserRoutes(user, userApiCredentialHandler)`,
				`auditlogtransport.RegisterUserRoutes(user, userAuditLogHandler)`,
				`giftcardtransport.RegisterUserRoutes(user, userGiftCardHandler)`,
				`wallettransport.RegisterUserRoutes(user, userWalletHandler)`,
				`userauthtransport.RegisterUserProfileRoutes(user, userProfileHandler)`,
				`userauthtransport.RegisterUserEmailRoutes(user, userEmailHandler)`,
				`userauthtransport.RegisterUserVerifyAuthRoutes(auth, userVerifyHandler)`,
				`userauthtransport.RegisterUserRegisterAuthRoutes(auth, userLoginHandler)`,
				`userauthtransport.RegisterUserLoginAuthRoutes(auth, userLoginHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIPAndJSONField("email")))`,
				`userauthtransport.RegisterUser2FAAuthRoutes(auth, user2FAHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIP))`,
				`userauthtransport.RegisterUser2FARoutes(user, user2FAHandler)`,
				`userauthtransport.RegisterUserTelegramAuthRoutes(auth, userTelegramHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIP))`,
				`userauthtransport.RegisterUserTelegramRoutes(user, userTelegramHandler)`,
				`userauthtransport.RegisterUserTelegramOIDCAuthRoutes(auth, userTelegramOIDCHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIP))`,
				`userauthtransport.RegisterUserTelegramOIDCRoutes(user, userTelegramOIDCHandler)`,
				`userauthtransport.RegisterUserPasswordAuthRoutes(auth, userPasswordHandler)`,
				`userauthtransport.RegisterUserPasswordRoutes(user, userPasswordHandler)`,
				`memberleveltransport.RegisterPublicRoutes(public, publicMemberLevelHandler)`,
				`publicconfigtransport.RegisterPublicRoutes(public, publicConfigHandler)`,
				`carttransport.RegisterUserRoutes(user, userCartHandler)`,
				`ordertransport.RegisterUserReadRoutes(user, userOrderHandler)`,
				`ordertransport.RegisterUserCancelRoute(user, userOrderHandler)`,
				`ordertransport.RegisterUserPreviewRoute(user, orderPreviewHandler)`,
				`ordertransport.RegisterUserCreateRoute(user, orderCreateHandler)`,
				`ordertransport.RegisterUserCreateAndPayRoute(user, orderCreateHandler)`,
				`ordertransport.RegisterUserPaymentChannelsRoute(user, userOrderHandler)`,
				`ordertransport.RegisterGuestReadRoutes(guestRead, guestOrderHandler)`,
				`guestRead.Use(middleware.RateLimitMiddleware(redisClient, guestReadRule, middleware.KeyByIP))`,
				`ordertransport.RegisterGuestPreviewRoute(guestRead, orderPreviewHandler)`,
				`ordertransport.RegisterGuestCreateRoute(guestWrite, orderCreateHandler)`,
				`ordertransport.RegisterGuestCreateAndPayRoute(guestWrite, orderCreateHandler)`,
				`guestWrite.Use(middleware.RateLimitMiddleware(redisClient, guestWriteRule, middleware.KeyByIP))`,
				`paymenttransport.RegisterGuestWriteRoutes(guestWrite, paymentWriteHandler)`,
				`paymenttransport.RegisterGuestLatestRoute(guestRead, paymentLatestHandler)`,
				`paymenttransport.RegisterUserWriteRoutes(user, paymentWriteHandler)`,
				`paymenttransport.RegisterUserLatestRoute(user, paymentLatestHandler)`,
				`paymenttransport.RegisterWebhookRoutes(apiV1, webhookHandler)`,
			},
		},
		{
			file: "routes_upstream.go",
			required: []string{
				`upstreamAPI.Use(middleware.RateLimitMiddleware(`,
				`upstreamAPI.Use(middleware.UpstreamAPIAuthMiddleware(`,
				`apiV1.POST("/upstream/callback",`,
			},
		},
		{
			file: "routes_channel.go",
			required: []string{
				`channelAPI.Use(middleware.ChannelAPIAuthMiddleware(`,
				`telegramtransport.RegisterChannelBotRoutes(channelAPI, channelTelegramBotHandler)`,
				`affiliatetransport.RegisterChannelRoutes(channelAPI, channelAffiliateHandler)`,
				`memberleveltransport.RegisterChannelRoutes(channelAPI, channelMemberLevelHandler)`,
				`wallettransport.RegisterChannelRoutes(channelAPI, channelWalletHandler)`,
				`giftcardtransport.RegisterChannelRoutes(channelAPI, channelGiftCardHandler)`,
			},
		},
		{
			file: "routes_admin.go",
			required: []string{
				`adminauthtransport.RegisterAdminLoginAuthRoutes(admin, adminLoginHandler, middleware.RateLimitMiddleware(redisClient, adminLoginRule, middleware.KeyByIP))`,
				`adminauthtransport.RegisterAdmin2FAAuthRoutes(admin, admin2FAHandler, middleware.RateLimitMiddleware(redisClient, adminLoginRule, middleware.KeyByIP))`,
				`adminauthtransport.RegisterAdminPasswordRoutes(authorized, adminLoginHandler)`,
				`adminauthtransport.RegisterAdmin2FARoutes(authorized, admin2FAHandler)`,
				`adminauthtransport.RegisterAdminUser2FARoutes(authorized, adminUser2FAHandler)`,
				`adminusertransport.RegisterAdminRoutes(authorized, adminUserHandler)`,
				`adminauthztransport.RegisterAdminRoutes(authorized, adminAuthzHandler)`,
				`ordertransport.RegisterAdminRoutes(authorized, adminOrderHandler)`,
				`ordertransport.RegisterAdminRefundWriteRoutes(authorized, adminOrderRefundHandler)`,
				`ordertransport.RegisterAdminRefundRoutes(authorized, adminOrderRefundHandler)`,
				`fulfillmenttransport.RegisterAdminRoutes(authorized, adminFulfillmentHandler)`,
				`authorized := admin.Use(middleware.JWTAuthMiddleware(`,
				`paymentProtected := admin.Group("", middleware.PaymentComplianceRequired(`,
				`compliancetransport.RegisterAdminRoutes(authorized,`,
				`systemtransport.RegisterAdminRoutes(authorized,`,
				`adproxytransport.RegisterAdminRoutes(authorized,`,
				`contenttransport.RegisterAdminRoutes(authorized, adminContentHandler)`,
				`dashboardtransport.RegisterAdminRoutes(authorized, adminDashboardHandler)`,
				`memberleveltransport.RegisterAdminRoutes(authorized, adminMemberLevelHandler)`,
				`apicredentialtransport.RegisterAdminRoutes(authorized, adminApiCredentialHandler)`,
				`auditlogtransport.RegisterAdminRoutes(authorized, adminAuditLogHandler)`,
				`cardsecrettransport.RegisterAdminRoutes(authorized, adminCardSecretHandler)`,
				`giftcardtransport.RegisterAdminRoutes(authorized, adminGiftCardHandler)`,
				`settingstransport.RegisterAdminRoutes(authorized, adminSettingsHandler)`,
				`settingstransport.RegisterAdminSMTPRoutes(authorized,`,
				`settingstransport.RegisterAdminCaptchaRoutes(authorized,`,
				`settingstransport.RegisterAdminTelegramAuthRoutes(authorized,`,
				`settingstransport.RegisterAdminOrderEmailTemplateRoutes(authorized,`,
				`settingstransport.RegisterAdminAffiliateRoutes(authorized,`,
				`settingstransport.RegisterAdminTelegramBotRoutes(authorized,`,
				`uploadtransport.RegisterAdminRoutes(authorized,`,
				`broadcasthttp.RegisterAdminRoutes(authorized,`,
				`channelclienthttp.RegisterAdminRoutes(authorized,`,
				`siteconnectiontransport.RegisterAdminRoutes(authorized,`,
				`affiliatetransport.RegisterAdminRoutes(authorized, adminAffiliateHandler)`,
				`affiliatetransport.RegisterAdminFinanceRoutes(paymentProtected, adminAffiliateHandler)`,
				`producthttp.RegisterAdminRoutes(authorized, adminCatalogProductHandler)`,
				`categoryhttp.RegisterAdminRoutes(authorized, adminCatalogCategoryHandler)`,
				`mappinghttp.RegisterAdminRoutes(authorized, adminCatalogProductMappingHandler)`,
				`coupontransport.RegisterAdminRoutes(authorized, adminCouponHandler)`,
				`promotiontransport.RegisterAdminRoutes(authorized, adminPromotionHandler)`,
				`notificationtransport.RegisterAdminRoutes(authorized, adminNotificationHandler)`,
				`procurementtransport.RegisterAdminRoutes(authorized, adminProcurementHandler)`,
				`resellertransport.RegisterOperationsOverviewRoutes(authorized, adminResellerOperationsHandler)`,
				`resellertransport.RegisterManagementRoutes(authorized, adminResellerManagementHandler)`,
				`resellertransport.RegisterProfileDetailRoutes(authorized, adminResellerProfileDetailHandler)`,
				`resellertransport.RegisterSiteConfigRoutes(authorized, adminResellerSiteConfigHandler)`,
				`resellertransport.RegisterProductSettingRoutes(authorized, adminResellerProductSettingHandler)`,
				`resellertransport.RegisterOperationsFinanceRoutes(paymentProtected, adminResellerOperationsHandler)`,
				`resellertransport.RegisterFinanceRoutes(paymentProtected, adminResellerFinanceHandler)`,
				`paymenttransport.RegisterAdminChannelRoutes(paymentProtected, adminPaymentChannelHandler)`,
				`paymenttransport.RegisterAdminRoutes(paymentProtected, adminPaymentHandler)`,
				`wallettransport.RegisterAdminRoutes(paymentProtected, adminWalletHandler)`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			source := readRouterSource(t, filepath.Join(routerDirectory, test.file))
			for _, required := range test.required {
				if !strings.Contains(source, required) {
					t.Errorf("%s must preserve trust-boundary statement %q", test.file, required)
				}
			}
		})
	}
}

func currentRouterDirectory(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve router test filename")
	}
	return filepath.Dir(thisFile)
}

func readRouterSource(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read router source %s: %v", path, err)
	}
	return string(raw)
}

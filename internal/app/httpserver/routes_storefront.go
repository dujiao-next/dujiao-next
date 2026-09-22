package httpserver

import (
	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/app/httpserver/middleware"
	affiliatebootstrap "github.com/dujiao-next/internal/bootstrap/affiliate"
	"github.com/dujiao-next/internal/config"
	affiliatetransport "github.com/dujiao-next/internal/modules/affiliate/transport/http"
	apicredentialtransport "github.com/dujiao-next/internal/modules/apicredential/transport/http"
	auditlogtransport "github.com/dujiao-next/internal/modules/auditlog/transport/http"
	captchatransport "github.com/dujiao-next/internal/modules/captcha/transport/http"
	carttransport "github.com/dujiao-next/internal/modules/cart/transport/http"
	categoryhttp "github.com/dujiao-next/internal/modules/catalog/category/transport/http"
	producthttp "github.com/dujiao-next/internal/modules/catalog/product/transport/http"
	contenttransport "github.com/dujiao-next/internal/modules/content/transport/http"
	giftcardtransport "github.com/dujiao-next/internal/modules/giftcard/transport/http"
	userauthtransport "github.com/dujiao-next/internal/modules/identity/userauth/transport/http"
	memberleveltransport "github.com/dujiao-next/internal/modules/memberlevel/transport/http"
	ordertransport "github.com/dujiao-next/internal/modules/order/transport/http"
	paymenttransport "github.com/dujiao-next/internal/modules/payment/transport/http"
	paymentcallbacktransport "github.com/dujiao-next/internal/modules/payment/transport/http/callback"
	resellertransport "github.com/dujiao-next/internal/modules/reseller/transport/http/user"
	publicconfigtransport "github.com/dujiao-next/internal/modules/settings/transport/http/public"
	wallettransport "github.com/dujiao-next/internal/modules/wallet/transport/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func registerStorefrontRoutes(
	apiV1 *gin.RouterGroup,
	cfg *config.Config,
	c *container.Container,
	publicContentHandler *contenttransport.PublicHandler,
	publicCatalogHandler *producthttp.PublicHandler,
	publicCategoryHandler *categoryhttp.PublicHandler,
	userResellerHandler *resellertransport.UserHandler,
	userResellerProductSettingHandler *resellertransport.UserProductSettingHandler,
	userResellerFinanceHandler *resellertransport.UserFinanceHandler,
	userResellerOrderHandler *resellertransport.UserOrderHandler,
	userApiCredentialHandler *apicredentialtransport.UserHandler,
	userAuditLogHandler *auditlogtransport.UserHandler,
	userGiftCardHandler *giftcardtransport.UserHandler,
	publicMemberLevelHandler *memberleveltransport.PublicHandler,
	userProfileHandler *userauthtransport.UserProfileHandler,
	userEmailHandler *userauthtransport.UserEmailHandler,
	userPasswordHandler *userauthtransport.UserPasswordHandler,
	userVerifyHandler *userauthtransport.UserVerifyHandler,
	userTelegramOIDCHandler *userauthtransport.UserTelegramOIDCHandler,
	userTelegramHandler *userauthtransport.UserTelegramHandler,
	userGoogleHandler *userauthtransport.UserGoogleHandler,
	userGitHubHandler *userauthtransport.UserGitHubHandler,
	userLoginHandler *userauthtransport.UserLoginHandler,
	user2FAHandler *userauthtransport.User2FAHandler,
	publicConfigHandler *publicconfigtransport.Handler,
	userCartHandler *carttransport.UserHandler,
	userOrderHandler *ordertransport.UserHandler,
	guestOrderHandler *ordertransport.GuestHandler,
	orderPreviewHandler *ordertransport.PreviewHandler,
	orderCreateHandler *ordertransport.CreateHandler,
	paymentLatestHandler *paymenttransport.LatestHandler,
	paymentWriteHandler *paymenttransport.WriteHandler,
	userWalletHandler *wallettransport.UserHandler,
	redisClient *redis.Client,
	loginRule middleware.RateLimitRule,
	guestReadRule middleware.RateLimitRule,
	guestWriteRule middleware.RateLimitRule,
	giftCardRedeemRule middleware.RateLimitRule,
) {
	storefront := apiV1.Group("")
	storefront.Use(middleware.ResellerTenantMiddleware(c.ResellerDomainResolver))
	affiliateHandler := affiliatebootstrap.NewStorefrontHandler(c)

	// 公开接口
	public := storefront.Group("/public")
	{
		publicconfigtransport.RegisterPublicRoutes(public, publicConfigHandler)
		producthttp.RegisterPublicRoutes(public, publicCatalogHandler)
		categoryhttp.RegisterPublicRoutes(public, publicCategoryHandler)
		contenttransport.RegisterPublicRoutes(public, publicContentHandler)
		captchatransport.RegisterPublicRoutes(public, captchatransport.NewPublicHandler(c.CaptchaService))
		affiliatetransport.RegisterPublicRoutes(public, affiliateHandler)
		memberleveltransport.RegisterPublicRoutes(public, publicMemberLevelHandler)
	}

	// 游客接口
	guest := storefront.Group("/guest")
	guestRead := guest.Group("")
	guestRead.Use(middleware.RateLimitMiddleware(redisClient, guestReadRule, middleware.KeyByIP))
	{
		ordertransport.RegisterGuestPreviewRoute(guestRead, orderPreviewHandler)
		ordertransport.RegisterGuestReadRoutes(guestRead, guestOrderHandler)
		paymenttransport.RegisterGuestLatestRoute(guestRead, paymentLatestHandler)
	}
	guestWrite := guest.Group("")
	guestWrite.Use(middleware.RateLimitMiddleware(redisClient, guestWriteRule, middleware.KeyByIP))
	{
		ordertransport.RegisterGuestCreateRoute(guestWrite, orderCreateHandler)
		ordertransport.RegisterGuestCreateAndPayRoute(guestWrite, orderCreateHandler)
		paymenttransport.RegisterGuestWriteRoutes(guestWrite, paymentWriteHandler)
	}

	// 用户认证接口
	auth := storefront.Group("/auth")
	{
		userauthtransport.RegisterUserVerifyAuthRoutes(auth, userVerifyHandler)
		userauthtransport.RegisterUserRegisterAuthRoutes(auth, userLoginHandler)
		userauthtransport.RegisterUserLoginAuthRoutes(auth, userLoginHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIPAndJSONField("email")))
		userauthtransport.RegisterUser2FAAuthRoutes(auth, user2FAHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIP))
		userauthtransport.RegisterUserTelegramAuthRoutes(auth, userTelegramHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIP))
		userauthtransport.RegisterUserTelegramOIDCAuthRoutes(auth, userTelegramOIDCHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIP))
		userauthtransport.RegisterUserGoogleAuthRoutes(auth, userGoogleHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIP))
		userauthtransport.RegisterUserGitHubAuthRoutes(auth, userGitHubHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIP))
		userauthtransport.RegisterUserPasswordAuthRoutes(auth, userPasswordHandler)
	}

	// 用户接口（需鉴权）
	user := storefront.Group("")
	user.Use(middleware.UserJWTAuthMiddleware(cfg.UserJWT.SecretKey, c.UserStore))
	{
		userauthtransport.RegisterUserProfileRoutes(user, userProfileHandler)
		auditlogtransport.RegisterUserRoutes(user, userAuditLogHandler)
		userauthtransport.RegisterUserPasswordRoutes(user, userPasswordHandler)
		userauthtransport.RegisterUserTelegramRoutes(user, userTelegramHandler)
		userauthtransport.RegisterUserTelegramOIDCRoutes(user, userTelegramOIDCHandler)
		userauthtransport.RegisterUserGoogleRoutes(
			user,
			userGoogleHandler,
			middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByUserIDAndIP),
		)
		userauthtransport.RegisterUserEmailRoutes(user, userEmailHandler)
		userauthtransport.RegisterUser2FARoutes(user, user2FAHandler)
		carttransport.RegisterUserRoutes(user, userCartHandler)
		ordertransport.RegisterUserCreateRoute(user, orderCreateHandler)
		ordertransport.RegisterUserCreateAndPayRoute(user, orderCreateHandler)
		ordertransport.RegisterUserPreviewRoute(user, orderPreviewHandler)
		ordertransport.RegisterUserPaymentChannelsRoute(user, userOrderHandler)
		ordertransport.RegisterUserReadRoutes(user, userOrderHandler)
		ordertransport.RegisterUserCancelRoute(user, userOrderHandler)
		paymenttransport.RegisterUserWriteRoutes(user, paymentWriteHandler)
		paymenttransport.RegisterUserLatestRoute(user, paymentLatestHandler)
		wallettransport.RegisterUserRoutes(user, userWalletHandler)
		giftCardRedeem := user.Group("", middleware.RateLimitMiddleware(redisClient, giftCardRedeemRule, middleware.KeyByUserIDAndIP))
		giftcardtransport.RegisterUserRoutes(giftCardRedeem, userGiftCardHandler)
		affiliatetransport.RegisterUserRoutes(user, affiliateHandler)

		resellerConsole := user.Group("/reseller")
		resellerConsole.Use(middleware.RequireMainTenantForResellerConsole())
		{
			resellertransport.RegisterUserConsoleRoutes(resellerConsole, userResellerHandler)
			resellertransport.RegisterUserProductSettingRoutes(resellerConsole, userResellerProductSettingHandler)
			resellertransport.RegisterUserFinanceRoutes(resellerConsole, userResellerFinanceHandler)
			resellertransport.RegisterUserOrderRoutes(resellerConsole, userResellerOrderHandler)
		}

		// API 对接权限（用户中心）
		apicredentialtransport.RegisterUserRoutes(user, userApiCredentialHandler)
	}
}

func registerPaymentCallbackRoutes(apiV1 *gin.RouterGroup, callbackHandler *paymentcallbacktransport.Handler, webhookHandler *paymenttransport.WebhookHandler, redisClient *redis.Client, callbackRule middleware.RateLimitRule) {
	callbacks := apiV1.Group("", middleware.RateLimitMiddleware(redisClient, callbackRule, middleware.KeyByIP))
	paymentcallbacktransport.RegisterRoutes(callbacks, callbackHandler)
	paymenttransport.RegisterWebhookRoutes(callbacks, webhookHandler)
}

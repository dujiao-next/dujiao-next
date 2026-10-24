package paymenthttp

import "github.com/gin-gonic/gin"

// RegisterGuestLatestRoute 注册前台游客最新待支付查询路由。
func RegisterGuestLatestRoute(guest gin.IRoutes, handler *LatestHandler) {
	if guest == nil || handler == nil {
		panic("payment guest latest route: required dependency is nil")
	}
	guest.GET("/payments/latest", handler.GetGuestLatestPayment)
}

// RegisterUserLatestRoute 注册前台用户最新待支付查询路由。
func RegisterUserLatestRoute(user gin.IRoutes, handler *LatestHandler) {
	if user == nil || handler == nil {
		panic("payment user latest route: required dependency is nil")
	}
	user.GET("/payments/latest", handler.GetLatestPayment)
}

// RegisterGuestWriteRoutes 注册前台游客支付创建/捕获路由。
func RegisterGuestWriteRoutes(guest gin.IRoutes, handler *WriteHandler) {
	if guest == nil || handler == nil {
		panic("payment guest write routes: required dependency is nil")
	}
	guest.POST("/payments", handler.CreateGuestPayment)
	guest.POST("/payments/:id/capture", handler.CaptureGuestPayment)
}

// RegisterUserWriteRoutes 注册前台用户支付创建/捕获路由。
func RegisterUserWriteRoutes(user gin.IRoutes, handler *WriteHandler) {
	if user == nil || handler == nil {
		panic("payment user write routes: required dependency is nil")
	}
	user.POST("/payments", handler.CreatePayment)
	user.POST("/payments/:id/capture", handler.CapturePayment)
}

// RegisterAdminRoutes 注册后台支付只读路由。
func RegisterAdminRoutes(authorized gin.IRoutes, handler *AdminHandler) {
	if authorized == nil || handler == nil {
		panic("payment admin routes: required dependency is nil")
	}
	authorized.GET("/payments", handler.GetAdminPayments)
	authorized.GET("/payments/export", handler.ExportAdminPayments)
	authorized.GET("/payments/:id", handler.GetAdminPayment)
}

// RegisterAdminChannelRoutes 注册后台支付渠道路由。
func RegisterAdminChannelRoutes(authorized gin.IRoutes, handler *AdminChannelHandler) {
	if authorized == nil || handler == nil {
		panic("payment admin channel routes: required dependency is nil")
	}
	authorized.POST("/payment-channels", handler.CreatePaymentChannel)
	authorized.GET("/payment-channels", handler.GetPaymentChannels)
	authorized.GET("/payment-channels/:id", handler.GetPaymentChannel)
	authorized.POST("/payment-channels/:id/wechatpay-public-key-test", handler.TestWechatPayPublicKey)
	authorized.PUT("/payment-channels/:id", handler.UpdatePaymentChannel)
	authorized.DELETE("/payment-channels/:id", handler.DeletePaymentChannel)
}

// RegisterWebhookRoutes 注册支付 webhook 默认路由。
func RegisterWebhookRoutes(api gin.IRoutes, handler *WebhookHandler) {
	if api == nil || handler == nil {
		panic("payment webhook routes: required dependency is nil")
	}
	api.POST("/payments/webhook/dujiaopay", handler.DujiaoPayWebhook)
	api.POST("/payments/webhook/paypal", handler.PaypalWebhook)
	api.POST("/payments/webhook/stripe", handler.StripeWebhook)
}

// RegisterEpayRedirectRoute 注册易支付 v2 跳转表单。挂在回调限流组上，不要求用户登录。
func RegisterEpayRedirectRoute(api gin.IRoutes, handler *EpayRedirectHandler) {
	if api == nil || handler == nil {
		panic("epay redirect route: required dependency is nil")
	}
	api.GET("/payments/epay-redirect", handler.EpayRedirect)
}

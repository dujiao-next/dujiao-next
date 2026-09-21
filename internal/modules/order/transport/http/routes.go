package orderhttp

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册后台订单路由（列表/详情/状态更新）。
func RegisterAdminRoutes(authorized gin.IRoutes, handler *AdminHandler) {
	if authorized == nil || handler == nil {
		panic("order admin routes: required dependency is nil")
	}
	authorized.GET("/orders", handler.AdminListOrders)
	authorized.GET("/orders/:id", handler.AdminGetOrder)
	authorized.PATCH("/orders/:id", handler.AdminUpdateOrderStatus)
}

// RegisterAdminRefundRoutes 注册后台退款只读路由。
func RegisterAdminRefundRoutes(authorized gin.IRoutes, handler *AdminRefundHandler) {
	if authorized == nil || handler == nil {
		panic("order admin refund routes: required dependency is nil")
	}
	authorized.GET("/order-refunds", handler.GetAdminOrderRefunds)
	authorized.GET("/order-refunds/:id", handler.GetAdminOrderRefund)
}

// RegisterAdminRefundWriteRoutes 注册后台退款写路由。
func RegisterAdminRefundWriteRoutes(authorized gin.IRoutes, handler *AdminRefundHandler) {
	if authorized == nil || handler == nil {
		panic("order admin refund write routes: required dependency is nil")
	}
	authorized.POST("/orders/:id/refund-to-wallet", handler.AdminRefundOrderToWallet)
	authorized.POST("/orders/:id/manual-refund", handler.AdminManualRefundOrder)
	authorized.PATCH("/order-refunds/:id/payment-fee", handler.UpdateAdminOrderRefundPaymentFee)
}

// RegisterUserReadRoutes 注册前台用户订单只读路由。
func RegisterUserReadRoutes(user gin.IRoutes, handler *UserHandler) {
	if user == nil || handler == nil {
		panic("order user read routes: required dependency is nil")
	}
	user.GET("/orders", handler.ListOrders)
	user.GET("/orders/stats", handler.OrderStats)
	user.GET("/orders/:order_no", handler.GetOrderByOrderNo)
	user.GET("/orders/:order_no/fulfillment/download", handler.DownloadFulfillment)
}

// RegisterUserPaymentChannelsRoute 注册前台用户订单可用支付渠道查询路由。
func RegisterUserPaymentChannelsRoute(user gin.IRoutes, handler *UserHandler) {
	if user == nil || handler == nil {
		panic("order user payment channels route: required dependency is nil")
	}
	user.POST("/order/payment-channels", handler.GetOrderPaymentChannels)
}

// RegisterUserCancelRoute 注册前台用户取消订单路由。
func RegisterUserCancelRoute(user gin.IRoutes, handler *UserHandler) {
	if user == nil || handler == nil {
		panic("order user cancel route: required dependency is nil")
	}
	user.POST("/orders/:order_no/cancel", handler.CancelOrder)
}

// RegisterGuestReadRoutes 注册前台游客订单只读路由。
func RegisterGuestReadRoutes(guest gin.IRoutes, handler *GuestHandler) {
	if guest == nil || handler == nil {
		panic("order guest read routes: required dependency is nil")
	}
	guest.GET("/orders/browser", handler.ListBrowserOrders)
	guest.GET("/orders", handler.ListGuestOrders)
	guest.GET("/orders/:order_no", handler.GetGuestOrderByOrderNo)
	guest.GET("/orders/:order_no/fulfillment/download", handler.DownloadGuestFulfillment)
	guest.POST("/orders/:order_no/cancel", handler.CancelGuestOrder)
}

// RegisterUserPreviewRoute 注册前台用户订单预览路由。
func RegisterUserPreviewRoute(user gin.IRoutes, handler *PreviewHandler) {
	if user == nil || handler == nil {
		panic("order user preview route: required dependency is nil")
	}
	user.POST("/orders/preview", handler.PreviewOrder)
}

// RegisterGuestPreviewRoute 注册前台游客订单预览路由。
func RegisterGuestPreviewRoute(guest gin.IRoutes, handler *PreviewHandler) {
	if guest == nil || handler == nil {
		panic("order guest preview route: required dependency is nil")
	}
	guest.POST("/orders/preview", handler.PreviewGuestOrder)
}

// RegisterUserCreateRoute 注册前台用户创建订单路由。
func RegisterUserCreateRoute(user gin.IRoutes, handler *CreateHandler) {
	if user == nil || handler == nil {
		panic("order user create route: required dependency is nil")
	}
	user.POST("/orders", handler.CreateOrder)
}

// RegisterUserCreateAndPayRoute 注册前台用户创建订单并发起支付路由。
func RegisterUserCreateAndPayRoute(user gin.IRoutes, handler *CreateHandler) {
	if user == nil || handler == nil {
		panic("order user create-and-pay route: required dependency is nil")
	}
	user.POST("/orders/create-and-pay", handler.CreateOrderAndPay)
}

// RegisterGuestCreateRoute 注册前台游客创建订单路由。
func RegisterGuestCreateRoute(guest gin.IRoutes, handler *CreateHandler) {
	if guest == nil || handler == nil {
		panic("order guest create route: required dependency is nil")
	}
	guest.POST("/orders", handler.CreateGuestOrder)
}

// RegisterGuestCreateAndPayRoute 注册前台游客创建订单并发起支付路由。
func RegisterGuestCreateAndPayRoute(guest gin.IRoutes, handler *CreateHandler) {
	if guest == nil || handler == nil {
		panic("order guest create-and-pay route: required dependency is nil")
	}
	guest.POST("/orders/create-and-pay", handler.CreateGuestOrderAndPay)
}

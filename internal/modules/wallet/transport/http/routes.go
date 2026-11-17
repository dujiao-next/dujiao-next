package wallethttp

import "github.com/gin-gonic/gin"

// RegisterUserRoutes 注册用户钱包端点。
func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	if user == nil || handler == nil {
		panic("wallet user routes: required dependency is nil")
	}
	user.GET("/wallet", handler.GetWallet)
	user.GET("/wallet/transactions", handler.GetTransactions)
	user.POST("/wallet/payment-channels", handler.GetPaymentChannels)
	user.POST("/wallet/recharge", handler.Recharge)
	user.GET("/wallet/recharges", handler.ListRecharges)
	user.GET("/wallet/recharges/stats", handler.RechargeStats)
	user.GET("/wallet/recharges/:recharge_no", handler.GetRecharge)
	user.POST("/wallet/recharge/payments/:id/capture", handler.CaptureRechargePayment)
}

// RegisterAdminRoutes 注册后台钱包端点（须挂在 paymentProtected 分组）。
func RegisterAdminRoutes(paymentProtected gin.IRoutes, handler *AdminHandler) {
	if paymentProtected == nil || handler == nil {
		panic("wallet admin routes: required dependency is nil")
	}
	paymentProtected.GET("/users/:id/wallet", handler.GetUserWallet)
	paymentProtected.GET("/users/:id/wallet/transactions", handler.GetUserTransactions)
	paymentProtected.POST("/users/:id/wallet/adjust", handler.AdjustUserWallet)
	paymentProtected.GET("/wallet/recharges", handler.GetRecharges)
	paymentProtected.GET("/wallet/recharges/:id", handler.GetRechargeDetail)
	paymentProtected.POST("/wallet/recharges/:id/manual-confirm-payment", handler.ConfirmRecharge)
	paymentProtected.POST("/wallet/recharges/:id/manual-refund", handler.RefundRecharge)
}

// RegisterChannelRoutes 注册渠道钱包端点。
func RegisterChannelRoutes(channel gin.IRoutes, handler *ChannelHandler) {
	if channel == nil || handler == nil {
		panic("wallet channel routes: required dependency is nil")
	}
	channel.GET("/wallet", handler.GetWallet)
	channel.GET("/wallet/transactions", handler.GetWalletTransactions)
	channel.POST("/wallet/recharge", handler.CreateWalletRecharge)
}

package affiliatehttp

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册公开推广点击路由。
func RegisterPublicRoutes(public gin.IRoutes, handler *Handler) {
	public.POST("/affiliate/click", handler.TrackAffiliateClick)
}

// RegisterUserRoutes 注册需登录的推广返利路由。
func RegisterUserRoutes(user gin.IRoutes, handler *Handler) {
	user.POST("/affiliate/open", handler.OpenAffiliate)
	user.GET("/affiliate/dashboard", handler.GetAffiliateDashboard)
	user.GET("/affiliate/products/:id/rates", handler.GetProductRates)
	user.GET("/affiliate/commissions", handler.ListAffiliateCommissions)
	user.GET("/affiliate/withdraws", handler.ListAffiliateWithdraws)
	user.POST("/affiliate/withdraws", handler.ApplyAffiliateWithdraw)
}

// RegisterAdminRoutes 注册后台推广用户管理路由。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/affiliates/users", handler.ListAffiliateUsers)
	admin.PATCH("/affiliates/users/:id/status", handler.UpdateAffiliateUserStatus)
	admin.PATCH("/affiliates/users/batch-status", handler.BatchUpdateAffiliateUserStatus)
}

// RegisterAdminFinanceRoutes 注册后台推广财务审核路由（需支付合规）。
func RegisterAdminFinanceRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/affiliates/commissions", handler.ListAffiliateCommissions)
	admin.GET("/affiliates/withdraws", handler.ListAffiliateWithdraws)
	admin.POST("/affiliates/withdraws/:id/reject", handler.RejectAffiliateWithdraw)
	admin.POST("/affiliates/withdraws/:id/pay", handler.PayAffiliateWithdraw)
}

// RegisterChannelRoutes 注册渠道推广返利路由。
func RegisterChannelRoutes(channel gin.IRoutes, handler *ChannelHandler) {
	channel.POST("/affiliate/click", handler.TrackAffiliateClick)
	channel.POST("/affiliate/open", handler.OpenAffiliate)
	channel.GET("/affiliate/dashboard", handler.GetAffiliateDashboard)
	channel.GET("/affiliate/commissions", handler.ListAffiliateCommissions)
	channel.GET("/affiliate/withdraws", handler.ListAffiliateWithdraws)
	channel.POST("/affiliate/withdraws", handler.ApplyAffiliateWithdraw)
}

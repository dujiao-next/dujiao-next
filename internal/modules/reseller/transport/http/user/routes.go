package userhttp

import "github.com/gin-gonic/gin"

// RegisterUserConsoleRoutes 注册用户中心分销商控制台（入驻/域名/站点配置）路由。
// 调用方必须传入已挂载登录鉴权与 RequireMainTenantForResellerConsole 的 RouterGroup。
func RegisterUserConsoleRoutes(console gin.IRoutes, handler *UserHandler) {
	if console == nil || handler == nil {
		panic("reseller user console routes: required dependency is nil")
	}
	console.GET("/profile", handler.GetManagementSnapshot)
	console.PUT("/profile/default-markup", handler.UpdateDefaultMarkup)
	console.POST("/apply", handler.ApplyProfile)
	console.GET("/domains", handler.ListDomains)
	console.POST("/domains", handler.SubmitCustomDomain)
	console.GET("/site-config", handler.GetSiteConfig)
	console.PUT("/site-config", handler.UpdateSiteConfig)
	console.POST("/upload", handler.UploadImage)
}

// RegisterUserProductSettingRoutes 注册用户中心分销商品配置路由。
func RegisterUserProductSettingRoutes(console gin.IRoutes, handler *UserProductSettingHandler) {
	if console == nil || handler == nil {
		panic("reseller user product setting routes: required dependency is nil")
	}
	console.GET("/product-settings", handler.ListProductSettings)
	console.GET("/product-settings/:product_id", handler.GetProductSetting)
	console.POST("/product-settings/:product_id/preview", handler.PreviewProductSettings)
	console.PUT("/product-settings/:product_id", handler.UpdateProductSettings)
	console.DELETE("/product-settings/:product_id", handler.ResetProductSetting)
}

// RegisterUserFinanceRoutes 注册用户中心分销财务路由。
func RegisterUserFinanceRoutes(console gin.IRoutes, handler *UserFinanceHandler) {
	if console == nil || handler == nil {
		panic("reseller user finance routes: required dependency is nil")
	}
	console.GET("/dashboard", handler.GetDashboard)
	console.GET("/balance-accounts", handler.ListBalanceAccounts)
	console.GET("/ledger-entries", handler.ListLedgerEntries)
	console.GET("/withdraws", handler.ListWithdraws)
	console.POST("/withdraws", handler.ApplyWithdraw)
}

// RegisterUserOrderRoutes 注册用户中心分销销售订单只读路由。
func RegisterUserOrderRoutes(console gin.IRoutes, handler *UserOrderHandler) {
	if console == nil || handler == nil {
		panic("reseller user order routes: required dependency is nil")
	}
	console.GET("/orders", handler.ListOrders)
	console.GET("/orders/stats", handler.GetOrderStats)
	console.GET("/orders/:order_no", handler.GetOrderDetail)
}

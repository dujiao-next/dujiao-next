package adminhttp

import "github.com/gin-gonic/gin"

func RegisterManagementRoutes(admin gin.IRoutes, handler *AdminManagementHandler) {
	if admin == nil || handler == nil {
		panic("reseller admin management routes: required dependency is nil")
	}
	admin.GET("/resellers/profiles", handler.ListProfiles)
	admin.PUT("/resellers/profiles/:id", handler.UpdateProfile)
	admin.PUT("/resellers/profiles/:id/system-domain", handler.AssignSystemDomain)
	admin.POST("/resellers/profiles/:id/approve", handler.ApproveProfile)
	admin.POST("/resellers/profiles/:id/reject", handler.RejectProfile)
	admin.POST("/resellers/profiles/:id/disable", handler.DisableProfile)
	admin.POST("/resellers/profiles/:id/restore", handler.RestoreProfile)
	admin.GET("/resellers/domains", handler.ListDomains)
	admin.POST("/resellers/domains/:id/setup-https", handler.SetupDomainHTTPS)
	admin.POST("/resellers/domains/:id/approve", handler.ApproveDomain)
	admin.POST("/resellers/domains/:id/disable", handler.DisableDomain)
	admin.POST("/resellers/domains/:id/set-primary", handler.SetPrimaryDomain)
}

func RegisterProfileDetailRoutes(admin gin.IRoutes, handler *AdminProfileDetailHandler) {
	if admin == nil || handler == nil {
		panic("reseller admin profile detail routes: required dependency is nil")
	}
	admin.GET("/resellers/profiles/:id", handler.GetProfileDetail)
}

func RegisterSiteConfigRoutes(admin gin.IRoutes, handler *AdminSiteConfigHandler) {
	if admin == nil || handler == nil {
		panic("reseller admin site config routes: required dependency is nil")
	}
	admin.GET("/resellers/site-configs", handler.ListSiteConfigs)
	admin.GET("/resellers/site-configs/:reseller_id", handler.GetSiteConfig)
	admin.PUT("/resellers/site-configs/:reseller_id", handler.UpdateSiteConfig)
	admin.POST("/resellers/site-configs/:reseller_id/reset", handler.ResetSiteConfig)
}

func RegisterProductSettingRoutes(admin gin.IRoutes, handler *AdminProductSettingHandler) {
	if admin == nil || handler == nil {
		panic("reseller admin product setting routes: required dependency is nil")
	}
	admin.GET("/resellers/product-settings", handler.ListProductSettings)
	admin.GET("/resellers/product-settings/:reseller_id/:product_id", handler.GetProductSetting)
	admin.POST("/resellers/product-settings/:reseller_id/:product_id/preview", handler.PreviewProductSettings)
	admin.PUT("/resellers/product-settings/:reseller_id/:product_id", handler.UpdateProductSettings)
	admin.DELETE("/resellers/product-settings/:reseller_id/:product_id", handler.ResetProductSetting)
}

func RegisterOperationsOverviewRoutes(admin gin.IRoutes, handler *AdminOperationsHandler) {
	if admin == nil || handler == nil {
		panic("reseller admin operations overview routes: required dependency is nil")
	}
	admin.GET("/resellers/operations/overview", handler.GetOverview)
}

func RegisterOperationsFinanceRoutes(admin gin.IRoutes, handler *AdminOperationsHandler) {
	if admin == nil || handler == nil {
		panic("reseller admin operations finance routes: required dependency is nil")
	}
	admin.GET("/resellers/operations/finance", handler.GetFinance)
}

func RegisterFinanceRoutes(admin gin.IRoutes, handler *AdminFinanceHandler) {
	if admin == nil || handler == nil {
		panic("reseller admin finance routes: required dependency is nil")
	}
	admin.GET("/resellers/ledger-entries", handler.ListLedgerEntries)
	admin.GET("/resellers/balance-accounts", handler.ListBalanceAccounts)
	admin.GET("/resellers/withdraws", handler.ListWithdraws)
	admin.POST("/resellers/withdraws/:id/reject", handler.RejectWithdraw)
	admin.POST("/resellers/withdraws/:id/pay", handler.PayWithdraw)
}

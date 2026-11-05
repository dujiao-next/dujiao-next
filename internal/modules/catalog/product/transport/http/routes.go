package producthttp

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册公开商品目录端点。
func RegisterPublicRoutes(public gin.IRoutes, handler *PublicHandler) {
	if public == nil || handler == nil {
		panic("catalog product public routes: required dependency is nil")
	}
	public.GET("/products", handler.GetProducts)
	public.GET("/products/:slug", handler.GetProductBySlug)
}

// RegisterAdminRoutes 注册商品后台端点。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminProductHandler) {
	if admin == nil || handler == nil {
		panic("catalog product admin routes: required dependency is nil")
	}
	admin.GET("/products", handler.GetAdminProducts)
	admin.GET("/products/:id", handler.GetAdminProduct)
	admin.POST("/products", handler.CreateProduct)
	admin.PUT("/products/:id", handler.UpdateProduct)
	admin.PATCH("/products/:id/wholesale-prices", handler.UpdateProductWholesalePrices)
	admin.PATCH("/products/:id", handler.QuickUpdateProduct)
	admin.DELETE("/products/:id", handler.DeleteProduct)
	admin.POST("/products/batch-status", handler.BatchUpdateProductStatus)
	admin.POST("/products/batch-category", handler.BatchUpdateProductCategory)
	admin.POST("/products/batch-stock-display", handler.BatchUpdateProductStockDisplay)
	admin.POST("/products/batch-delete", handler.BatchDeleteProducts)
}

// RegisterPublicCompareRoutes 注册公开对比路由。
func RegisterPublicCompareRoutes(public gin.IRoutes, handler *PublicCompareHandler) {
	if public == nil || handler == nil {
		panic("catalog product public compare routes: required dependency is nil")
	}
	public.GET("/products/:slug/compare", handler.GetProductCompare)
	public.GET("/compare", handler.GetCompareByIDs)
}

// RegisterAdminCompareGroupRoutes 注册后台对比维度组路由。
func RegisterAdminCompareGroupRoutes(admin gin.IRoutes, handler *AdminCompareGroupHandler) {
	if admin == nil || handler == nil {
		panic("catalog product admin compare group routes: required dependency is nil")
	}
	admin.GET("/compare-groups", handler.GetCompareGroups)
	admin.GET("/compare-groups/:id", handler.GetCompareGroup)
	admin.POST("/compare-groups", handler.CreateCompareGroup)
	admin.PUT("/compare-groups/:id", handler.UpdateCompareGroup)
	admin.DELETE("/compare-groups/:id", handler.DeleteCompareGroup)
}

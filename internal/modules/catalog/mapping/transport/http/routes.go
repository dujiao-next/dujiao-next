package mappinghttp

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册商品映射后台端点。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	if admin == nil || handler == nil {
		panic("catalog admin product mapping routes: required dependency is nil")
	}
	admin.GET("/product-mappings", handler.GetProductMappings)
	admin.GET("/product-mappings/:id", handler.GetProductMapping)
	admin.POST("/product-mappings/import", handler.ImportUpstreamProduct)
	admin.POST("/product-mappings/batch-import", handler.BatchImportUpstreamProducts)
	admin.POST("/product-mappings/:id/sync", handler.SyncProductMapping)
	admin.PUT("/product-mappings/:id/status", handler.UpdateProductMappingStatus)
	admin.DELETE("/product-mappings/:id", handler.DeleteProductMapping)
	admin.POST("/product-mappings/batch-sync", handler.BatchSyncProductMappings)
	admin.POST("/product-mappings/batch-status", handler.BatchUpdateProductMappingStatus)
	admin.POST("/product-mappings/batch-delete", handler.BatchDeleteProductMappings)
	admin.GET("/upstream-products", handler.ListUpstreamProducts)
	admin.GET("/upstream-categories", handler.ListUpstreamCategories)
	admin.GET("/upstream-category-counts", handler.ListUpstreamCategoryCounts)
	admin.POST("/product-mappings/batch-import-by-category", handler.BatchImportByCategory)
}

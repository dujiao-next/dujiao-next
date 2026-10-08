package contenthttp

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册公开 Content 路由。
func RegisterPublicRoutes(public gin.IRoutes, handler *PublicHandler) {
	public.GET("/posts", handler.GetPosts)
	public.GET("/posts/:slug", handler.GetPostBySlug)
	public.GET("/banners", handler.GetPublicBanners)
	public.GET("/post-categories", handler.GetPostCategories)
}

// RegisterAdminRoutes 注册后台 Content 路由。
// 调用方必须传入已经挂载认证、RBAC 和后台安全中间件的 RouterGroup。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/posts", handler.GetAdminPosts)
	admin.POST("/posts", handler.CreatePost)
	// 静态 batch-* 必须先于参数路由注册。
	admin.POST("/posts/batch-status", handler.BatchUpdatePostStatus)
	admin.POST("/posts/batch-category", handler.BatchUpdatePostCategory)
	admin.POST("/posts/batch-delete", handler.BatchDeletePosts)
	admin.PUT("/posts/:id", handler.UpdatePost)
	admin.DELETE("/posts/:id", handler.DeletePost)
	admin.GET("/posts/:id/products", handler.GetAdminPostProductIDs)

	admin.GET("/post-categories", handler.GetPostCategories)
	admin.POST("/post-categories", handler.CreatePostCategory)
	admin.PUT("/post-categories/:id", handler.UpdatePostCategory)
	admin.DELETE("/post-categories/:id", handler.DeletePostCategory)
	admin.PATCH("/post-categories/:id/status", handler.PatchPostCategoryStatus)

	admin.GET("/banners", handler.GetAdminBanners)
	admin.GET("/banners/:id", handler.GetAdminBanner)
	admin.POST("/banners", handler.CreateBanner)
	admin.PUT("/banners/:id", handler.UpdateBanner)
	admin.DELETE("/banners/:id", handler.DeleteBanner)

	// 静态 batch-delete 必须先于参数路由注册。
	admin.GET("/media", handler.GetAdminMedia)
	admin.POST("/media/batch-delete", handler.BatchDeleteMedia)
	admin.PUT("/media/:id", handler.UpdateMedia)
	admin.DELETE("/media/:id", handler.DeleteMedia)
}

package memberlevelhttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/member-levels", handler.GetAdminMemberLevels)
	admin.POST("/member-levels", handler.CreateMemberLevel)
	admin.PUT("/member-levels/:id", handler.UpdateMemberLevel)
	admin.DELETE("/member-levels/:id", handler.DeleteMemberLevel)
	admin.GET("/member-level-prices", handler.GetMemberLevelPrices)
	admin.POST("/member-level-prices/batch", handler.BatchUpsertMemberLevelPrices)
	admin.DELETE("/member-level-prices/:id", handler.DeleteMemberLevelPrice)
	admin.GET("/member-user-prices", handler.GetUserPrices)
	admin.POST("/member-user-prices/batch", handler.BatchUpsertUserPrices)
	admin.DELETE("/member-user-prices/:id", handler.DeleteUserPrice)
	admin.POST("/member-levels/backfill", handler.BackfillMemberLevels)
	admin.PUT("/users/:id/member-level", handler.SetUserMemberLevel)
}

func RegisterPublicRoutes(public gin.IRoutes, handler *PublicHandler) {
	public.GET("/member-levels", handler.List)
}

// RegisterUserRoutes 注册登录用户专属价端点。
func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	user.GET("/me/product-prices", handler.ListMyPrices)
}

func RegisterChannelRoutes(channel gin.IRoutes, handler *ChannelHandler) {
	channel.GET("/member-levels", handler.List)
}

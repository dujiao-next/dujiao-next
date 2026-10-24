package invitecodehttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/invite-codes", handler.List)
	admin.POST("/invite-codes/generate", handler.Generate)
	admin.PUT("/invite-codes/:id", handler.Update)
	admin.DELETE("/invite-codes/:id", handler.Delete)
	admin.GET("/invite-codes/:id/users", handler.Users)
}

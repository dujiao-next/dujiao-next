package adminapitokenhttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(authorized gin.IRoutes, handler *Handler) {
	authorized.GET("/authz/admins/:id/api-tokens", handler.List)
	authorized.POST("/authz/admins/:id/api-tokens", handler.Create)
	authorized.DELETE("/authz/admins/:id/api-tokens/:token_id", handler.Revoke)
	authorized.POST("/authz/admins/:id/api-tokens/:token_id/rotate", handler.Rotate)
}

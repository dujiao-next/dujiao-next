package supporthttp

import "github.com/gin-gonic/gin"

func registerCustomer(routes gin.IRoutes, h *Handler) {
	routes.GET("/support/conversations", h.List)
	routes.POST("/support/conversations", h.Create)
	routes.GET("/support/conversations/:id", h.Get)
	routes.GET("/support/conversations/:id/messages", h.Messages)
	routes.POST("/support/conversations/:id/messages", h.Send)
	routes.POST("/support/conversations/:id/read", h.Read)
	routes.POST("/support/conversations/:id/order", h.Link)
	routes.POST("/support/conversations/:id/attachments", h.Upload)
	routes.GET("/support/conversations/:id/attachments/:attachmentID", h.Download)
}
func RegisterUserRoutes(user gin.IRoutes, h *Handler)   { registerCustomer(user, h) }
func RegisterGuestRoutes(guest gin.IRoutes, h *Handler) { registerCustomer(guest, h) }
func RegisterGuestSplitRoutes(read, write gin.IRoutes, h *Handler) {
	read.GET("/support/conversations", h.List)
	read.GET("/support/conversations/:id", h.Get)
	read.GET("/support/conversations/:id/messages", h.Messages)
	read.GET("/support/conversations/:id/attachments/:attachmentID", h.Download)
	read.POST("/support/conversations/:id/read", h.Read)
	write.POST("/support/conversations", h.Create)
	write.POST("/support/conversations/:id/messages", h.Send)
	write.POST("/support/conversations/:id/order", h.Link)
	write.POST("/support/conversations/:id/attachments", h.Upload)
}
func RegisterAdminRoutes(admin gin.IRoutes, h *Handler) {
	admin.GET("/support/conversations", h.List)
	admin.GET("/support/agents", h.Agents)
	admin.GET("/support/conversations/:id", h.Get)
	admin.GET("/support/conversations/:id/messages", h.Messages)
	admin.POST("/support/conversations/:id/messages", h.Send)
	admin.POST("/support/conversations/:id/read", h.Read)
	admin.POST("/support/conversations/:id/claim", h.Claim)
	admin.PUT("/support/conversations/:id/assignee", h.Assign)
	admin.PUT("/support/conversations/:id/status", h.Status)
	admin.POST("/support/conversations/:id/attachments", h.Upload)
	admin.GET("/support/conversations/:id/attachments/:attachmentID", h.Download)
}

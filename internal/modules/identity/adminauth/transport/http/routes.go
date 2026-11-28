package adminauthhttp

import "github.com/gin-gonic/gin"

// RegisterAdminLoginAuthRoutes 注册公开的管理员登录端点（需附带限流中间件）。
func RegisterAdminLoginAuthRoutes(admin gin.IRoutes, handler *AdminLoginHandler, rateLimit gin.HandlerFunc) {
	if admin == nil || handler == nil || rateLimit == nil {
		panic("admin login auth routes: required dependency is nil")
	}
	admin.POST("/login", rateLimit, handler.AdminLogin)
}

// RegisterAdmin2FAAuthRoutes 注册公开的管理员 2FA 挑战验证端点（需附带限流中间件）。
func RegisterAdmin2FAAuthRoutes(admin gin.IRoutes, handler *Admin2FAHandler, rateLimit gin.HandlerFunc) {
	if admin == nil || handler == nil || rateLimit == nil {
		panic("admin 2fa auth routes: required dependency is nil")
	}
	admin.POST("/login/verify-2fa", rateLimit, handler.Verify2FA)
}

// RegisterAdminSessionRoutes 注册登录态管理员会话续期端点。
func RegisterAdminSessionRoutes(authorized gin.IRoutes, handler *AdminLoginHandler) {
	if authorized == nil || handler == nil {
		panic("admin session routes: required dependency is nil")
	}
	authorized.POST("/auth/refresh", handler.RefreshSession)
}

// RegisterAdminPasswordRoutes 注册登录态管理员改密端点。
func RegisterAdminPasswordRoutes(authorized gin.IRoutes, handler *AdminLoginHandler) {
	if authorized == nil || handler == nil {
		panic("admin password routes: required dependency is nil")
	}
	authorized.PUT("/password", handler.UpdateAdminPassword)
}

// RegisterAdmin2FARoutes 注册登录态管理员 2FA 管理端点。
func RegisterAdmin2FARoutes(authorized gin.IRoutes, handler *Admin2FAHandler) {
	if authorized == nil || handler == nil {
		panic("admin 2fa routes: required dependency is nil")
	}
	authorized.GET("/2fa/status", handler.Get2FAStatus)
	authorized.POST("/2fa/setup", handler.Setup2FA)
	authorized.POST("/2fa/enable", handler.Enable2FA)
	authorized.POST("/2fa/disable", handler.Disable2FA)
	authorized.POST("/2fa/recovery-codes/regenerate", handler.RegenerateRecoveryCodes)
	authorized.POST("/authz/admins/:id/2fa/reset", handler.ResetTargetAdmin2FA)
}

// RegisterAdminUser2FARoutes 注册管理员重置用户 2FA 端点。
func RegisterAdminUser2FARoutes(authorized gin.IRoutes, handler *AdminUser2FAHandler) {
	if authorized == nil || handler == nil {
		panic("admin user 2fa routes: required dependency is nil")
	}
	authorized.DELETE("/users/:id/2fa", handler.ResetUser2FA)
}

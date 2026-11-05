package userauthhttp

import "github.com/gin-gonic/gin"

// RegisterUserProfileRoutes 注册当前用户资料端点。
func RegisterUserProfileRoutes(user gin.IRoutes, handler *UserProfileHandler) {
	if user == nil || handler == nil {
		panic("user profile routes: required dependency is nil")
	}
	user.GET("/me", handler.GetCurrentUser)
	user.PUT("/me/profile", handler.UpdateUserProfile)
}

// RegisterUserEmailRoutes 注册当前用户更换邮箱端点。
func RegisterUserEmailRoutes(user gin.IRoutes, handler *UserEmailHandler) {
	if user == nil || handler == nil {
		panic("user email routes: required dependency is nil")
	}
	user.POST("/me/email/send-verify-code", handler.SendChangeEmailCode)
	user.POST("/me/email/change", handler.ChangeEmail)
}

// RegisterUserVerifyAuthRoutes 注册公开的发送邮箱验证码端点（需附带限流中间件）。
func RegisterUserVerifyAuthRoutes(auth gin.IRoutes, handler *UserVerifyHandler, rateLimit gin.HandlerFunc) {
	if auth == nil || handler == nil || rateLimit == nil {
		panic("user verify auth routes: required dependency is nil")
	}
	auth.POST("/send-verify-code", rateLimit, handler.SendUserVerifyCode)
}

// RegisterUserRegisterAuthRoutes 注册公开的用户注册端点（需附带限流中间件）。
func RegisterUserRegisterAuthRoutes(auth gin.IRoutes, handler *UserLoginHandler, rateLimit gin.HandlerFunc) {
	if auth == nil || handler == nil || rateLimit == nil {
		panic("user register auth routes: required dependency is nil")
	}
	auth.POST("/register", rateLimit, handler.UserRegister)
}

// RegisterUserInviteAuthRoutes 注册第三方登录新用户补填邀请码端点（需附带限流中间件）。
func RegisterUserInviteAuthRoutes(auth gin.IRoutes, handler *UserInviteHandler, rateLimit gin.HandlerFunc) {
	if auth == nil || handler == nil || rateLimit == nil {
		panic("user invite auth routes: required dependency is nil")
	}
	auth.POST("/invite/complete", rateLimit, handler.CompleteInviteRegistration)
}

// RegisterUserLoginAuthRoutes 注册公开的用户登录端点（需附带限流中间件）。
func RegisterUserLoginAuthRoutes(auth gin.IRoutes, handler *UserLoginHandler, rateLimit gin.HandlerFunc) {
	if auth == nil || handler == nil || rateLimit == nil {
		panic("user login auth routes: required dependency is nil")
	}
	auth.POST("/login", rateLimit, handler.UserLogin)
}

// RegisterUser2FAAuthRoutes 注册公开的 2FA 挑战验证端点（需附带限流中间件）。
func RegisterUser2FAAuthRoutes(auth gin.IRoutes, handler *User2FAHandler, rateLimit gin.HandlerFunc) {
	if auth == nil || handler == nil || rateLimit == nil {
		panic("user 2fa auth routes: required dependency is nil")
	}
	auth.POST("/login/verify-2fa", rateLimit, handler.VerifyUser2FA)
}

// RegisterUser2FARoutes 注册登录态 2FA 管理端点。
func RegisterUser2FARoutes(user gin.IRoutes, handler *User2FAHandler) {
	if user == nil || handler == nil {
		panic("user 2fa routes: required dependency is nil")
	}
	user.GET("/me/2fa/status", handler.GetUser2FAStatus)
	user.POST("/me/2fa/setup", handler.SetupUser2FA)
	user.POST("/me/2fa/enable", handler.EnableUser2FA)
	user.POST("/me/2fa/disable", handler.DisableUser2FA)
	user.POST("/me/2fa/recovery-codes/regenerate", handler.RegenerateUser2FARecoveryCodes)
}

// RegisterUserPasswordAuthRoutes 注册公开的忘记密码端点（需附带限流中间件）。
func RegisterUserPasswordAuthRoutes(auth gin.IRoutes, handler *UserPasswordHandler, rateLimit gin.HandlerFunc) {
	if auth == nil || handler == nil || rateLimit == nil {
		panic("user password auth routes: required dependency is nil")
	}
	auth.POST("/forgot-password", rateLimit, handler.UserForgotPassword)
}

// RegisterUserPasswordRoutes 注册登录态改密端点。
func RegisterUserPasswordRoutes(user gin.IRoutes, handler *UserPasswordHandler) {
	if user == nil || handler == nil {
		panic("user password routes: required dependency is nil")
	}
	user.PUT("/me/password", handler.ChangeUserPassword)
}

// RegisterUserTelegramOIDCAuthRoutes 注册公开 Telegram OIDC 登录端点（需附带限流中间件）。
func RegisterUserTelegramOIDCAuthRoutes(auth gin.IRoutes, handler *UserTelegramOIDCHandler, rateLimit gin.HandlerFunc) {
	if auth == nil || handler == nil || rateLimit == nil {
		panic("user telegram oidc auth routes: required dependency is nil")
	}
	auth.GET("/telegram/oidc/start", rateLimit, handler.StartTelegramOIDCLogin)
	auth.POST("/telegram/oidc/callback", rateLimit, handler.TelegramOIDCLoginCallback)
}

// RegisterUserTelegramOIDCRoutes 注册登录态 Telegram OIDC 绑定端点。
func RegisterUserTelegramOIDCRoutes(user gin.IRoutes, handler *UserTelegramOIDCHandler) {
	if user == nil || handler == nil {
		panic("user telegram oidc routes: required dependency is nil")
	}
	user.GET("/me/telegram/oidc/start", handler.StartTelegramOIDCBind)
	user.POST("/me/telegram/oidc/callback", handler.TelegramOIDCBindCallback)
}

// RegisterUserTelegramAuthRoutes 注册公开 Telegram widget/MiniApp 登录端点（需附带限流中间件）。
func RegisterUserTelegramAuthRoutes(auth gin.IRoutes, handler *UserTelegramHandler, rateLimit gin.HandlerFunc) {
	if auth == nil || handler == nil || rateLimit == nil {
		panic("user telegram auth routes: required dependency is nil")
	}
	auth.POST("/telegram/login", rateLimit, handler.UserTelegramLogin)
	auth.POST("/telegram/miniapp/login", rateLimit, handler.UserTelegramMiniAppLogin)
}

// RegisterUserTelegramRoutes 注册登录态 Telegram 绑定端点。
func RegisterUserTelegramRoutes(user gin.IRoutes, handler *UserTelegramHandler) {
	if user == nil || handler == nil {
		panic("user telegram routes: required dependency is nil")
	}
	user.GET("/me/telegram", handler.GetMyTelegramBinding)
	user.POST("/me/telegram/bind", handler.BindMyTelegram)
	user.POST("/me/telegram/miniapp/bind", handler.BindMyTelegramMiniApp)
	user.DELETE("/me/telegram/unbind", handler.UnbindMyTelegram)
}

// RegisterUserGoogleAuthRoutes registers public Google Identity Services login.
func RegisterUserGoogleAuthRoutes(auth gin.IRoutes, handler *UserGoogleHandler, rateLimit gin.HandlerFunc) {
	if auth == nil || handler == nil || rateLimit == nil {
		panic("user google auth routes: required dependency is nil")
	}
	auth.POST("/google/login", rateLimit, handler.UserGoogleLogin)
	auth.POST("/google/redirect/intent", rateLimit, handler.CreateGoogleRedirectLoginIntent)
	auth.POST("/google/redirect/callback", handler.CompleteGoogleRedirect)
	auth.POST("/google/redirect/exchange", handler.ExchangeGoogleRedirectLogin)
}

// RegisterUserGoogleRoutes registers authenticated Google binding endpoints.
func RegisterUserGoogleRoutes(user gin.IRoutes, handler *UserGoogleHandler, bindRateLimit gin.HandlerFunc) {
	if user == nil || handler == nil || bindRateLimit == nil {
		panic("user google routes: required dependency is nil")
	}
	user.GET("/me/google", handler.GetMyGoogleBinding)
	user.POST("/me/google/bind", bindRateLimit, handler.BindMyGoogle)
	user.POST("/me/google/redirect/intent", bindRateLimit, handler.CreateGoogleRedirectBindIntent)
	user.POST("/me/google/redirect/exchange", handler.ExchangeGoogleRedirectBind)
	user.DELETE("/me/google/unbind", handler.UnbindMyGoogle)
}

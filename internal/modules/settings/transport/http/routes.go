package settingshttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/settings", handler.Get)
	admin.PUT("/settings", handler.Update)
}

func RegisterAdminSMTPRoutes(admin gin.IRoutes, handler *SMTPHandler) {
	admin.GET("/settings/smtp", handler.GetSMTP)
	admin.PUT("/settings/smtp", handler.UpdateSMTP)
	admin.POST("/settings/smtp/test", handler.TestSMTP)
}

func RegisterAdminCaptchaRoutes(admin gin.IRoutes, handler *CaptchaHandler) {
	admin.GET("/settings/captcha", handler.GetCaptcha)
	admin.PUT("/settings/captcha", handler.UpdateCaptcha)
}

func RegisterAdminTelegramAuthRoutes(admin gin.IRoutes, handler *TelegramAuthHandler) {
	admin.GET("/settings/telegram-auth", handler.GetTelegramAuth)
	admin.PUT("/settings/telegram-auth", handler.UpdateTelegramAuth)
}

func RegisterAdminGoogleAuthRoutes(admin gin.IRoutes, handler *GoogleAuthHandler) {
	admin.GET("/settings/google-auth", handler.GetGoogleAuth)
	admin.PUT("/settings/google-auth", handler.UpdateGoogleAuth)
}

func RegisterAdminGitHubAuthRoutes(admin gin.IRoutes, handler *GitHubAuthHandler) {
	admin.GET("/settings/github-auth", handler.GetGitHubAuth)
	admin.PUT("/settings/github-auth", handler.UpdateGitHubAuth)
}

func RegisterAdminAffiliateRoutes(admin gin.IRoutes, handler *AffiliateHandler) {
	admin.GET("/settings/affiliate", handler.GetAffiliate)
	admin.PUT("/settings/affiliate", handler.UpdateAffiliate)
}

func RegisterAdminOrderEmailTemplateRoutes(admin gin.IRoutes, handler *OrderEmailTemplateHandler) {
	admin.GET("/settings/order-email-template", handler.GetOrderEmailTemplate)
	admin.PUT("/settings/order-email-template", handler.UpdateOrderEmailTemplate)
	admin.POST("/settings/order-email-template/reset", handler.ResetOrderEmailTemplate)
}

func RegisterAdminTelegramBotRoutes(admin gin.IRoutes, handler *TelegramBotHandler) {
	admin.GET("/settings/telegram-bot", handler.GetTelegramBotConfig)
	admin.PUT("/settings/telegram-bot", handler.UpdateTelegramBotConfig)
	admin.GET("/settings/telegram-bot/runtime-status", handler.GetTelegramBotRuntimeStatus)
}

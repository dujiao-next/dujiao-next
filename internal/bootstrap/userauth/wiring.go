package userauthwiring

import (
	"github.com/dujiao-next/internal/app/container"
	captchahttp "github.com/dujiao-next/internal/modules/captcha/transport/http"
	userauthtransport "github.com/dujiao-next/internal/modules/identity/userauth/transport/http"
)

// Handlers contains the complete user authentication HTTP entrypoint set.
type Handlers struct {
	Profile      *userauthtransport.UserProfileHandler
	Email        *userauthtransport.UserEmailHandler
	Password     *userauthtransport.UserPasswordHandler
	Verify       *userauthtransport.UserVerifyHandler
	Login        *userauthtransport.UserLoginHandler
	TwoFA        *userauthtransport.User2FAHandler
	TelegramOIDC *userauthtransport.UserTelegramOIDCHandler
	Telegram     *userauthtransport.UserTelegramHandler
	Google       *userauthtransport.UserGoogleHandler
	GitHub       *userauthtransport.UserGitHubHandler
}

// New assembles user authentication transports at the application boundary.
func New(c *container.Container) Handlers {
	verify := userVerifyTransportAdapter{auth: c.UserAuthService, settings: c.SettingService}
	login := userLoginTransportAdapter{auth: c.UserAuthService, settings: c.SettingService}
	recorder := userLoginRecorderAdapter{logs: c.UserLoginLogService}
	captcha := captchahttp.NewVerifier(c.CaptchaService)

	return Handlers{
		Profile: userauthtransport.NewUserProfileHandler(
			userProfileTransportAdapter{service: c.UserAuthService},
		),
		Email: userauthtransport.NewUserEmailHandler(
			userEmailTransportAdapter{service: c.UserAuthService},
		),
		Password: userauthtransport.NewUserPasswordHandler(userPasswordTransportAdapter{
			auth: c.UserAuthService, settings: c.SettingService,
		}),
		Verify: userauthtransport.NewUserVerifyHandler(
			verify,
			captcha,
			verify,
		),
		Login: userauthtransport.NewUserLoginHandler(
			login,
			login,
			captcha,
			recorder,
		),
		TwoFA: userauthtransport.NewUser2FAHandler(
			user2FATOTPTransportAdapter{totp: c.UserTOTPService},
			user2FAAuthTransportAdapter{auth: c.UserAuthService, users: c.UserStore},
			user2FAChallengeStoreAdapter{},
			recorder,
		),
		TelegramOIDC: userauthtransport.NewUserTelegramOIDCHandler(
			userTelegramOIDCTransportAdapter{auth: c.UserAuthService},
			recorder,
		),
		Telegram: userauthtransport.NewUserTelegramHandler(
			userTelegramTransportAdapter{auth: c.UserAuthService},
			recorder,
		),
		Google: userauthtransport.NewUserGoogleHandler(
			userGoogleTransportAdapter{auth: c.UserAuthService},
			recorder,
		),
		GitHub: userauthtransport.NewUserGitHubHandler(c.GitHubAuthService, c.UserAuthService, recorder),
	}
}

package support

import (
	"github.com/dujiao-next/internal/modules/notification/infrastructure/wxpusher"
	"github.com/dujiao-next/internal/modules/support/application"
)

func NewNotifications(s *application.Service, secret string) *application.Notifications {
	return application.NewNotifications(s, wxpusher.New(), SecretBox{Secret: secret})
}

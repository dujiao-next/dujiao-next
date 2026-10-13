package app

import (
	"github.com/dujiao-next/internal/app/container"
	supportbootstrap "github.com/dujiao-next/internal/bootstrap/support"
	"github.com/dujiao-next/internal/modules/notification/infrastructure/wxpusher"
	"github.com/dujiao-next/internal/modules/support/application"
)

func supportReminderService(c *container.Container) *application.Notifications {
	return application.NewNotifications(c.SupportService, wxpusher.New(), supportbootstrap.SecretBox{Secret: c.Config.App.SecretKey})
}

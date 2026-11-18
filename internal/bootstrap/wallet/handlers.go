package walletbootstrap

import (
	"github.com/dujiao-next/internal/app/container"
	channeluserwiring "github.com/dujiao-next/internal/bootstrap/channeluser"
	wallettransport "github.com/dujiao-next/internal/modules/wallet/transport/http"
)

type Handlers struct {
	User    *wallettransport.UserHandler
	Admin   *wallettransport.AdminHandler
	Channel *wallettransport.ChannelHandler
}

func New(c *container.Container) Handlers {
	wallets := walletTransportAdapter{wallets: c.WalletService, payments: c.PaymentService}
	return Handlers{
		User: wallettransport.NewUserHandler(
			wallets, wallets, c.UserStore,
		),
		Admin: wallettransport.NewAdminHandler(
			wallets, c.UserStore, c.PaymentChannelStore, c.PaymentStore, c.SettingService,
		),
		Channel: wallettransport.NewChannelHandler(
			wallets,
			wallets,
			channeluserwiring.NewSimpleProvisioner(c.UserAuthService),
		),
	}
}

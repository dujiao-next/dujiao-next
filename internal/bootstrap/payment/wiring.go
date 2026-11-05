package paymentbootstrap

import (
	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/modules/payment/application/cashier"
	paymenttransport "github.com/dujiao-next/internal/modules/payment/transport/http"
	paymentcallbacktransport "github.com/dujiao-next/internal/modules/payment/transport/http/callback"
)

// Handlers is the complete payment HTTP entrypoint set assembled at the
// application boundary.
type Handlers struct {
	Latest       *paymenttransport.LatestHandler
	Write        *paymenttransport.WriteHandler
	Admin        *paymenttransport.AdminHandler
	AdminChannel *paymenttransport.AdminChannelHandler
	Webhook      *paymenttransport.WebhookHandler
	Callback     *paymentcallbacktransport.Handler
	EpayRedirect *paymenttransport.EpayRedirectHandler
}

// New assembles payment transports from application services and HTTP handlers.
func New(c *container.Container) Handlers {
	guestOrders := guestOrderLookupAdapter{orders: c.OrderService}
	userOrders := userOrderLookupAdapter{orders: c.OrderService}
	alerter := exceptionAlerterAdapter{notifications: c.NotificationService}

	return Handlers{
		Latest: paymenttransport.NewLatestHandler(
			guestOrders,
			userOrders,
			pendingLookupAdapter{payments: c.PaymentStore},
			&cashier.Service{Store: c.PaymentStore, Registry: c.PaymentProviderRegistry, Enabled: c.Config != nil && c.Config.PaymentChainVerification.Enabled && c.Config.PaymentChainVerification.SettlementEnabled && c.Config.PaymentChainVerification.CashierEnabled},
		),
		Write: paymenttransport.NewWriteHandler(
			guestOrders,
			userOrders,
			writerAdapter{payments: c.PaymentService},
		),
		Admin: paymenttransport.NewAdminHandler(
			adminQueryAdapter{payments: c.PaymentService},
			adminChannelLookupAdapter{channels: c.PaymentChannelStore},
			adminOrderLookupAdapter{orders: c.OrderStore},
			adminRechargeLookupAdapter{wallets: c.WalletRepo},
			adminPaymentConfirmerAdapter{payments: c.PaymentService},
		),
		AdminChannel: paymenttransport.NewAdminChannelHandler(
			adminChannelCatalogAdapter{payments: c.PaymentService, channels: c.PaymentChannelStore},
		),
		Webhook: paymenttransport.NewWebhookHandler(
			webhookServiceAdapter{payments: c.PaymentService},
			alerter,
		),
		Callback: paymentcallbacktransport.NewHandler(
			callbackServiceAdapter{payments: c.PaymentService},
			c.PaymentStore,
			c.PaymentChannelStore,
			alerter,
		),
		EpayRedirect: paymenttransport.NewEpayRedirectHandler(c.PaymentStore),
	}
}

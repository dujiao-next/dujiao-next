package container

import (
	"context"

	catalogmappingbootstrap "github.com/dujiao-next/internal/bootstrap/catalogmapping"
	"github.com/dujiao-next/internal/bootstrap/resellernotify"
	"github.com/dujiao-next/internal/bootstrap/restocknotify"
	telegrambroadcast "github.com/dujiao-next/internal/bootstrap/telegrambroadcast"
	"github.com/dujiao-next/internal/logger"
	apicredentialapp "github.com/dujiao-next/internal/modules/apicredential/application"
	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	channelclientapp "github.com/dujiao-next/internal/modules/channelclient/application"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	localfilestore "github.com/dujiao-next/internal/modules/content/infrastructure/filestore/local"
	contentgormstore "github.com/dujiao-next/internal/modules/content/infrastructure/gormstore"
	dashboardapp "github.com/dujiao-next/internal/modules/dashboard/application"
	downstreamcallbackapp "github.com/dujiao-next/internal/modules/downstreamcallback/application"
	downstreamcallbackcontract "github.com/dujiao-next/internal/modules/downstreamcallback/contract"
	downstreamcallbackclient "github.com/dujiao-next/internal/modules/downstreamcallback/infrastructure/callbackclient"
	downstreamcallbackcredentialreader "github.com/dujiao-next/internal/modules/downstreamcallback/infrastructure/credentialreader"
	downstreamcallbackorderreader "github.com/dujiao-next/internal/modules/downstreamcallback/infrastructure/orderreader"
	downstreamcallbackqueue "github.com/dujiao-next/internal/modules/downstreamcallback/infrastructure/queueadapter"
	errorlogapp "github.com/dujiao-next/internal/modules/errorlog/application"
	adminapitokenapp "github.com/dujiao-next/internal/modules/identity/adminapitoken/application"
	notificationapp "github.com/dujiao-next/internal/modules/notification/application"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	notificationasyncqueue "github.com/dujiao-next/internal/modules/notification/infrastructure/asyncqueue"
	notificationfeishu "github.com/dujiao-next/internal/modules/notification/infrastructure/feishu"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	paymentqueue "github.com/dujiao-next/internal/modules/payment/infrastructure/queueadapter"
	procurementapp "github.com/dujiao-next/internal/modules/procurement/application"
	procurementmapping "github.com/dujiao-next/internal/modules/procurement/infrastructure/mappingreader"
	procurementnotification "github.com/dujiao-next/internal/modules/procurement/infrastructure/notificationadapter"
	procurementorder "github.com/dujiao-next/internal/modules/procurement/infrastructure/orderreader"
	procurementqueue "github.com/dujiao-next/internal/modules/procurement/infrastructure/queueadapter"
	procurementupstream "github.com/dujiao-next/internal/modules/procurement/infrastructure/upstreamgateway"
	reconciliationapp "github.com/dujiao-next/internal/modules/reconciliation/application"
	reconciliationnotification "github.com/dujiao-next/internal/modules/reconciliation/infrastructure/notificationadapter"
	reconciliationprocurement "github.com/dujiao-next/internal/modules/reconciliation/infrastructure/procurementreader"
	reconciliationqueue "github.com/dujiao-next/internal/modules/reconciliation/infrastructure/queueadapter"
	reconciliationupstream "github.com/dujiao-next/internal/modules/reconciliation/infrastructure/upstreamreader"
	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
	broadcastapp "github.com/dujiao-next/internal/modules/telegram/broadcast/application"
	notifyapp "github.com/dujiao-next/internal/modules/telegram/notify/application"
	notifycontract "github.com/dujiao-next/internal/modules/telegram/notify/contract"
	notifybotapi "github.com/dujiao-next/internal/modules/telegram/notify/infrastructure/botapi"
	ticketapp "github.com/dujiao-next/internal/modules/ticket/application"
	ticketnotifier "github.com/dujiao-next/internal/modules/ticket/infrastructure/notifier"
	ticketorderadapter "github.com/dujiao-next/internal/modules/ticket/infrastructure/orderadapter"
	uploadlocal "github.com/dujiao-next/internal/modules/upload/infrastructure/localstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/platform/retention"
)

// initIntegrationServices 装配通知、站点对接、支付、采购、渠道与 Telegram 集成。
func (c *Container) initIntegrationServices() {
	c.AdminAPITokenService = adminapitokenapp.NewService(c.AdminAPITokenStore, c.AdminStore)
	c.UserLoginLogService = auditlogapp.NewUserLoginService(c.UserLoginLogRepo)
	c.AuthzAuditService = auditlogapp.NewAuthzService(c.AuthzAuditLogRepo)
	c.AdminLoginLogService = auditlogapp.NewAdminLoginService(c.AdminLoginLogRepo)
	c.ErrorLogService = errorlogapp.NewService(c.ErrorLogRepo, &errorlogapp.Options{
		Collect4xx: c.Config.ErrorLog.Collect4xx,
	})
	c.NotificationLogService = notificationapp.NewLogService(c.NotificationLogRepo)
	c.LogRetentionRunner = retention.NewRunner([]retention.Target{
		{Name: "error_logs", RetentionDays: c.Config.LogRetention.ErrorLog, Delete: c.ErrorLogService.DeleteOlderThan},
		{Name: "user_login_logs", RetentionDays: c.Config.LogRetention.UserLogin, Delete: c.UserLoginLogRepo.DeleteOlderThan},
		{Name: "admin_login_logs", RetentionDays: c.Config.LogRetention.AdminLogin, Delete: c.AdminLoginLogRepo.DeleteOlderThan},
		{Name: "authz_audit_logs", RetentionDays: c.Config.LogRetention.AuthzAudit, Delete: c.AuthzAuditLogRepo.DeleteOlderThan},
		{Name: "notification_logs", RetentionDays: c.Config.LogRetention.NotificationLog, Delete: c.NotificationLogRepo.DeleteOlderThan},
	}, retention.Options{})
	c.DashboardService = dashboardapp.NewService(c.DashboardRepo, c.SettingService)
	telegramNotifyService := notifyapp.NewService(c.SettingService, c.Config.TelegramAuth, notifybotapi.New())
	c.NotificationService = notificationapp.NewService(
		c.SettingService,
		c.EmailSender,
		notificationasyncqueue.New(c.QueueClient),
		c.DashboardService,
		c.NotificationLogService,
		telegramNotifySenderAdapter{svc: telegramNotifyService},
		notificationfeishu.New(),
	)
	restockNotifier := restocknotify.New(c.NotificationService, c.SettingService)
	c.CardSecretService.SetRestockNotifier(restockNotifier)
	c.ProductWriteService.SetRestockNotifier(restockNotifier)
	// 提现申请通知：走普通通知事件（与订单通知同一路径，发到通知中心收件人），
	// 不是补货广播。两个模块共用同一个通知器实例。
	withdrawNotifier := resellernotify.New(c.NotificationService, c.UserStore, c.SettingService)
	c.ResellerAccountingWithdraw.SetWithdrawNotifier(withdrawNotifier)
	c.AffiliateService.SetWithdrawNotifier(withdrawNotifier)
	c.ApiCredentialService = apicredentialapp.NewService(c.ApiCredentialRepo)
	c.SiteConnectionService = siteconnectionapp.NewService(c.SiteConnectionRepo, c.Config.App.SecretKey, "uploads")
	mediaCore := contentapp.NewMediaService(
		contentgormstore.NewMediaStore(gormdb.DB),
		localfilestore.New(),
		contentapp.WarningLoggerFunc(logger.Warnw),
	)
	c.ContentMediaService = mediaCore
	productMappingService, err := catalogmappingbootstrap.New(catalogmappingbootstrap.Dependencies{
		Mappings:    c.ProductMappingRepo,
		SKUMappings: c.SKUMappingRepo,
		Products:    c.ProductRepo,
		SKUs:        c.ProductSKURepo,
		Categories:  c.CategoryRepo,
		Connections: c.SiteConnectionService,
		Media:       mediaCore,
	})
	if err != nil {
		logger.Errorw("provider_init_product_mapping_failed", "error", err)
		panic(err)
	}
	c.ProductMappingService = productMappingService
	c.ProductMappingService.SetCategoryCreator(c.CategoryService)
	c.ProductMappingService.SetSettings(c.SettingService)
	c.SiteConnectionService.SetMarkupReapplier(c.ProductMappingService)
	c.OrderService.SetProductMappingService(c.ProductMappingService)
	var downstreamQueue downstreamcallbackcontract.CallbackQueue
	if c.QueueClient != nil {
		downstreamQueue = downstreamcallbackqueue.New(c.QueueClient)
	}
	c.DownstreamCallbackService = downstreamcallbackapp.NewService(downstreamcallbackapp.Options{
		References:  c.DownstreamOrderRefRepo,
		Orders:      downstreamcallbackorderreader.New(c.OrderStore),
		Credentials: downstreamcallbackcredentialreader.New(c.ApiCredentialRepo),
		Queue:       downstreamQueue,
		Deliverer:   downstreamcallbackclient.New(),
	})
	c.PaymentService = paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{
		OrderStore:              c.OrderStore,
		ProductRepo:             c.ProductRepo,
		ProductSKURepo:          c.ProductSKURepo,
		PaymentStore:            c.PaymentStore,
		ChannelStore:            c.PaymentChannelStore,
		WalletRepo:              c.WalletRepo,
		UserStore:               c.UserStore,
		ExternalIdentityStore:   c.ExternalIdentityStore,
		Queue:                   paymentqueue.New(c.QueueClient),
		WalletService:           c.WalletService,
		SettingService:          c.SettingService,
		DefaultEmailConfig:      c.Config.Email,
		ExpireMinutes:           c.Config.Order.PaymentExpireMinutes,
		AffiliateService:        c.AffiliateService,
		NotificationService:     c.NotificationService,
		PaymentProviderRegistry: c.PaymentProviderRegistry,
		ResellerAccounting:      c.ResellerAccountingLedger,
		AutoStockCounter:        c.CardSecretRepo,
	})
	c.TicketService = ticketapp.NewService(
		c.TicketStore,
		ticketorderadapter.New(c.OrderStore),
		ticketnotifier.New(c.NotificationService, c.QueueClient, c.UserStore, c.SettingService),
		uploadlocal.New("uploads"),
	)
	c.ProcurementOrderService = procurementapp.NewService(procurementapp.Options{
		Repository:         c.ProcurementOrderRepo,
		Orders:             procurementorder.New(c.OrderStore),
		ProductMappings:    procurementmapping.NewProducts(c.ProductMappingRepo),
		SKUMappings:        procurementmapping.NewSKUs(c.SKUMappingRepo),
		Connections:        procurementupstream.New(c.SiteConnectionService),
		Queue:              procurementqueue.New(c.QueueClient),
		OrderLifecycle:     c.ProcurementOrderRepo.NewLifecycle(c.QueueClient, c.SettingService, c.Config.Email),
		DownstreamCallback: c.DownstreamCallbackService,
		BotNotifier:        c.FulfillmentService,
		Notifications:      procurementnotification.New(c.NotificationService),
	})
	c.ReconciliationService = reconciliationapp.NewService(reconciliationapp.Options{
		Jobs: c.ReconciliationJobRepo, Items: c.ReconciliationItemRepo,
		Procurements:  reconciliationprocurement.New(c.ProcurementOrderRepo),
		Upstream:      reconciliationupstream.New(c.SiteConnectionService),
		Queue:         reconciliationqueue.New(c.QueueClient),
		Notifications: reconciliationnotification.New(c.NotificationService),
	})
	c.ChannelClientService = channelclientapp.NewService(c.ChannelClientStore, c.Config.App.SecretKey)
	c.TelegramBroadcastService = broadcastapp.NewService(
		c.TelegramBroadcastRepo,
		telegrambroadcast.NewUserDirectory(c.ExternalIdentityStore),
		telegrambroadcast.NewBotTokenResolver(c.ChannelClientService),
		telegrambroadcast.NewDispatcher(c.QueueClient),
		telegramNotifyService,
	)
}

// telegramNotifySenderAdapter 让 telegram/notify 应用服务满足 notification/contract.TelegramSender，
// 避免 notification/contract 直接依赖 telegram/notify/contract 造成循环引用。
type telegramNotifySenderAdapter struct {
	svc *notifyapp.Service
}

func (a telegramNotifySenderAdapter) SendMessage(ctx context.Context, chatID, message string) error {
	return a.svc.SendMessage(ctx, chatID, message)
}

func (a telegramNotifySenderAdapter) SendMessageWithOptions(ctx context.Context, options notificationcontract.TelegramSendOptions) error {
	return a.svc.SendMessageWithOptions(ctx, notifycontract.SendOptions{
		ChatID:                options.ChatID,
		Message:               options.Message,
		ParseMode:             options.ParseMode,
		DisableWebPagePreview: options.DisableWebPagePreview,
		AttachmentURL:         options.AttachmentURL,
		AttachmentDisplayName: options.AttachmentDisplayName,
		ReplyMarkup:           options.ReplyMarkup,
	})
}

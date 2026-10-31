package container

import (
	paymentsecurityalerts "github.com/dujiao-next/internal/modules/payment/application/securityalerts"
	paymentalertstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/securityalerts"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/securitytelegram"
	"time"

	catalogmappingbootstrap "github.com/dujiao-next/internal/bootstrap/catalogmapping"
	telegrambroadcast "github.com/dujiao-next/internal/bootstrap/telegrambroadcast"
	"github.com/dujiao-next/internal/config"
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
	notificationapp "github.com/dujiao-next/internal/modules/notification/application"
	notificationasyncqueue "github.com/dujiao-next/internal/modules/notification/infrastructure/asyncqueue"
	notificationfeishu "github.com/dujiao-next/internal/modules/notification/infrastructure/feishu"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	paymentcandidates "github.com/dujiao-next/internal/modules/payment/application/candidates"
	paymentchainverification "github.com/dujiao-next/internal/modules/payment/application/chainverification"
	paymentsettlement "github.com/dujiao-next/internal/modules/payment/application/settlement"
	paymentchainrpc "github.com/dujiao-next/internal/modules/payment/infrastructure/chainrpc"
	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"
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
	notifybotapi "github.com/dujiao-next/internal/modules/telegram/notify/infrastructure/botapi"
	"github.com/dujiao-next/internal/platform/database/gormdb"
)

// initIntegrationServices 装配通知、站点对接、支付、采购、渠道与 Telegram 集成。
func (c *Container) initIntegrationServices() {
	c.UserLoginLogService = auditlogapp.NewUserLoginService(c.UserLoginLogRepo)
	c.AuthzAuditService = auditlogapp.NewAuthzService(c.AuthzAuditLogRepo)
	c.AdminLoginLogService = auditlogapp.NewAdminLoginService(c.AdminLoginLogRepo)
	c.NotificationLogService = notificationapp.NewLogService(c.NotificationLogRepo)
	c.DashboardService = dashboardapp.NewService(c.DashboardRepo, c.SettingService)
	telegramNotifyService := notifyapp.NewService(c.SettingService, c.Config.TelegramAuth, notifybotapi.New())
	c.NotificationService = notificationapp.NewService(
		c.SettingService,
		c.EmailSender,
		notificationasyncqueue.New(c.QueueClient),
		c.DashboardService,
		c.NotificationLogService,
		telegramNotifyService,
		notificationfeishu.New(),
	)
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
		PaymentEffects: paymentgormstore.New(gormdb.DB, c.Config.App.SecretKey),
		References:     c.DownstreamOrderRefRepo,
		Orders:         downstreamcallbackorderreader.New(c.OrderStore),
		Credentials:    downstreamcallbackcredentialreader.New(c.ApiCredentialRepo),
		Queue:          downstreamQueue,
		Deliverer:      downstreamcallbackclient.New(),
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
	})
	c.PaymentCandidateService = &paymentcandidates.Service{Channels: c.PaymentChannelStore, Registry: c.PaymentProviderRegistry,
		Inbox: paymentgormstore.New(gormdb.DB, c.Config.App.SecretKey), Recovery: c.PaymentService}
	c.PaymentEffectsService = &paymentsettlement.Effects{Store: paymentgormstore.New(gormdb.DB, c.Config.App.SecretKey), Handler: c.PaymentService}
	if settings := c.Config.PaymentSecurityAlerts; settings.Enabled {
		if !c.Config.Queue.Enabled {
			panic("payment security alerts require worker queue")
		}
		sender, err := securitytelegram.New(settings.BotToken, settings.ChatID)
		if err != nil {
			panic("invalid payment security alert configuration")
		}
		c.PaymentSecurityAlertsService = &paymentsecurityalerts.Service{Store: paymentalertstore.New(gormdb.DB), Sender: sender}
	}

	if c.Config.PaymentChainVerification.SettlementEnabled && !c.Config.PaymentChainVerification.Enabled {
		panic("payment settlement requires chain verification")
	}
	if settings := c.Config.PaymentChainVerification; settings.Enabled {
		if !c.Config.Queue.Enabled {
			panic("payment chain verification requires worker queue")
		}
		networkPolicy := func(cfg config.PaymentRPCNetworkConfig) paymentchainrpc.NetworkPolicy {
			p := paymentchainrpc.NetworkPolicy{AnchorNumber: cfg.AnchorNumber, AnchorHash: cfg.AnchorHash}
			for _, source := range cfg.Sources {
				p.Sources = append(p.Sources, paymentchainrpc.Source{Name: source.Name, Provider: source.Provider, URL: source.URL, APIKey: source.APIKey})
			}
			return p
		}
		verifier, err := paymentchainrpc.New(paymentchainrpc.Policy{Version: settings.Version, MaxHeadAge: time.Duration(settings.MaxHeadAgeSeconds) * time.Second, BSC: networkPolicy(settings.BSC), TRON: networkPolicy(settings.TRON)})
		if err != nil {
			panic("invalid payment chain verification configuration")
		}
		c.PaymentChainVerificationService = &paymentchainverification.Service{Store: paymentgormstore.New(gormdb.DB, c.Config.App.SecretKey), Registry: c.PaymentProviderRegistry, Verifier: verifier}
		if settings.SettlementEnabled {
			c.PaymentSettlementService = &paymentsettlement.Service{Store: paymentgormstore.New(gormdb.DB, c.Config.App.SecretKey), Registry: c.PaymentProviderRegistry, Verifier: verifier, Apply: c.PaymentService.ApplyChainSettlement}
		}
	}
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

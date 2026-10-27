package container

import (
	"fmt"

	affiliategormstore "github.com/dujiao-next/internal/modules/affiliate/infrastructure/gormstore"
	apicredentialgormstore "github.com/dujiao-next/internal/modules/apicredential/infrastructure/gormstore"
	auditloggormstore "github.com/dujiao-next/internal/modules/auditlog/infrastructure/gormstore"
	cardsecretgormstore "github.com/dujiao-next/internal/modules/cardsecret/infrastructure/gormstore"
	cartgormstore "github.com/dujiao-next/internal/modules/cart/infrastructure/gormstore"
	categorygormstore "github.com/dujiao-next/internal/modules/catalog/category/infrastructure/gormstore"
	mappinggormstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	channelclientstore "github.com/dujiao-next/internal/modules/channelclient/infrastructure/gormstore"
	coupongormstore "github.com/dujiao-next/internal/modules/coupon/infrastructure/gormstore"
	dashboardgormstore "github.com/dujiao-next/internal/modules/dashboard/infrastructure/gormstore"
	downstreamcallbackgormstore "github.com/dujiao-next/internal/modules/downstreamcallback/infrastructure/gormstore"
	fulfillmentgormstore "github.com/dujiao-next/internal/modules/fulfillment/infrastructure/gormstore"
	giftcardgormstore "github.com/dujiao-next/internal/modules/giftcard/infrastructure/gormstore"
	adminstore "github.com/dujiao-next/internal/modules/identity/admin/infrastructure/gormstore"
	emailverificationstore "github.com/dujiao-next/internal/modules/identity/emailverification/infrastructure/gormstore"
	externalidentitystore "github.com/dujiao-next/internal/modules/identity/externalidentity/infrastructure/gormstore"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	memberlevelgormstore "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	notificationgormstore "github.com/dujiao-next/internal/modules/notification/infrastructure/gormstore"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"
	procurementgormstore "github.com/dujiao-next/internal/modules/procurement/infrastructure/gormstore"
	promotiongormstore "github.com/dujiao-next/internal/modules/promotion/infrastructure/gormstore"
	reconciliationgormstore "github.com/dujiao-next/internal/modules/reconciliation/infrastructure/gormstore"
	resellergormstore "github.com/dujiao-next/internal/modules/reseller/infrastructure/gormstore"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	siteconnectiongormstore "github.com/dujiao-next/internal/modules/siteconnection/infrastructure/gormstore"
	broadcaststore "github.com/dujiao-next/internal/modules/telegram/broadcast/infrastructure/gormstore"
	walletgormstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
)

func (c *Container) initRepositories() error {
	db := gormdb.DB
	c.AdminStore = adminstore.New(db)
	c.UserStore = userstore.New(db)
	c.ExternalIdentityStore = externalidentitystore.New(db)
	c.EmailVerificationStore = emailverificationstore.New(db)
	orderStore := ordergormstore.New(db, c.Config.App.SecretKey)
	if _, err := orderStore.BackfillGuestCredentialHashes(); err != nil {
		return fmt.Errorf("backfill guest order credentials: %w", err)
	}
	c.OrderStore = orderStore
	c.PaymentStore = paymentgormstore.New(db, c.Config.App.SecretKey)
	c.PaymentChannelStore = paymentgormstore.NewChannelStore(db)
	c.CardSecretRepo = cardsecretgormstore.New(db)
	c.CardSecretBatchRepo = cardsecretgormstore.NewBatch(db)
	c.GiftCardRepo = giftcardgormstore.New(db)
	c.FulfillmentStore = fulfillmentgormstore.New(db)
	c.ProductRepo = productgormstore.NewProductStore(db)
	c.ProductSKURepo = productgormstore.NewSKUStore(db)
	c.CartRepo = cartgormstore.New(db)
	c.CouponRepo = coupongormstore.New(db)
	c.CouponUsageRepo = coupongormstore.NewUsageStore(db)
	c.PromotionRepo = promotiongormstore.New(db)
	c.WalletRepo = walletgormstore.New(db)
	c.CategoryRepo = categorygormstore.NewCategoryStore(db)
	c.SettingRepo = settingsstore.New(db)
	c.UserLoginLogRepo = auditloggormstore.NewUserLoginStore(db)
	c.AuthzAuditLogRepo = auditloggormstore.NewAuthzStore(db)
	c.NotificationLogRepo = notificationgormstore.NewLogStore(db)
	c.AdminLoginLogRepo = auditloggormstore.NewAdminLoginStore(db)
	c.DashboardRepo = dashboardgormstore.New(db)
	c.AffiliateRepo = affiliategormstore.New(db)
	c.ResellerStore = resellergormstore.New(db)
	c.ApiCredentialRepo = apicredentialgormstore.New(db)
	c.SiteConnectionRepo = siteconnectiongormstore.New(db)
	c.ProductMappingRepo = mappinggormstore.NewMappingStore(db)
	c.SKUMappingRepo = mappinggormstore.NewSKUMappingStore(db)
	c.ProcurementOrderRepo = procurementgormstore.New(db)
	c.DownstreamOrderRefRepo = downstreamcallbackgormstore.New(db)
	c.ReconciliationJobRepo = reconciliationgormstore.NewJobStore(db)
	c.ReconciliationItemRepo = reconciliationgormstore.NewItemStore(db)
	c.ChannelClientStore = channelclientstore.New(db)
	c.TelegramBroadcastRepo = broadcaststore.New(db)
	c.MemberLevelRepo = memberlevelgormstore.NewLevelStore(db)
	c.MemberLevelPriceRepo = memberlevelgormstore.NewPriceStore(db)
	c.UserProductPriceRepo = memberlevelgormstore.NewUserPriceStore(db)
	c.MemberLevelUserRepo = memberlevelgormstore.NewUserStore(db)
	return nil
}

package channelwiring

import (
	"errors"
	"fmt"

	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderapp "github.com/dujiao-next/internal/modules/order/application"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/app/container"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	"github.com/dujiao-next/internal/modules/catalog/product/manualform"
	channeltransport "github.com/dujiao-next/internal/modules/channelapi/transport/http"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	orderriskcontract "github.com/dujiao-next/internal/modules/orderrisk/contract"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
)

// NewHandler connects application services to the channel HTTP
// transport. Conversion stays at the composition boundary so transport can
// depend on narrow contracts only.
func NewHandler(c *container.Container) *channeltransport.Handler {
	return channeltransport.New(channeltransport.Dependencies{
		CategoryService: c.CategoryService, CategoryRepo: c.CategoryRepo,
		ProductService: c.ProductReadService, ProductRepo: c.ProductRepo,
		ProductMappingRepo: c.ProductMappingRepo, SKUMappingRepo: c.SKUMappingRepo,
		UserAuthService: identityAdapter{auth: c.UserAuthService}, MemberLevelService: c.MemberLevelService,
		SettingService: c.SettingService, OrderService: orderAdapter{orders: c.OrderService},
		PaymentService: paymentAdapter{payments: c.PaymentService}, PaymentStore: c.PaymentStore,
	})
}

type identityAdapter struct {
	auth *userauthapp.Service
}

func identityServiceInput(input channeltransport.TelegramIdentityInput) userauthapp.TelegramChannelIdentityInput {
	return userauthapp.TelegramChannelIdentityInput{
		ChannelUserID: input.ChannelUserID, Username: input.Username,
		FirstName: input.FirstName, LastName: input.LastName, AvatarURL: input.AvatarURL,
		InviteCode: input.InviteCode,
	}
}

func (a identityAdapter) ResolveTelegramChannelIdentity(input channeltransport.TelegramIdentityInput) (*userdomain.User, *externalidentitydomain.Identity, error) {
	return a.auth.ResolveTelegramChannelIdentity(identityServiceInput(input))
}

func (a identityAdapter) ProvisionTelegramChannelIdentity(input channeltransport.TelegramIdentityInput) (*userdomain.User, *externalidentitydomain.Identity, bool, error) {
	return a.auth.ProvisionTelegramChannelIdentity(identityServiceInput(input))
}

func (a identityAdapter) ProvisionTelegramChannelUserID(input channeltransport.TelegramIdentityInput) (uint, error) {
	user, _, _, err := a.auth.ProvisionTelegramChannelIdentity(identityServiceInput(input))
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, userauthapp.ErrNotFound
	}
	return user.ID, nil
}

func (a identityAdapter) BindTelegramChannelByEmailCode(input channeltransport.BindTelegramIdentityInput) (*userdomain.User, *externalidentitydomain.Identity, uint, error) {
	return a.auth.BindTelegramChannelByEmailCode(userauthapp.BindTelegramChannelByEmailCodeInput{
		Identity: identityServiceInput(input.Identity), Email: input.Email, Code: input.Code,
	})
}

type orderAdapter struct {
	orders *orderapp.OrderService
}

func createOrderServiceInput(input channeltransport.CreateOrderInput) orderapp.CreateOrderInput {
	items := make([]orderapp.CreateOrderItem, 0, len(input.Items))
	for _, item := range input.Items {
		items = append(items, orderapp.CreateOrderItem{
			ProductID: item.ProductID, SKUID: item.SKUID, Quantity: item.Quantity, FulfillmentType: item.FulfillmentType,
		})
	}
	return orderapp.CreateOrderInput{
		UserID: input.UserID, Items: items, CouponCode: input.CouponCode,
		AffiliateCode: input.AffiliateCode, AffiliateVisitorKey: input.AffiliateVisitorKey,
		ClientIP: input.ClientIP, ManualFormData: input.ManualFormData, SkipIPRiskControl: input.SkipIPRiskControl,
	}
}

func (a orderAdapter) PreviewOrder(input channeltransport.CreateOrderInput) (*channeltransport.OrderPreview, error) {
	preview, err := a.orders.PreviewOrder(createOrderServiceInput(input))
	if err != nil {
		return nil, mapError(err)
	}
	if preview == nil {
		return nil, nil
	}
	items := make([]channeltransport.OrderPreviewItem, 0, len(preview.Items))
	for _, item := range preview.Items {
		items = append(items, channeltransport.OrderPreviewItem{
			ProductID: item.ProductID, SKUID: item.SKUID, TitleJSON: item.TitleJSON, SKUSnapshotJSON: item.SKUSnapshotJSON,
			OriginalUnitPrice: item.OriginalUnitPrice, UnitPrice: item.UnitPrice, Quantity: item.Quantity,
			OriginalTotalPrice: item.OriginalTotalPrice, TotalPrice: item.TotalPrice,
			CouponDiscount: item.CouponDiscount, PromotionDiscount: item.PromotionDiscount,
			WholesaleDiscount: item.WholesaleDiscount, FulfillmentType: item.FulfillmentType,
		})
	}
	return &channeltransport.OrderPreview{
		Currency: preview.Currency, OriginalAmount: preview.OriginalAmount, DiscountAmount: preview.DiscountAmount,
		PromotionDiscountAmount: preview.PromotionDiscountAmount, WholesaleDiscountAmount: preview.WholesaleDiscountAmount,
		TotalAmount: preview.TotalAmount, Items: items,
	}, nil
}

func (a orderAdapter) CreateOrder(input channeltransport.CreateOrderInput) (*orderdomain.Order, error) {
	order, err := a.orders.CreateOrder(createOrderServiceInput(input))
	return order, mapError(err)
}

func (a orderAdapter) GetOrderByUser(orderID, userID uint) (*orderdomain.Order, error) {
	order, err := a.orders.GetOrderByUser(orderID, userID)
	return order, mapError(err)
}

func (a orderAdapter) GetOrderByUserOrderNo(orderNo string, userID uint) (*orderdomain.Order, error) {
	order, err := a.orders.GetOrderByUserOrderNo(orderNo, userID)
	return order, mapError(err)
}

func (a orderAdapter) CancelOrder(orderID, userID uint) (*orderdomain.Order, error) {
	order, err := a.orders.CancelOrder(orderID, userID)
	return order, mapError(err)
}

func (a orderAdapter) ListOrdersByUser(filter channeltransport.OrderListFilter) ([]orderdomain.Order, int64, error) {
	return a.orders.ListOrdersByUser(ordercontract.ListFilter{
		Page: filter.Page, PageSize: filter.PageSize, UserID: filter.UserID, Status: filter.Status,
	})
}

type paymentAdapter struct {
	payments *paymentapp.PaymentService
}

func (a paymentAdapter) GetWalletRechargeChannels() ([]paymentdomain.PaymentChannel, error) {
	return a.payments.GetWalletRechargeChannels()
}

func (a paymentAdapter) ListChannels(filter channeltransport.PaymentChannelListFilter) ([]paymentdomain.PaymentChannel, int64, error) {
	return a.payments.ListChannels(paymentcontract.ChannelListFilter{
		Page: filter.Page, PageSize: filter.PageSize, ActiveOnly: filter.ActiveOnly,
	})
}

func (a paymentAdapter) GetAllowedChannelsForProducts(productIDs []uint) ([]paymentdomain.PaymentChannel, error) {
	return a.payments.GetAllowedChannelsForProducts(productIDs)
}

func (a paymentAdapter) CreatePayment(input channeltransport.CreatePaymentInput) (*channeltransport.CreatePaymentResult, error) {
	result, err := a.payments.CreatePayment(paymentapp.CreatePaymentInput{
		OrderID: input.OrderID, ChannelID: input.ChannelID, UseBalance: input.UseBalance,
		ClientIP: input.ClientIP, Context: input.Context,
	})
	if err != nil {
		return nil, mapError(err)
	}
	if result == nil {
		return nil, nil
	}
	return &channeltransport.CreatePaymentResult{
		Payment: result.Payment, Channel: result.Channel, OrderPaid: result.OrderPaid,
		WalletPaidAmount: result.WalletPaidAmount, OnlinePayAmount: result.OnlinePayAmount,
	}, nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	for _, mapping := range []struct {
		source error
		target error
	}{
		{orderriskcontract.ErrIPBlacklisted, channeltransport.ErrRiskIPBlacklisted},
		{orderriskcontract.ErrClientIPUnavailable, channeltransport.ErrRiskClientIPUnavailable},
		{orderriskcontract.ErrTooManyPendingOrders, channeltransport.ErrRiskTooManyPendingOrders},
		{orderriskcontract.ErrProductQuantityLimit, channeltransport.ErrRiskProductQuantityLimit},
		{orderriskcontract.ErrPendingProductQuantityLimit, channeltransport.ErrRiskPendingProductLimit},
		{orderriskcontract.ErrOrderRateLimited, channeltransport.ErrRiskOrderRateLimited},
		{orderapp.ErrProductSKURequired, channeltransport.ErrProductSKURequired},
		{orderapp.ErrProductSKUInvalid, channeltransport.ErrProductSKUInvalid},
		{orderapp.ErrInvalidOrderItem, channeltransport.ErrInvalidOrderItem},
		{orderapp.ErrInvalidOrderAmount, channeltransport.ErrInvalidOrderAmount},
		{orderapp.ErrProductPurchaseNotAllowed, channeltransport.ErrProductPurchaseNotAllowed},
		{orderapp.ErrProductMaxPurchaseExceeded, channeltransport.ErrProductMaxPurchaseExceeded},
		{orderapp.ErrProductMinPurchaseNotMet, channeltransport.ErrProductMinPurchaseNotMet},
		{orderapp.ErrProductNotAvailable, channeltransport.ErrProductNotAvailable},
		{orderapp.ErrManualStockInsufficient, channeltransport.ErrManualStockInsufficient},
		{orderapp.ErrCardSecretInsufficient, channeltransport.ErrCardSecretInsufficient},
		{orderapp.ErrOrderCurrencyMismatch, channeltransport.ErrOrderCurrencyMismatch},
		{productcontract.ErrProductPriceInvalid, channeltransport.ErrProductPriceInvalid},
		{manualform.ErrSchemaInvalid, channeltransport.ErrManualFormSchemaInvalid},
		{manualform.ErrRequiredMissing, channeltransport.ErrManualFormRequiredMissing},
		{manualform.ErrFieldInvalid, channeltransport.ErrManualFormFieldInvalid},
		{manualform.ErrTypeInvalid, channeltransport.ErrManualFormTypeInvalid},
		{manualform.ErrOptionInvalid, channeltransport.ErrManualFormOptionInvalid},
		{paymentapp.ErrPaymentInvalid, channeltransport.ErrPaymentInvalid},
		{orderapp.ErrOrderNotFound, channeltransport.ErrOrderNotFound},
		{orderapp.ErrOrderStatusInvalid, channeltransport.ErrOrderStatusInvalid},
		{paymentapp.ErrPaymentChannelNotFound, channeltransport.ErrPaymentChannelNotFound},
		{paymentapp.ErrPaymentChannelInactive, channeltransport.ErrPaymentChannelInactive},
		{paymentapp.ErrPaymentProviderNotSupported, channeltransport.ErrPaymentProviderUnsupported},
		{paymentapp.ErrPaymentChannelConfigInvalid, channeltransport.ErrPaymentChannelConfigInvalid},
		{paymentapp.ErrPaymentGatewayRequestFailed, channeltransport.ErrPaymentGatewayRequestFailed},
		{paymentapp.ErrPaymentGatewayResponseInvalid, channeltransport.ErrPaymentGatewayResponseInvalid},
		{paymentapp.ErrPaymentCurrencyMismatch, channeltransport.ErrPaymentCurrencyMismatch},
		{walletcontract.ErrOnlyPaymentRequired, channeltransport.ErrWalletOnlyPaymentRequired},
	} {
		if errors.Is(err, mapping.source) {
			mapped := fmt.Errorf("%w: %v", mapping.target, err)
			return channeltransport.WithRetryAfter(mapped, orderriskcontract.GetRetryAfter(err))
		}
	}
	return channeltransport.WithRetryAfter(err, orderriskcontract.GetRetryAfter(err))
}

package upstreamwiring

import (
	"errors"
	"fmt"
	"time"

	paymentapp "github.com/dujiao-next/internal/modules/payment/application"

	orderapp "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/app/container"
	productapplication "github.com/dujiao-next/internal/modules/catalog/product/application"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	"github.com/dujiao-next/internal/modules/catalog/product/manualform"
	upstreamtransport "github.com/dujiao-next/internal/modules/upstreamapi/transport/http"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// NewHandler connects application services to the upstream HTTP
// transport without leaking concrete implementations into transport.
func NewHandler(c *container.Container) *upstreamtransport.Handler {
	return upstreamtransport.New(upstreamtransport.Dependencies{
		Categories:        c.CategoryRepo,
		Products:          productServiceAdapter{products: c.ProductReadService},
		Users:             c.UserStore,
		ProductRepository: c.ProductRepo,
		SKUs:              c.ProductSKURepo,
		ProductMappings:   c.ProductMappingRepo,
		SKUMappings:       c.SKUMappingRepo,
		MemberLevels:      c.MemberLevelService,
		Settings:          c.SettingService,
		Wallet:            c.WalletService,
		Orders:            orderServiceAdapter{orders: c.OrderService},
		Payments:          paymentServiceAdapter{payments: c.PaymentService},
		Procurements:      c.ProcurementOrderService,
		DownstreamRefs:    c.DownstreamOrderRefRepo,
		Connections:       c.SiteConnectionRepo,
		ConnectionSecrets: c.SiteConnectionService,
	})
}

type productServiceAdapter struct {
	products *productapplication.Service
}

func (a productServiceAdapter) ListForUpstreamSync(updatedAfter *time.Time, includeInactive bool, page, pageSize int) ([]productdomain.Product, int64, error) {
	return a.products.ListForUpstreamSync(updatedAfter, includeInactive, page, pageSize)
}

func (a productServiceAdapter) ApplyAutoStockCounts(products []productdomain.Product) error {
	return a.products.ApplyAutoStockCounts(products)
}

func (a productServiceAdapter) GetAdminByID(id string) (*productdomain.Product, error) {
	product, err := a.products.GetAdminByID(id)
	if errors.Is(err, productcontract.ErrNotFound) {
		return nil, fmt.Errorf("%w: %v", upstreamtransport.ErrProductNotFound, err)
	}
	return product, err
}

type orderServiceAdapter struct {
	orders *orderapp.OrderService
}

func (a orderServiceAdapter) CreateOrder(input upstreamtransport.CreateOrderInput) (*orderdomain.Order, error) {
	items := make([]orderapp.CreateOrderItem, 0, len(input.Items))
	for _, item := range input.Items {
		items = append(items, orderapp.CreateOrderItem{
			ProductID: item.ProductID, SKUID: item.SKUID, Quantity: item.Quantity, FulfillmentType: item.FulfillmentType,
		})
	}
	order, err := a.orders.CreateOrder(orderapp.CreateOrderInput{
		UserID: input.UserID, Items: items, ClientIP: input.ClientIP,
		ManualFormData: input.ManualFormData, SkipRiskControl: input.SkipRiskControl,
		IsApiOrder: input.IsApiOrder,
	})
	return order, mapOrderError(err)
}

func (a orderServiceAdapter) GetOrderByUser(orderID, userID uint) (*orderdomain.Order, error) {
	order, err := a.orders.GetOrderByUser(orderID, userID)
	return order, mapOrderError(err)
}

func (a orderServiceAdapter) CancelOrder(orderID, userID uint) (*orderdomain.Order, error) {
	order, err := a.orders.CancelOrder(orderID, userID)
	return order, mapOrderError(err)
}

func (a orderServiceAdapter) BuildLocalRefundRecordsForOrder(order *orderdomain.Order) ([]jsonmap.JSON, error) {
	return a.orders.BuildLocalRefundRecordsForOrder(order)
}

type paymentServiceAdapter struct {
	payments *paymentapp.PaymentService
}

func (a paymentServiceAdapter) CreatePayment(input upstreamtransport.CreatePaymentInput) (*upstreamtransport.CreatePaymentResult, error) {
	result, err := a.payments.CreatePayment(paymentapp.CreatePaymentInput{
		OrderID: input.OrderID, UseBalance: input.UseBalance, ClientIP: input.ClientIP,
	})
	if err != nil || result == nil {
		return nil, err
	}
	return &upstreamtransport.CreatePaymentResult{OrderPaid: result.OrderPaid}, nil
}

func mapOrderError(err error) error {
	if err == nil {
		return nil
	}
	for _, mapping := range []struct {
		sources []error
		target  error
	}{
		{[]error{orderapp.ErrOrderNotFound}, upstreamtransport.ErrOrderNotFound},
		{[]error{orderapp.ErrOrderCancelNotAllowed}, upstreamtransport.ErrOrderCancelNotAllowed},
		{[]error{walletcontract.ErrInsufficientBalance}, upstreamtransport.ErrWalletInsufficient},
		{[]error{orderapp.ErrCardSecretInsufficient, orderapp.ErrManualStockInsufficient}, upstreamtransport.ErrStockInsufficient},
		{[]error{orderapp.ErrProductNotAvailable, productcontract.ErrNotFound}, upstreamtransport.ErrProductUnavailable},
		{[]error{orderapp.ErrProductSKUInvalid, orderapp.ErrProductSKURequired}, upstreamtransport.ErrSKUUnavailable},
		{[]error{orderapp.ErrInvalidOrderItem}, upstreamtransport.ErrInvalidOrderItem},
		{[]error{manualform.ErrRequiredMissing, manualform.ErrFieldInvalid, manualform.ErrTypeInvalid, manualform.ErrOptionInvalid}, upstreamtransport.ErrManualFormInvalid},
	} {
		for _, source := range mapping.sources {
			if errors.Is(err, source) {
				return fmt.Errorf("%w: %v", mapping.target, err)
			}
		}
	}
	return err
}

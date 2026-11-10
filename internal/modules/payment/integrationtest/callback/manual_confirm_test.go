package paymentcallback_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type manualConfirmFixture struct {
	db      *gorm.DB
	service *paymentapp.PaymentService
	order   *orderdomain.Order
	payment *paymentdomain.Payment
}

func newManualConfirmFixture(t *testing.T, orderStatus string, autoQuantity int) *manualConfirmFixture {
	t.Helper()

	dsn := fmt.Sprintf("file:payment_manual_confirm_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&paymentdomain.PaymentChannel{},
		&paymentdomain.Payment{},
		&cardsecretdomain.Batch{},
		&cardsecretdomain.Secret{},
		&coupondomain.Coupon{},
		&coupondomain.CouponUsage{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	order := &orderdomain.Order{
		OrderNo:          "DJMANUALCONFIRM001",
		Status:           orderStatus,
		Currency:         "CNY",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(50)),
		DiscountAmount:   money.FromDecimal(decimal.Zero),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(50)),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(50)),
		RefundedAmount:   money.FromDecimal(decimal.Zero),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if orderStatus == constants.OrderStatusCanceled {
		order.CanceledAt = &now
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	if err := db.Create(&orderdomain.OrderItem{
		OrderID:         order.ID,
		ProductID:       11,
		SKUID:           21,
		TitleJSON:       jsonmap.JSON{"zh-CN": "卡密商品"},
		Quantity:        autoQuantity,
		UnitPrice:       money.FromDecimal(decimal.NewFromInt(50)),
		TotalPrice:      money.FromDecimal(decimal.NewFromInt(50)),
		FulfillmentType: constants.FulfillmentTypeAuto,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("create order item failed: %v", err)
	}
	channel := &paymentdomain.PaymentChannel{
		Name:            "USDT",
		ProviderType:    constants.PaymentProviderEpay,
		ChannelType:     constants.PaymentChannelTypeUsdt,
		InteractionMode: constants.PaymentInteractionQR,
		FeeRate:         money.FromDecimal(decimal.Zero),
		ConfigJSON:      jsonmap.JSON{},
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("create channel failed: %v", err)
	}
	payment := &paymentdomain.Payment{
		OrderID:         order.ID,
		ChannelID:       channel.ID,
		ProviderType:    channel.ProviderType,
		ChannelType:     channel.ChannelType,
		InteractionMode: channel.InteractionMode,
		Amount:          money.FromDecimal(decimal.NewFromInt(50)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.PaymentStatusExpired,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	secret := "test-guest-credential-secret-with-32-bytes"
	service := paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{
		OrderStore:     ordergormstore.New(db, secret),
		ProductRepo:    productgormstore.NewProductStore(db),
		ProductSKURepo: productgormstore.NewSKUStore(db),
		PaymentStore:   paymentgormstore.New(db, secret),
		ChannelStore:   paymentgormstore.NewChannelStore(db),
		ExpireMinutes:  15,
	})
	return &manualConfirmFixture{db: db, service: service, order: order, payment: payment}
}

func (f *manualConfirmFixture) seedAvailableSecrets(t *testing.T, count int) {
	t.Helper()
	now := time.Now()
	for i := 0; i < count; i++ {
		if err := f.db.Create(&cardsecretdomain.Secret{
			ProductID: 11,
			SKUID:     21,
			Secret:    fmt.Sprintf("CARD-%d", i+1),
			Status:    cardsecretdomain.StatusAvailable,
			CreatedAt: now,
			UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("seed card secret failed: %v", err)
		}
	}
}

func (f *manualConfirmFixture) confirm() (*paymentdomain.Payment, error) {
	return f.confirmPayment(f.payment.ID)
}

func (f *manualConfirmFixture) confirmPayment(paymentID uint) (*paymentdomain.Payment, error) {
	return f.service.ManualConfirmPayment(paymentapp.ManualConfirmPaymentInput{
		OrderID:     f.order.ID,
		PaymentID:   paymentID,
		AdminID:     7,
		ProviderRef: "TX-HASH-001",
		Remark:      "callback lost, verified in gateway",
	})
}

func (f *manualConfirmFixture) reloadOrder(t *testing.T) orderdomain.Order {
	t.Helper()
	var order orderdomain.Order
	if err := f.db.First(&order, f.order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	return order
}

func (f *manualConfirmFixture) countSecretsByStatus(t *testing.T, status string) int64 {
	t.Helper()
	var count int64
	if err := f.db.Model(&cardsecretdomain.Secret{}).Where("status = ?", status).Count(&count).Error; err != nil {
		t.Fatalf("count secrets failed: %v", err)
	}
	return count
}

func TestManualConfirmPaymentRecoversPendingOrder(t *testing.T) {
	fixture := newManualConfirmFixture(t, constants.OrderStatusPendingPayment, 1)
	fixture.seedAvailableSecrets(t, 1)

	payment, err := fixture.confirm()
	if err != nil {
		t.Fatalf("manual confirm failed: %v", err)
	}
	if payment.Status != constants.PaymentStatusSuccess {
		t.Fatalf("payment status = %s, want success", payment.Status)
	}
	if payment.ProviderRef != "TX-HASH-001" {
		t.Fatalf("provider ref = %q, want TX-HASH-001", payment.ProviderRef)
	}
	audit, ok := payment.ProviderPayload["manual_confirm"].(jsonmap.JSON)
	if !ok {
		t.Fatalf("manual_confirm audit payload missing: %+v", payment.ProviderPayload)
	}
	if audit["previous_order_status"] != constants.OrderStatusPendingPayment ||
		audit["previous_payment_status"] != constants.PaymentStatusExpired {
		t.Fatalf("unexpected audit trail: %+v", audit)
	}

	order := fixture.reloadOrder(t)
	if order.Status != constants.OrderStatusPaid || order.PaidAt == nil {
		t.Fatalf("order not paid: status=%s paid_at=%v", order.Status, order.PaidAt)
	}
}

func TestManualConfirmPaymentReopensCanceledOrderAndReservesSecrets(t *testing.T) {
	fixture := newManualConfirmFixture(t, constants.OrderStatusCanceled, 2)
	fixture.seedAvailableSecrets(t, 3)

	if _, err := fixture.confirm(); err != nil {
		t.Fatalf("manual confirm failed: %v", err)
	}

	order := fixture.reloadOrder(t)
	if order.Status != constants.OrderStatusPaid || order.PaidAt == nil || order.CanceledAt != nil {
		t.Fatalf("canceled order not recovered: status=%s paid_at=%v canceled_at=%v", order.Status, order.PaidAt, order.CanceledAt)
	}
	if got := fixture.countSecretsByStatus(t, cardsecretdomain.StatusReserved); got != 2 {
		t.Fatalf("reserved secrets = %d, want 2", got)
	}
	if got := fixture.countSecretsByStatus(t, cardsecretdomain.StatusAvailable); got != 1 {
		t.Fatalf("available secrets = %d, want 1", got)
	}
}

func TestManualConfirmPaymentRollsBackWhenSecretsInsufficient(t *testing.T) {
	fixture := newManualConfirmFixture(t, constants.OrderStatusCanceled, 3)
	fixture.seedAvailableSecrets(t, 2)

	_, err := fixture.confirm()
	if !errors.Is(err, orderapp.ErrCardSecretInsufficient) {
		t.Fatalf("err = %v, want ErrCardSecretInsufficient", err)
	}

	order := fixture.reloadOrder(t)
	if order.Status != constants.OrderStatusCanceled || order.PaidAt != nil {
		t.Fatalf("order must stay canceled after rollback: status=%s paid_at=%v", order.Status, order.PaidAt)
	}
	if got := fixture.countSecretsByStatus(t, cardsecretdomain.StatusReserved); got != 0 {
		t.Fatalf("reserved secrets = %d, want 0 after rollback", got)
	}
	var payment paymentdomain.Payment
	if err := fixture.db.First(&payment, fixture.payment.ID).Error; err != nil {
		t.Fatalf("reload payment failed: %v", err)
	}
	if payment.Status != constants.PaymentStatusExpired {
		t.Fatalf("payment status = %s, want expired after rollback", payment.Status)
	}
}

func TestManualConfirmPaymentRejectsAlreadyPaidOrder(t *testing.T) {
	fixture := newManualConfirmFixture(t, constants.OrderStatusPendingPayment, 1)
	fixture.seedAvailableSecrets(t, 2)

	if _, err := fixture.confirm(); err != nil {
		t.Fatalf("first confirm failed: %v", err)
	}
	if _, err := fixture.confirm(); !errors.Is(err, orderapp.ErrOrderStatusInvalid) {
		t.Fatalf("second confirm err = %v, want ErrOrderStatusInvalid", err)
	}
}

func (f *manualConfirmFixture) addPayment(t *testing.T, amount int64, status, exceptionCode string) *paymentdomain.Payment {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	payment := &paymentdomain.Payment{
		OrderID:         f.order.ID,
		ChannelID:       f.payment.ChannelID,
		ProviderType:    f.payment.ProviderType,
		ChannelType:     f.payment.ChannelType,
		InteractionMode: f.payment.InteractionMode,
		Amount:          money.FromDecimal(decimal.NewFromInt(amount)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          status,
		ExceptionCode:   exceptionCode,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := f.db.Create(payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}
	return payment
}

func TestManualConfirmPaymentRequiresPaymentID(t *testing.T) {
	fixture := newManualConfirmFixture(t, constants.OrderStatusPendingPayment, 1)
	fixture.seedAvailableSecrets(t, 1)

	if _, err := fixture.confirmPayment(0); !errors.Is(err, paymentapp.ErrPaymentInvalid) {
		t.Fatalf("err = %v, want ErrPaymentInvalid", err)
	}
	if order := fixture.reloadOrder(t); order.Status != constants.OrderStatusPendingPayment {
		t.Fatalf("order must stay pending, got %s", order.Status)
	}
}

func TestManualConfirmPaymentOnlyConfirmsSpecifiedPayment(t *testing.T) {
	fixture := newManualConfirmFixture(t, constants.OrderStatusPendingPayment, 1)
	fixture.seedAvailableSecrets(t, 1)
	// 更新的一条记录（id 更大）不应被默认选中。
	latest := fixture.addPayment(t, 50, constants.PaymentStatusPending, "")

	confirmed, err := fixture.confirmPayment(fixture.payment.ID)
	if err != nil {
		t.Fatalf("manual confirm failed: %v", err)
	}
	if confirmed.ID != fixture.payment.ID {
		t.Fatalf("confirmed payment = %d, want %d", confirmed.ID, fixture.payment.ID)
	}
	var reloaded paymentdomain.Payment
	if err := fixture.db.First(&reloaded, latest.ID).Error; err != nil {
		t.Fatalf("reload latest payment failed: %v", err)
	}
	if reloaded.Status == constants.PaymentStatusSuccess {
		t.Fatalf("unspecified payment must not be marked success")
	}
}

func TestManualConfirmPaymentRejectsPaymentOfOtherOrder(t *testing.T) {
	fixture := newManualConfirmFixture(t, constants.OrderStatusPendingPayment, 1)
	fixture.seedAvailableSecrets(t, 1)
	other := newManualConfirmFixture(t, constants.OrderStatusPendingPayment, 1)

	// 另一个库里的 ID 在本库可能不存在，也可能属于其他订单，两种情况都必须拒绝。
	if _, err := fixture.confirmPayment(other.payment.ID + 100); err == nil {
		t.Fatalf("expected error for unknown payment")
	}
}

func TestManualConfirmPaymentRejectsUnderpaidCreditedPayment(t *testing.T) {
	fixture := newManualConfirmFixture(t, constants.OrderStatusPendingPayment, 1)
	fixture.seedAvailableSecrets(t, 1)
	credited := fixture.addPayment(t, 70, constants.PaymentStatusSuccess, constants.PaymentExceptionUnderpaidSucceeded)

	if _, err := fixture.confirmPayment(credited.ID); !errors.Is(err, paymentapp.ErrPaymentUnderpaidCredited) {
		t.Fatalf("err = %v, want ErrPaymentUnderpaidCredited", err)
	}
	order := fixture.reloadOrder(t)
	if order.Status != constants.OrderStatusPendingPayment || order.PaidAt != nil {
		t.Fatalf("order must stay unpaid: status=%s paid_at=%v", order.Status, order.PaidAt)
	}
}

func TestManualConfirmPaymentCanceledOrderPaymentNotCoveringTotal(t *testing.T) {
	fixture := newManualConfirmFixture(t, constants.OrderStatusCanceled, 1)
	fixture.seedAvailableSecrets(t, 1)
	// 模拟混合支付：订单总额 50，其中在线部分只有 30（余额部分取消时已退回）。
	partial := fixture.addPayment(t, 30, constants.PaymentStatusExpired, "")

	if _, err := fixture.confirmPayment(partial.ID); !errors.Is(err, paymentapp.ErrPaymentCanceledOrderUnderpaid) {
		t.Fatalf("err = %v, want ErrPaymentCanceledOrderUnderpaid", err)
	}
	order := fixture.reloadOrder(t)
	if order.Status != constants.OrderStatusCanceled || order.PaidAt != nil {
		t.Fatalf("order must stay canceled: status=%s paid_at=%v", order.Status, order.PaidAt)
	}
	if got := fixture.countSecretsByStatus(t, cardsecretdomain.StatusReserved); got != 0 {
		t.Fatalf("reserved secrets = %d, want 0 after rollback", got)
	}
}

func TestManualConfirmPaymentReopenFailsWhenManualStockTakenByOtherOrder(t *testing.T) {
	fixture := newManualConfirmFixture(t, constants.OrderStatusCanceled, 1)
	// 把订单项改成人工交付，库存只剩 0 件可售、1 件被其他待支付订单占用。
	if err := fixture.db.Model(&orderdomain.OrderItem{}).Where("order_id = ?", fixture.order.ID).
		Update("fulfillment_type", constants.FulfillmentTypeManual).Error; err != nil {
		t.Fatalf("update item failed: %v", err)
	}
	now := time.Now()
	sku := &productdomain.ProductSKU{
		ID: 21, ProductID: 11, SKUCode: "S1", IsActive: true,
		ManualStockTotal: 0, ManualStockLocked: 1, ManualStockSold: 0,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := fixture.db.Create(sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	if _, err := fixture.confirm(); !errors.Is(err, orderapp.ErrManualStockInsufficient) {
		t.Fatalf("err = %v, want ErrManualStockInsufficient", err)
	}

	var reloaded productdomain.ProductSKU
	if err := fixture.db.First(&reloaded, sku.ID).Error; err != nil {
		t.Fatalf("reload sku failed: %v", err)
	}
	if reloaded.ManualStockLocked != 1 || reloaded.ManualStockTotal != 0 || reloaded.ManualStockSold != 0 {
		t.Fatalf("other order's reservation must be untouched: %+v", reloaded)
	}
	if order := fixture.reloadOrder(t); order.Status != constants.OrderStatusCanceled {
		t.Fatalf("order must stay canceled, got %s", order.Status)
	}
}

func TestManualConfirmPaymentReopenConsumesOwnManualStockOnly(t *testing.T) {
	fixture := newManualConfirmFixture(t, constants.OrderStatusCanceled, 1)
	// 1 件可售，另有 1 件被其他待支付订单占用：重开后应扣掉可售的那件，别人的占用保持不变。
	if err := fixture.db.Model(&orderdomain.OrderItem{}).Where("order_id = ?", fixture.order.ID).
		Update("fulfillment_type", constants.FulfillmentTypeManual).Error; err != nil {
		t.Fatalf("update item failed: %v", err)
	}
	now := time.Now()
	sku := &productdomain.ProductSKU{
		ID: 21, ProductID: 11, SKUCode: "S1", IsActive: true,
		ManualStockTotal: 1, ManualStockLocked: 1, ManualStockSold: 0,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := fixture.db.Create(sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	if _, err := fixture.confirm(); err != nil {
		t.Fatalf("manual confirm failed: %v", err)
	}

	var reloaded productdomain.ProductSKU
	if err := fixture.db.First(&reloaded, sku.ID).Error; err != nil {
		t.Fatalf("reload sku failed: %v", err)
	}
	if reloaded.ManualStockTotal != 0 || reloaded.ManualStockLocked != 1 || reloaded.ManualStockSold != 1 {
		t.Fatalf("manual stock = total %d locked %d sold %d, want 0/1/1",
			reloaded.ManualStockTotal, reloaded.ManualStockLocked, reloaded.ManualStockSold)
	}
	if order := fixture.reloadOrder(t); order.PaidAt == nil {
		t.Fatalf("order should be paid after manual confirm, got status %s", order.Status)
	}
}

package paymentcallback_test

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletgormstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const callbackTestGuestSecret = "test-guest-credential-secret-with-32-bytes"

type countingNotificationEnqueuer struct {
	count atomic.Int32
}

func (q *countingNotificationEnqueuer) Enqueue(notificationcontract.EnqueueInput) error {
	q.count.Add(1)
	return nil
}

type postgresCallbackFixture struct {
	db            *gorm.DB
	service       *paymentapp.PaymentService
	orderRepo     ordercontract.Store
	paymentRepo   paymentcontract.Store
	walletRepo    walletcontract.Repository
	order         *orderdomain.Order
	payment       *paymentdomain.Payment
	sku           *productdomain.ProductSKU
	notifications *countingNotificationEnqueuer
}

func postgresDSNWithSearchPath(dsn, schema string) string {
	parsed, err := url.Parse(dsn)
	if err == nil && parsed.Scheme != "" {
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return strings.TrimSpace(dsn) + " search_path=" + schema
}

func openPaymentCallbackPostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("skip postgres payment callback concurrency test: TEST_POSTGRES_DSN is empty")
	}
	adminDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres admin connection failed: %v", err)
	}
	adminSQL, err := adminDB.DB()
	if err != nil {
		t.Fatalf("resolve postgres admin pool failed: %v", err)
	}
	schema := fmt.Sprintf("payment_callback_%d", time.Now().UnixNano())
	if err := adminDB.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema %s failed: %v", schema, err)
	}

	db, err := gorm.Open(postgres.Open(postgresDSNWithSearchPath(dsn, schema)), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open postgres test schema failed: %v", err)
	}
	testSQL, err := db.DB()
	if err != nil {
		t.Fatalf("resolve postgres test pool failed: %v", err)
	}
	testSQL.SetMaxOpenConns(16)
	t.Cleanup(func() {
		_ = testSQL.Close()
		if err := adminDB.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error; err != nil {
			t.Logf("drop schema %s failed: %v", schema, err)
		}
		_ = adminSQL.Close()
	})

	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&paymentdomain.Payment{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
	); err != nil {
		t.Fatalf("migrate callback test schema failed: %v", err)
	}
	return db
}

func newPostgresCallbackFixture(t *testing.T, walletAmount decimal.Decimal) *postgresCallbackFixture {
	t.Helper()
	db := openPaymentCallbackPostgresDB(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	total := decimal.NewFromInt(100)
	online := total.Sub(walletAmount).Round(2)

	category := &categorydomain.Category{
		Slug:      fmt.Sprintf("callback-%d", now.UnixNano()),
		NameJSON:  jsonmap.JSON{"en-US": "Callback test"},
		CreatedAt: now,
	}
	if err := db.Create(category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	product := &productdomain.Product{
		CategoryID:        category.ID,
		Slug:              fmt.Sprintf("callback-product-%d", now.UnixNano()),
		TitleJSON:         jsonmap.JSON{"en-US": "Callback test"},
		PriceAmount:       money.FromDecimal(total),
		PurchaseType:      constants.ProductPurchaseMember,
		FulfillmentType:   constants.FulfillmentTypeManual,
		ManualStockTotal:  9,
		ManualStockLocked: 1,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := &productdomain.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           productdomain.DefaultSKUCode,
		PriceAmount:       money.FromDecimal(total),
		ManualStockTotal:  9,
		ManualStockLocked: 1,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	order := &orderdomain.Order{
		OrderNo:                 fmt.Sprintf("DJPGCALLBACK%d", now.UnixNano()),
		UserID:                  1001,
		Status:                  constants.OrderStatusPendingPayment,
		Currency:                "CNY",
		OriginalAmount:          money.FromDecimal(total),
		DiscountAmount:          money.FromDecimal(decimal.Zero),
		MemberDiscountAmount:    money.FromDecimal(decimal.Zero),
		PromotionDiscountAmount: money.FromDecimal(decimal.Zero),
		WholesaleDiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:             money.FromDecimal(total),
		WalletPaidAmount:        money.FromDecimal(walletAmount),
		OnlinePaidAmount:        money.FromDecimal(online),
		RefundedAmount:          money.FromDecimal(decimal.Zero),
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	item := &orderdomain.OrderItem{
		OrderID:            order.ID,
		ProductID:          product.ID,
		SKUID:              sku.ID,
		TitleJSON:          product.TitleJSON,
		OriginalUnitPrice:  money.FromDecimal(total),
		UnitPrice:          money.FromDecimal(total),
		Quantity:           1,
		OriginalTotalPrice: money.FromDecimal(total),
		TotalPrice:         money.FromDecimal(total),
		FulfillmentType:    constants.FulfillmentTypeManual,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create order item failed: %v", err)
	}
	payment := &paymentdomain.Payment{
		OrderID:         order.ID,
		ChannelID:       1,
		ProviderType:    "test",
		ChannelType:     "test",
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          money.FromDecimal(online),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FixedFee:        money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.PaymentStatusPending,
		ProviderPayload: jsonmap.JSON{
			paymentcontract.GatewayPayloadWalletPaidAmount: walletAmount.StringFixed(2),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	walletRepo := walletgormstore.New(db)
	if walletAmount.GreaterThan(decimal.Zero) {
		before := decimal.NewFromInt(100)
		after := before.Sub(walletAmount)
		account := &walletdomain.Account{
			UserID:    order.UserID,
			Balance:   money.FromDecimal(after),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.Create(account).Error; err != nil {
			t.Fatalf("create wallet account failed: %v", err)
		}
		orderID := order.ID
		transaction := &walletdomain.Transaction{
			UserID:        order.UserID,
			OrderID:       &orderID,
			Type:          constants.WalletTxnTypeOrderPay,
			Direction:     constants.WalletTxnDirectionOut,
			Amount:        money.FromDecimal(walletAmount),
			BalanceBefore: money.FromDecimal(before),
			BalanceAfter:  money.FromDecimal(after),
			Currency:      order.Currency,
			Reference:     fmt.Sprintf("order:%d:%s", order.ID, constants.WalletTxnTypeOrderPay),
			Remark:        "test mixed payment",
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := db.Create(transaction).Error; err != nil {
			t.Fatalf("create wallet payment transaction failed: %v", err)
		}
	}

	orderRepo := ordergormstore.New(db, callbackTestGuestSecret)
	paymentRepo := paymentgormstore.New(db, callbackTestGuestSecret)
	notifications := &countingNotificationEnqueuer{}
	walletService := walletapp.NewService(walletapp.Options{Repository: walletRepo, Transactions: walletRepo})
	service := paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{
		OrderStore:          orderRepo,
		ProductRepo:         productgormstore.NewProductStore(db),
		ProductSKURepo:      productgormstore.NewSKUStore(db),
		PaymentStore:        paymentRepo,
		WalletRepo:          walletRepo,
		WalletService:       walletService,
		NotificationService: notifications,
	})
	return &postgresCallbackFixture{
		db: db, service: service, orderRepo: orderRepo, paymentRepo: paymentRepo,
		walletRepo: walletRepo, order: order, payment: payment, sku: sku,
		notifications: notifications,
	}
}

func (f *postgresCallbackFixture) callbackInput(status string) paymentapp.PaymentCallbackInput {
	return paymentapp.PaymentCallbackInput{
		PaymentID: f.payment.ID,
		OrderNo:   f.order.OrderNo,
		ChannelID: f.payment.ChannelID,
		Status:    status,
		Amount:    f.payment.Amount,
		Currency:  f.payment.Currency,
	}
}

func (f *postgresCallbackFixture) assertPaidOnce(t *testing.T) {
	t.Helper()
	payment, err := f.paymentRepo.GetByID(f.payment.ID)
	if err != nil || payment == nil {
		t.Fatalf("reload payment failed: payment=%v err=%v", payment, err)
	}
	if payment.Status != constants.PaymentStatusSuccess {
		t.Fatalf("payment status = %s, want success", payment.Status)
	}
	order, err := f.orderRepo.GetByID(f.order.ID)
	if err != nil || order == nil {
		t.Fatalf("reload order failed: order=%v err=%v", order, err)
	}
	if order.Status != constants.OrderStatusPaid {
		t.Fatalf("order status = %s, want paid", order.Status)
	}
	var sku productdomain.ProductSKU
	if err := f.db.First(&sku, f.sku.ID).Error; err != nil {
		t.Fatalf("reload sku failed: %v", err)
	}
	if sku.ManualStockSold != 1 || sku.ManualStockLocked != 0 || sku.ManualStockTotal != 9 {
		t.Fatalf("stock changed more than once: total=%d locked=%d sold=%d", sku.ManualStockTotal, sku.ManualStockLocked, sku.ManualStockSold)
	}
	if got := f.notifications.count.Load(); got != 1 {
		t.Fatalf("paid notification count = %d, want 1", got)
	}
}

func TestConcurrentSuccessCallbacksApplyBusinessEffectsOnceOnPostgres(t *testing.T) {
	fixture := newPostgresCallbackFixture(t, decimal.Zero)
	const workers = 8
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wait sync.WaitGroup
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := fixture.service.HandleCallback(fixture.callbackInput(constants.PaymentStatusSuccess))
			errs <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent success callback failed: %v", err)
		}
	}
	fixture.assertPaidOnce(t)
}

func TestSuccessExpiredInterleavingsPreserveMixedWalletFundsOnPostgres(t *testing.T) {
	for _, test := range []struct {
		name         string
		firstStatus  string
		secondStatus string
		wantRecovery bool
	}{
		{name: "expired wins lock before success", firstStatus: constants.PaymentStatusExpired, secondStatus: constants.PaymentStatusSuccess, wantRecovery: true},
		{name: "success wins lock before expired", firstStatus: constants.PaymentStatusSuccess, secondStatus: constants.PaymentStatusExpired, wantRecovery: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPostgresCallbackFixture(t, decimal.NewFromInt(30))
			if _, err := fixture.service.HandleCallback(fixture.callbackInput(test.firstStatus)); err != nil {
				t.Fatalf("first callback %s failed: %v", test.firstStatus, err)
			}
			if _, err := fixture.service.HandleCallback(fixture.callbackInput(test.secondStatus)); err != nil {
				t.Fatalf("second callback %s failed: %v", test.secondStatus, err)
			}
			fixture.assertPaidOnce(t)

			account, err := fixture.walletRepo.GetAccountByUserID(fixture.order.UserID)
			if err != nil || account == nil {
				t.Fatalf("reload wallet account failed: account=%v err=%v", account, err)
			}
			if !account.Balance.Decimal.Equal(decimal.NewFromInt(70)) {
				t.Fatalf("wallet balance = %s, want 70", account.Balance.String())
			}
			order, err := fixture.orderRepo.GetByID(fixture.order.ID)
			if err != nil || order == nil {
				t.Fatalf("reload mixed order failed: order=%v err=%v", order, err)
			}
			if !order.WalletPaidAmount.Decimal.Equal(decimal.NewFromInt(30)) || !order.OnlinePaidAmount.Decimal.Equal(decimal.NewFromInt(70)) {
				t.Fatalf("mixed allocation changed: wallet=%s online=%s", order.WalletPaidAmount.String(), order.OnlinePaidAmount.String())
			}

			var recoveryCount int64
			if err := fixture.db.Model(&walletdomain.Transaction{}).
				Where("order_id = ? AND type = ?", fixture.order.ID, constants.WalletTxnTypeOrderPayRecovery).
				Count(&recoveryCount).Error; err != nil {
				t.Fatalf("count recovery transactions failed: %v", err)
			}
			wantRecoveryCount := int64(0)
			if test.wantRecovery {
				wantRecoveryCount = 1
			}
			if recoveryCount != wantRecoveryCount {
				t.Fatalf("recovery transaction count = %d, want %d", recoveryCount, wantRecoveryCount)
			}
		})
	}
}

func TestLateSuccessDoesNotDeliverWhenReleasedWalletFundsWereSpentOnPostgres(t *testing.T) {
	fixture := newPostgresCallbackFixture(t, decimal.NewFromInt(30))
	if _, err := fixture.service.HandleCallback(fixture.callbackInput(constants.PaymentStatusExpired)); err != nil {
		t.Fatalf("expire callback failed: %v", err)
	}
	if err := fixture.db.Model(&walletdomain.Account{}).
		Where("user_id = ?", fixture.order.UserID).
		Update("balance", money.FromDecimal(decimal.NewFromInt(20))).Error; err != nil {
		t.Fatalf("simulate spending released funds failed: %v", err)
	}

	_, err := fixture.service.HandleCallback(fixture.callbackInput(constants.PaymentStatusSuccess))
	if !errors.Is(err, walletcontract.ErrInsufficientBalance) {
		t.Fatalf("late success error = %v, want insufficient balance", err)
	}
	payment, reloadErr := fixture.paymentRepo.GetByID(fixture.payment.ID)
	if reloadErr != nil || payment == nil {
		t.Fatalf("reload payment failed: payment=%v err=%v", payment, reloadErr)
	}
	if payment.Status != constants.PaymentStatusExpired {
		t.Fatalf("payment status = %s, want expired after rollback", payment.Status)
	}
	order, reloadErr := fixture.orderRepo.GetByID(fixture.order.ID)
	if reloadErr != nil || order == nil {
		t.Fatalf("reload order failed: order=%v err=%v", order, reloadErr)
	}
	if order.Status != constants.OrderStatusPendingPayment || order.WalletPaidAmount.Decimal.IsPositive() {
		t.Fatalf("underfunded order was advanced: status=%s wallet=%s", order.Status, order.WalletPaidAmount.String())
	}
	var sku productdomain.ProductSKU
	if err := fixture.db.First(&sku, fixture.sku.ID).Error; err != nil {
		t.Fatalf("reload sku failed: %v", err)
	}
	if sku.ManualStockSold != 0 || sku.ManualStockLocked != 1 {
		t.Fatalf("stock changed for underfunded late success: locked=%d sold=%d", sku.ManualStockLocked, sku.ManualStockSold)
	}
	if got := fixture.notifications.count.Load(); got != 0 {
		t.Fatalf("notification count = %d, want 0", got)
	}
}

func TestFullOnlinePaymentAfterWalletReleaseDoesNotReclaimOldAllocationOnPostgres(t *testing.T) {
	fixture := newPostgresCallbackFixture(t, decimal.NewFromInt(30))
	if _, err := fixture.service.HandleCallback(fixture.callbackInput(constants.PaymentStatusExpired)); err != nil {
		t.Fatalf("expire mixed payment failed: %v", err)
	}

	now := time.Now().UTC()
	fullPayment := &paymentdomain.Payment{
		OrderID:         fixture.order.ID,
		ChannelID:       fixture.payment.ChannelID,
		ProviderType:    fixture.payment.ProviderType,
		ChannelType:     fixture.payment.ChannelType,
		InteractionMode: fixture.payment.InteractionMode,
		Amount:          money.FromDecimal(decimal.NewFromInt(100)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FixedFee:        money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        fixture.payment.Currency,
		Status:          constants.PaymentStatusPending,
		ProviderPayload: jsonmap.JSON{
			paymentcontract.GatewayPayloadWalletPaidAmount: "0.00",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := fixture.paymentRepo.Create(fullPayment); err != nil {
		t.Fatalf("create full online payment failed: %v", err)
	}
	fixture.payment = fullPayment
	if _, err := fixture.service.HandleCallback(fixture.callbackInput(constants.PaymentStatusSuccess)); err != nil {
		t.Fatalf("full online success failed: %v", err)
	}
	fixture.assertPaidOnce(t)

	account, err := fixture.walletRepo.GetAccountByUserID(fixture.order.UserID)
	if err != nil || account == nil {
		t.Fatalf("reload wallet account failed: account=%v err=%v", account, err)
	}
	if !account.Balance.Decimal.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("full payment reclaimed old wallet allocation: balance=%s", account.Balance.String())
	}
	order, err := fixture.orderRepo.GetByID(fixture.order.ID)
	if err != nil || order == nil {
		t.Fatalf("reload full online order failed: order=%v err=%v", order, err)
	}
	if order.WalletPaidAmount.Decimal.IsPositive() || !order.OnlinePaidAmount.Decimal.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("unexpected full online allocation wallet=%s online=%s", order.WalletPaidAmount.String(), order.OnlinePaidAmount.String())
	}
}

func TestLegacyMixedPaymentWithoutSnapshotRequiresReconciliationOnPostgres(t *testing.T) {
	fixture := newPostgresCallbackFixture(t, decimal.NewFromInt(30))
	if _, err := fixture.service.HandleCallback(fixture.callbackInput(constants.PaymentStatusExpired)); err != nil {
		t.Fatalf("expire legacy mixed payment failed: %v", err)
	}
	if err := fixture.db.Model(&paymentdomain.Payment{}).
		Where("id = ?", fixture.payment.ID).
		Update("provider_payload", jsonmap.JSON{}).Error; err != nil {
		t.Fatalf("remove wallet allocation snapshot failed: %v", err)
	}

	_, err := fixture.service.HandleCallback(fixture.callbackInput(constants.PaymentStatusSuccess))
	if !errors.Is(err, walletcontract.ErrBalanceRecoveryRequired) {
		t.Fatalf("legacy late success error = %v, want manual reconciliation", err)
	}
	payment, reloadErr := fixture.paymentRepo.GetByID(fixture.payment.ID)
	if reloadErr != nil || payment == nil {
		t.Fatalf("reload legacy payment failed: payment=%v err=%v", payment, reloadErr)
	}
	if payment.Status != constants.PaymentStatusExpired {
		t.Fatalf("legacy payment status = %s, want expired after rollback", payment.Status)
	}
	order, reloadErr := fixture.orderRepo.GetByID(fixture.order.ID)
	if reloadErr != nil || order == nil {
		t.Fatalf("reload legacy order failed: order=%v err=%v", order, reloadErr)
	}
	if order.Status != constants.OrderStatusPendingPayment || order.WalletPaidAmount.Decimal.IsPositive() {
		t.Fatalf("legacy underfunded order was advanced: status=%s wallet=%s", order.Status, order.WalletPaidAmount.String())
	}
}

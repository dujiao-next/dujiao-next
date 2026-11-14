package consumer

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/mailbrand"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"
)

// orderStatusEmailSenderStub 捕获订单及充值退款邮件，避免测试连接真实 SMTP 服务。
type orderStatusEmailSenderStub struct {
	calls      int
	to, locale string
	input      notificationcontract.OrderStatusEmailInput
	template   *settingsmessaging.OrderEmailTemplateSetting
	err        error
}

// SendOrderStatusEmailWithTemplate 记录本次状态通知的模板及变量。
func (s *orderStatusEmailSenderStub) SendOrderStatusEmailWithTemplate(to string, input notificationcontract.OrderStatusEmailInput, locale string, template *settingsmessaging.OrderEmailTemplateSetting) error {
	s.calls++
	s.to = to
	s.input = input
	s.locale = locale
	s.template = template
	return s.err
}

func TestWalletRechargeRefundEmailUsesOrderTemplatesAndPerRefundFacts(t *testing.T) {
	transient := errors.New("temporary SMTP error")
	for _, tc := range []struct {
		name, email, status string
		refundID            uint
		enabled, notify     bool
		calls               int
		sendErr             error
		wantErr             bool
	}{
		{name: "partial", email: "buyer@example.com", status: "partially_refunded", refundID: 1, enabled: true, notify: true, calls: 1},
		{name: "full", email: "buyer@example.com", status: "refunded", refundID: 2, enabled: true, notify: true, calls: 1},
		{name: "smtp_disabled", email: "buyer@example.com", status: "refunded", refundID: 2, notify: true},
		{name: "notifications_disabled", email: "buyer@example.com", status: "refunded", refundID: 2, enabled: true},
		{name: "placeholder", email: "telegram_123@login.local", status: "refunded", refundID: 2, enabled: true, notify: true},
		{name: "empty_receiver", status: "refunded", refundID: 2, enabled: true, notify: true},
		{name: "missing_refund", email: "buyer@example.com", status: "refunded", refundID: 99, enabled: true, notify: true},
		{name: "wrong_status", email: "buyer@example.com", status: "success", refundID: 2, enabled: true, notify: true},
		{name: "retry_smtp_error", email: "buyer@example.com", status: "refunded", refundID: 2, enabled: true, notify: true, calls: 1, sendErr: transient, wantErr: true},
		{name: "invalid_email_no_retry", email: "buyer@example.com", status: "refunded", refundID: 2, enabled: true, notify: true, calls: 1, sendErr: notificationcontract.ErrInvalidEmail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:refund_email_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&userdomain.User{}, &walletdomain.RechargeOrder{}, &walletdomain.Transaction{}, &settingsstore.SettingRecord{}); err != nil {
				t.Fatal(err)
			}
			raw, _ := db.DB()
			raw.SetMaxOpenConns(1)
			t.Cleanup(func() { _ = raw.Close() })
			user := userdomain.User{ID: 1, Email: tc.email, Locale: "zh-TW", Status: constants.UserStatusActive}
			if err := db.Create(&user).Error; err != nil {
				t.Fatal(err)
			}
			recharge := walletdomain.RechargeOrder{ID: 1, RechargeNo: "WR-MAIL-001", UserID: 1, PaymentID: 1, Amount: money.FromDecimal(decimal.NewFromInt(100)), Status: constants.WalletRechargeStatusRefunded, Currency: "CNY"}
			if err := db.Create(&recharge).Error; err != nil {
				t.Fatal(err)
			}
			for i, amount := range []int64{20, 80} {
				record := walletdomain.Transaction{ID: uint(i + 1), UserID: 1, RechargeOrderID: &recharge.ID, Type: constants.WalletTxnTypeAdminRefund, Direction: constants.WalletTxnDirectionOut, Amount: money.FromDecimal(decimal.NewFromInt(amount)), Reference: fmt.Sprintf("refund-%d", i), Remark: fmt.Sprintf("reason-%d", i)}
				if err := db.Create(&record).Error; err != nil {
					t.Fatal(err)
				}
			}
			settings := settingsstore.New(db)
			if _, err := settings.Upsert(constants.SettingKeySMTPConfig, jsonmap.JSON{"enabled": tc.enabled, "order_notification_enabled": tc.notify}); err != nil {
				t.Fatal(err)
			}
			sender := &orderStatusEmailSenderStub{err: tc.sendErr}
			worker := &Consumer{Container: &container.Container{
				Config:         &config.Config{Email: config.EmailConfig{Enabled: true}},
				SettingService: settingsapp.NewService(settings), WalletRepo: walletstore.New(db), UserStore: userstore.New(db),
			}, emailSender: sender}
			task, err := queue.NewWalletRechargeRefundEmailTask(queue.WalletRechargeRefundEmailPayload{RechargeID: 1, RefundID: tc.refundID, Status: tc.status})
			if err != nil {
				t.Fatal(err)
			}
			err = worker.handleWalletRechargeRefundEmail(context.Background(), task)
			if (err != nil) != tc.wantErr || sender.calls != tc.calls {
				t.Fatalf("error=%v calls=%d", err, sender.calls)
			}
			if sender.calls > 0 {
				want := "20.00"
				reason := "reason-0"
				if tc.refundID == 2 {
					want = "80.00"
					reason = "reason-1"
				}
				if sender.to != tc.email || sender.locale != "zh-TW" || sender.input.OrderNo != recharge.RechargeNo || sender.input.Status != tc.status || sender.input.RefundAmount.String() != want || sender.input.RefundReason != reason || sender.template == nil {
					t.Fatalf("email: %+v", sender)
				}
			}
		})
	}
}

func TestBuildBotNotifyRequestURLReplacesPath(t *testing.T) {
	got, err := buildBotNotifyRequestURL("https://bot.example.com/internal/order-fulfilled", "/internal/wallet-recharge-succeeded")
	if err != nil {
		t.Fatalf("build bot notify request url failed: %v", err)
	}
	want := "https://bot.example.com/internal/wallet-recharge-succeeded"
	if got != want {
		t.Fatalf("request url want %s got %s", want, got)
	}
}

func TestBuildBotNotifyRequestURLReplacesOrderPaidPath(t *testing.T) {
	got, err := buildBotNotifyRequestURL("https://bot.example.com/internal/order-fulfilled", "/internal/order-paid")
	if err != nil {
		t.Fatalf("build bot notify request url failed: %v", err)
	}
	want := "https://bot.example.com/internal/order-paid"
	if got != want {
		t.Fatalf("request url want %s got %s", want, got)
	}
}

func TestBuildOrderFulfillmentEmailPayloadNilOrder(t *testing.T) {
	if got := buildOrderFulfillmentEmailPayload(nil); got != "" {
		t.Fatalf("expected empty payload for nil order, got %q", got)
	}
}

func TestBuildOrderInstructionsEmailText(t *testing.T) {
	t.Run("nil order returns empty", func(t *testing.T) {
		if got := buildOrderInstructionsEmailText(nil, "zh-CN"); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("locale preferred over fallback", func(t *testing.T) {
		order := &orderdomain.Order{
			Items: []orderdomain.OrderItem{
				{InstructionsJSON: jsonmap.JSON{
					"zh-CN": "<p>中文说明</p>",
					"en-US": "<p>English</p>",
				}},
			},
		}
		if got := buildOrderInstructionsEmailText(order, "en-US"); got != "English" {
			t.Fatalf("want 'English', got %q", got)
		}
	})

	t.Run("falls back to zh-CN when locale missing", func(t *testing.T) {
		order := &orderdomain.Order{
			Items: []orderdomain.OrderItem{
				{InstructionsJSON: jsonmap.JSON{"zh-CN": "fallback"}},
			},
		}
		if got := buildOrderInstructionsEmailText(order, "ja-JP"); got != "fallback" {
			t.Fatalf("want 'fallback', got %q", got)
		}
	})

	t.Run("dedupes identical items and joins distinct", func(t *testing.T) {
		order := &orderdomain.Order{
			Items: []orderdomain.OrderItem{
				{InstructionsJSON: jsonmap.JSON{"zh-CN": "<p>A</p>"}},
				{InstructionsJSON: jsonmap.JSON{"zh-CN": "<p>A</p>"}}, // 重复，应去重
				{InstructionsJSON: jsonmap.JSON{"zh-CN": "<p>B</p>"}},
			},
		}
		got := buildOrderInstructionsEmailText(order, "zh-CN")
		if got != "A\n\nB" {
			t.Fatalf("want 'A\\n\\nB', got %q", got)
		}
	})

	t.Run("collects from children items", func(t *testing.T) {
		order := &orderdomain.Order{
			Children: []orderdomain.Order{
				{Items: []orderdomain.OrderItem{{InstructionsJSON: jsonmap.JSON{"zh-CN": "child1"}}}},
				{Items: []orderdomain.OrderItem{{InstructionsJSON: jsonmap.JSON{"zh-CN": "child2"}}}},
			},
		}
		got := buildOrderInstructionsEmailText(order, "zh-CN")
		if got != "child1\n\nchild2" {
			t.Fatalf("unexpected: %q", got)
		}
	})

	t.Run("empty instructions yield empty result", func(t *testing.T) {
		order := &orderdomain.Order{
			Items: []orderdomain.OrderItem{{InstructionsJSON: nil}},
		}
		if got := buildOrderInstructionsEmailText(order, "zh-CN"); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("strips HTML from instructions", func(t *testing.T) {
		order := &orderdomain.Order{
			Items: []orderdomain.OrderItem{
				{InstructionsJSON: jsonmap.JSON{"zh-CN": "<p>步骤一</p><ul><li>登录</li><li>激活</li></ul>"}},
			},
		}
		got := buildOrderInstructionsEmailText(order, "zh-CN")
		want := "步骤一\n\n• 登录\n• 激活"
		if got != want {
			t.Fatalf("want %q, got %q", want, got)
		}
	})
}

func TestBuildOrderFulfillmentEmailPayloadPreferOrderFulfillment(t *testing.T) {
	order := &orderdomain.Order{
		Fulfillment: &fulfillmentdomain.Fulfillment{Payload: "  MAIN-LINE-1\nMAIN-LINE-2  "},
		Children: []orderdomain.Order{
			{
				OrderNo:     "CHILD-1",
				Fulfillment: &fulfillmentdomain.Fulfillment{Payload: "SECRET-1"},
			},
		},
	}

	got := buildOrderFulfillmentEmailPayload(order)
	want := "MAIN-LINE-1\nMAIN-LINE-2"
	if got != want {
		t.Fatalf("unexpected payload, want %q, got %q", want, got)
	}
}

type orderStatusEmailWorkerOrderRepoStub struct {
	order *orderdomain.Order
	err   error
}

func (s orderStatusEmailWorkerOrderRepoStub) GetByID(_ uint) (*orderdomain.Order, error) {
	return s.order, s.err
}

type orderEmailBrandResolverStub struct {
	brand mailbrand.Brand
	scope mailbrand.Scope
}

func (s *orderEmailBrandResolverStub) ResolveEmailBrand(_ context.Context, scope mailbrand.Scope) (mailbrand.Brand, error) {
	s.scope = scope
	return s.brand, nil
}

func TestResolveOrderEmailBrandUsesResellerScope(t *testing.T) {
	resellerID := uint(17)
	resolver := &orderEmailBrandResolverStub{brand: mailbrand.Brand{
		SiteName: "White Label Store",
		SiteURL:  "https://shop.example.test",
		FromName: "White Label Store",
		ReplyTo:  "support@example.test",
	}}
	consumer := &Consumer{
		Container: &container.Container{EmailBrandResolver: resolver},
	}
	order := &orderdomain.Order{
		ResellerID:     &resellerID,
		ResellerDomain: "shop.example.test",
	}

	got, err := consumer.resolveOrderEmailBrand(context.Background(), order)
	if err != nil {
		t.Fatalf("resolve order email brand failed: %v", err)
	}
	if resolver.scope.ResellerID == nil || *resolver.scope.ResellerID != resellerID || resolver.scope.Host != "shop.example.test" {
		t.Fatalf("unexpected resolver scope: %+v", resolver.scope)
	}
	if got.SiteName != "White Label Store" || got.SiteURL != "https://shop.example.test" || got.FromName != "White Label Store" {
		t.Fatalf("unexpected reseller brand: %+v", got)
	}
}

func TestResolveOrderEmailBrandNeverFallsBackToMainBrandForReseller(t *testing.T) {
	resellerID := uint(23)
	consumer := &Consumer{
		Container: &container.Container{},
	}

	got, err := consumer.resolveOrderEmailBrand(context.Background(), &orderdomain.Order{
		ResellerID:     &resellerID,
		ResellerDomain: "fallback.example.test",
	})
	if err != nil {
		t.Fatalf("resolve safe fallback failed: %v", err)
	}
	if got.SiteName != "fallback.example.test" || got.SiteURL != "https://fallback.example.test" || got.FromName != "fallback.example.test" {
		t.Fatalf("unexpected safe reseller fallback: %+v", got)
	}
}

func TestHandleOrderStatusEmailSkipsNonRetryableEmailErrors(t *testing.T) {
	retryable := errors.New("temporary SMTP failure")
	for _, tc := range []struct {
		name    string
		email   string
		sendErr error
		wantErr bool
	}{
		{name: "smtp_disabled", email: "buyer@example.com", sendErr: notificationcontract.ErrEmailServiceDisabled},
		{name: "smtp_not_configured", email: "buyer@example.com", sendErr: notificationcontract.ErrEmailNotConfigured},
		{name: "invalid_receiver_email", email: "invalid-email", sendErr: notificationcontract.ErrInvalidEmail},
		{name: "generic_send_failure_keeps_retryable_error", email: "buyer@example.com", sendErr: retryable, wantErr: true},
		{name: "successful_send", email: "buyer@example.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			order := &orderdomain.Order{ID: 1, OrderNo: "DJ-ORDER-001", GuestEmail: tc.email, GuestLocale: "zh-CN", Currency: "CNY"}
			task, err := queue.NewOrderStatusEmailTask(queue.OrderStatusEmailPayload{OrderID: order.ID, Status: "paid"})
			if err != nil {
				t.Fatal(err)
			}
			sender := &orderStatusEmailSenderStub{err: tc.sendErr}
			worker := &Consumer{Container: &container.Container{}, orderReader: orderStatusEmailWorkerOrderRepoStub{order: order}, emailSender: sender}
			err = worker.handleOrderStatusEmail(context.Background(), task)
			if tc.wantErr {
				if !errors.Is(err, tc.sendErr) {
					t.Fatalf("expected retryable error %v, got %v", tc.sendErr, err)
				}
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if sender.calls != 1 || sender.to != tc.email || sender.input.OrderNo != order.OrderNo || sender.input.Status != "paid" {
				t.Fatalf("unexpected send: %+v", sender)
			}
		})
	}
}

func TestHandleOrderStatusEmailSkipsCanceledWithoutDependencies(t *testing.T) {
	task, err := queue.NewOrderStatusEmailTask(queue.OrderStatusEmailPayload{
		OrderID: 106,
		Status:  constants.OrderStatusCanceled,
	})
	if err != nil {
		t.Fatalf("new order status email task failed: %v", err)
	}

	consumer := &Consumer{}
	if err := consumer.handleOrderStatusEmail(context.Background(), task); err != nil {
		t.Fatalf("expected canceled email task to be dropped, got %v", err)
	}
}

func TestHandleOrderStatusEmailSkipsCanceledRegisteredOrderWhenPayloadStatusEmpty(t *testing.T) {
	task, err := queue.NewOrderStatusEmailTask(queue.OrderStatusEmailPayload{OrderID: 107})
	if err != nil {
		t.Fatalf("new order status email task failed: %v", err)
	}

	consumer := &Consumer{
		Container: &container.Container{},
		orderReader: orderStatusEmailWorkerOrderRepoStub{order: &orderdomain.Order{
			ID:      107,
			OrderNo: "DJ-ORDER-107",
			UserID:  42,
			Status:  constants.OrderStatusCanceled,
		}},
	}
	if err := consumer.handleOrderStatusEmail(context.Background(), task); err != nil {
		t.Fatalf("expected canceled registered-order task to be dropped, got %v", err)
	}
}

func TestBuildOrderFulfillmentEmailPayloadFromChildren(t *testing.T) {
	order := &orderdomain.Order{
		Children: []orderdomain.Order{
			{
				OrderNo:     "DJ-CHILD-01",
				Fulfillment: &fulfillmentdomain.Fulfillment{Payload: "  SECRET-01  "},
			},
			{
				OrderNo:     "DJ-CHILD-02",
				Fulfillment: nil,
			},
			{
				OrderNo:     "DJ-CHILD-03",
				Fulfillment: &fulfillmentdomain.Fulfillment{Payload: "    "},
			},
			{
				OrderNo:     "DJ-CHILD-04",
				Fulfillment: &fulfillmentdomain.Fulfillment{Payload: "SECRET-04-L1\nSECRET-04-L2"},
			},
		},
	}

	got := buildOrderFulfillmentEmailPayload(order)
	want := "[DJ-CHILD-01]\nSECRET-01\n\n[DJ-CHILD-04]\nSECRET-04-L1\nSECRET-04-L2"
	if got != want {
		t.Fatalf("unexpected payload, want %q, got %q", want, got)
	}
}

package consumer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	channelclientapp "github.com/dujiao-next/internal/modules/channelclient/application"
	channelclientcontract "github.com/dujiao-next/internal/modules/channelclient/contract"
	channelclientdomain "github.com/dujiao-next/internal/modules/channelclient/domain"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingscontract "github.com/dujiao-next/internal/modules/settings/contract"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	notificationsmtp "github.com/dujiao-next/internal/modules/notification/infrastructure/smtp"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/mailbrand"
)

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
	testCases := []struct {
		name         string
		order        *orderdomain.Order
		emailConfig  config.EmailConfig
		expectNilErr bool
	}{
		{
			name: "smtp_disabled",
			order: &orderdomain.Order{
				ID:          1,
				OrderNo:     "DJ-ORDER-001",
				GuestEmail:  "buyer@example.com",
				GuestLocale: "zh-CN",
				Currency:    "CNY",
			},
			emailConfig:  config.EmailConfig{Enabled: false},
			expectNilErr: true,
		},
		{
			name: "smtp_not_configured",
			order: &orderdomain.Order{
				ID:          2,
				OrderNo:     "DJ-ORDER-002",
				GuestEmail:  "buyer@example.com",
				GuestLocale: "zh-CN",
				Currency:    "CNY",
			},
			emailConfig:  config.EmailConfig{Enabled: true},
			expectNilErr: true,
		},
		{
			name: "invalid_receiver_email",
			order: &orderdomain.Order{
				ID:          3,
				OrderNo:     "DJ-ORDER-003",
				GuestEmail:  "invalid-email",
				GuestLocale: "zh-CN",
				Currency:    "CNY",
			},
			emailConfig: config.EmailConfig{
				Enabled: true,
				Host:    "127.0.0.1",
				Port:    1,
				From:    "sender@example.com",
			},
			expectNilErr: true,
		},
		{
			name: "generic_send_failure_keeps_retryable_error",
			order: &orderdomain.Order{
				ID:          4,
				OrderNo:     "DJ-ORDER-004",
				GuestEmail:  "buyer@example.com",
				GuestLocale: "zh-CN",
				Currency:    "CNY",
			},
			emailConfig: config.EmailConfig{
				Enabled: true,
				Host:    "127.0.0.1",
				Port:    1,
				From:    "sender@example.com",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			task, err := queue.NewOrderStatusEmailTask(queue.OrderStatusEmailPayload{
				OrderID: tc.order.ID,
				Status:  "paid",
			})
			if err != nil {
				t.Fatalf("new order status email task failed: %v", err)
			}

			consumer := &Consumer{
				Container: &container.Container{
					EmailSender: notificationsmtp.New(&tc.emailConfig),
				},
				orderReader: orderStatusEmailWorkerOrderRepoStub{order: tc.order},
			}

			err = consumer.handleOrderStatusEmail(context.Background(), task)
			if tc.expectNilErr {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected retryable send error, got nil")
			}
			if errors.Is(err, notificationcontract.ErrEmailServiceDisabled) || errors.Is(err, notificationcontract.ErrEmailNotConfigured) || errors.Is(err, notificationcontract.ErrInvalidEmail) {
				t.Fatalf("expected generic retryable error, got %v", err)
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

type paymentWorkerOwner struct{ paymentcontract.Store }

func (paymentWorkerOwner) UsesFixedPaymentEffects(uint) (bool, error) { return true, nil }

type paymentWorkerSettings struct {
	settingscontract.Store
	err error
}

func (s paymentWorkerSettings) GetByKey(string) (jsonmap.JSON, bool, error) { return nil, false, s.err }

type paymentWorkerChannel struct {
	channelclientcontract.Store
	client *channelclientdomain.Client
}

func (s paymentWorkerChannel) FindActiveByChannelType(string) (*channelclientdomain.Client, error) {
	return s.client, nil
}

func TestDurableBotDeliveryWaitsForHTTPAndRespectsAdmission(t *testing.T) {
	var calls atomic.Int32
	var status atomic.Int32
	status.Store(503)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/internal/order-paid" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(int(status.Load()))
	}))
	defer srv.Close()
	c := &Consumer{Container: &container.Container{
		PaymentStore:         paymentWorkerOwner{},
		ChannelClientService: channelclientapp.NewService(paymentWorkerChannel{client: &channelclientdomain.Client{CallbackURL: srv.URL, ChannelKey: "synthetic"}}, "synthetic"),
	}}
	input := paymentcontract.BotNotification{EventType: paymentcontract.BotNotificationOrderPaid, OrderID: 1, TelegramUserID: "synthetic"}
	permit := func(context.Context) error { return nil }
	for _, code := range []int{503, 429, 408, 403, 200} {
		status.Store(int32(code))
		err := c.SendPaymentBot(context.Background(), input, permit)
		if code == 200 && err != nil {
			t.Fatal(err)
		}
		if code != 200 && err == nil {
			t.Fatalf("status %d acknowledged", code)
		}
		if errors.Is(err, paymentcontract.ErrEffectPermanent) != (code == 403) {
			t.Fatalf("status %d classification: %v", code, err)
		}
	}
	frozen := errors.New("synthetic frozen")
	if err := c.SendPaymentBot(context.Background(), input, func(context.Context) error { return frozen }); !errors.Is(err, frozen) {
		t.Fatal(err)
	}
	task, err := queue.NewBotNotifyTask(queue.BotNotifyPayload{EventType: queue.BotNotifyEventOrderPaid, OrderID: 1, TelegramUserID: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.handleBotNotify(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 5 {
		t.Fatalf("frozen or legacy task sent: %d", calls.Load())
	}
}

// A local SMTP receiver verifies DATA rejection is retained, then accepted on
// retry. It never relays messages and only contains synthetic addresses.
func TestDurableOrderEmailDeliveryAndLegacyRefundScope(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var messages atomic.Int32
	var accept atomic.Bool
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				_, _ = fmt.Fprint(conn, "220 synthetic\r\n")
				reader := bufio.NewReader(conn)
				data := false
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimSpace(line)
					if data {
						if line != "." {
							continue
						}
						data = false
						messages.Add(1)
						if accept.Load() {
							_, _ = fmt.Fprint(conn, "250 accepted\r\n")
						} else {
							_, _ = fmt.Fprint(conn, "451 try again\r\n")
						}
						continue
					}
					switch {
					case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"), strings.HasPrefix(line, "MAIL"), strings.HasPrefix(line, "RCPT"):
						_, _ = fmt.Fprint(conn, "250 OK\r\n")
					case line == "DATA":
						data = true
						_, _ = fmt.Fprint(conn, "354 send data\r\n")
					case line == "QUIT":
						_, _ = fmt.Fprint(conn, "221 bye\r\n")
						return
					default:
						_, _ = fmt.Fprint(conn, "500 unexpected\r\n")
					}
				}
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); <-stopped })
	cfg := &config.Config{Email: config.EmailConfig{Enabled: true, Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, From: "sender@example.test"}}
	order := &orderdomain.Order{ID: 1, OrderNo: "synthetic", GuestEmail: "receiver@example.test", Status: constants.OrderStatusPaid, Currency: "CNY"}
	c := &Consumer{Container: &container.Container{Config: cfg, PaymentStore: paymentWorkerOwner{}, SettingService: settingsapp.NewService(paymentWorkerSettings{}), EmailSender: notificationsmtp.New(&cfg.Email)}, orderReader: orderStatusEmailWorkerOrderRepoStub{order: order}}
	permit := func(context.Context) error { return nil }
	if err := c.SendPaymentStatusEmail(context.Background(), 1, constants.OrderStatusPaid, permit); err == nil {
		t.Fatal("SMTP rejection acknowledged")
	}
	accept.Store(true)
	if err := c.SendPaymentStatusEmail(context.Background(), 1, constants.OrderStatusPaid, permit); err != nil {
		t.Fatal(err)
	}
	if messages.Load() != 2 {
		t.Fatal("SMTP retry not sent")
	}
	frozen := errors.New("synthetic frozen")
	if err := c.SendPaymentStatusEmail(context.Background(), 1, constants.OrderStatusPaid, func(context.Context) error { return frozen }); !errors.Is(err, frozen) {
		t.Fatal(err)
	}
	for _, status := range []string{constants.OrderStatusPaid, "", constants.OrderStatusCompleted, constants.OrderStatusRefunded} {
		task, err := queue.NewOrderStatusEmailTask(queue.OrderStatusEmailPayload{OrderID: 1, Status: status})
		if err != nil {
			t.Fatal(err)
		}
		if err := c.handleOrderStatusEmail(context.Background(), task); err != nil {
			t.Fatal(err)
		}
	}
	if messages.Load() != 3 {
		t.Fatalf("legacy payment bypass or lost refund notification: %d", messages.Load())
	}
	c.SettingService = settingsapp.NewService(paymentWorkerSettings{err: errors.New("synthetic DB failure")})
	if err := c.SendPaymentStatusEmail(context.Background(), 1, constants.OrderStatusPaid, permit); err == nil {
		t.Fatal("settings failure acknowledged")
	}
	c.SettingService = settingsapp.NewService(paymentWorkerSettings{})
	c.EmailSender = notificationsmtp.New(&config.EmailConfig{Enabled: true})
	if err := c.SendPaymentStatusEmail(context.Background(), 1, constants.OrderStatusPaid, permit); !errors.Is(err, notificationcontract.ErrEmailNotConfigured) {
		t.Fatalf("missing config: %v", err)
	}
	c.EmailSender = notificationsmtp.New(&cfg.Email)
	order.GuestEmail = "invalid-address"
	if err := c.SendPaymentStatusEmail(context.Background(), 1, constants.OrderStatusPaid, permit); !errors.Is(err, paymentcontract.ErrEffectPermanent) {
		t.Fatalf("invalid recipient: %v", err)
	}
}

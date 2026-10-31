package cashier_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	od "github.com/dujiao-next/internal/modules/order/domain"
	"github.com/dujiao-next/internal/modules/payment/application/cashier"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	adapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/epusdt"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/epusdt"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/provider"
	store "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func fixture(t *testing.T, network string) (*cashier.Service, *gorm.DB, pd.Payment, od.Order) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cashier.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	must(t, err)
	conn, err := db.DB()
	must(t, err)
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	must(t, db.AutoMigrate(&pd.AutomationControl{}, &od.Order{}, &od.OrderItem{}, &pd.Payment{}, &pd.Allocation{}))
	must(t, automation.Ensure(db))
	must(t, automation.SetFrozen(context.Background(), db, false, 1, "synthetic acceptance"))
	created, expires := time.Now().Add(-time.Minute), time.Now().Add(5*time.Minute)
	amount := money.FromDecimal(decimal.NewFromInt(100))
	o := od.Order{ID: 1, UserID: 1, OrderNo: "synthetic-order", Currency: "CNY", TotalAmount: amount, OnlinePaidAmount: amount, Status: constants.OrderStatusPendingPayment, ExpiresAt: &expires}
	must(t, db.Create(&o).Error)
	item := od.OrderItem{ID: 1, OrderID: 1, ProductID: 1, SKUID: 1, Quantity: 1, TitleJSON: jsonmap.JSON{"en": "synthetic"}, FulfillmentType: constants.FulfillmentTypeAuto}
	must(t, db.Create(&item).Error)
	o.Items = []od.OrderItem{item}
	p := pd.Payment{ID: 1, OrderID: 1, ChannelID: 1, ProviderType: constants.PaymentProviderEpusdt, ChannelType: "epusdt", Currency: "CNY", Amount: amount,
		FeePolicy: constants.PaymentFeePolicyNone, Status: constants.PaymentStatusPending, RequiresChainVerification: true, GatewayOrderNo: "synthetic-payment", ExpiredAt: &expires}
	recipient, contract, raw, decimals := "0x"+strings.Repeat("1", 40), "0x55d398326f99059ff775485246999027b3197955", "14000100000000000000", 18
	if network == "tron" {
		recipient, contract, raw, decimals = "TBXSw8fM4jpQkGc6zZjsVABFpVN7UvXPdV", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", "14000100", 6
	}
	cfg := &epusdt.Config{FixedPaymentIntent: true, GatewayURL: "https://gateway.example", PID: "1000", SecretKey: "synthetic", OrderMode: "transaction", Token: "usdt", Network: network, Currency: "cny", NotifyURL: "https://shop.example/notify", ReturnURL: "https://shop.example/return", QuotePolicy: &epusdt.QuotePolicy{Version: "test", Rate: "0.14", Precision: 4, MaxTail: "0.0099", LifetimeSeconds: 900, Recipient: recipient}}
	p.PaymentIntent, err = epusdt.PrepareIntent(cfg, epusdt.CreateInput{OrderNo: p.GatewayOrderNo, Amount: "100", Name: "synthetic"}, p.ID, o.ID, "CNY", &expires, nil, created)
	must(t, err)
	p.SettlementBasis = pd.OrderBasis(p, o)
	must(t, db.Create(&p).Error)
	p.ProviderRef = "synthetic-trade"
	p.PaymentIntent["terms"] = map[string]interface{}{"version": 1, "trade_id": p.ProviderRef, "order_id": p.GatewayOrderNo, "api_key_id": 7, "network": network, "token": "USDT", "contract": contract, "decimals": decimals, "recipient": recipient, "raw_amount": raw, "fiat_amount": "100", "request_amount": "100", "currency": "CNY", "rate": "0.14", "created_at_ms": created.Add(time.Second).UnixMilli(), "expires_at_ms": expires.UnixMilli()}
	expected, err := epusdt.ReadChainExpectation(p)
	must(t, err)
	repo := store.New(db, "synthetic-guest-secret")
	must(t, repo.ReserveAllocation(pd.Allocation{PaymentID: 1, OrderID: 1, SlotKey: expected.SlotKey(), Network: network, Contract: contract, Recipient: recipient, RawAmount: raw, Snapshot: pd.SettlementSnapshot(p, o), CreatedAt: created.Add(2 * time.Second)}))
	// Malicious gateway presentation fields must never influence the cashier.
	p.PayURL = "https://attacker.example/pay"
	p.QRCode = "https://attacker.example/qr.png"
	p.ProviderPayload = jsonmap.JSON{"data": map[string]interface{}{"receive_address": "attacker", "actual_amount": "0.01", "api_key": "never-return"}}
	must(t, db.Save(&p).Error)
	reg := provider.NewRegistry()
	reg.Register(constants.PaymentProviderEpusdt, "", adapter.NewEpusdtAdapter())
	return &cashier.Service{Store: repo, Registry: reg, Enabled: true}, db, p, o
}

func TestCashierUsesOnlyValidatedMerchantInstructions(t *testing.T) {
	for _, network := range []string{"tron", "binance"} {
		t.Run(network, func(t *testing.T) {
			svc, db, p, _ := fixture(t, network)
			view, err := svc.Read(context.Background(), 1, 1, 1)
			must(t, err)
			if view.State != "ready" || view.Details == nil || view.Details.Amount != "14.0001" || view.Details.Network != network || view.DisplayUntilMs-view.ServerTimeMs > 15000 {
				t.Fatalf("unexpected view: %+v", view)
			}
			encoded, _ := json.Marshal(view)
			for _, forbidden := range []string{"attacker", "never-return", "synthetic-order", "synthetic-trade", "payment_intent", "user_id"} {
				if strings.Contains(string(encoded), forbidden) {
					t.Fatalf("private/untrusted data exposed: %s", forbidden)
				}
			}
			must(t, automation.SetFrozen(context.Background(), db, true, 2, "synthetic freeze"))
			view, err = svc.Read(context.Background(), 1, 1, 1)
			must(t, err)
			if view.State != "paused" || view.Details != nil {
				t.Fatal("frozen instructions exposed")
			}
			must(t, automation.SetFrozen(context.Background(), db, false, 3, "synthetic resume"))
			view, err = svc.Read(context.Background(), 1, 1, 1)
			must(t, err)
			if view.Details == nil || view.Details.Amount != "14.0001" {
				t.Fatal("reopening changed instructions")
			}
			must(t, db.First(&p, 1).Error)
			if p.Status != constants.PaymentStatusPending || p.PaidAt != nil {
				t.Fatal("cashier mutated payment")
			}
			svc.Enabled = false
			view, err = svc.Read(context.Background(), 1, 1, 1)
			must(t, err)
			if view.State != "paused" || view.Details != nil {
				t.Fatal("disabled cashier exposed instructions")
			}
		})
	}
}

func TestCashierRefusesUnsafeOrChangedOrders(t *testing.T) {
	cases := map[string]func(*testing.T, *gorm.DB, *pd.Payment, *od.Order){
		"no_items": func(t *testing.T, db *gorm.DB, p *pd.Payment, o *od.Order) {
			must(t, db.Where("order_id = ?", 1).Delete(&od.OrderItem{}).Error)
		},
		"upstream": func(t *testing.T, db *gorm.DB, p *pd.Payment, o *od.Order) {
			must(t, db.Model(&od.OrderItem{}).Where("id = 1").Update("fulfillment_type", "upstream").Error)
		},
		"no_allocation": func(t *testing.T, db *gorm.DB, p *pd.Payment, o *od.Order) {
			must(t, db.Where("payment_id = 1").Delete(&pd.Allocation{}).Error)
		},
		"foreign_user": func(t *testing.T, db *gorm.DB, p *pd.Payment, o *od.Order) {
			must(t, db.Model(o).Update("user_id", 2).Error)
		},
		"wrong_online_share": func(t *testing.T, db *gorm.DB, p *pd.Payment, o *od.Order) {
			must(t, db.Model(o).Update("online_paid_amount", 1).Error)
		},
		"changed_terms": func(t *testing.T, db *gorm.DB, p *pd.Payment, o *od.Order) {
			p.PaymentIntent["terms"].(map[string]interface{})["recipient"] = "0x" + strings.Repeat("2", 40)
			must(t, db.Save(p).Error)
		},
		"paid_order": func(t *testing.T, db *gorm.DB, p *pd.Payment, o *od.Order) {
			must(t, db.Model(o).Update("status", constants.OrderStatusPaid).Error)
		},
		"expired_order": func(t *testing.T, db *gorm.DB, p *pd.Payment, o *od.Order) {
			must(t, db.Model(o).Update("expires_at", time.Now().Add(-time.Minute)).Error)
		},
		"missing_gate": func(t *testing.T, db *gorm.DB, p *pd.Payment, o *od.Order) {
			must(t, db.Where("id = 1").Delete(&pd.AutomationControl{}).Error)
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			svc, db, p, o := fixture(t, "binance")
			change(t, db, &p, &o)
			view, _ := svc.Read(context.Background(), 1, 1, 1)
			if view.State == "ready" || view.Details != nil {
				t.Fatal("unsafe order exposed payment instructions")
			}
		})
	}
	svc, _, _, _ := fixture(t, "tron")
	for _, ids := range [][3]uint{{1, 1, 0}, {2, 1, 1}, {1, 2, 1}, {1, 1, 2}} {
		view, err := svc.Read(context.Background(), ids[0], ids[1], ids[2])
		if err == nil || view.Details != nil {
			t.Fatal("foreign/missing payment accepted")
		}
	}
}

func TestUnboundCashierIsPreparingAndDoesNotAllocate(t *testing.T) {
	svc, db, p, _ := fixture(t, "tron")
	delete(p.PaymentIntent, "terms")
	p.ProviderRef = ""
	p.Status = constants.PaymentStatusInitiated
	must(t, db.Save(&p).Error)
	must(t, db.Where("payment_id = ?", p.ID).Delete(&pd.Allocation{}).Error)
	view, err := svc.Read(context.Background(), p.ID, p.OrderID, 1)
	must(t, err)
	if view.State != "preparing" || view.Details != nil {
		t.Fatalf("unexpected view: %+v", view)
	}
	var count int64
	must(t, db.Model(&pd.Allocation{}).Count(&count).Error)
	if count != 0 {
		t.Fatal("read allocated a payment")
	}
}

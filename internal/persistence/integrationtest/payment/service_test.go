package settlement_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	cd "github.com/dujiao-next/internal/modules/catalog/product/domain"
	fd "github.com/dujiao-next/internal/modules/fulfillment/domain"
	ud "github.com/dujiao-next/internal/modules/identity/user/domain"
	od "github.com/dujiao-next/internal/modules/order/domain"
	os "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	app "github.com/dujiao-next/internal/modules/payment/application"
	"github.com/dujiao-next/internal/modules/payment/application/settlement"
	pc "github.com/dujiao-next/internal/modules/payment/contract"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	epusdtadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/epusdt"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/epusdt"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/provider"
	store "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"
	wd "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type verifier struct {
	during func()
	err    error
	stale  bool
}

func (v *verifier) Verify(_ context.Context, e pd.ChainExpectation, hash string) (*pd.ChainProof, error) {
	if v.during != nil {
		v.during()
	}
	if v.err != nil {
		return nil, v.err
	}
	checked := time.Now()
	if v.stale {
		checked = checked.Add(-2 * time.Minute)
	}
	return &pd.ChainProof{Version: 1, Network: e.Network, TransactionHash: hash, EventIndex: 2, BlockHash: "0x" + strings.Repeat("b", 64), BlockNumber: 123, BlockTimeMs: e.CreatedAtMs + 1000, Contract: e.Contract, Recipient: e.Recipient, RawAmount: e.RawAmount, PolicyVersion: "current-test-policy", Sources: []string{"one", "two"}, Finality: []pd.SourceFinality{{Name: "one"}, {Name: "two"}}, CheckedAtMs: checked.UnixMilli()}, nil
}
func openDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	// One real SQLite connection serializes writers; goroutine races still exercise leases.
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&pd.AutomationControl{}, &pd.Payment{}, &pd.Allocation{}, &pd.EventConsumption{}, &pd.PaymentEffect{}, &pd.RecipientReceipt{}, &pd.CandidateReceipt{}, &pd.CandidateEvidence{}, &ud.User{}, &od.Order{}, &od.OrderItem{}, &fd.Fulfillment{}, &cd.Product{}, &wd.Account{}, &wd.Transaction{}); err != nil {
		t.Fatal(err)
	}
	// Synthetic test acceptance; production migrations only create the closed row.
	var controls int64
	must(t, db.Model(&pd.AutomationControl{}).Count(&controls).Error)
	if controls == 0 {
		must(t, automation.Ensure(db))
		must(t, automation.SetFrozen(context.Background(), db, false, 1, "synthetic test acceptance"))
	}
	return db
}
func services(db *gorm.DB, v *verifier) (*settlement.Service, *app.PaymentService, *store.Store) {
	repo := store.New(db, "test-only-guest-secret-32-bytes")
	reg := provider.NewRegistry()
	reg.Register(constants.PaymentProviderEpusdt, "", epusdtadapter.NewEpusdtAdapter())
	application := app.NewPaymentService(app.PaymentServiceOptions{OrderStore: os.New(db, "test-only-guest-secret-32-bytes"), PaymentStore: repo, PaymentProviderRegistry: reg})
	return &settlement.Service{Store: repo, Registry: reg, Verifier: v, Apply: application.ApplyChainSettlement}, application, repo
}
func seed(t *testing.T, db *gorm.DB, id uint, tail int64) (pd.Payment, od.Order) {
	t.Helper()
	created := time.Now().Add(-time.Hour)
	expiry := created.Add(15 * time.Minute)
	// The payment window has passed, but the business order has not been canceled.
	user := ud.User{ID: id, Email: fmt.Sprintf("buyer%d@example.com", id), PasswordHash: "test", Status: "active"}
	must(t, db.Create(&user).Error)
	o := od.Order{ID: id, OrderNo: fmt.Sprintf("ORDER-%d", id), UserID: id, Status: constants.OrderStatusPendingPayment, Currency: "CNY", TotalAmount: amt(110), WalletPaidAmount: amt(10), ExpiresAt: &expiry}
	must(t, db.Create(&o).Error)
	item := od.OrderItem{TitleJSON: jsonmap.JSON{"en": "test"}, OrderID: id, ProductID: id, SKUID: id, Quantity: 1, UnitPrice: amt(110), TotalPrice: amt(110), FulfillmentType: constants.FulfillmentTypeAuto}
	must(t, db.Create(&item).Error)
	o.Items = []od.OrderItem{item}
	p := pd.Payment{ID: id, OrderID: id, ChannelID: 1, ProviderType: constants.PaymentProviderEpusdt, ChannelType: "epusdt", Amount: amt(100), FeePolicy: constants.PaymentFeePolicyNone, Currency: "CNY", Status: constants.PaymentStatusPending, RequiresChainVerification: true, GatewayOrderNo: fmt.Sprintf("DJP-%d", id), ExpiredAt: &expiry}
	cfg := &epusdt.Config{FixedPaymentIntent: true, GatewayURL: "https://gateway.example", PID: "1000", SecretKey: "test-only", OrderMode: "transaction", Token: "usdt", Network: "binance", Currency: "cny", NotifyURL: "https://shop.example/notify", ReturnURL: "https://shop.example/return", QuotePolicy: &epusdt.QuotePolicy{Version: "q1", Rate: "0.14", Precision: 4, MaxTail: "0.0099", LifetimeSeconds: 900, Recipient: "0x" + strings.Repeat("1", 40)}}
	intent, err := epusdt.PrepareIntent(cfg, epusdt.CreateInput{OrderNo: p.GatewayOrderNo, Amount: "100", Name: "test"}, p.ID, o.ID, "CNY", &expiry, nil, created)
	must(t, err)
	p.SettlementBasis = pd.OrderBasis(p, o)
	p.PaymentIntent = intent
	must(t, db.Create(&p).Error)
	p.ProviderRef = fmt.Sprintf("trade-%d", id)
	raw := decimal.RequireFromString("14000000000000000000").Add(decimal.NewFromInt(tail).Mul(decimal.RequireFromString("100000000000000"))).String()
	intent["terms"] = map[string]interface{}{"version": 1, "trade_id": p.ProviderRef, "order_id": p.GatewayOrderNo, "api_key_id": 7, "network": "binance", "token": "USDT", "contract": "0x55d398326f99059ff775485246999027b3197955", "decimals": 18, "recipient": cfg.QuotePolicy.Recipient, "raw_amount": raw, "fiat_amount": "100", "request_amount": "100", "currency": "CNY", "rate": "0.14", "created_at_ms": created.UnixMilli() + 1000, "expires_at_ms": expiry.UnixMilli()}
	expected, err := epusdt.ReadChainExpectation(p)
	must(t, err)
	must(t, store.New(db, "test-only-guest-secret").ReserveAllocation(pd.Allocation{SlotKey: expected.SlotKey(), PaymentID: p.ID, OrderID: o.ID, Network: expected.Network, Contract: expected.Contract, Recipient: expected.Recipient, RawAmount: expected.RawAmount, Snapshot: pd.SettlementSnapshot(p, o), CreatedAt: created.Add(2 * time.Second)}))
	must(t, db.Save(&p).Error)
	return p, o
}
func amt(n int64) money.Amount { return money.FromDecimal(decimal.NewFromInt(n)) }
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func evidence(t *testing.T, db *gorm.DB, p pd.Payment, id, hash string) pd.CandidateEvidence {
	t.Helper()
	c := pd.TransferCandidate{ID: id, OrderID: p.GatewayOrderNo, TradeID: p.ProviderRef, Network: "binance", TransactionHash: hash, RawAmount: "untrusted", Recipient: "attacker", Removed: true}
	repo := store.New(db, "test-only-guest-secret")
	must(t, repo.CommitCandidates(context.Background(), pd.CandidateSource{GatewayURL: "https://gateway.example", PID: "1000"}, []pd.TransferCandidate{c}))
	var e pd.CandidateEvidence
	must(t, db.Order("id DESC").First(&e).Error)
	must(t, db.Model(&e).Updates(map[string]interface{}{"check_state": "chain_checked", "chain_proof": "forged-old-proof", "event_key": "not-authority"}).Error)
	return e
}
func count(t *testing.T, db *gorm.DB, model interface{}) int64 {
	t.Helper()
	var n int64
	must(t, db.Model(model).Count(&n).Error)
	return n
}
func assertUnpaid(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()
	var p pd.Payment
	must(t, db.First(&p, id).Error)
	if p.Status == constants.PaymentStatusSuccess || p.PaidAt != nil {
		t.Fatal("rejected work paid payment")
	}
	if count(t, db, &pd.EventConsumption{}) != 0 || count(t, db, &pd.PaymentEffect{}) != 0 || count(t, db, &wd.Transaction{}) != 0 {
		t.Fatal("rejected work granted benefit")
	}
	var u ud.User
	must(t, db.First(&u, id).Error)
	if !u.TotalSpent.Decimal.IsZero() {
		t.Fatal("rejected work incremented spend")
	}
}
func TestSettlementUniqueEventAndPayment(t *testing.T) {
	db := openDB(t, filepath.Join(t.TempDir(), "merchant.db"))
	p, _ := seed(t, db, 1, 1)
	hash := "0x" + strings.Repeat("a", 64)
	evidence(t, db, p, "1", hash)
	evidence(t, db, p, "2", hash)
	evidence(t, db, p, "3", "0x"+strings.Repeat("c", 64))
	svc, application, _ := services(db, &verifier{})
	must(t, svc.Run(context.Background()))
	must(t, svc.Run(context.Background()))
	if count(t, db, &pd.EventConsumption{}) != 1 {
		t.Fatal("duplicate consumption")
	}
	var stored pd.Payment
	must(t, db.First(&stored, p.ID).Error)
	if stored.Status != constants.PaymentStatusSuccess {
		t.Fatal("not settled")
	}
	var order od.Order
	must(t, db.First(&order, p.OrderID).Error)
	if order.Status != constants.OrderStatusPaid || order.OnlinePaidAmount.String() != "100.00" || order.WalletPaidAmount.String() != "10.00" {
		t.Fatal("wrong split")
	}
	var u ud.User
	must(t, db.First(&u, 1).Error)
	if u.TotalSpent.String() != "110.00" {
		t.Fatal("duplicate spend")
	}
	if _, err := application.HandleCallback(app.PaymentCallbackInput{PaymentID: p.ID, Status: constants.PaymentStatusSuccess, Amount: p.Amount, Currency: p.Currency}); err == nil {
		t.Fatal("callback bypass")
	}
	// Reuse of the event by another otherwise valid order must fail at the DB fence.
	p2, _ := seed(t, db, 2, 2)
	evidence(t, db, p2, "4", hash)
	must(t, svc.Run(context.Background()))
	var second pd.Payment
	must(t, db.First(&second, p2.ID).Error)
	if second.Status == constants.PaymentStatusSuccess || count(t, db, &pd.EventConsumption{}) != 1 {
		t.Fatal("cross-order event replay")
	}
}
func TestSettlementRechecksWhileRPCInFlight(t *testing.T) {
	cases := map[string]func(*gorm.DB, pd.Payment){
		"cancel": func(db *gorm.DB, p pd.Payment) {
			must(t, db.Model(&od.Order{}).Where("id = ?", p.OrderID).Update("status", constants.OrderStatusCanceled).Error)
		},
		"wallet_split": func(db *gorm.DB, p pd.Payment) {
			must(t, db.Model(&od.Order{}).Where("id = ?", p.OrderID).Update("wallet_paid_amount", 0).Error)
		},
		"missing_allocation": func(db *gorm.DB, p pd.Payment) {
			must(t, db.Where("payment_id = ?", p.ID).Delete(&pd.Allocation{}).Error)
		},
		"expired_status": func(db *gorm.DB, p pd.Payment) {
			must(t, db.Model(&p).Update("status", constants.PaymentStatusExpired).Error)
		},
		"snapshot_corrupt": func(db *gorm.DB, p pd.Payment) { must(t, db.Model(&p).Update("settlement_basis", "bad").Error) },
		"guest": func(db *gorm.DB, p pd.Payment) {
			must(t, db.Model(&od.Order{}).Where("id = ?", p.OrderID).Update("user_id", 0).Error)
		},
		"proof_disallowed": func(db *gorm.DB, p pd.Payment) {
			must(t, db.Model(&p).Update("requires_chain_verification", false).Error)
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			db := openDB(t, filepath.Join(t.TempDir(), "merchant.db"))
			p, _ := seed(t, db, 1, 1)
			evidence(t, db, p, "1", "hash")
			svc, _, _ := services(db, &verifier{during: func() { change(db, p) }})
			must(t, svc.Run(context.Background()))
			assertUnpaid(t, db, 1)
		})
	}
}
func TestSettlementRollbackIncludesOutboxAndSpend(t *testing.T) {
	db := openDB(t, filepath.Join(t.TempDir(), "merchant.db"))
	p, _ := seed(t, db, 1, 1)
	e := evidence(t, db, p, "1", "hash")
	must(t, db.Exec("CREATE TRIGGER reject_effect BEFORE INSERT ON payment_effects BEGIN SELECT RAISE(ABORT, 'injected failure'); END").Error)
	svc, _, _ := services(db, &verifier{})
	if svc.Run(context.Background()) == nil {
		t.Fatal("failure swallowed")
	}
	assertUnpaid(t, db, 1)
	must(t, db.Exec("DROP TRIGGER reject_effect").Error)
	must(t, db.Model(&e).Update("check_after", time.Now().Add(-time.Second)).Error)
	must(t, svc.Run(context.Background()))
	if count(t, db, &pd.EventConsumption{}) != 1 {
		t.Fatal("retry failed")
	}
}
func TestSettlementProofFreshnessAndConflict(t *testing.T) {
	for _, mode := range []string{"stale", "rpc_failure", "outside_window", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			db := openDB(t, filepath.Join(t.TempDir(), "merchant.db"))
			p, _ := seed(t, db, 1, 1)
			evidence(t, db, p, "1", "hash")
			v := &verifier{stale: mode == "stale"}
			if mode == "rpc_failure" {
				v.err = pc.ErrChainUnavailable
			}
			if mode == "outside_window" {
				v.err = pc.ErrChainOutsideWindow
			}
			if mode == "conflict" {
				v.during = func() { evidence(t, db, p, "1", "different-hash") }
			}
			svc, _, _ := services(db, v)
			must(t, svc.Run(context.Background()))
			assertUnpaid(t, db, 1)
		})
	}
}
func TestSettlementConcurrentWorkersAndLeaseTakeover(t *testing.T) {
	db := openDB(t, filepath.Join(t.TempDir(), "merchant.db"))
	p, _ := seed(t, db, 1, 1)
	evidence(t, db, p, "1", "hash")
	_, _, repo := services(db, &verifier{})
	old, err := repo.LeaseSettlements(context.Background(), time.Now())
	must(t, err)
	if len(old) != 1 {
		t.Fatal("missing lease")
	}
	must(t, db.Model(&pd.CandidateEvidence{}).Where("id = ?", old[0].Evidence.ID).Update("check_after", time.Now().Add(-time.Minute)).Error)
	a, _, _ := services(db, &verifier{})
	b, _, _ := services(db, &verifier{})
	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	go func() { defer wg.Done(); errs <- a.Run(context.Background()) }()
	go func() { defer wg.Done(); errs <- b.Run(context.Background()) }()
	wg.Wait()
	close(errs)
	for err := range errs {
		must(t, err)
	}
	called := false
	must(t, repo.WithSettlement(context.Background(), old[0], func(pc.SettlementTransaction, *pd.Payment, *od.Order, *pd.CandidateEvidence, *pd.CandidateReceipt) error {
		called = true
		return nil
	}))
	if called || count(t, db, &pd.EventConsumption{}) != 1 {
		t.Fatal("stale lease or duplicate settlement")
	}
}
func TestAllocationHistorySurvivesExpiryAndDeletion(t *testing.T) {
	db := openDB(t, filepath.Join(t.TempDir(), "merchant.db"))
	p, _ := seed(t, db, 1, 1)
	var a pd.Allocation
	must(t, db.First(&a).Error)
	must(t, db.Model(&p).Updates(map[string]interface{}{"status": constants.PaymentStatusExpired, "deleted_at": time.Now()}).Error)
	other := a
	other.ID = 0
	other.PaymentID = 2
	other.OrderID = 2
	repo := store.New(db, "test-only-guest-secret")
	if !errors.Is(repo.ReserveAllocation(other), pc.ErrAllocationConflict) {
		t.Fatal("historical slot released")
	}
	// Incomplete pre-ledger history blocks even a different slot; never synthesize ownership.
	must(t, db.Delete(&a).Error)
	other.SlotKey = strings.Repeat("b", 64)
	if !errors.Is(repo.ReserveAllocation(other), pc.ErrAllocationConflict) {
		t.Fatal("unindexed bound history ignored")
	}
}

type effectRecorder struct {
	calls int
	err   error
}

func (h *effectRecorder) HandlePaymentEffect(context.Context, pd.PaymentEffect) (bool, error) {
	h.calls++
	return h.err == nil, h.err
}
func TestPaymentEffectsSurviveRealDBRestartAndLostAcknowledgment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "merchant.db")
	db := openDB(t, path)
	p, _ := seed(t, db, 1, 1)
	evidence(t, db, p, "1", "hash")
	svc, _, repo := services(db, &verifier{})
	must(t, svc.Run(context.Background()))
	total := count(t, db, &pd.PaymentEffect{})
	leased, err := repo.LeasePaymentEffects(context.Background(), time.Now())
	must(t, err)
	if len(leased) != 1 {
		t.Fatal("missing effect lease")
	}
	sqlDB, _ := db.DB()
	must(t, sqlDB.Close())
	db = openDB(t, path)
	_, _, repo = services(db, &verifier{})
	must(t, db.Model(&pd.PaymentEffect{}).Where("id = ?", leased[0].ID).Update("after", time.Now().Add(-time.Minute)).Error)
	h := &effectRecorder{err: errors.New("queue unavailable")}
	runner := &settlement.Effects{Store: repo, Handler: h}
	if runner.Run(context.Background()) == nil {
		t.Fatal("effect failure swallowed")
	}
	var done int64
	must(t, db.Model(&pd.PaymentEffect{}).Where("state = ?", "done").Count(&done).Error)
	if done != 0 {
		t.Fatal("failed effects acknowledged")
	}
	must(t, db.Model(&pd.PaymentEffect{}).Where("state = ?", "pending").Update("after", time.Now().Add(-time.Minute)).Error)
	h.err = nil
	must(t, runner.Run(context.Background()))
	must(t, db.Model(&pd.PaymentEffect{}).Where("state = ?", "done").Count(&done).Error)
	if done != total {
		t.Fatal("restart lost effects")
	}
	// A pre-restart token must never revert an acknowledged effect.
	must(t, repo.FinishPaymentEffect(context.Background(), leased[0], "pending", "old", time.Now()))
	must(t, db.Model(&pd.PaymentEffect{}).Where("state = ?", "done").Count(&done).Error)
	if done != total {
		t.Fatal("stale effect lease won")
	}
	var u ud.User
	must(t, db.First(&u, 1).Error)
	if u.TotalSpent.String() != "110.00" {
		t.Fatal("restart duplicated financial benefit")
	}
	var c pd.EventConsumption
	must(t, db.First(&c).Error)
	var proof pd.ChainProof
	must(t, json.Unmarshal([]byte(c.Proof), &proof))
	if proof.PolicyVersion != "current-test-policy" {
		t.Fatal("used saved forged proof")
	}
}

func TestSettlementRequiresPriorMerchantAllocationAndSupportedDelivery(t *testing.T) {
	for _, kind := range []string{"before_allocation", "upstream"} {
		t.Run(kind, func(t *testing.T) {
			db := openDB(t, filepath.Join(t.TempDir(), "merchant.db"))
			p, o := seed(t, db, 1, 1)
			if kind == "before_allocation" {
				must(t, db.Model(&pd.Allocation{}).Where("payment_id = ?", p.ID).Update("created_at", time.Now()).Error)
			} else {
				must(t, db.Model(&od.OrderItem{}).Where("order_id = ?", o.ID).Update("fulfillment_type", constants.FulfillmentTypeUpstream).Error)
				must(t, db.Preload("Items").First(&o, o.ID).Error)
				p.SettlementBasis = pd.OrderBasis(p, o)
				must(t, db.Save(&p).Error)
				must(t, db.Model(&pd.Allocation{}).Where("payment_id = ?", p.ID).Update("snapshot", pd.SettlementSnapshot(p, o)).Error)
			}
			e := evidence(t, db, p, "1", "hash")
			svc, _, _ := services(db, &verifier{})
			must(t, svc.Run(context.Background()))
			assertUnpaid(t, db, p.ID)
			e = pd.CandidateEvidence{ID: e.ID}
			must(t, db.First(&e).Error)
			want := "before_merchant_allocation"
			if kind == "upstream" {
				want = "upstream_delivery_not_ready"
			}
			if e.CheckCode != want {
				t.Fatalf("code=%s want=%s", e.CheckCode, want)
			}
		})
	}
}
func TestAllocationCompetingPaymentClaims(t *testing.T) {
	db := openDB(t, filepath.Join(t.TempDir(), "merchant.db"))
	repo := store.New(db, "test-only-guest-secret")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	start := make(chan struct{})
	for id := uint(1); id <= 2; id++ {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			<-start
			errs <- repo.ReserveAllocation(pd.Allocation{SlotKey: strings.Repeat("a", 64), PaymentID: id, OrderID: id, Snapshot: strings.Repeat("b", 64)})
		}(id)
	}
	close(start)
	wg.Wait()
	close(errs)
	won := 0
	for err := range errs {
		if err == nil {
			won++
		} else if !errors.Is(err, pc.ErrAllocationConflict) {
			t.Fatal(err)
		}
	}
	if won != 1 || count(t, db, &pd.Allocation{}) != 1 {
		t.Fatal("competing allocations did not have one owner")
	}
}

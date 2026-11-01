package chainverification_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/application/chainverification"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	epusdtadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/epusdt"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/epusdt"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/provider"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type fakeVerifier struct {
	calls  int
	err    error
	during func()
	last   paymentdomain.ChainExpectation
}

func (f *fakeVerifier) Verify(_ context.Context, e paymentdomain.ChainExpectation, tx string) (*paymentdomain.ChainProof, error) {
	f.calls++
	f.last = e
	if f.during != nil {
		f.during()
	}
	if f.err != nil {
		return nil, f.err
	}
	return &paymentdomain.ChainProof{Version: 1, Network: e.Network, TransactionHash: tx, EventIndex: 2, BlockHash: "0x" + strings.Repeat("b", 64), BlockNumber: 100, BlockTimeMs: e.CreatedAtMs + 5000,
		Contract: e.Contract, Recipient: e.Recipient, RawAmount: e.RawAmount, PolicyVersion: "test-v1", Sources: []string{"one", "two"}, Finality: []paymentdomain.SourceFinality{{Name: "one"}, {Name: "two"}}, CheckedAtMs: time.Now().UnixMilli()}, nil
}

func seedBoundPayment(t *testing.T, db *gorm.DB) paymentdomain.Payment {
	t.Helper()
	cfg := &epusdt.Config{FixedPaymentIntent: true, GatewayURL: "https://gateway.example", PID: "1000", SecretKey: "test-secret", OrderMode: "transaction", Token: "usdt", Network: "binance", Currency: "cny",
		NotifyURL: "https://shop.example/notify", ReturnURL: "https://shop.example/return", QuotePolicy: &epusdt.QuotePolicy{Version: "quote-v1", Rate: "0.14", Precision: 4, MaxTail: "0.0099", LifetimeSeconds: 900, Recipient: "0x" + strings.Repeat("1", 40)}}
	created := time.Now().Add(-time.Hour)
	saved, err := epusdt.PrepareIntent(cfg, epusdt.CreateInput{OrderNo: "DJP-1", Amount: "100", Name: "test"}, 1, 2, "CNY", nil, nil, created)
	if err != nil {
		t.Fatal(err)
	}
	saved["terms"] = map[string]interface{}{"version": 1, "trade_id": "trade-1", "order_id": "DJP-1", "api_key_id": 7, "network": "binance", "token": "USDT", "contract": "0x55d398326f99059ff775485246999027b3197955",
		"decimals": 18, "recipient": cfg.QuotePolicy.Recipient, "raw_amount": "14000100000000000000", "fiat_amount": "100", "currency": "CNY", "request_amount": "100", "rate": "0.14", "created_at_ms": created.UnixMilli() + 1000, "expires_at_ms": created.UnixMilli() + 600000}
	p := paymentdomain.Payment{ID: 1, OrderID: 2, GatewayOrderNo: "DJP-1", ProviderRef: "trade-1", ProviderType: constants.PaymentProviderEpusdt, ChannelType: "epusdt", RequiresChainVerification: true, PaymentIntent: saved,
		Amount: money.FromDecimal(decimal.NewFromInt(100)), Currency: "CNY", Status: constants.PaymentStatusExpired}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	return p
}

func openCheckDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&paymentdomain.Payment{}, &paymentdomain.CandidateReceipt{}, &paymentdomain.CandidateSubscription{}, &paymentdomain.CandidateEvidence{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newCheckService(db *gorm.DB, f *fakeVerifier) *chainverification.Service {
	reg := provider.NewRegistry()
	reg.Register(constants.PaymentProviderEpusdt, "", epusdtadapter.NewEpusdtAdapter())
	return &chainverification.Service{Store: gormstore.New(db, "test-guest-key-32-bytes"), Registry: reg, Verifier: f}
}

func TestChainChecksPersistRetryAcrossRestartWithoutSettlement(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "merchant.db")
	db := openCheckDB(t, dsn)
	defer func() { conn, _ := db.DB(); _ = conn.Close() }()
	payment := seedBoundPayment(t, db)
	source := paymentdomain.CandidateSource{GatewayURL: "https://gateway.example", PID: "1000"}
	index := uint64(999)
	candidate := paymentdomain.TransferCandidate{ID: "1", OrderID: "DJP-1", TradeID: "trade-1", Network: "binance", TransactionHash: "0x" + strings.Repeat("1", 64),
		RawAmount: "1", Recipient: "untrusted", BlockTimeMs: 1, EventIndex: &index, Removed: true, ReviewReason: "late"}
	repo := gormstore.New(db, "test-guest-key-32-bytes")
	if err := repo.CommitCandidates(context.Background(), source, []paymentdomain.TransferCandidate{candidate}); err != nil {
		t.Fatal(err)
	}
	f := &fakeVerifier{err: paymentcontract.ErrChainPending}
	if err := newCheckService(db, f).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var evidence paymentdomain.CandidateEvidence
	if err := db.First(&evidence).Error; err != nil || evidence.CheckState != "retry" || evidence.CheckCode != "not_finalized" || evidence.ChainProof != "" || f.calls != 1 {
		t.Fatalf("pending result not persisted: %v %#v", err, evidence)
	}
	if f.last.RawAmount == candidate.RawAmount || f.last.Recipient == candidate.Recipient {
		t.Fatal("gateway amount/address became authority")
	}
	// SQL data, not the previous process's verifier instance, drives recovery.
	conn, _ := db.DB()
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	db = openCheckDB(t, dsn)
	if err := db.Model(&paymentdomain.CandidateEvidence{}).Where("id = ?", evidence.ID).Update("check_after", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	f = &fakeVerifier{}
	if err := newCheckService(db, f).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	evidence = paymentdomain.CandidateEvidence{}
	if err := db.First(&evidence).Error; err != nil || evidence.CheckState != "chain_checked" || evidence.CheckToken != "" || evidence.EventKey == "" {
		t.Fatalf("proof not committed: %v %#v", err, evidence)
	}
	var proof paymentdomain.ChainProof
	if json.Unmarshal([]byte(evidence.ChainProof), &proof) != nil || proof.EventIndex != 2 || proof.RawAmount != f.last.RawAmount {
		t.Fatal("proof did not use independent facts")
	}
	if err := db.First(&payment, payment.ID).Error; err != nil || payment.Status != constants.PaymentStatusExpired || payment.PaidAt != nil {
		t.Fatal("verification settled/reopened payment")
	}
	if err := newCheckService(db, f).Run(context.Background()); err != nil || f.calls != 1 {
		t.Fatal("finished proof unnecessarily retried")
	}
	// A later conflicting gateway version invalidates readiness, including an
	// earlier successful proof. Evidence remains available for investigation.
	candidate.RawAmount = "2"
	if err := gormstore.New(db, "test-guest-key-32-bytes").CommitCandidates(context.Background(), source, []paymentdomain.TransferCandidate{candidate}); err != nil {
		t.Fatal(err)
	}
	var all []paymentdomain.CandidateEvidence
	if err := db.Find(&all).Error; err != nil || len(all) != 2 {
		t.Fatal("conflict evidence missing")
	}
	for _, row := range all {
		if row.CheckState != "manual" || row.CheckCode != "candidate_conflict" {
			t.Fatal("conflict left a usable verified state")
		}
	}
}

func TestChainCheckLeaseAndChangesDuringRPC(t *testing.T) {
	for _, change := range []string{"conflicting evidence", "changed authorization", "expired lease"} {
		t.Run(change, func(t *testing.T) {
			db := openCheckDB(t, filepath.Join(t.TempDir(), "merchant.db"))
			defer func() { conn, _ := db.DB(); _ = conn.Close() }()
			seedBoundPayment(t, db)
			repo := gormstore.New(db, "test-guest-key-32-bytes")
			source := paymentdomain.CandidateSource{GatewayURL: "https://gateway.example", PID: "1000"}
			candidate := paymentdomain.TransferCandidate{ID: "1", OrderID: "DJP-1", TradeID: "trade-1", Network: "binance", TransactionHash: "0x" + strings.Repeat("1", 64)}
			if err := repo.CommitCandidates(context.Background(), source, []paymentdomain.TransferCandidate{candidate}); err != nil {
				t.Fatal(err)
			}
			f := &fakeVerifier{during: func() {
				switch change {
				case "conflicting evidence":
					candidate.RawAmount = "changed"
					if err := repo.CommitCandidates(context.Background(), source, []paymentdomain.TransferCandidate{candidate}); err != nil {
						t.Fatal(err)
					}
				case "changed authorization":
					if err := db.Model(&paymentdomain.Payment{}).Where("id = 1").Update("payment_intent", jsonmap.JSON{"changed": true}).Error; err != nil {
						t.Fatal(err)
					}
				case "expired lease":
					if err := db.Model(&paymentdomain.CandidateEvidence{}).Where("id = 1").Update("check_after", time.Now().Add(-time.Second)).Error; err != nil {
						t.Fatal(err)
					}
					if items, err := repo.LeaseChainChecks(context.Background(), time.Now(), 1); err != nil || len(items) != 1 {
						t.Fatal("expired lease not reclaimable")
					}
				}
			}}
			if err := newCheckService(db, f).Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			var row paymentdomain.CandidateEvidence
			if err := db.First(&row).Error; err != nil || row.CheckState == "chain_checked" || row.ChainProof != "" {
				t.Fatalf("stale/changed check committed: %v %#v", err, row)
			}
		})
	}
}

func TestChainMismatchClassificationAndIdentity(t *testing.T) {
	for _, scenario := range []string{"wrong source", "unbound", "outside window", "multiple transfers", "source unavailable", "disagreement"} {
		t.Run(scenario, func(t *testing.T) {
			db := openCheckDB(t, filepath.Join(t.TempDir(), "merchant.db"))
			defer func() { conn, _ := db.DB(); _ = conn.Close() }()
			p := seedBoundPayment(t, db)
			source := paymentdomain.CandidateSource{GatewayURL: "https://gateway.example", PID: "1000"}
			f := &fakeVerifier{}
			switch scenario {
			case "wrong source":
				source.PID = "other"
			case "unbound":
				delete(p.PaymentIntent, "terms")
				if err := db.Save(&p).Error; err != nil {
					t.Fatal(err)
				}
			case "outside window":
				f.err = paymentcontract.ErrChainOutsideWindow
			case "multiple transfers":
				f.err = paymentcontract.ErrChainAmbiguous
			case "source unavailable":
				f.err = paymentcontract.ErrChainUnavailable
			case "disagreement":
				f.err = paymentcontract.ErrChainDisagreement
			}
			repo := gormstore.New(db, "test-guest-key-32-bytes")
			if err := repo.CommitCandidates(context.Background(), source, []paymentdomain.TransferCandidate{{ID: "1", OrderID: "DJP-1", TradeID: "trade-1", Network: "binance", TransactionHash: "0x" + strings.Repeat("1", 64)}}); err != nil {
				t.Fatal(err)
			}
			if err := newCheckService(db, f).Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			var row paymentdomain.CandidateEvidence
			if err := db.First(&row).Error; err != nil {
				t.Fatal(err)
			}
			if row.CheckState == "chain_checked" || row.ChainProof != "" {
				t.Fatal("invalid verification accepted")
			}
			if (scenario == "wrong source" || scenario == "unbound") && f.calls != 0 {
				t.Fatal("untrusted identity reached RPC")
			}
		})
	}
}

func TestChainCheckConcurrentLeaseHasOneWinner(t *testing.T) {
	db := openCheckDB(t, filepath.Join(t.TempDir(), "merchant.db"))
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	defer conn.Close()
	seedBoundPayment(t, db)
	repo := gormstore.New(db, "test-guest-key-32-bytes")
	if err := repo.CommitCandidates(context.Background(), paymentdomain.CandidateSource{GatewayURL: "https://gateway.example", PID: "1000"}, []paymentdomain.TransferCandidate{{ID: "1", OrderID: "DJP-1", TradeID: "trade-1"}}); err != nil {
		t.Fatal(err)
	}
	counts := make([]int, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range counts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rows, err := repo.LeaseChainChecks(context.Background(), time.Now(), 1)
			counts[i], errs[i] = len(rows), err
		}(i)
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || counts[0]+counts[1] != 1 {
		t.Fatalf("lease race counts=%v errors=%v", counts, errs)
	}
}

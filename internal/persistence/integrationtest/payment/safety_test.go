package settlement_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	cs "github.com/dujiao-next/internal/modules/cardsecret/domain"
	fa "github.com/dujiao-next/internal/modules/fulfillment/application"
	fd "github.com/dujiao-next/internal/modules/fulfillment/domain"
	fs "github.com/dujiao-next/internal/modules/fulfillment/infrastructure/gormstore"
	od "github.com/dujiao-next/internal/modules/order/domain"
	os "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	pc "github.com/dujiao-next/internal/modules/payment/contract"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	store "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestIncompleteAllocationHistoryCannotAuthorizeNewSlots(t *testing.T) {
	cases := []struct {
		name    string
		change  func(*pd.Payment)
		allowed bool
	}{
		{"valid_unbound_expired_deleted", func(p *pd.Payment) {}, true},
		{"missing_basis", func(p *pd.Payment) { p.SettlementBasis = "" }, false},
		{"non_hash_basis", func(p *pd.Payment) { p.SettlementBasis = strings.Repeat("z", 64) }, false},
		{"missing_authorization", func(p *pd.Payment) { p.PaymentIntent = nil }, false},
		{"unknown_version", func(p *pd.Payment) { p.PaymentIntent["version"] = 99 }, false},
		{"wrong_raw_quote", func(p *pd.Payment) { p.PaymentIntent["base_raw"] = "1" }, false},
		{"wrong_owner", func(p *pd.Payment) { p.PaymentIntent["payment_id"] = 999 }, false},
		{"terms_no_reference", func(p *pd.Payment) { p.PaymentIntent["terms"] = map[string]interface{}{"trade_id": "lost"} }, false},
		{"null_terms", func(p *pd.Payment) { p.PaymentIntent["terms"] = nil }, false},
		{"empty_terms", func(p *pd.Payment) { p.PaymentIntent["terms"] = map[string]interface{}{} }, false},
		{"scalar_terms", func(p *pd.Payment) { p.PaymentIntent["terms"] = "corrupt" }, false},
		{"reference_no_terms", func(p *pd.Payment) { p.ProviderRef = "lost" }, false},
		{"already_paid", func(p *pd.Payment) { p.Status = constants.PaymentStatusSuccess }, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			db := openDB(t, filepath.Join(t.TempDir(), "history.db"))
			p, _ := seed(t, db, 1, 1)
			must(t, db.Where("payment_id = ?", p.ID).Delete(&pd.Allocation{}).Error)
			delete(p.PaymentIntent, "terms")
			p.ProviderRef = ""
			p.Status = constants.PaymentStatusExpired
			now := time.Now()
			p.DeletedAt = &now
			tt.change(&p)
			must(t, db.Save(&p).Error)
			err := store.New(db, "test-guest-key").ReserveAllocation(pd.Allocation{PaymentID: 2, OrderID: 2, SlotKey: strings.Repeat("c", 64), Snapshot: strings.Repeat("d", 64)})
			if tt.allowed {
				must(t, err)
			} else if !errors.Is(err, pc.ErrAllocationConflict) {
				t.Fatalf("history accepted: %v", err)
			}
		})
	}
}

func TestFreezeDuringRPCInvalidatesSettlementEvenAfterResume(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(map[bool]string{false: "frozen", true: "resumed_generation"}[resume], func(t *testing.T) {
			db := openDB(t, filepath.Join(t.TempDir(), "merchant.db"))
			p, _ := seed(t, db, 1, 1)
			e := evidence(t, db, p, "1", "hash")
			v := &verifier{during: func() {
				must(t, automation.SetFrozen(context.Background(), db, true, 2, "synthetic incident"))
				if resume {
					must(t, automation.SetFrozen(context.Background(), db, false, 3, "synthetic review completed"))
				}
			}}
			svc, _, _ := services(db, v)
			if err := svc.Run(context.Background()); !errors.Is(err, pd.ErrAutomationFrozen) {
				t.Fatalf("stale proof survived freeze: %v", err)
			}
			if count(t, db, &pd.EventConsumption{}) != 0 || count(t, db, &pd.PaymentEffect{}) != 0 {
				t.Fatal("frozen settlement committed")
			}
			var saved pd.Payment
			must(t, db.First(&saved, p.ID).Error)
			if saved.Status != constants.PaymentStatusPending {
				t.Fatal("frozen payment changed")
			}
			if !resume {
				must(t, automation.SetFrozen(context.Background(), db, false, 3, "synthetic review completed"))
			}
			// The durable lease is recoverable; a new generation must verify again.
			must(t, db.Model(&pd.CandidateEvidence{}).Where("id = ?", e.ID).Update("check_after", time.Now().Add(-time.Minute)).Error)
			v.during = nil
			must(t, svc.Run(context.Background()))
			if count(t, db, &pd.EventConsumption{}) != 1 {
				t.Fatal("resume did not recover settlement")
			}
		})
	}
}

func TestFreezeBlocksQueuedAndManualFixedDeliveryAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "merchant.db")
	db := openDB(t, path)
	must(t, db.AutoMigrate(&cs.Secret{}, &cs.Batch{}))
	p, _ := seed(t, db, 1, 1)
	evidence(t, db, p, "1", "hash")
	svc, _, repo := services(db, &verifier{})
	must(t, svc.Run(context.Background()))
	card := cs.Secret{ProductID: 1, SKUID: 1, Secret: "synthetic-card", Status: cs.StatusAvailable}
	must(t, db.Create(&card).Error)
	leaves, err := repo.LeasePaymentEffects(context.Background(), time.Now())
	must(t, err)
	if len(leaves) != 1 {
		t.Fatal("missing prefreeze lease")
	}
	must(t, automation.SetFrozen(context.Background(), db, true, 2, "synthetic incident"))
	conn, _ := db.DB()
	must(t, conn.Close())
	db = openDB(t, path)
	_, app, repo := services(db, &verifier{})
	if _, err := app.HandlePaymentEffect(context.Background(), leaves[0]); !errors.Is(err, pd.ErrAutomationFrozen) {
		t.Fatalf("prefreeze effect ran: %v", err)
	}
	rows, err := repo.LeasePaymentEffects(context.Background(), time.Now().Add(time.Hour))
	must(t, err)
	if len(rows) != 0 {
		t.Fatal("frozen outbox leased work")
	}
	delivery := fa.New(fa.Options{OrderStore: os.New(db, "test-guest-key"), FulfillmentStore: fs.New(db)})
	for _, run := range []func() error{
		func() error { _, err := delivery.CreateAutoWithDurableEffects(p.OrderID); return err },
		func() error { _, err := delivery.CreateAuto(p.OrderID); return err },
		func() error {
			_, err := delivery.CreateManual(fa.CreateManualInput{OrderID: p.OrderID, AdminID: 1, Payload: "synthetic"})
			return err
		},
	} {
		if run() == nil {
			t.Fatal("delivery bypassed freeze")
		}
	}
	if count(t, db, &fd.Fulfillment{}) != 0 {
		t.Fatal("frozen delivery persisted")
	}
	var saved cs.Secret
	must(t, db.First(&saved, card.ID).Error)
	if saved.Status != cs.StatusAvailable {
		t.Fatal("frozen card consumed")
	}
	// Non-USDT legacy order remains fulfillable while USDT is frozen.
	o := od.Order{ID: 2, OrderNo: "ordinary", Status: constants.OrderStatusPaid}
	must(t, db.Create(&o).Error)
	item := od.OrderItem{OrderID: 2, ProductID: 1, SKUID: 1, Quantity: 1, FulfillmentType: constants.FulfillmentTypeAuto, TitleJSON: jsonmap.JSON{"en": "test"}}
	must(t, db.Create(&item).Error)
	_, err = delivery.CreateAuto(2)
	must(t, err)
	must(t, automation.SetFrozen(context.Background(), db, false, 3, "synthetic review completed"))
	second := cs.Secret{ProductID: 1, SKUID: 1, Secret: "second synthetic card", Status: cs.StatusAvailable}
	must(t, db.Create(&second).Error)
	_, err = delivery.CreateAutoWithDurableEffects(p.OrderID)
	must(t, err)
}

func TestFixedDeliveryRechecksPaidAllocationSnapshot(t *testing.T) {
	db := openDB(t, filepath.Join(t.TempDir(), "merchant.db"))
	must(t, db.AutoMigrate(&cs.Secret{}, &cs.Batch{}))
	p, _ := seed(t, db, 1, 1)
	evidence(t, db, p, "1", "hash")
	svc, _, _ := services(db, &verifier{})
	must(t, svc.Run(context.Background()))
	must(t, db.Create(&cs.Secret{ProductID: 1, SKUID: 1, Secret: "synthetic", Status: cs.StatusAvailable}).Error)
	must(t, db.Create(&cs.Secret{ProductID: 1, SKUID: 1, Secret: "second synthetic", Status: cs.StatusAvailable}).Error)
	must(t, db.Model(&od.OrderItem{}).Where("order_id = ?", p.OrderID).Update("quantity", 2).Error)
	delivery := fa.New(fa.Options{OrderStore: os.New(db, "test-guest-key"), FulfillmentStore: fs.New(db)})
	if _, err := delivery.CreateAuto(p.OrderID); err == nil {
		t.Fatal("changed goods accepted")
	}
	if count(t, db, &fd.Fulfillment{}) != 0 {
		t.Fatal("changed snapshot delivered")
	}
	var used int64
	must(t, db.Model(&cs.Secret{}).Where("status = ?", cs.StatusUsed).Count(&used).Error)
	if used != 0 {
		t.Fatal("changed snapshot consumed cards")
	}
}

func TestSettlementAcceptsOnlyConsistentExpectedOnlineShare(t *testing.T) {
	for _, share := range []int64{100, 99} {
		db := openDB(t, filepath.Join(t.TempDir(), "online.db"))
		p, o := seed(t, db, 1, 1)
		must(t, db.Model(&o).Update("online_paid_amount", amt(share)).Error)
		evidence(t, db, p, "1", "hash")
		svc, _, _ := services(db, &verifier{})
		must(t, svc.Run(context.Background()))
		want := int64(0)
		if share == 100 {
			want = 1
		}
		if count(t, db, &pd.EventConsumption{}) != want {
			t.Fatalf("incorrect checkout share handling: %d", share)
		}
	}
}

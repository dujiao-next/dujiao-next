package candidates_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/application/candidates"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	epusdtadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/epusdt"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/epusdt"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/provider"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCandidateDeliveryCommitBeforeACKAndRestart(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "merchant.db")
	open := func() *gorm.DB {
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.AutoMigrate(&paymentdomain.Payment{}, &paymentdomain.PaymentChannel{}, &paymentdomain.CandidateReceipt{}, &paymentdomain.CandidateSubscription{}, &paymentdomain.CandidateEvidence{}); err != nil {
			t.Fatal(err)
		}
		return db
	}
	db := open()
	defer func() { conn, _ := db.DB(); _ = conn.Close() }()
	var acks, lists atomic.Int32
	var alter, loseACK atomic.Bool
	loseACK.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var params map[string]interface{}
		if json.NewDecoder(r.Body).Decode(&params) != nil {
			t.Error("invalid request")
			w.WriteHeader(400)
			return
		}
		signature := params["signature"]
		delete(params, "signature")
		if signature != epusdt.Sign(params, "test-secret") {
			t.Error("signature mismatch")
			w.WriteHeader(403)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/ack") {
			acks.Add(1)
			if params["action"] != "transfer_candidates.ack.v1" || params["ids"] != "18446744073709551615" {
				t.Error("ACK action/uint64 string not preserved")
			}
			var count int64
			if err := db.Model(&paymentdomain.CandidateEvidence{}).Count(&count).Error; err != nil || count == 0 {
				t.Error("ACK before durable evidence")
			}
			if loseACK.Swap(false) {
				w.WriteHeader(503)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status_code": 200, "data": map[string]interface{}{"received": true}})
			return
		}
		lists.Add(1)
		if params["action"] != "transfer_candidates.list.v1" {
			t.Error("wrong list action")
		}
		amount := "14000100000000000000"
		if alter.Load() {
			amount = "1"
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status_code": 200, "data": []paymentdomain.TransferCandidate{{
			ID: "18446744073709551615", OrderID: "DJP-1", TradeID: "trade-1", Network: "binance", Kind: "evm_log", RawAmount: amount,
			TransactionHash: "0x" + strings.Repeat("1", 64), ReviewReason: "independent_check_required"}}})
	}))
	defer srv.Close()
	channel := paymentdomain.PaymentChannel{ProviderType: constants.PaymentProviderEpusdt, ChannelType: "epusdt", IsActive: false,
		ConfigJSON: jsonmap.JSON{"gateway_url": srv.URL, "pid": "1000", "secret_key": "test-secret", "notify_url": "https://shop.example/notify", "return_url": "https://shop.example/return",
			"token": "usdt", "network": "binance", "currency": "cny", "fixed_payment_intent": true,
			"quote_policy": map[string]interface{}{"version": "v1", "rate": "0.14", "precision": 4, "max_tail": "0.0099", "lifetime_seconds": 900, "recipient": "0x1111111111111111111111111111111111111111"}}}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	payment := paymentdomain.Payment{ID: 1, OrderID: 2, ChannelID: channel.ID, ProviderType: constants.PaymentProviderEpusdt,
		GatewayOrderNo: "DJP-1", ProviderRef: "trade-1", RequiresChainVerification: true, Status: constants.PaymentStatusPending,
		PaymentIntent: jsonmap.JSON{"payment_id": 1, "order_id": 2, "order_no": "DJP-1", "gateway_url": srv.URL, "pid": "1000", "terms": map[string]interface{}{"trade_id": "trade-1"}}}
	if err := db.Create(&payment).Error; err != nil {
		t.Fatal(err)
	}
	reg := provider.NewRegistry()
	reg.Register(constants.PaymentProviderEpusdt, "", epusdtadapter.NewEpusdtAdapter())
	service := func() *candidates.Service {
		return &candidates.Service{Channels: gormstore.NewChannelStore(db), Registry: reg, Inbox: gormstore.New(db, "test-secret-32-bytes-for-guest-key")}
	}
	if err := db.Exec("CREATE TRIGGER reject_evidence BEFORE INSERT ON payment_candidate_evidence BEGIN SELECT RAISE(ABORT, 'test write failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	if service().Run(context.Background()) == nil || acks.Load() != 0 {
		t.Fatal("write failure acknowledged")
	}
	var count int64
	if err := db.Model(&paymentdomain.CandidateReceipt{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("batch not rolled back")
	}
	if err := db.Exec("DROP TRIGGER reject_evidence").Error; err != nil {
		t.Fatal(err)
	}
	if service().Run(context.Background()) == nil || acks.Load() != 1 {
		t.Fatal("lost ACK response not reported")
	}
	// Genuine SQL close/reopen plus a fresh application service; no in-memory cursor.
	conn, _ := db.DB()
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	db = open()
	if err := service().Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&paymentdomain.CandidateEvidence{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("redelivery duplicated evidence")
	}
	var evidence paymentdomain.CandidateEvidence
	if err := db.First(&evidence).Error; err != nil || evidence.PaymentID == nil || *evidence.PaymentID != payment.ID || evidence.MatchState != "awaiting_chain_check" {
		t.Fatal("local identity not matched")
	}
	alter.Store(true)
	if err := service().Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var receipt paymentdomain.CandidateReceipt
	if err := db.First(&receipt).Error; err != nil || !receipt.Conflicted {
		t.Fatal("candidate ID content replacement not flagged")
	}
	if err := db.Model(&paymentdomain.CandidateEvidence{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("conflicting evidence overwritten")
	}
	// New PID using the same remote ID cannot overwrite or match the old inbox.
	channel.ConfigJSON["pid"] = "2000"
	if err := db.Save(&channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := service().Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&paymentdomain.CandidateReceipt{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("source isolation failed")
	}
	var foreign paymentdomain.CandidateEvidence
	if err := db.Order("id DESC").First(&foreign).Error; err != nil || foreign.PaymentID != nil || foreign.MatchState != "identity_mismatch" {
		t.Fatal("foreign source matched a payment")
	}
	if err := db.First(&payment, payment.ID).Error; err != nil || payment.Status != constants.PaymentStatusPending || payment.PaidAt != nil {
		t.Fatal("candidate changed payment status")
	}
	if acks.Load() != 5 || lists.Load() != 6 {
		t.Fatalf("unexpected delivery counts: lists=%d acks=%d", lists.Load(), acks.Load())
	}
	// Both historical identities still poll after sales-channel removal.
	if err := db.Delete(&channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := service().Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if acks.Load() != 7 || lists.Load() != 8 {
		t.Fatal("deleted channel lost historical subscriptions")
	}

}

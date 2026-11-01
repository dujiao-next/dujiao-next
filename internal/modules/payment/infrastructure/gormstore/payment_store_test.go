package gormstore

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupStoreTest(t *testing.T) (*Store, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:payment_repo_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&orderdomain.Order{},
		&paymentdomain.Payment{},
		&paymentdomain.PaymentChannel{},
		&walletdomain.RechargeOrder{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return New(db, "test-guest-credential-secret-with-32-bytes"), db
}

func TestPaymentStoresExcludeSoftDeletedRecords(t *testing.T) {
	repo, db := setupStoreTest(t)
	channels := NewChannelStore(db)
	now := time.Now().UTC().Truncate(time.Second)
	deletedAt := now.Add(time.Minute)

	activeChannel := paymentdomain.PaymentChannel{
		Name:         "active",
		ProviderType: constants.PaymentProviderOfficial,
		ChannelType:  constants.PaymentChannelTypeAlipay,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	deletedChannel := paymentdomain.PaymentChannel{
		Name:         "deleted",
		ProviderType: constants.PaymentProviderOfficial,
		ChannelType:  constants.PaymentChannelTypeWechat,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
		DeletedAt:    &deletedAt,
	}
	if err := db.Create(&activeChannel).Error; err != nil {
		t.Fatalf("create active channel failed: %v", err)
	}
	if err := db.Create(&deletedChannel).Error; err != nil {
		t.Fatalf("create deleted channel failed: %v", err)
	}

	newPayment := func(channelID uint, deleted *time.Time) paymentdomain.Payment {
		return paymentdomain.Payment{
			OrderID:         channelID,
			ChannelID:       channelID,
			ProviderType:    constants.PaymentProviderOfficial,
			ChannelType:     constants.PaymentChannelTypeAlipay,
			InteractionMode: constants.PaymentInteractionRedirect,
			Amount:          money.FromDecimal(decimal.NewFromInt(100)),
			FeeRate:         money.FromDecimal(decimal.Zero),
			FeeAmount:       money.FromDecimal(decimal.Zero),
			Currency:        "CNY",
			Status:          constants.PaymentStatusPending,
			CreatedAt:       now,
			UpdatedAt:       now,
			DeletedAt:       deleted,
		}
	}
	activePayment := newPayment(activeChannel.ID, nil)
	deletedPayment := newPayment(deletedChannel.ID, &deletedAt)
	if err := db.Create(&activePayment).Error; err != nil {
		t.Fatalf("create active payment failed: %v", err)
	}
	if err := db.Create(&deletedPayment).Error; err != nil {
		t.Fatalf("create deleted payment failed: %v", err)
	}

	if row, err := repo.GetByID(deletedPayment.ID); err != nil || row != nil {
		t.Fatalf("soft-deleted payment must be hidden, row=%+v err=%v", row, err)
	}
	payments, total, err := repo.ListAdmin(paymentcontract.ListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list payments failed: %v", err)
	}
	if total != 1 || len(payments) != 1 || payments[0].ID != activePayment.ID {
		t.Fatalf("expected only active payment, total=%d rows=%+v", total, payments)
	}

	if row, err := channels.GetByID(deletedChannel.ID); err != nil || row != nil {
		t.Fatalf("soft-deleted channel must be hidden, row=%+v err=%v", row, err)
	}
	channelRows, channelTotal, err := channels.List(paymentcontract.ChannelListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list channels failed: %v", err)
	}
	if channelTotal != 1 || len(channelRows) != 1 || channelRows[0].ID != activeChannel.ID {
		t.Fatalf("expected only active channel, total=%d rows=%+v", channelTotal, channelRows)
	}
}

func TestStoreListAdminByUserIncludesWalletRechargePayments(t *testing.T) {
	repo, db := setupStoreTest(t)
	now := time.Now().UTC().Truncate(time.Second)

	user1 := userdomain.User{
		Email:        "payment_repo_user1@example.com",
		PasswordHash: "hash",
		Status:       constants.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	user2 := userdomain.User{
		Email:        "payment_repo_user2@example.com",
		PasswordHash: "hash",
		Status:       constants.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&user1).Error; err != nil {
		t.Fatalf("create user1 failed: %v", err)
	}
	if err := db.Create(&user2).Error; err != nil {
		t.Fatalf("create user2 failed: %v", err)
	}

	order := orderdomain.Order{
		OrderNo:                 "DJPAYREPO001",
		UserID:                  user1.ID,
		Status:                  constants.OrderStatusPendingPayment,
		Currency:                "CNY",
		OriginalAmount:          money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount:          money.FromDecimal(decimal.Zero),
		PromotionDiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:             money.FromDecimal(decimal.NewFromInt(100)),
		WalletPaidAmount:        money.FromDecimal(decimal.Zero),
		OnlinePaidAmount:        money.FromDecimal(decimal.NewFromInt(100)),
		RefundedAmount:          money.FromDecimal(decimal.Zero),
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	orderPayment := paymentdomain.Payment{
		OrderID:         order.ID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeAlipay,
		InteractionMode: constants.PaymentInteractionRedirect,
		Amount:          money.FromDecimal(decimal.NewFromInt(100)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.PaymentStatusSuccess,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&orderPayment).Error; err != nil {
		t.Fatalf("create order payment failed: %v", err)
	}

	rechargePaymentUser1 := paymentdomain.Payment{
		OrderID:         0,
		ChannelID:       2,
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeWechat,
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          money.FromDecimal(decimal.NewFromInt(50)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.PaymentStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&rechargePaymentUser1).Error; err != nil {
		t.Fatalf("create user1 recharge payment failed: %v", err)
	}
	if err := db.Create(&walletdomain.RechargeOrder{
		RechargeNo:      "DJRUSER1001",
		UserID:          user1.ID,
		PaymentID:       rechargePaymentUser1.ID,
		ChannelID:       rechargePaymentUser1.ChannelID,
		ProviderType:    rechargePaymentUser1.ProviderType,
		ChannelType:     rechargePaymentUser1.ChannelType,
		InteractionMode: rechargePaymentUser1.InteractionMode,
		Amount:          money.FromDecimal(decimal.NewFromInt(50)),
		PayableAmount:   money.FromDecimal(decimal.NewFromInt(50)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.WalletRechargeStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("create user1 recharge order failed: %v", err)
	}

	rechargePaymentUser2 := paymentdomain.Payment{
		OrderID:         0,
		ChannelID:       3,
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeWechat,
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          money.FromDecimal(decimal.NewFromInt(60)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.PaymentStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&rechargePaymentUser2).Error; err != nil {
		t.Fatalf("create user2 recharge payment failed: %v", err)
	}
	if err := db.Create(&walletdomain.RechargeOrder{
		RechargeNo:      "DJRUSER2001",
		UserID:          user2.ID,
		PaymentID:       rechargePaymentUser2.ID,
		ChannelID:       rechargePaymentUser2.ChannelID,
		ProviderType:    rechargePaymentUser2.ProviderType,
		ChannelType:     rechargePaymentUser2.ChannelType,
		InteractionMode: rechargePaymentUser2.InteractionMode,
		Amount:          money.FromDecimal(decimal.NewFromInt(60)),
		PayableAmount:   money.FromDecimal(decimal.NewFromInt(60)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "CNY",
		Status:          constants.WalletRechargeStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("create user2 recharge order failed: %v", err)
	}

	rows, total, err := repo.ListAdmin(paymentcontract.ListFilter{
		Page:     1,
		PageSize: 50,
		UserID:   user1.ID,
	})
	if err != nil {
		t.Fatalf("list admin payments failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("total want 2 got %d", total)
	}
	if len(rows) != 2 {
		t.Fatalf("rows len want 2 got %d", len(rows))
	}

	foundOrderPayment := false
	foundUser1Recharge := false
	for _, row := range rows {
		if row.ID == orderPayment.ID {
			foundOrderPayment = true
		}
		if row.ID == rechargePaymentUser1.ID {
			foundUser1Recharge = true
		}
		if row.ID == rechargePaymentUser2.ID {
			t.Fatalf("should not include other user's recharge payment, got payment_id=%d", row.ID)
		}
	}
	if !foundOrderPayment {
		t.Fatalf("missing normal order payment for user")
	}
	if !foundUser1Recharge {
		t.Fatalf("missing wallet recharge payment for user")
	}
}

func TestStoreListAdminLightweightSkipCount(t *testing.T) {
	repo, db := setupStoreTest(t)
	now := time.Now().UTC().Truncate(time.Second)

	user := userdomain.User{
		Email:        "payment_repo_lightweight@example.com",
		PasswordHash: "hash",
		Status:       constants.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	order := orderdomain.Order{
		OrderNo:                 "DJLIGHTWEIGHT001",
		UserID:                  user.ID,
		Status:                  constants.OrderStatusPendingPayment,
		Currency:                "USD",
		OriginalAmount:          money.FromDecimal(decimal.NewFromInt(12)),
		DiscountAmount:          money.FromDecimal(decimal.Zero),
		PromotionDiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:             money.FromDecimal(decimal.NewFromInt(12)),
		WalletPaidAmount:        money.FromDecimal(decimal.Zero),
		OnlinePaidAmount:        money.FromDecimal(decimal.NewFromInt(12)),
		RefundedAmount:          money.FromDecimal(decimal.Zero),
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	payload := jsonmap.JSON{
		"display_channel_type": "usdt.arbitrum",
		"foo":                  "bar",
		"nested":               map[string]interface{}{"key": "value"},
	}
	payment := paymentdomain.Payment{
		OrderID:         order.ID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeStripe,
		InteractionMode: constants.PaymentInteractionRedirect,
		Amount:          money.FromDecimal(decimal.NewFromInt(12)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "USD",
		Status:          constants.PaymentStatusPending,
		ProviderRef:     "pi_lightweight_001",
		ProviderPayload: payload,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	rows, total, err := repo.ListAdmin(paymentcontract.ListFilter{
		Page:        1,
		PageSize:    20,
		SkipCount:   true,
		Lightweight: true,
	})
	if err != nil {
		t.Fatalf("list admin payments failed: %v", err)
	}
	if total != 0 {
		t.Fatalf("total want 0 when skip count got %d", total)
	}
	if len(rows) != 1 {
		t.Fatalf("rows len want 1 got %d", len(rows))
	}
	if rows[0].ID != payment.ID {
		t.Fatalf("payment id mismatch, want %d got %d", payment.ID, rows[0].ID)
	}
	if rows[0].ProviderRef != payment.ProviderRef {
		t.Fatalf("provider ref mismatch, want %s got %s", payment.ProviderRef, rows[0].ProviderRef)
	}
	if len(rows[0].ProviderPayload) != 0 {
		t.Fatalf("provider payload should be empty in lightweight query, got %+v", rows[0].ProviderPayload)
	}
	if rows[0].DisplayChannelType != "usdt.arbitrum" {
		t.Fatalf("display channel type want usdt.arbitrum got %s", rows[0].DisplayChannelType)
	}
}

func TestLatestPaymentRestoreExcludesLegacyFeeLinksAndSupersededRows(t *testing.T) {
	repo, db := setupStoreTest(t)
	now := time.Now().UTC().Truncate(time.Second)
	channel := paymentdomain.PaymentChannel{
		Name: "restore-filter", ProviderType: constants.PaymentProviderOfficial,
		ChannelType: constants.PaymentChannelTypeWechat, InteractionMode: constants.PaymentInteractionQR,
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel failed: %v", err)
	}
	merchant := paymentdomain.Payment{
		OrderID: 88, ChannelID: channel.ID, ProviderType: channel.ProviderType, ChannelType: channel.ChannelType,
		InteractionMode: channel.InteractionMode, Amount: money.FromDecimal(decimal.NewFromInt(100)),
		FeeAmount: money.FromDecimal(decimal.NewFromInt(3)), FeePolicy: constants.PaymentFeePolicyMerchantAbsorbed,
		Currency: "CNY", Status: constants.PaymentStatusPending, QRCode: "merchant-link", CreatedAt: now, UpdatedAt: now,
	}
	legacy := merchant
	legacy.Amount = money.FromDecimal(decimal.NewFromInt(103))
	legacy.FeePolicy = constants.PaymentFeePolicyLegacyCustomerSurcharge
	legacy.QRCode = "legacy-link"
	legacy.CreatedAt = now.Add(time.Second)
	legacy.UpdatedAt = legacy.CreatedAt
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create merchant payment failed: %v", err)
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("create legacy payment failed: %v", err)
	}

	latest, err := repo.GetLatestPendingByOrder(merchant.OrderID, now)
	if err != nil {
		t.Fatalf("get latest restorable payment failed: %v", err)
	}
	if latest == nil || latest.ID != merchant.ID {
		t.Fatalf("legacy fee link must not be restored automatically: %+v", latest)
	}
	explicit, err := repo.GetLatestPendingByOrderChannel(merchant.OrderID, channel.ID, now)
	if err != nil {
		t.Fatalf("get explicit channel payment failed: %v", err)
	}
	if explicit == nil || explicit.ID != legacy.ID {
		t.Fatalf("service must be able to evaluate legacy compatibility: %+v", explicit)
	}

	count, err := repo.SupersedePendingByOrderID(merchant.OrderID, merchant.ID, now.Add(2*time.Second))
	if err != nil || count != 1 {
		t.Fatalf("supersede legacy payment count=%d err=%v", count, err)
	}
	var stored paymentdomain.Payment
	if err := db.First(&stored, legacy.ID).Error; err != nil {
		t.Fatalf("reload superseded payment failed: %v", err)
	}
	if stored.Status != constants.PaymentStatusExpired || stored.SupersededAt == nil || stored.SupersededByPaymentID == nil || *stored.SupersededByPaymentID != merchant.ID {
		t.Fatalf("unexpected superseded payment: %+v", stored)
	}
}

func TestLatestFixedPaymentRestoresPreparingWithoutGatewayPresentation(t *testing.T) {
	repo, db := setupStoreTest(t)
	now := time.Now()
	expiry := now.Add(time.Minute)
	p := paymentdomain.Payment{OrderID: 91, ChannelID: 1, RequiresChainVerification: true, Status: constants.PaymentStatusInitiated, ExpiredAt: &expiry, FeePolicy: constants.PaymentFeePolicyNone}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetLatestPendingByOrder(p.OrderID, now)
	if err != nil || got == nil || got.ID != p.ID {
		t.Fatalf("pending fixed intent lost on reload: %+v %v", got, err)
	}
	if got, err = repo.GetLatestPendingByOrder(p.OrderID, expiry.Add(time.Second)); err != nil || got != nil {
		t.Fatal("expired intent restored")
	}
}

func TestCandidateInboxConcurrentConflictsAndAtomicBatch(t *testing.T) {
	repo, db := setupStoreTest(t)
	if err := db.AutoMigrate(&paymentdomain.CandidateReceipt{}, &paymentdomain.CandidateSubscription{}, &paymentdomain.CandidateEvidence{}); err != nil {
		t.Fatal(err)
	}
	source := paymentdomain.CandidateSource{GatewayURL: "https://gateway.example", PID: "1000"}
	rows := []paymentdomain.TransferCandidate{{ID: "1", OrderID: "DJP-1", TradeID: "trade-1", RawAmount: "100"}, {ID: "1", OrderID: "DJP-1", TradeID: "trade-1", RawAmount: "1"}}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range rows {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = repo.CommitCandidates(context.Background(), source, []paymentdomain.TransferCandidate{rows[i]})
		}(i)
	}
	wg.Wait()
	// SQLite may reject a competing writer. Failed delivery remains unacknowledged
	// and is retried; both accepted versions must survive without overwrite.
	for i, err := range errs {
		if err != nil {
			if err := repo.CommitCandidates(context.Background(), source, []paymentdomain.TransferCandidate{rows[i]}); err != nil {
				t.Fatal(err)
			}
		}
	}
	var receipt paymentdomain.CandidateReceipt
	if err := db.First(&receipt).Error; err != nil || !receipt.Conflicted {
		t.Fatal("concurrent conflict was lost")
	}
	var count int64
	if err := db.Model(&paymentdomain.CandidateEvidence{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("concurrent evidence overwritten")
	}
	batch := []paymentdomain.TransferCandidate{{ID: "2", OrderID: "DJP-2", TradeID: "trade-2"}, {ID: "invalid", OrderID: "DJP-3", TradeID: "trade-3"}}
	if repo.CommitCandidates(context.Background(), source, batch) == nil {
		t.Fatal("invalid batch accepted")
	}
	if err := db.Model(&paymentdomain.CandidateReceipt{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("partial batch committed")
	}
}

func TestFixedIntentRecoveryLeaseAndEligibility(t *testing.T) {
	repo, db := setupStoreTest(t)
	now := time.Now()
	future := now.Add(time.Minute)
	past := now.Add(-time.Minute)
	for i := uint(1); i <= 4; i++ {
		payment := paymentdomain.Payment{ID: i, OrderID: 1, RequiresChainVerification: true, ExpiredAt: &future, IntentRetryAt: &past, Status: constants.PaymentStatusInitiated}
		if i == 2 {
			payment.ExpiredAt = &past
		}
		if i == 3 {
			payment.ProviderRef = "bound"
		}
		if i == 4 {
			payment.SupersededAt = &past
		}
		if err := db.Create(&payment).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows, err := repo.ListIntentRecoveries(now, 20)
	if err != nil || len(rows) != 1 || rows[0].ID != 1 {
		t.Fatalf("invalid recovery selection: %v %#v", err, rows)
	}
	var wg sync.WaitGroup
	wins := make([]bool, 2)
	for i := range wins {
		wg.Add(1)
		go func(i int) { defer wg.Done(); wins[i], _ = repo.ClaimIntentRecovery(1, now, future) }(i)
	}
	wg.Wait()
	if wins[0] == wins[1] {
		t.Fatalf("lease must have one winner: %v", wins)
	}
	if rows, err := repo.ListIntentRecoveries(now, 20); err != nil || len(rows) != 0 {
		t.Fatal("live lease was selected again")
	}
}

func TestChainCheckMigrationPreservesReceivedEvidence(t *testing.T) {
	_, db := setupStoreTest(t)
	if err := db.Exec(`CREATE TABLE payment_candidate_evidence (id integer PRIMARY KEY, receipt_id integer NOT NULL, digest text NOT NULL, payload text NOT NULL, payment_id integer, match_state text NOT NULL, received_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO payment_candidate_evidence (id,receipt_id,digest,payload,match_state) VALUES (1,1,'retained-digest','{}','awaiting_chain_check')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&paymentdomain.CandidateEvidence{}); err != nil {
		t.Fatal(err)
	}
	var row paymentdomain.CandidateEvidence
	if err := db.First(&row).Error; err != nil || row.Digest != "retained-digest" || row.Payload != "{}" || row.CheckState != "pending" || row.ChainProof != "" {
		t.Fatalf("old evidence migration failed: %v %#v", err, row)
	}
}

package settlement_test

import (
	"context"
	"errors"
	nc "github.com/dujiao-next/internal/modules/notification/contract"
	"path/filepath"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	aa "github.com/dujiao-next/internal/modules/affiliate/application"
	ad "github.com/dujiao-next/internal/modules/affiliate/domain"
	as "github.com/dujiao-next/internal/modules/affiliate/infrastructure/gormstore"
	cs "github.com/dujiao-next/internal/modules/cardsecret/domain"
	cd "github.com/dujiao-next/internal/modules/catalog/product/domain"
	cp "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	fa "github.com/dujiao-next/internal/modules/fulfillment/application"
	fd "github.com/dujiao-next/internal/modules/fulfillment/domain"
	fs "github.com/dujiao-next/internal/modules/fulfillment/infrastructure/gormstore"
	ud "github.com/dujiao-next/internal/modules/identity/user/domain"
	us "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	oc "github.com/dujiao-next/internal/modules/order/contract"
	od "github.com/dujiao-next/internal/modules/order/domain"
	os "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	app "github.com/dujiao-next/internal/modules/payment/application"
	"github.com/dujiao-next/internal/modules/payment/application/settlement"
	pc "github.com/dujiao-next/internal/modules/payment/contract"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	si "github.com/dujiao-next/internal/modules/settings/schema/integration"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"gorm.io/gorm"
)

type fulfillQueue struct {
	pc.Queue
	calls int
}

func (q *fulfillQueue) Enabled() bool                              { return true }
func (q *fulfillQueue) EnqueueOrderAutoFulfill(uint) error         { q.calls++; return nil }
func (q *fulfillQueue) EnqueueVerifiedOrderAutoFulfill(uint) error { q.calls++; return nil }
func TestPaymentDeliveryOutboxWaitsForActualGoodsAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "merchant.db")
	db := openDB(t, path)
	must(t, db.AutoMigrate(&cs.Secret{}, &cs.Batch{}))
	p, _ := seed(t, db, 1, 1)
	evidence(t, db, p, "1", "hash")
	svc, _, repo := services(db, &verifier{})
	must(t, svc.Run(context.Background()))
	card := cs.Secret{ProductID: 1, SKUID: 1, Secret: "synthetic-card", Status: cs.StatusAvailable}
	must(t, db.Create(&card).Error)
	// Isolate the delivery responsibility; other effects have separate retry rows.
	must(t, db.Model(&pd.PaymentEffect{}).Where("kind <> ?", "auto_fulfill").Update("state", "done").Error)
	q := &fulfillQueue{}
	makeRunner := func() *settlement.Effects {
		_, _, repo = services(db, &verifier{})
		application := app.NewPaymentService(app.PaymentServiceOptions{OrderStore: os.New(db, "test-guest-key"), PaymentStore: repo, Queue: q})
		return &settlement.Effects{Store: repo, Handler: application}
	}
	runner := makeRunner()
	must(t, runner.Run(context.Background()))
	if q.calls != 1 {
		t.Fatal("not enqueued")
	}
	var work pd.PaymentEffect
	must(t, db.Where("kind = ?", "auto_fulfill").First(&work).Error)
	if work.State != "pending" {
		t.Fatal("enqueue prematurely acknowledged delivery")
	}
	// Lose the queue and restart the SQL process; no delivery record exists yet.
	conn, _ := db.DB()
	must(t, conn.Close())
	db = openDB(t, path)
	must(t, db.Model(&pd.PaymentEffect{}).Where("id = ?", work.ID).Update("after", time.Now().Add(-time.Minute)).Error)
	runner = makeRunner()
	must(t, runner.Run(context.Background()))
	if q.calls != 2 {
		t.Fatal("SQL did not recover lost Redis task")
	}
	delivery := fa.New(fa.Options{OrderStore: os.New(db, "test-guest-key"), FulfillmentStore: fs.New(db)})
	if _, err := delivery.CreateAuto(p.OrderID); err != nil {
		t.Fatal(err)
	}
	// Replayed task after lost worker acknowledgment must not consume more goods.
	if _, err := delivery.CreateAuto(p.OrderID); err == nil {
		t.Fatal("duplicate delivery allowed")
	}
	must(t, db.Model(&pd.PaymentEffect{}).Where("id = ?", work.ID).Update("after", time.Now().Add(-time.Minute)).Error)
	must(t, runner.Run(context.Background()))
	work = pd.PaymentEffect{}
	must(t, db.Where("kind = ?", "auto_fulfill").First(&work).Error)
	if work.State != "done" || count(t, db, &fd.Fulfillment{}) != 1 {
		t.Fatal("delivery not durably acknowledged")
	}
	var used int64
	must(t, db.Model(&cs.Secret{}).Where("status = ?", cs.StatusUsed).Count(&used).Error)
	if used != 1 {
		t.Fatal("duplicate card use")
	}
}

type staleOrderStore struct {
	oc.Store
	db *gorm.DB
}

func (s staleOrderStore) GetByID(id uint) (*od.Order, error) {
	o, err := s.Store.GetByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.db.Model(&od.Order{}).Where("id = ?", id).Update("status", constants.OrderStatusCanceled).Error; err != nil {
		return nil, err
	}
	return o, nil
}
func TestPaymentDeliveryDoesNotUsePreCancellationRead(t *testing.T) {
	db := openDB(t, filepath.Join(t.TempDir(), "merchant.db"))
	must(t, db.AutoMigrate(&cs.Secret{}, &cs.Batch{}))
	p, _ := seed(t, db, 1, 1)
	evidence(t, db, p, "1", "hash")
	svc, _, _ := services(db, &verifier{})
	must(t, svc.Run(context.Background()))
	card := cs.Secret{ProductID: 1, SKUID: 1, Secret: "synthetic-card", Status: cs.StatusAvailable}
	must(t, db.Create(&card).Error)
	delivery := fa.New(fa.Options{OrderStore: staleOrderStore{os.New(db, "test-guest-key"), db}, FulfillmentStore: fs.New(db)})
	if _, err := delivery.CreateAuto(p.OrderID); err == nil {
		t.Fatal("stale order delivered")
	}
	if count(t, db, &fd.Fulfillment{}) != 0 {
		t.Fatal("created delivery")
	}
	var stored cs.Secret
	must(t, db.First(&stored, card.ID).Error)
	if stored.Status != cs.StatusAvailable {
		t.Fatal("consumed card after cancellation")
	}
}

type affiliateSettings struct{}

func (affiliateSettings) GetAffiliateSetting() (si.AffiliateSetting, error) {
	return si.AffiliateSetting{Enabled: true, CommissionRate: 10}, nil
}
func TestPaymentAffiliateEffectCommitsWithAcknowledgment(t *testing.T) {
	db := openDB(t, filepath.Join(t.TempDir(), "merchant.db"))
	must(t, db.AutoMigrate(&ad.Profile{}, &ad.Commission{}, &ad.WithdrawRequest{}))
	p, _ := seed(t, db, 1, 1)
	svc, _, repo := services(db, &verifier{})
	ref := ud.User{Email: "affiliate@example.com", PasswordHash: "test", Status: "active"}
	must(t, db.Create(&ref).Error)
	profile := ad.Profile{UserID: ref.ID, AffiliateCode: "testcode", Status: constants.AffiliateProfileStatusActive}
	must(t, db.Create(&profile).Error)
	must(t, db.Model(&od.Order{}).Where("id = ?", 1).Update("affiliate_profile_id", profile.ID).Error)
	product := cd.Product{ID: 1, TitleJSON: jsonmap.JSON{"en": "test"}, Slug: "test", IsAffiliateEnabled: true}
	must(t, db.Create(&product).Error)
	var authorizedOrder od.Order
	must(t, db.Preload("Items").First(&authorizedOrder, 1).Error)
	p.SettlementBasis = pd.OrderBasis(p, authorizedOrder)
	must(t, db.Save(&p).Error)
	must(t, db.Model(&pd.Allocation{}).Where("payment_id = ?", p.ID).Update("snapshot", pd.SettlementSnapshot(p, authorizedOrder)).Error)
	evidence(t, db, p, "1", "hash")
	must(t, svc.Run(context.Background()))
	orders := os.New(db, "test-guest-key")
	affiliate := aa.NewService(as.New(db), us.New(db), orders, cp.NewProductStore(db), affiliateSettings{})
	application := app.NewPaymentService(app.PaymentServiceOptions{OrderStore: orders, PaymentStore: repo, AffiliateService: affiliate})
	must(t, db.Model(&pd.PaymentEffect{}).Where("kind <> ?", "affiliate").Update("state", "done").Error)
	rows, err := repo.LeasePaymentEffects(context.Background(), time.Now())
	must(t, err)
	if len(rows) != 1 {
		t.Fatal("missing affiliate lease")
	}
	w := rows[0]
	must(t, db.Exec("CREATE TRIGGER reject_commission BEFORE INSERT ON affiliate_commissions BEGIN SELECT RAISE(ABORT, 'test failure'); END").Error)
	if _, err := application.HandlePaymentEffect(context.Background(), w); err == nil {
		t.Fatal("commission failure ignored")
	}
	var current pd.PaymentEffect
	must(t, db.First(&current, w.ID).Error)
	if current.State != "working" {
		t.Fatal("failed commission acknowledged")
	}
	must(t, db.Exec("DROP TRIGGER reject_commission").Error)
	if _, err := application.HandlePaymentEffect(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	// Simulate crash after SQL completion but before the outer dispatcher ACK.
	if _, err := application.HandlePaymentEffect(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	if count(t, db, &ad.Commission{}) != 1 {
		t.Fatal("duplicate commission")
	}
	current = pd.PaymentEffect{}
	must(t, db.First(&current, w.ID).Error)
	if current.State != "done" {
		t.Fatal("commission and ack not atomic")
	}
	var commission ad.Commission
	must(t, db.First(&commission).Error)
	if commission.CommissionAmount.String() != "11.00" {
		t.Fatal("wrong commission")
	}
}

type synchronousNotification struct {
	calls   int
	failure error
}

func (*synchronousNotification) Enqueue(nc.EnqueueInput) error {
	panic("durable effect entered Redis notification queue")
}
func (s *synchronousNotification) DispatchRecipients(ctx context.Context, _ nc.EnqueueInput, batch nc.DeliverBatch) error {
	return batch(ctx, []nc.RecipientDelivery{{Key: nc.RecipientKey("synthetic"), Send: func(context.Context) error { s.calls++; return s.failure }}})
}

func TestNotificationOutboxWaitsForSenderAcrossRestartAndRejectsStaleLease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notification.db")
	db := openDB(t, path)
	p, _ := seed(t, db, 1, 1)
	evidence(t, db, p, "1", "hash")
	svc, _, _ := services(db, &verifier{})
	must(t, svc.Run(context.Background()))
	must(t, db.Model(&pd.PaymentEffect{}).Where("kind <> ?", "paid_notification").Update("state", "done").Error)
	sender := &synchronousNotification{failure: errors.New("synthetic send failure")}
	makeRunner := func() (*settlement.Effects, *app.PaymentService, pc.EffectStore) {
		_, _, repo := services(db, &verifier{})
		application := app.NewPaymentService(app.PaymentServiceOptions{PaymentStore: repo, OrderStore: os.New(db, "test-guest-key"), NotificationService: sender})
		return &settlement.Effects{Store: repo, Handler: application}, application, repo
	}
	runner, _, _ := makeRunner()
	if err := runner.Run(context.Background()); err == nil {
		t.Fatal("sender failure acknowledged")
	}
	var w pd.PaymentEffect
	must(t, db.Where("kind = ?", "paid_notification").First(&w).Error)
	if w.State != "pending" || w.DispatchedAt == nil || w.CompletedAt != nil || sender.calls != 1 {
		t.Fatal("sender failure lost retry state")
	}
	conn, _ := db.DB()
	must(t, conn.Close())
	db = openDB(t, path)
	must(t, db.Model(&pd.PaymentEffect{}).Where("id = ?", w.ID).Update("after", time.Now().Add(-time.Minute)).Error)
	_, application, repo := makeRunner()
	old, err := repo.LeasePaymentEffects(context.Background(), time.Now())
	must(t, err)
	must(t, db.Model(&pd.PaymentEffect{}).Where("id = ?", w.ID).Update("after", time.Now().Add(-time.Minute)).Error)
	current, err := repo.LeasePaymentEffects(context.Background(), time.Now())
	must(t, err)
	if _, err := application.HandlePaymentEffect(context.Background(), old[0]); err == nil || sender.calls != 1 {
		t.Fatal("stale lease sent a notification")
	}
	sender.failure = nil
	if _, err := application.HandlePaymentEffect(context.Background(), current[0]); err != nil {
		t.Fatal(err)
	}
	// Recipient SQL ACK committed before parent completion: replay now skips this
	// destination. A crash before recipient ACK is tested separately.
	must(t, db.Model(&pd.PaymentEffect{}).Where("id = ?", w.ID).Update("after", time.Now().Add(-time.Minute)).Error)
	runner, _, _ = makeRunner()
	must(t, runner.Run(context.Background()))
	w = pd.PaymentEffect{}
	must(t, db.Where("kind = ?", "paid_notification").First(&w).Error)
	if sender.calls != 2 || w.State != "done" || w.CompletedAt == nil {
		t.Fatal("sender completion did not persist")
	}
}

func TestNotificationPermanentRejectionAndRefundDoNotAutoRetry(t *testing.T) {
	for _, refunded := range []bool{false, true} {
		db := openDB(t, filepath.Join(t.TempDir(), "notification-review.db"))
		p, order := seed(t, db, 1, 1)
		evidence(t, db, p, "1", "hash")
		svc, _, repo := services(db, &verifier{})
		must(t, svc.Run(context.Background()))
		must(t, db.Model(&pd.PaymentEffect{}).Where("kind <> ?", "paid_notification").Update("state", "done").Error)
		if refunded {
			must(t, db.Model(&order).Update("status", constants.OrderStatusRefunded).Error)
		}
		sender := &synchronousNotification{failure: pc.ErrEffectPermanent}
		handler := app.NewPaymentService(app.PaymentServiceOptions{PaymentStore: repo, OrderStore: os.New(db, "test-guest-key"), NotificationService: sender})
		runner := &settlement.Effects{Store: repo, Handler: handler}
		must(t, runner.Run(context.Background()))
		var work pd.PaymentEffect
		must(t, db.Where("kind = ?", "paid_notification").First(&work).Error)
		expectedCalls := 1
		if refunded {
			expectedCalls = 0
		}
		if work.State != "manual" || work.CompletedAt != nil || sender.calls != expectedCalls {
			t.Fatal("rejection or obsolete success notification incorrectly acknowledged")
		}
		must(t, runner.Run(context.Background()))
		if sender.calls != expectedCalls {
			t.Fatal("permanent rejection retried automatically")
		}
	}
}

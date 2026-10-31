package settlement_test

import (
	"context"
	"errors"
	"github.com/dujiao-next/internal/constants"
	ud "github.com/dujiao-next/internal/modules/identity/user/domain"
	ma "github.com/dujiao-next/internal/modules/memberlevel/application"
	mc "github.com/dujiao-next/internal/modules/memberlevel/contract"
	md "github.com/dujiao-next/internal/modules/memberlevel/domain"
	ms "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	na "github.com/dujiao-next/internal/modules/notification/application"
	nc "github.com/dujiao-next/internal/modules/notification/contract"
	"github.com/dujiao-next/internal/modules/payment/application/settlement"
	pc "github.com/dujiao-next/internal/modules/payment/contract"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	store "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	sm "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	ss "github.com/dujiao-next/internal/modules/settings/schema/storefront"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"path/filepath"
	"testing"
	"time"
)

type recipientSettings struct{ setting sm.NotificationCenterSetting }

func (s *recipientSettings) GetNotificationCenterSetting() (sm.NotificationCenterSetting, error) {
	return s.setting, nil
}
func (*recipientSettings) GetDashboardSetting() (ss.DashboardSetting, error) {
	return ss.DashboardSetting{}, nil
}

type recipientEmail struct {
	calls map[string]int
	fail  string
}

func (s *recipientEmail) SendCustomEmail(to, _, _ string) error {
	s.calls[to]++
	if to == s.fail {
		return errors.New("synthetic refusal")
	}
	return nil
}

func TestRecipientReceiptsSurvivePartialFailureRestartAndConfigChange(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "recipients.db")
	db := openDB(t, path)
	repo := store.New(db, "test-only")
	w := pd.PaymentEffect{PaymentID: 1, OrderID: 1, Kind: "paid_notification", State: "pending", CreatedAt: time.Now()}
	must(t, db.Create(&w).Error)
	lease := func() pd.PaymentEffect {
		must(t, db.Model(&pd.PaymentEffect{}).Where("id = ?", w.ID).Update("after", time.Now().Add(-time.Minute)).Error)
		rows, err := repo.LeasePaymentEffects(ctx, time.Now())
		must(t, err)
		if len(rows) != 1 {
			t.Fatal("lease missing")
		}
		return rows[0]
	}
	settings := &recipientSettings{}
	settings.setting.Scenes.OrderPaidSuccess = true
	settings.setting.Channels.Email.Enabled = true
	settings.setting.Channels.Email.Recipients = []string{"first@example.test", "second@example.test", "first@example.test"}
	email := &recipientEmail{calls: map[string]int{}, fail: "second@example.test"}
	notifications := na.NewService(settings, email, nil, nil, nil, nil, nil)
	input := nc.EnqueueInput{EventType: constants.NotificationEventOrderPaidSuccess, BizID: 1}
	w = lease()
	if err := notifications.DispatchRecipients(ctx, input, settlement.RecipientBatch(repo, w)); err == nil {
		t.Fatal("partial failure acknowledged")
	}
	if email.calls["first@example.test"] != 1 || email.calls["second@example.test"] != 1 {
		t.Fatal("duplicate recipient or missing send")
	}
	conn, _ := db.DB()
	must(t, conn.Close())
	db = openDB(t, path)
	repo = store.New(db, "test-only")
	w = lease()
	email.fail = ""
	must(t, notifications.DispatchRecipients(ctx, input, settlement.RecipientBatch(repo, w)))
	if email.calls["first@example.test"] != 1 || email.calls["second@example.test"] != 2 {
		t.Fatal("confirmed destination was repeated")
	}
	// Crash after recipient SQL ACK, before parent effect ACK.
	w = lease()
	must(t, notifications.DispatchRecipients(ctx, input, settlement.RecipientBatch(repo, w)))
	if email.calls["first@example.test"] != 1 || email.calls["second@example.test"] != 2 {
		t.Fatal("parent replay repeated receipts")
	}
	settings.setting.Channels.Email.Recipients = []string{"new@example.test"}
	if err := notifications.DispatchRecipients(ctx, input, settlement.RecipientBatch(repo, w)); !errors.Is(err, pc.ErrEffectPermanent) {
		t.Fatal("changed destination set silently accepted")
	}
	if email.calls["new@example.test"] != 0 {
		t.Fatal("retry rerouted to a new recipient")
	}
}

func TestRecipientFreezeConcurrentAdmissionAndLostACK(t *testing.T) {
	ctx := context.Background()
	db := openDB(t, filepath.Join(t.TempDir(), "admission.db"))
	repo := store.New(db, "test-only")
	w := pd.PaymentEffect{PaymentID: 1, OrderID: 1, Kind: "paid_notification", State: "pending"}
	must(t, db.Create(&w).Error)
	lease := func() pd.PaymentEffect {
		must(t, db.Model(&pd.PaymentEffect{}).Where("id = ?", w.ID).Update("after", time.Now().Add(-time.Minute)).Error)
		rows, err := repo.LeasePaymentEffects(ctx, time.Now())
		must(t, err)
		return rows[0]
	}
	w = lease()
	key := nc.RecipientKey("synthetic")
	started, release := make(chan struct{}), make(chan struct{})
	result := make(chan error, 1)
	delivery := nc.RecipientDelivery{Key: key, Send: func(context.Context) error { close(started); <-release; return nil }}
	go func() { result <- settlement.RecipientBatch(repo, w)(ctx, []nc.RecipientDelivery{delivery}) }()
	<-started
	if err := settlement.RecipientBatch(repo, w)(ctx, []nc.RecipientDelivery{delivery}); err == nil {
		t.Fatal("concurrent sender admitted twice")
	}
	must(t, automation.SetFrozen(ctx, db, true, 2, "synthetic freeze"))
	close(release)
	must(t, <-result)
	var receipt pd.RecipientReceipt
	must(t, db.First(&receipt).Error)
	if receipt.State != "done" {
		t.Fatal("freeze discarded prior send ACK")
	}
	if err := settlement.RecipientBatch(repo, w)(ctx, []nc.RecipientDelivery{delivery}); !errors.Is(err, pd.ErrAutomationFrozen) {
		t.Fatal("frozen send admitted")
	}
	must(t, automation.SetFrozen(ctx, db, false, 3, "synthetic resume"))
	// New effect: real SQL acknowledgment failure leaves remote success uncertain.
	w = pd.PaymentEffect{PaymentID: 2, OrderID: 2, Kind: "paid_notification", State: "pending"}
	must(t, db.Create(&w).Error)
	must(t, db.Model(&pd.PaymentEffect{}).Where("payment_id = 1").Update("state", "done").Error)
	w = lease()
	must(t, db.Exec("CREATE TRIGGER reject_receipt_ack BEFORE UPDATE ON payment_recipient_receipts WHEN NEW.state = 'done' BEGIN SELECT RAISE(ABORT, 'synthetic ACK failure'); END").Error)
	sends := 0
	d := nc.RecipientDelivery{Key: key, Send: func(context.Context) error { sends++; return nil }}
	if err := settlement.RecipientBatch(repo, w)(ctx, []nc.RecipientDelivery{d}); err == nil {
		t.Fatal("lost ACK hidden")
	}
	must(t, db.Exec("DROP TRIGGER reject_receipt_ack").Error)
	w = lease()
	must(t, settlement.RecipientBatch(repo, w)(ctx, []nc.RecipientDelivery{d}))
	if sends != 2 {
		t.Fatal("uncertain remote outcome did not retry")
	}
}

type hookedLevels struct {
	mc.LevelRepository
	hook func()
}

func (s *hookedLevels) ListAllActive() ([]md.MemberLevel, error) {
	rows, err := s.LevelRepository.ListAllActive()
	if s.hook != nil {
		s.hook()
	}
	return rows, err
}

func TestMemberLevelAndEffectACKCommitTogetherAndRespectFreezeRevision(t *testing.T) {
	ctx := context.Background()
	db := openDB(t, filepath.Join(t.TempDir(), "member.db"))
	must(t, db.AutoMigrate(&md.MemberLevel{}))
	p, o := seed(t, db, 1, 1)
	evidence(t, db, p, "1", "hash")
	svc, application, repo := services(db, &verifier{})
	must(t, svc.Run(ctx))
	must(t, db.Model(&pd.PaymentEffect{}).Where("kind <> ?", "member_level").Update("state", "done").Error)
	level := md.MemberLevel{NameJSON: jsonmap.JSON{"en": "synthetic"}, Slug: "synthetic", SortOrder: 1, IsActive: true, SpendThreshold: money.FromDecimal(decimal.NewFromInt(10))}
	must(t, db.Create(&level).Error)
	levels := &hookedLevels{LevelRepository: ms.NewLevelStore(db)}
	application.SetMemberLevelService(ma.NewService(levels, ms.NewPriceStore(db), ms.NewUserStore(db)))
	lease := func() pd.PaymentEffect {
		must(t, db.Model(&pd.PaymentEffect{}).Where("kind = ?", "member_level").Update("after", time.Now().Add(-time.Minute)).Error)
		rows, err := repo.LeasePaymentEffects(ctx, time.Now())
		must(t, err)
		return rows[0]
	}
	w := lease()
	var before ud.User
	must(t, db.First(&before, o.UserID).Error)
	levels.hook = func() {
		must(t, automation.SetFrozen(ctx, db, true, 2, "synthetic freeze"))
		must(t, automation.SetFrozen(ctx, db, false, 3, "synthetic resume"))
	}
	if _, err := application.HandlePaymentEffect(ctx, w); !errors.Is(err, pd.ErrAutomationFrozen) {
		t.Fatalf("stale upgrade applied: %v", err)
	}
	levels.hook = nil
	w = lease()
	must(t, db.Exec("CREATE TRIGGER reject_member BEFORE UPDATE OF member_level_id ON users BEGIN SELECT RAISE(ABORT, 'synthetic member failure'); END").Error)
	if _, err := application.HandlePaymentEffect(ctx, w); err == nil {
		t.Fatal("failed member update acknowledged")
	}
	var effect pd.PaymentEffect
	must(t, db.First(&effect, w.ID).Error)
	if effect.State != "working" {
		t.Fatal("ACK committed without level")
	}
	must(t, db.Exec("DROP TRIGGER reject_member").Error)
	if _, err := application.HandlePaymentEffect(ctx, w); err != nil {
		t.Fatal(err)
	}
	var after ud.User
	must(t, db.First(&after, o.UserID).Error)
	effect = pd.PaymentEffect{}
	must(t, db.First(&effect, w.ID).Error)
	if after.MemberLevelID != level.ID || !after.TotalSpent.Decimal.Equal(before.TotalSpent.Decimal) || effect.State != "done" {
		t.Fatal("upgrade/ACK/spend invariant failed")
	}
	_, _ = application.HandlePaymentEffect(ctx, w)
	must(t, db.First(&after, o.UserID).Error)
	if !after.TotalSpent.Decimal.Equal(before.TotalSpent.Decimal) {
		t.Fatal("replay incremented spend")
	}
}

func TestObsoleteMemberEffectSkipsUpgradeAfterRefundStatus(t *testing.T) {
	ctx := context.Background()
	db := openDB(t, filepath.Join(t.TempDir(), "member-refund.db"))
	must(t, db.AutoMigrate(&md.MemberLevel{}))
	p, o := seed(t, db, 1, 1)
	evidence(t, db, p, "1", "hash")
	svc, application, repo := services(db, &verifier{})
	must(t, svc.Run(ctx))
	must(t, db.Model(&pd.PaymentEffect{}).Where("kind <> ?", "member_level").Update("state", "done").Error)
	level := md.MemberLevel{NameJSON: jsonmap.JSON{"en": "synthetic"}, Slug: "synthetic", SortOrder: 1, IsActive: true, SpendThreshold: money.FromDecimal(decimal.NewFromInt(10))}
	must(t, db.Create(&level).Error)
	application.SetMemberLevelService(ma.NewService(ms.NewLevelStore(db), ms.NewPriceStore(db), ms.NewUserStore(db)))
	must(t, db.Model(&o).Update("status", constants.OrderStatusRefunded).Error)
	rows, err := repo.LeasePaymentEffects(ctx, time.Now())
	must(t, err)
	if _, err := application.HandlePaymentEffect(ctx, rows[0]); err != nil {
		t.Fatal(err)
	}
	var user ud.User
	must(t, db.First(&user, o.UserID).Error)
	if user.MemberLevelID != 0 {
		t.Fatal("obsolete payment upgraded refunded user")
	}
}

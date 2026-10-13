package gormstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/support/application"
	"github.com/dujiao-next/internal/modules/support/contract"
	"github.com/dujiao-next/internal/modules/support/domain"
)

type box struct{}

func (box) Encrypt(s string) (string, error) { return "encrypted:" + s, nil }
func (box) Decrypt(s string) (string, error) { return strings.TrimPrefix(s, "encrypted:"), nil }

type push struct {
	calls   int
	content string
	fail    bool
}

func (p *push) Send(ctx context.Context, token, uid, summary, content, url string) (contract.PushResult, error) {
	p.calls++
	p.content = content
	if p.fail {
		return contract.PushResult{}, errors.New("offline")
	}
	return contract.PushResult{SendRecordID: 100}, nil
}
func TestBindingRequiresPossessionAndConfigurationIsMasked(t *testing.T) {
	s := service(t)
	ctx := context.Background()
	p := &push{}
	n := application.NewNotifications(s, p, box{})
	if err := n.SaveSettings(ctx, application.NotificationSettings{Enabled: true, AppToken: "AT_test", BaseURL: "https://shop.example/admin"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := n.Settings(ctx)
	if err != nil || cfg.AppToken != "********" {
		t.Fatal("token exposed")
	}
	if err = n.BeginBinding(ctx, 1, "UID_test"); err != nil {
		t.Fatal(err)
	}
	b, _ := n.Binding(ctx, 1)
	if b.Enabled {
		t.Fatal("unverified binding enabled")
	}
	if err = n.ConfirmBinding(ctx, 1, "incorrect"); !errors.Is(err, contract.ErrForbidden) {
		t.Fatal("invalid verification accepted")
	}
	b, _ = n.Binding(ctx, 1)
	if b.PendingHash != "" {
		t.Fatal("failed verification remains reusable")
	}
}
func TestDurableReminderIsPrivateAndReadSuppresses(t *testing.T) {
	s := service(t)
	ctx := context.Background()
	p := &push{}
	n := application.NewNotifications(s, p, box{})
	if err := n.SaveSettings(ctx, application.NotificationSettings{Enabled: true, AppToken: "AT_test", BaseURL: "https://shop.example/admin"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.SaveBinding(ctx, &domain.Binding{AdminID: 1, UID: "UID_test", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	c, _, err := s.Create(ctx, contract.Actor{UserID: 6}, application.CreateInput{Content: "secret-card-content", ClientID: "first"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	events, err := s.Store.DueReminders(ctx, time.Now().Add(time.Hour))
	if err != nil || len(events) != 1 {
		t.Fatal("missing reminder")
	}
	if err = s.Store.RetryReminder(ctx, events[0].ID, time.Now().Add(-time.Second), ""); err != nil {
		t.Fatal(err)
	}
	n.Process(ctx)
	if p.calls != 1 || strings.Contains(p.content, "secret-card-content") {
		t.Fatal("push missing or leaked message")
	}
	delivery, err := s.Store.Delivery(ctx, events[0].ID, 1)
	if err != nil || delivery.Status != "accepted" || delivery.SendRecordID != 100 {
		t.Fatal("acceptance not recorded")
	}
	msg, err := s.Append(ctx, contract.Actor{UserID: 6}, c.ID, application.MessageInput{Content: "follow-up", ClientID: "second"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Claim(ctx, contract.Actor{AdminID: 1}, c.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Read(ctx, contract.Actor{AdminID: 1}, c.ID, msg.ID); err != nil {
		t.Fatal(err)
	}
	events, _ = s.Store.DueReminders(ctx, time.Now().Add(time.Hour))
	for _, event := range events {
		_ = s.Store.RetryReminder(ctx, event.ID, time.Now().Add(-time.Second), "")
	}
	n.Process(ctx)
	if p.calls != 1 {
		t.Fatal("read reminder sent")
	}
}

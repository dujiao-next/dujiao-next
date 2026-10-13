package gormstore_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/dujiao-next/internal/modules/support/application"
	"github.com/dujiao-next/internal/modules/support/contract"
	"github.com/dujiao-next/internal/modules/support/domain"
	"github.com/dujiao-next/internal/modules/support/infrastructure/gormstore"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

type agents struct{}

func (agents) Eligible(context.Context, uint) bool            { return true }
func (agents) List(context.Context) ([]contract.Agent, error) { return nil, nil }

type orders struct{}

func (orders) Verify(context.Context, contract.Actor, string, string, string) (*contract.OrderSummary, error) {
	return nil, contract.ErrForbidden
}
func service(t *testing.T) *application.Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&domain.Conversation{}, &domain.Message{}, &domain.Attachment{}, &domain.Audit{}, &domain.Reminder{}, &domain.NotificationConfig{}, &domain.Binding{}, &domain.Delivery{}, &domain.ScanSession{}); err != nil {
		t.Fatal(err)
	}
	return application.New(gormstore.New(db), orders{}, agents{})
}
func TestGuestAndTenantIsolation(t *testing.T) {
	s := service(t)
	ctx := context.Background()
	c, token, err := s.Create(ctx, contract.Actor{TenantID: 2}, application.CreateInput{Content: "hello", ClientID: "first"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 64 {
		t.Fatal("weak token")
	}
	for _, a := range []contract.Actor{{TenantID: 1, GuestToken: token}, {TenantID: 2, GuestToken: "wrong"}, {TenantID: 2, UserID: 3}} {
		if _, err = s.Get(ctx, a, c.ID); !errors.Is(err, contract.ErrNotFound) {
			t.Fatal("cross actor access")
		}
	}
	if _, err = s.Get(ctx, contract.Actor{TenantID: 2, GuestToken: token}, c.ID); err != nil {
		t.Fatal(err)
	}
}
func TestInternalMessagesAndClaim(t *testing.T) {
	s := service(t)
	ctx := context.Background()
	u := contract.Actor{UserID: 5}
	c, _, err := s.Create(ctx, u, application.CreateInput{Content: "question", ClientID: "first"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	a := contract.Actor{AdminID: 1}
	if err = s.Claim(ctx, a, c.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Claim(ctx, contract.Actor{AdminID: 2, Supervisor: true}, c.ID); !errors.Is(err, contract.ErrConflict) {
		t.Fatalf("claim did not conflict: %v", err)
	}
	in := application.MessageInput{Content: "private", ClientID: "note", Internal: true}
	m, err := s.Append(ctx, a, c.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Append(ctx, a, c.ID, in)
	if err != nil || m.ID != again.ID {
		t.Fatal("idempotency failed")
	}
	visible, err := s.Messages(ctx, u, c.ID, 0, 100)
	if err != nil || len(visible) != 1 {
		t.Fatalf("internal leak: %+v %v", visible, err)
	}
	if _, err = s.Append(ctx, u, c.ID, in); !errors.Is(err, contract.ErrInvalid) {
		t.Fatal("customer could write private note")
	}
	if _, err = s.Get(ctx, contract.Actor{AdminID: 2}, c.ID); !errors.Is(err, contract.ErrNotFound) {
		t.Fatal("another agent could read assigned conversation")
	}
	reminders, err := s.Store.DueReminders(ctx, time.Now().Add(time.Hour))
	if err != nil || len(reminders) != 1 {
		t.Fatal("missing durable event")
	}
}
func TestOrderAndAttachmentGuards(t *testing.T) {
	s := service(t)
	ctx := context.Background()
	u := contract.Actor{UserID: 6}
	if _, _, err := s.Create(ctx, u, application.CreateInput{Content: "help", ClientID: "bad-order", OrderNo: "someone-elses"}, "", ""); !errors.Is(err, contract.ErrForbidden) {
		t.Fatal("unverified order accepted")
	}
	c, _, err := s.Create(ctx, u, application.CreateInput{Content: "help", ClientID: "first"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Upload(ctx, u, c.ID, "image/svg+xml", []byte("<svg/>")); !errors.Is(err, contract.ErrInvalid) {
		t.Fatal("unsafe MIME accepted")
	}
	if _, err = s.Upload(ctx, contract.Actor{UserID: 7}, c.ID, "image/png", []byte("x")); !errors.Is(err, contract.ErrNotFound) {
		t.Fatal("cross user upload allowed")
	}
}

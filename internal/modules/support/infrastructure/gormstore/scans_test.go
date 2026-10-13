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

type qrPush struct {
	push
	queries int
	uid     string
	extra   string
}

func (p *qrPush) CreateQRCode(ctx context.Context, token, extra string, ttl int) (contract.ParameterQRCode, error) {
	p.extra = extra
	if ttl != 300 {
		return contract.ParameterQRCode{}, errors.New("wrong expiry")
	}
	return contract.ParameterQRCode{Code: "private-provider-code", URL: "https://wxpusher.zjiecode.com/api/qrcode/test.jpg"}, nil
}
func (p *qrPush) ScanUID(context.Context, string) (string, error) { p.queries++; return p.uid, nil }
func TestScanBindingOwnershipThrottleAndConfirmation(t *testing.T) {
	s := service(t)
	ctx := context.Background()
	p := &qrPush{uid: "UID_test"}
	n := application.NewNotifications(s, p, box{})
	if err := n.SaveSettings(ctx, application.NotificationSettings{AppToken: "AT_test", BaseURL: "https://shop.example/admin"}); err != nil {
		t.Fatal(err)
	}
	v, err := n.CreateScan(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.ID) != 64 || p.extra != v.ID {
		t.Fatal("parameter must be a random session nonce")
	}
	if _, err = n.PollScan(ctx, 2, v.ID); !errors.Is(err, contract.ErrNotFound) {
		t.Fatal("another admin can read QR")
	}
	if _, err = n.PollScan(ctx, 1, v.ID); err != nil || p.queries != 0 {
		t.Fatal("queried before 10 seconds")
	}
	// Use a second persisted session whose first polling time has elapsed, without sleeping.
	v.ID = strings.Repeat("a", 64)
	v.NextPollAt = time.Now().Add(-time.Second)
	if err = s.Store.CreateScan(ctx, v); err != nil {
		t.Fatal(err)
	}
	v, err = n.PollScan(ctx, 1, v.ID)
	if err != nil || v.Status != "scanned" || p.queries != 1 || p.calls != 1 {
		t.Fatalf("scan not detected: %+v %v", v, err)
	}
	code := strings.Split(strings.TrimPrefix(p.content, "验证码 / Verification code: "), "\n")[0]
	if err = n.ConfirmScan(ctx, 2, v.ID, code); err == nil {
		t.Fatal("another admin confirmed")
	}
	if err = n.ConfirmScan(ctx, 1, v.ID, code); err != nil {
		t.Fatal(err)
	}
	b, _ := n.Binding(ctx, 1)
	if !b.Enabled || b.UID != "UID_test" {
		t.Fatal("binding not enabled")
	}
	if err = n.ConfirmScan(ctx, 1, v.ID, code); err == nil {
		t.Fatal("confirmation replay accepted")
	}
}
func TestWaitingScanPollingAndExpiry(t *testing.T) {
	s := service(t)
	ctx := context.Background()
	p := &qrPush{}
	n := application.NewNotifications(s, p, box{})
	v := &domain.ScanSession{ID: strings.Repeat("b", 64), AdminID: 1, Status: "waiting", ProviderCode: "private", NextPollAt: time.Now().Add(-time.Second), ExpiresAt: time.Now().Add(time.Minute)}
	if err := s.Store.CreateScan(ctx, v); err != nil {
		t.Fatal(err)
	}
	_, _ = n.PollScan(ctx, 1, v.ID)
	_, _ = n.PollScan(ctx, 1, v.ID)
	if p.queries != 1 {
		t.Fatal("polling not throttled")
	}
	if err := n.CancelScan(ctx, 1, v.ID); err != nil {
		t.Fatal(err)
	}
	_, _ = n.PollScan(ctx, 1, v.ID)
	if p.queries != 1 {
		t.Fatal("cancelled QR queried")
	}
	v.ID = strings.Repeat("c", 64)
	v.Status = "waiting"
	v.ExpiresAt = time.Now().Add(-time.Second)
	if err := s.Store.CreateScan(ctx, v); err != nil {
		t.Fatal(err)
	}
	expired, err := n.PollScan(ctx, 1, v.ID)
	if err != nil || expired.Status != "expired" || p.queries != 1 {
		t.Fatal("expired scan polled")
	}
}

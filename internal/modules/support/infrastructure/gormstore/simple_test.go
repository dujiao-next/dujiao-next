package gormstore_test

import (
	"context"
	"encoding/json"
	"github.com/dujiao-next/internal/modules/support/application"
	"github.com/dujiao-next/internal/modules/support/contract"
	"strings"
	"testing"
	"time"
)

type simplePush struct {
	push
	simpleCalls int
}

func (p *simplePush) SendSimple(ctx context.Context, spt, summary, content, link string) (contract.PushResult, error) {
	p.simpleCalls++
	p.content = content
	return contract.PushResult{}, nil
}
func TestSPTBindingAndReminderWithoutAppToken(t *testing.T) {
	s := service(t)
	ctx := context.Background()
	p := &simplePush{}
	n := application.NewNotifications(s, p, box{})
	if err := n.SaveSettings(ctx, application.NotificationSettings{Enabled: true, BaseURL: "https://shop.example/admin"}); err != nil {
		t.Fatal(err)
	}
	if err := n.BeginSimpleBinding(ctx, 1, "SPT_test_secret"); err != nil {
		t.Fatal(err)
	}
	b, _ := n.Binding(ctx, 1)
	if b.Enabled || b.PendingSPTCipher == "SPT_test_secret" || b.PendingSPTCipher == "" {
		t.Fatal("SPT enabled prematurely or not encrypted")
	}
	data, _ := json.Marshal(b)
	if strings.Contains(string(data), "SPT_test_secret") {
		t.Fatal("SPT leaked through JSON")
	}
	code := strings.Split(strings.TrimPrefix(p.content, "验证码 / Verification code: "), "\n")[0]
	if err := n.ConfirmBinding(ctx, 1, code); err != nil {
		t.Fatal(err)
	}
	b, _ = n.Binding(ctx, 1)
	if !b.Enabled || b.Mode != "simple" || b.UID != "" || b.SPTCipher == "" {
		t.Fatal("simple binding not committed")
	}
	_, _, err := s.Create(ctx, contract.Actor{UserID: 3}, application.CreateInput{Content: "private-content", ClientID: "first"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	events, _ := s.Store.DueReminders(ctx, time.Now().Add(time.Hour))
	for _, event := range events {
		_ = s.Store.RetryReminder(ctx, event.ID, time.Now().Add(-time.Second), "")
	}
	n.Process(ctx)
	if p.simpleCalls != 2 || p.calls != 0 || strings.Contains(p.content, "private-content") {
		t.Fatal("wrong route or leaked content")
	}
	if err = n.Unbind(ctx, 1); err != nil {
		t.Fatal(err)
	}
	b, _ = n.Binding(ctx, 1)
	if b.Enabled || b.SPTCipher != "" || b.PendingSPTCipher != "" {
		t.Fatal("unbind retains SPT")
	}
}

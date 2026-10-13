package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/dujiao-next/internal/modules/support/contract"
	"github.com/dujiao-next/internal/modules/support/domain"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Notifications struct {
	support *Service
	push    contract.Pusher
	box     contract.SecretBox
	mu      sync.Mutex
}

func NewNotifications(s *Service, p contract.Pusher, b contract.SecretBox) *Notifications {
	return &Notifications{support: s, push: p, box: b}
}

type NotificationSettings struct {
	Enabled  bool   `json:"enabled"`
	AppToken string `json:"app_token"`
	BaseURL  string `json:"base_url"`
}

func (n *Notifications) Settings(ctx context.Context) (NotificationSettings, error) {
	c, err := n.support.Store.Config(ctx)
	if err != nil {
		return NotificationSettings{}, err
	}
	token := ""
	if c.TokenCipher != "" {
		token = "********"
	}
	return NotificationSettings{c.Enabled, token, c.BaseURL}, nil
}
func (n *Notifications) SaveSettings(ctx context.Context, in NotificationSettings) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	c, err := n.support.Store.Config(ctx)
	if err != nil {
		return err
	}
	u, err := url.Parse(in.BaseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(in.BaseURL) > 800 {
		return contract.ErrInvalid
	}
	if in.AppToken != "" && in.AppToken != "********" {
		if !strings.HasPrefix(in.AppToken, "AT_") || len(in.AppToken) > 256 {
			return contract.ErrInvalid
		}
		c.TokenCipher, err = n.box.Encrypt(in.AppToken)
		if err != nil {
			return err
		}
	}

	c.Enabled = in.Enabled
	c.BaseURL = strings.TrimRight(in.BaseURL, "/")
	return n.support.Store.SaveConfig(ctx, c)
}
func (n *Notifications) Binding(ctx context.Context, id uint) (*domain.Binding, error) {
	return n.support.Store.Binding(ctx, id)
}
func (n *Notifications) BeginBinding(ctx context.Context, id uint, uid string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.support.agents.Eligible(ctx, id) || !strings.HasPrefix(uid, "UID_") || len(uid) > 80 || len(uid) < 8 {
		return contract.ErrInvalid
	}
	b, err := n.support.Store.Binding(ctx, id)
	if err != nil {
		return err
	}
	if time.Since(b.LastAttemptAt) < time.Minute {
		return contract.ErrConflict
	}
	c, err := n.support.Store.Config(ctx)
	if err != nil {
		return err
	}
	if c.TokenCipher == "" {
		return contract.ErrInvalid
	}
	token, err := n.box.Decrypt(c.TokenCipher)
	if err != nil {
		return err
	}
	bytes := make([]byte, 4)
	if _, err = rand.Read(bytes); err != nil {
		return err
	}
	code := hex.EncodeToString(bytes)
	b.PendingMode = "standard"
	b.PendingSPTCipher = ""
	b.PendingUID = uid
	if err = n.support.Store.CancelAdminScans(ctx, id); err != nil {
		return err
	}
	b.PendingHash = HashToken(code)
	b.PendingExpires = time.Now().Add(5 * time.Minute)
	b.LastAttemptAt = time.Now()
	if err = n.support.Store.SaveBinding(ctx, b); err != nil {
		return err
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err = n.push.Send(sendCtx, token, uid, "客服提醒绑定验证 / Support verification", "验证码 / Verification code: "+code+"\n5 分钟内在客服工作台确认 / Confirm in the support desk within 5 minutes.", c.BaseURL+"/support")
	return err
}
func (n *Notifications) ConfirmBinding(ctx context.Context, id uint, code string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	b, err := n.support.Store.Binding(ctx, id)
	if err != nil {
		return err
	}
	if b.PendingHash == "" || !b.PendingExpires.After(time.Now()) || HashToken(strings.TrimSpace(code)) != b.PendingHash {
		b.PendingHash = ""
		_ = n.support.Store.SaveBinding(ctx, b)
		return contract.ErrForbidden
	}
	if b.PendingMode == "simple" {
		b.Mode = "simple"
		b.SPTCipher = b.PendingSPTCipher
		b.UID = ""
	} else {
		b.Mode = "standard"
		b.SPTCipher = ""
		b.UID = b.PendingUID
	}
	b.PendingMode = ""
	b.PendingSPTCipher = ""
	b.Enabled = true
	b.PendingUID = ""
	b.PendingHash = ""
	return n.support.Store.SaveBinding(ctx, b)
}
func (n *Notifications) Unbind(ctx context.Context, id uint) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	b, err := n.support.Store.Binding(ctx, id)
	if err != nil {
		return err
	}
	b.Enabled = false
	b.Mode = ""
	b.SPTCipher = ""
	b.PendingMode = ""
	b.PendingSPTCipher = ""
	if err = n.support.Store.CancelAdminScans(ctx, id); err != nil {
		return err
	}
	b.UID = ""
	b.PendingUID = ""
	b.PendingHash = ""
	return n.support.Store.SaveBinding(ctx, b)
}
func (n *Notifications) Test(ctx context.Context, id uint) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	b, err := n.support.Store.Binding(ctx, id)
	if err != nil {
		return err
	}
	if !b.Enabled || time.Since(b.LastAttemptAt) < time.Minute {
		return contract.ErrConflict
	}
	c, err := n.support.Store.Config(ctx)
	if err != nil {
		return err
	}
	b.LastAttemptAt = time.Now()
	if err = n.support.Store.SaveBinding(ctx, b); err != nil {
		return err
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err = n.sendToBinding(sendCtx, c, b, "客服提醒测试 / Support test", "客服提醒已连接。请回站处理咨询。 / Support notifications connected. Return to the store to reply.", c.BaseURL+"/support")
	return err
}
func (n *Notifications) Name() string               { return "support-reminders" }
func (n *Notifications) Stop(context.Context) error { return nil }
func (n *Notifications) Start(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			n.Process(ctx)
		}
	}
}

// Process consumes the durable outbox independently of optional Redis/asynq.
// Leases protect concurrent API instances; remote acceptance is not exactly-once.
func (n *Notifications) Process(ctx context.Context) {
	n.mu.Lock()
	defer n.mu.Unlock()
	c, err := n.support.Store.Config(ctx)
	if err != nil || !c.Enabled {
		return
	}
	events, err := n.support.Store.DueReminders(ctx, time.Now())
	if err != nil {
		return
	}
	bindings, err := n.support.Store.Bindings(ctx)
	if err != nil {
		return
	}
	for _, event := range events {
		if ctx.Err() != nil {
			return
		}
		now := time.Now()
		claimed, err := n.support.Store.ClaimReminder(ctx, event.ID, now)
		if err != nil || !claimed {
			continue
		}
		conv, err := n.support.Store.Get(ctx, event.ConversationID)
		if err != nil {
			_ = n.support.Store.RetryReminder(ctx, event.ID, now.Add(time.Minute), "conversation lookup failed")
			continue
		}
		if conv.Status == "resolved" || conv.AdminReadID >= event.MessageID || now.Sub(event.CreatedAt) > 24*time.Hour {
			_ = n.support.Store.DeleteReminder(ctx, event.ID)
			continue
		}
		msgs, err := n.support.Store.Messages(ctx, conv.ID, event.MessageID, 100, true)
		if err != nil {
			_ = n.support.Store.RetryReminder(ctx, event.ID, now.Add(time.Minute), "message lookup failed")
			continue
		}
		responded := false
		for _, m := range msgs {
			if m.SenderType == "admin" && !m.Internal {
				responded = true
			}
		}
		if responded {
			_ = n.support.Store.DeleteReminder(ctx, event.ID)
			continue
		}
		failed := false
		recipients := 0
		for i := range bindings {
			if recipients >= 10 {
				failed = true
				break
			}
			b := &bindings[i]
			if conv.AssigneeID != 0 && conv.AssigneeID != b.AdminID {
				continue
			}
			if !n.support.agents.Eligible(ctx, b.AdminID) {
				continue
			}
			recipients++
			delivery, err := n.support.Store.Delivery(ctx, event.ID, b.AdminID)
			if err != nil {
				failed = true
				continue
			}
			if delivery.Status == "accepted" || delivery.Status == "suppressed" {
				continue
			}
			if now.Sub(b.LastSentAt) < 3*time.Minute {
				delivery.Status = "suppressed"
				if n.support.Store.SaveDelivery(ctx, delivery) != nil {
					failed = true
				}
				continue
			}
			sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			result, sendErr := n.sendToBinding(sendCtx, c, b, "客服有待回复咨询 / Support needs attention", fmt.Sprintf("会话 / Conversation: CS-%d\n类型 / Category: %s\n请回站查看，提醒不展示聊天正文或卡密。 / Return to the support desk. Message content and secrets are not included.", conv.ID, conv.Category), fmt.Sprintf("%s/support?conversation=%d", c.BaseURL, conv.ID))
			cancel()
			if errors.Is(sendErr, contract.ErrNotificationNotConfigured) {
				// Standard mode without a configured app token must not retry forever.
				delivery.Status = "suppressed"
				delivery.Error = "notification channel not configured"
				_ = n.support.Store.SaveDelivery(ctx, delivery)
				continue
			}
			if sendErr != nil {
				delivery.Status = "failed"
				delivery.Error = "channel request failed"
				failed = true
			} else {
				delivery.Status = "accepted"
				delivery.SendRecordID = result.SendRecordID
				delivery.Error = ""
				b.LastSentAt = time.Now()
				_ = n.support.Store.SaveBinding(ctx, b)
			}
			if n.support.Store.SaveDelivery(ctx, delivery) != nil {
				failed = true
			}
		}
		if recipients == 0 {
			_ = n.support.Store.RetryReminder(ctx, event.ID, now.Add(time.Minute), "no bound eligible agent")
		} else if failed {
			delay := time.Minute * time.Duration(1<<min(event.Attempts, 5))
			_ = n.support.Store.RetryReminder(ctx, event.ID, now.Add(delay), "channel request failed")
		} else {
			_ = n.support.Store.DeleteReminder(ctx, event.ID)
		}
		// One event per tick bounds push volume; recipient count is separately capped below.
		break
	}
}

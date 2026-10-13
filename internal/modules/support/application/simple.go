package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/support/contract"
	"github.com/dujiao-next/internal/modules/support/domain"
)

func (n *Notifications) BeginSimpleBinding(ctx context.Context, id uint, spt string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	spt = strings.TrimSpace(spt)
	if !n.support.agents.Eligible(ctx, id) || !strings.HasPrefix(spt, "SPT_") || len(spt) < 8 || len(spt) > 512 || strings.ContainsAny(spt, " \r\n\t") {
		return contract.ErrInvalid
	}
	if _, ok := n.push.(contract.SimplePusher); !ok {
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
	cipher, err := n.box.Encrypt(spt)
	if err != nil {
		return err
	}
	bytes := make([]byte, 4)
	if _, err = rand.Read(bytes); err != nil {
		return err
	}
	code := hex.EncodeToString(bytes)
	if err = n.support.Store.CancelAdminScans(ctx, id); err != nil {
		return err
	}
	b.PendingMode = "simple"
	b.PendingSPTCipher = cipher
	b.PendingUID = ""
	b.PendingHash = HashToken(code)
	b.PendingExpires = time.Now().Add(5 * time.Minute)
	b.LastAttemptAt = time.Now()
	if err = n.support.Store.SaveBinding(ctx, b); err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	_, err = n.push.(contract.SimplePusher).SendSimple(callCtx, spt, "客服极简绑定验证 / Support verification", "验证码 / Verification code: "+code+"\n请在客服工作台确认 / Confirm in the support desk.", c.BaseURL+"/support")
	return err
}

func (n *Notifications) sendToBinding(ctx context.Context, c *domain.NotificationConfig, b *domain.Binding, summary, content, link string) (contract.PushResult, error) {
	if b.Mode == "simple" {
		p, ok := n.push.(contract.SimplePusher)
		if !ok || b.SPTCipher == "" {
			return contract.PushResult{}, contract.ErrInvalid
		}
		spt, err := n.box.Decrypt(b.SPTCipher)
		if err != nil {
			return contract.PushResult{}, err
		}
		return p.SendSimple(ctx, spt, summary, content, link)
	}
	if c.TokenCipher == "" || b.UID == "" {
		return contract.PushResult{}, contract.ErrNotificationNotConfigured
	}
	token, err := n.box.Decrypt(c.TokenCipher)
	if err != nil {
		return contract.PushResult{}, err
	}
	return n.push.Send(ctx, token, b.UID, summary, content, link)
}

package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/dujiao-next/internal/modules/support/contract"
	"github.com/dujiao-next/internal/modules/support/domain"
)

func (n *Notifications) CreateScan(ctx context.Context, admin uint) (*domain.ScanSession, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	provider, ok := n.push.(contract.QRSubscriber)
	if !ok || !n.support.agents.Eligible(ctx, admin) {
		return nil, contract.ErrForbidden
	}
	b, err := n.support.Store.Binding(ctx, admin)
	if err != nil {
		return nil, err
	}
	if time.Since(b.LastAttemptAt) < time.Minute {
		return nil, contract.ErrConflict
	}
	cfg, err := n.support.Store.Config(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.TokenCipher == "" {
		return nil, contract.ErrNotificationNotConfigured
	}
	token, err := n.box.Decrypt(cfg.TokenCipher)
	if err != nil {
		return nil, err
	}
	b.LastAttemptAt = time.Now()
	if err = n.support.Store.SaveBinding(ctx, b); err != nil {
		return nil, err
	}
	bytes := make([]byte, 32)
	if _, err = rand.Read(bytes); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(bytes)
	expires := time.Now().Add(5 * time.Minute)
	callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	qr, err := provider.CreateQRCode(callCtx, token, id, 300)
	if err != nil {
		return nil, err
	}
	v := &domain.ScanSession{ID: id, AdminID: admin, ProviderCode: qr.Code, URL: qr.URL, Status: "waiting", ExpiresAt: expires, NextPollAt: time.Now().Add(11 * time.Second)}
	if err = n.support.Store.CreateScan(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}
func (n *Notifications) PollScan(ctx context.Context, admin uint, id string) (*domain.ScanSession, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(id) != 64 {
		return nil, contract.ErrInvalid
	}
	v, err := n.support.Store.Scan(ctx, admin, id)
	if err != nil {
		return nil, err
	}
	if !v.ExpiresAt.After(time.Now()) && v.Status != "confirmed" {
		v.Status = "expired"
		_ = n.support.Store.CancelScan(ctx, admin, id)
		return v, nil
	}
	if v.Status != "waiting" {
		return v, nil
	}
	provider, ok := n.push.(contract.QRSubscriber)
	if !ok {
		return nil, contract.ErrInvalid
	}
	claimed, err := n.support.Store.ClaimScanPoll(ctx, admin, id, time.Now())
	if err != nil {
		return nil, err
	}
	if !claimed {
		return v, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	uid, err := provider.ScanUID(callCtx, v.ProviderCode)
	if err != nil {
		return nil, err
	}
	if uid == "" {
		return v, nil
	}
	bytes := make([]byte, 4)
	if _, err = rand.Read(bytes); err != nil {
		return nil, err
	}
	code := hex.EncodeToString(bytes)
	if err = n.support.Store.SetScanCandidate(ctx, admin, id, uid, HashToken(code)); err != nil {
		return nil, err
	}
	cfg, err := n.support.Store.Config(ctx)
	if err != nil {
		_ = n.support.Store.CancelScan(ctx, admin, id)
		return nil, err
	}
	token, err := n.box.Decrypt(cfg.TokenCipher)
	if err != nil {
		_ = n.support.Store.CancelScan(ctx, admin, id)
		return nil, err
	}
	_, err = n.push.Send(callCtx, token, uid, "客服扫码绑定验证 / Support verification", "验证码 / Verification code: "+code+"\n请在生成二维码的客服工作台确认。不要向他人分享二维码或验证码。 / Confirm in the support desk that generated this QR code. Do not share the QR code or verification code.", cfg.BaseURL+"/support")
	if err != nil {
		_ = n.support.Store.CancelScan(ctx, admin, id)
		return nil, err
	}
	v.Status = "scanned"
	v.UID = uid
	return v, nil
}
func (n *Notifications) ConfirmScan(ctx context.Context, admin uint, id, code string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(id) != 64 || len(code) != 8 {
		return contract.ErrInvalid
	}
	if !n.support.agents.Eligible(ctx, admin) {
		return contract.ErrForbidden
	}
	if err := n.support.Store.ConfirmScan(ctx, admin, id, HashToken(code)); err != nil {
		return err
	}
	return nil
}
func (n *Notifications) CancelScan(ctx context.Context, admin uint, id string) error {
	if len(id) != 64 {
		return contract.ErrInvalid
	}
	return n.support.Store.CancelScan(ctx, admin, id)
}

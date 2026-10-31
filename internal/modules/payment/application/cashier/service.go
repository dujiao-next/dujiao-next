// Package cashier exposes only merchant-owned, validated payment instructions.
package cashier

import (
	"context"
	"errors"
	"time"

	"github.com/dujiao-next/internal/constants"
	pc "github.com/dujiao-next/internal/modules/payment/contract"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/shopspring/decimal"
)

type Service struct {
	Store    pc.Store
	Registry pc.GatewayRegistry
	Enabled  bool
}

// Read does not call a gateway/RPC or create/settle/cancel a payment. The caller
// must authorize orderID for the current tenant; ownership is checked again here.
func (s *Service) Read(ctx context.Context, paymentID, orderID, userID uint) (pd.CashierView, error) {
	view := pd.NewCashierView(paymentID, "unavailable")
	if s == nil || s.Store == nil || s.Registry == nil || userID == 0 || orderID == 0 || paymentID == 0 {
		return view, pd.ErrCashierIneligible
	}
	if !s.Enabled {
		view.State = "paused"
		return view, nil
	}
	err := s.Store.WithinTransaction(func(base pc.Transaction) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		tx, ok := base.(pc.SettlementTransaction)
		reader, supported := base.Payments().(interface {
			OpenAutomationRevision(context.Context) (uint64, error)
		})
		if !ok || !supported {
			return pd.ErrAutomationFrozen
		}
		revision, err := reader.OpenAutomationRevision(ctx)
		if err != nil {
			return err
		}
		if err := tx.LockAutomation(revision); err != nil {
			return err
		}
		order, err := tx.Orders().GetByIDForUpdateWithChildren(orderID)
		if err != nil {
			return err
		}
		if order == nil || order.UserID != userID || order.ParentID != nil {
			return pd.ErrCashierIneligible
		}
		p, err := tx.Payments().GetByIDForUpdate(paymentID)
		if err != nil {
			return err
		}
		if p == nil || p.OrderID != orderID || !p.RequiresChainVerification || p.DeletedAt != nil || p.SupersededAt != nil {
			return pd.ErrCashierIneligible
		}
		if order.Status != constants.OrderStatusPendingPayment || p.Status == constants.PaymentStatusSuccess {
			view.State = "closed"
			return nil
		}
		now := time.Now()
		if order.ExpiresAt == nil || !order.ExpiresAt.After(now) || p.ExpiredAt == nil || !p.ExpiredAt.After(now) {
			view.State = "expired"
			return nil
		}
		if pd.ValidateFixedOrder(order) != nil || p.PaidAt != nil || p.ExceptionCode != "" ||
			(p.Status != constants.PaymentStatusInitiated && p.Status != constants.PaymentStatusPending) ||
			p.SettlementBasis == "" || p.SettlementBasis != pd.OrderBasis(*p, *order) {
			return pd.ErrCashierIneligible
		}
		for _, child := range order.Children {
			if child.ExpiresAt == nil || !child.ExpiresAt.After(now) {
				return pd.ErrCashierIneligible
			}
		}
		provider, ok := s.Registry.Lookup(p.ProviderType, p.ChannelType)
		if !ok {
			return pd.ErrCashierIneligible
		}
		intent, ok := provider.(pc.GatewayChainIntentReader)
		if !ok {
			return pd.ErrCashierIneligible
		}
		expected, err := intent.ReadChainExpectation(*p)
		if errors.Is(err, pc.ErrChainPending) {
			view.State = "preparing"
			return nil
		}
		if err != nil {
			return err
		}
		allocation, err := tx.Allocation(p.ID)
		if err != nil {
			return err
		}
		if allocation == nil || allocation.OrderID != order.ID || allocation.PaymentID != p.ID || allocation.SlotKey != expected.SlotKey() ||
			allocation.Network != expected.Network || allocation.Contract != expected.Contract || allocation.Recipient != expected.Recipient || allocation.RawAmount != expected.RawAmount ||
			allocation.Snapshot != pd.SettlementSnapshot(*p, *order) {
			return pd.ErrCashierIneligible
		}
		if expected.CreatedAtMs > now.UnixMilli() || expected.ExpiresAtMs <= now.UnixMilli() || expected.ExpiresAtMs > order.ExpiresAt.UnixMilli() || expected.ExpiresAtMs != p.ExpiredAt.UnixMilli() {
			return pd.ErrCashierIneligible
		}
		covered := p.Amount.Decimal
		switch p.FeePolicy {
		case constants.PaymentFeePolicyCustomerSurcharge:
			covered = covered.Sub(p.FeeAmount.Decimal)
		case constants.PaymentFeePolicyMerchantAbsorbed:
		case constants.PaymentFeePolicyNone:
			if !p.FeeAmount.Decimal.IsZero() {
				return pd.ErrCashierIneligible
			}
		default:
			return pd.ErrCashierIneligible
		}
		online := order.TotalAmount.Decimal.Sub(order.WalletPaidAmount.Decimal)
		if p.Currency != order.Currency || p.FeeAmount.Decimal.IsNegative() || !online.IsPositive() || !covered.Equal(online) ||
			(!order.OnlinePaidAmount.Decimal.IsZero() && !order.OnlinePaidAmount.Decimal.Equal(online)) {
			return pd.ErrCashierIneligible
		}
		for _, child := range order.Children {
			if expected.ExpiresAtMs > child.ExpiresAt.UnixMilli() {
				return pd.ErrCashierIneligible
			}
		}
		var decimals int32
		switch expected.Network {
		case "tron":
			decimals = 6
		case "binance":
			decimals = 18
		default:
			return pd.ErrCashierIneligible
		}
		raw, err := decimal.NewFromString(expected.RawAmount)
		if err != nil || !raw.IsPositive() || raw.StringFixed(0) != expected.RawAmount {
			return pd.ErrCashierIneligible
		}
		view = pd.NewCashierView(paymentID, "ready")
		view.DisplayUntilMs = min(view.ServerTimeMs+15000, expected.ExpiresAtMs)
		view.Details = &pd.CashierDetails{Network: expected.Network, Token: "USDT", Contract: expected.Contract, Recipient: expected.Recipient,
			RawAmount: expected.RawAmount, Decimals: decimals, Amount: raw.Shift(-decimals).String(), FiatAmount: p.Amount.String(), FiatCurrency: p.Currency, ExpiresAtMs: expected.ExpiresAtMs}
		return ctx.Err()
	})
	if errors.Is(err, pd.ErrAutomationFrozen) {
		return pd.NewCashierView(paymentID, "paused"), nil
	}
	if err != nil {
		return pd.NewCashierView(paymentID, "unavailable"), err
	}
	return view, nil
}

package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	od "github.com/dujiao-next/internal/modules/order/domain"
	"github.com/shopspring/decimal"
)

var ErrCashierIneligible = errors.New("fixed payment order is not eligible")

// ValidateFixedOrder checks persisted obligations, not a frontend projection
// (which may flatten child items or mask upstream fulfillment as manual).
func ValidateFixedOrder(o *od.Order) error {
	if o == nil || o.ID == 0 || o.UserID == 0 || o.ParentID != nil || !o.TotalAmount.Decimal.IsPositive() {
		return ErrCashierIneligible
	}
	orders := append([]od.Order{*o}, o.Children...)
	seenOrders, seenItems := map[uint]bool{}, map[uint]bool{}
	childTotal := decimal.Zero
	for i, current := range orders {
		if current.ID == 0 || seenOrders[current.ID] || current.UserID != o.UserID || current.Currency != o.Currency ||
			current.ExpiresAt == nil || !current.ExpiresAt.After(time.Now()) || current.Status != constants.OrderStatusPendingPayment || current.DeletedAt != nil || current.PaidAt != nil || current.CanceledAt != nil ||
			!current.RefundedAmount.Decimal.IsZero() || current.TotalAmount.Decimal.IsNegative() || current.WalletPaidAmount.Decimal.IsNegative() ||
			current.WalletPaidAmount.Decimal.GreaterThan(current.TotalAmount.Decimal) || strings.TrimSpace(current.OrderNo) == "" {
			return ErrCashierIneligible
		}
		seenOrders[current.ID] = true
		if i > 0 {
			if current.ParentID == nil || *current.ParentID != o.ID || len(current.Children) != 0 ||
				(current.ResellerID == nil) != (o.ResellerID == nil) || (current.ResellerID != nil && *current.ResellerID != *o.ResellerID) {
				return ErrCashierIneligible
			}
			childTotal = childTotal.Add(current.TotalAmount.Decimal)
		}
		if i == 0 && len(o.Children) > 0 {
			if len(current.Items) != 0 {
				return ErrCashierIneligible
			}
			continue
		}
		if len(current.Items) == 0 {
			return ErrCashierIneligible
		}
		for _, item := range current.Items {
			if item.ID == 0 || seenItems[item.ID] || item.OrderID != current.ID || item.ProductID == 0 || item.SKUID == 0 || item.Quantity <= 0 || item.DeletedAt != nil ||
				(item.FulfillmentType != constants.FulfillmentTypeAuto && item.FulfillmentType != constants.FulfillmentTypeManual) {
				return ErrCashierIneligible
			}
			seenItems[item.ID] = true
		}
	}
	if len(o.Children) > 0 && !childTotal.Equal(o.TotalAmount.Decimal) {
		return ErrCashierIneligible
	}
	return nil
}

// CashierView deliberately omits order contents, customer data and provider
// payloads. DisplayUntil bounds stale display; it cannot revoke copied addresses.
type CashierView struct {
	PaymentID      uint            `json:"payment_id"`
	State          string          `json:"state"`
	ServerTimeMs   int64           `json:"server_time_ms"`
	DisplayUntilMs int64           `json:"display_until_ms"`
	Details        *CashierDetails `json:"details,omitempty"`
}

type CashierDetails struct {
	Network      string `json:"network"`
	Token        string `json:"token"`
	Contract     string `json:"contract"`
	Recipient    string `json:"recipient"`
	RawAmount    string `json:"raw_amount"`
	Decimals     int32  `json:"decimals"`
	Amount       string `json:"amount"`
	FiatAmount   string `json:"fiat_amount"`
	FiatCurrency string `json:"fiat_currency"`
	ExpiresAtMs  int64  `json:"expires_at_ms"`
}

func NewCashierView(id uint, state string) CashierView {
	return CashierView{PaymentID: id, State: state, ServerTimeMs: time.Now().UnixMilli()}
}

package gormstore

import (
	"errors"

	"github.com/dujiao-next/internal/constants"
	od "github.com/dujiao-next/internal/modules/order/domain"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	"gorm.io/gorm/clause"
)

// LockPaymentDelivery is called on the existing fulfillment transaction BEFORE
// order/card locks. A legacy queue payload or manual endpoint cannot bypass it.
// It also detects consumption after a damaged payment flag, without trusting Redis.
func (r *Store) LockPaymentDelivery(rootID uint, required bool) (bool, error) {
	var fixed, consumed int64
	if err := r.db.Model(&pd.Payment{}).Where("order_id = ? AND requires_chain_verification = ? AND status = ?", rootID, true, constants.PaymentStatusSuccess).Count(&fixed).Error; err != nil {
		return false, err
	}
	if err := r.db.Model(&pd.EventConsumption{}).Where("order_id = ?", rootID).Count(&consumed).Error; err != nil {
		return false, err
	}
	if fixed == 0 && consumed == 0 && !required {
		return false, nil
	}
	if err := automation.LockOpen(r.db, 0); err != nil {
		return true, err
	}
	invalid := errors.New("fixed payment delivery authorization invalid")
	if fixed != 1 || consumed != 1 {
		return true, invalid
	}
	var root od.Order
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").Preload("Children.Items").First(&root, rootID).Error; err != nil {
		return true, err
	}
	if root.ParentID != nil || root.PaidAt == nil || root.DeletedAt != nil || root.CanceledAt != nil || !root.RefundedAmount.Decimal.IsZero() || root.Status == constants.OrderStatusCanceled {
		return true, invalid
	}
	// Preloads do not inherit FOR UPDATE. All children precede the payment lock.
	for i := range root.Children {
		var child od.Order
		if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").First(&child, root.Children[i].ID).Error; err != nil {
			return true, err
		}
		if child.DeletedAt != nil || child.CanceledAt != nil || !child.RefundedAmount.Decimal.IsZero() {
			return true, invalid
		}
		root.Children[i] = child
	}
	var event pd.EventConsumption
	if err := r.db.Where("order_id = ?", rootID).First(&event).Error; err != nil {
		return true, err
	}
	var payment pd.Payment
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&payment, event.PaymentID).Error; err != nil {
		return true, err
	}
	if payment.OrderID != root.ID || !payment.RequiresChainVerification || payment.Status != constants.PaymentStatusSuccess || payment.PaidAt == nil || payment.DeletedAt != nil || payment.SupersededAt != nil || payment.SettlementBasis != pd.OrderBasis(payment, root) {
		return true, invalid
	}
	var allocation pd.Allocation
	if err := r.db.First(&allocation, event.AllocationID).Error; err != nil {
		return true, err
	}
	if allocation.PaymentID != payment.ID || allocation.OrderID != root.ID || allocation.Snapshot != pd.SettlementSnapshot(payment, root) {
		return true, invalid
	}
	return true, nil
}

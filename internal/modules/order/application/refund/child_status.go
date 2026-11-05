package refund

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// applyParentRefundChildStatusUpdates 根据父订单退款结果统一更新子订单状态。
// 规则:子订单退款状态始终跟随父订单,避免分支逻辑造成状态混乱。
//
// orderStore 必须来自订单工作单元，确保父子状态在同一事务中更新。
func applyParentRefundChildStatusUpdates(orderStore ordercontract.Store, parentOrderID uint, parentTargetStatus string, now time.Time) error {
	if orderStore == nil || parentOrderID == 0 {
		return nil
	}

	target := strings.ToLower(strings.TrimSpace(parentTargetStatus))
	if target != constants.OrderStatusPartiallyRefunded && target != constants.OrderStatusRefunded {
		return nil
	}

	affected, err := orderStore.UpdateChildrenStatus(parentOrderID, target, now)
	if err != nil {
		return err
	}
	if affected > 0 {
		logger.Debugw("refund_child_status_propagated",
			"parent_order_id", parentOrderID,
			"target_status", target,
			"rows_affected", affected,
		)
	}
	return nil
}

// reserveParentRefund 子订单退款同时受父订单剩余可退金额约束，并累加到父订单已退金额。
// 父订单退款只看父订单自身的已退金额，若子订单退款不计入父订单，就能"先退子单、再退父单"重复退款。
func reserveParentRefund(orderStore ordercontract.Store, order *orderdomain.Order, amount decimal.Decimal, now time.Time) error {
	if order == nil || order.ParentID == nil || *order.ParentID == 0 {
		return nil
	}
	parent, err := orderStore.GetByIDForUpdate(*order.ParentID)
	if err != nil {
		return err
	}
	if parent == nil {
		return ErrOrderNotFound
	}
	parentRefunded := parent.RefundedAmount.Decimal.Round(2)
	if amount.GreaterThan(parent.TotalAmount.Decimal.Sub(parentRefunded).Round(2)) {
		return walletcontract.ErrRefundExceeded
	}
	return orderStore.UpdateFields(parent.ID, map[string]interface{}{
		"refunded_amount": money.FromDecimal(parentRefunded.Add(amount).Round(2)),
		"updated_at":      now,
	})
}

package application

import (
	"fmt"
	"strings"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type refundAllocationItem struct {
	OrderItemID          string `json:"order_item_id"`
	RefundRatio          string `json:"refund_ratio"`
	OriginalProfitAmount string `json:"original_profit_amount"`
	DeductAmount         string `json:"deduct_amount"`
}

type refundAllocation struct {
	RefundRecordID uint                   `json:"refund_record_id"`
	OrderID        uint                   `json:"order_id"`
	RefundAmount   string                 `json:"refund_amount"`
	OrderAmount    string                 `json:"order_amount"`
	Items          []refundAllocationItem `json:"items"`
}

func decimalFromSnapshotValue(v interface{}) decimal.Decimal {
	switch val := v.(type) {
	case string:
		d, err := decimal.NewFromString(strings.TrimSpace(val))
		if err == nil {
			return d.Round(2)
		}
	case float64:
		return decimal.NewFromFloat(val).Round(2)
	case int:
		return decimal.NewFromInt(int64(val)).Round(2)
	case int64:
		return decimal.NewFromInt(val).Round(2)
	case decimal.Decimal:
		return val.Round(2)
	}
	return decimal.Zero
}

func stringFromSnapshotValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case float64:
		return decimal.NewFromFloat(val).StringFixed(0)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	}
	return ""
}

// HandleRefundDeduct 在调用方已开启的事务 store 上写入退款利润扣减流水。
func (s *AccountingLedgerService) HandleRefundDeduct(
	store resellercontract.AccountingLedgerStore,
	order *orderdomain.Order,
	refundRecord *orderdomain.OrderRefundRecord,
	refundedBefore decimal.Decimal,
) error {
	if s == nil || store == nil || order == nil || refundRecord == nil || refundRecord.ID == 0 {
		return nil
	}
	if order.ResellerID == nil || *order.ResellerID == 0 {
		return nil
	}
	refundAmount := refundRecord.Amount.Decimal.Round(2)
	if refundAmount.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	snapshot, err := store.GetOrderSnapshotByOrderID(order.ID)
	if err != nil {
		return err
	}
	if snapshot == nil {
		logger.Warnw("reseller_refund_missing_snapshot_skip", "order_id", order.ID, "order_no", order.OrderNo, "refund_record_id", refundRecord.ID)
		return nil
	}
	if !snapshot.ProfitEligible {
		return nil
	}
	grossProfit := snapshot.ProfitAmount.Decimal.Round(2)
	if grossProfit.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	// 入账时可能已按通道费分摊扣减过利润，退款冲回要以实际入账的净利为基数，
	// 否则会按毛利润多扣。历史订单没有分摊时两者相等。
	profitEntry, err := store.GetLedgerEntryByIdempotencyKey(fmt.Sprintf("order_profit:%d", order.ID))
	if err != nil {
		return err
	}
	refundableProfit := grossProfit
	if profitEntry != nil {
		refundableProfit = profitEntry.Amount.Decimal.Round(2)
	}
	if refundableProfit.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	orderAmount := snapshot.ResellerAmount.Decimal.Round(2)
	if orderAmount.LessThanOrEqual(decimal.Zero) {
		orderAmount = order.TotalAmount.Decimal.Round(2)
	}
	if orderAmount.LessThanOrEqual(decimal.Zero) {
		return resellercontract.ErrLedgerInvalidSnapshot
	}
	refundedBefore = refundedBefore.Round(2)
	remainingBefore := orderAmount.Sub(refundedBefore).Round(2)
	if remainingBefore.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	if refundAmount.GreaterThan(remainingBefore) {
		refundAmount = remainingBefore
	}
	deductedSoFar, err := store.SumLedgerAmountByOrderAndType(order.ID, resellerdomain.LedgerTypeRefundDeduct)
	if err != nil {
		return err
	}
	remainingProfit := refundableProfit.Sub(deductedSoFar.Abs().Round(2)).Round(2)
	if remainingProfit.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	ratio := refundAmount.Div(orderAmount)
	deduct := refundableProfit.Mul(ratio).Round(2)
	fullyRefunded := refundedBefore.Add(refundAmount).GreaterThanOrEqual(orderAmount)
	if fullyRefunded || deduct.GreaterThan(remainingProfit) {
		deduct = remainingProfit
	}
	if deduct.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	// 按毛利占比分摊到订单项，保持原有审计口径。
	allocRatio := deduct.Div(grossProfit)
	allocation := refundAllocation{
		RefundRecordID: refundRecord.ID,
		OrderID:        order.ID,
		RefundAmount:   refundAmount.StringFixed(2),
		OrderAmount:    orderAmount.StringFixed(2),
		Items:          make([]refundAllocationItem, 0),
	}
	if rawItems, ok := snapshot.PricingSnapshotJSON["items"].([]interface{}); ok {
		for _, raw := range rawItems {
			itemMap, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			itemProfit := decimalFromSnapshotValue(itemMap["profit_amount"])
			itemDeduct := itemProfit.Mul(allocRatio).Round(2)
			if itemDeduct.LessThanOrEqual(decimal.Zero) {
				continue
			}
			allocation.Items = append(allocation.Items, refundAllocationItem{
				OrderItemID:          stringFromSnapshotValue(itemMap["order_item_id"]),
				RefundRatio:          ratio.StringFixed(8),
				OriginalProfitAmount: itemProfit.StringFixed(2),
				DeductAmount:         itemDeduct.StringFixed(2),
			})
		}
	}
	now := time.Now()
	orderID := order.ID

	deductStatus := resellerdomain.LedgerStatusAvailable
	var deductAvailableAt *time.Time
	if profitEntry != nil && profitEntry.Status == resellerdomain.LedgerStatusPendingConfirm {
		deductStatus = resellerdomain.LedgerStatusPendingConfirm
		if profitEntry.AvailableAt != nil {
			deductAvailableAt = profitEntry.AvailableAt
		} else {
			at := now.AddDate(0, 0, s.confirmDays)
			deductAvailableAt = &at
		}
	}

	entry := &resellerdomain.LedgerEntry{
		ResellerID:  snapshot.ResellerID,
		OrderID:     &orderID,
		Type:        resellerdomain.LedgerTypeRefundDeduct,
		Amount:      money.FromDecimal(deduct.Neg()),
		Currency:    strings.TrimSpace(snapshot.Currency),
		Status:      deductStatus,
		AvailableAt: deductAvailableAt,
		MetadataJSON: jsonmap.JSON{
			"refund_record_id":       refundRecord.ID,
			"refund_type":            refundRecord.Type,
			"refund_amount":          refundAmount.StringFixed(2),
			"refunded_before":        refundedBefore.Round(2).StringFixed(2),
			"refund_allocation_json": allocation,
			"snapshot_id":            snapshot.ID,
			"deduct_status":          deductStatus,
		},
		IdempotencyKey: fmt.Sprintf("refund_deduct:%d", refundRecord.ID),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if entry.Currency == "" {
		entry.Currency = strings.TrimSpace(refundRecord.Currency)
	}
	if entry.Currency == "" {
		entry.Currency = strings.TrimSpace(order.Currency)
	}
	if entry.Currency == "" {
		return resellercontract.ErrLedgerInvalidSnapshot
	}
	_, err = store.CreateLedgerEntryIfNotExists(entry)
	if err != nil {
		return err
	}
	return RefreshBalanceAccount(store, snapshot.ResellerID, entry.Currency, now)
}

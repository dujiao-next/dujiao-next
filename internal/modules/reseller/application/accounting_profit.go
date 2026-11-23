package application

import (
	"fmt"
	"strings"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

// AccountingLedgerService 分销利润入账、退款扣减与到期确认用例。
type AccountingLedgerService struct {
	store             resellercontract.AccountingLedgerStore
	confirmDays       int
	channelFeePercent decimal.Decimal
}

func NewAccountingLedgerService(store resellercontract.AccountingLedgerStore, confirmDays int, channelFeePercent float64) *AccountingLedgerService {
	const maxConfirmDays = 3650
	if confirmDays < 0 {
		confirmDays = 0
	}
	if confirmDays > maxConfirmDays {
		confirmDays = maxConfirmDays
	}
	rate := decimal.NewFromFloat(channelFeePercent)
	if rate.LessThan(decimal.Zero) {
		rate = decimal.Zero
	}
	if rate.GreaterThan(decimal.NewFromInt(100)) {
		rate = decimal.NewFromInt(100)
	}
	return &AccountingLedgerService{
		store:             store,
		confirmDays:       confirmDays,
		channelFeePercent: rate.Round(4),
	}
}

// isCustomerPaidFeePolicy 判断该笔支付的手续费是否已由客户承担（加收）。
// 客户已付的情形不再从代理利润里分摊，避免重复收费。
func isCustomerPaidFeePolicy(policy string) bool {
	return policy == constants.PaymentFeePolicyCustomerSurcharge ||
		policy == constants.PaymentFeePolicyLegacyCustomerSurcharge
}

// PostOrderProfit 在调用方已开启的事务 store 上写入订单利润流水。
func (s *AccountingLedgerService) PostOrderProfit(store resellercontract.AccountingLedgerStore, order *orderdomain.Order, payment *paymentdomain.Payment) error {
	if s == nil || store == nil || order == nil || order.ID == 0 {
		return nil
	}
	if order.ResellerID == nil || *order.ResellerID == 0 {
		return nil
	}
	snapshot, err := store.GetOrderSnapshotByOrderID(order.ID)
	if err != nil {
		return err
	}
	if snapshot == nil {
		logger.Warnw("reseller_accounting_missing_snapshot_skip", "order_id", order.ID, "order_no", order.OrderNo)
		return nil
	}
	if !snapshot.ProfitEligible {
		return nil
	}
	profit := snapshot.ProfitAmount.Decimal.Round(2)
	if profit.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	// 分销订单通道费分摊：按在线实付金额的比例从代理利润中扣除。
	// 客户已加收手续费的支付（FeePolicy 为加收）不再重复扣；钱包支付没走网关，payment 为 nil 同样跳过。
	// 扣额封顶到当单利润，保证代理利润不会被扣成负数，超出部分由主站承担。
	grossProfit := profit
	channelFeeBase := decimal.Zero
	channelFee := decimal.Zero
	if s.channelFeePercent.GreaterThan(decimal.Zero) && payment != nil && !isCustomerPaidFeePolicy(payment.FeePolicy) {
		channelFeeBase = order.OnlinePaidAmount.Decimal.Round(2)
		if channelFeeBase.GreaterThan(decimal.Zero) {
			channelFee = channelFeeBase.Mul(s.channelFeePercent).Div(decimal.NewFromInt(100)).Round(2)
			if channelFee.GreaterThan(profit) {
				channelFee = profit
			}
			profit = profit.Sub(channelFee).Round(2)
		}
	}
	now := time.Now()
	availableAt := now.AddDate(0, 0, s.confirmDays)
	orderID := order.ID
	metadata := jsonmap.JSON{
		"order_no":            order.OrderNo,
		"reseller_domain":     snapshot.Domain,
		"wallet_paid_amount":  order.WalletPaidAmount.String(),
		"online_paid_amount":  order.OnlinePaidAmount.String(),
		"snapshot_id":         snapshot.ID,
		"profit_block_reason": snapshot.ProfitBlockReason,
	}
	if s.channelFeePercent.GreaterThan(decimal.Zero) {
		metadata["gross_profit_amount"] = grossProfit.StringFixed(2)
		metadata["channel_fee_percent"] = s.channelFeePercent.String()
		metadata["channel_fee_base"] = channelFeeBase.StringFixed(2)
		metadata["channel_fee_amount"] = channelFee.StringFixed(2)
	}
	if payment != nil {
		metadata["payment_id"] = payment.ID
		metadata["payment_channel_id"] = payment.ChannelID
		metadata["payment_amount"] = payment.Amount.String()
		metadata["payment_status"] = payment.Status
	}
	entry := &resellerdomain.LedgerEntry{
		ResellerID:     snapshot.ResellerID,
		OrderID:        &orderID,
		Type:           resellerdomain.LedgerTypeOrderProfit,
		Amount:         money.FromDecimal(profit),
		Currency:       strings.TrimSpace(snapshot.Currency),
		IdempotencyKey: fmt.Sprintf("order_profit:%d", order.ID),
		MetadataJSON:   metadata,
		Status:         resellerdomain.LedgerStatusPendingConfirm,
		AvailableAt:    &availableAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if entry.Currency == "" {
		entry.Currency = strings.TrimSpace(order.Currency)
	}
	if entry.Currency == "" {
		return resellercontract.ErrLedgerInvalidSnapshot
	}
	created, err := store.CreateLedgerEntryIfNotExists(entry)
	if err != nil {
		return err
	}
	if !created {
		return nil
	}
	return RefreshBalanceAccount(store, snapshot.ResellerID, entry.Currency, now)
}

// PostOrderProfitForOrder 在订单状态流转事务中入账分销利润。
// 订单用例不需要感知尚未迁移的支付持久化实体。
func (s *AccountingLedgerService) PostOrderProfitForOrder(store resellercontract.AccountingLedgerStore, order *orderdomain.Order) error {
	return s.PostOrderProfit(store, order, nil)
}

// ConfirmDueLedgerEntries 将到期待确认流水转为可用并刷新余额缓存。
func (s *AccountingLedgerService) ConfirmDueLedgerEntries(now time.Time) (int64, error) {
	if s == nil || s.store == nil {
		return 0, nil
	}
	var affected int64
	err := s.store.WithinLedgerTransaction(func(store resellercontract.AccountingLedgerStore) error {
		scopes, err := store.ListDueLedgerScopes(now)
		if err != nil {
			return err
		}
		marked, err := store.MarkDueLedgerEntriesAvailable(now)
		if err != nil {
			return err
		}
		affected = marked
		for _, scope := range scopes {
			if err := RefreshBalanceAccount(store, scope.ResellerID, scope.Currency, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return affected, nil
}

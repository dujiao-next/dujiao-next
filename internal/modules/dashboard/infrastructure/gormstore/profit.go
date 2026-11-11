package gormstore

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"

	dashboard "github.com/dujiao-next/internal/modules/dashboard/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
)

type refundAdjustmentRow struct {
	Day                string
	RefundAmount       float64
	RefundedCost       float64
	PaymentFeeRefunded float64
}

type refundAggregateRow struct {
	Day                string
	OrderID            uint
	RefundAmount       float64 `gorm:"column:refund_amount"`
	PaymentFeeRefunded float64 `gorm:"column:payment_fee_refunded"`
}

// rechargeFeeRefundRow 按日汇总充值退款退回的渠道手续费。
type rechargeFeeRefundRow struct {
	Day                string
	PaymentFeeRefunded float64
}

func profitOrderStatuses() []string {
	statuses := append([]string{}, paidOrderStatuses()...)
	return append(statuses, constants.OrderStatusRefunded)
}

// getRefundAdjustments 按退款发生日计算退款金额与同比例冲回的成本。
// 父订单本身没有订单项，因此其成本基数包含自身及直接子订单的订单项成本。
func (r *Store) getRefundAdjustments(startAt, endAt time.Time) ([]refundAdjustmentRow, error) {
	refundDayExpr := dateGroupExpr(r.db, "order_refund_records.created_at", startAt.Location(), startAt)
	refundRows := make([]refundAggregateRow, 0)
	timeWhere, timeArgs := timeRangeQuery(r.db, "order_refund_records.created_at", startAt, endAt)
	if err := r.db.Model(&orderdomain.OrderRefundRecord{}).
		Select(fmt.Sprintf(`
			%s as day,
			order_refund_records.order_id as order_id,
			COALESCE(SUM(order_refund_records.amount), 0) as refund_amount,
			COALESCE(SUM(order_refund_records.payment_fee_refunded_amount), 0) as payment_fee_refunded
		`, refundDayExpr)).
		Where("order_refund_records.deleted_at IS NULL AND "+timeWhere, timeArgs...).
		Group(fmt.Sprintf("%s, order_refund_records.order_id", refundDayExpr)).
		Scan(&refundRows).Error; err != nil {
		return nil, err
	}
	if len(refundRows) == 0 {
		return []refundAdjustmentRow{}, nil
	}

	orderIDs := make([]uint, 0, len(refundRows))
	seenOrderIDs := make(map[uint]struct{}, len(refundRows))
	for _, row := range refundRows {
		if row.OrderID == 0 {
			continue
		}
		if _, exists := seenOrderIDs[row.OrderID]; exists {
			continue
		}
		seenOrderIDs[row.OrderID] = struct{}{}
		orderIDs = append(orderIDs, row.OrderID)
	}

	type refundOrderRow struct {
		ID          uint
		TotalAmount float64 `gorm:"column:total_amount"`
	}
	orderRows := make([]refundOrderRow, 0, len(orderIDs))
	if err := r.db.Model(&orderdomain.Order{}).
		Select("id, total_amount").
		Where("deleted_at IS NULL AND id IN ?", orderIDs).
		Scan(&orderRows).Error; err != nil {
		return nil, err
	}
	orderTotalByID := make(map[uint]float64, len(orderRows))
	for _, row := range orderRows {
		orderTotalByID[row.ID] = row.TotalAmount
	}

	type orderCostRow struct {
		OrderID   uint
		ParentID  *uint
		TotalCost float64 `gorm:"column:total_cost"`
	}
	costRows := make([]orderCostRow, 0)
	if err := r.db.Model(&orderdomain.OrderItem{}).
		Select(`
			orders.id as order_id,
			orders.parent_id as parent_id,
			COALESCE(SUM(order_items.cost_price * order_items.quantity), 0) as total_cost
		`).
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("order_items.deleted_at IS NULL AND orders.deleted_at IS NULL AND (orders.id IN ? OR orders.parent_id IN ?)", orderIDs, orderIDs).
		Group("orders.id, orders.parent_id").
		Scan(&costRows).Error; err != nil {
		return nil, err
	}
	directCostByOrderID := make(map[uint]float64, len(costRows))
	childCostByParentID := make(map[uint]float64, len(costRows))
	for _, row := range costRows {
		directCostByOrderID[row.OrderID] += row.TotalCost
		if row.ParentID != nil {
			childCostByParentID[*row.ParentID] += row.TotalCost
		}
	}

	adjustments := make([]refundAdjustmentRow, 0, len(refundRows))
	for _, row := range refundRows {
		adjustment := refundAdjustmentRow{
			Day:                row.Day,
			RefundAmount:       row.RefundAmount,
			PaymentFeeRefunded: row.PaymentFeeRefunded,
		}
		orderTotal := orderTotalByID[row.OrderID]
		if orderTotal > 0 && row.RefundAmount > 0 {
			costBasis := directCostByOrderID[row.OrderID] + childCostByParentID[row.OrderID]
			adjustment.RefundedCost = costBasis * row.RefundAmount / orderTotal
		}
		adjustments = append(adjustments, adjustment)
	}
	return adjustments, nil
}

// getRechargeFeeRefunds 按退款发生日汇总充值退回的渠道手续费，不调整商品销售收入和商品成本。
func (r *Store) getRechargeFeeRefunds(startAt, endAt time.Time) ([]rechargeFeeRefundRow, error) {
	dayExpr := dateGroupExpr(r.db, "wallet_transactions.created_at", startAt.Location(), startAt)
	timeWhere, timeArgs := timeRangeQuery(r.db, "wallet_transactions.created_at", startAt, endAt)
	rows := make([]rechargeFeeRefundRow, 0)
	err := r.db.Table("wallet_transactions").
		Select(fmt.Sprintf("%s as day, COALESCE(SUM(payment_fee_refunded_amount), 0) as payment_fee_refunded", dayExpr)).
		Where("deleted_at IS NULL AND recharge_order_id IS NOT NULL AND type = ? AND payment_fee_refunded = ?", constants.WalletTxnTypeAdminRefund, true).
		Where(timeWhere, timeArgs...).Group(dayExpr).Scan(&rows).Error
	return rows, err
}

// GetProfitOverview 获取利润总览统计
func (r *Store) GetProfitOverview(startAt, endAt time.Time) (dashboard.ProfitOverviewRow, error) {
	result := dashboard.ProfitOverviewRow{}
	timeWhere, timeArgs := timeRangeQuery(r.db, "orders.created_at", startAt, endAt)
	args := append(timeArgs, profitOrderStatuses())
	if err := r.db.Model(&orderdomain.OrderItem{}).
		Select(`
			COALESCE(SUM(order_items.total_price - order_items.coupon_discount), 0) as total_revenue,
			COALESCE(SUM(order_items.cost_price * order_items.quantity), 0) as total_cost
		`).
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("order_items.deleted_at IS NULL AND orders.deleted_at IS NULL AND "+timeWhere+" AND orders.status IN ?", args...).
		Scan(&result).Error; err != nil {
		return result, err
	}

	refundAdjustments, err := r.getRefundAdjustments(startAt, endAt)
	if err != nil {
		return result, err
	}
	paymentFeeRefunded := 0.0
	for _, adjustment := range refundAdjustments {
		result.TotalRevenue -= adjustment.RefundAmount
		result.RefundedCost += adjustment.RefundedCost
		paymentFeeRefunded += adjustment.PaymentFeeRefunded
	}
	paymentTimeWhere, paymentTimeArgs := timeRangeQuery(r.db, "created_at", startAt, endAt)
	paymentArgs := append(paymentTimeArgs, constants.PaymentStatusSuccess, constants.PaymentProviderWallet, constants.PaymentFeePolicyMerchantAbsorbed)
	if err := r.db.Model(&paymentdomain.Payment{}).
		Select("COALESCE(SUM(fee_amount), 0)").
		Where("deleted_at IS NULL AND "+paymentTimeWhere+" AND status = ? AND provider_type <> ? AND fee_policy = ?", paymentArgs...).
		Scan(&result.PaymentFee).Error; err != nil {
		return result, err
	}
	rechargeFees, err := r.getRechargeFeeRefunds(startAt, endAt)
	if err != nil {
		return result, err
	}
	for _, row := range rechargeFees {
		paymentFeeRefunded += row.PaymentFeeRefunded
	}
	result.PaymentFee -= paymentFeeRefunded
	return result, nil
}

// GetProfitTrends 获取利润趋势
func (r *Store) GetProfitTrends(startAt, endAt time.Time) ([]dashboard.ProfitTrendRow, error) {
	orderDayExpr := dateGroupExpr(r.db, "orders.created_at", startAt.Location(), startAt)

	rows := make([]dashboard.ProfitTrendRow, 0)
	timeWhere, timeArgs := timeRangeQuery(r.db, "orders.created_at", startAt, endAt)
	args := append(timeArgs, profitOrderStatuses())
	if err := r.db.Model(&orderdomain.OrderItem{}).Select(fmt.Sprintf(`
		%s as day,
		COALESCE(SUM(order_items.total_price - order_items.coupon_discount), 0) as revenue,
		COALESCE(SUM(order_items.cost_price * order_items.quantity), 0) as cost
	`, orderDayExpr)).
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("order_items.deleted_at IS NULL AND orders.deleted_at IS NULL AND "+timeWhere+" AND orders.status IN ?", args...).
		Group(orderDayExpr).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	refundRows, err := r.getRefundAdjustments(startAt, endAt)
	if err != nil {
		return nil, err
	}

	type paymentFeeTrendRow struct {
		Day        string
		PaymentFee float64 `gorm:"column:payment_fee"`
	}
	paymentFeeRows := make([]paymentFeeTrendRow, 0)
	paymentFeeDayExpr := dateGroupExpr(r.db, "payments.created_at", startAt.Location(), startAt)
	paymentTimeWhere, paymentTimeArgs := timeRangeQuery(r.db, "created_at", startAt, endAt)
	paymentArgs := append(paymentTimeArgs, constants.PaymentStatusSuccess, constants.PaymentProviderWallet, constants.PaymentFeePolicyMerchantAbsorbed)
	if err := r.db.Model(&paymentdomain.Payment{}).
		Select(fmt.Sprintf(`
			%s as day,
			COALESCE(SUM(fee_amount), 0) as payment_fee
		`, paymentFeeDayExpr)).
		Where("deleted_at IS NULL AND "+paymentTimeWhere+" AND status = ? AND provider_type <> ? AND fee_policy = ?", paymentArgs...).
		Group(paymentFeeDayExpr).
		Scan(&paymentFeeRows).Error; err != nil {
		return nil, err
	}

	byDay := make(map[string]dashboard.ProfitTrendRow, len(rows)+len(refundRows))
	for _, row := range rows {
		byDay[row.Day] = row
	}
	for _, refundRow := range refundRows {
		day := refundRow.Day
		if day == "" {
			continue
		}
		row := byDay[day]
		row.Day = day
		row.Revenue -= refundRow.RefundAmount
		row.RefundedCost += refundRow.RefundedCost
		row.PaymentFee -= refundRow.PaymentFeeRefunded
		byDay[day] = row
	}
	rechargeFees, err := r.getRechargeFeeRefunds(startAt, endAt)
	if err != nil {
		return nil, err
	}
	for _, refund := range rechargeFees {
		day := strings.TrimSpace(refund.Day)
		if day == "" {
			continue
		}
		row := byDay[day]
		row.Day = day
		row.PaymentFee -= refund.PaymentFeeRefunded
		byDay[day] = row
	}
	for _, paymentFeeRow := range paymentFeeRows {
		day := strings.TrimSpace(paymentFeeRow.Day)
		if day == "" {
			continue
		}
		row := byDay[day]
		row.Day = day
		row.PaymentFee += paymentFeeRow.PaymentFee
		byDay[day] = row
	}

	merged := make([]dashboard.ProfitTrendRow, 0, len(byDay))
	for _, row := range byDay {
		merged = append(merged, row)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Day < merged[j].Day
	})
	return merged, nil
}

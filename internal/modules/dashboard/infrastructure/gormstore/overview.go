package gormstore

import (
	"fmt"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	dashboard "github.com/dujiao-next/internal/modules/dashboard/contract"
)

// GetOverview 获取总览统计
func (r *Store) GetOverview(startAt, endAt time.Time) (dashboard.OverviewRow, error) {
	result := dashboard.OverviewRow{}

	// 订单聚合：将 6 个串行 COUNT/SUM 查询合并为 1 个
	paidIn := quotedStatusList(paidOrderStatuses())
	processingStatuses := []string{
		constants.OrderStatusPaid,
		constants.OrderStatusFulfilling,
		constants.OrderStatusPartiallyDelivered,
		constants.OrderStatusDelivered,
	}
	processingIn := quotedStatusList(processingStatuses)

	var orderAgg struct {
		OrdersTotal          int64   `gorm:"column:orders_total"`
		PaidOrders           int64   `gorm:"column:paid_orders"`
		CompletedOrders      int64   `gorm:"column:completed_orders"`
		PendingPaymentOrders int64   `gorm:"column:pending_payment_orders"`
		ProcessingOrders     int64   `gorm:"column:processing_orders"`
		GMVPaid              float64 `gorm:"column:gmv_paid"`
	}
	orderSelectSQL := fmt.Sprintf(`
		COUNT(*) as orders_total,
		COALESCE(SUM(CASE WHEN status IN (%s) THEN 1 ELSE 0 END), 0) as paid_orders,
		COALESCE(SUM(CASE WHEN status = '%s' THEN 1 ELSE 0 END), 0) as completed_orders,
		COALESCE(SUM(CASE WHEN status = '%s' THEN 1 ELSE 0 END), 0) as pending_payment_orders,
		COALESCE(SUM(CASE WHEN status IN (%s) THEN 1 ELSE 0 END), 0) as processing_orders,
		COALESCE(SUM(CASE WHEN status IN (%s) THEN total_amount ELSE 0 END), 0) as gmv_paid
	`, paidIn, constants.OrderStatusCompleted, constants.OrderStatusPendingPayment, processingIn, paidIn)

	timeWhere, timeArgs := timeRangeQuery(r.db, "created_at", startAt, endAt)
	if err := r.db.Model(&orderdomain.Order{}).
		Select(orderSelectSQL).
		Where("deleted_at IS NULL AND parent_id IS NULL AND "+timeWhere, timeArgs...).
		Scan(&orderAgg).Error; err != nil {
		return result, err
	}
	result.OrdersTotal = orderAgg.OrdersTotal
	result.PaidOrders = orderAgg.PaidOrders
	result.CompletedOrders = orderAgg.CompletedOrders
	result.PendingPaymentOrders = orderAgg.PendingPaymentOrders
	result.ProcessingOrders = orderAgg.ProcessingOrders
	result.GMVPaid = orderAgg.GMVPaid

	// 支付聚合：将 3 个串行 COUNT 查询合并为 1 个
	var paymentAgg struct {
		PaymentsTotal   int64 `gorm:"column:payments_total"`
		PaymentsSuccess int64 `gorm:"column:payments_success"`
		PaymentsFailed  int64 `gorm:"column:payments_failed"`
	}
	paymentSelectSQL := fmt.Sprintf(`
		COUNT(*) as payments_total,
		COALESCE(SUM(CASE WHEN status = '%s' THEN 1 ELSE 0 END), 0) as payments_success,
		COALESCE(SUM(CASE WHEN status = '%s' THEN 1 ELSE 0 END), 0) as payments_failed
	`, constants.PaymentStatusSuccess, constants.PaymentStatusFailed)

	if err := onlinePaymentBase(r.db, startAt, endAt).
		Select(paymentSelectSQL).
		Scan(&paymentAgg).Error; err != nil {
		return result, err
	}
	result.PaymentsTotal = paymentAgg.PaymentsTotal
	result.PaymentsSuccess = paymentAgg.PaymentsSuccess
	result.PaymentsFailed = paymentAgg.PaymentsFailed

	timeWhere2, timeArgs2 := timeRangeQuery(r.db, "created_at", startAt, endAt)
	if err := r.db.Model(&userdomain.User{}).
		Where("deleted_at IS NULL AND "+timeWhere2, timeArgs2...).
		Count(&result.NewUsers).Error; err != nil {
		return result, err
	}

	if err := r.db.Model(&productdomain.Product{}).
		Where("deleted_at IS NULL AND is_active = ?", true).
		Count(&result.ActiveProducts).Error; err != nil {
		return result, err
	}

	timeWhere3, timeArgs3 := timeRangeQuery(r.db, "created_at", startAt, endAt)
	_ = r.db.Model(&orderdomain.Order{}).
		Where("deleted_at IS NULL AND parent_id IS NULL AND "+timeWhere3+" AND currency <> ''", timeArgs3...).
		Order("id DESC").
		Limit(1).
		Pluck("currency", &result.Currency).Error
	// 时间范围内无订单时，回退到最近一笔订单的币种
	if result.Currency == "" {
		_ = r.db.Model(&orderdomain.Order{}).
			Where("deleted_at IS NULL AND parent_id IS NULL AND currency <> ''").
			Order("id DESC").
			Limit(1).
			Pluck("currency", &result.Currency).Error
	}

	return result, nil
}

// GetPaymentOrderAlertCounts 获取支付订单告警计数
func (r *Store) GetPaymentOrderAlertCounts(startAt, endAt time.Time) (dashboard.PaymentOrderAlertCountsRow, error) {
	result := dashboard.PaymentOrderAlertCountsRow{}

	timeWhere, timeArgs := timeRangeQuery(r.db, "created_at", startAt, endAt)
	if err := r.db.Model(&orderdomain.Order{}).
		Where("deleted_at IS NULL AND parent_id IS NULL AND status = ? AND "+timeWhere, append([]interface{}{constants.OrderStatusPendingPayment}, timeArgs...)...).
		Count(&result.PendingPaymentOrders).Error; err != nil {
		return result, err
	}

	if err := onlinePaymentBase(r.db, startAt, endAt).
		Where("status = ?", constants.PaymentStatusFailed).
		Count(&result.PaymentsFailed).Error; err != nil {
		return result, err
	}

	return result, nil
}

// GetTotalUserBalance 获取全站用户余额总数
func (r *Store) GetTotalUserBalance() (float64, error) {
	var total float64
	if err := r.db.Model(&walletdomain.Account{}).
		Where("deleted_at IS NULL").
		Select("COALESCE(SUM(balance), 0)").
		Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

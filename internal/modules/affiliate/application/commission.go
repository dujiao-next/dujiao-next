package application

import (
	"strconv"
	"strings"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// HandleOrderPaid 处理订单支付成功后的佣金生成
func (s *Service) HandleOrderPaid(orderID uint) error {
	if orderID == 0 || s.repo == nil || s.orderRepo == nil {
		return nil
	}
	setting, err := s.settings.GetAffiliateSetting()
	if err != nil {
		return err
	}
	if !setting.Enabled || setting.CommissionRate <= 0 {
		return nil
	}

	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return err
	}
	// 重试任务可能晚于退款/取消执行，此时退款流程已找不到佣金可冲销，不能再生成
	if order == nil || order.Status == constants.OrderStatusRefunded || order.Status == constants.OrderStatusCanceled {
		return nil
	}
	profile, err := s.resolveAffiliateProfileForOrder(order)
	if err != nil {
		return err
	}
	if profile == nil {
		return nil
	}
	if strings.TrimSpace(profile.Status) != constants.AffiliateProfileStatusActive {
		return nil
	}
	if order.UserID > 0 && profile.UserID == order.UserID {
		return nil
	}

	commissionType := constants.AffiliateCommissionTypeOrder
	existing, err := s.repo.GetCommissionByOrderAndProfile(order.ID, profile.ID, commissionType)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	baseAmount, err := s.calculateCommissionBaseAmount(order)
	if err != nil {
		return err
	}
	if baseAmount.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	rate := decimal.NewFromFloat(setting.CommissionRate).Round(2)
	commissionAmount := baseAmount.Mul(rate).Div(decimal.NewFromInt(100)).Round(2)
	if commissionAmount.LessThanOrEqual(decimal.Zero) {
		return nil
	}

	paidAt := time.Now()
	if order.PaidAt != nil {
		paidAt = *order.PaidAt
	}
	status := constants.AffiliateCommissionStatusPendingConfirm
	var confirmAt *time.Time
	var availableAt *time.Time
	if setting.ConfirmDays <= 0 {
		status = constants.AffiliateCommissionStatusAvailable
		availableAt = &paidAt
	} else {
		t := paidAt.Add(time.Duration(setting.ConfirmDays) * 24 * time.Hour)
		confirmAt = &t
	}

	commission := &affiliatedomain.Commission{
		AffiliateProfileID: profile.ID,
		OrderID:            order.ID,
		CommissionType:     commissionType,
		BaseAmount:         money.FromDecimal(baseAmount),
		RatePercent:        money.FromDecimal(rate),
		CommissionAmount:   money.FromDecimal(commissionAmount),
		Status:             status,
		ConfirmAt:          confirmAt,
		AvailableAt:        availableAt,
	}
	return s.repo.CreateCommission(commission)
}

// ConfirmDueCommissions 将到期佣金转可提现
func (s *Service) ConfirmDueCommissions(now time.Time) error {
	if s.repo == nil {
		return nil
	}
	_, err := s.repo.MarkPendingCommissionsAvailable(now, now)
	return err
}

// HandleOrderCanceled 处理订单取消/退款后的佣金逆向
func (s *Service) HandleOrderCanceled(orderID uint, reason string) error {
	if orderID == 0 || s.repo == nil {
		return nil
	}
	rows, err := s.repo.ListCommissionsByOrder(orderID, []string{
		constants.AffiliateCommissionStatusPendingConfirm,
		constants.AffiliateCommissionStatusAvailable,
	})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	now := time.Now()
	reasonText := strings.TrimSpace(reason)
	if reasonText == "" {
		reasonText = "order_canceled"
	}
	for i := range rows {
		item := rows[i]
		if item.WithdrawRequestID != nil {
			// 已进入提现流程，按业务规则不影响用户提现。
			continue
		}
		item.Status = constants.AffiliateCommissionStatusRejected
		item.InvalidReason = reasonText
		item.UpdatedAt = now
		if err := s.repo.UpdateCommission(&item); err != nil {
			return err
		}
	}
	return nil
}

// HandleOrderRefunded 使用调用方提供的事务 Store 处理退款后的佣金回滚。
func (s *Service) HandleOrderRefunded(
	repoTx affiliatecontract.Store,
	order *orderdomain.Order,
	refundDelta decimal.Decimal,
	refundedBefore decimal.Decimal,
	reason string,
) error {
	if repoTx == nil || order == nil || order.ID == 0 {
		return nil
	}
	delta := refundDelta.Round(2)
	if delta.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	totalAmount := order.TotalAmount.Decimal.Round(2)
	if totalAmount.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	before := refundedBefore.Round(2)
	if before.LessThan(decimal.Zero) {
		before = decimal.Zero
	}
	if before.GreaterThan(totalAmount) {
		before = totalAmount
	}
	remaining := totalAmount.Sub(before).Round(2)
	if remaining.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	if delta.GreaterThan(remaining) {
		delta = remaining
	}

	rows, err := repoTx.ListCommissionsByOrderForUpdate(order.ID, []string{
		constants.AffiliateCommissionStatusPendingConfirm,
		constants.AffiliateCommissionStatusAvailable,
	})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	now := time.Now()
	reasonText := strings.TrimSpace(reason)
	if reasonText == "" {
		reasonText = "order_refunded"
	}
	for i := range rows {
		item := rows[i]
		if item.WithdrawRequestID != nil {
			// 已进入提现流程，按业务规则不影响用户提现。
			continue
		}

		currentCommission := item.CommissionAmount.Decimal.Round(2)
		if currentCommission.LessThanOrEqual(decimal.Zero) {
			item.Status = constants.AffiliateCommissionStatusRejected
			item.InvalidReason = reasonText
			item.ConfirmAt = nil
			item.AvailableAt = nil
			item.UpdatedAt = now
			if err := repoTx.UpdateCommission(&item); err != nil {
				return err
			}
			continue
		}

		// 按“本次退款金额 / 当前剩余未退款金额”比例扣减当前佣金，避免多次退款时重复放大扣减。
		deduct := currentCommission.Mul(delta).Div(remaining).Round(2)
		nextCommission := currentCommission.Sub(deduct).Round(2)
		if nextCommission.LessThan(decimal.Zero) {
			nextCommission = decimal.Zero
		}
		currentBase := item.BaseAmount.Decimal.Round(2)
		nextBase := currentBase
		if currentBase.GreaterThan(decimal.Zero) {
			baseDeduct := currentBase.Mul(delta).Div(remaining).Round(2)
			nextBase = currentBase.Sub(baseDeduct).Round(2)
			if nextBase.LessThan(decimal.Zero) {
				nextBase = decimal.Zero
			}
		}

		item.CommissionAmount = money.FromDecimal(nextCommission)
		item.BaseAmount = money.FromDecimal(nextBase)
		item.UpdatedAt = now
		if nextCommission.LessThanOrEqual(decimal.Zero) {
			item.Status = constants.AffiliateCommissionStatusRejected
			item.InvalidReason = reasonText
			item.ConfirmAt = nil
			item.AvailableAt = nil
		}
		if err := repoTx.UpdateCommission(&item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) resolveAffiliateProfileForOrder(order *orderdomain.Order) (*affiliatedomain.Profile, error) {
	if order == nil || s.repo == nil {
		return nil, nil
	}
	if order.AffiliateProfileID != nil && *order.AffiliateProfileID > 0 {
		return s.repo.GetProfileByID(*order.AffiliateProfileID)
	}
	if strings.TrimSpace(order.AffiliateCode) != "" {
		return s.repo.GetProfileByCode(order.AffiliateCode)
	}
	return nil, nil
}

func (s *Service) calculateCommissionBaseAmount(order *orderdomain.Order) (decimal.Decimal, error) {
	if order == nil || s.productRepo == nil {
		return decimal.Zero, nil
	}
	productIDs := collectAffiliateProductIDs(order)
	if len(productIDs) == 0 {
		return decimal.Zero, nil
	}
	products, err := s.productRepo.ListByIDs(productIDs)
	if err != nil {
		return decimal.Zero, err
	}
	productMap := make(map[uint]productdomain.Product, len(products))
	for _, product := range products {
		productMap[product.ID] = product
	}

	targetOrders := order.Children
	if len(targetOrders) == 0 {
		targetOrders = []orderdomain.Order{*order}
	}

	total := decimal.Zero
	for _, current := range targetOrders {
		for _, item := range current.Items {
			product, ok := productMap[item.ProductID]
			if !ok || !product.IsAffiliateEnabled {
				continue
			}
			payable := item.TotalPrice.Decimal.Sub(item.CouponDiscount.Decimal).Round(2)
			if payable.LessThan(decimal.Zero) {
				payable = decimal.Zero
			}
			total = total.Add(payable).Round(2)
		}
	}
	return total, nil
}

func collectAffiliateProductIDs(order *orderdomain.Order) []uint {
	if order == nil {
		return nil
	}
	ids := make([]uint, 0)
	seen := make(map[uint]struct{})
	appendItem := func(item orderdomain.OrderItem) {
		if item.ProductID == 0 {
			return
		}
		if _, ok := seen[item.ProductID]; ok {
			return
		}
		seen[item.ProductID] = struct{}{}
		ids = append(ids, item.ProductID)
	}
	for _, item := range order.Items {
		appendItem(item)
	}
	for _, child := range order.Children {
		for _, item := range child.Items {
			appendItem(item)
		}
	}
	return ids
}

func buildSplitCommissionType(sourceID uint) string {
	suffix := strconv.FormatInt(time.Now().UnixNano()%1000000, 10)
	base := affiliateSplitTypePrefix + strconv.FormatUint(uint64(sourceID), 36)
	result := base + suffix
	if len(result) > 20 {
		return result[:20]
	}
	return result
}

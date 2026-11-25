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
	// 全站比例可能为 0：此时仍要处理商品/SKU 级覆盖的订单项，不能提前返回。
	if !setting.Enabled {
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

	calculation, err := s.calculateCommission(order, setting.CommissionRate)
	if err != nil {
		return err
	}
	baseAmount := calculation.BaseAmount
	commissionAmount := calculation.CommissionAmount
	if commissionAmount.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	rate := calculation.RatePercent

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

// ResolveAffiliateRate 按 SKU → 商品 → 全站 的优先级解析返利比例（百分比）。
//
// 任一层显式配置了比例即停止向上继承：0 表示该层级单独关闭返利，
// 因此配置为 0 的 SKU 不会回退到商品或全站比例。
// 结果被约束在 0-100 之间；返回 0 时调用方应视为该订单项不参与返利。
func ResolveAffiliateRate(skuRate, productRate *money.Amount, globalRate decimal.Decimal) decimal.Decimal {
	// 商品显式设为 0 表示整个商品不参与返利，SKU 上残留的比例不再生效。
	if productRate != nil && clampAffiliateRate(productRate.Decimal).IsZero() {
		return decimal.Zero
	}
	if skuRate != nil {
		return clampAffiliateRate(skuRate.Decimal)
	}
	if productRate != nil {
		return clampAffiliateRate(productRate.Decimal)
	}
	return clampAffiliateRate(globalRate)
}

// clampAffiliateRate 把比例限制在 0-100 并保留两位小数。
func clampAffiliateRate(rate decimal.Decimal) decimal.Decimal {
	rate = rate.Round(2)
	if rate.LessThan(decimal.Zero) {
		return decimal.Zero
	}
	if rate.GreaterThan(decimal.NewFromInt(100)) {
		return decimal.NewFromInt(100)
	}
	return rate
}

// commissionCalculation 描述一次佣金计算的结果：可返利基数、佣金金额与加权平均比例。
type commissionCalculation struct {
	BaseAmount       decimal.Decimal
	CommissionAmount decimal.Decimal
	// RatePercent 是按各订单项金额加权后的平均比例，用于佣金记录快照展示。
	RatePercent decimal.Decimal
}

// calculateCommission 按订单项解析各自生效的返利比例并汇总佣金。
//
// 比例优先级为 SKU → 商品 → 全站；任一层显式配置 0 即表示该订单项不参与返利，
// 因此不会回退到更上层的非零比例。
func (s *Service) calculateCommission(order *orderdomain.Order, globalCommissionRate float64) (commissionCalculation, error) {
	result := commissionCalculation{}
	if order == nil || s.productRepo == nil {
		return result, nil
	}
	productIDs := collectAffiliateItemIDs(order, func(item orderdomain.OrderItem) uint { return item.ProductID })
	if len(productIDs) == 0 {
		return result, nil
	}
	products, err := s.productRepo.ListByIDs(productIDs)
	if err != nil {
		return result, err
	}
	productMap := make(map[uint]productdomain.Product, len(products))
	for _, product := range products {
		productMap[product.ID] = product
	}
	skuMap, err := s.loadAffiliateSKUMap(order)
	if err != nil {
		return result, err
	}

	globalRate := decimal.NewFromFloat(globalCommissionRate).Round(2)

	targetOrders := order.Children
	if len(targetOrders) == 0 {
		targetOrders = []orderdomain.Order{*order}
	}

	baseTotal := decimal.Zero
	// 逐项金额先不取整，汇总后只取整一次：每个订单项各自四舍五入会让小额多行订单累计偏差，
	// 也会让「记录里的加权比例」偏离实际比例（三项 0.33 元、比例 10% 时差 1 分钱）。
	commissionRaw := decimal.Zero
	for _, current := range targetOrders {
		for _, item := range current.Items {
			product, ok := productMap[item.ProductID]
			if !ok {
				continue
			}
			payable := item.TotalPrice.Decimal.Sub(item.CouponDiscount.Decimal).Round(2)
			if payable.LessThanOrEqual(decimal.Zero) {
				continue
			}
			var skuRate *money.Amount
			if item.SKUID > 0 {
				if sku, exists := skuMap[item.SKUID]; exists {
					skuRate = sku.AffiliateRatePercent
				}
			}
			rate := ResolveAffiliateRate(skuRate, product.AffiliateRatePercent, globalRate)
			if rate.LessThanOrEqual(decimal.Zero) {
				continue
			}
			baseTotal = baseTotal.Add(payable)
			commissionRaw = commissionRaw.Add(payable.Mul(rate).Div(decimal.NewFromInt(100)))
		}
	}

	baseTotal = baseTotal.Round(2)
	commissionTotal := commissionRaw.Round(2)

	result.BaseAmount = baseTotal
	result.CommissionAmount = commissionTotal
	if baseTotal.GreaterThan(decimal.Zero) {
		// 用未取整的佣金反推比例：拿取整后的佣金会让单一比例订单也漂移
		// （9.99 元 × 10% → 佣金 1.00 → 显示 10.01%）。
		result.RatePercent = commissionRaw.Div(baseTotal).Mul(decimal.NewFromInt(100)).Round(2)
	}
	return result, nil
}

// loadAffiliateSKUMap 批量读取订单项引用的 SKU，用于解析 SKU 级返利比例。
// 仅当订单中出现具体 SKU 时才查询；缺失的 SKU（已删除）会回退到商品级比例。
func (s *Service) loadAffiliateSKUMap(order *orderdomain.Order) (map[uint]productdomain.ProductSKU, error) {
	if order == nil || s.skuRepo == nil {
		return map[uint]productdomain.ProductSKU{}, nil
	}
	skuIDs := collectAffiliateItemIDs(order, func(item orderdomain.OrderItem) uint { return item.SKUID })
	if len(skuIDs) == 0 {
		return map[uint]productdomain.ProductSKU{}, nil
	}
	skus, err := s.skuRepo.ListByIDs(skuIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[uint]productdomain.ProductSKU, len(skus))
	for _, sku := range skus {
		result[sku.ID] = sku
	}
	return result, nil
}

// collectAffiliateItemIDs 收集订单（含子订单）订单项里去重后的非零 ID，pick 决定取商品 ID 还是 SKU ID。
func collectAffiliateItemIDs(order *orderdomain.Order, pick func(orderdomain.OrderItem) uint) []uint {
	if order == nil {
		return nil
	}
	ids := make([]uint, 0)
	seen := make(map[uint]struct{})
	appendItem := func(item orderdomain.OrderItem) {
		id := pick(item)
		if id == 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
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

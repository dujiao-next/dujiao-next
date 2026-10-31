package application

import (
	"strings"

	"github.com/dujiao-next/internal/constants"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/money"
)

// computeProductChannelIntersection 计算多个商品允许支付渠道的交集
// 空列表表示不限制（全部允许），不参与交集计算
// 返回 nil 表示无限制，返回空切片表示交集为空（无可用渠道）
func computeProductChannelIntersection(products []productdomain.Product) []uint {
	var intersection map[uint]struct{}
	hasRestriction := false

	for _, p := range products {
		allowed := productdomain.DecodePaymentChannelIDs(p.PaymentChannelIDs)
		if len(allowed) == 0 {
			continue // 该商品不限制
		}
		hasRestriction = true
		allowedSet := make(map[uint]struct{}, len(allowed))
		for _, id := range allowed {
			allowedSet[id] = struct{}{}
		}
		if intersection == nil {
			intersection = allowedSet
		} else {
			for id := range intersection {
				if _, ok := allowedSet[id]; !ok {
					delete(intersection, id)
				}
			}
		}
	}

	if !hasRestriction {
		return nil // 所有商品都不限制
	}

	result := make([]uint, 0, len(intersection))
	for id := range intersection {
		result = append(result, id)
	}
	return result
}

// validateProductPaymentChannel 校验支付渠道是否被订单中的商品允许
// store 参数可选：在事务内调用时传入事务绑定的商品 Store，避免脱离工作单元。
func (s *PaymentService) validateProductPaymentChannel(items []orderdomain.OrderItem, channelID uint, stores ...productcontract.Repository) error {
	if len(items) == 0 {
		return nil
	}
	productIDSet := make(map[uint]struct{})
	for _, item := range items {
		if item.ProductID > 0 {
			productIDSet[item.ProductID] = struct{}{}
		}
	}
	if len(productIDSet) == 0 {
		return nil
	}
	productIDs := make([]uint, 0, len(productIDSet))
	for id := range productIDSet {
		productIDs = append(productIDs, id)
	}
	var repo productcontract.Repository = s.productRepo
	if len(stores) > 0 && stores[0] != nil {
		repo = stores[0]
	}
	products, err := repo.ListByIDs(productIDs)
	if err != nil {
		return ErrProductFetchFailed
	}
	allowed := computeProductChannelIntersection(products)
	if allowed == nil {
		return nil // 无限制
	}
	for _, id := range allowed {
		if id == channelID {
			return nil
		}
	}
	return ErrPaymentChannelNotAllowedForProduct
}

// validateWalletRechargeChannel 校验支付渠道是否被钱包充值设置允许
func (s *PaymentService) validateWalletRechargeChannel(channelID uint) error {
	if s.settingService == nil {
		return nil
	}
	allowedIDs := s.settingService.GetWalletRechargeChannelIDs()
	if len(allowedIDs) == 0 {
		return nil // 无限制
	}
	for _, id := range allowedIDs {
		if id == channelID {
			return nil
		}
	}
	return ErrPaymentChannelNotAllowedForRecharge
}

// GetAllowedChannelsForProducts 获取商品允许的支付渠道列表
func (s *PaymentService) GetAllowedChannelsForProducts(productIDs []uint) ([]paymentdomain.PaymentChannel, error) {
	// 仅钱包余额支付模式下不返回任何在线支付渠道
	if s.settingService != nil && s.settingService.GetWalletOnlyPayment() {
		return []paymentdomain.PaymentChannel{}, nil
	}
	channels, _, err := s.ListChannels(paymentcontract.ChannelListFilter{
		Page:       1,
		PageSize:   200,
		ActiveOnly: true,
	})
	if err != nil {
		return nil, err
	}
	if len(productIDs) == 0 {
		return channels, nil
	}
	products, err := s.productRepo.ListByIDs(productIDs)
	if err != nil {
		return nil, ErrProductFetchFailed
	}
	allowed := computeProductChannelIntersection(products)
	if allowed == nil {
		return channels, nil // 无限制
	}
	allowedSet := make(map[uint]struct{}, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = struct{}{}
	}
	filtered := make([]paymentdomain.PaymentChannel, 0, len(allowed))
	for _, ch := range channels {
		if _, ok := allowedSet[ch.ID]; ok {
			filtered = append(filtered, ch)
		}
	}
	return filtered, nil
}

// GetWalletRechargeChannels 获取钱包充值允许的支付渠道列表
func (s *PaymentService) GetWalletRechargeChannels() ([]paymentdomain.PaymentChannel, error) {
	channels, _, err := s.ListChannels(paymentcontract.ChannelListFilter{
		Page:       1,
		PageSize:   200,
		ActiveOnly: true,
	})
	if err != nil {
		return nil, err
	}
	filteredByProvider := make([]paymentdomain.PaymentChannel, 0, len(channels))
	for _, channel := range channels {
		if !isEpusdtProvider(channel.ProviderType) {
			filteredByProvider = append(filteredByProvider, channel)
		}
	}
	channels = filteredByProvider
	if s.settingService == nil {
		return channels, nil
	}
	allowedIDs := s.settingService.GetWalletRechargeChannelIDs()
	if len(allowedIDs) == 0 {
		return channels, nil
	}
	allowedSet := make(map[uint]struct{}, len(allowedIDs))
	for _, id := range allowedIDs {
		allowedSet[id] = struct{}{}
	}
	filtered := make([]paymentdomain.PaymentChannel, 0, len(allowedIDs))
	for _, ch := range channels {
		if _, ok := allowedSet[ch.ID]; ok {
			filtered = append(filtered, ch)
		}
	}
	return filtered, nil
}

// GetAllowedChannelIDsForOrder 获取订单中所有商品允许的支付渠道ID交集
func (s *PaymentService) GetAllowedChannelIDsForOrder(items []orderdomain.OrderItem) []uint {
	if len(items) == 0 {
		return nil
	}
	productIDSet := make(map[uint]struct{})
	for _, item := range items {
		if item.ProductID > 0 {
			productIDSet[item.ProductID] = struct{}{}
		}
	}
	if len(productIDSet) == 0 {
		return nil
	}
	productIDs := make([]uint, 0, len(productIDSet))
	for id := range productIDSet {
		productIDs = append(productIDs, id)
	}
	products, err := s.productRepo.ListByIDs(productIDs)
	if err != nil {
		return nil
	}
	return computeProductChannelIntersection(products)
}

// AvailablePaymentChannelFilter 可用支付渠道过滤参数。
type AvailablePaymentChannelFilter struct {
	TargetAmount *money.Amount
	User         *userdomain.User
	PaymentType  string
}

func (s *PaymentService) GetAvailableChannels(filter AvailablePaymentChannelFilter) ([]map[string]interface{}, error) {
	channels, _, err := s.ListChannels(paymentcontract.ChannelListFilter{
		Page: 1, PageSize: 200, ActiveOnly: true,
	})
	if err != nil {
		return nil, err
	}
	customerFeeEnabled := s.settingService != nil && s.settingService.GetPaymentFeeConfig().CustomerFeeEnabled
	available := make([]map[string]interface{}, 0, len(channels))
	for _, channel := range channels {
		if !matchesChannelAmount(channel, filter.TargetAmount) ||
			!matchesChannelRole(channel, filter.User) ||
			!matchesChannelMemberLevel(channel, filter.User) ||
			!matchesChannelPaymentType(channel, filter.PaymentType) {
			continue
		}
		item := map[string]interface{}{
			"id": channel.ID, "name": channel.Name,
			"provider_type": channel.ProviderType, "channel_type": channel.ChannelType,
			"interaction_mode": channel.InteractionMode, "min_amount": channel.MinAmount,
			"max_amount": channel.MaxAmount, "hide_amount_out_range": channel.HideAmountOutRange,
		}
		if customerFeeEnabled {
			item["fee_policy"] = constants.PaymentFeePolicyCustomerSurcharge
			item["fee_rate"] = channel.FeeRate
			item["fixed_fee"] = channel.FixedFee
		}
		if channel.Icon != "" {
			item["icon"] = channel.Icon
		}
		available = append(available, item)
	}
	return available, nil
}

func matchesChannelAmount(channel paymentdomain.PaymentChannel, targetAmount *money.Amount) bool {
	if targetAmount == nil || !channel.HideAmountOutRange {
		return true
	}
	amount := targetAmount.Decimal
	return (!channel.MinAmount.Decimal.IsPositive() || !amount.LessThan(channel.MinAmount.Decimal)) &&
		(!channel.MaxAmount.Decimal.IsPositive() || !amount.GreaterThan(channel.MaxAmount.Decimal))
}

func isEpusdtProvider(provider string) bool {
	return strings.EqualFold(strings.TrimSpace(provider), constants.PaymentProviderEpusdt)
}

func matchesChannelRole(channel paymentdomain.PaymentChannel, user *userdomain.User) bool {
	// Epusdt recovery is restricted to persisted member orders, even for legacy unrestricted channels.
	if isEpusdtProvider(channel.ProviderType) && (user == nil || user.ID == 0) {
		return false
	}
	if len(channel.PaymentRoles) == 0 {
		return true
	}
	targetRole := constants.PaymentRoleGuest
	if user != nil {
		targetRole = constants.PaymentRoleMember
	}
	for _, role := range channel.PaymentRoles {
		if role == targetRole {
			return true
		}
	}
	return false
}

func matchesChannelMemberLevel(channel paymentdomain.PaymentChannel, user *userdomain.User) bool {
	if len(channel.MemberLevels) == 0 {
		return true
	}
	if user == nil || user.MemberLevelID == 0 {
		return false
	}
	for _, levelID := range channel.MemberLevels {
		if levelID == user.MemberLevelID {
			return true
		}
	}
	return false
}

func matchesChannelPaymentType(channel paymentdomain.PaymentChannel, paymentType string) bool {
	if isEpusdtProvider(channel.ProviderType) && paymentType != "" && paymentType != constants.PaymentTypeOrder {
		return false
	}
	if len(channel.PaymentTypes) == 0 || paymentType == "" {
		return true
	}
	for _, candidate := range channel.PaymentTypes {
		if candidate == paymentType {
			return true
		}
	}
	return false
}

// validateOrderChannelEligibility 对写路径重复执行列表页使用的角色、会员等级和付款类型规则。
// 订单快照是授权事实，不能信任客户端提交的角色或等级。
func validateOrderChannelEligibility(channel paymentdomain.PaymentChannel, order *orderdomain.Order) error {
	if order == nil {
		return ErrPaymentInvalid
	}
	var user *userdomain.User
	if order.UserID > 0 {
		user = &userdomain.User{ID: order.UserID}
		if order.MemberLevelID != nil {
			user.MemberLevelID = *order.MemberLevelID
		}
	}
	if !matchesChannelRole(channel, user) ||
		!matchesChannelMemberLevel(channel, user) ||
		!matchesChannelPaymentType(channel, constants.PaymentTypeOrder) {
		return ErrPaymentChannelNotAllowedForProduct
	}
	return nil
}

func validateWalletChannelEligibility(channel paymentdomain.PaymentChannel, user *userdomain.User) error {
	if user == nil || user.ID == 0 {
		return ErrPaymentInvalid
	}
	if !matchesChannelRole(channel, user) ||
		!matchesChannelMemberLevel(channel, user) ||
		!matchesChannelPaymentType(channel, constants.PaymentTypeWallet) {
		return ErrPaymentChannelNotAllowedForRecharge
	}
	return nil
}

package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/shopspring/decimal"
)

// CouponService 优惠券服务
type CouponService struct {
	couponRepo repository.CouponRepository
	usageRepo  repository.CouponUsageRepository
}

type couponEligibility struct {
	subtotal models.Money
	quantity int
}

// NewCouponService 创建优惠券服务
func NewCouponService(couponRepo repository.CouponRepository, usageRepo repository.CouponUsageRepository) *CouponService {
	return &CouponService{
		couponRepo: couponRepo,
		usageRepo:  usageRepo,
	}
}

// ApplyCoupon 计算优惠券折扣金额
func (s *CouponService) ApplyCoupon(subtotal models.Money, code string, userID uint, items []models.OrderItem, isGuest bool, memberLevelID uint) (models.Money, *models.Coupon, error) {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return models.Money{}, nil, ErrCouponInvalid
	}

	coupon, err := s.couponRepo.GetByCode(trimmed)
	if err != nil {
		return models.Money{}, nil, err
	}
	if coupon == nil {
		return models.Money{}, nil, ErrCouponNotFound
	}
	if !coupon.IsActive {
		return models.Money{}, coupon, ErrCouponInactive
	}

	now := time.Now()
	if coupon.StartsAt != nil && now.Before(*coupon.StartsAt) {
		return models.Money{}, coupon, ErrCouponNotStarted
	}
	if coupon.EndsAt != nil && now.After(*coupon.EndsAt) {
		return models.Money{}, coupon, ErrCouponExpired
	}

	if coupon.UsageLimit > 0 && coupon.UsedCount >= coupon.UsageLimit {
		return models.Money{}, coupon, ErrCouponUsageLimit
	}
	if roleErr := resolveCouponPaymentRoleError(coupon, isGuest); roleErr != nil {
		return models.Money{}, coupon, roleErr
	}
	if !matchesCouponMemberLevel(coupon, memberLevelID) {
		return models.Money{}, coupon, ErrCouponMemberLevelNotAllowed
	}

	if coupon.PerUserLimit > 0 && userID != 0 {
		count, err := s.usageRepo.CountByUser(coupon.ID, userID)
		if err != nil {
			return models.Money{}, coupon, err
		}
		if int(count) >= coupon.PerUserLimit {
			return models.Money{}, coupon, ErrCouponPerUserLimit
		}
	}

	eligibility, err := s.resolveCouponEligibility(coupon, items)
	if err != nil {
		return models.Money{}, coupon, err
	}

	if eligibility.subtotal.Decimal.Cmp(coupon.MinAmount.Decimal) < 0 {
		return models.Money{}, coupon, ErrCouponMinAmount
	}

	discount, err := s.calculateDiscount(coupon, eligibility)
	if err != nil {
		return models.Money{}, coupon, err
	}

	if coupon.MaxDiscount.Decimal.GreaterThan(decimal.Zero) && discount.Decimal.GreaterThan(coupon.MaxDiscount.Decimal) {
		discount = models.NewMoneyFromDecimal(coupon.MaxDiscount.Decimal)
	}

	if discount.Decimal.GreaterThan(eligibility.subtotal.Decimal) {
		discount = models.NewMoneyFromDecimal(eligibility.subtotal.Decimal)
	}

	return discount, coupon, nil
}

// matchesCouponRole 判断当前下单角色是否满足优惠券付款角色限制；未配置限制时默认允许。
func matchesCouponRole(coupon *models.Coupon, isGuest bool) bool {
	if coupon == nil || len(coupon.PaymentRoles) == 0 {
		return true
	}
	targetRole := constants.PaymentRoleMember
	if isGuest {
		targetRole = constants.PaymentRoleGuest
	}
	for _, role := range coupon.PaymentRoles {
		if strings.EqualFold(strings.TrimSpace(role), targetRole) {
			return true
		}
	}
	return false
}

// resolveCouponPaymentRoleError 解析付款角色限制不满足时的业务错误。
// 当限制仅单选一个角色时返回更精确的提示错误；否则返回通用角色不匹配错误。
func resolveCouponPaymentRoleError(coupon *models.Coupon, isGuest bool) error {
	if matchesCouponRole(coupon, isGuest) {
		return nil
	}
	if coupon == nil || len(coupon.PaymentRoles) == 0 {
		return ErrCouponPaymentRoleNotAllowed
	}

	roles := make(map[string]struct{}, len(coupon.PaymentRoles))
	for _, role := range coupon.PaymentRoles {
		normalized := strings.ToLower(strings.TrimSpace(role))
		if normalized != constants.PaymentRoleGuest && normalized != constants.PaymentRoleMember {
			continue
		}
		roles[normalized] = struct{}{}
	}

	if len(roles) == 1 {
		if _, ok := roles[constants.PaymentRoleGuest]; ok {
			return ErrCouponPaymentRoleGuestOnly
		}
		if _, ok := roles[constants.PaymentRoleMember]; ok {
			return ErrCouponPaymentRoleMemberOnly
		}
	}
	return ErrCouponPaymentRoleNotAllowed
}

// matchesCouponMemberLevel 判断当前会员等级是否满足优惠券会员等级限制；未配置限制时默认允许。
func matchesCouponMemberLevel(coupon *models.Coupon, memberLevelID uint) bool {
	if coupon == nil || len(coupon.MemberLevels) == 0 {
		return true
	}
	if memberLevelID == 0 {
		return false
	}
	for _, levelID := range coupon.MemberLevels {
		if levelID == memberLevelID {
			return true
		}
	}
	return false
}

func (s *CouponService) resolveCouponEligibility(coupon *models.Coupon, items []models.OrderItem) (couponEligibility, error) {
	if strings.ToLower(strings.TrimSpace(coupon.ScopeType)) != constants.ScopeTypeProduct {
		return couponEligibility{}, ErrCouponScopeInvalid
	}

	ids, err := decodeScopeIDs(coupon.ScopeRefIDs)
	if err != nil {
		return couponEligibility{}, ErrCouponScopeInvalid
	}
	if len(ids) == 0 {
		return couponEligibility{}, ErrCouponScopeInvalid
	}

	eligible := decimal.Zero
	eligibleQuantity := 0
	scopeMatched := 0
	wholesaleExcluded := 0
	for _, item := range items {
		if _, ok := ids[item.ProductID]; !ok {
			continue
		}
		scopeMatched++
		if coupon.DisabledWholesalePrice && item.WholesaleDiscount.Decimal.GreaterThan(decimal.Zero) {
			wholesaleExcluded++
			continue
		}
		eligible = eligible.Add(item.TotalPrice.Decimal)
		if item.Quantity > 0 {
			eligibleQuantity += item.Quantity
		}
	}

	if eligible.IsZero() {
		if scopeMatched > 0 && wholesaleExcluded == scopeMatched {
			return couponEligibility{}, ErrCouponWholesaleDisabled
		}
		return couponEligibility{}, ErrCouponScopeInvalid
	}
	return couponEligibility{
		subtotal: models.NewMoneyFromDecimal(eligible),
		quantity: eligibleQuantity,
	}, nil
}

func (s *CouponService) calculateDiscount(coupon *models.Coupon, eligibility couponEligibility) (models.Money, error) {
	switch strings.ToLower(strings.TrimSpace(coupon.Type)) {
	case constants.CouponTypeFixed:
		if coupon.Value.Decimal.LessThanOrEqual(decimal.Zero) {
			return models.Money{}, ErrCouponInvalid
		}
		if coupon.PerItemDiscount {
			if eligibility.quantity <= 0 {
				return models.Money{}, ErrCouponScopeInvalid
			}
			discount := coupon.Value.Decimal.Mul(decimal.NewFromInt(int64(eligibility.quantity)))
			return models.NewMoneyFromDecimal(discount), nil
		}
		return models.NewMoneyFromDecimal(coupon.Value.Decimal), nil
	case constants.CouponTypePercent:
		if coupon.Value.Decimal.LessThanOrEqual(decimal.Zero) {
			return models.Money{}, ErrCouponInvalid
		}
		percent := coupon.Value.Decimal.Div(decimal.NewFromInt(100))
		discount := eligibility.subtotal.Decimal.Mul(percent)
		return models.NewMoneyFromDecimal(discount), nil
	default:
		return models.Money{}, ErrCouponInvalid
	}
}

func decodeScopeIDs(raw string) (map[uint]struct{}, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[uint]struct{}{}, nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(trimmed), &ids); err != nil {
		return nil, err
	}
	result := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		result[id] = struct{}{}
	}
	return result, nil
}

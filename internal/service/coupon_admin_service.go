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

// CouponAdminService 优惠券管理服务
type CouponAdminService struct {
	repo repository.CouponRepository
}

// NewCouponAdminService 创建优惠券管理服务
func NewCouponAdminService(repo repository.CouponRepository) *CouponAdminService {
	return &CouponAdminService{repo: repo}
}

// CreateCouponInput 创建优惠券输入
type CreateCouponInput struct {
	Code                   string
	Type                   string
	Value                  models.Money
	MinAmount              models.Money
	MaxDiscount            models.Money
	UsageLimit             int
	PerUserLimit           int
	DisabledWholesalePrice *bool
	PerItemDiscount        *bool
	PaymentRoles           []string
	MemberLevels           []uint
	ScopeRefIDs            []uint
	StartsAt               *time.Time
	EndsAt                 *time.Time
	IsActive               *bool
}

// UpdateCouponInput 更新优惠券输入
type UpdateCouponInput struct {
	Code                   string
	Type                   string
	Value                  models.Money
	MinAmount              models.Money
	MaxDiscount            models.Money
	UsageLimit             int
	PerUserLimit           int
	DisabledWholesalePrice *bool
	PerItemDiscount        *bool
	PaymentRoles           []string
	MemberLevels           []uint
	ScopeRefIDs            []uint
	StartsAt               *time.Time
	EndsAt                 *time.Time
	IsActive               *bool
}

// Create 创建优惠券
func (s *CouponAdminService) Create(input CreateCouponInput) (*models.Coupon, error) {
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return nil, ErrCouponInvalid
	}
	couponType := strings.ToLower(strings.TrimSpace(input.Type))
	if couponType != constants.CouponTypeFixed && couponType != constants.CouponTypePercent {
		return nil, ErrCouponInvalid
	}
	if input.Value.Decimal.LessThanOrEqual(decimal.Zero) {
		return nil, ErrCouponInvalid
	}
	if couponType == constants.CouponTypePercent && input.Value.Decimal.GreaterThan(decimal.NewFromInt(100)) {
		return nil, ErrCouponInvalid
	}

	exist, err := s.repo.GetByCode(code)
	if err != nil {
		return nil, err
	}
	if exist != nil {
		return nil, ErrCouponInvalid
	}

	scopeRefIDs, err := encodeScopeRefIDs(input.ScopeRefIDs)
	if err != nil {
		return nil, err
	}
	paymentRoles, err := normalizeCouponPaymentRoles(input.PaymentRoles)
	if err != nil {
		return nil, err
	}
	memberLevels := normalizeCouponMemberLevels(input.MemberLevels)

	if input.StartsAt != nil && input.EndsAt != nil && input.EndsAt.Before(*input.StartsAt) {
		return nil, ErrCouponInvalid
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}
	disabledWholesalePrice := false
	if input.DisabledWholesalePrice != nil {
		disabledWholesalePrice = *input.DisabledWholesalePrice
	}
	perItemDiscount := false
	if couponType == constants.CouponTypeFixed && input.PerItemDiscount != nil {
		perItemDiscount = *input.PerItemDiscount
	}

	coupon := &models.Coupon{
		Code:                   code,
		Type:                   couponType,
		Value:                  input.Value,
		MinAmount:              input.MinAmount,
		MaxDiscount:            input.MaxDiscount,
		UsageLimit:             input.UsageLimit,
		UsedCount:              0,
		PerUserLimit:           input.PerUserLimit,
		DisabledWholesalePrice: disabledWholesalePrice,
		PerItemDiscount:        perItemDiscount,
		PaymentRoles:           paymentRoles,
		MemberLevels:           memberLevels,
		ScopeType:              constants.ScopeTypeProduct,
		ScopeRefIDs:            scopeRefIDs,
		StartsAt:               input.StartsAt,
		EndsAt:                 input.EndsAt,
		IsActive:               isActive,
	}

	if err := s.repo.Create(coupon); err != nil {
		return nil, err
	}
	return coupon, nil
}

// Update 更新优惠券
func (s *CouponAdminService) Update(id uint, input UpdateCouponInput) (*models.Coupon, error) {
	if id == 0 {
		return nil, ErrCouponInvalid
	}
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrCouponNotFound
	}

	code := strings.TrimSpace(input.Code)
	if code == "" {
		return nil, ErrCouponInvalid
	}
	couponType := strings.ToLower(strings.TrimSpace(input.Type))
	if couponType != constants.CouponTypeFixed && couponType != constants.CouponTypePercent {
		return nil, ErrCouponInvalid
	}
	if input.Value.Decimal.LessThanOrEqual(decimal.Zero) {
		return nil, ErrCouponInvalid
	}
	if couponType == constants.CouponTypePercent && input.Value.Decimal.GreaterThan(decimal.NewFromInt(100)) {
		return nil, ErrCouponInvalid
	}

	if code != existing.Code {
		dup, err := s.repo.GetByCode(code)
		if err != nil {
			return nil, err
		}
		if dup != nil {
			return nil, ErrCouponInvalid
		}
	}

	scopeRefIDs, err := encodeScopeRefIDs(input.ScopeRefIDs)
	if err != nil {
		return nil, err
	}
	paymentRoles, err := normalizeCouponPaymentRoles(input.PaymentRoles)
	if err != nil {
		return nil, err
	}
	memberLevels := normalizeCouponMemberLevels(input.MemberLevels)
	if input.StartsAt != nil && input.EndsAt != nil && input.EndsAt.Before(*input.StartsAt) {
		return nil, ErrCouponInvalid
	}

	isActive := existing.IsActive
	if input.IsActive != nil {
		isActive = *input.IsActive
	}
	disabledWholesalePrice := existing.DisabledWholesalePrice
	if input.DisabledWholesalePrice != nil {
		disabledWholesalePrice = *input.DisabledWholesalePrice
	}
	perItemDiscount := existing.PerItemDiscount
	if input.PerItemDiscount != nil {
		perItemDiscount = *input.PerItemDiscount
	}
	if couponType != constants.CouponTypeFixed {
		perItemDiscount = false
	}

	existing.Code = code
	existing.Type = couponType
	existing.Value = input.Value
	existing.MinAmount = input.MinAmount
	existing.MaxDiscount = input.MaxDiscount
	existing.UsageLimit = input.UsageLimit
	existing.PerUserLimit = input.PerUserLimit
	existing.DisabledWholesalePrice = disabledWholesalePrice
	existing.PerItemDiscount = perItemDiscount
	existing.PaymentRoles = paymentRoles
	existing.MemberLevels = memberLevels
	existing.ScopeType = constants.ScopeTypeProduct
	existing.ScopeRefIDs = scopeRefIDs
	existing.StartsAt = input.StartsAt
	existing.EndsAt = input.EndsAt
	existing.IsActive = isActive

	if err := s.repo.Update(existing); err != nil {
		return nil, ErrCouponUpdateFailed
	}
	return existing, nil
}

// Delete 删除优惠券
func (s *CouponAdminService) Delete(id uint) error {
	if id == 0 {
		return ErrCouponInvalid
	}
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrCouponNotFound
	}
	if err := s.repo.Delete(id); err != nil {
		return ErrCouponDeleteFailed
	}
	return nil
}

// List 获取优惠券列表
func (s *CouponAdminService) List(filter repository.CouponListFilter) ([]models.Coupon, int64, error) {
	return s.repo.List(filter)
}

func encodeScopeRefIDs(ids []uint) (string, error) {
	if len(ids) == 0 {
		return "", ErrCouponScopeInvalid
	}
	payload, err := json.Marshal(ids)
	if err != nil {
		return "", ErrCouponScopeInvalid
	}
	return string(payload), nil
}

// normalizeCouponPaymentRoles 归一化优惠券付款角色限制，仅允许 guest/member，自动去重与去空。
func normalizeCouponPaymentRoles(raw []string) (models.StringArray, error) {
	if len(raw) == 0 {
		return models.StringArray{}, nil
	}
	allowed := map[string]struct{}{
		constants.PaymentRoleGuest:  {},
		constants.PaymentRoleMember: {},
	}
	seen := make(map[string]struct{}, len(raw))
	normalized := make(models.StringArray, 0, len(raw))
	for _, item := range raw {
		role := strings.ToLower(strings.TrimSpace(item))
		if role == "" {
			continue
		}
		if _, ok := allowed[role]; !ok {
			return nil, ErrCouponInvalid
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		normalized = append(normalized, role)
	}
	return normalized, nil
}

// normalizeCouponMemberLevels 归一化优惠券会员等级限制，过滤 0 并去重。
func normalizeCouponMemberLevels(raw []uint) models.UintArray {
	if len(raw) == 0 {
		return models.UintArray{}
	}
	seen := make(map[uint]struct{}, len(raw))
	normalized := make(models.UintArray, 0, len(raw))
	for _, item := range raw {
		if item == 0 {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	return normalized
}

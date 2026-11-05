package application

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// AdminService 优惠券管理服务
type AdminService struct {
	repo couponcontract.Repository
	skus couponcontract.SKUDirectory
}

// WithSKUDirectory 注入规格目录，用于保存时校验规格是否属于所选商品；未注入时跳过该校验。
func (s *AdminService) WithSKUDirectory(skus couponcontract.SKUDirectory) *AdminService {
	s.skus = skus
	return s
}

// NewAdminService 创建优惠券管理服务
func NewAdminService(repo couponcontract.Repository) *AdminService {
	return &AdminService{repo: repo}
}

// CreateCouponInput 创建优惠券输入
type CreateCouponInput struct {
	Code                   string
	Type                   string
	Value                  money.Amount
	MinAmount              money.Amount
	MaxDiscount            money.Amount
	UsageLimit             int
	PerUserLimit           int
	DisabledWholesalePrice *bool
	PerItemDiscount        *bool
	PaymentRoles           []string
	MemberLevels           []uint
	ScopeRefIDs            []uint
	ScopeSKUIDs            []uint
	StartsAt               *time.Time
	EndsAt                 *time.Time
	IsActive               *bool
}

// UpdateCouponInput 更新优惠券输入
type UpdateCouponInput struct {
	Code                   string
	Type                   string
	Value                  money.Amount
	MinAmount              money.Amount
	MaxDiscount            money.Amount
	UsageLimit             int
	PerUserLimit           int
	DisabledWholesalePrice *bool
	PerItemDiscount        *bool
	PaymentRoles           []string
	MemberLevels           []uint
	ScopeRefIDs            []uint
	ScopeSKUIDs            []uint
	StartsAt               *time.Time
	EndsAt                 *time.Time
	IsActive               *bool
}

// Create 创建优惠券
func (s *AdminService) Create(input CreateCouponInput) (*coupondomain.Coupon, error) {
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return nil, couponcontract.ErrInvalid
	}
	couponType := strings.ToLower(strings.TrimSpace(input.Type))
	if couponType != constants.CouponTypeFixed && couponType != constants.CouponTypePercent {
		return nil, couponcontract.ErrInvalid
	}
	if input.Value.Decimal.LessThanOrEqual(decimal.Zero) {
		return nil, couponcontract.ErrInvalid
	}
	if couponType == constants.CouponTypePercent && input.Value.Decimal.GreaterThan(decimal.NewFromInt(100)) {
		return nil, couponcontract.ErrInvalid
	}

	exist, err := s.repo.GetByCode(code)
	if err != nil {
		return nil, err
	}
	if exist != nil {
		return nil, couponcontract.ErrInvalid
	}

	scopeRefIDs, err := encodeScopeRefIDs(input.ScopeRefIDs)
	if err != nil {
		return nil, err
	}
	scopeSKUIDs, err := encodeScopeSKUIDs(input.ScopeSKUIDs)
	if err != nil {
		return nil, err
	}
	if err := s.validateScopeSKUs(input.ScopeRefIDs, input.ScopeSKUIDs); err != nil {
		return nil, err
	}
	paymentRoles, err := normalizeCouponPaymentRoles(input.PaymentRoles)
	if err != nil {
		return nil, err
	}
	memberLevels := normalizeCouponMemberLevels(input.MemberLevels)

	if input.StartsAt != nil && input.EndsAt != nil && input.EndsAt.Before(*input.StartsAt) {
		return nil, couponcontract.ErrInvalid
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

	coupon := &coupondomain.Coupon{
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
		ScopeSKUIDs:            scopeSKUIDs,
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
func (s *AdminService) Update(id uint, input UpdateCouponInput) (*coupondomain.Coupon, error) {
	if id == 0 {
		return nil, couponcontract.ErrInvalid
	}
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, couponcontract.ErrNotFound
	}

	code := strings.TrimSpace(input.Code)
	if code == "" {
		return nil, couponcontract.ErrInvalid
	}
	couponType := strings.ToLower(strings.TrimSpace(input.Type))
	if couponType != constants.CouponTypeFixed && couponType != constants.CouponTypePercent {
		return nil, couponcontract.ErrInvalid
	}
	if input.Value.Decimal.LessThanOrEqual(decimal.Zero) {
		return nil, couponcontract.ErrInvalid
	}
	if couponType == constants.CouponTypePercent && input.Value.Decimal.GreaterThan(decimal.NewFromInt(100)) {
		return nil, couponcontract.ErrInvalid
	}

	if code != existing.Code {
		dup, err := s.repo.GetByCode(code)
		if err != nil {
			return nil, err
		}
		if dup != nil {
			return nil, couponcontract.ErrInvalid
		}
	}

	scopeRefIDs, err := encodeScopeRefIDs(input.ScopeRefIDs)
	if err != nil {
		return nil, err
	}
	scopeSKUIDs, err := encodeScopeSKUIDs(input.ScopeSKUIDs)
	if err != nil {
		return nil, err
	}
	if err := s.validateScopeSKUs(input.ScopeRefIDs, input.ScopeSKUIDs); err != nil {
		return nil, err
	}
	paymentRoles, err := normalizeCouponPaymentRoles(input.PaymentRoles)
	if err != nil {
		return nil, err
	}
	memberLevels := normalizeCouponMemberLevels(input.MemberLevels)
	if input.StartsAt != nil && input.EndsAt != nil && input.EndsAt.Before(*input.StartsAt) {
		return nil, couponcontract.ErrInvalid
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
	existing.ScopeSKUIDs = scopeSKUIDs
	existing.StartsAt = input.StartsAt
	existing.EndsAt = input.EndsAt
	existing.IsActive = isActive

	if err := s.repo.Update(existing); err != nil {
		return nil, couponcontract.ErrUpdateFailed
	}
	return existing, nil
}

// Delete 删除优惠券
func (s *AdminService) Delete(id uint) error {
	if id == 0 {
		return couponcontract.ErrInvalid
	}
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return couponcontract.ErrNotFound
	}
	if err := s.repo.Delete(id); err != nil {
		return couponcontract.ErrDeleteFailed
	}
	return nil
}

// List 获取优惠券列表
func (s *AdminService) List(filter couponcontract.ListFilter) ([]coupondomain.Coupon, int64, error) {
	return s.repo.List(filter)
}

func encodeScopeRefIDs(ids []uint) (string, error) {
	if len(ids) == 0 {
		return "", couponcontract.ErrScopeInvalid
	}
	payload, err := json.Marshal(ids)
	if err != nil {
		return "", couponcontract.ErrScopeInvalid
	}
	return string(payload), nil
}

// validateScopeSKUs 校验每个规格都存在且属于所选商品范围，
// 并保证每个所选商品至少有一个规格被选中（否则该商品永远不会参与优惠）。
func (s *AdminService) validateScopeSKUs(productIDs, skuIDs []uint) error {
	if s.skus == nil || len(skuIDs) == 0 {
		return nil
	}
	allowed := make(map[uint]struct{}, len(productIDs))
	for _, id := range productIDs {
		allowed[id] = struct{}{}
	}
	owners, err := s.skus.ResolveSKUProducts(skuIDs)
	if err != nil {
		return err
	}
	covered := make(map[uint]struct{}, len(skuIDs))
	for _, id := range skuIDs {
		if id == 0 {
			continue
		}
		productID, ok := owners[id]
		if !ok {
			return couponcontract.ErrScopeInvalid
		}
		if _, ok := allowed[productID]; !ok {
			return couponcontract.ErrScopeInvalid
		}
		covered[productID] = struct{}{}
	}
	for _, id := range productIDs {
		if id == 0 {
			continue
		}
		if _, ok := covered[id]; !ok {
			return couponcontract.ErrScopeInvalid
		}
	}
	return nil
}

// encodeScopeSKUIDs 编码优惠券适用规格集合；为空表示不限制规格（商品下全部规格）。
func encodeScopeSKUIDs(ids []uint) (string, error) {
	seen := make(map[uint]struct{}, len(ids))
	normalized := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	if len(normalized) == 0 {
		return "", nil
	}
	payload, _ := json.Marshal(normalized)
	return string(payload), nil
}

// normalizeCouponPaymentRoles 归一化优惠券付款角色限制，仅允许 guest/member，自动去重与去空。
func normalizeCouponPaymentRoles(raw []string) (jsonslice.Strings, error) {
	if len(raw) == 0 {
		return jsonslice.Strings{}, nil
	}
	allowed := map[string]struct{}{
		constants.PaymentRoleGuest:  {},
		constants.PaymentRoleMember: {},
	}
	seen := make(map[string]struct{}, len(raw))
	normalized := make(jsonslice.Strings, 0, len(raw))
	for _, item := range raw {
		role := strings.ToLower(strings.TrimSpace(item))
		if role == "" {
			continue
		}
		if _, ok := allowed[role]; !ok {
			return nil, couponcontract.ErrInvalid
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
func normalizeCouponMemberLevels(raw []uint) jsonslice.Uints {
	if len(raw) == 0 {
		return jsonslice.Uints{}
	}
	seen := make(map[uint]struct{}, len(raw))
	normalized := make(jsonslice.Uints, 0, len(raw))
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

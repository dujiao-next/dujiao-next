package application

import (
	"sort"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/logger"
	resellerapplication "github.com/dujiao-next/internal/modules/reseller/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

const (
	resellerRuleSourceSKU     = resellerapplication.RuleSourceSKU
	resellerRuleSourceProduct = resellerapplication.RuleSourceProduct
	resellerRuleSourceProfile = resellerapplication.RuleSourceProfile
	resellerRuleSourceInherit = resellerapplication.RuleSourceInherit
)

// ResellerPricingResolver resolves reseller-facing prices before order transactions.
type ResellerPricingResolver struct {
	repo resellerPricingStore
}

type resellerPricingStore interface {
	GetProfileByID(id uint) (*resellerdomain.Profile, error)
	ListProductSettingsForPricing(resellerID uint, productIDs, skuIDs []uint) ([]resellerdomain.ProductSetting, error)
	IsActiveRelatedAccount(resellerID, userID uint) (bool, error)
}

func NewResellerPricingResolver(repo resellerPricingStore) *ResellerPricingResolver {
	return &ResellerPricingResolver{repo: repo}
}

func (r *ResellerPricingResolver) ApplyToOrderBuildResult(tenant resellercontract.TenantContext, buyerUserID uint, result *orderBuildResult) (*resellercontract.OrderPricingContext, error) {
	if !isResellerOrderContext(tenant) {
		return nil, nil
	}
	if r == nil || r.repo == nil || result == nil || tenant.ResellerID == nil {
		return nil, ErrResellerProductNotListed
	}
	profile, err := r.loadActiveProfile(*tenant.ResellerID)
	if err != nil {
		return nil, err
	}
	productIDs, skuIDs := collectOrderPlanIDs(result.Plans)
	settings, err := r.repo.ListProductSettingsForPricing(*tenant.ResellerID, productIDs, skuIDs)
	if err != nil {
		return nil, err
	}
	settingsByProduct, settingsBySKU := buildSettingIndexes(settings)

	ctx := &resellercontract.OrderPricingContext{
		ResellerID:     *tenant.ResellerID,
		Domain:         resellerSnapshotDomain(tenant),
		Currency:       result.Currency,
		ResellerUserID: profile.UserID,
		BuyerUserID:    buyerUserID,
		ProfitEligible: true,
		Items:          make([]resellercontract.OrderPricingItem, 0, len(result.Plans)),
	}

	for i := range result.Plans {
		plan := &result.Plans[i]
		if plan == nil || plan.Product == nil || plan.SKU == nil {
			return nil, ErrProductSKUInvalid
		}
		productSetting := settingsByProduct[plan.Product.ID]
		skuSetting := settingsBySKU[resellercontract.SettingKey{ProductID: plan.Product.ID, SKUID: plan.SKU.ID}]
		if productSetting != nil && !productSetting.IsListed {
			return nil, ErrResellerProductNotListed
		}
		if skuSetting != nil && !skuSetting.IsListed {
			return nil, ErrResellerProductNotListed
		}

		baseUnit := plan.SKU.PriceAmount.Decimal.Round(2)
		if plan.SKU.AgencyPriceAmount.Decimal.GreaterThan(decimal.Zero) {
			baseUnit = plan.SKU.AgencyPriceAmount.Decimal.Round(2)
		}
		masterUnit := plan.SKU.PriceAmount.Decimal.Round(2)
		resellerUnit, rule, err := resolveResellerUnitAmount(profile, productSetting, skuSetting, baseUnit, masterUnit)
		if err != nil {
			return nil, err
		}
		if err := validateResellerUnitAmount(profile, plan.SKU, baseUnit, resellerUnit, masterUnit); err != nil {
			return nil, err
		}
		quantity := decimal.NewFromInt(int64(plan.Item.Quantity))
		baseTotal := baseUnit.Mul(quantity).Round(2)
		resellerTotal := resellerUnit.Mul(quantity).Round(2)
		profit := resellerTotal.Sub(baseTotal).Round(2)

		zeroMoney := money.FromDecimal(decimal.Zero)
		plan.TotalAmount = resellerTotal
		plan.CouponDiscount = decimal.Zero
		plan.MemberDiscount = decimal.Zero
		plan.PromotionDiscount = decimal.Zero
		plan.WholesaleDiscount = decimal.Zero
		plan.Item.OriginalUnitPrice = money.FromDecimal(resellerUnit)
		plan.Item.UnitPrice = money.FromDecimal(resellerUnit)
		plan.Item.OriginalTotalPrice = money.FromDecimal(resellerTotal)
		plan.Item.TotalPrice = money.FromDecimal(resellerTotal)
		plan.Item.MemberDiscount = zeroMoney
		plan.Item.CouponDiscount = zeroMoney
		plan.Item.PromotionDiscount = zeroMoney
		plan.Item.WholesaleDiscount = zeroMoney
		plan.Item.PromotionID = nil

		ctx.BaseAmount = ctx.BaseAmount.Add(baseTotal).Round(2)
		ctx.ResellerAmount = ctx.ResellerAmount.Add(resellerTotal).Round(2)
		ctx.ProfitAmount = ctx.ProfitAmount.Add(profit).Round(2)
		ctx.Items = append(ctx.Items, resellercontract.OrderPricingItem{
			ProductID:           plan.Product.ID,
			SKUID:               plan.SKU.ID,
			Quantity:            plan.Item.Quantity,
			BaseUnitAmount:      baseUnit,
			ResellerUnitAmount:  resellerUnit,
			BaseTotalAmount:     baseTotal,
			ResellerTotalAmount: resellerTotal,
			ProfitAmount:        profit,
			PricingMode:         rule.Mode,
			RuleSource:          rule.Source,
			SettingID:           rule.SettingID,
		})
	}

	result.OriginalAmount = decimal.Zero
	result.TotalAmount = decimal.Zero
	for _, plan := range result.Plans {
		result.OriginalAmount = result.OriginalAmount.Add(plan.TotalAmount).Round(2)
		result.TotalAmount = result.TotalAmount.Add(plan.TotalAmount.Sub(plan.CouponDiscount)).Round(2)
	}
	result.DiscountAmount = decimal.Zero
	result.MemberDiscountAmount = decimal.Zero
	result.PromotionDiscountAmount = decimal.Zero
	result.WholesaleDiscountAmount = decimal.Zero
	result.AppliedCoupon = nil
	result.OrderPromotionID = nil
	result.MemberLevelID = nil

	if err := r.applySelfDealingRisk(ctx, profile); err != nil {
		return nil, err
	}
	if ctx.ProfitEligible {
		ctx.EffectiveProfit = ctx.ProfitAmount
	} else {
		ctx.EffectiveProfit = decimal.Zero
	}
	ctx.PricingSnapshot = ctx.BuildPricingSnapshotJSON()
	ctx.RiskSnapshot = ctx.BuildRiskSnapshotJSON()
	return ctx, nil
}

func (r *ResellerPricingResolver) LoadDisplayPricingBatch(tenant resellercontract.TenantContext, products []productdomain.Product) (*resellercontract.DisplayPricingBatch, error) {
	if !isResellerOrderContext(tenant) {
		return nil, nil
	}
	if r == nil || r.repo == nil || tenant.ResellerID == nil {
		return nil, ErrResellerProductNotListed
	}
	profile, err := r.loadActiveProfile(*tenant.ResellerID)
	if err != nil {
		return nil, err
	}
	productIDs, skuIDs := collectProductIDs(products)
	settings, err := r.repo.ListProductSettingsForPricing(*tenant.ResellerID, productIDs, skuIDs)
	if err != nil {
		return nil, err
	}
	byProduct := make(map[uint][]resellerdomain.ProductSetting)
	for _, setting := range settings {
		byProduct[setting.ProductID] = append(byProduct[setting.ProductID], setting)
	}
	return &resellercontract.DisplayPricingBatch{
		Tenant:            tenant,
		Profile:           profile,
		SettingsByProduct: byProduct,
	}, nil
}

func (r *ResellerPricingResolver) ResolveDisplayPrices(tenant resellercontract.TenantContext, product *productdomain.Product, batch *resellercontract.DisplayPricingBatch) (*resellercontract.DisplayPriceResult, error) {
	if !isResellerOrderContext(tenant) {
		return nil, nil
	}
	if product == nil || batch == nil || batch.Profile == nil {
		return nil, ErrResellerProductNotListed
	}
	productSettings, skuSettings := buildSettingIndexes(batch.SettingsByProduct[product.ID])
	productSetting := productSettings[product.ID]
	if productSetting != nil && !productSetting.IsListed {
		return &resellercontract.DisplayPriceResult{Visible: false, ProductID: product.ID}, nil
	}

	result := &resellercontract.DisplayPriceResult{
		Visible:      false,
		ProductID:    product.ID,
		SKUPrices:    map[uint]money.Amount{},
		HiddenSKUIDs: map[uint]bool{},
	}
	for _, sku := range product.SKUs {
		if !sku.IsActive {
			continue
		}
		skuSetting := skuSettings[resellercontract.SettingKey{ProductID: product.ID, SKUID: sku.ID}]
		if skuSetting != nil && !skuSetting.IsListed {
			result.HiddenSKUIDs[sku.ID] = true
			continue
		}
		base := sku.PriceAmount.Decimal.Round(2)
		if sku.AgencyPriceAmount.Decimal.GreaterThan(decimal.Zero) {
			base = sku.AgencyPriceAmount.Decimal.Round(2)
		}
		master := sku.PriceAmount.Decimal.Round(2)
		price, _, err := resolveResellerUnitAmount(batch.Profile, productSetting, skuSetting, base, master)
		if err == nil {
			err = validateResellerUnitAmount(batch.Profile, &sku, base, price, master)
		}
		if err != nil {
			// 定价配置可能在保存后因基准价/成本价/上限调整而失效；
			// 展示路径不应整单失败，仅隐藏该 SKU 并记录告警，便于运营发现需修正的脏配置。
			logger.Warnw("reseller_display_price_sku_hidden",
				"reseller_id", batch.Profile.ID,
				"product_id", product.ID,
				"sku_id", sku.ID,
				"error", err.Error(),
			)
			result.HiddenSKUIDs[sku.ID] = true
			continue
		}
		money := money.FromDecimal(price)
		result.SKUPrices[sku.ID] = money
		if !result.Visible {
			result.Visible = true
			result.DisplaySKUID = sku.ID
			result.DisplayPrice = money
		}
	}
	if len(product.SKUs) == 0 {
		base := product.PriceAmount.Decimal.Round(2)
		if product.AgencyPriceAmount.Decimal.GreaterThan(decimal.Zero) {
			base = product.AgencyPriceAmount.Decimal.Round(2)
		}
		master := product.PriceAmount.Decimal.Round(2)
		price, _, err := resolveResellerUnitAmount(batch.Profile, productSetting, nil, base, master)
		if err != nil {
			logger.Warnw("reseller_display_price_product_hidden",
				"reseller_id", batch.Profile.ID,
				"product_id", product.ID,
				"error", err.Error(),
			)
			return &resellercontract.DisplayPriceResult{Visible: false, ProductID: product.ID}, nil
		}
		result.Visible = true
		result.DisplayPrice = money.FromDecimal(price)
	}
	return result, nil
}

func (r *ResellerPricingResolver) loadActiveProfile(resellerID uint) (*resellerdomain.Profile, error) {
	profile, err := r.repo.GetProfileByID(resellerID)
	if err != nil {
		return nil, err
	}
	if profile == nil || profile.Status != resellerdomain.ProfileStatusActive {
		return nil, ErrResellerProductNotListed
	}
	return profile, nil
}

func (r *ResellerPricingResolver) applySelfDealingRisk(ctx *resellercontract.OrderPricingContext, profile *resellerdomain.Profile) error {
	if ctx == nil || profile == nil {
		return nil
	}
	relatedMatch := false
	if ctx.BuyerUserID > 0 && ctx.BuyerUserID != profile.UserID {
		matched, err := r.repo.IsActiveRelatedAccount(ctx.ResellerID, ctx.BuyerUserID)
		if err != nil {
			return err
		}
		relatedMatch = matched
	}
	resellercontract.ApplySelfDealingRisk(ctx, profile, relatedMatch)
	return nil
}

func buildSettingIndexes(settings []resellerdomain.ProductSetting) (map[uint]*resellerdomain.ProductSetting, map[resellercontract.SettingKey]*resellerdomain.ProductSetting) {
	return resellercontract.BuildSettingIndexes(settings)
}

func resolveResellerUnitAmount(profile *resellerdomain.Profile, productSetting *resellerdomain.ProductSetting, skuSetting *resellerdomain.ProductSetting, baseUnit decimal.Decimal, masterUnit ...decimal.Decimal) (decimal.Decimal, resellerapplication.PricingRule, error) {
	return resellerapplication.ResolveUnitAmount(profile, productSetting, skuSetting, baseUnit, masterUnit...)
}

func validateResellerUnitAmount(profile *resellerdomain.Profile, sku *productdomain.ProductSKU, baseUnit decimal.Decimal, resellerUnit decimal.Decimal, masterPrice ...decimal.Decimal) error {
	return resellerapplication.ValidateUnitAmount(profile, sku, baseUnit, resellerUnit, masterPrice...)
}

func collectOrderPlanIDs(plans []childOrderPlan) ([]uint, []uint) {
	productIDs := make([]uint, 0, len(plans))
	skuIDs := make([]uint, 0, len(plans))
	for _, plan := range plans {
		if plan.Product != nil {
			productIDs = append(productIDs, plan.Product.ID)
		} else if plan.Item.ProductID > 0 {
			productIDs = append(productIDs, plan.Item.ProductID)
		}
		if plan.SKU != nil {
			skuIDs = append(skuIDs, plan.SKU.ID)
		} else if plan.Item.SKUID > 0 {
			skuIDs = append(skuIDs, plan.Item.SKUID)
		}
	}
	return uniqueServiceUintSlice(productIDs), uniqueServiceUintSlice(skuIDs)
}

func collectProductIDs(products []productdomain.Product) ([]uint, []uint) {
	productIDs := make([]uint, 0, len(products))
	skuIDs := []uint{}
	for _, product := range products {
		if product.ID > 0 {
			productIDs = append(productIDs, product.ID)
		}
		for _, sku := range product.SKUs {
			if sku.ID > 0 {
				skuIDs = append(skuIDs, sku.ID)
			}
		}
	}
	return uniqueServiceUintSlice(productIDs), uniqueServiceUintSlice(skuIDs)
}

func uniqueServiceUintSlice(values []uint) []uint {
	if len(values) == 0 {
		return nil
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	result := make([]uint, 0, len(values))
	var last uint
	for i, value := range values {
		if value == 0 {
			continue
		}
		if i > 0 && value == last {
			continue
		}
		result = append(result, value)
		last = value
	}
	return result
}

func isResellerOrderContext(tenant resellercontract.TenantContext) bool {
	return tenant.IsReseller()
}

func resellerSnapshotDomain(tenant resellercontract.TenantContext) string {
	return resellercontract.SnapshotDomain(tenant)
}

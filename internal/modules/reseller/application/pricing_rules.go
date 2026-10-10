package application

import (
	"strings"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/shopspring/decimal"
)

const (
	RuleSourceSKU     = "sku"
	RuleSourceProduct = "product"
	RuleSourceProfile = "profile"
	RuleSourceInherit = "inherit"
)

// PricingRule 描述分销改价规则解析结果。
type PricingRule struct {
	Mode              string
	Source            string
	SettingID         *uint
	MarkupPercent     decimal.Decimal
	FixedMarkupAmount decimal.Decimal
	FixedPriceAmount  decimal.Decimal
}

// ResolveUnitAmount 按 SKU → 商品 → 资料默认加价 → 继承主站售价的优先级解析成交单价。
// baseUnit 为分销进货底价（优先代理价）；可选参数 masterUnit 为主站零售价格。
func ResolveUnitAmount(profile *resellerdomain.Profile, productSetting *resellerdomain.ProductSetting, skuSetting *resellerdomain.ProductSetting, baseUnit decimal.Decimal, masterUnit ...decimal.Decimal) (decimal.Decimal, PricingRule, error) {
	master := baseUnit
	if len(masterUnit) > 0 && masterUnit[0].GreaterThan(decimal.Zero) {
		master = masterUnit[0].Round(2)
	}
	if skuSetting != nil && strings.TrimSpace(skuSetting.PricingMode) != resellerdomain.PricingModeInherit {
		return ApplyPricingRule(*skuSetting, RuleSourceSKU, baseUnit, master)
	}
	if productSetting != nil && strings.TrimSpace(productSetting.PricingMode) != resellerdomain.PricingModeInherit {
		return ApplyPricingRule(*productSetting, RuleSourceProduct, baseUnit, master)
	}
	if profile != nil && profile.DefaultMarkupPercent.Decimal.GreaterThan(decimal.Zero) {
		rule := PricingRule{
			Mode:          resellerdomain.PricingModeMarkupPercent,
			Source:        RuleSourceProfile,
			MarkupPercent: profile.DefaultMarkupPercent.Decimal.Round(2),
		}
		price := ApplyMarkupPercent(baseUnit, rule.MarkupPercent)
		if price.LessThan(master) {
			price = master
		}
		return price, rule, nil
	}
	return master.Round(2), PricingRule{Mode: resellerdomain.PricingModeInherit, Source: RuleSourceInherit}, nil
}

// ApplyPricingRule 按单条分销配置计算单价。
func ApplyPricingRule(setting resellerdomain.ProductSetting, source string, baseUnit decimal.Decimal, masterUnit ...decimal.Decimal) (decimal.Decimal, PricingRule, error) {
	master := baseUnit
	if len(masterUnit) > 0 && masterUnit[0].GreaterThan(decimal.Zero) {
		master = masterUnit[0].Round(2)
	}
	settingID := setting.ID
	rule := PricingRule{
		Mode:              strings.TrimSpace(setting.PricingMode),
		Source:            source,
		SettingID:         &settingID,
		MarkupPercent:     setting.MarkupPercent.Decimal.Round(2),
		FixedMarkupAmount: setting.FixedMarkupAmount.Decimal.Round(2),
		FixedPriceAmount:  setting.FixedPriceAmount.Decimal.Round(2),
	}
	switch rule.Mode {
	case resellerdomain.PricingModeMarkupPercent:
		return ApplyMarkupPercent(baseUnit, rule.MarkupPercent), rule, nil
	case resellerdomain.PricingModeFixedMarkup:
		return baseUnit.Add(rule.FixedMarkupAmount).Round(2), rule, nil
	case resellerdomain.PricingModeFixedPrice:
		return rule.FixedPriceAmount.Round(2), rule, nil
	case resellerdomain.PricingModeInherit:
		return master.Round(2), rule, nil
	default:
		return decimal.Zero, rule, resellercontract.ErrPricingModeInvalid
	}
}

// ApplyMarkupPercent 按百分比加价。
func ApplyMarkupPercent(baseUnit decimal.Decimal, percent decimal.Decimal) decimal.Decimal {
	return baseUnit.Mul(decimal.NewFromInt(100).Add(percent)).Div(decimal.NewFromInt(100)).Round(2)
}

// ValidateUnitAmount 校验分销单价不低于底价/成本价，且不低于主站售价，且不超过最大加价比例。
func ValidateUnitAmount(profile *resellerdomain.Profile, sku *productdomain.ProductSKU, baseUnit decimal.Decimal, resellerUnit decimal.Decimal, masterPrice ...decimal.Decimal) error {
	baseUnit = baseUnit.Round(2)
	resellerUnit = resellerUnit.Round(2)
	if resellerUnit.LessThanOrEqual(decimal.Zero) || resellerUnit.LessThan(baseUnit) {
		return resellercontract.ErrPriceBelowBase
	}
	var master decimal.Decimal
	if len(masterPrice) > 0 && masterPrice[0].GreaterThan(decimal.Zero) {
		master = masterPrice[0].Round(2)
	} else if sku != nil && sku.PriceAmount.Decimal.GreaterThan(decimal.Zero) {
		master = sku.PriceAmount.Decimal.Round(2)
	}
	if master.GreaterThan(decimal.Zero) && resellerUnit.LessThan(master) {
		return resellercontract.ErrPriceBelowMasterPrice
	}
	if sku != nil && sku.CostPriceAmount.Decimal.GreaterThan(decimal.Zero) && resellerUnit.LessThan(sku.CostPriceAmount.Decimal.Round(2)) {
		return resellercontract.ErrPriceBelowBase
	}
	if profile != nil && profile.MaxMarkupPercent.Decimal.GreaterThan(decimal.Zero) && baseUnit.GreaterThan(decimal.Zero) {
		if master.GreaterThan(decimal.Zero) && resellerUnit.LessThanOrEqual(master) {
			return nil
		}
		implicit := resellerUnit.Sub(baseUnit).Div(baseUnit).Mul(decimal.NewFromInt(100)).Round(4)
		if implicit.GreaterThan(profile.MaxMarkupPercent.Decimal.Round(4)) {
			return resellercontract.ErrMarkupExceeded
		}
	}
	return nil
}

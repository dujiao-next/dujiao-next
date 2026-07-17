package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dujiao-next/internal/models"

	"github.com/shopspring/decimal"
)

func normalizeWholesalePriceInputs(inputs []WholesalePriceInput) (models.WholesalePriceTiers, error) {
	if len(inputs) == 0 {
		return models.WholesalePriceTiers{}, nil
	}

	seen := make(map[string]struct{}, len(inputs))
	tiers := make(models.WholesalePriceTiers, 0, len(inputs))
	for _, input := range inputs {
		skuCode := strings.TrimSpace(input.SKUCode)
		minQuantity := input.MinQuantity
		unitPrice := input.UnitPrice.Round(2)
		if minQuantity <= 0 || unitPrice.LessThanOrEqual(decimal.Zero) {
			return nil, ErrWholesalePriceInvalid
		}
		key := wholesaleTierUniqueKey(input.SKUID, skuCode, minQuantity)
		if _, ok := seen[key]; ok {
			return nil, ErrWholesalePriceInvalid
		}
		seen[key] = struct{}{}
		tiers = append(tiers, models.WholesalePriceTier{
			SKUID:       input.SKUID,
			SKUCode:     skuCode,
			MinQuantity: minQuantity,
			UnitPrice:   models.NewMoneyFromDecimal(unitPrice),
		})
	}

	sort.SliceStable(tiers, func(i, j int) bool {
		scopeI := wholesaleTierScopeKey(tiers[i].SKUID, tiers[i].SKUCode)
		scopeJ := wholesaleTierScopeKey(tiers[j].SKUID, tiers[j].SKUCode)
		if scopeI != scopeJ {
			return scopeI < scopeJ
		}
		return tiers[i].MinQuantity < tiers[j].MinQuantity
	})

	lastKey := ""
	var lastPrice decimal.Decimal
	for i, tier := range tiers {
		key := wholesaleTierScopeKey(tier.SKUID, tier.SKUCode)
		if i == 0 || key != lastKey {
			lastKey = key
			lastPrice = tier.UnitPrice.Decimal
			continue
		}
		if tier.UnitPrice.Decimal.GreaterThanOrEqual(lastPrice) {
			return nil, ErrWholesalePriceInvalid
		}
		lastPrice = tier.UnitPrice.Decimal
	}
	return tiers, nil
}

func normalizeWholesalePriceInputsForSKUs(inputs []WholesalePriceInput, skus []models.ProductSKU) (models.WholesalePriceTiers, error) {
	if len(inputs) == 0 {
		return normalizeWholesalePriceInputs(inputs)
	}

	skuByID := make(map[uint]models.ProductSKU, len(skus))
	skuByCode := make(map[string]models.ProductSKU, len(skus))
	for _, sku := range skus {
		code := strings.TrimSpace(sku.SKUCode)
		if sku.ID > 0 {
			skuByID[sku.ID] = sku
		}
		if code != "" {
			skuByCode[strings.ToLower(code)] = sku
		}
	}

	normalized := make([]WholesalePriceInput, 0, len(inputs))
	for _, input := range inputs {
		skuCode := strings.TrimSpace(input.SKUCode)
		switch {
		case input.SKUID > 0:
			sku, ok := skuByID[input.SKUID]
			if !ok {
				return nil, ErrWholesalePriceInvalid
			}
			if skuCode != "" && !strings.EqualFold(skuCode, sku.SKUCode) {
				return nil, ErrWholesalePriceInvalid
			}
			input.SKUCode = strings.TrimSpace(sku.SKUCode)
		case skuCode != "":
			sku, ok := skuByCode[strings.ToLower(skuCode)]
			if !ok {
				return nil, ErrWholesalePriceInvalid
			}
			input.SKUID = sku.ID
			input.SKUCode = strings.TrimSpace(sku.SKUCode)
		default:
			input.SKUCode = ""
		}
		normalized = append(normalized, input)
	}
	return normalizeWholesalePriceInputs(normalized)
}

func wholesaleTierUniqueKey(skuID uint, skuCode string, minQuantity int) string {
	return fmt.Sprintf("%s:%d", wholesaleTierScopeKey(skuID, skuCode), minQuantity)
}

func wholesaleTierScopeKey(skuID uint, skuCode string) string {
	if code := strings.ToLower(strings.TrimSpace(skuCode)); code != "" {
		return "code:" + code
	}
	if skuID > 0 {
		return fmt.Sprintf("id:%d", skuID)
	}
	return "all"
}

// ResolveWholesaleUnitPrice 根据商品批发价阶梯解析成交单价。
// 仅在阶梯单价低于当前基准单价时生效，避免错误配置导致价格上浮。
func ResolveWholesaleUnitPrice(product *models.Product, baseUnitPrice decimal.Decimal, quantity int) (unitPrice decimal.Decimal, discount decimal.Decimal, matched bool) {
	return resolveWholesaleUnitPrice(product, baseUnitPrice, 0, "", quantity, quantity)
}

// ResolveWholesaleUnitPriceWithMatchQuantity 按 matchQuantity 判断批发档位，按 quantity 计算当前行优惠。
// 商品存在多 SKU 时，批发门槛按同一商品总购买数判断，但每个订单行只计算自己的优惠金额。
func ResolveWholesaleUnitPriceWithMatchQuantity(product *models.Product, baseUnitPrice decimal.Decimal, matchQuantity, quantity int) (unitPrice decimal.Decimal, discount decimal.Decimal, matched bool) {
	return resolveWholesaleUnitPrice(product, baseUnitPrice, 0, "", matchQuantity, quantity)
}

// ResolveWholesaleUnitPriceForSKU 按指定 SKU 的批发价阶梯解析成交单价。
func ResolveWholesaleUnitPriceForSKU(product *models.Product, baseUnitPrice decimal.Decimal, skuID uint, skuCode string, matchQuantity, quantity int) (unitPrice decimal.Decimal, discount decimal.Decimal, matched bool) {
	return resolveWholesaleUnitPrice(product, baseUnitPrice, skuID, skuCode, matchQuantity, quantity)
}

// resolveWholesaleUnitPrice 解析单个订单行的批发成交价。
// matchQuantity 用于判定通用档位，quantity 为当前行数量、仅用于计算本行优惠金额。
// SKU 专属批发价优先于通用批发价；某 SKU 配了专属阶梯后，不再回退通用阶梯。
func resolveWholesaleUnitPrice(product *models.Product, baseUnitPrice decimal.Decimal, skuID uint, skuCode string, matchQuantity, quantity int) (unitPrice decimal.Decimal, discount decimal.Decimal, matched bool) {
	base := baseUnitPrice.Round(2)
	if product == nil || matchQuantity <= 0 || quantity <= 0 || base.LessThanOrEqual(decimal.Zero) || len(product.WholesalePrices) == 0 {
		return base, decimal.Zero, false
	}

	hasSpecificTier := hasWholesaleSpecificTier(product.WholesalePrices, skuID, skuCode)
	var best *models.WholesalePriceTier
	for i := range product.WholesalePrices {
		tier := &product.WholesalePrices[i]
		if tier.MinQuantity <= 0 || tier.UnitPrice.Decimal.LessThanOrEqual(decimal.Zero) {
			continue
		}
		priority := wholesaleTierMatchPriority(tier, skuID, skuCode)
		if priority <= 0 {
			continue
		}
		if hasSpecificTier && priority < 2 {
			continue
		}
		if !hasSpecificTier && priority != 1 {
			continue
		}
		tierMatchQuantity := matchQuantity
		if priority >= 2 {
			tierMatchQuantity = quantity
		}
		if tierMatchQuantity < tier.MinQuantity {
			continue
		}
		if best == nil || tier.UnitPrice.Decimal.LessThan(best.UnitPrice.Decimal) {
			best = tier
		}
	}
	if best == nil {
		return base, decimal.Zero, false
	}

	tierPrice := best.UnitPrice.Decimal.Round(2)
	if tierPrice.GreaterThanOrEqual(base) {
		return base, decimal.Zero, false
	}

	discount = base.Sub(tierPrice).Mul(decimal.NewFromInt(int64(quantity))).Round(2)
	return tierPrice, discount, true
}

func hasWholesaleSpecificTier(tiers models.WholesalePriceTiers, skuID uint, skuCode string) bool {
	for i := range tiers {
		if wholesaleTierMatchPriority(&tiers[i], skuID, skuCode) >= 2 {
			return true
		}
	}
	return false
}

func wholesaleTierMatchPriority(tier *models.WholesalePriceTier, skuID uint, skuCode string) int {
	if tier == nil {
		return 0
	}
	if tier.SKUID <= 0 && strings.TrimSpace(tier.SKUCode) == "" {
		return 1
	}
	tierCode := strings.ToLower(strings.TrimSpace(tier.SKUCode))
	skuCode = strings.ToLower(strings.TrimSpace(skuCode))
	if tier.SKUID > 0 {
		if skuID <= 0 || tier.SKUID != skuID {
			return 0
		}
		if tierCode != "" && tierCode != skuCode {
			return 0
		}
		return 3
	}
	if tierCode != "" && tierCode == skuCode {
		return 2
	}
	return 0
}

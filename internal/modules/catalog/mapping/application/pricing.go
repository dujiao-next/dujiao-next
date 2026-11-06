package application

import (
	"strconv"

	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/shopspring/decimal"
)

// maxCategoryDepth 沿父分类向上查找加价时的最大层数，防止上游分类数据成环导致死循环。
const maxCategoryDepth = 16

// parseCategoryMarkup 把 category_markups 里的值转成百分比。
// 值只有两个来源：normalizeCategoryMarkups 写入前已转成 float64，从库里读回来
// （JSON 列）同样是 float64。无法解析或为负数时视为未配置。
func parseCategoryMarkup(raw interface{}) (decimal.Decimal, bool) {
	value, ok := raw.(float64)
	if !ok {
		return decimal.Zero, false
	}
	percent := decimal.NewFromFloat(value)
	if percent.IsNegative() {
		return decimal.Zero, false
	}
	return percent, true
}

// categoryMarkupsNeedParents 判断按分类算价是否真的需要父分类链。
// 该分类自己就配了加价时不必向上找，可以省掉一次 ListCategories。
// 上游分类 ID 为 0（未分类）时也无需继承——ResolveCategoryMarkup 直接返回默认值。
func categoryMarkupsNeedParents(markups jsonmap.JSON, upstreamCategoryID uint) bool {
	if len(markups) == 0 || upstreamCategoryID == 0 {
		return false
	}
	if _, ok := markups[strconv.FormatUint(uint64(upstreamCategoryID), 10)]; ok {
		return false
	}
	return true
}

// ResolveCategoryMarkup 返回上游分类应使用的加价百分比。
// 先取该分类自己的配置；没有则沿父分类向上找（子分类继承父分类）；都没有则用连接默认加价。
// parents 为「上游分类 ID → 父分类 ID」，可为 nil（此时不继承，只看分类自身）。
func ResolveCategoryMarkup(defaultPercent decimal.Decimal, markups jsonmap.JSON, upstreamCategoryID uint, parents map[uint]uint) decimal.Decimal {
	if len(markups) == 0 || upstreamCategoryID == 0 {
		return defaultPercent
	}
	current := upstreamCategoryID
	for depth := 0; depth < maxCategoryDepth && current != 0; depth++ {
		if raw, ok := markups[strconv.FormatUint(uint64(current), 10)]; ok {
			if percent, valid := parseCategoryMarkup(raw); valid {
				return percent
			}
		}
		current = parents[current]
	}
	return defaultPercent
}

var (
	hundred    = decimal.NewFromInt(100)
	pointOne   = decimal.NewFromFloat(0.1)
	roundScale = int32(2)
)

// convertCurrency 仅按汇率转换，不应用加价。
// 用于计算成本价：上游价格 × 汇率 = 本地币种成本（不含加价利润）。
// exchangeRate 为 0 或负数时视为 1（同币种）。
func convertCurrency(upstreamPrice, exchangeRate decimal.Decimal) decimal.Decimal {
	if exchangeRate.LessThanOrEqual(decimal.Zero) {
		exchangeRate = decimal.NewFromInt(1)
	}
	return upstreamPrice.Mul(exchangeRate)
}

// CalculateLocalPrice 先按汇率转换，再应用加价和取整。
// 计算链路：上游价格 × 汇率 → 加价 → 取整 → 本地售价。
// exchangeRate 为 0 或负数时视为 1（同币种）。
func CalculateLocalPrice(upstreamPrice, exchangeRate, markupPercent decimal.Decimal, roundingMode string) decimal.Decimal {
	if exchangeRate.LessThanOrEqual(decimal.Zero) {
		exchangeRate = decimal.NewFromInt(1)
	}
	converted := upstreamPrice.Mul(exchangeRate)
	return CalculateMarkedUpPrice(converted, markupPercent, roundingMode)
}

// CalculateMarkedUpPrice 根据加价百分比计算本地售价。
// markupPercent=100 表示上浮 100%，即 upstream × 2。
// markupPercent=0 时直接返回原价（向后兼容）。
func CalculateMarkedUpPrice(upstreamPrice, markupPercent decimal.Decimal, roundingMode string) decimal.Decimal {
	if markupPercent.IsZero() {
		return upstreamPrice.Round(roundScale)
	}

	// result = upstreamPrice * (1 + markupPercent / 100)
	multiplier := decimal.NewFromInt(1).Add(markupPercent.Div(hundred))
	result := upstreamPrice.Mul(multiplier)

	if result.IsNegative() {
		return decimal.Zero
	}

	switch roundingMode {
	case "ceil_int":
		// 向上取整到整数：12.01 → 13
		if result.Equal(result.Floor()) {
			return result
		}
		return result.Ceil()
	case "ceil_tenth":
		// 向上取整到 0.1：12.34 → 12.40
		scaled := result.Div(pointOne)
		if scaled.Equal(scaled.Floor()) {
			return result.Round(1)
		}
		return scaled.Ceil().Mul(pointOne)
	default:
		return result.Round(roundScale)
	}
}

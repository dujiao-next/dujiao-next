package application

import "github.com/shopspring/decimal"

var (
	hundred    = decimal.NewFromInt(100)
	pointOne   = decimal.NewFromFloat(0.1)
	pointCents = decimal.NewFromFloat(0.01)
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
	case "ceil_cent":
		// 以 0.01 为最小单位向上取整，适用于 USDT 两位小数售价。
		scaled := result.Div(pointCents)
		return scaled.Ceil().Mul(pointCents)
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

package contract

import "time"

const (
	DefaultOrderMaxRefundDays = 30
	OrderMaxRefundDaysMin     = 0
	OrderMaxRefundDaysLimit   = 3650
)

// OrderRefundConfig 订单与钱包充值共用的退款期限配置。
type OrderRefundConfig struct {
	MaxRefundDays int `json:"max_refund_days"`
}

// OrderRefundConfigReader 读取当前生效的退款期限配置。
type OrderRefundConfigReader interface {
	GetOrderRefundConfig() (OrderRefundConfig, error)
}

// DefaultOrderRefundConfig 返回默认退款期限配置。
func DefaultOrderRefundConfig() OrderRefundConfig {
	return OrderRefundConfig{MaxRefundDays: DefaultOrderMaxRefundDays}
}

// NormalizeOrderRefundConfig 规范退款期限，零表示不限制。
func NormalizeOrderRefundConfig(cfg OrderRefundConfig) OrderRefundConfig {
	if cfg.MaxRefundDays < OrderMaxRefundDaysMin {
		cfg.MaxRefundDays = DefaultOrderMaxRefundDays
	}
	if cfg.MaxRefundDays > OrderMaxRefundDaysLimit {
		cfg.MaxRefundDays = OrderMaxRefundDaysLimit
	}
	return cfg
}

// IsOrderRefundWindowExpired 判断是否超出退款期限，优先以支付时间计算，缺失时使用创建时间。
func IsOrderRefundWindowExpired(createdAt time.Time, paidAt *time.Time, maxRefundDays int, now time.Time) bool {
	days := NormalizeOrderRefundConfig(OrderRefundConfig{MaxRefundDays: maxRefundDays}).MaxRefundDays
	if days == 0 {
		return false
	}
	baseAt := createdAt
	if paidAt != nil && !paidAt.IsZero() {
		baseAt = *paidAt
	}
	return !baseAt.IsZero() && now.After(baseAt.AddDate(0, 0, days))
}

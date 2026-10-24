package settingsapp

import (
	"encoding/json"
	"errors"
	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"net/mail"
	"strings"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

var (
	ErrInvalidEmail          = errors.New("invalid email")
	ErrEmailDomainNotAllowed = errors.New("email domain not allowed")
)

const (
	orderConfigFieldPaymentExpireMinutes = "payment_expire_minutes"
	OrderConfigFieldMaxRefundDays        = "max_refund_days"

	orderPaymentExpireMinutesDefault = 15
	orderPaymentExpireMinutesMin     = 1
	orderPaymentExpireMinutesMax     = 10080

	orderRefundMaxDaysDefault = 30
	orderRefundMaxDaysMin     = 0
	orderRefundMaxDaysMax     = 3650
)

// OrderConfig 订单配置。
type OrderConfig struct {
	PaymentExpireMinutes int `json:"payment_expire_minutes"`
	MaxRefundDays        int `json:"max_refund_days"`
}

// OrderRefundConfig 订单退款配置。
type OrderRefundConfig struct {
	MaxRefundDays int `json:"max_refund_days"`
}

// SiteBrand 站点品牌信息
type SiteBrand struct {
	SiteName string
	SiteURL  string
}

// RegistrationEmailDomainPolicy 描述注册邮箱域名白名单策略。
type RegistrationEmailDomainPolicy struct {
	Enabled        bool
	AllowedDomains []string
}

// PaymentFeeConfig 控制新支付的手续费承担方式，以及旧版加收链接的过渡策略。
// 配置只影响后续创建/选择支付，不会重写单笔支付已经保存的 FeePolicy 快照。
type PaymentFeeConfig struct {
	CustomerFeeEnabled         bool `json:"customer_fee_enabled"`
	ReuseLegacyOrderFeePayment bool `json:"reuse_legacy_order_fee_payment"`
}

// DefaultPaymentFeeConfig 默认遵循文档语义：手续费由商户承担，旧加收链接由新链接替换。
func DefaultPaymentFeeConfig() PaymentFeeConfig {
	return PaymentFeeConfig{}
}

// NormalizePaymentFeeConfig 将支付手续费配置限制在明确的布尔开关内。
func NormalizePaymentFeeConfig(value jsonmap.JSON) jsonmap.JSON {
	config := DefaultPaymentFeeConfig()
	if value != nil {
		config.CustomerFeeEnabled = settingsvalue.ParseBool(value[constants.SettingFieldCustomerFeeEnabled])
		config.ReuseLegacyOrderFeePayment = settingsvalue.ParseBool(value[constants.SettingFieldReuseLegacyOrderFeePayment])
	}
	return jsonmap.JSON{
		constants.SettingFieldCustomerFeeEnabled:         config.CustomerFeeEnabled,
		constants.SettingFieldReuseLegacyOrderFeePayment: config.ReuseLegacyOrderFeePayment,
	}
}

// GetConfig 获取站点配置（合并默认值）
func (s *Service) GetConfig(defaults map[string]interface{}) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	for k, v := range defaults {
		data[k] = v
	}
	if s == nil {
		return data, nil
	}

	value, err := s.GetByKey(constants.SettingKeySiteConfig)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return data, nil
	}

	for k, v := range value {
		data[k] = v
	}
	return data, nil
}

// DefaultOrderConfig 默认订单配置。
func DefaultOrderConfig() OrderConfig {
	return OrderConfig{
		PaymentExpireMinutes: orderPaymentExpireMinutesDefault,
		MaxRefundDays:        orderRefundMaxDaysDefault,
	}
}

// NormalizeOrderConfig 归一化订单配置。
func NormalizeOrderConfig(cfg OrderConfig) OrderConfig {
	if cfg.PaymentExpireMinutes < orderPaymentExpireMinutesMin {
		cfg.PaymentExpireMinutes = orderPaymentExpireMinutesDefault
	}
	if cfg.PaymentExpireMinutes > orderPaymentExpireMinutesMax {
		cfg.PaymentExpireMinutes = orderPaymentExpireMinutesMax
	}
	if cfg.MaxRefundDays < orderRefundMaxDaysMin {
		cfg.MaxRefundDays = orderRefundMaxDaysDefault
	}
	if cfg.MaxRefundDays > orderRefundMaxDaysMax {
		cfg.MaxRefundDays = orderRefundMaxDaysMax
	}
	return cfg
}

// DefaultOrderRefundConfig 默认订单退款配置。
func DefaultOrderRefundConfig() OrderRefundConfig {
	return OrderRefundConfig{
		MaxRefundDays: DefaultOrderConfig().MaxRefundDays,
	}
}

// NormalizeOrderRefundConfig 归一化订单退款配置。
func NormalizeOrderRefundConfig(cfg OrderRefundConfig) OrderRefundConfig {
	normalized := NormalizeOrderConfig(OrderConfig{
		PaymentExpireMinutes: DefaultOrderConfig().PaymentExpireMinutes,
		MaxRefundDays:        cfg.MaxRefundDays,
	})
	return OrderRefundConfig{MaxRefundDays: normalized.MaxRefundDays}
}

// orderConfigFromJSON 从 JSON map 解析订单配置。
func orderConfigFromJSON(raw jsonmap.JSON, fallback OrderConfig) OrderConfig {
	result := NormalizeOrderConfig(fallback)
	if raw == nil {
		return result
	}
	if parsed, err := parseSettingInt(raw[orderConfigFieldPaymentExpireMinutes]); err == nil {
		result.PaymentExpireMinutes = parsed
	}
	if parsed, err := parseSettingInt(raw[OrderConfigFieldMaxRefundDays]); err == nil {
		result.MaxRefundDays = parsed
	}
	return NormalizeOrderConfig(result)
}

// OrderConfigToMap 将订单配置转为 map 用于存储。
func OrderConfigToMap(cfg OrderConfig) jsonmap.JSON {
	normalized := NormalizeOrderConfig(cfg)
	data, err := json.Marshal(normalized)
	if err != nil {
		return jsonmap.JSON{}
	}
	var result jsonmap.JSON
	_ = json.Unmarshal(data, &result)
	return result
}

func defaultOrderConfigWithFallback(defaultCfg config.OrderConfig, serviceDefault config.OrderConfig, useServiceDefault bool) OrderConfig {
	cfg := DefaultOrderConfig()
	if useServiceDefault {
		if serviceDefault.PaymentExpireMinutes > 0 {
			cfg.PaymentExpireMinutes = serviceDefault.PaymentExpireMinutes
		}
		if serviceDefault.MaxRefundDays >= orderRefundMaxDaysMin && serviceDefault.MaxRefundDays <= orderRefundMaxDaysMax {
			cfg.MaxRefundDays = serviceDefault.MaxRefundDays
		}
	}
	if defaultCfg.PaymentExpireMinutes > 0 {
		cfg.PaymentExpireMinutes = defaultCfg.PaymentExpireMinutes
	}
	// 0 表示不限制，仅当显式传入正数时覆盖服务默认值（服务默认值可由 config.yaml 提供 0）。
	if defaultCfg.MaxRefundDays > 0 {
		cfg.MaxRefundDays = defaultCfg.MaxRefundDays
	}
	return NormalizeOrderConfig(cfg)
}

// GetOrderConfig 获取订单配置。
func (s *Service) GetOrderConfig(defaultCfg config.OrderConfig) (OrderConfig, error) {
	var serviceDefault config.OrderConfig
	useServiceDefault := false
	if s != nil {
		serviceDefault = s.defaultOrderConfig
		useServiceDefault = s.hasDefaultOrderConfig
	}
	fallback := defaultOrderConfigWithFallback(defaultCfg, serviceDefault, useServiceDefault)
	if s == nil {
		return fallback, nil
	}
	value, err := s.GetByKey(constants.SettingKeyOrderConfig)
	if err != nil {
		return fallback, err
	}
	return orderConfigFromJSON(value, fallback), nil
}

// GetOrderRefundConfig 获取订单退款配置。
func (s *Service) GetOrderRefundConfig() (OrderRefundConfig, error) {
	fallback := DefaultOrderRefundConfig()
	cfg, err := s.GetOrderConfig(config.OrderConfig{})
	if err != nil {
		return fallback, err
	}
	return OrderRefundConfig{MaxRefundDays: cfg.MaxRefundDays}, nil
}

// IsOrderRefundWindowExpired 判断订单是否已超过可退款时间窗口（优先 paidAt，其次 createdAt）。
func IsOrderRefundWindowExpired(createdAt time.Time, paidAt *time.Time, maxRefundDays int, now time.Time) bool {
	normalizedDays := NormalizeOrderRefundConfig(OrderRefundConfig{
		MaxRefundDays: maxRefundDays,
	}).MaxRefundDays
	if normalizedDays == 0 {
		return false
	}
	baseAt := createdAt
	if paidAt != nil && !paidAt.IsZero() {
		baseAt = *paidAt
	}
	if baseAt.IsZero() {
		return false
	}
	deadline := baseAt.AddDate(0, 0, normalizedDays)
	return now.After(deadline)
}

// GetOrderPaymentExpireMinutes 获取订单超时分钟配置
func (s *Service) GetOrderPaymentExpireMinutes(defaultValue int) (int, error) {
	fallback := defaultValue
	if fallback < orderPaymentExpireMinutesMin {
		fallback = orderPaymentExpireMinutesDefault
	}
	if fallback > orderPaymentExpireMinutesMax {
		fallback = orderPaymentExpireMinutesMax
	}
	if s == nil {
		return fallback, nil
	}
	cfg, err := s.GetOrderConfig(config.OrderConfig{
		PaymentExpireMinutes: fallback,
	})
	if err != nil {
		return fallback, err
	}
	return cfg.PaymentExpireMinutes, nil
}

// GetRegistrationEnabled 获取注册开关
func (s *Service) GetRegistrationEnabled(defaultValue bool) (bool, error) {
	if s == nil {
		return defaultValue, nil
	}
	value, err := s.GetByKey(constants.SettingKeyRegistrationConfig)
	if err != nil {
		return defaultValue, err
	}
	if value == nil {
		return defaultValue, nil
	}
	raw, ok := value[constants.SettingFieldRegistrationEnabled]
	if !ok {
		return defaultValue, nil
	}
	return settingsvalue.ParseBool(raw), nil
}

// GetRegistrationInviteOnly 获取"仅凭邀请码注册"开关（默认关闭）
func (s *Service) GetRegistrationInviteOnly() (bool, error) {
	if s == nil {
		return false, nil
	}
	value, err := s.GetByKey(constants.SettingKeyRegistrationConfig)
	if err != nil || value == nil {
		return false, err
	}
	return settingsvalue.ParseBool(value[constants.SettingFieldRegistrationInviteOnly]), nil
}

// GetEmailVerificationEnabled 获取邮箱验证开关
func (s *Service) GetEmailVerificationEnabled(defaultValue bool) (bool, error) {
	if s == nil {
		return defaultValue, nil
	}
	value, err := s.GetByKey(constants.SettingKeyRegistrationConfig)
	if err != nil {
		return defaultValue, err
	}
	if value == nil {
		return defaultValue, nil
	}
	raw, ok := value[constants.SettingFieldEmailVerificationEnabled]
	if !ok {
		return defaultValue, nil
	}
	return settingsvalue.ParseBool(raw), nil
}

// GetRegistrationEmailDomainPolicy 获取注册邮箱域名白名单策略。
func (s *Service) GetRegistrationEmailDomainPolicy() (RegistrationEmailDomainPolicy, error) {
	policy := RegistrationEmailDomainPolicy{Enabled: false, AllowedDomains: []string{}}
	if s == nil {
		return policy, nil
	}
	value, err := s.GetByKey(constants.SettingKeyRegistrationConfig)
	if err != nil {
		return policy, err
	}
	if value == nil {
		return policy, nil
	}
	if raw, ok := value[constants.SettingFieldEmailDomainAllowlistEnabled]; ok {
		policy.Enabled = settingsvalue.ParseBool(raw)
	}
	policy.AllowedDomains = normalizeRegistrationEmailDomains(value[constants.SettingFieldAllowedEmailDomains])
	return policy, nil
}

// CheckRegistrationEmailDomainAllowed 校验邮箱格式与注册域名白名单策略。
func CheckRegistrationEmailDomainAllowed(email string, policy RegistrationEmailDomainPolicy) error {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return ErrInvalidEmail
	}
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized {
		return ErrInvalidEmail
	}
	if !policy.Enabled {
		return nil
	}
	at := strings.LastIndex(parsed.Address, "@")
	if at < 0 || at == len(parsed.Address)-1 {
		return ErrInvalidEmail
	}
	domain := strings.ToLower(strings.TrimSpace(parsed.Address[at+1:]))
	for _, allowed := range normalizeRegistrationEmailDomains(policy.AllowedDomains) {
		if domain == allowed {
			return nil
		}
	}
	return ErrEmailDomainNotAllowed
}

// GetSiteCurrency 获取站点币种配置
func (s *Service) GetSiteCurrency(defaultValue string) (string, error) {
	fallback := normalizeSiteCurrency(defaultValue)
	if s == nil {
		return fallback, nil
	}
	value, err := s.GetByKey(constants.SettingKeySiteConfig)
	if err != nil {
		return fallback, err
	}
	if value == nil {
		return fallback, nil
	}
	raw, ok := value[constants.SettingFieldSiteCurrency]
	if !ok {
		return fallback, nil
	}
	return normalizeSiteCurrency(raw), nil
}

// GetSiteBrand 获取站点品牌配置（brand.site_name / brand.site_url）
func (s *Service) GetSiteBrand() (SiteBrand, error) {
	if s == nil {
		return SiteBrand{}, nil
	}
	value, err := s.GetByKey(constants.SettingKeySiteConfig)
	if err != nil {
		return SiteBrand{}, err
	}
	if value == nil {
		return SiteBrand{}, nil
	}
	rawBrand, ok := value["brand"]
	if !ok || rawBrand == nil {
		return SiteBrand{}, nil
	}
	brand, ok := rawBrand.(map[string]interface{})
	if !ok || brand == nil {
		return SiteBrand{}, nil
	}
	return SiteBrand{
		SiteName: normalizeSettingText(brand["site_name"]),
		SiteURL:  strings.TrimRight(normalizeSettingText(brand["site_url"]), "/"),
	}, nil
}

// GetWalletOnlyPayment 获取是否仅允许钱包余额支付
func (s *Service) GetWalletOnlyPayment() bool {
	if s == nil {
		return false
	}
	value, err := s.GetByKey(constants.SettingKeyWalletConfig)
	if err != nil || value == nil {
		return false
	}
	raw, ok := value[constants.SettingFieldWalletOnlyPayment]
	if !ok {
		return false
	}
	return settingsvalue.ParseBool(raw)
}

// GetPaymentFeeConfig 获取支付手续费与旧订单兼容配置。
func (s *Service) GetPaymentFeeConfig() PaymentFeeConfig {
	fallback := DefaultPaymentFeeConfig()
	if s == nil {
		return fallback
	}
	value, err := s.GetByKey(constants.SettingKeyPaymentConfig)
	if err != nil || value == nil {
		return fallback
	}
	normalized := NormalizePaymentFeeConfig(value)
	return PaymentFeeConfig{
		CustomerFeeEnabled:         settingsvalue.ParseBool(normalized[constants.SettingFieldCustomerFeeEnabled]),
		ReuseLegacyOrderFeePayment: settingsvalue.ParseBool(normalized[constants.SettingFieldReuseLegacyOrderFeePayment]),
	}
}

// GetWalletRechargeChannelIDs 获取钱包充值允许的支付渠道ID列表
func (s *Service) GetWalletRechargeChannelIDs() []uint {
	if s == nil {
		return nil
	}
	value, err := s.GetByKey(constants.SettingKeyWalletConfig)
	if err != nil || value == nil {
		return nil
	}
	raw, ok := value["recharge_channel_ids"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	result := make([]uint, 0, len(arr))
	for _, item := range arr {
		switch v := item.(type) {
		case float64:
			if v > 0 {
				result = append(result, uint(v))
			}
		case int:
			if v > 0 {
				result = append(result, uint(v))
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

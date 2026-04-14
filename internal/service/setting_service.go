package service

import (
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
)

// SettingService 设置业务服务
type SettingService struct {
	repo repository.SettingRepository
}

// SiteBrand 站点品牌信息
type SiteBrand struct {
	SiteName string
	SiteURL  string
}

// NewSettingService 创建设置服务
func NewSettingService(repo repository.SettingRepository) *SettingService {
	return &SettingService{repo: repo}
}

// GetConfig 获取站点配置（合并默认值）
func (s *SettingService) GetConfig(defaults map[string]interface{}) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	for k, v := range defaults {
		data[k] = v
	}

	setting, err := s.repo.GetByKey(constants.SettingKeySiteConfig)
	if err != nil {
		return nil, err
	}
	if setting == nil {
		return data, nil
	}

	for k, v := range setting.ValueJSON {
		data[k] = v
	}
	return data, nil
}

// GetByKey 获取设置
func (s *SettingService) GetByKey(key string) (models.JSON, error) {
	setting, err := s.repo.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if setting == nil {
		return nil, nil
	}
	return setting.ValueJSON, nil
}

// Update 设置值
func (s *SettingService) Update(key string, value map[string]interface{}) (models.JSON, error) {
	normalized := normalizeSettingValueByKey(key, value)

	setting, err := s.repo.Upsert(key, normalized)
	if err != nil {
		return nil, err
	}
	return setting.ValueJSON, nil
}

// Delete 删除指定配置键。
func (s *SettingService) Delete(key string) error {
	if s == nil || s.repo == nil || key == "" {
		return nil
	}
	return s.repo.DeleteByKey(key)
}

// GetOrderPaymentExpireMinutes 获取订单超时分钟配置
func (s *SettingService) GetOrderPaymentExpireMinutes(defaultValue int) (int, error) {
	if s == nil {
		return defaultValue, nil
	}
	value, err := s.GetByKey(constants.SettingKeyOrderConfig)
	if err != nil {
		return defaultValue, err
	}
	if value == nil {
		return defaultValue, nil
	}
	raw, ok := value[constants.SettingFieldPaymentExpireMinutes]
	if !ok {
		return defaultValue, nil
	}
	minutes, err := parseSettingInt(raw)
	if err != nil {
		return defaultValue, err
	}
	if minutes <= 0 {
		return defaultValue, nil
	}
	return minutes, nil
}

// GetRegistrationEnabled 获取注册开关
func (s *SettingService) GetRegistrationEnabled(defaultValue bool) (bool, error) {
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
	return parseSettingBool(raw), nil
}

// GetEmailVerificationEnabled 获取邮箱验证开关
func (s *SettingService) GetEmailVerificationEnabled(defaultValue bool) (bool, error) {
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
	return parseSettingBool(raw), nil
}

// GetSiteCurrency 获取站点币种配置
func (s *SettingService) GetSiteCurrency(defaultValue string) (string, error) {
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
func (s *SettingService) GetSiteBrand() (SiteBrand, error) {
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
func (s *SettingService) GetWalletOnlyPayment() bool {
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
	return parseSettingBool(raw)
}

// GetCallbackRoutes 获取自定义回调路由配置。未配置时返回 nil。
func (s *SettingService) GetCallbackRoutes() *CallbackRoutesSetting {
	if s == nil {
		return nil
	}
	value, err := s.GetByKey(constants.SettingKeyCallbackRoutesConfig)
	if err != nil || value == nil {
		return nil
	}
	setting := callbackRoutesSettingFromJSON(value)
	if !setting.HasCustomRoutes() {
		return nil
	}
	return &setting
}

// GetWalletRechargeChannelIDs 获取钱包充值允许的支付渠道ID列表
func (s *SettingService) GetWalletRechargeChannelIDs() []uint {
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

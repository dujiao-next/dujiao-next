package settingsapp

import (
	"github.com/dujiao-next/internal/config"
	settingscontract "github.com/dujiao-next/internal/modules/settings/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// Service 是站点设置的核心用例入口：读写、Registry 归一化与声明式副作用。
type Service struct {
	repo                  settingscontract.Store
	registry              Registry
	defaultOrderConfig    config.OrderConfig
	hasDefaultOrderConfig bool
}

// UpdateResult 包含持久化后的设置值及其声明式外部影响。
type UpdateResult struct {
	Value   jsonmap.JSON
	Effects []Effect
}

// HasEffect 判断设置更新是否声明了指定外部影响。
func (result UpdateResult) HasEffect(effect Effect) bool {
	for _, candidate := range result.Effects {
		if candidate == effect {
			return true
		}
	}
	return false
}

// NewService 创建设置服务。可选的订单配置仅在数据库尚未覆盖设置时使用。
func NewService(repo settingscontract.Store, defaultOrderConfig ...config.OrderConfig) *Service {
	service := &Service{
		repo:     repo,
		registry: defaultSettingRegistry,
	}
	if len(defaultOrderConfig) > 0 {
		service.defaultOrderConfig = defaultOrderConfig[0]
		service.hasDefaultOrderConfig = true
	}
	return service
}

// GetByKey 获取设置原始 JSON；不存在时返回 nil。
func (s *Service) GetByKey(key string) (jsonmap.JSON, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	value, found, err := s.repo.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return value, nil
}

// Update 设置值。
func (s *Service) Update(key string, value map[string]interface{}) (jsonmap.JSON, error) {
	result, err := s.UpdateWithEffects(key, value)
	if err != nil {
		return nil, err
	}
	return result.Value, nil
}

// UpdateWithEffects 设置值并返回成功写入后需要处理的声明式外部影响。
func (s *Service) UpdateWithEffects(key string, value map[string]interface{}) (UpdateResult, error) {
	if s == nil || s.repo == nil {
		return UpdateResult{}, nil
	}
	normalized := s.registry.Normalize(key, jsonmap.JSON(value))
	if err := s.registry.Validate(key, normalized); err != nil {
		return UpdateResult{}, err
	}

	stored, err := s.repo.Upsert(key, normalized)
	if err != nil {
		return UpdateResult{}, err
	}
	return UpdateResult{
		Value:   stored,
		Effects: s.registry.Effects(key),
	}, nil
}

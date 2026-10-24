package settingsapp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// Normalizer 把任意设置 JSON 归一化为可持久化形状。
type Normalizer func(value jsonmap.JSON) jsonmap.JSON

// Validator 校验归一化后的设置；返回错误时拒绝写入。
type Validator func(value jsonmap.JSON) error

// Effect 描述设置成功写入后需要由调用方处理的外部影响。
// Registry 只声明影响，不直接依赖缓存、HTTP 或其他基础设施。
type Effect string

const (
	EffectInvalidatePublicConfigCache   Effect = "invalidate_public_config_cache"
	EffectInvalidateCallbackRoutesCache Effect = "invalidate_callback_routes_cache"
)

// Definition 描述一个设置键的写入策略。
// Normalize 可以为空以支持只有外部影响的透传设置；定义至少需要一种能力。
// 校验、脱敏和读取默认值仍由现有 typed setting API 负责。
type Definition struct {
	Key       string
	Normalize Normalizer
	Validate  Validator
	Effects   []Effect
}

// Registry 是按设置键索引的不可变定义集合。
// 构造完成后没有写方法，可安全地由多个 SettingService 实例共享。
type Registry struct {
	definitions map[string]Definition
}

// NewRegistry 校验定义并构造 Registry。
func NewRegistry(definitions ...Definition) (Registry, error) {
	indexed := make(map[string]Definition, len(definitions))
	for _, definition := range definitions {
		if definition.Key == "" {
			return Registry{}, fmt.Errorf("settings registry: definition key is required")
		}
		if strings.TrimSpace(definition.Key) != definition.Key {
			return Registry{}, fmt.Errorf("settings registry: definition key %q contains surrounding whitespace", definition.Key)
		}
		if definition.Normalize == nil && len(definition.Effects) == 0 {
			return Registry{}, fmt.Errorf("settings registry: definition %q requires at least one capability", definition.Key)
		}
		if _, exists := indexed[definition.Key]; exists {
			return Registry{}, fmt.Errorf("settings registry: duplicate definition key %q", definition.Key)
		}
		seenEffects := make(map[Effect]struct{}, len(definition.Effects))
		for _, effect := range definition.Effects {
			if effect == "" {
				return Registry{}, fmt.Errorf("settings registry: definition %q contains an empty effect", definition.Key)
			}
			if _, exists := seenEffects[effect]; exists {
				return Registry{}, fmt.Errorf("settings registry: definition %q contains duplicate effect %q", definition.Key, effect)
			}
			seenEffects[effect] = struct{}{}
		}
		definition.Effects = append([]Effect(nil), definition.Effects...)
		indexed[definition.Key] = definition
	}
	return Registry{definitions: indexed}, nil
}

// MustNewRegistry 构造静态 Registry；定义无效时立即 panic，避免以不完整配置启动服务。
func MustNewRegistry(definitions ...Definition) Registry {
	registry, err := NewRegistry(definitions...)
	if err != nil {
		panic(err)
	}
	return registry
}

// Normalize 执行已登记的写入策略；未知 key 保持历史行为并原样透传。
func (registry Registry) Normalize(key string, value jsonmap.JSON) jsonmap.JSON {
	definition, exists := registry.definitions[key]
	if !exists {
		return value
	}
	if definition.Normalize == nil {
		return value
	}
	return definition.Normalize(value)
}

// Validate 执行已登记的校验；未知 key 或未声明校验时直接放行。
func (registry Registry) Validate(key string, value jsonmap.JSON) error {
	definition, exists := registry.definitions[key]
	if !exists || definition.Validate == nil {
		return nil
	}
	return definition.Validate(value)
}

// Effects 返回设置成功写入后的影响集合副本；未知 key 没有声明式副作用。
func (registry Registry) Effects(key string) []Effect {
	definition, exists := registry.definitions[key]
	if !exists || len(definition.Effects) == 0 {
		return nil
	}
	return append([]Effect(nil), definition.Effects...)
}

// Keys 返回排序后的定义键副本，供覆盖检查和诊断使用。
func (registry Registry) Keys() []string {
	keys := make([]string, 0, len(registry.definitions))
	for key := range registry.definitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

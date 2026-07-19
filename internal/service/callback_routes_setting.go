package service

import (
	"strings"
	"sync"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
)

// CallbackRoutesSetting 回调路由配置
type CallbackRoutesSetting struct {
	PaymentCallback        string
	DujiaoPayWebhook       string
	PaypalWebhook          string
	StripeWebhook          string
	UpstreamCallback       string
	GenericWebhookCallback string
}

// HasCustomRoutes 返回是否设置了任何自定义回调路由
func (s *CallbackRoutesSetting) HasCustomRoutes() bool {
	return s.PaymentCallback != "" || s.DujiaoPayWebhook != "" || s.PaypalWebhook != "" ||
		s.StripeWebhook != "" || s.UpstreamCallback != "" || s.GenericWebhookCallback != ""
}

// callbackRoutesSettingFromJSON 从 JSON map 解析回调路由配置
func callbackRoutesSettingFromJSON(value models.JSON) CallbackRoutesSetting {
	return CallbackRoutesSetting{
		PaymentCallback:        normalizeCallbackRoutePath(readString(value, constants.SettingFieldPaymentCallback, "")),
		DujiaoPayWebhook:       normalizeCallbackRoutePath(readString(value, constants.SettingFieldDujiaoPayWebhook, "")),
		PaypalWebhook:          normalizeCallbackRoutePath(readString(value, constants.SettingFieldPaypalWebhook, "")),
		StripeWebhook:          normalizeCallbackRoutePath(readString(value, constants.SettingFieldStripeWebhook, "")),
		UpstreamCallback:       normalizeCallbackRoutePath(readString(value, constants.SettingFieldUpstreamCallback, "")),
		GenericWebhookCallback: normalizeCallbackRoutePath(readString(value, constants.SettingFieldGenericWebhookCallback, "")),
	}
}

// CallbackRoutesSettingToMap 将回调路由配置序列化为 JSON map
func CallbackRoutesSettingToMap(s CallbackRoutesSetting) models.JSON {
	return models.JSON{
		constants.SettingFieldPaymentCallback:        s.PaymentCallback,
		constants.SettingFieldDujiaoPayWebhook:       s.DujiaoPayWebhook,
		constants.SettingFieldPaypalWebhook:          s.PaypalWebhook,
		constants.SettingFieldStripeWebhook:          s.StripeWebhook,
		constants.SettingFieldUpstreamCallback:       s.UpstreamCallback,
		constants.SettingFieldGenericWebhookCallback: s.GenericWebhookCallback,
	}
}

// normalizeCallbackRoutesSetting 归一化回调路由配置
func normalizeCallbackRoutesSetting(value map[string]interface{}) models.JSON {
	setting := callbackRoutesSettingFromJSON(models.JSON(value))
	deduplicateCallbackRoutes(&setting)
	return CallbackRoutesSettingToMap(setting)
}

// reservedRoutePrefixes 已有路由前缀，自定义回调路由不得与之冲突
var reservedRoutePrefixes = []string{
	"/api/v1/public/",
	"/api/v1/admin/",
	"/api/v1/auth/",
	"/api/v1/guest/",
	"/api/v1/channel/",
	"/api/v1/upstream/api/",
	"/api/v1/user/",
}

// normalizeCallbackRoutePath 归一化单条回调路由路径。
// 空字符串表示使用默认值；非空路径必须以 /api/ 开头，不含 query string，
// 且不得与已有路由前缀冲突。
func normalizeCallbackRoutePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	// 去除 query string 和 fragment
	if idx := strings.IndexAny(path, "?#"); idx != -1 {
		path = path[:idx]
	}

	// 去除尾部斜杠
	path = strings.TrimRight(path, "/")

	// 必须以 /api/ 开头
	if !strings.HasPrefix(path, "/api/") {
		return ""
	}

	// 不得与已有路由前缀冲突
	pathWithSlash := path + "/"
	for _, prefix := range reservedRoutePrefixes {
		if strings.HasPrefix(pathWithSlash, prefix) || strings.HasPrefix(prefix, pathWithSlash) {
			return ""
		}
	}

	return path
}

// deduplicateCallbackRoutes 去除重复路径：后出现的重复路径被清空
func deduplicateCallbackRoutes(s *CallbackRoutesSetting) {
	seen := make(map[string]bool, 6)
	fields := []*string{
		&s.PaymentCallback,
		&s.DujiaoPayWebhook,
		&s.PaypalWebhook,
		&s.StripeWebhook,
		&s.UpstreamCallback,
		&s.GenericWebhookCallback,
	}
	for _, f := range fields {
		if *f == "" {
			continue
		}
		if seen[*f] {
			*f = "" // 重复路径清空
		} else {
			seen[*f] = true
		}
	}
}

// --- 回调路由内存缓存 ---

var callbackRoutesCache struct {
	mu      sync.RWMutex
	routes  *CallbackRoutesSetting
	loaded  bool
	expires time.Time
}

const callbackRoutesCacheTTL = 5 * time.Minute

// InvalidateCallbackRoutesCache 清除回调路由内存缓存（管理员更新设置时调用）
func (s *SettingService) InvalidateCallbackRoutesCache() {
	callbackRoutesCache.mu.Lock()
	callbackRoutesCache.loaded = false
	callbackRoutesCache.routes = nil
	callbackRoutesCache.expires = time.Time{}
	callbackRoutesCache.mu.Unlock()
}

// GetCallbackRoutesCached 从内存缓存获取自定义回调路由配置，缓存未命中时从 DB 加载。
func (s *SettingService) GetCallbackRoutesCached() *CallbackRoutesSetting {
	callbackRoutesCache.mu.RLock()
	if callbackRoutesCache.loaded && time.Now().Before(callbackRoutesCache.expires) {
		routes := callbackRoutesCache.routes
		callbackRoutesCache.mu.RUnlock()
		return routes
	}
	callbackRoutesCache.mu.RUnlock()

	callbackRoutesCache.mu.Lock()
	defer callbackRoutesCache.mu.Unlock()

	// 双重检查
	if callbackRoutesCache.loaded && time.Now().Before(callbackRoutesCache.expires) {
		return callbackRoutesCache.routes
	}

	routes := s.GetCallbackRoutes()
	callbackRoutesCache.routes = routes
	callbackRoutesCache.loaded = true
	callbackRoutesCache.expires = time.Now().Add(callbackRoutesCacheTTL)
	return routes
}

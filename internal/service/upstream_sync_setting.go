package service

import (
	"fmt"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
)

const (
	upstreamSyncIntervalMinDefault = 5
	upstreamSyncIntervalMinMin     = 5
	upstreamSyncIntervalMinMax     = 1440 // 24h 上限，避免误填超长间隔

	upstreamSyncPageSizeDefault    = 50
	upstreamSyncPageSizeMin        = 10
	upstreamSyncPageSizeMax        = 200
	upstreamSyncMaxPagesDefault    = 200
	upstreamSyncMaxPagesMin        = 10
	upstreamSyncMaxPagesMax        = 500
	upstreamSyncConnConcurrencyDef = 3
	upstreamSyncConnConcurrencyMin = 1
	upstreamSyncConnConcurrencyMax = 10
)

// UpstreamSyncConfig 上游同步配置。
type UpstreamSyncConfig struct {
	IntervalMinutes           int  `json:"interval_minutes"`
	PreOrderStockCheckEnabled bool `json:"pre_order_stock_check_enabled"`
	SyncPageSize              int  `json:"sync_page_size"`
	SyncMaxPages              int  `json:"sync_max_pages"`
	SyncConnConcurrency       int  `json:"sync_conn_concurrency"`
}

// DefaultUpstreamSyncConfig 默认上游同步配置。
func DefaultUpstreamSyncConfig() UpstreamSyncConfig {
	return UpstreamSyncConfig{
		IntervalMinutes:           upstreamSyncIntervalMinDefault,
		PreOrderStockCheckEnabled: true,
		SyncPageSize:              upstreamSyncPageSizeDefault,
		SyncMaxPages:              upstreamSyncMaxPagesDefault,
		SyncConnConcurrency:       upstreamSyncConnConcurrencyDef,
	}
}

// NormalizeUpstreamSyncConfig 归一化上游同步配置。
func NormalizeUpstreamSyncConfig(cfg UpstreamSyncConfig) UpstreamSyncConfig {
	if cfg.IntervalMinutes < upstreamSyncIntervalMinMin {
		cfg.IntervalMinutes = upstreamSyncIntervalMinDefault
	}
	if cfg.IntervalMinutes > upstreamSyncIntervalMinMax {
		cfg.IntervalMinutes = upstreamSyncIntervalMinMax
	}
	if cfg.SyncPageSize < upstreamSyncPageSizeMin {
		cfg.SyncPageSize = upstreamSyncPageSizeDefault
	}
	if cfg.SyncPageSize > upstreamSyncPageSizeMax {
		cfg.SyncPageSize = upstreamSyncPageSizeMax
	}
	if cfg.SyncMaxPages < upstreamSyncMaxPagesMin {
		cfg.SyncMaxPages = upstreamSyncMaxPagesDefault
	}
	if cfg.SyncMaxPages > upstreamSyncMaxPagesMax {
		cfg.SyncMaxPages = upstreamSyncMaxPagesMax
	}
	if cfg.SyncConnConcurrency < upstreamSyncConnConcurrencyMin {
		cfg.SyncConnConcurrency = upstreamSyncConnConcurrencyDef
	}
	if cfg.SyncConnConcurrency > upstreamSyncConnConcurrencyMax {
		cfg.SyncConnConcurrency = upstreamSyncConnConcurrencyMax
	}
	return cfg
}

// upstreamSyncConfigFromJSON 从 JSON map 解析上游同步配置。
func upstreamSyncConfigFromJSON(raw models.JSON, fallback UpstreamSyncConfig) UpstreamSyncConfig {
	result := NormalizeUpstreamSyncConfig(fallback)
	if raw == nil {
		return result
	}
	if v, err := parseSettingInt(raw[constants.SettingFieldUpstreamSyncIntervalMin]); err == nil {
		result.IntervalMinutes = v
	}
	if v, ok := raw[constants.SettingFieldUpstreamPreOrderCheck]; ok {
		result.PreOrderStockCheckEnabled = parseSettingBool(v)
	}
	if v, err := parseSettingInt(raw[constants.SettingFieldUpstreamSyncPageSize]); err == nil {
		result.SyncPageSize = v
	}
	if v, err := parseSettingInt(raw[constants.SettingFieldUpstreamSyncMaxPages]); err == nil {
		result.SyncMaxPages = v
	}
	if v, err := parseSettingInt(raw[constants.SettingFieldUpstreamSyncConcurrency]); err == nil {
		result.SyncConnConcurrency = v
	}
	return NormalizeUpstreamSyncConfig(result)
}

// UpstreamSyncConfigToMap 将配置转为 map 用于存储。
func UpstreamSyncConfigToMap(cfg UpstreamSyncConfig) models.JSON {
	normalized := NormalizeUpstreamSyncConfig(cfg)
	return models.JSON{
		constants.SettingFieldUpstreamSyncIntervalMin: normalized.IntervalMinutes,
		constants.SettingFieldUpstreamPreOrderCheck:   normalized.PreOrderStockCheckEnabled,
		constants.SettingFieldUpstreamSyncPageSize:    normalized.SyncPageSize,
		constants.SettingFieldUpstreamSyncMaxPages:    normalized.SyncMaxPages,
		constants.SettingFieldUpstreamSyncConcurrency: normalized.SyncConnConcurrency,
	}
}

// GetUpstreamSyncConfig 获取上游同步配置。
// fallbackInterval 来自 config.yml 的兜底值（如 "5m"、"10m"），仅在 DB 未配置时使用。
func (s *SettingService) GetUpstreamSyncConfig(fallbackInterval string) (UpstreamSyncConfig, error) {
	fallback := DefaultUpstreamSyncConfig()
	if mins := parseDurationToMinutes(fallbackInterval); mins > 0 {
		fallback.IntervalMinutes = mins
	}
	fallback = NormalizeUpstreamSyncConfig(fallback)
	if s == nil {
		return fallback, nil
	}
	value, err := s.GetByKey(constants.SettingKeyUpstreamSyncConfig)
	if err != nil {
		return fallback, err
	}
	return upstreamSyncConfigFromJSON(value, fallback), nil
}

// GetUpstreamSyncInterval 返回归一化后的同步间隔 Duration，便于 scheduler 直接使用。
func (s *SettingService) GetUpstreamSyncInterval(fallbackInterval string) (time.Duration, error) {
	cfg, err := s.GetUpstreamSyncConfig(fallbackInterval)
	if err != nil {
		return time.Duration(cfg.IntervalMinutes) * time.Minute, err
	}
	return time.Duration(cfg.IntervalMinutes) * time.Minute, nil
}

// parseDurationToMinutes 将 "5m"/"1h" 等字符串转换为分钟数，解析失败返回 0。
func parseDurationToMinutes(s string) int {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	mins := int(d / time.Minute)
	if mins <= 0 {
		return 0
	}
	return mins
}

// FormatUpstreamSyncIntervalForScheduler 返回 asynq scheduler "@every" 可接受的间隔字符串。
func FormatUpstreamSyncIntervalForScheduler(d time.Duration) string {
	if d < time.Duration(upstreamSyncIntervalMinMin)*time.Minute {
		d = time.Duration(upstreamSyncIntervalMinDefault) * time.Minute
	}
	return fmt.Sprintf("%dm", int(d/time.Minute))
}

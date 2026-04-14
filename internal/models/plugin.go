package models

import "time"

// 插件状态常量。
const (
	PluginStatusUploaded              = "uploaded"
	PluginStatusInstalled             = "installed"
	PluginStatusEnabled               = "enabled"
	PluginStatusDisabled              = "disabled"
	PluginStatusLoadFailed            = "load_failed"
	PluginStatusUpgradePendingRestart = "upgrade_pending_restart"
	PluginStatusRemovePendingRestart  = "remove_pending_restart"
)

// Plugin 插件主记录。
type Plugin struct {
	ID             string      `gorm:"primaryKey;type:varchar(120)" json:"id"`
	Name           string      `gorm:"type:varchar(200);not null" json:"name"`
	Type           string      `gorm:"type:varchar(40);index;not null" json:"type"`
	Author         string      `gorm:"type:varchar(120);not null;default:''" json:"author"`
	Summary        string      `gorm:"type:text" json:"summary"`
	Description    string      `gorm:"type:text" json:"description"`
	Icon           string      `gorm:"type:varchar(500);not null;default:''" json:"icon"`
	Cover          string      `gorm:"type:varchar(500);not null;default:''" json:"cover"`
	Source         string      `gorm:"type:varchar(40);index;not null;default:'local'" json:"source"`
	Status         string      `gorm:"type:varchar(60);index;not null;default:'uploaded'" json:"status"`
	ReviewStatus   string      `gorm:"type:varchar(40);index;not null;default:'pending'" json:"review_status"`
	CurrentVersion string      `gorm:"type:varchar(60);index;not null;default:''" json:"current_version"`
	PendingVersion string      `gorm:"type:varchar(60);not null;default:''" json:"pending_version"`
	HostAPIVersion string      `gorm:"type:varchar(60);not null;default:''" json:"host_api_version"`
	GoVersion      string      `gorm:"type:varchar(40);not null;default:''" json:"go_version"`
	BuildTarget    string      `gorm:"type:varchar(60);not null;default:''" json:"build_target"`
	EntrySymbol    string      `gorm:"type:varchar(120);not null;default:''" json:"entry_symbol"`
	Permissions    StringArray `gorm:"type:json" json:"permissions"`
	ConfigSchema   JSON        `gorm:"type:json" json:"config_schema"`
	MetaJSON       JSON        `gorm:"type:json" json:"meta"`
	IsEnabled      bool        `gorm:"not null;default:false;index" json:"is_enabled"`
	NeedsRestart   bool        `gorm:"not null;default:false;index" json:"needs_restart"`
	LastLoadedAt   *time.Time  `json:"last_loaded_at"`
	LastError      string      `gorm:"type:text" json:"last_error"`
	CreatedAt      time.Time   `gorm:"index" json:"created_at"`
	UpdatedAt      time.Time   `gorm:"index" json:"updated_at"`
}

func (Plugin) TableName() string { return "plugins" }

// PluginVersion 插件版本记录。
type PluginVersion struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	PluginID     string    `gorm:"type:varchar(120);index:idx_plugin_versions_pid_ver,priority:1;not null" json:"plugin_id"`
	Version      string    `gorm:"type:varchar(60);index:idx_plugin_versions_pid_ver,priority:2;not null" json:"version"`
	Status       string    `gorm:"type:varchar(40);index;not null;default:'uploaded'" json:"status"`
	PackagePath  string    `gorm:"type:varchar(500);not null;default:''" json:"package_path"`
	InstallPath  string    `gorm:"type:varchar(500);not null;default:''" json:"install_path"`
	Checksum     string    `gorm:"type:varchar(128);not null;default:''" json:"checksum"`
	ManifestJSON JSON      `gorm:"type:json" json:"manifest"`
	MetaJSON     JSON      `gorm:"type:json" json:"meta"`
	IsActive     bool      `gorm:"not null;default:false;index" json:"is_active"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time `gorm:"index" json:"updated_at"`
}

func (PluginVersion) TableName() string { return "plugin_versions" }

// PluginRuntimeLog 插件运行日志。
type PluginRuntimeLog struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	PluginID      string    `gorm:"type:varchar(120);index;not null" json:"plugin_id"`
	PluginVersion string    `gorm:"type:varchar(60);index;not null;default:''" json:"plugin_version"`
	Level         string    `gorm:"type:varchar(20);index;not null;default:'info'" json:"level"`
	EventType     string    `gorm:"type:varchar(80);index;not null;default:''" json:"event_type"`
	Message       string    `gorm:"type:text;not null" json:"message"`
	DetailsJSON   JSON      `gorm:"type:json" json:"details"`
	CreatedAt     time.Time `gorm:"index" json:"created_at"`
}

func (PluginRuntimeLog) TableName() string { return "plugin_runtime_logs" }

// PluginRouteRegistry 插件路由登记。
type PluginRouteRegistry struct {
	ID        uint        `gorm:"primaryKey" json:"id"`
	PluginID  string      `gorm:"type:varchar(120);index;not null" json:"plugin_id"`
	Scope     string      `gorm:"type:varchar(20);index;not null" json:"scope"`
	Path      string      `gorm:"type:varchar(240);not null" json:"path"`
	Methods   StringArray `gorm:"type:json" json:"methods"`
	MetaJSON  JSON        `gorm:"type:json" json:"meta"`
	CreatedAt time.Time   `gorm:"index" json:"created_at"`
}

func (PluginRouteRegistry) TableName() string { return "plugin_route_registry" }

// PluginPageRegistry 插件页面登记。
type PluginPageRegistry struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PluginID  string    `gorm:"type:varchar(120);index;not null" json:"plugin_id"`
	Scope     string    `gorm:"type:varchar(20);index;not null" json:"scope"`
	RoutePath string    `gorm:"type:varchar(240);not null" json:"route_path"`
	Title     string    `gorm:"type:varchar(200);not null;default:''" json:"title"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	MetaJSON  JSON      `gorm:"type:json" json:"meta"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (PluginPageRegistry) TableName() string { return "plugin_page_registry" }

// PluginEventSubscription 插件事件订阅登记。
type PluginEventSubscription struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PluginID  string    `gorm:"type:varchar(120);index;not null" json:"plugin_id"`
	EventType string    `gorm:"type:varchar(120);index;not null" json:"event_type"`
	MetaJSON  JSON      `gorm:"type:json" json:"meta"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (PluginEventSubscription) TableName() string { return "plugin_event_subscriptions" }

// PluginMarketRegistry 在线插件库注册表。
type PluginMarketRegistry struct {
	ID              string     `gorm:"primaryKey;type:varchar(60)" json:"id"`
	Name            string     `gorm:"type:varchar(120);not null" json:"name"`
	Description     string     `gorm:"type:text" json:"description"`
	SourceType      string     `gorm:"type:varchar(40);index;not null;default:'builtin'" json:"source_type"`
	IndexURL        string     `gorm:"type:varchar(500);not null;default:''" json:"index_url"`
	IsBuiltIn       bool       `gorm:"not null;default:true" json:"is_built_in"`
	IsEnabled       bool       `gorm:"not null;default:true;index" json:"is_enabled"`
	SortOrder       int        `gorm:"not null;default:0" json:"sort_order"`
	LastSyncAt      *time.Time `json:"last_sync_at"`
	LastSyncStatus  string     `gorm:"type:varchar(40);not null;default:''" json:"last_sync_status"`
	LastSyncMessage string     `gorm:"type:text" json:"last_sync_message"`
	MetaJSON        JSON       `gorm:"type:json" json:"meta"`
	CreatedAt       time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"index" json:"updated_at"`
}

func (PluginMarketRegistry) TableName() string { return "plugin_market_registries" }

// PluginMarketCache 在线插件市场缓存。
type PluginMarketCache struct {
	ID             uint        `gorm:"primaryKey" json:"id"`
	RegistryID     string      `gorm:"type:varchar(60);index:idx_plugin_market_registry_plugin_ver,priority:1;not null" json:"registry_id"`
	PluginID       string      `gorm:"type:varchar(120);index:idx_plugin_market_registry_plugin_ver,priority:2;not null" json:"plugin_id"`
	Version        string      `gorm:"type:varchar(60);index:idx_plugin_market_registry_plugin_ver,priority:3;not null" json:"version"`
	Name           string      `gorm:"type:varchar(200);not null" json:"name"`
	Author         string      `gorm:"type:varchar(120);not null;default:''" json:"author"`
	Type           string      `gorm:"type:varchar(40);index;not null" json:"type"`
	Summary        string      `gorm:"type:text" json:"summary"`
	Description    string      `gorm:"type:text" json:"description"`
	Icon           string      `gorm:"type:varchar(500);not null;default:''" json:"icon"`
	Cover          string      `gorm:"type:varchar(500);not null;default:''" json:"cover"`
	HostAPIVersion string      `gorm:"type:varchar(60);not null;default:''" json:"host_api_version"`
	GoVersion      string      `gorm:"type:varchar(40);not null;default:''" json:"go_version"`
	BuildTarget    string      `gorm:"type:varchar(60);not null;default:''" json:"build_target"`
	Permissions    StringArray `gorm:"type:json" json:"permissions"`
	DownloadURL    string      `gorm:"type:varchar(500);not null;default:''" json:"download_url"`
	Checksum       string      `gorm:"type:varchar(128);not null;default:''" json:"checksum"`
	ReviewStatus   string      `gorm:"type:varchar(40);index;not null;default:'approved'" json:"review_status"`
	Changelog      string      `gorm:"type:text" json:"changelog"`
	ConfigSchema   JSON        `gorm:"type:json" json:"config_schema"`
	MetaJSON       JSON        `gorm:"type:json" json:"meta"`
	SyncedAt       time.Time   `gorm:"index" json:"synced_at"`
	CreatedAt      time.Time   `gorm:"index" json:"created_at"`
	UpdatedAt      time.Time   `gorm:"index" json:"updated_at"`
}

func (PluginMarketCache) TableName() string { return "plugin_market_cache" }

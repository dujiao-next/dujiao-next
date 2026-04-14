package models

import "time"

// PluginMarketPublisher 在线插件中心发布者。
type PluginMarketPublisher struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	PublisherCode string    `gorm:"type:varchar(120);uniqueIndex;not null" json:"publisher_code"`
	Name          string    `gorm:"type:varchar(200);not null" json:"name"`
	ContactEmail  string    `gorm:"type:varchar(200);not null;default:''" json:"contact_email"`
	Status        string    `gorm:"type:varchar(40);index;not null;default:'active'" json:"status"`
	IsOfficial    bool      `gorm:"not null;default:false;index" json:"is_official"`
	MetaJSON      JSON      `gorm:"type:json" json:"meta"`
	CreatedAt     time.Time `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time `gorm:"index" json:"updated_at"`
}

func (PluginMarketPublisher) TableName() string { return "plugin_market_publishers" }

// PluginMarketCatalogPlugin 在线插件中心插件主记录。
type PluginMarketCatalogPlugin struct {
	ID          uint        `gorm:"primaryKey" json:"id"`
	PluginID    string      `gorm:"type:varchar(120);uniqueIndex;not null" json:"plugin_id"`
	Slug        string      `gorm:"type:varchar(160);uniqueIndex;not null" json:"slug"`
	PublisherID uint        `gorm:"index;not null;default:0" json:"publisher_id"`
	Name        string      `gorm:"type:varchar(200);not null" json:"name"`
	Summary     string      `gorm:"type:text" json:"summary"`
	Description string      `gorm:"type:text" json:"description"`
	PluginType  string      `gorm:"type:varchar(40);index;not null;default:'feature'" json:"plugin_type"`
	BillingMode string      `gorm:"type:varchar(40);index;not null;default:'free'" json:"billing_mode"`
	LicenseMode string      `gorm:"type:varchar(40);index;not null;default:'free'" json:"license_mode"`
	Status      string      `gorm:"type:varchar(40);index;not null;default:'draft'" json:"status"`
	IsOfficial  bool        `gorm:"not null;default:false;index" json:"is_official"`
	IsPublic    bool        `gorm:"not null;index" json:"is_public"`
	IconURL     string      `gorm:"type:varchar(500);not null;default:''" json:"icon_url"`
	CoverURL    string      `gorm:"type:varchar(500);not null;default:''" json:"cover_url"`
	HomepageURL string      `gorm:"type:varchar(500);not null;default:''" json:"homepage_url"`
	SourceURL   string      `gorm:"type:varchar(500);not null;default:''" json:"source_url"`
	TagsJSON    StringArray `gorm:"type:json" json:"tags"`
	MetaJSON    JSON        `gorm:"type:json" json:"meta"`
	CreatedAt   time.Time   `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time   `gorm:"index" json:"updated_at"`
}

func (PluginMarketCatalogPlugin) TableName() string { return "plugin_market_plugins" }

// PluginMarketVersion 在线插件中心版本记录。
type PluginMarketVersion struct {
	ID                 uint        `gorm:"primaryKey" json:"id"`
	PluginID           string      `gorm:"type:varchar(120);uniqueIndex:idx_plugin_market_versions_pid_ver,priority:1;not null" json:"plugin_id"`
	Version            string      `gorm:"type:varchar(60);uniqueIndex:idx_plugin_market_versions_pid_ver,priority:2;not null" json:"version"`
	ReleaseChannel     string      `gorm:"type:varchar(40);index;not null;default:'stable'" json:"release_channel"`
	PackageStorageKey  string      `gorm:"type:varchar(500);not null;default:''" json:"package_storage_key"`
	PackageDownloadURL string      `gorm:"type:varchar(500);not null;default:''" json:"package_download_url"`
	ChecksumSHA256     string      `gorm:"type:varchar(128);not null;default:''" json:"checksum_sha256"`
	PackageSizeBytes   int64       `gorm:"not null;default:0" json:"package_size_bytes"`
	HostAPIVersion     string      `gorm:"type:varchar(60);not null;default:''" json:"host_api_version"`
	BuildTarget        string      `gorm:"type:varchar(60);not null;default:''" json:"build_target"`
	GoVersion          string      `gorm:"type:varchar(40);not null;default:''" json:"go_version"`
	PermissionsJSON    StringArray `gorm:"type:json" json:"permissions"`
	ConfigSchemaJSON   JSON        `gorm:"type:json" json:"config_schema"`
	ChangelogMD        string      `gorm:"type:text" json:"changelog_md"`
	ReviewStatus       string      `gorm:"type:varchar(40);index;not null;default:'draft'" json:"review_status"`
	PublishedAt        *time.Time  `json:"published_at"`
	MetaJSON           JSON        `gorm:"type:json" json:"meta"`
	CreatedAt          time.Time   `gorm:"index" json:"created_at"`
	UpdatedAt          time.Time   `gorm:"index" json:"updated_at"`
}

func (PluginMarketVersion) TableName() string { return "plugin_market_versions" }

// PluginMarketPlan 在线插件中心套餐方案。
type PluginMarketPlan struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	PluginID         string    `gorm:"type:varchar(120);uniqueIndex:idx_plugin_market_plans_pid_code,priority:1;not null" json:"plugin_id"`
	PlanCode         string    `gorm:"type:varchar(120);uniqueIndex:idx_plugin_market_plans_pid_code,priority:2;not null" json:"plan_code"`
	PlanName         string    `gorm:"type:varchar(160);not null" json:"plan_name"`
	BillingMode      string    `gorm:"type:varchar(40);index;not null;default:'free'" json:"billing_mode"`
	LicenseMode      string    `gorm:"type:varchar(40);index;not null;default:'free'" json:"license_mode"`
	PriceAmount      Money     `gorm:"type:decimal(20,2);not null;default:0" json:"price_amount"`
	PriceCurrency    string    `gorm:"type:varchar(20);not null;default:'CNY'" json:"price_currency"`
	DurationDays     *int      `json:"duration_days"`
	MaxSites         int       `gorm:"not null;default:1" json:"max_sites"`
	MaxActivations   int       `gorm:"not null;default:1" json:"max_activations"`
	FeatureFlagsJSON JSON      `gorm:"type:json" json:"feature_flags"`
	Status           string    `gorm:"type:varchar(40);index;not null;default:'active'" json:"status"`
	SortOrder        int       `gorm:"not null;default:0" json:"sort_order"`
	MetaJSON         JSON      `gorm:"type:json" json:"meta"`
	CreatedAt        time.Time `gorm:"index" json:"created_at"`
	UpdatedAt        time.Time `gorm:"index" json:"updated_at"`
}

func (PluginMarketPlan) TableName() string { return "plugin_market_plans" }

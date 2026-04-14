package models

import "time"

// PluginLicense 插件授权主表。
type PluginLicense struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	LicenseID            string     `gorm:"type:varchar(120);uniqueIndex;not null" json:"license_id"`
	PluginID             string     `gorm:"type:varchar(120);index;not null" json:"plugin_id"`
	PlanID               uint       `gorm:"index;not null;default:0" json:"plan_id"`
	CustomerID           uint       `gorm:"index;not null;default:0" json:"customer_id"`
	OrderID              uint       `gorm:"index;not null;default:0" json:"order_id"`
	LicenseKey           string     `gorm:"type:varchar(160);uniqueIndex;not null" json:"license_key"`
	LicenseMode          string     `gorm:"type:varchar(40);index;not null;default:'free'" json:"license_mode"`
	Status               string     `gorm:"type:varchar(40);index;not null;default:'pending'" json:"status"`
	BoundDomain          string     `gorm:"type:varchar(255);index;not null;default:''" json:"bound_domain"`
	BoundServerIP        string     `gorm:"type:varchar(100);index;not null;default:''" json:"bound_server_ip"`
	ExpireAt             *time.Time `json:"expire_at"`
	GraceDeadlineAt      *time.Time `json:"grace_deadline_at"`
	FeatureFlagsJSON     JSON       `gorm:"type:json" json:"feature_flags"`
	ActivationSecretHash string     `gorm:"type:varchar(255);not null;default:''" json:"activation_secret_hash"`
	IssuedAt             time.Time  `gorm:"index;not null" json:"issued_at"`
	ActivatedAt          *time.Time `json:"activated_at"`
	LastValidatedAt      *time.Time `json:"last_validated_at"`
	MetaJSON             JSON       `gorm:"type:json" json:"meta"`
	CreatedAt            time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt            time.Time  `gorm:"index" json:"updated_at"`
}

func (PluginLicense) TableName() string { return "plugin_license_licenses" }

// PluginLicenseActivation 插件授权激活实例。
type PluginLicenseActivation struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	LicenseID         string     `gorm:"type:varchar(120);index:idx_plugin_license_activation_license_status,priority:1;not null" json:"license_id"`
	SiteID            string     `gorm:"type:varchar(120);not null;default:''" json:"site_id"`
	InstallID         string     `gorm:"type:varchar(160);index;not null" json:"install_id"`
	HostFingerprint   string     `gorm:"type:varchar(255);not null;default:''" json:"host_fingerprint"`
	ReportedDomain    string     `gorm:"type:varchar(255);not null;default:''" json:"reported_domain"`
	ReportedIP        string     `gorm:"type:varchar(100);not null;default:''" json:"reported_ip"`
	ValidatedDomain   string     `gorm:"type:varchar(255);not null;default:''" json:"validated_domain"`
	ValidatedServerIP string     `gorm:"type:varchar(100);not null;default:''" json:"validated_server_ip"`
	ActivationToken   string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"activation_token"`
	Status            string     `gorm:"type:varchar(40);index:idx_plugin_license_activation_license_status,priority:2;not null;default:'active'" json:"status"`
	ActivatedAt       time.Time  `gorm:"index;not null" json:"activated_at"`
	LastHeartbeatAt   *time.Time `json:"last_heartbeat_at"`
	LastSeenAt        *time.Time `json:"last_seen_at"`
	CreatedAt         time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"index" json:"updated_at"`
}

func (PluginLicenseActivation) TableName() string { return "plugin_license_activations" }

// PluginLicenseHeartbeat 插件授权心跳明细。
type PluginLicenseHeartbeat struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	LicenseID         string    `gorm:"type:varchar(120);index:idx_plugin_license_heartbeat_license_time,priority:1;not null" json:"license_id"`
	ActivationID      uint      `gorm:"index;not null;default:0" json:"activation_id"`
	ReportedDomain    string    `gorm:"type:varchar(255);not null;default:''" json:"reported_domain"`
	ReportedIP        string    `gorm:"type:varchar(100);not null;default:''" json:"reported_ip"`
	ReportedVersion   string    `gorm:"type:varchar(80);not null;default:''" json:"reported_version"`
	RuntimeLoaded     bool      `gorm:"not null;default:false" json:"runtime_loaded"`
	MatchedDomain     bool      `gorm:"not null;default:false" json:"matched_domain"`
	MatchedServerIP   bool      `gorm:"not null;default:false" json:"matched_server_ip"`
	LicenseStatus     string    `gorm:"type:varchar(40);index;not null;default:'pending'" json:"license_status"`
	EnforcementAction string    `gorm:"type:varchar(40);index;not null;default:'warn'" json:"enforcement_action"`
	Message           string    `gorm:"type:text" json:"message"`
	CreatedAt         time.Time `gorm:"index:idx_plugin_license_heartbeat_license_time,priority:2" json:"created_at"`
}

func (PluginLicenseHeartbeat) TableName() string { return "plugin_license_heartbeats" }

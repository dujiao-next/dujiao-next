package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

const (
	ErrorScopeUnsupported = "scope_unsupported"
	ScanInterval          = 3 * time.Hour
)

type Scan struct {
	ConnectionID    uint       `gorm:"primaryKey;autoIncrement:false" json:"connection_id"`
	Mode            string     `gorm:"size:20" json:"mode"`
	AttemptAt       *time.Time `json:"attempt_at"`
	LastSuccess     *time.Time `json:"last_success_at"`
	LastFull        *time.Time `json:"last_full_at"`
	Cursor          *time.Time `json:"-"`
	NextScanAt      *time.Time `json:"next_scan_at"`
	LeaseUntil      *time.Time `json:"-"`
	Failures        int        `gorm:"not null;default:0" json:"-"`
	ErrorCode       string     `gorm:"size:32" json:"error_code"`
	CatalogIndexed  bool       `gorm:"not null;default:false" json:"-"`
	CategoryRetryAt *time.Time `json:"-"`
	CategoryIDs     []uint     `gorm:"serializer:json;type:text" json:"-"`
	ScopeRevision   uint64     `gorm:"not null;default:0" json:"-"`
	DirectoryRead   bool       `gorm:"not null;default:false" json:"-"`
}

func (Scan) TableName() string { return "upstream_catalog_scans" }

type ScopeCategory struct {
	ConnectionID uint `gorm:"primaryKey;autoIncrement:false"`
	CategoryID   uint `gorm:"primaryKey;autoIncrement:false"`
	Initialized  bool `gorm:"not null;default:false"`
}

func (ScopeCategory) TableName() string { return "upstream_catalog_scope_categories" }

type Product struct {
	ConnectionID      uint           `gorm:"primaryKey;autoIncrement:false;index:catalog_supplier_category,priority:1" json:"connection_id"`
	UpstreamProductID uint           `gorm:"primaryKey;autoIncrement:false" json:"upstream_product_id"`
	CategoryID        *uint          `gorm:"index:catalog_supplier_category,priority:4" json:"category_id"`
	Title             jsonmap.JSON   `gorm:"type:json" json:"title"`
	Price             string         `gorm:"size:80" json:"price"`
	Currency          string         `gorm:"size:16" json:"currency"`
	Stock             *int64         `json:"stock"`
	Active            bool           `gorm:"not null" json:"active"`
	Present           bool           `gorm:"not null;index:catalog_supplier_category,priority:3" json:"present"`
	IsNew             bool           `gorm:"not null;index:catalog_product_inbox,priority:1;index:catalog_supplier_category,priority:2" json:"is_new"`
	Relisted          bool           `gorm:"not null;default:false" json:"relisted"`
	Restocked         bool           `gorm:"not null;default:false" json:"restocked"`
	FirstSeenAt       time.Time      `gorm:"index:catalog_product_inbox,priority:2,sort:desc;index:catalog_supplier_category,priority:5,sort:desc" json:"first_seen_at"`
	CategoryPath      []jsonmap.JSON `gorm:"-" json:"category_path,omitempty"`
}

func (Product) TableName() string { return "upstream_catalog_products" }

// UnavailableProduct keeps the ID and reason for removal without caching product details.
type UnavailableProduct struct {
	ConnectionID      uint `gorm:"primaryKey;autoIncrement:false"`
	UpstreamProductID uint `gorm:"primaryKey;autoIncrement:false"`
	Delisted          bool `gorm:"not null;default:false"`
	OutOfStock        bool `gorm:"not null;default:false"`
}

func (UnavailableProduct) TableName() string { return "upstream_catalog_unavailable_products" }

func (p Product) OutOfStock() bool { return p.Stock != nil && *p.Stock == 0 }

type Category struct {
	ConnectionID uint         `gorm:"primaryKey;autoIncrement:false" json:"-"`
	ID           uint         `gorm:"primaryKey;autoIncrement:false" json:"id"`
	ParentID     uint         `json:"parent_id"`
	Name         jsonmap.JSON `gorm:"type:json" json:"name"`
	SortOrder    int          `json:"sort_order"`
}

func (Category) TableName() string { return "upstream_catalog_categories" }

type CategoryGroup struct {
	ID        uint           `json:"id"`
	Total     int64          `json:"total"`
	Path      []jsonmap.JSON `gorm:"-" json:"path"`
	SortOrder int            `gorm:"-" json:"sort_order"`
}

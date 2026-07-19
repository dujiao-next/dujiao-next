package models

import (
	"time"

	"gorm.io/gorm"
)

// 上游商品状态枚举（ProductMapping.UpstreamStatus）
const (
	UpstreamStatusActive   = "active"   // 正常在售
	UpstreamStatusInactive = "inactive" // 上游已下架，但商品仍存在
	UpstreamStatusDeleted  = "deleted"  // 上游已删除（软删），不再存在
)

// ProductMapping 商品映射表
type ProductMapping struct {
	ID                      uint           `gorm:"primarykey" json:"id"`
	ConnectionID            uint           `gorm:"index;not null" json:"connection_id"`
	LocalProductID          uint           `gorm:"index;not null" json:"local_product_id"`
	UpstreamProductID       uint           `gorm:"not null" json:"upstream_product_id"`
	UpstreamFulfillmentType string         `gorm:"type:varchar(20);not null;default:'manual'" json:"upstream_fulfillment_type"` // 上游原始交付类型（auto/manual）
	UpstreamStatus          string         `gorm:"type:varchar(16);not null;default:'active';index" json:"upstream_status"`     // 上游商品状态：active/inactive/deleted
	IsActive                bool           `gorm:"not null;default:true" json:"is_active"`
	LastSyncedAt            *time.Time     `json:"last_synced_at,omitempty"`
	CreatedAt               time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt               time.Time      `gorm:"index" json:"updated_at"`
	DeletedAt               gorm.DeletedAt `gorm:"index" json:"-"`

	Connection *SiteConnection `gorm:"foreignKey:ConnectionID" json:"connection,omitempty"`
	Product    *Product        `gorm:"foreignKey:LocalProductID" json:"product,omitempty"`
}

// TableName 指定表名
func (ProductMapping) TableName() string {
	return "product_mappings"
}

// SKUMapping SKU 映射表
type SKUMapping struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	ProductMappingID uint           `gorm:"index;not null" json:"product_mapping_id"`
	LocalSKUID       uint           `gorm:"column:local_sku_id;index;not null" json:"local_sku_id"`
	UpstreamSKUID    uint           `gorm:"column:upstream_sku_id;not null" json:"upstream_sku_id"`
	UpstreamPrice    Money          `gorm:"type:decimal(20,2);not null;default:0" json:"upstream_price"`
	UpstreamStock    int            `gorm:"not null;default:0" json:"upstream_stock"`
	UpstreamIsActive bool           `gorm:"not null;default:true" json:"upstream_is_active"`
	StockSyncedAt    *time.Time     `json:"stock_synced_at,omitempty"`
	CreatedAt        time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"index" json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (SKUMapping) TableName() string {
	return "sku_mappings"
}

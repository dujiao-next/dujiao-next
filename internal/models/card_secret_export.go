package models

import "time"

// CardSecretExport 卡密库存出库导出记录。
type CardSecretExport struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	ProductID         uint      `gorm:"not null;index" json:"product_id"`
	SKUID             uint      `gorm:"column:sku_id;not null;default:0;index" json:"sku_id"`
	BatchID           *uint     `gorm:"index" json:"batch_id,omitempty"`
	AdminID           uint      `gorm:"not null;default:0;index" json:"admin_id"`
	Format            string    `gorm:"type:varchar(8);not null" json:"format"`
	Count             int       `gorm:"not null" json:"count"`
	DeleteAfterExport bool      `gorm:"not null;default:false" json:"delete_after_export"`
	CreatedAt         time.Time `gorm:"index" json:"created_at"`
}

func (CardSecretExport) TableName() string {
	return "card_secret_exports"
}

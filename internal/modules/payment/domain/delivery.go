package domain

import "time"

// RecipientReceipt is per effect and destination, not proof that a human read it.
type RecipientReceipt struct {
	ID           uint   `gorm:"primaryKey"`
	EffectID     uint   `gorm:"not null;uniqueIndex:payment_recipient_identity"`
	RecipientKey string `gorm:"size:64;not null;uniqueIndex:payment_recipient_identity"`
	State        string `gorm:"size:16;not null"`
	Token        string `gorm:"size:36"`
	Attempts     int    `gorm:"not null;default:0"`
	DispatchedAt *time.Time
	CompletedAt  *time.Time
}

func (RecipientReceipt) TableName() string { return "payment_recipient_receipts" }

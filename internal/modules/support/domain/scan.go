package domain

import "time"

// ScanSession is scoped to the logged-in administrator. ProviderCode is never exposed.
type ScanSession struct {
	ID               string    `gorm:"primaryKey;size:64" json:"id"`
	AdminID          uint      `gorm:"index" json:"-"`
	ProviderCode     string    `json:"-"`
	URL              string    `json:"url"`
	Status           string    `gorm:"size:20" json:"status"`
	UID              string    `json:"uid,omitempty"`
	VerificationHash string    `json:"-"`
	ExpiresAt        time.Time `gorm:"index" json:"expires_at"`
	NextPollAt       time.Time `json:"-"`
	CreatedAt        time.Time `json:"-"`
}

func (ScanSession) TableName() string { return "support_notification_scans" }

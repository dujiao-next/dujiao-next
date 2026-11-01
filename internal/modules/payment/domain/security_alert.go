package domain

import "time"

// SecurityAlert is an aggregate operational condition, not raw gateway data.
type SecurityAlert struct {
	Kind       string `gorm:"primaryKey;size:40"`
	Active     bool   `gorm:"not null"`
	Count      int64  `gorm:"not null"`
	Version    uint64 `gorm:"not null"`
	Pending    bool   `gorm:"not null"`
	Attempts   int    `gorm:"not null"`
	Token      string `gorm:"size:36"`
	LeaseUntil *time.Time
	NextAt     time.Time `gorm:"index"`
	ObservedAt time.Time
	SentAt     *time.Time
}

func (SecurityAlert) TableName() string { return "payment_security_alerts" }

type SecurityCondition struct {
	Kind  string
	Count int64
}

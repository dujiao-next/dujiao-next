package adminapitokendomain

import "time"

// Token is a machine credential owned by a non-super administrator.
type Token struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	AdminID           uint       `gorm:"not null;index" json:"admin_id"`
	Name              string     `gorm:"size:80;not null" json:"name"`
	Digest            string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	AdminTokenVersion uint64     `gorm:"not null" json:"-"`
	IPWhitelist       []string   `gorm:"type:text;serializer:json" json:"ip_whitelist"`
	ExpiresAt         time.Time  `gorm:"not null;index" json:"expires_at"`
	LastUsedAt        *time.Time `json:"last_used_at"`
	RevokedAt         *time.Time `json:"revoked_at"`
	CreatedAt         time.Time  `json:"created_at"`
	Status            string     `gorm:"-" json:"status"`
}

func (Token) TableName() string { return "admin_api_tokens" }

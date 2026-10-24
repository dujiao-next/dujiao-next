package invitecodedomain

import (
	"strings"
	"time"
)

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// InviteCode 注册邀请码。MaxUses 为 0 表示不限次数。
type InviteCode struct {
	ID        uint       `gorm:"primarykey" json:"id"`                                           // 主键
	Code      string     `gorm:"type:varchar(32);uniqueIndex;not null" json:"code"`              // 邀请码（大写）
	MaxUses   int        `gorm:"not null;default:1" json:"max_uses"`                             // 可用次数，0 为不限
	UsedCount int        `gorm:"not null;default:0" json:"used_count"`                           // 已用次数
	ExpiresAt *time.Time `gorm:"index" json:"expires_at"`                                        // 过期时间
	Status    string     `gorm:"type:varchar(16);index;not null;default:'active'" json:"status"` // 状态
	Note      string     `gorm:"type:varchar(255);default:''" json:"note"`                       // 备注（分发渠道等）
	CreatedBy *uint      `gorm:"index" json:"created_by,omitempty"`                              // 创建管理员
	CreatedAt time.Time  `gorm:"index" json:"created_at"`                                        // 创建时间
	UpdatedAt time.Time  `json:"updated_at"`                                                     // 更新时间
}

// TableName 指定表名
func (InviteCode) TableName() string {
	return "invite_codes"
}

// NormalizeCode 统一邀请码大小写与空白，用户输入不区分大小写。
func NormalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// Usable 判断邀请码当前是否可用于注册。
func (c *InviteCode) Usable(now time.Time) bool {
	return c != nil &&
		c.Status == StatusActive &&
		(c.MaxUses == 0 || c.UsedCount < c.MaxUses) &&
		(c.ExpiresAt == nil || c.ExpiresAt.After(now))
}

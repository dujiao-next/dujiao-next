package models

import (
	"time"

	"gorm.io/gorm"
)

// User 用户表
type User struct {
	ID                    uint           `gorm:"primarykey" json:"id"`                                         // 主键
	Email                 string         `gorm:"uniqueIndex;not null" json:"email"`                            // 邮箱
	PasswordHash          string         `gorm:"not null" json:"-"`                                            // 密码哈希（不返回给前端）
	PasswordSetupRequired bool           `gorm:"not null;default:false" json:"-"`                              // 是否需要首次设置密码（Telegram 自动建号场景）
	DisplayName           string         `gorm:"default:''" json:"display_name"`                               // 昵称
	Locale                string         `gorm:"default:'zh-CN'" json:"locale"`                                // 语言偏好
	Status                string         `gorm:"default:'active'" json:"status"`                               // 账号状态
	MemberLevelID         uint           `gorm:"index;not null;default:0" json:"member_level_id"`              // 当前会员等级ID
	TotalRecharged        Money          `gorm:"type:decimal(20,2);not null;default:0" json:"total_recharged"` // 充值累计
	TotalSpent            Money          `gorm:"type:decimal(20,2);not null;default:0" json:"total_spent"`     // 消费累计
	AdminNote             string         `gorm:"type:text;default:''" json:"admin_note,omitempty"`             // 管理员备注（仅后台可见）
	TokenVersion          uint64         `gorm:"not null;default:0" json:"-"`                                  // Token 版本（用于全量失效）
	TokenInvalidBefore    *time.Time     `gorm:"index" json:"-"`                                               // 该时间点前签发的 Token 失效
	TOTPSecret            string         `gorm:"type:varchar(512);default:''" json:"-"`                        // AES-GCM 加密后的 hex 密文，未启用为空
	TOTPEnabledAt         *time.Time     `gorm:"index" json:"totp_enabled_at,omitempty"`                       // 启用时间，NULL 表示未启用
	TOTPPendingSecret     string         `gorm:"type:varchar(512);default:''" json:"-"`                        // 绑定流程中尚未首次验证的 secret（加密）
	TOTPPendingExpiresAt  *time.Time     `json:"-"`                                                            // 待绑定 secret 过期时间（10 分钟）
	RecoveryCodes         string         `gorm:"type:text;default:''" json:"-"`                                // JSON 数组：[{"hash":"...","used_at":null|"..."}]
	EmailVerifiedAt       *time.Time     `json:"email_verified_at"`                                            // 邮箱验证时间
	LastLoginAt           *time.Time     `json:"last_login_at"`                                                // 最后登录时间
	CreatedAt             time.Time      `gorm:"index" json:"created_at"`                                      // 创建时间
	UpdatedAt             time.Time      `gorm:"index" json:"updated_at"`                                      // 更新时间
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"`                                               // 软删除时间
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

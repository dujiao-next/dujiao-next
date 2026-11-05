package userdomain

import (
	"strings"
	"time"
	"unicode"

	"github.com/dujiao-next/internal/shared/money"
)

// User 用户表
type User struct {
	ID                    uint         `gorm:"primarykey" json:"id"`                                         // 主键
	Email                 string       `gorm:"uniqueIndex;not null" json:"email"`                            // 邮箱
	PasswordHash          string       `gorm:"not null" json:"-"`                                            // 密码哈希（不返回给前端）
	PasswordSetupRequired bool         `gorm:"not null;default:false" json:"-"`                              // 是否需要首次设置密码（Telegram 自动建号场景）
	DisplayName           string       `gorm:"default:''" json:"display_name"`                               // 昵称
	Locale                string       `gorm:"default:'zh-CN'" json:"locale"`                                // 语言偏好
	Status                string       `gorm:"default:'active'" json:"status"`                               // 账号状态
	MemberLevelID         uint         `gorm:"not null;default:0" json:"member_level_id"`                    // 当前会员等级ID
	TotalRecharged        money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"total_recharged"` // 充值累计
	TotalSpent            money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"total_spent"`     // 消费累计
	AdminNote             string       `gorm:"type:text;default:''" json:"admin_note,omitempty"`             // 管理员备注（仅后台可见）
	TokenVersion          uint64       `gorm:"not null;default:0" json:"-"`                                  // Token 版本（用于全量失效）
	TokenInvalidBefore    *time.Time   `gorm:"index" json:"-"`                                               // 该时间点前签发的 Token 失效
	TOTPSecret            string       `gorm:"type:varchar(512);default:''" json:"-"`                        // AES-GCM 加密后的 hex 密文，未启用为空
	TOTPEnabledAt         *time.Time   `gorm:"index" json:"totp_enabled_at,omitempty"`                       // 启用时间，NULL 表示未启用
	TOTPPendingSecret     string       `gorm:"type:varchar(512);default:''" json:"-"`                        // 绑定流程中尚未首次验证的 secret（加密）
	TOTPPendingExpiresAt  *time.Time   `json:"-"`                                                            // 待绑定 secret 过期时间（10 分钟）
	RecoveryCodes         string       `gorm:"type:text;default:''" json:"-"`                                // jsonmap.JSON 数组：[{"hash":"...","used_at":null|"..."}]
	EmailVerifiedAt       *time.Time   `json:"email_verified_at"`                                            // 邮箱验证时间
	LastLoginAt           *time.Time   `json:"last_login_at"`                                                // 最后登录时间
	InviteCodeID          *uint        `gorm:"index" json:"invite_code_id,omitempty"`                        // 注册时核销的邀请码
	CreatedAt             time.Time    `gorm:"index" json:"created_at"`                                      // 创建时间
	UpdatedAt             time.Time    `gorm:"index" json:"updated_at"`                                      // 更新时间
	DeletedAt             *time.Time   `gorm:"index" json:"-"`                                               // 软删除时间
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// DisplayNameMaxRunes 昵称最大字符数。
const DisplayNameMaxRunes = 32

func isDisallowedDisplayNameRune(r rune) bool {
	return unicode.IsControl(r) || strings.ContainsRune("<>\"'`", r)
}

// SanitizeDisplayName 剔除 HTML 敏感字符与控制字符并截断，用于第三方/系统生成的昵称。
func SanitizeDisplayName(raw string) string {
	cleaned := strings.TrimSpace(strings.Map(func(r rune) rune {
		if isDisallowedDisplayNameRune(r) {
			return -1
		}
		return r
	}, raw))
	if runes := []rune(cleaned); len(runes) > DisplayNameMaxRunes {
		cleaned = strings.TrimSpace(string(runes[:DisplayNameMaxRunes]))
	}
	return cleaned
}

// ValidDisplayName 校验手动填写的昵称（调用方先 TrimSpace）：非空、不超长、不含 HTML 敏感字符与控制字符。
func ValidDisplayName(name string) bool {
	return name != "" && SanitizeDisplayName(name) == name
}

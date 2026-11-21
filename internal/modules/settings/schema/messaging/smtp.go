package settingsmessaging

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

var ErrSMTPConfigInvalid = errors.New("smtp config invalid")

// SMTPVerifyCodeSetting SMTP 验证码相关配置。
type SMTPVerifyCodeSetting struct {
	ExpireMinutes       int `json:"expire_minutes"`
	SendIntervalSeconds int `json:"send_interval_seconds"`
	MaxAttempts         int `json:"max_attempts"`
	Length              int `json:"length"`
}

// SMTPOrderNotificationStatuses 按订单状态控制是否发送订单邮件（总开关开启时才生效）。
// Refunded 同时覆盖 refunded 与 partially_refunded。
type SMTPOrderNotificationStatuses struct {
	Paid      bool `json:"paid"`
	Delivered bool `json:"delivered"`
	Completed bool `json:"completed"`
	Refunded  bool `json:"refunded"`
}

// SMTPOrderNotificationStatusesPatch 按状态开关补丁。
type SMTPOrderNotificationStatusesPatch struct {
	Paid      *bool `json:"paid"`
	Delivered *bool `json:"delivered"`
	Completed *bool `json:"completed"`
	Refunded  *bool `json:"refunded"`
}

func defaultOrderNotificationStatuses() SMTPOrderNotificationStatuses {
	return SMTPOrderNotificationStatuses{Paid: true, Delivered: true, Completed: true, Refunded: true}
}

func (s SMTPOrderNotificationStatuses) toMap() map[string]interface{} {
	return map[string]interface{}{
		"paid":      s.Paid,
		"delivered": s.Delivered,
		"completed": s.Completed,
		"refunded":  s.Refunded,
	}
}

// SMTPSetting SMTP 配置实体。
type SMTPSetting struct {
	Enabled                   bool                          `json:"enabled"`
	Host                      string                        `json:"host"`
	Port                      int                           `json:"port"`
	Username                  string                        `json:"username"`
	Password                  string                        `json:"password"`
	From                      string                        `json:"from"`
	FromName                  string                        `json:"from_name"`
	UseTLS                    bool                          `json:"use_tls"`
	UseSSL                    bool                          `json:"use_ssl"`
	OrderNotificationEnabled  bool                          `json:"order_notification_enabled"`
	OrderNotificationStatuses SMTPOrderNotificationStatuses `json:"order_notification_statuses"`
	VerifyCode                SMTPVerifyCodeSetting         `json:"verify_code"`
}

// IsOrderStatusEmailEnabled 判断某订单状态的邮件是否应发送。未列出的状态沿用总开关。
func (s SMTPSetting) IsOrderStatusEmailEnabled(status string) bool {
	if !s.Enabled || !s.OrderNotificationEnabled {
		return false
	}
	switch strings.TrimSpace(status) {
	case constants.OrderStatusPaid:
		return s.OrderNotificationStatuses.Paid
	case constants.OrderStatusDelivered:
		return s.OrderNotificationStatuses.Delivered
	case constants.OrderStatusCompleted:
		return s.OrderNotificationStatuses.Completed
	case constants.OrderStatusRefunded, constants.OrderStatusPartiallyRefunded:
		return s.OrderNotificationStatuses.Refunded
	}
	return true
}

// SMTPVerifyCodePatch SMTP 验证码配置补丁。
type SMTPVerifyCodePatch struct {
	ExpireMinutes       *int `json:"expire_minutes"`
	SendIntervalSeconds *int `json:"send_interval_seconds"`
	MaxAttempts         *int `json:"max_attempts"`
	Length              *int `json:"length"`
}

// SMTPSettingPatch SMTP 配置补丁（支持部分更新）。
type SMTPSettingPatch struct {
	Enabled                   *bool                               `json:"enabled"`
	Host                      *string                             `json:"host"`
	Port                      *int                                `json:"port"`
	Username                  *string                             `json:"username"`
	Password                  *string                             `json:"password"`
	From                      *string                             `json:"from"`
	FromName                  *string                             `json:"from_name"`
	UseTLS                    *bool                               `json:"use_tls"`
	UseSSL                    *bool                               `json:"use_ssl"`
	OrderNotificationEnabled  *bool                               `json:"order_notification_enabled"`
	OrderNotificationStatuses *SMTPOrderNotificationStatusesPatch `json:"order_notification_statuses"`
	VerifyCode                *SMTPVerifyCodePatch                `json:"verify_code"`
}

// DefaultSMTPSetting 根据静态配置生成默认 SMTP 设置。
func DefaultSMTPSetting(cfg config.EmailConfig) SMTPSetting {
	setting := SMTPSetting{
		Enabled:                   cfg.Enabled,
		Host:                      strings.TrimSpace(cfg.Host),
		Port:                      cfg.Port,
		Username:                  strings.TrimSpace(cfg.Username),
		Password:                  strings.TrimSpace(cfg.Password),
		From:                      strings.TrimSpace(cfg.From),
		FromName:                  strings.TrimSpace(cfg.FromName),
		UseTLS:                    cfg.UseTLS,
		UseSSL:                    cfg.UseSSL,
		OrderNotificationEnabled:  true,
		OrderNotificationStatuses: defaultOrderNotificationStatuses(),
		VerifyCode: SMTPVerifyCodeSetting{
			ExpireMinutes:       cfg.VerifyCode.ExpireMinutes,
			SendIntervalSeconds: cfg.VerifyCode.SendIntervalSeconds,
			MaxAttempts:         cfg.VerifyCode.MaxAttempts,
			Length:              cfg.VerifyCode.Length,
		},
	}
	return NormalizeSMTPSetting(setting)
}

// NormalizeSMTPSetting 归一化 SMTP 配置并补齐默认值。
func NormalizeSMTPSetting(setting SMTPSetting) SMTPSetting {
	setting.Host = strings.TrimSpace(setting.Host)
	setting.Username = strings.TrimSpace(setting.Username)
	setting.Password = strings.TrimSpace(setting.Password)
	setting.From = strings.TrimSpace(setting.From)
	setting.FromName = strings.TrimSpace(setting.FromName)

	if setting.Port <= 0 || setting.Port > 65535 {
		setting.Port = 587
	}

	if setting.VerifyCode.ExpireMinutes <= 0 {
		setting.VerifyCode.ExpireMinutes = 10
	}
	if setting.VerifyCode.SendIntervalSeconds <= 0 {
		setting.VerifyCode.SendIntervalSeconds = 60
	}
	if setting.VerifyCode.MaxAttempts <= 0 {
		setting.VerifyCode.MaxAttempts = 5
	}
	if setting.VerifyCode.Length < 4 || setting.VerifyCode.Length > 10 {
		setting.VerifyCode.Length = 6
	}

	return setting
}

// ValidateSMTPSetting 校验 SMTP 配置合法性。
func ValidateSMTPSetting(setting SMTPSetting) error {
	if setting.Port <= 0 || setting.Port > 65535 {
		return fmt.Errorf("%w: SMTP 端口必须在 1-65535", ErrSMTPConfigInvalid)
	}
	if setting.UseTLS && setting.UseSSL {
		return fmt.Errorf("%w: TLS 与 SSL 不能同时开启", ErrSMTPConfigInvalid)
	}
	if setting.VerifyCode.Length < 4 || setting.VerifyCode.Length > 10 {
		return fmt.Errorf("%w: 验证码长度需在 4-10 之间", ErrSMTPConfigInvalid)
	}
	if setting.VerifyCode.ExpireMinutes <= 0 {
		return fmt.Errorf("%w: 验证码过期时间必须大于 0", ErrSMTPConfigInvalid)
	}
	if setting.VerifyCode.SendIntervalSeconds <= 0 {
		return fmt.Errorf("%w: 验证码发送间隔必须大于 0", ErrSMTPConfigInvalid)
	}
	if setting.VerifyCode.MaxAttempts <= 0 {
		return fmt.Errorf("%w: 验证码尝试次数必须大于 0", ErrSMTPConfigInvalid)
	}
	if !setting.Enabled {
		return nil
	}
	if strings.TrimSpace(setting.Host) == "" {
		return fmt.Errorf("%w: SMTP 主机不能为空", ErrSMTPConfigInvalid)
	}
	if strings.TrimSpace(setting.From) == "" {
		return fmt.Errorf("%w: 发件人邮箱不能为空", ErrSMTPConfigInvalid)
	}
	if _, err := mail.ParseAddress(setting.From); err != nil {
		return fmt.Errorf("%w: 发件人邮箱格式无效", ErrSMTPConfigInvalid)
	}
	return nil
}

// SMTPSettingToConfig 将 SMTP 设置转换为运行时配置。
func SMTPSettingToConfig(setting SMTPSetting) config.EmailConfig {
	normalized := NormalizeSMTPSetting(setting)
	return config.EmailConfig{
		Enabled:  normalized.Enabled,
		Host:     normalized.Host,
		Port:     normalized.Port,
		Username: normalized.Username,
		Password: normalized.Password,
		From:     normalized.From,
		FromName: normalized.FromName,
		UseTLS:   normalized.UseTLS,
		UseSSL:   normalized.UseSSL,
		VerifyCode: config.VerifyCodeConfig{
			ExpireMinutes:       normalized.VerifyCode.ExpireMinutes,
			SendIntervalSeconds: normalized.VerifyCode.SendIntervalSeconds,
			MaxAttempts:         normalized.VerifyCode.MaxAttempts,
			Length:              normalized.VerifyCode.Length,
		},
	}
}

// EncodeSMTPSetting 将 SMTP 设置编码为 settings 表结构。
func EncodeSMTPSetting(setting SMTPSetting) jsonmap.JSON {
	normalized := NormalizeSMTPSetting(setting)
	return jsonmap.JSON{
		"enabled":                     normalized.Enabled,
		"host":                        normalized.Host,
		"port":                        normalized.Port,
		"username":                    normalized.Username,
		"password":                    normalized.Password,
		"from":                        normalized.From,
		"from_name":                   normalized.FromName,
		"use_tls":                     normalized.UseTLS,
		"use_ssl":                     normalized.UseSSL,
		"order_notification_enabled":  normalized.OrderNotificationEnabled,
		"order_notification_statuses": normalized.OrderNotificationStatuses.toMap(),
		"verify_code": map[string]interface{}{
			"expire_minutes":        normalized.VerifyCode.ExpireMinutes,
			"send_interval_seconds": normalized.VerifyCode.SendIntervalSeconds,
			"max_attempts":          normalized.VerifyCode.MaxAttempts,
			"length":                normalized.VerifyCode.Length,
		},
	}
}

// MaskSMTPSettingForAdmin 返回脱敏后的 SMTP 设置。
func MaskSMTPSettingForAdmin(setting SMTPSetting) jsonmap.JSON {
	normalized := NormalizeSMTPSetting(setting)
	return jsonmap.JSON{
		"enabled":                     normalized.Enabled,
		"host":                        normalized.Host,
		"port":                        normalized.Port,
		"username":                    normalized.Username,
		"password":                    "",
		"has_password":                normalized.Password != "",
		"from":                        normalized.From,
		"from_name":                   normalized.FromName,
		"use_tls":                     normalized.UseTLS,
		"use_ssl":                     normalized.UseSSL,
		"order_notification_enabled":  normalized.OrderNotificationEnabled,
		"order_notification_statuses": normalized.OrderNotificationStatuses.toMap(),
		"verify_code": map[string]interface{}{
			"expire_minutes":        normalized.VerifyCode.ExpireMinutes,
			"send_interval_seconds": normalized.VerifyCode.SendIntervalSeconds,
			"max_attempts":          normalized.VerifyCode.MaxAttempts,
			"length":                normalized.VerifyCode.Length,
		},
	}
}

// DecodeSMTPSetting 从持久化 JSON 解码，并对缺失字段使用 fallback。
func DecodeSMTPSetting(raw jsonmap.JSON, fallback SMTPSetting) SMTPSetting {
	next := fallback
	if raw == nil {
		return next
	}

	if value, exists := raw["enabled"]; exists {
		next.Enabled = settingsvalue.ParseBool(value)
	}
	if value, exists := raw["host"]; exists {
		if text, ok := value.(string); ok {
			next.Host = text
		}
	}
	if value, exists := raw["port"]; exists {
		if parsed, err := settingsvalue.ParseInt(value); err == nil {
			next.Port = parsed
		}
	}
	if value, exists := raw["username"]; exists {
		if text, ok := value.(string); ok {
			next.Username = text
		}
	}
	if value, exists := raw["password"]; exists {
		if text, ok := value.(string); ok {
			next.Password = text
		}
	}
	if value, exists := raw["from"]; exists {
		if text, ok := value.(string); ok {
			next.From = text
		}
	}
	if value, exists := raw["from_name"]; exists {
		if text, ok := value.(string); ok {
			next.FromName = text
		}
	}
	if value, exists := raw["use_tls"]; exists {
		next.UseTLS = settingsvalue.ParseBool(value)
	}
	if value, exists := raw["use_ssl"]; exists {
		next.UseSSL = settingsvalue.ParseBool(value)
	}
	if value, exists := raw["order_notification_enabled"]; exists {
		next.OrderNotificationEnabled = settingsvalue.ParseBool(value)
	}
	if statusRaw, ok := raw["order_notification_statuses"]; ok {
		if statusMap := settingsvalue.ToStringAnyMap(statusRaw); statusMap != nil {
			if value, exists := statusMap["paid"]; exists {
				next.OrderNotificationStatuses.Paid = settingsvalue.ParseBool(value)
			}
			if value, exists := statusMap["delivered"]; exists {
				next.OrderNotificationStatuses.Delivered = settingsvalue.ParseBool(value)
			}
			if value, exists := statusMap["completed"]; exists {
				next.OrderNotificationStatuses.Completed = settingsvalue.ParseBool(value)
			}
			if value, exists := statusMap["refunded"]; exists {
				next.OrderNotificationStatuses.Refunded = settingsvalue.ParseBool(value)
			}
		}
	}

	if verifyRaw, ok := raw["verify_code"]; ok {
		if verifyMap := settingsvalue.ToStringAnyMap(verifyRaw); verifyMap != nil {
			if value, exists := verifyMap["expire_minutes"]; exists {
				if parsed, err := settingsvalue.ParseInt(value); err == nil {
					next.VerifyCode.ExpireMinutes = parsed
				}
			}
			if value, exists := verifyMap["send_interval_seconds"]; exists {
				if parsed, err := settingsvalue.ParseInt(value); err == nil {
					next.VerifyCode.SendIntervalSeconds = parsed
				}
			}
			if value, exists := verifyMap["max_attempts"]; exists {
				if parsed, err := settingsvalue.ParseInt(value); err == nil {
					next.VerifyCode.MaxAttempts = parsed
				}
			}
			if value, exists := verifyMap["length"]; exists {
				if parsed, err := settingsvalue.ParseInt(value); err == nil {
					next.VerifyCode.Length = parsed
				}
			}
		}
	}

	return next
}

// ApplySMTPSettingPatch 把补丁应用到当前 SMTP 配置并完成校验。
func ApplySMTPSettingPatch(current SMTPSetting, patch SMTPSettingPatch) (SMTPSetting, error) {
	next := current
	if patch.Enabled != nil {
		next.Enabled = *patch.Enabled
	}
	if patch.Host != nil {
		next.Host = strings.TrimSpace(*patch.Host)
	}
	if patch.Port != nil {
		next.Port = *patch.Port
	}
	if patch.Username != nil {
		next.Username = strings.TrimSpace(*patch.Username)
	}
	if patch.Password != nil {
		password := strings.TrimSpace(*patch.Password)
		if password != "" {
			next.Password = password
		}
	}
	if patch.From != nil {
		next.From = strings.TrimSpace(*patch.From)
	}
	if patch.FromName != nil {
		next.FromName = strings.TrimSpace(*patch.FromName)
	}
	if patch.UseTLS != nil {
		next.UseTLS = *patch.UseTLS
	}
	if patch.UseSSL != nil {
		next.UseSSL = *patch.UseSSL
	}
	if patch.OrderNotificationEnabled != nil {
		next.OrderNotificationEnabled = *patch.OrderNotificationEnabled
	}
	if patch.OrderNotificationStatuses != nil {
		st := patch.OrderNotificationStatuses
		if st.Paid != nil {
			next.OrderNotificationStatuses.Paid = *st.Paid
		}
		if st.Delivered != nil {
			next.OrderNotificationStatuses.Delivered = *st.Delivered
		}
		if st.Completed != nil {
			next.OrderNotificationStatuses.Completed = *st.Completed
		}
		if st.Refunded != nil {
			next.OrderNotificationStatuses.Refunded = *st.Refunded
		}
	}
	if patch.VerifyCode != nil {
		if patch.VerifyCode.ExpireMinutes != nil {
			next.VerifyCode.ExpireMinutes = *patch.VerifyCode.ExpireMinutes
		}
		if patch.VerifyCode.SendIntervalSeconds != nil {
			next.VerifyCode.SendIntervalSeconds = *patch.VerifyCode.SendIntervalSeconds
		}
		if patch.VerifyCode.MaxAttempts != nil {
			next.VerifyCode.MaxAttempts = *patch.VerifyCode.MaxAttempts
		}
		if patch.VerifyCode.Length != nil {
			next.VerifyCode.Length = *patch.VerifyCode.Length
		}
	}

	normalized := NormalizeSMTPSetting(next)
	if err := ValidateSMTPSetting(normalized); err != nil {
		return SMTPSetting{}, err
	}
	return normalized, nil
}

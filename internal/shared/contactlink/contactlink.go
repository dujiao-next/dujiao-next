// Package contactlink 统一主站与分销站客服联系方式链接的校验规则。
package contactlink

import "strings"

// IsTelegram 判断是否为 Telegram 联系链接。
func IsTelegram(value string) bool {
	return strings.HasPrefix(value, "https://telegram.me/") ||
		strings.HasPrefix(value, "https://t.me/") ||
		strings.HasPrefix(value, "tg://")
}

// IsWhatsApp 判断是否为 WhatsApp 联系链接。
func IsWhatsApp(value string) bool {
	return strings.HasPrefix(value, "https://wa.me/") ||
		strings.HasPrefix(value, "https://api.whatsapp.com/")
}

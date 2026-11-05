package settingsapp

import "strings"

// normalizeSettingText 统一清洗设置中的文本值。
func normalizeSettingText(raw interface{}) string {
	text, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

// normalizeSettingTextWithRuneLimit 清洗文本并限制最大字符数。
func normalizeSettingTextWithRuneLimit(raw interface{}, maxRuneCount int) string {
	text := normalizeSettingText(raw)
	if text == "" || maxRuneCount <= 0 {
		return text
	}

	runes := []rune(text)
	if len(runes) <= maxRuneCount {
		return text
	}
	return string(runes[:maxRuneCount])
}

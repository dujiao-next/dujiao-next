package service

import (
	"regexp"
	"strings"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
)

var settingSupportedLanguages = []string{"zh-CN", "zh-TW", "en-US"}
var settingCurrencyCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)

const (
	settingSiteScriptsMaxCount       = 20
	settingSiteScriptNameMaxRuneSize = 120
	settingSiteScriptCodeMaxRuneSize = 20000
)

// normalizeSettingValueByKey 按设置键执行归一化，避免非法值入库。
func normalizeSettingValueByKey(key string, value map[string]interface{}) models.JSON {
	switch key {
	case constants.SettingKeyDashboardConfig:
		setting := dashboardSettingFromJSON(models.JSON(value), DashboardDefaultSetting())
		return DashboardSettingToMap(setting)
	case constants.SettingKeyOrderConfig:
		return normalizeOrderSetting(value)
	case constants.SettingKeySiteConfig:
		return normalizeSiteSetting(value)
	case constants.SettingKeyTelegramAuthConfig:
		setting := telegramAuthSettingFromJSON(models.JSON(value), TelegramAuthDefaultSetting(config.TelegramAuthConfig{}))
		return TelegramAuthSettingToMap(setting)
	default:
		return models.JSON(value)
	}
}

// normalizeOrderSetting 归一化订单设置。
func normalizeOrderSetting(value map[string]interface{}) models.JSON {
	normalized := make(models.JSON, len(value)+1)
	for key, raw := range value {
		normalized[key] = raw
	}

	expireMinutes := 15
	if raw, ok := value[constants.SettingFieldPaymentExpireMinutes]; ok {
		if parsed, err := parseSettingInt(raw); err == nil {
			if parsed > 0 {
				expireMinutes = parsed
			}
		}
	}
	if expireMinutes > 10080 {
		expireMinutes = 10080
	}
	normalized[constants.SettingFieldPaymentExpireMinutes] = expireMinutes
	return normalized
}

// normalizeSiteSetting 归一化站点配置结构。
func normalizeSiteSetting(value map[string]interface{}) models.JSON {
	normalized := make(models.JSON, len(value)+8)
	for key, raw := range value {
		normalized[key] = raw
	}

	normalized["brand"] = normalizeSiteBrand(value["brand"])
	normalized["contact"] = normalizeSiteContact(value["contact"])
	normalized["seo"] = normalizeSiteLocalizedBlock(value["seo"], []string{"title", "keywords", "description"})
	normalized["legal"] = normalizeSiteLocalizedBlock(value["legal"], []string{"terms", "privacy"})
	normalized["about"] = normalizeSiteAbout(value["about"])
	normalized["user_registration"] = normalizeSiteUserRegistration(value["user_registration"])
	normalized["scripts"] = normalizeSiteScripts(value["scripts"])
	normalized[constants.SettingFieldSiteCurrency] = normalizeSiteCurrency(value[constants.SettingFieldSiteCurrency])

	if raw, ok := value["languages"]; ok {
		normalized["languages"] = normalizeSiteLanguages(raw)
	}

	return normalized
}

func normalizeSiteScripts(raw interface{}) []interface{} {
	listRaw, ok := raw.([]interface{})
	if !ok {
		return make([]interface{}, 0)
	}

	result := make([]interface{}, 0, len(listRaw))
	for _, itemRaw := range listRaw {
		itemMap, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}

		code := normalizeSettingTextWithRuneLimit(itemMap["code"], settingSiteScriptCodeMaxRuneSize)
		if code == "" {
			continue
		}

		position := normalizeSettingText(itemMap["position"])
		if position != "head" && position != "body_end" {
			position = "head"
		}

		result = append(result, map[string]interface{}{
			"name":     normalizeSettingTextWithRuneLimit(itemMap["name"], settingSiteScriptNameMaxRuneSize),
			"enabled":  parseSettingBool(itemMap["enabled"]),
			"position": position,
			"code":     code,
		})

		if len(result) >= settingSiteScriptsMaxCount {
			break
		}
	}

	return result
}

func normalizeSiteCurrency(raw interface{}) string {
	currency := strings.ToUpper(normalizeSettingText(raw))
	if !settingCurrencyCodePattern.MatchString(currency) {
		return constants.SiteCurrencyDefault
	}
	return currency
}

// normalizeSiteUserRegistration 归一化用户注册配置。
func normalizeSiteUserRegistration(raw interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"enabled":              true,
		"require_email_verify": true,
	}
	regMap, ok := raw.(map[string]interface{})
	if !ok {
		return result
	}
	if v, ok := regMap["enabled"].(bool); ok {
		result["enabled"] = v
	}
	if v, ok := regMap["require_email_verify"].(bool); ok {
		result["require_email_verify"] = v
	}
	return result
}


func normalizeSiteContact(raw interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"telegram": "",
		"whatsapp": "",
	}
	contactMap, ok := raw.(map[string]interface{})
	if !ok {
		return result
	}
	result["telegram"] = normalizeSettingText(contactMap["telegram"])
	result["whatsapp"] = normalizeSettingText(contactMap["whatsapp"])
	return result
}

func normalizeSiteBrand(raw interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"site_name": "",
	}
	brandMap, ok := raw.(map[string]interface{})
	if !ok {
		return result
	}
	result["site_name"] = normalizeSettingText(brandMap["site_name"])
	return result
}

func normalizeSiteLocalizedBlock(raw interface{}, fields []string) map[string]interface{} {
	result := make(map[string]interface{}, len(fields))
	blockMap, _ := raw.(map[string]interface{})

	for _, field := range fields {
		if blockMap == nil {
			result[field] = normalizeSiteLocalizedField(nil)
			continue
		}
		result[field] = normalizeSiteLocalizedField(blockMap[field])
	}

	return result
}

func normalizeSiteLocalizedField(raw interface{}) map[string]interface{} {
	fieldResult := make(map[string]interface{}, len(settingSupportedLanguages))
	for _, language := range settingSupportedLanguages {
		fieldResult[language] = ""
	}

	fieldRaw, ok := raw.(map[string]interface{})
	if !ok {
		return fieldResult
	}

	for _, language := range settingSupportedLanguages {
		fieldResult[language] = normalizeSettingText(fieldRaw[language])
	}

	return fieldResult
}

func normalizeSiteLocalizedList(raw interface{}, maxItems int) []interface{} {
	listRaw, ok := raw.([]interface{})
	if !ok {
		return make([]interface{}, 0)
	}

	result := make([]interface{}, 0, len(listRaw))
	for _, item := range listRaw {
		normalized := normalizeSiteLocalizedField(item)
		hasText := false
		for _, language := range settingSupportedLanguages {
			text, _ := normalized[language].(string)
			if text != "" {
				hasText = true
				break
			}
		}
		if !hasText {
			continue
		}

		result = append(result, normalized)
		if maxItems > 0 && len(result) >= maxItems {
			break
		}
	}

	return result
}

func normalizeSiteAbout(raw interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"hero":         normalizeSiteLocalizedBlock(nil, []string{"title", "subtitle"}),
		"introduction": normalizeSiteLocalizedField(nil),
		"services": map[string]interface{}{
			"title": normalizeSiteLocalizedField(nil),
			"items": make([]interface{}, 0),
		},
		"contact": map[string]interface{}{
			"title": normalizeSiteLocalizedField(nil),
			"text":  normalizeSiteLocalizedField(nil),
		},
	}

	aboutMap, ok := raw.(map[string]interface{})
	if !ok {
		return result
	}

	result["hero"] = normalizeSiteLocalizedBlock(aboutMap["hero"], []string{"title", "subtitle"})
	result["introduction"] = normalizeSiteLocalizedField(aboutMap["introduction"])

	services := map[string]interface{}{
		"title": normalizeSiteLocalizedField(nil),
		"items": make([]interface{}, 0),
	}
	if servicesRaw, ok := aboutMap["services"].(map[string]interface{}); ok {
		services["title"] = normalizeSiteLocalizedField(servicesRaw["title"])
		services["items"] = normalizeSiteLocalizedList(servicesRaw["items"], 12)
	}
	result["services"] = services

	contact := map[string]interface{}{
		"title": normalizeSiteLocalizedField(nil),
		"text":  normalizeSiteLocalizedField(nil),
	}
	if contactRaw, ok := aboutMap["contact"].(map[string]interface{}); ok {
		contact["title"] = normalizeSiteLocalizedField(contactRaw["title"])
		contact["text"] = normalizeSiteLocalizedField(contactRaw["text"])
	}
	result["contact"] = contact

	return result
}

func normalizeSiteLanguages(raw interface{}) []string {
	list := make([]string, 0)
	switch value := raw.(type) {
	case []string:
		list = append(list, value...)
	case []interface{}:
		for _, item := range value {
			list = append(list, normalizeSettingText(item))
		}
	default:
		return append([]string(nil), settingSupportedLanguages...)
	}

	result := make([]string, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, item := range list {
		lang := strings.TrimSpace(item)
		if lang == "" {
			continue
		}
		if _, exists := seen[lang]; exists {
			continue
		}
		seen[lang] = struct{}{}
		result = append(result, lang)
	}
	if len(result) == 0 {
		return append([]string(nil), settingSupportedLanguages...)
	}
	return result
}

func normalizeSettingText(raw interface{}) string {
	text, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

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

func parseSettingBool(raw interface{}) bool {
	switch value := raw.(type) {
	case bool:
		return value
	case int:
		return value != 0
	case int64:
		return value != 0
	case float64:
		return value != 0
	case string:
		normalized := strings.ToLower(strings.TrimSpace(value))
		return normalized == "1" || normalized == "true" || normalized == "yes" || normalized == "on"
	default:
		return false
	}
}

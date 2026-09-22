package settingsstorefront

import (
	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// NormalizeHomepageAdJSON keeps the independent inline homepage ad minimal.
func NormalizeHomepageAdJSON(value jsonmap.JSON) jsonmap.JSON {
	return jsonmap.JSON{
		"enabled": settingsvalue.ParseBool(value["enabled"]),
		"title":   normalizeAnnouncementLocalizedField(value["title"]),
		"content": normalizeAnnouncementLocalizedField(value["content"]),
	}
}

// ActiveHomepageAd omits disabled or empty ads from public config.
func ActiveHomepageAd(value jsonmap.JSON) (jsonmap.JSON, bool) {
	ad := NormalizeHomepageAdJSON(value)
	if !settingsvalue.ParseBool(ad["enabled"]) || !hasHomeAnnouncementContent(ad["content"].(map[string]interface{})) {
		return nil, false
	}
	return jsonmap.JSON{"title": ad["title"], "content": ad["content"]}, true
}

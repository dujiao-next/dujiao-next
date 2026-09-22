package settingsstorefront

import (
	"testing"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestHomepageAdNormalizesLocalizedFields(t *testing.T) {
	got := NormalizeHomepageAdJSON(jsonmap.JSON{"enabled": true, "title": map[string]interface{}{"zh-CN": " 标题 ", "fr": "ignore"}, "content": map[string]interface{}{"en-US": "<p>Welcome</p>"}})
	if got["enabled"] != true {
		t.Fatalf("enabled = %v", got["enabled"])
	}
	title := got["title"].(map[string]interface{})
	if title["zh-CN"] != "标题" || title["zh-TW"] != "" {
		t.Fatalf("title = %#v", title)
	}
	if _, ok := title["fr"]; ok {
		t.Fatalf("unsupported locale retained: %#v", title)
	}
	if got["content"].(map[string]interface{})["en-US"] != "<p>Welcome</p>" {
		t.Fatalf("content = %#v", got["content"])
	}
}

func TestHomepageAdOnlyPublishesEnabledNonemptyContent(t *testing.T) {
	for _, input := range []jsonmap.JSON{
		{"enabled": false, "content": map[string]interface{}{"zh-CN": "hello"}},
		{"enabled": true, "content": map[string]interface{}{"zh-CN": "   "}},
	} {
		if _, ok := ActiveHomepageAd(input); ok {
			t.Fatalf("published inactive ad: %#v", input)
		}
	}
	got, ok := ActiveHomepageAd(jsonmap.JSON{"enabled": true, "title": map[string]interface{}{"zh-CN": "广告"}, "content": map[string]interface{}{"zh-CN": "<p>正文</p>"}})
	if !ok || got["title"].(map[string]interface{})["zh-CN"] != "广告" {
		t.Fatalf("active ad = %#v, %t", got, ok)
	}
}

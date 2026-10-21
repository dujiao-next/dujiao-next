package application

import (
	"context"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/platform/translation"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/upstream"
)

// fakeSettingsProvider 以固定同步间隔实现 SettingsProvider。
type fakeSettingsProvider struct {
	interval time.Duration
}

func (f fakeSettingsProvider) GetUpstreamSyncConfig(fallbackInterval string) (settingsintegration.UpstreamSyncConfig, error) {
	cfg := settingsintegration.DefaultUpstreamSyncConfig()
	cfg.IntervalMinutes = int(f.interval / time.Minute)
	return cfg, nil
}

func (f fakeSettingsProvider) GetUpstreamSyncInterval(fallbackInterval string) (time.Duration, error) {
	return f.interval, nil
}

func TestComputeFullSyncIntervalFloorsAt24h(t *testing.T) {
	svc := &Service{settings: fakeSettingsProvider{interval: 5 * time.Minute}}

	// 5m 同步间隔 × 3 = 15m < 24h，期望落到 24h floor
	got := svc.computeFullSyncInterval()
	if got != fullSyncIntervalFloor {
		t.Fatalf("expected floor=24h, got %v", got)
	}
}

func TestComputeFullSyncIntervalScalesWithLongInterval(t *testing.T) {
	svc := &Service{settings: fakeSettingsProvider{interval: 12 * time.Hour}}

	// 12h * 3 = 36h，应使用 scaled 值
	got := svc.computeFullSyncInterval()
	want := 36 * time.Hour
	if got != want {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestComputeFullSyncIntervalWithoutSettings(t *testing.T) {
	svc := &Service{}
	got := svc.computeFullSyncInterval()
	if got != fullSyncIntervalFloor {
		t.Fatalf("expected floor when settings=nil, got %v", got)
	}
}

type fakeProductTranslator struct {
	calls int
}

func (f *fakeProductTranslator) TranslateProduct(_ context.Context, source translation.ProductText) (translation.ProductText, error) {
	f.calls++
	return translation.ProductText{
		Title:       "中文 " + source.Title,
		Description: "中文 " + source.Description,
		Content:     "中文 " + source.Content,
	}, nil
}

func TestTranslateProdSellerProductFillsChineseAndAvoidsRepeatedCalls(t *testing.T) {
	translator := &fakeProductTranslator{}
	svc := &Service{translator: translator}
	conn := &siteconnectiondomain.Connection{ID: 7, Protocol: constants.ConnectionProtocolProdSeller}
	product := &upstream.UpstreamProduct{
		ExternalID:  "product-1",
		Title:       jsonmap.JSON{"en-US": "English title", "zh-CN": "English title"},
		Description: jsonmap.JSON{"en-US": "English description", "zh-CN": "English description"},
		Content:     jsonmap.JSON{"en-US": "English content", "zh-CN": "English content"},
	}

	svc.translateProdSellerProduct(context.Background(), conn, nil, product)
	if translator.calls != 1 {
		t.Fatalf("translation calls = %d, want 1", translator.calls)
	}
	if got := product.Title["zh-CN"]; got != "中文 English title" {
		t.Fatalf("translated title = %#v", got)
	}

	local := &productdomain.Product{
		TitleJSON:       jsonmap.JSON{"en-US": "English title", "zh-CN": "中文 English title"},
		DescriptionJSON: jsonmap.JSON{"en-US": "English description", "zh-CN": "中文 English description"},
		ContentJSON:     jsonmap.JSON{"en-US": "English content", "zh-CN": "中文 English content"},
	}
	unchanged := &upstream.UpstreamProduct{
		ExternalID:  "product-1",
		Title:       jsonmap.JSON{"en-US": "English title", "zh-CN": "English title"},
		Description: jsonmap.JSON{"en-US": "English description", "zh-CN": "English description"},
		Content:     jsonmap.JSON{"en-US": "English content", "zh-CN": "English content"},
	}
	svc.translateProdSellerProduct(context.Background(), conn, local, unchanged)
	if translator.calls != 1 {
		t.Fatalf("unchanged product triggered another translation: %d calls", translator.calls)
	}

	changed := &upstream.UpstreamProduct{
		ExternalID:  "product-1",
		Title:       jsonmap.JSON{"en-US": "Updated title", "zh-CN": "Updated title"},
		Description: jsonmap.JSON{"en-US": "English description", "zh-CN": "English description"},
		Content:     jsonmap.JSON{"en-US": "English content", "zh-CN": "English content"},
	}
	svc.translateProdSellerProduct(context.Background(), conn, local, changed)
	if translator.calls != 2 {
		t.Fatalf("changed source did not trigger translation: %d calls", translator.calls)
	}
}

func TestTranslateProdSellerProductKeepsUpstreamChinese(t *testing.T) {
	translator := &fakeProductTranslator{}
	svc := &Service{translator: translator}
	conn := &siteconnectiondomain.Connection{Protocol: constants.ConnectionProtocolProdSeller}
	product := &upstream.UpstreamProduct{
		ExternalID: "product-2",
		Title:      jsonmap.JSON{"en-US": "English title", "zh-CN": "已有中文标题"},
	}
	svc.translateProdSellerProduct(context.Background(), conn, nil, product)
	if translator.calls != 0 {
		t.Fatalf("upstream Chinese value should not be translated, calls=%d", translator.calls)
	}
}

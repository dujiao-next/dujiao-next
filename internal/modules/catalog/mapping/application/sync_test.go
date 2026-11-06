package application

import (
	"context"
	"errors"
	"testing"
	"time"

	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/upstream"

	"github.com/shopspring/decimal"
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

// failingCategoryAdapter 让分类接口直接报错，商品接口正常返回。
type failingCategoryAdapter struct {
	upstream.Adapter
	product *upstream.UpstreamProduct
}

func (a failingCategoryAdapter) ListCategories(_ context.Context) (*upstream.CategoryListResult, error) {
	return nil, errors.New("upstream category endpoint timed out")
}

func (a failingCategoryAdapter) GetProduct(_ context.Context, _ uint) (*upstream.UpstreamProduct, error) {
	return a.product, nil
}

type staticConnectionProvider struct {
	conn    *siteconnectiondomain.Connection
	adapter upstream.Adapter
}

func (p staticConnectionProvider) GetByID(_ uint) (*siteconnectiondomain.Connection, error) {
	return p.conn, nil
}

func (p staticConnectionProvider) GetAdapter(_ *siteconnectiondomain.Connection) (upstream.Adapter, error) {
	return p.adapter, nil
}

type staticMappingRepo struct {
	mappingcontract.MappingRepository
	mapping *mappingdomain.Mapping
}

func (r staticMappingRepo) GetByID(_ uint) (*mappingdomain.Mapping, error) {
	return r.mapping, nil
}

// recordingSKURepo 记录被写回的 SKU，用来断言价格没被动过。
type recordingSKURepo struct {
	mappingcontract.SKURepository
	updates []*productdomain.ProductSKU
}

func (r *recordingSKURepo) Update(item *productdomain.ProductSKU) error {
	copied := *item
	r.updates = append(r.updates, &copied)
	return nil
}

func (r *recordingSKURepo) GetByID(_ uint) (*productdomain.ProductSKU, error) {
	return &productdomain.ProductSKU{ID: 1, PriceAmount: money.FromDecimal(decimal.NewFromInt(100))}, nil
}

// 分类接口报错时，已启用自动同步价格的商品售价必须保持不变。
// 这是修复前最危险的行为：拿不到父分类就退回连接默认加价（可能是 0%），
// 把「父分类 +50%」的商品按成本价写回数据库。
func TestSyncProductKeepsPriceWhenCategoryTreeUnavailable(t *testing.T) {
	markups := jsonmap.JSON{"1": float64(50)}
	conn := &siteconnectiondomain.Connection{
		ID:                 1,
		ExchangeRate:       decimal.NewFromInt(1),
		PriceMarkupPercent: decimal.Zero,
		CategoryMarkups:    markups,
		AutoSyncPrice:      true,
	}
	mapping := &mappingdomain.Mapping{ID: 1, ConnectionID: 1, LocalProductID: 10, UpstreamProductID: 20}
	adapter := failingCategoryAdapter{product: &upstream.UpstreamProduct{
		ID:          20,
		CategoryID:  2, // 自身没配加价，需要向父分类 1 继承 50%
		IsActive:    true,
		PriceAmount: "10",
	}}
	skus := &recordingSKURepo{}
	svc := &Service{
		mappings:    staticMappingRepo{mapping: mapping},
		connections: staticConnectionProvider{conn: conn, adapter: adapter},
		skus:        skus,
	}

	err := svc.SyncProduct(1)
	if !errors.Is(err, mappingcontract.ErrUpstreamCategoryUnavailable) {
		t.Fatalf("error = %v, want ErrUpstreamCategoryUnavailable", err)
	}
	if len(skus.updates) != 0 {
		t.Fatalf("price must not be written when the category tree is unavailable, got %d SKU updates", len(skus.updates))
	}
}

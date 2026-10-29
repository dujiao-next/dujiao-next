package application

import (
	"context"
	"testing"

	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/upstream"

	"github.com/shopspring/decimal"
)

func TestConvertUpstreamWholesalePricesRemapsUpstreamSKUScope(t *testing.T) {
	tiers := convertUpstreamWholesalePrices(productdomain.WholesalePriceTiers{
		{SKUID: 201, MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
	}, decimal.NewFromInt(1), decimal.Zero, "none", buildUpstreamWholesaleSKUIndex(
		[]productdomain.ProductSKU{{ID: 11, SKUCode: "SKU-A"}},
		[]upstream.UpstreamSKU{{ID: 201, SKUCode: "SKU-A"}},
		nil,
	))

	if len(tiers) != 1 {
		t.Fatalf("expected 1 tier, got %d", len(tiers))
	}
	if tiers[0].SKUID != 11 || tiers[0].SKUCode != "SKU-A" {
		t.Fatalf("expected upstream SKU scope to be remapped, got %+v", tiers[0])
	}
}

func TestConvertUpstreamWholesalePricesDropsUnmappedUpstreamSKUID(t *testing.T) {
	tiers := convertUpstreamWholesalePrices(productdomain.WholesalePriceTiers{
		{SKUID: 201, MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
	}, decimal.NewFromInt(1), decimal.Zero, "none")

	if len(tiers) != 0 {
		t.Fatalf("expected unmapped upstream sku_id tier to be dropped, got %+v", tiers)
	}
}

// pagedProductsAdapter 只实现 ListProducts：前 total 条商品按页返回，ignorePage 模拟上游忽略 page 参数。
type pagedProductsAdapter struct {
	upstream.Adapter
	total      int
	ignorePage bool
	calls      int
}

func (a *pagedProductsAdapter) ListProducts(_ context.Context, opts upstream.ListProductsOpts) (*upstream.ProductListResult, error) {
	a.calls++
	page := opts.Page
	if a.ignorePage {
		page = 1
	}
	var items []upstream.UpstreamProduct
	for id := (page-1)*opts.PageSize + 1; id <= a.total && len(items) < opts.PageSize; id++ {
		items = append(items, upstream.UpstreamProduct{ID: uint(id), CategoryID: uint(id % 3)})
	}
	if a.ignorePage {
		return &upstream.ProductListResult{Total: 1 << 30, Items: items}, nil
	}
	return &upstream.ProductListResult{Total: a.total, Items: items}, nil
}

func TestListAllUpstreamProductsWalksEveryPage(t *testing.T) {
	adapter := &pagedProductsAdapter{total: 120}
	products, err := listAllUpstreamProducts(context.Background(), adapter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(products) != 120 || adapter.calls != 3 {
		t.Fatalf("expected 120 products in 3 calls, got %d in %d", len(products), adapter.calls)
	}
}

func TestListAllUpstreamProductsStopsAtPageLimit(t *testing.T) {
	adapter := &pagedProductsAdapter{total: 50, ignorePage: true}
	if _, err := listAllUpstreamProducts(context.Background(), adapter); err == nil {
		t.Fatal("expected error when upstream pagination never terminates")
	}
	if adapter.calls != maxUpstreamProductPages {
		t.Fatalf("expected %d calls, got %d", maxUpstreamProductPages, adapter.calls)
	}
}

// updateRecordingMappings 只实现 Update，用于验证回填写入了哪些映射。
type updateRecordingMappings struct {
	mappingcontract.MappingRepository
	updated map[uint]uint
}

func (r *updateRecordingMappings) Update(m *mappingdomain.Mapping) error {
	r.updated[m.ID] = m.UpstreamCategoryID
	return nil
}

func TestBackfillUpstreamCategoryIDsFillsOnlyMissing(t *testing.T) {
	repo := &updateRecordingMappings{updated: map[uint]uint{}}
	svc := &Service{mappings: repo}
	adapter := &pagedProductsAdapter{total: 5}
	// 商品 ID → 分类 ID = id%3：1→1, 2→2, 3→0, 4→1
	mappings := []mappingdomain.Mapping{
		{ID: 1, UpstreamProductID: 1},
		{ID: 2, UpstreamProductID: 2, UpstreamCategoryID: 99}, // 已有分类，不能被覆盖
		{ID: 3, UpstreamProductID: 3},                         // 上游未分类，保持 0
		{ID: 4, UpstreamProductID: 4},
	}

	svc.backfillUpstreamCategoryIDs(context.Background(), adapter, 1, mappings)

	if mappings[0].UpstreamCategoryID != 1 || mappings[1].UpstreamCategoryID != 99 ||
		mappings[2].UpstreamCategoryID != 0 || mappings[3].UpstreamCategoryID != 1 {
		t.Fatalf("unexpected backfill result: %+v", mappings)
	}
	if len(repo.updated) != 2 || repo.updated[1] != 1 || repo.updated[4] != 1 {
		t.Fatalf("expected only mappings 1 and 4 persisted, got %v", repo.updated)
	}

	// 全部都有分类时不应再请求上游
	adapter.calls = 0
	svc.backfillUpstreamCategoryIDs(context.Background(), adapter, 1, []mappingdomain.Mapping{{ID: 9, UpstreamCategoryID: 5}})
	if adapter.calls != 0 {
		t.Fatalf("expected no upstream request when nothing to backfill, got %d", adapter.calls)
	}
}

package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	siteconnection "github.com/dujiao-next/internal/modules/siteconnection/contract"
	connectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/modules/upstreamwatch/contract"
	"github.com/dujiao-next/internal/modules/upstreamwatch/domain"
	"github.com/dujiao-next/internal/modules/upstreamwatch/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/upstream"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type testConnections struct{ adapter upstream.Adapter }

func (c testConnections) CatalogConnections() ([]siteconnection.CatalogConnection, error) {
	return []siteconnection.CatalogConnection{{ID: 1, Name: "Test supplier", Status: constants.ConnectionStatusActive}}, nil
}

func (c testConnections) OpenCatalog(uint) (upstream.Adapter, error) { return c.adapter, nil }

type catalogFixture struct {
	products        []upstream.UpstreamProduct
	breakSecondPage bool
	unsupported     bool
	directoryOnly   bool
	retryAfter      *time.Time
	categoryReads   *atomic.Int64
	productReads    *atomic.Int64
	unscopedReads   *atomic.Int64
	thirdLevelReads *atomic.Int64
}

func (f catalogFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/v1/public/products" {
		http.NotFound(w, r)
		return
	}
	if r.URL.Path == "/api/v1/upstream/categories" {
		f.categoryReads.Add(1)
		if f.unsupported {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "product_category_filter": !f.directoryOnly, "product_updated_after_filter": !f.directoryOnly, "categories": []upstream.UpstreamCategory{
			{ID: 2, Name: jsonmap.JSON{"zh-CN": "Accounts"}},
			{ID: 7, ParentID: 2, Name: jsonmap.JSON{"zh-CN": "Email"}},
			{ID: 8, Name: jsonmap.JSON{"zh-CN": "Other"}},
			{ID: 9, ParentID: 7, Name: jsonmap.JSON{"zh-CN": "Gmail"}},
			{ID: 10, ParentID: 9, Name: jsonmap.JSON{"zh-CN": "Business"}},
		}})
		return
	}
	q := r.URL.Query()
	category, _ := strconv.Atoi(q.Get("category_id"))
	if f.productReads != nil {
		f.productReads.Add(1)
		if category == 0 || category == 8 || category == 10 {
			f.unscopedReads.Add(1)
		}
		if category == 9 {
			f.thirdLevelReads.Add(1)
		}
	}
	if f.retryAfter != nil {
		w.Header().Set("Retry-After", f.retryAfter.UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	if f.directoryOnly {
		category = 0
	}
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("page_size"))
	if f.breakSecondPage && page == 2 {
		// A perfectly formed HTTP response with a missing page still fails the scan.
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "category_id": category, "items": []int{}, "total": len(f.products)})
		return
	}
	results := make([]upstream.UpstreamProduct, 0, len(f.products))
	for _, item := range f.products {
		if !f.directoryOnly && item.CategoryID != uint(category) {
			continue
		}
		cutoff, _ := time.Parse(time.RFC3339, q.Get("updated_after"))
		if q.Get("updated_after") == "" || item.UpdatedAt.After(cutoff) {
			results = append(results, item)
		}
	}
	start := min((page-1)*size, len(results))
	end := min(start+size, len(results))
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "category_id": category, "items": results[start:end], "total": len(results)})
}

func TestCatalogWatchBaselineNewItemsFailuresAndImport(t *testing.T) {
	clock := time.Date(2026, time.November, 23, 15, 0, 0, 0, chinaTime)
	catalogItem := func(id uint, name string, timestamp time.Time) upstream.UpstreamProduct {
		return upstream.UpstreamProduct{
			ID: id, Title: jsonmap.JSON{"zh-CN": name}, IsActive: true, UpdatedAt: timestamp,
			PriceAmount: "7.00", Currency: "CNY", CategoryID: 7, SKUs: []upstream.UpstreamSKU{{IsActive: true, StockQuantity: 3}},
		}
	}
	original := func() []upstream.UpstreamProduct {
		return []upstream.UpstreamProduct{catalogItem(11, "Existing", clock.Add(-time.Hour)), catalogItem(12, "Already imported", clock.Add(-time.Hour))}
	}
	var categoryReads, productReads, unscopedReads, thirdLevelReads atomic.Int64
	fixture := catalogFixture{products: original(), categoryReads: &categoryReads, productReads: &productReads, unscopedReads: &unscopedReads, thirdLevelReads: &thirdLevelReads}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture.ServeHTTP(w, r)
	}))
	defer server.Close()
	adapter := upstream.NewDujiaoNextAdapter(&connectiondomain.Connection{BaseURL: server.URL}, t.TempDir())

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&domain.Scan{}, &domain.ScopeCategory{}, &domain.Product{}, &domain.UnavailableProduct{}, &domain.Category{}, &connectiondomain.Connection{}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO site_connections (id,name,base_url,api_key,api_secret,status) VALUES (1,'Example','https://example.invalid','test','test','active')`,
		`CREATE TABLE product_mappings (connection_id INTEGER, upstream_product_id INTEGER, deleted_at DATETIME)`,
		`INSERT INTO product_mappings VALUES (1,12,NULL)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	service := New(gormstore.New(db), testConnections{adapter})
	service.now = func() time.Time { return clock }
	service.requestInterval = 0
	run := func() ScanView {
		t.Helper()
		if err := service.RunDue(t.Context()); err != nil {
			t.Fatal(err)
		}
		rows, err := service.Overview(t.Context())
		if err != nil || len(rows) != 1 {
			t.Fatalf("overview: %+v %v", rows, err)
		}
		return rows[0]
	}
	assertList := func(historical bool, ids ...uint) []domain.Product {
		t.Helper()
		rows, count, err := service.List(t.Context(), contract.ListFilter{Historical: historical, Page: 1, PageSize: 20})
		if err != nil || count != int64(len(ids)) || len(rows) != len(ids) {
			t.Fatalf("products historical=%v: found %d, want %d, err=%v", historical, count, len(ids), err)
		}
		for i, id := range ids {
			if rows[i].UpstreamProductID != id {
				t.Fatalf("row %d product = %d, want %d", i, rows[i].UpstreamProductID, id)
			}
		}
		return rows
	}
	if view := run(); view.State != "unconfigured" || productReads.Load() != 0 || categoryReads.Load() != 0 {
		t.Fatalf("unconfigured source was contacted: %+v", view)
	}
	fixture.unsupported = true
	if err := service.SaveSettings(t.Context(), 1, []uint{2}); !errors.Is(err, ErrScopeUnsupported) || productReads.Load() != 0 {
		t.Fatalf("unsupported source could be subscribed: %v", err)
	}
	fixture.unsupported = false
	if err := service.SaveSettings(t.Context(), 1, []uint{10}); !errors.Is(err, ErrScopeCategory) {
		t.Fatalf("fourth-level direct subscription accepted: %v", err)
	}
	if err := service.SaveSettings(t.Context(), 1, []uint{2, 7, 7}); err != nil {
		t.Fatal(err)
	}
	baseline := run()
	if baseline.Mode != incrementalMode || baseline.State != "ready" || !baseline.NextScanAt.Equal(clock.Add(domain.ScanInterval)) {
		t.Fatalf("baseline not ready: %+v", baseline)
	}
	initialReads := productReads.Load()
	clock = clock.Add(domain.ScanInterval - time.Minute)
	run()
	if productReads.Load() != initialReads {
		t.Fatal("scan ran before the three-hour interval elapsed")
	}
	clock = clock.Add(time.Minute)
	assertList(false)
	first := assertList(true, 11)[0]
	groups, err := service.CategoryGroups(t.Context(), 1, true)
	if err != nil || len(groups) != 1 || groups[0].ID != 7 || groups[0].Total != 1 ||
		len(groups[0].Path) != 2 || groups[0].Path[0]["zh-CN"] != "Accounts" || groups[0].Path[1]["zh-CN"] != "Email" {
		t.Fatalf("wrong cached groups: %+v %v", groups, err)
	}
	categoryID := uint(7)
	filter := contract.ListFilter{Historical: true, ConnectionID: 1, CategoryID: &categoryID, Page: 1, PageSize: 20}
	if rows, count, err := service.List(t.Context(), filter); err != nil || count != 1 || rows[0].UpstreamProductID != 11 {
		t.Fatalf("category filter: %+v %d %v", rows, count, err)
	}
	categoryID = 0
	if _, count, err := service.List(t.Context(), filter); err != nil || count != 0 {
		t.Fatalf("empty category filter: %d %v", count, err)
	}

	fixture.products = append(original(), catalogItem(13, "First new listing", clock))
	inScope, outOfScope := catalogItem(15, "Direct first-level listing", clock), catalogItem(16, "Other category", clock)
	thirdLevel, fourthLevel := catalogItem(17, "Third level listing", clock), catalogItem(18, "Fourth level listing", clock)
	inScope.CategoryID, outOfScope.CategoryID, thirdLevel.CategoryID, fourthLevel.CategoryID = 2, 8, 9, 10
	fixture.products = append(fixture.products, inScope, outOfScope, thirdLevel, fourthLevel)
	fixture.products[0].UpdatedAt = clock
	fixture.products[0].Title = jsonmap.JSON{"zh-CN": "Changed upstream"}
	fixture.products[0].PriceAmount = "99.00"
	if view := run(); view.State != "ready" {
		t.Fatalf("incremental scan not ready: %+v", view)
	}
	newItem := assertList(false, 13, 15)[0]
	if newItem.Stock == nil || *newItem.Stock != 3 || !newItem.Active || !newItem.Present || newItem.Price != "7.00" ||
		newItem.Relisted || newItem.Restocked {
		t.Fatalf("incorrect product details: %+v", newItem)
	}
	if item := assertList(true, 11)[0]; item.Title["zh-CN"] != "Existing" || item.Price != "7.00" ||
		!item.FirstSeenAt.Equal(first.FirstSeenAt) || unscopedReads.Load() != 0 || thirdLevelReads.Load() != 0 {
		t.Fatalf("historical cache was rewritten: %+v, category reads %d", item, categoryReads.Load())
	}
	assertUnavailable := func(id uint) {
		t.Helper()
		var cached, unavailable int64
		if err := db.Model(&domain.Product{}).Where("connection_id = 1 AND upstream_product_id = ?", id).Count(&cached).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&domain.UnavailableProduct{}).Where("connection_id = 1 AND upstream_product_id = ?", id).Count(&unavailable).Error; err != nil {
			t.Fatal(err)
		}
		if cached != 0 || unavailable != 1 {
			t.Fatalf("inactive %d: details=%d identity=%d", id, cached, unavailable)
		}
	}

	// Existing deployments have a baseline with no category assignments.
	if err := db.Exec("UPDATE upstream_catalog_products SET category_id=NULL WHERE connection_id=1").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE upstream_catalog_scans SET last_full=NULL WHERE connection_id=1").Error; err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(domain.ScanInterval)
	if view := run(); view.State != "ready" || !view.LastFull.Equal(clock) {
		t.Fatalf("category backfill did not run: %+v", view)
	}
	if item := assertList(true, 11)[0]; item.CategoryID == nil || *item.CategoryID != 7 ||
		!item.FirstSeenAt.Equal(first.FirstSeenAt) || item.IsNew {
		t.Fatalf("category backfill changed the historical baseline: %+v", item)
	}
	baseline = run()

	// At the next daily cutoff the scan must be full. A failed second page changes no product or cursor.
	clock = time.Date(2026, time.November, 24, 4, 0, 0, 0, chinaTime)
	fixture.products = original()
	for id := uint(20); id < 71; id++ {
		fixture.products = append(fixture.products, catalogItem(id, fmt.Sprint(id), clock))
	}
	fixture.breakSecondPage = true
	failed := run()
	if failed.State != "error" || failed.ErrorCode != "incomplete" || !failed.LastSuccess.Equal(*baseline.LastSuccess) ||
		!failed.LastFull.Equal(*baseline.LastFull) {
		t.Fatalf("failed scan advanced last success or baseline: %+v", failed)
	}
	if item := assertList(false, 13, 15)[0]; !item.Present {
		t.Fatal("incomplete full scan removed existing product")
	}

	clock = clock.Add(domain.ScanInterval)
	fixture.breakSecondPage = false
	fixture.products = original()
	if view := run(); view.State != "ready" {
		t.Fatalf("full scan retry failed: %+v", view)
	}
	assertList(false)
	clock = clock.Add(domain.ScanInterval)
	fixture.products = append(original(), catalogItem(13, "Relisted", clock))
	run()
	relisted := assertList(false, 13)[0]
	if !relisted.Present || relisted.Relisted || !relisted.FirstSeenAt.Equal(newItem.FirstSeenAt) || relisted.Title["zh-CN"] != "Relisted" {
		t.Fatalf("scoped absence was mistaken for a delisting: %+v", relisted)
	}
	if err := db.Exec("INSERT INTO product_mappings VALUES (1,13,NULL)").Error; err != nil {
		t.Fatal(err)
	}
	assertList(false)
	assertList(true, 11)

	// A delisting returned by an incremental feed removes the historical details.
	clock = clock.Add(domain.ScanInterval)
	item := catalogItem(11, "Delisted historical", clock)
	item.IsActive = false
	fixture.products[0] = item
	run()
	assertUnavailable(11)
	assertList(true)
	if groups, err := service.CategoryGroups(t.Context(), 1, true); err != nil || len(groups) != 0 {
		t.Fatalf("delisted item still counted in historical categories: %+v %v", groups, err)
	}
	clock = clock.Add(domain.ScanInterval)
	item = catalogItem(11, "Relisted historical", clock)
	item.SKUs[0].StockStatus = constants.ProductStockStatusOutOfStock
	item.SKUs[0].StockQuantity = 0
	fixture.products[0] = item
	run()
	assertUnavailable(11)
	assertList(false)

	// An unreported stock count cannot confirm a restock.
	clock = clock.Add(domain.ScanInterval)
	item.UpdatedAt = clock
	item.SKUs = nil
	fixture.products[0] = item
	run()
	assertUnavailable(11)
	assertList(false)

	clock = clock.Add(domain.ScanInterval)
	item = catalogItem(11, "Relisted and restocked", clock)
	fixture.products[0] = item
	stockOnly := catalogItem(14, "Initially out of stock", clock)
	stockOnly.SKUs[0].StockQuantity = 0
	fixture.products = append(fixture.products, stockOnly)
	run()
	assertUnavailable(14)
	record := assertList(false, 11)[0]
	if !record.Relisted || !record.Restocked || record.Stock == nil || *record.Stock != 3 || !record.FirstSeenAt.Equal(clock) {
		t.Fatalf("restored product lost its relisting or restocking label: %+v", record)
	}
	clock = clock.Add(domain.ScanInterval)
	item.UpdatedAt = clock
	fixture.products[0] = item
	fixture.products[len(fixture.products)-1] = catalogItem(14, "Now available", clock)
	run()
	restored := assertList(false, 14, 11)
	if !restored[0].Restocked || restored[0].Relisted || !restored[0].FirstSeenAt.Equal(clock) {
		t.Fatalf("stock-only recovery is a restock: %+v", restored[0])
	}
	if repeated := restored[1]; !repeated.Relisted || !repeated.Restocked || !repeated.FirstSeenAt.Equal(record.FirstSeenAt) {
		t.Fatalf("a continuous listing changed discovery time or label: %+v", repeated)
	}
	fixture.products = append(fixture.products, thirdLevel, fourthLevel)
	if err := service.SaveSettings(t.Context(), 1, []uint{9}); err != nil {
		t.Fatal(err)
	}
	assertList(false)
	reads := productReads.Load()
	if view := run(); view.State != "ready" || thirdLevelReads.Load() == 0 || productReads.Load()-reads != thirdLevelReads.Load() {
		t.Fatalf("third-level subscription scanned outside its own category: %+v", view)
	}
	assertList(false)
	fixture.unsupported = true
	if err := db.Model(&domain.Scan{}).Where("connection_id = 1").Updates(map[string]any{
		"error_code": domain.ErrorScopeUnsupported, "next_scan_at": clock.UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if view := run(); view.State != "error" || view.ErrorCode != domain.ErrorScopeUnsupported || view.NextScanAt != nil {
		t.Fatalf("unsupported source should stop automatic retries after the legacy retry: %+v", view)
	}
	categoryReadsBeforePause := categoryReads.Load()
	clock = clock.Add(2 * time.Hour)
	if view := run(); view.NextScanAt != nil || categoryReads.Load() != categoryReadsBeforePause {
		t.Fatalf("paused subscription contacted its source: %+v", view)
	}

	if unscopedReads.Load() != 0 {
		t.Fatal("used directory reads for an exact-category feed")
	}
	fixture.unsupported, fixture.directoryOnly = false, true
	fixture.products = []upstream.UpstreamProduct{thirdLevel, fourthLevel}
	for id := uint(100); id <= 150; id++ {
		product := catalogItem(id, "Unsubscribed directory product", clock)
		product.CategoryID = 8
		fixture.products = append(fixture.products, product)
	}
	if err := service.SaveSettings(t.Context(), 1, []uint{7, 9}); err != nil {
		t.Fatal(err)
	}
	if view := run(); view.State != "ready" || view.Mode != snapshotMode || unscopedReads.Load() != 2 {
		t.Fatalf("expected one two-page directory baseline for both subscriptions: %+v, pages=%d", view, unscopedReads.Load())
	}
	assertList(false)
	directoryStart := unscopedReads.Load()
	productStart := productReads.Load()
	clock = clock.Add(domain.ScanInterval)
	fixture.products = append(fixture.products, catalogItem(200, "New subscribed product", clock))
	if view := run(); view.State != "ready" || unscopedReads.Load()-directoryStart != 2 || productReads.Load()-productStart != 2 {
		t.Fatalf("expected one two-page directory read without further scoped probes: %+v", view)
	}
	assertList(false, 200)
	var excludedCount int64
	if err := db.Model(&domain.Product{}).Where("upstream_product_id = 18 OR upstream_product_id BETWEEN 100 AND 150").Count(&excludedCount).Error; err != nil || excludedCount != 0 {
		t.Fatalf("directory fallback stored unsubscribed products: %d, %v", excludedCount, err)
	}
	clock = clock.Add(domain.ScanInterval)
	retryAfter := clock.Add(5 * time.Hour)
	fixture.retryAfter = &retryAfter
	if view := run(); view.ErrorCode != "rate_limited" || !view.NextScanAt.Equal(retryAfter) {
		t.Fatalf("expected the supplier's longer Retry-After to set the next scan: %+v", view)
	}
	fixture.retryAfter = nil
	readsBeforeRetry := productReads.Load()
	clock = clock.Add(domain.ScanInterval)
	run()
	if productReads.Load() != readsBeforeRetry {
		t.Fatal("scan ran before the supplier's cooldown expired")
	}

	reads = productReads.Load()
	if err := service.SaveSettings(t.Context(), 1, nil); err != nil {
		t.Fatal(err)
	}
	if view := run(); view.State != "unconfigured" || productReads.Load() != reads {
		t.Fatalf("disabled subscription still scans: %+v", view)
	}
	assertList(false)
}

func TestCatalogWatchRejectsRepeatedIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = strings.NewReader(`{"ok":true,"category_id":7,"total":2,"items":[{"id":5,"category_id":7}]}`).WriteTo(w)
	}))
	defer server.Close()
	adapter := upstream.NewDujiaoNextAdapter(&connectiondomain.Connection{BaseURL: server.URL}, t.TempDir())
	if data, err := readCatalog(t.Context(), adapter, 1, upstream.ListProductsOpts{CategoryID: 7}, map[uint]bool{7: false}, time.Now()); data != nil || err == nil {
		t.Fatalf("repeated IDs accepted: %+v %v", data, err)
	}
}

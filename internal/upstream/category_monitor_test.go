package upstream

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	siteconnection "github.com/dujiao-next/internal/modules/siteconnection/domain"
)

func TestCatalogPacingWaitsBetweenRequestsAndCancels(t *testing.T) {
	const interval = 25 * time.Millisecond
	ctx := WithCatalogPacing(t.Context(), interval)
	start := time.Now()
	if err := waitCatalogRequest(ctx); err != nil {
		t.Fatal(err)
	}
	blocked, cancel := context.WithCancel(ctx)
	cancel()
	if err := waitCatalogRequest(blocked); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request bypassed pacing: %v", err)
	}
	if err := waitCatalogRequest(ctx); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < interval-time.Millisecond {
		t.Fatal("second request started before the pacing interval")
	}
}

func TestDujiaoCategoryMonitorUsesProtocolCatalog(t *testing.T) {
	disableRequestPacing(t)
	for _, test := range []struct {
		name, categories, storefront, signed string
		expectedDetails, listingRequests     int64
		excluded                             int
		incremental, unsupported             bool
	}{
		{"bridge signed", `"product_category_filter":true,`, "", `{"total":1,"category_id":2,"items":[{"id":3,"category_id":2}]}`, 0, 1, 0, false, false},
		{"declared incremental", `"product_category_filter":true,"product_updated_after_filter":true,`, "", `{"total":1,"category_id":2,"items":[{"id":3,"category_id":2}]}`, 0, 1, 0, true, false},
		{"legacy storefront", "", `{"status_code":0,"data":[{"id":3,"category_id":2},{"id":4,"category_id":5}],"pagination":{"page":1,"total":2}}`, "", 1, 1, 1, false, false},
		{"legacy signed", "", "", `{"total":1,"items":[{"id":3,"category_id":2}]}`, 0, 2, 0, false, false},
		{"scope ignored", "", `{"status_code":0,"data":[{"id":3,"category_id":2},{"id":6,"category_id":9}],"pagination":{"page":1,"total":2}}`, `{"total":2,"items":[{"id":3,"category_id":2},{"id":6,"category_id":9}]}`, 0, 2, 0, false, true},
		{"storefront and signed unavailable", "", "", "", 0, 2, 0, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var requests, details atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				switch r.URL.Path {
				case "/api/v1/upstream/categories":
					_, _ = io.WriteString(w, `{"ok":true,`+test.categories+`"categories":[{"id":2},{"id":5,"parent_id":2},{"id":9}]}`)
				case "/api/v1/upstream/products", "/api/v1/public/products":
					if r.URL.Query().Get("category_id") != "2" {
						t.Error("monitor issued an unscoped read or a capability probe")
					}
					listing := test.signed
					if r.URL.Path == "/api/v1/public/products" {
						if test.categories != "" {
							t.Error("declared signed feed fell back to the storefront")
						}
						listing = test.storefront
					}
					if r.URL.Path == "/api/v1/public/products" && r.Header.Get(HeaderApiKey) != "" {
						t.Error("public request included signed credentials")
					}
					if listing == "" {
						http.NotFound(w, r)
						return
					}
					_, _ = io.WriteString(w, listing)
				case "/api/v1/upstream/products/3":
					details.Add(1)
					_, _ = io.WriteString(w, `{"ok":true,"product":{"id":3,"category_id":2,"price_amount":"2.30","is_active":true}}`)
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			adapter := NewDujiaoNextAdapter(&siteconnection.Connection{BaseURL: server.URL, ApiKey: "test"}, t.TempDir())
			monitor, err := adapter.OpenCategoryMonitor(t.Context())
			if err != nil || monitor.Incremental != test.incremental || requests.Load() != 1 {
				t.Fatalf("opening the monitor probed products or selected the wrong mode: %+v, %v", monitor, err)
			}
			products, err := monitor.ListProducts(t.Context(), ListProductsOpts{CategoryID: 2, Page: 1, PageSize: 2})
			if details.Load() != test.expectedDetails || requests.Load() != 1+test.listingRequests+test.expectedDetails {
				t.Fatalf("unexpected product detail reads: %d", details.Load())
			}
			if test.unsupported {
				if products != nil || !errors.Is(err, ErrScopedCatalogUnsupported) {
					t.Fatalf("unverified listing accepted: %+v, %v", products, err)
				}
				return
			}
			if err != nil || products.CategoryID != 2 || len(products.Items) != 1 || len(products.ExcludedProductIDs) != test.excluded {
				t.Fatalf("wrong scoped listing: %+v, %v", products, err)
			}
			if test.expectedDetails > 0 && products.Items[0].PriceAmount != "2.30" {
				t.Fatal("monitor did not use the authenticated upstream price")
			}
		})
	}
}

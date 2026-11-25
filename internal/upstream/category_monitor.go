package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var ErrScopedCatalogUnsupported = errors.New("upstream protocol does not provide a scoped product catalog")

type CategoryMonitor struct {
	Categories  []UpstreamCategory
	Incremental bool
	Directory   ProductCatalogReader
	list        func(context.Context, ListProductsOpts) (*ProductListResult, error)
}

type ProductCatalogReader interface {
	ListProducts(context.Context, ListProductsOpts) (*ProductListResult, error)
}

type CategoryMonitorProvider interface {
	OpenCategoryMonitor(context.Context) (*CategoryMonitor, error)
}

func (m *CategoryMonitor) ListProducts(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error) {
	if opts.CategoryID == 0 {
		return nil, errors.New("category monitor requires a category ID")
	}
	if m.list == nil {
		return nil, ErrScopedCatalogUnsupported
	}
	return m.list(ctx, opts)
}

// OpenCategoryMonitor uses the catalog contract of the Dujiao-Next protocol.
// Bridges with a signed category feed declare it in the normal category
// response. Earlier sites can expose the selected category either through
// the storefront or through the signed feed without a category echo.
func (a *DujiaoNextAdapter) OpenCategoryMonitor(ctx context.Context) (*CategoryMonitor, error) {
	result, err := a.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	if result == nil || !result.Supported {
		return nil, ErrScopedCatalogUnsupported
	}
	monitor := &CategoryMonitor{Categories: result.Categories}
	if result.ProductCategoryFilter {
		monitor.Incremental = result.ProductUpdatedAfterFilter
		monitor.list = a.ListProducts
		return monitor, nil
	}

	parents := make(map[uint]uint, len(result.Categories))
	monitor.Directory = a
	for _, category := range result.Categories {
		parents[category.ID] = category.ParentID
	}
	var useSigned bool
	monitor.list = func(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error) {
		if useSigned {
			return a.listLegacySignedCategory(ctx, opts)
		}
		products, err := a.listStorefrontCategory(ctx, opts, parents)
		if opts.Page != 1 || !errors.Is(err, ErrScopedCatalogUnsupported) {
			return products, err
		}
		products, err = a.listLegacySignedCategory(ctx, opts)
		useSigned = err == nil
		return products, err
	}
	return monitor, nil
}

func (a *DujiaoNextAdapter) listLegacySignedCategory(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error) {
	result, err := a.ListProducts(ctx, opts)
	if err != nil {
		var status *upstreamHTTPError
		if errors.As(err, &status) && (status.Status == http.StatusBadRequest ||
			status.Status == http.StatusUnauthorized || status.Status == http.StatusForbidden ||
			status.Status == http.StatusNotFound || status.Status == http.StatusMethodNotAllowed) {
			return nil, ErrScopedCatalogUnsupported
		}
		return nil, err
	}
	if result == nil || result.Items == nil || result.Total < len(result.Items) ||
		len(result.Items) > opts.PageSize || (result.CategoryID != 0 && result.CategoryID != opts.CategoryID) {
		return nil, ErrScopedCatalogUnsupported
	}
	for _, item := range result.Items {
		if item.ID == 0 || item.CategoryID != opts.CategoryID {
			return nil, ErrScopedCatalogUnsupported
		}
	}
	result.CategoryID = opts.CategoryID
	return result, nil
}

func (a *DujiaoNextAdapter) listStorefrontCategory(ctx context.Context, opts ListProductsOpts, parents map[uint]uint) (*ProductListResult, error) {
	endpoint := fmt.Sprintf("%s/api/v1/public/products?page=%d&page_size=%d&category_id=%d",
		a.baseURL, opts.Page, opts.PageSize, opts.CategoryID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if err := waitCatalogRequest(ctx); err != nil {
		return nil, err
	}
	response, err := a.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if limit := catalogRateLimit(ctx, response); limit != nil {
		return nil, limit
	}
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusMethodNotAllowed:
		return nil, ErrScopedCatalogUnsupported
	default:
		return nil, fmt.Errorf("storefront catalog returned HTTP %d", response.StatusCode)
	}

	var envelope struct {
		StatusCode *int `json:"status_code"`
		Data       []struct {
			ID         uint `json:"id"`
			CategoryID uint `json:"category_id"`
		} `json:"data"`
		Pagination *struct {
			Page  int  `json:"page"`
			Total *int `json:"total"`
		} `json:"pagination"`
	}
	const maxResponseBytes = 16 << 20
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("%w: invalid storefront response", ErrScopedCatalogUnsupported)
	}
	if envelope.StatusCode == nil || envelope.Data == nil || envelope.Pagination == nil || envelope.Pagination.Total == nil {
		return nil, ErrScopedCatalogUnsupported
	}
	if *envelope.StatusCode != 0 || envelope.Pagination.Page != opts.Page ||
		*envelope.Pagination.Total < len(envelope.Data) || len(envelope.Data) > opts.PageSize {
		return nil, errors.New("invalid storefront catalog pagination")
	}

	result := &ProductListResult{CategoryID: opts.CategoryID, Total: *envelope.Pagination.Total}
	ids := make([]uint, 0, len(envelope.Data))
	seen := make(map[uint]bool, len(envelope.Data))
	for _, row := range envelope.Data {
		if row.ID == 0 || seen[row.ID] {
			return nil, errors.New("invalid storefront catalog product ID")
		}
		seen[row.ID] = true
		switch {
		case row.CategoryID == opts.CategoryID:
			ids = append(ids, row.ID)
		case categoryDescendsFrom(parents, row.CategoryID, opts.CategoryID):
			result.ExcludedProductIDs = append(result.ExcludedProductIDs, row.ID)
		default:
			return nil, ErrScopedCatalogUnsupported
		}
	}
	// The storefront supplies membership. Use authenticated details for the
	// actual upstream price and stock, only for direct members of the selection.
	result.Items = make([]UpstreamProduct, 0, len(ids))
	for _, id := range ids {
		product, err := a.GetProduct(ctx, id)
		if err != nil {
			return nil, err
		}
		if product == nil || product.ID != id || product.CategoryID != opts.CategoryID {
			return nil, errors.New("storefront catalog changed while loading product details")
		}
		result.Items = append(result.Items, *product)
	}
	return result, nil
}

func categoryDescendsFrom(parents map[uint]uint, child, ancestor uint) bool {
	for id, remaining := parents[child], len(parents); id != 0 && remaining > 0; id, remaining = parents[id], remaining-1 {
		if id == ancestor {
			return true
		}
	}
	return false
}

package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	siteconnection "github.com/dujiao-next/internal/modules/siteconnection/contract"
	"github.com/dujiao-next/internal/modules/upstreamwatch/contract"
	"github.com/dujiao-next/internal/modules/upstreamwatch/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/upstream"
)

const (
	incrementalMode = "incremental"
	snapshotMode    = "snapshot"
	cursorOverlap   = 2 * time.Minute
	scanTimeout     = 45 * time.Minute
	leaseDuration   = 46 * time.Minute
	requestInterval = 2 * time.Second
	catalogPageSize = 50
	maxCatalogPages = 800
)

var chinaTime = time.FixedZone("UTC+8", 8*60*60)
var errCatalogIncomplete = errors.New("upstream catalog incomplete")

type Connections interface {
	CatalogConnections() ([]siteconnection.CatalogConnection, error)
	OpenCatalog(id uint) (upstream.Adapter, error)
}

type Service struct {
	store           contract.Store
	connections     Connections
	now             func() time.Time
	requestInterval time.Duration
}

func New(store contract.Store, connections Connections) *Service {
	return &Service{store: store, connections: connections, now: time.Now, requestInterval: requestInterval}
}

type ScanView struct {
	siteconnection.CatalogConnection
	Mode        string     `json:"mode"`
	State       string     `json:"state"`
	LastSuccess *time.Time `json:"last_success_at"`
	LastFull    *time.Time `json:"last_full_at"`
	NextScanAt  *time.Time `json:"next_scan_at"`
	ErrorCode   string     `json:"error_code"`
	CategoryIDs []uint     `json:"category_ids"`
}

func (s *Service) Overview(ctx context.Context) ([]ScanView, error) {
	connections, scans, err := s.loadScans(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	rows := make([]ScanView, 0, len(connections))
	for _, connection := range connections {
		scan := scans[connection.ID]
		view := ScanView{
			CatalogConnection: connection, Mode: scan.Mode, State: "waiting",
			LastSuccess: scan.LastSuccess, LastFull: scan.LastFull, NextScanAt: scan.NextScanAt, ErrorCode: scan.ErrorCode,
			CategoryIDs: append([]uint{}, scan.CategoryIDs...),
		}
		switch {
		case connection.Status != constants.ConnectionStatusActive:
			view.State = "disabled"
		case len(scan.CategoryIDs) == 0:
			view.State = "unconfigured"
		case scan.LeaseUntil != nil && scan.LeaseUntil.After(now):
			view.State = "scanning"
		case scan.ErrorCode != "" || scan.LeaseUntil != nil:
			view.State = "error"
		case scan.LastSuccess != nil && scan.LastFull != nil:
			view.State = "ready"
		}
		rows = append(rows, view)
	}
	return rows, nil
}

func (s *Service) List(ctx context.Context, filter contract.ListFilter) ([]domain.Product, int64, error) {
	rows, total, err := s.store.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	categoryMaps := make(map[uint]map[uint]domain.Category)
	for i := range rows {
		product := &rows[i]
		if product.CategoryID == nil || *product.CategoryID == 0 {
			continue
		}
		if _, loaded := categoryMaps[product.ConnectionID]; !loaded {
			categories, err := s.store.Categories(ctx, product.ConnectionID)
			if err != nil {
				return nil, 0, err
			}
			categoryMaps[product.ConnectionID] = indexCategories(categories)
		}
		product.CategoryPath = categoryPath(categoryMaps[product.ConnectionID], *product.CategoryID)
	}
	return rows, total, nil
}

func (s *Service) CategoryGroups(ctx context.Context, connectionID uint, historical bool) ([]domain.CategoryGroup, error) {
	groups, err := s.store.CategoryGroups(ctx, connectionID, historical)
	if err != nil || len(groups) == 0 {
		return groups, err
	}
	categories, err := s.store.Categories(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	byID := indexCategories(categories)
	for i := range groups {
		group := &groups[i]
		group.SortOrder = byID[group.ID].SortOrder
		group.Path = categoryPath(byID, group.ID)
	}
	return groups, nil
}

func indexCategories(categories []domain.Category) map[uint]domain.Category {
	byID := make(map[uint]domain.Category, len(categories))
	for _, category := range categories {
		byID[category.ID] = category
	}
	return byID
}

func categoryPath(byID map[uint]domain.Category, id uint) []jsonmap.JSON {
	path := make([]jsonmap.JSON, 0)
	seen := make(map[uint]bool)
	for id != 0 && !seen[id] {
		category, ok := byID[id]
		if !ok {
			break
		}
		seen[id] = true
		path = append(path, category.Name)
		id = category.ParentID
	}
	slices.Reverse(path)
	return path
}

func (s *Service) RunDue(ctx context.Context) error {
	connections, scans, err := s.loadScans(ctx)
	if err != nil {
		return err
	}
	now := s.now()
	for _, connection := range connections {
		if ctx.Err() != nil {
			break
		}
		scan := scans[connection.ID]
		if connection.Status != constants.ConnectionStatusActive || len(scan.CategoryIDs) == 0 ||
			(scan.ErrorCode == domain.ErrorScopeUnsupported && scan.NextScanAt == nil) ||
			(scan.NextScanAt != nil && scan.NextScanAt.After(now)) ||
			(scan.LeaseUntil != nil && scan.LeaseUntil.After(now)) {
			continue
		}
		// The unique scheduler task processes at most one upstream at a time.
		if err := s.scan(ctx, connection.ID, scan.ScopeRevision); err != nil {
			logger.Warnw("upstream_catalog_scan_persist_failed", "connection_id", connection.ID, "error", err)
		}
		break
	}
	return nil
}

func (s *Service) loadScans(ctx context.Context) ([]siteconnection.CatalogConnection, map[uint]domain.Scan, error) {
	connections, err := s.connections.CatalogConnections()
	if err != nil {
		return nil, nil, err
	}
	ids := make([]uint, 0, len(connections))
	for _, conn := range connections {
		ids = append(ids, conn.ID)
	}
	rows, err := s.store.Scans(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	scans := make(map[uint]domain.Scan, len(rows))
	for _, row := range rows {
		scans[row.ConnectionID] = row
	}
	return connections, scans, nil
}

func (s *Service) scan(ctx context.Context, id uint, revision uint64) error {
	start := s.now().UTC().Truncate(time.Microsecond)
	scan, err := s.store.Claim(ctx, id, revision, start, start.Add(leaseDuration))
	if err != nil || scan == nil {
		return err
	}
	adapter, err := s.connections.OpenCatalog(id)
	if err != nil {
		return s.recordFailure(ctx, *scan, "connection", err)
	}

	readCtx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()
	readCtx = upstream.WithCatalogPacing(readCtx, s.requestInterval)
	catalog, err := openWatchCatalog(readCtx, adapter, id)
	if errors.Is(err, ErrScopeUnsupported) {
		return s.recordUnsupported(ctx, *scan)
	}
	if err != nil {
		return s.recordFailure(ctx, *scan, "categories", err)
	}
	scope, missingSelection := resolveScope(catalog.categories, scan.CategoryIDs)
	previous, err := s.store.Scope(readCtx, id)
	if err != nil {
		return s.recordFailure(ctx, *scan, "catalog", err)
	}
	initialized := make(map[uint]bool, len(previous))
	for _, category := range previous {
		initialized[category.CategoryID] = category.Initialized
	}
	scopeChanged := len(scope) != len(previous)
	selected := make(map[uint]bool, len(scope))
	for _, category := range scope {
		isNew := initialized[category.CategoryID]
		selected[category.CategoryID] = isNew
		scopeChanged = scopeChanged || !isNew
	}
	// The daily reconciliation and a changed scope both use scoped snapshots.
	full := scopeChanged || !catalog.products.Incremental || scan.Mode != incrementalMode || scan.Cursor == nil ||
		scan.LastFull == nil || scan.LastFull.Before(dailyCutoff(start))
	var after *time.Time
	if !full {
		window := scan.Cursor.Add(-cursorOverlap)
		after = &window
	}
	var products []domain.Product
	directory := len(scope) > 0 && catalog.products.Directory != nil && scan.DirectoryRead
	if !directory {
		products, err = readScope(readCtx, catalog.products, after, scope, selected, start)
		directory = errors.Is(err, ErrScopeUnsupported) && catalog.products.Directory != nil
	}
	if directory {
		full = true
		products, err = readCatalog(readCtx, catalog.products.Directory, id, upstream.ListProductsOpts{}, selected, start)
	}
	if errors.Is(err, ErrScopeUnsupported) {
		return s.recordUnsupported(ctx, *scan)
	}
	if err != nil {
		code := "catalog"
		if errors.Is(err, errCatalogIncomplete) {
			code = "incomplete"
		} else if errors.Is(readCtx.Err(), context.DeadlineExceeded) {
			code = "timeout"
		}
		return s.recordFailure(ctx, *scan, code, err)
	}

	scan.ErrorCode = ""
	if missingSelection {
		scan.ErrorCode = "scope_missing"
	}
	if full {
		scan.Mode = snapshotMode
		if catalog.products.Incremental && !directory {
			scan.Mode = incrementalMode
		}
		scan.LastFull = &start
	}
	next := start.Add(domain.ScanInterval)
	finished := s.now().UTC().Truncate(time.Microsecond)
	scan.DirectoryRead = directory
	scan.Cursor, scan.LastSuccess, scan.NextScanAt = &start, &finished, &next
	for i := range scope {
		scope[i].Initialized = true
	}
	saveCtx, done := context.WithTimeout(context.WithoutCancel(ctx), 25*time.Second)
	defer done()
	return s.store.Complete(saveCtx, *scan, products, catalog.categories, scope, full)
}

func (s *Service) recordUnsupported(ctx context.Context, scan domain.Scan) error {
	saveCtx, done := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer done()
	return s.store.Fail(saveCtx, scan, nil, domain.ErrorScopeUnsupported)
}

func (s *Service) recordFailure(ctx context.Context, scan domain.Scan, code string, cause error) error {
	saveCtx, done := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer done()
	retry := s.now().UTC().Add(domain.ScanInterval)
	var limited *upstream.CatalogRateLimitError
	if errors.As(cause, &limited) {
		code = "rate_limited"
		if limited.RetryAt.After(retry) {
			retry = limited.RetryAt
		}
	}
	logger.Warnw("upstream_catalog_scan_failed", "connection_id", scan.ConnectionID, "code", code)
	return s.store.Fail(saveCtx, scan, &retry, code)
}

func dailyCutoff(now time.Time) time.Time {
	local := now.In(chinaTime)
	cutoff := time.Date(local.Year(), local.Month(), local.Day(), 4, 0, 0, 0, chinaTime)
	if local.Before(cutoff) {
		cutoff = cutoff.AddDate(0, 0, -1)
	}
	return cutoff
}

func readScope(ctx context.Context, adapter upstream.ProductCatalogReader, after *time.Time, scope []domain.ScopeCategory, selected map[uint]bool, seenAt time.Time) ([]domain.Product, error) {
	var products []domain.Product
	seen := make(map[uint]bool)
	for _, category := range scope {
		rows, err := readCatalog(ctx, adapter, category.ConnectionID, upstream.ListProductsOpts{
			CategoryID: category.CategoryID, UpdatedAfter: after,
		}, selected, seenAt)
		if err != nil {
			return nil, err
		}
		for _, item := range rows {
			if seen[item.UpstreamProductID] {
				return nil, fmt.Errorf("%w: duplicate product across scoped categories", errCatalogIncomplete)
			}
			seen[item.UpstreamProductID] = true
		}
		products = append(products, rows...)
	}
	return products, nil
}

func readCatalog(ctx context.Context, adapter upstream.ProductCatalogReader, connectionID uint, opts upstream.ListProductsOpts, selected map[uint]bool, seenAt time.Time) ([]domain.Product, error) {
	var products []domain.Product
	seen := make(map[uint]struct{})
	total, stride := 0, 0
	opts.PageSize, opts.IncludeInactive = catalogPageSize, true
	for page := 1; page <= maxCatalogPages; page++ {
		pageCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		opts.Page = page
		result, err := adapter.ListProducts(pageCtx, opts)
		cancel()
		if err != nil {
			return nil, err
		}
		if result == nil || result.CategoryID != opts.CategoryID || result.Total < 0 || (result.Total == 0 && result.Items == nil) {
			return nil, errCatalogIncomplete
		}
		rawCount := len(result.Items) + len(result.ExcludedProductIDs)
		if page == 1 {
			total, stride = result.Total, rawCount
			if total > stride*maxCatalogPages {
				return nil, errCatalogIncomplete
			}
			products = make([]domain.Product, 0, min(total, catalogPageSize))
		}
		if result.Total != total || rawCount != min(stride, total-len(seen)) {
			return nil, errCatalogIncomplete
		}
		for _, id := range result.ExcludedProductIDs {
			if _, exists := seen[id]; id == 0 || exists {
				return nil, errCatalogIncomplete
			}
			seen[id] = struct{}{}
		}
		for _, item := range result.Items {
			if _, exists := seen[item.ID]; item.ID == 0 || exists || (opts.CategoryID != 0 && item.CategoryID != opts.CategoryID) {
				return nil, fmt.Errorf("%w: invalid product ID or category", errCatalogIncomplete)
			}
			seen[item.ID] = struct{}{}
			isNew, subscribed := selected[item.CategoryID]
			if !subscribed {
				continue
			}
			products = append(products, domain.Product{
				ConnectionID: connectionID, UpstreamProductID: item.ID, CategoryID: &item.CategoryID, Title: item.Title,
				Currency: strings.ToUpper(strings.TrimSpace(item.Currency)), Price: strings.TrimSpace(item.PriceAmount),
				Stock: productStock(item), Active: item.IsActive, Present: true, IsNew: isNew,
				FirstSeenAt: seenAt,
			})
		}
		if len(seen) == total {
			return products, nil
		}
	}
	return nil, errCatalogIncomplete
}

func productStock(product upstream.UpstreamProduct) *int64 {
	if !product.IsActive {
		zero := int64(0)
		return &zero
	}
	var total int64
	unknown := len(product.SKUs) == 0
	for _, sku := range product.SKUs {
		if !sku.IsActive || sku.StockStatus == constants.ProductStockStatusOutOfStock {
			continue
		}
		switch {
		case sku.StockQuantity == -1 || sku.StockStatus == constants.ProductStockStatusUnlimited:
			unlimited := int64(-1)
			return &unlimited
		case sku.StockQuantity > 0:
			total += min(math.MaxInt64-total, int64(sku.StockQuantity))
		case sku.StockQuantity == 0 && sku.StockQuantityReported && sku.StockStatus == "":
			continue
		default:
			unknown = true
		}
	}
	if unknown {
		return nil
	}
	return &total
}

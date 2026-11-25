package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dujiao-next/internal/constants"
	siteconnection "github.com/dujiao-next/internal/modules/siteconnection/contract"
	"github.com/dujiao-next/internal/modules/upstreamwatch/domain"
	"github.com/dujiao-next/internal/upstream"
)

var (
	ErrScopeUnsupported = upstream.ErrScopedCatalogUnsupported
	ErrScopeCategory    = errors.New("invalid subscription category")
)

type ScopeSettings struct {
	CategoryIDs []uint            `json:"category_ids"`
	Categories  []domain.Category `json:"categories"`
	Supported   bool              `json:"supported"`
}

func (s *Service) Settings(ctx context.Context, id uint) (ScopeSettings, error) {
	connection, scan, err := s.settingsConnection(ctx, id)
	if err != nil {
		return ScopeSettings{}, err
	}
	settings := ScopeSettings{CategoryIDs: append([]uint{}, scan.CategoryIDs...), Categories: []domain.Category{}}
	if connection.Status != constants.ConnectionStatusActive {
		return settings, nil
	}
	settings.Categories, err = s.watchCategories(ctx, id)
	if errors.Is(err, ErrScopeUnsupported) {
		settings.Categories = []domain.Category{}
		return settings, nil
	}
	settings.Supported = err == nil
	return settings, err
}

func (s *Service) SaveSettings(ctx context.Context, id uint, selected []uint) error {
	connection, _, err := s.settingsConnection(ctx, id)
	if err != nil {
		return err
	}
	selected = slices.Clone(selected)
	slices.Sort(selected)
	selected = slices.Compact(selected)
	if len(selected) == 0 {
		return s.store.SetScope(ctx, id, []uint{}, nil, nil)
	}
	if connection.Status != constants.ConnectionStatusActive {
		return ErrScopeUnsupported
	}
	categories, err := s.watchCategories(ctx, id)
	if err != nil {
		return err
	}
	scope, missing := resolveScope(categories, selected)
	if missing {
		return ErrScopeCategory
	}
	return s.store.SetScope(ctx, id, selected, categories, scope)
}

func (s *Service) settingsConnection(ctx context.Context, id uint) (siteconnection.CatalogConnection, domain.Scan, error) {
	connections, scans, err := s.loadScans(ctx)
	if err != nil {
		return siteconnection.CatalogConnection{}, domain.Scan{}, err
	}
	for _, connection := range connections {
		if connection.ID == id {
			return connection, scans[id], nil
		}
	}
	return siteconnection.CatalogConnection{}, domain.Scan{}, siteconnection.ErrNotFound
}

func (s *Service) watchCategories(ctx context.Context, id uint) ([]domain.Category, error) {
	adapter, err := s.connections.OpenCatalog(id)
	if err != nil {
		return nil, err
	}
	catalog, err := openWatchCatalog(ctx, adapter, id)
	return catalog.categories, err
}

type watchCatalog struct {
	categories []domain.Category
	products   *upstream.CategoryMonitor
}

func openWatchCatalog(ctx context.Context, adapter upstream.Adapter, id uint) (watchCatalog, error) {
	provider, ok := adapter.(upstream.CategoryMonitorProvider)
	if !ok {
		return watchCatalog{}, ErrScopeUnsupported
	}
	requestCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	result, err := provider.OpenCategoryMonitor(requestCtx)
	if err != nil {
		return watchCatalog{}, err
	}
	categories := make([]domain.Category, 0, len(result.Categories))
	seen := make(map[uint]bool, len(result.Categories))
	for _, category := range result.Categories {
		if category.ID == 0 || seen[category.ID] {
			return watchCatalog{}, fmt.Errorf("%w: bad category ID", errCatalogIncomplete)
		}
		seen[category.ID] = true
		categories = append(categories, domain.Category{
			ConnectionID: id, ID: category.ID, ParentID: category.ParentID, Name: category.Name, SortOrder: category.SortOrder,
		})
	}
	// Verify the whole tree before using it to remove categories from scope.
	if len(categoryLevels(categories)) != len(categories) {
		return watchCatalog{}, fmt.Errorf("%w: invalid category hierarchy", errCatalogIncomplete)
	}
	return watchCatalog{categories: categories, products: result}, nil
}

func resolveScope(categories []domain.Category, selected []uint) ([]domain.ScopeCategory, bool) {
	levels := categoryLevels(categories)
	byID := indexCategories(categories)
	scope := make([]domain.ScopeCategory, 0, len(selected))
	missing := false
	for _, id := range selected {
		if level := levels[id]; level == 0 || level > 3 {
			missing = true
			continue
		}
		scope = append(scope, domain.ScopeCategory{ConnectionID: byID[id].ConnectionID, CategoryID: id})
	}
	return scope, missing
}

func categoryLevels(categories []domain.Category) map[uint]int {
	children := make(map[uint][]uint)
	for _, category := range categories {
		children[category.ParentID] = append(children[category.ParentID], category.ID)
	}
	levels := make(map[uint]int, len(categories))
	queue := slices.Clone(children[0])
	for _, id := range queue {
		levels[id] = 1
	}
	for i := 0; i < len(queue); i++ {
		id := queue[i]
		for _, child := range children[id] {
			levels[child] = levels[id] + 1
			queue = append(queue, child)
		}
	}
	return levels
}

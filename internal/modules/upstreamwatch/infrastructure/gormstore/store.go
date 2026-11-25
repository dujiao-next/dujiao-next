package gormstore

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/dujiao-next/internal/modules/upstreamwatch/contract"
	"github.com/dujiao-next/internal/modules/upstreamwatch/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct{ db *gorm.DB }

func New(db *gorm.DB) *Store { return &Store{db: db} }

var _ contract.Store = (*Store)(nil)

func (s *Store) Scans(ctx context.Context, ids []uint) ([]domain.Scan, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []domain.Scan
	err := s.db.WithContext(ctx).Where("connection_id IN ?", ids).Find(&rows).Error
	return rows, err
}

func (s *Store) Claim(ctx context.Context, id uint, revision uint64, now, until time.Time) (*domain.Scan, error) {
	var result *domain.Scan
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		scan := domain.Scan{ConnectionID: id}
		claim := tx.Model(&domain.Scan{}).Where(`connection_id = ?
			AND scope_revision = ?
			AND (COALESCE(error_code, '') <> ? OR next_scan_at IS NOT NULL)
			AND (next_scan_at IS NULL OR next_scan_at <= ?)
			AND (lease_until IS NULL OR lease_until <= ?)`, id, revision, domain.ErrorScopeUnsupported, now, now).
			Updates(map[string]any{"attempt_at": now, "lease_until": until})
		if claim.Error != nil || claim.RowsAffected == 0 {
			return claim.Error
		}
		if err := tx.First(&scan, "connection_id = ?", id).Error; err != nil {
			return err
		}
		result = &scan
		return nil
	})
	return result, err
}

func (s *Store) Scope(ctx context.Context, connectionID uint) ([]domain.ScopeCategory, error) {
	return scopeCategories(s.db.WithContext(ctx), connectionID)
}

func scopeCategories(db *gorm.DB, connectionID uint) ([]domain.ScopeCategory, error) {
	var rows []domain.ScopeCategory
	err := db.Where("connection_id = ?", connectionID).Find(&rows).Error
	return rows, err
}

func (s *Store) SetScope(ctx context.Context, connectionID uint, selected []uint, categories []domain.Category, scope []domain.ScopeCategory) error {
	ids, err := json.Marshal(selected)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		scan := domain.Scan{ConnectionID: connectionID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&scan).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.Scan{}).Where("connection_id = ?", connectionID).Updates(map[string]any{
			"category_ids": string(ids), "scope_revision": gorm.Expr("scope_revision + 1"),
			"cursor": nil, "next_scan_at": nil, "lease_until": nil, "last_full": nil,
			"error_code": "", "failures": 0,
		}).Error; err != nil {
			return err
		}
		if err := saveCategories(tx, categories); err != nil {
			return err
		}
		return saveScope(tx, connectionID, scope)
	})
}

func saveScope(tx *gorm.DB, connectionID uint, scope []domain.ScopeCategory) error {
	current, err := scopeCategories(tx, connectionID)
	if err != nil {
		return err
	}
	allowed := make(map[uint]bool, len(scope))
	for _, category := range scope {
		allowed[category.CategoryID] = true
	}
	var removed []uint
	for _, category := range current {
		if !allowed[category.CategoryID] {
			removed = append(removed, category.CategoryID)
		}
	}
	for batch := range slices.Chunk(removed, 200) {
		if err := tx.Where("connection_id = ? AND category_id IN ?", connectionID, batch).Delete(&domain.ScopeCategory{}).Error; err != nil {
			return err
		}
	}
	if len(scope) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "connection_id"}, {Name: "category_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"initialized"}),
		Where:     clause.Where{Exprs: []clause.Expression{gorm.Expr("excluded.initialized AND NOT upstream_catalog_scope_categories.initialized")}},
	}).CreateInBatches(scope, 100).Error
}

func saveCategories(tx *gorm.DB, categories []domain.Category) error {
	if len(categories) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "connection_id"}, {Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"parent_id", "name", "sort_order"}),
		Where: clause.Where{Exprs: []clause.Expression{gorm.Expr(`
			upstream_catalog_categories.parent_id IS DISTINCT FROM excluded.parent_id OR
			CAST(upstream_catalog_categories.name AS TEXT) IS DISTINCT FROM CAST(excluded.name AS TEXT) OR
			upstream_catalog_categories.sort_order IS DISTINCT FROM excluded.sort_order`)}},
	}).CreateInBatches(categories, 100).Error
}

func (s *Store) Fail(ctx context.Context, scan domain.Scan, retryAt *time.Time, code string) error {
	return s.db.WithContext(ctx).Model(&domain.Scan{}).
		Where("connection_id = ? AND scope_revision = ? AND attempt_at = ? AND lease_until IS NOT NULL", scan.ConnectionID, scan.ScopeRevision, scan.AttemptAt).
		Updates(map[string]any{"lease_until": nil, "error_code": code, "next_scan_at": retryAt, "failures": scan.Failures + 1}).Error
}

func (s *Store) Complete(ctx context.Context, scan domain.Scan, products []domain.Product, categories []domain.Category, scope []domain.ScopeCategory, full bool) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&domain.Scan{}).
			Where("connection_id = ? AND scope_revision = ? AND attempt_at = ? AND lease_until > ?", scan.ConnectionID, scan.ScopeRevision, scan.AttemptAt, *scan.LastSuccess).
			Updates(map[string]any{
				"mode": scan.Mode, "last_success": scan.LastSuccess, "last_full": scan.LastFull,
				"cursor": scan.Cursor, "next_scan_at": scan.NextScanAt, "lease_until": nil,
				"error_code": scan.ErrorCode, "failures": 0, "directory_read": scan.DirectoryRead,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("upstream catalog scan lease expired")
		}
		if err := saveCategories(tx, categories); err != nil {
			return err
		}
		if err := saveScope(tx, scan.ConnectionID, scope); err != nil {
			return err
		}
		availableProducts := make([]domain.Product, 0, len(products))
		unavailable := make([]domain.UnavailableProduct, 0)
		for _, product := range products {
			if product.Active && !product.OutOfStock() {
				availableProducts = append(availableProducts, product)
			} else {
				unavailable = append(unavailable, domain.UnavailableProduct{
					ConnectionID: scan.ConnectionID, UpstreamProductID: product.UpstreamProductID,
					Delisted: !product.Active, OutOfStock: product.Active && product.OutOfStock(),
				})
			}
		}
		if full {
			if err := markMissing(tx, scan.ConnectionID, products); err != nil {
				return err
			}
		}
		if err := removeUnavailable(tx, scan.ConnectionID, unavailable); err != nil {
			return err
		}
		if len(availableProducts) > 0 {
			availableProducts, err := restoreAvailable(tx, scan.ConnectionID, availableProducts)
			if err != nil {
				return err
			}
			if len(availableProducts) == 0 {
				return nil
			}
			// Backfill historical categories and keep newer inbox details up to date.
			assignments := map[string]any{
				"category_id": gorm.Expr("excluded.category_id"),
				"present":     true,
			}
			for _, field := range []string{"title", "price", "currency", "stock"} {
				assignments[field] = gorm.Expr("CASE WHEN upstream_catalog_products.is_new THEN excluded." + field +
					" ELSE upstream_catalog_products." + field + " END")
			}
			upsert := clause.OnConflict{
				Columns:   []clause.Column{{Name: "connection_id"}, {Name: "upstream_product_id"}},
				DoUpdates: clause.Assignments(assignments),
				Where: clause.Where{Exprs: []clause.Expression{gorm.Expr(`
					upstream_catalog_products.category_id IS DISTINCT FROM excluded.category_id OR
					NOT upstream_catalog_products.present OR
					(upstream_catalog_products.is_new AND (
						CAST(upstream_catalog_products.title AS TEXT) IS DISTINCT FROM CAST(excluded.title AS TEXT) OR
						upstream_catalog_products.price IS DISTINCT FROM excluded.price OR
						upstream_catalog_products.currency IS DISTINCT FROM excluded.currency OR
						upstream_catalog_products.stock IS DISTINCT FROM excluded.stock
					))`)}},
			}
			if err := tx.Clauses(upsert).CreateInBatches(availableProducts, 60).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Absence from a scoped read can mean a category move. Keep the record for
// deduplication and reserve relisted status for an observed delisting.
func markMissing(tx *gorm.DB, connectionID uint, products []domain.Product) error {
	var tracked []uint
	if err := tx.Model(&domain.Product{}).Where("connection_id = ? AND present = ?", connectionID, true).
		Where(inWatchScope).
		Pluck("upstream_product_id", &tracked).Error; err != nil {
		return err
	}
	seen := make(map[uint]struct{}, len(products))
	for _, product := range products {
		seen[product.UpstreamProductID] = struct{}{}
	}
	missing := tracked[:0]
	for _, id := range tracked {
		if _, ok := seen[id]; !ok {
			missing = append(missing, id)
		}
	}
	for batch := range slices.Chunk(missing, 200) {
		if err := tx.Model(&domain.Product{}).Where("connection_id = ? AND upstream_product_id IN ?", connectionID, batch).
			Update("present", false).Error; err != nil {
			return err
		}
	}
	return nil
}

func removeUnavailable(tx *gorm.DB, connectionID uint, products []domain.UnavailableProduct) error {
	upsert := clause.OnConflict{
		Columns: []clause.Column{{Name: "connection_id"}, {Name: "upstream_product_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"delisted":     gorm.Expr("upstream_catalog_unavailable_products.delisted OR excluded.delisted"),
			"out_of_stock": gorm.Expr("upstream_catalog_unavailable_products.out_of_stock OR excluded.out_of_stock"),
		}),
		Where: clause.Where{Exprs: []clause.Expression{gorm.Expr(`
			(excluded.delisted AND NOT upstream_catalog_unavailable_products.delisted)
			OR (excluded.out_of_stock AND NOT upstream_catalog_unavailable_products.out_of_stock)
		`)}},
	}
	for batch := range slices.Chunk(products, 200) {
		ids := make([]uint, 0, len(batch))
		for _, product := range batch {
			ids = append(ids, product.UpstreamProductID)
		}
		if err := tx.Clauses(upsert).Create(&batch).Error; err != nil {
			return err
		}
		if err := tx.Where("connection_id = ? AND upstream_product_id IN ?", connectionID, ids).
			Delete(&domain.Product{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func restoreAvailable(tx *gorm.DB, connectionID uint, products []domain.Product) ([]domain.Product, error) {
	var unavailable []domain.UnavailableProduct
	if err := tx.Where("connection_id = ?", connectionID).Find(&unavailable).Error; err != nil {
		return nil, err
	}
	if len(unavailable) == 0 {
		return products, nil
	}
	byID := make(map[uint]domain.UnavailableProduct, len(unavailable))
	for _, item := range unavailable {
		byID[item.UpstreamProductID] = item
	}
	restored := make([]uint, 0)
	available := products[:0]
	for _, product := range products {
		if previous, ok := byID[product.UpstreamProductID]; ok {
			if previous.OutOfStock && product.Stock == nil {
				continue
			}
			if product.IsNew {
				product.Relisted = previous.Delisted
				product.Restocked = previous.OutOfStock
			}
			restored = append(restored, product.UpstreamProductID)
		}
		available = append(available, product)
	}
	for batch := range slices.Chunk(restored, 300) {
		if err := tx.Where("connection_id = ? AND upstream_product_id IN ?", connectionID, batch).
			Delete(&domain.UnavailableProduct{}).Error; err != nil {
			return nil, err
		}
	}
	return available, nil
}

func (s *Store) unmapped(ctx context.Context) *gorm.DB {
	return s.db.WithContext(ctx).Model(&domain.Product{}).
		Where("active = ? AND present = ?", true, true).
		Where("(stock IS NULL OR stock <> 0)").
		Where(`EXISTS (SELECT 1 FROM site_connections AS c WHERE c.id = upstream_catalog_products.connection_id AND c.deleted_at IS NULL)
			AND NOT EXISTS (SELECT 1 FROM product_mappings AS m
				WHERE m.connection_id = upstream_catalog_products.connection_id
					AND m.upstream_product_id = upstream_catalog_products.upstream_product_id AND m.deleted_at IS NULL)`)
}

func (s *Store) Categories(ctx context.Context, connectionID uint) ([]domain.Category, error) {
	rows := make([]domain.Category, 0)
	err := s.db.WithContext(ctx).Where("connection_id = ?", connectionID).Find(&rows).Error
	return rows, err
}

const inWatchScope = `EXISTS (SELECT 1 FROM upstream_catalog_scope_categories AS scope
	WHERE scope.connection_id = upstream_catalog_products.connection_id AND scope.category_id = upstream_catalog_products.category_id)`

func inInbox(query *gorm.DB) *gorm.DB {
	return query.Where("is_new = ?", true).Where(inWatchScope).
		Where(`EXISTS (SELECT 1 FROM site_connections AS c WHERE c.id = upstream_catalog_products.connection_id AND c.status = 'active')`)
}

func (s *Store) CategoryGroups(ctx context.Context, connectionID uint, historical bool) ([]domain.CategoryGroup, error) {
	rows := make([]domain.CategoryGroup, 0)
	query := s.unmapped(ctx).Where("connection_id = ?", connectionID)
	if historical {
		query = query.Where("is_new = ?", false)
	} else {
		query = inInbox(query)
	}
	err := query.
		Select("COALESCE(category_id, 0) AS id, COUNT(*) AS total").
		Group("COALESCE(category_id, 0)").Order("id").Scan(&rows).Error
	return rows, err
}

func (s *Store) List(ctx context.Context, filter contract.ListFilter) ([]domain.Product, int64, error) {
	query := s.unmapped(ctx)
	if filter.ConnectionID != 0 {
		query = query.Where("connection_id = ?", filter.ConnectionID)
	}
	switch {
	case filter.ProductID != 0:
		query = query.Where("upstream_product_id = ?", filter.ProductID)
	case filter.Historical:
		query = query.Where("is_new = ?", false)
	default:
		query = inInbox(query)
	}
	if filter.CategoryID != nil {
		if *filter.CategoryID == 0 {
			query = query.Where("(category_id = 0 OR category_id IS NULL)")
		} else {
			query = query.Where("category_id = ?", *filter.CategoryID)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]domain.Product, 0)
	err := query.Order("first_seen_at DESC, connection_id ASC, upstream_product_id ASC").
		Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Find(&rows).Error
	return rows, total, err
}

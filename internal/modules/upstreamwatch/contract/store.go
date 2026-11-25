package contract

import (
	"context"
	"time"

	"github.com/dujiao-next/internal/modules/upstreamwatch/domain"
)

type ListFilter struct {
	ConnectionID, ProductID uint
	CategoryID              *uint
	Historical              bool
	Page, PageSize          int
}

type Store interface {
	Scans(ctx context.Context, ids []uint) ([]domain.Scan, error)
	Claim(ctx context.Context, id uint, revision uint64, now, until time.Time) (*domain.Scan, error)
	Fail(ctx context.Context, scan domain.Scan, retryAt *time.Time, code string) error
	Complete(ctx context.Context, scan domain.Scan, products []domain.Product, categories []domain.Category, scope []domain.ScopeCategory, full bool) error
	Scope(ctx context.Context, connectionID uint) ([]domain.ScopeCategory, error)
	SetScope(ctx context.Context, connectionID uint, selected []uint, categories []domain.Category, scope []domain.ScopeCategory) error
	List(ctx context.Context, filter ListFilter) ([]domain.Product, int64, error)
	Categories(ctx context.Context, connectionID uint) ([]domain.Category, error)
	CategoryGroups(ctx context.Context, connectionID uint, historical bool) ([]domain.CategoryGroup, error)
}

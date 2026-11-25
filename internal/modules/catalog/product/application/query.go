package productapplication

import (
	"strconv"
	"strings"
	"time"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
)

// ProductRepository 是读取应用服务所需的最小商品持久化端口。
type ProductRepository interface {
	List(filter productcontract.ListFilter) ([]productdomain.Product, int64, error)
	GetBySlug(slug string, onlyActive bool) (*productdomain.Product, error)
	GetAdminByID(id string) (*productdomain.Product, error)
}

// CategoryRepository 是公开分类展开所需的最小分类端口。
type CategoryRepository interface {
	GetByID(id string) (*categorydomain.Category, error)
	List() ([]categorydomain.Category, error)
}

// HiddenProductRepository 是商品读取用例所需的最小分销可见性端口。
type HiddenProductRepository interface {
	ListHiddenProductIDs(resellerID uint) ([]uint, error)
}

// StockCounter 是商品库存聚合所需的最小卡密库存端口。
type StockCounter interface {
	CountStockByProductIDs(productIDs []uint) ([]cardsecretcontract.SKUStockCount, error)
}

// Options 描述 Product 读取应用服务的端口和领域错误。
type Options struct {
	Products   ProductRepository
	Categories CategoryRepository
	Stock      StockCounter
}

// Service 编排商品查询、租户可见性和库存聚合。
type Service struct {
	products   ProductRepository
	categories CategoryRepository
	stock      StockCounter
}

// NewService 创建 Product 读取应用服务。
func NewService(options Options) *Service {
	return &Service{
		products:   options.Products,
		categories: options.Categories,
		stock:      options.Stock,
	}
}

// ListPublic 获取公开商品列表
func (s *Service) ListPublic(categoryID, search string, page, pageSize int) ([]productdomain.Product, int64, error) {
	categoryIDs, err := expandPublicCategoryIDs(s.categories, categoryID)
	if err != nil {
		return nil, 0, err
	}

	filter := productcontract.ListFilter{
		Page:         page,
		PageSize:     pageSize,
		CategoryID:   categoryID,
		CategoryIDs:  categoryIDs,
		Search:       search,
		OnlyActive:   true,
		WithCategory: true,
	}
	return s.products.List(filter)
}

// ListPublicForTenant 获取当前租户上下文的公开商品列表。
func (s *Service) ListPublicForTenant(tenant reseller.TenantContext, resellerRepo HiddenProductRepository, categoryID, search string, page, pageSize int) ([]productdomain.Product, int64, error) {
	if !tenant.IsReseller() {
		return s.ListPublic(categoryID, search, page, pageSize)
	}
	if tenant.ResellerID == nil || resellerRepo == nil {
		return nil, 0, productcontract.ErrResellerProductNotListed
	}
	categoryIDs, err := expandPublicCategoryIDs(s.categories, categoryID)
	if err != nil {
		return nil, 0, err
	}
	hiddenIDs, err := resellerRepo.ListHiddenProductIDs(*tenant.ResellerID)
	if err != nil {
		return nil, 0, err
	}
	filter := productcontract.ListFilter{
		Page:              page,
		PageSize:          pageSize,
		CategoryID:        categoryID,
		CategoryIDs:       categoryIDs,
		Search:            search,
		OnlyActive:        true,
		WithCategory:      true,
		ExcludeProductIDs: hiddenIDs,
	}
	return s.products.List(filter)
}

// ListForUpstreamSync 上游同步专用：可选包含已下架商品，便于下游识别下架状态
// includeInactive=true 时返回所有未软删商品（含 is_active=false）
func (s *Service) ListForUpstreamSync(updatedAfter *time.Time, includeInactive bool, page, pageSize int, categoryID uint) ([]productdomain.Product, int64, error) {
	filter := productcontract.ListFilter{
		Page:         page,
		PageSize:     pageSize,
		OnlyActive:   !includeInactive,
		WithCategory: true,
		UpdatedAfter: updatedAfter,
	}
	if categoryID != 0 {
		filter.CategoryIDs = []uint{categoryID}
	}
	return s.products.List(filter)
}

// ListPublicExact 获取公开商品列表（精确匹配分类，不展开父分类）
func (s *Service) ListPublicExact(categoryID string, page, pageSize int) ([]productdomain.Product, int64, error) {
	filter := productcontract.ListFilter{
		Page:         page,
		PageSize:     pageSize,
		CategoryID:   categoryID,
		OnlyActive:   true,
		WithCategory: true,
	}
	return s.products.List(filter)
}

// GetPublicBySlug 获取公开商品详情
func (s *Service) GetPublicBySlug(slug string) (*productdomain.Product, error) {
	item, err := s.products.GetBySlug(slug, true)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, productcontract.ErrNotFound
	}
	return item, nil
}

// GetPublicBySlugForTenant 获取当前租户上下文的公开商品详情。
func (s *Service) GetPublicBySlugForTenant(tenant reseller.TenantContext, resellerRepo HiddenProductRepository, slug string) (*productdomain.Product, error) {
	item, err := s.GetPublicBySlug(slug)
	if err != nil {
		return nil, err
	}
	if !tenant.IsReseller() {
		return item, nil
	}
	if tenant.ResellerID == nil || resellerRepo == nil {
		return nil, productcontract.ErrNotFound
	}
	hiddenIDs, err := resellerRepo.ListHiddenProductIDs(*tenant.ResellerID)
	if err != nil {
		return nil, err
	}
	for _, id := range hiddenIDs {
		if id == item.ID {
			return nil, productcontract.ErrNotFound
		}
	}
	return item, nil
}

// ListAdmin 获取后台商品列表
// isActive 为三态上架筛选：nil=不限，true=仅已上架，false=仅已下架。
func (s *Service) ListAdmin(categoryID, search, fulfillmentType, stockStatus string, hasWholesalePrices, isActive *bool, lowStockThreshold int, page, pageSize int) ([]productdomain.Product, int64, error) {
	filter := productcontract.ListFilter{
		Page:               page,
		PageSize:           pageSize,
		CategoryID:         categoryID,
		Search:             search,
		FulfillmentType:    strings.TrimSpace(fulfillmentType),
		StockStatus:        normalizeStockStatus(stockStatus),
		HasWholesalePrices: hasWholesalePrices,
		IsActive:           isActive,
		LowStockThreshold:  lowStockThreshold,
		OnlyActive:         false,
		WithCategory:       true,
	}
	return s.products.List(filter)
}

// GetAdminByID 获取后台商品详情
func (s *Service) GetAdminByID(id string) (*productdomain.Product, error) {
	item, err := s.products.GetAdminByID(id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, productcontract.ErrNotFound
	}
	return item, nil
}

func normalizeStockStatus(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "all":
		return ""
	case "low", "normal", "unlimited":
		return value
	default:
		return ""
	}
}

func expandPublicCategoryIDs(categoryRepo CategoryRepository, categoryID string) ([]uint, error) {
	normalizedCategoryID := strings.TrimSpace(categoryID)
	if normalizedCategoryID == "" {
		return nil, nil
	}

	parsedCategoryID, err := strconv.ParseUint(normalizedCategoryID, 10, 64)
	if err != nil || parsedCategoryID == 0 {
		return nil, nil
	}
	if categoryRepo == nil {
		return []uint{uint(parsedCategoryID)}, nil
	}

	category, err := categoryRepo.GetByID(normalizedCategoryID)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return []uint{uint(parsedCategoryID)}, nil
	}
	if !category.IsActive {
		return []uint{}, nil
	}
	if category.ParentID > 0 {
		return []uint{category.ID}, nil
	}

	categories, err := categoryRepo.List()
	if err != nil {
		return nil, err
	}

	categoryIDs := []uint{category.ID}
	for _, item := range categories {
		if item.ParentID == category.ID && item.IsActive {
			categoryIDs = append(categoryIDs, item.ID)
		}
	}
	return categoryIDs, nil
}

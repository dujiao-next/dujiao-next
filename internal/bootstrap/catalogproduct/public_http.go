package catalogproductbootstrap

import (
	productapplication "github.com/dujiao-next/internal/modules/catalog/product/application"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	producthttp "github.com/dujiao-next/internal/modules/catalog/product/transport/http"
	promotionapp "github.com/dujiao-next/internal/modules/promotion/application"
	promotioncontract "github.com/dujiao-next/internal/modules/promotion/contract"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
)

// PublicHTTPDependencies 是公开 Product HTTP 入口的显式装配依赖。
type PublicHTTPDependencies struct {
	Products     *productapplication.Service
	Hidden       productapplication.HiddenProductRepository
	Pricer       producthttp.ResellerDisplayPricer
	Promotions   promotioncontract.Repository
	MemberLevels producthttp.MemberLevelPricing
	Mappings     producthttp.LocalProductMappingReader
	SKUMappings  producthttp.SKUMappingLookup
	RelatedPosts producthttp.RelatedPostReader
}

// publicProductAdapter 将 Product 查询服务和租户隐藏策略组合成公开查询端口。
type publicProductAdapter struct {
	products *productapplication.Service
	hidden   productapplication.HiddenProductRepository
}

func (adapter publicProductAdapter) ListPublicForTenant(
	tenant reseller.TenantContext,
	categoryID, search string,
	page, pageSize int,
) ([]productdomain.Product, int64, error) {
	return adapter.products.ListPublicForTenant(tenant, adapter.hidden, categoryID, search, page, pageSize)
}

func (adapter publicProductAdapter) GetPublicBySlugForTenant(
	tenant reseller.TenantContext,
	slug string,
) (*productdomain.Product, error) {
	return adapter.products.GetPublicBySlugForTenant(tenant, adapter.hidden, slug)
}

func (adapter publicProductAdapter) ApplyAutoStockCounts(products []productdomain.Product) error {
	return adapter.products.ApplyAutoStockCounts(products)
}

// NewPublicCompareHTTP 装配公开对比 Handler（复用同一套租户隐藏策略）。
func NewPublicCompareHTTP(
	products *productapplication.Service,
	hidden productapplication.HiddenProductRepository,
	store producthttp.CompareProductStore,
	groups producthttp.CompareGroupReader,
) *producthttp.PublicCompareHandler {
	return producthttp.NewPublicCompareHandler(
		publicProductAdapter{products: products, hidden: hidden},
		store,
		groups,
	)
}

// NewPublicHTTP 装配公开 Product HTTP Handler。
func NewPublicHTTP(dependencies PublicHTTPDependencies) *producthttp.PublicHandler {
	var promotions producthttp.ProductPromotionDecorator
	if dependencies.Promotions != nil {
		promotions = promotionapp.NewService(dependencies.Promotions)
	}
	return producthttp.NewPublicHandler(
		publicProductAdapter{products: dependencies.Products, hidden: dependencies.Hidden},
		dependencies.Pricer,
		promotions,
		dependencies.MemberLevels,
		dependencies.Mappings,
		dependencies.SKUMappings,
		dependencies.RelatedPosts,
	)
}

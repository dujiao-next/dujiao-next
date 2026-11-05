package producthttp

import (
	"context"
	"errors"
	"strings"

	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"

	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productpresenter "github.com/dujiao-next/internal/modules/catalog/product/transport/presenter"

	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const publicRelatedPostsLimit = 6

// PublicProductQueries 是公开商品列表/详情所需的最小用例接口。
type PublicProductQueries interface {
	ListPublicForTenant(tenant reseller.TenantContext, categoryID, search string, page, pageSize int) ([]productdomain.Product, int64, error)
	GetPublicBySlugForTenant(tenant reseller.TenantContext, slug string) (*productdomain.Product, error)
	ApplyAutoStockCounts(products []productdomain.Product) error
}

// ResellerDisplayPricer 是分销站展示价解析端口。
type ResellerDisplayPricer interface {
	LoadDisplayPricingBatch(tenant reseller.TenantContext, products []productdomain.Product) (*reseller.DisplayPricingBatch, error)
	ResolveDisplayPrices(tenant reseller.TenantContext, product *productdomain.Product, batch *reseller.DisplayPricingBatch) (*reseller.DisplayPriceResult, error)
}

// ProductPromotionDecorator 是公开商品促销装饰端口。
type ProductPromotionDecorator interface {
	GetProductPromotions(productID uint) ([]promotiondomain.Promotion, error)
	ApplyPromotion(product *productdomain.Product, quantity int) (*promotiondomain.Promotion, money.Amount, error)
}

// MemberLevelPricing 是公开商品会员价装饰端口。
type MemberLevelPricing interface {
	GetLevelPricesByProduct(productID uint) ([]memberleveldomain.MemberLevelPrice, error)
	ResolveMemberPrice(levelID, productID, skuID uint, basePrice decimal.Decimal) (decimal.Decimal, decimal.Decimal)
}

// LocalProductMappingReader 按本地商品 ID 读取上游映射。
type LocalProductMappingReader interface {
	GetByLocalProductID(productID uint) (*mappingdomain.Mapping, error)
}

// RelatedPostReader 是商品详情消费方所需的最小 Content 读取接口。
type RelatedPostReader interface {
	ListPostsForProduct(ctx context.Context, productID uint, limit int) ([]contentcontract.RelatedPost, error)
}

// PublicHandler 处理公开商品目录 HTTP 请求。
type PublicHandler struct {
	products     PublicProductQueries
	pricer       ResellerDisplayPricer
	promotions   ProductPromotionDecorator
	memberLevels MemberLevelPricing
	mappings     LocalProductMappingReader
	skuMappings  SKUMappingLookup
	relatedPosts RelatedPostReader
}

// NewPublicHandler 创建公开商品目录 Handler。
func NewPublicHandler(
	products PublicProductQueries,
	pricer ResellerDisplayPricer,
	promotions ProductPromotionDecorator,
	memberLevels MemberLevelPricing,
	mappings LocalProductMappingReader,
	skuMappings SKUMappingLookup,
	relatedPosts RelatedPostReader,
) *PublicHandler {
	if products == nil || relatedPosts == nil {
		panic("catalog public handler: required dependency is nil")
	}
	return &PublicHandler{
		products:     products,
		pricer:       pricer,
		promotions:   promotions,
		memberLevels: memberLevels,
		mappings:     mappings,
		skuMappings:  skuMappings,
		relatedPosts: relatedPosts,
	}
}

func tenantFromRequest(c *gin.Context) reseller.TenantContext {
	return reseller.TenantOrMain(c.Request.Context())
}

func isResellerTenant(tenant reseller.TenantContext) bool {
	return tenant.ResellerID != nil && !tenant.IsMain && !tenant.Unavailable
}

// GetProducts 获取商品列表
func (h *PublicHandler) GetProducts(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	categoryID := c.Query("category_id")
	search := strings.TrimSpace(c.Query("search"))
	tenant := tenantFromRequest(c)

	products, total, err := h.products.ListPublicForTenant(tenant, categoryID, search, page, pageSize)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	if err := h.products.ApplyAutoStockCounts(products); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	var resellerBatch *reseller.DisplayPricingBatch
	if isResellerTenant(tenant) {
		if h.pricer == nil {
			ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		resellerBatch, err = h.pricer.LoadDisplayPricingBatch(tenant, products)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
			return
		}
	}

	decorated := make([]productpresenter.Product, 0, len(products))
	for i := range products {
		item, derr := h.decoratePublicProductForTenant(&products[i], h.promotions, tenant, resellerBatch)
		if derr != nil {
			if errors.Is(derr, productcontract.ErrResellerProductNotListed) {
				continue
			}
			ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", derr)
			return
		}
		decorated = append(decorated, item)
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, decorated, pagination)
}

// GetProductBySlug 根据 slug 获取商品详情
func (h *PublicHandler) GetProductBySlug(c *gin.Context) {
	slug := c.Param("slug")
	tenant := tenantFromRequest(c)

	product, err := h.products.GetPublicBySlugForTenant(tenant, slug)
	if err != nil {
		if errors.Is(err, productcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	temp := []productdomain.Product{*product}
	if err := h.products.ApplyAutoStockCounts(temp); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}
	*product = temp[0]

	var resellerBatch *reseller.DisplayPricingBatch
	if isResellerTenant(tenant) {
		if h.pricer == nil {
			ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		resellerBatch, err = h.pricer.LoadDisplayPricingBatch(tenant, []productdomain.Product{*product})
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
			return
		}
	}

	decorated, derr := h.decoratePublicProductForTenant(product, h.promotions, tenant, resellerBatch)
	if derr != nil {
		if errors.Is(derr, productcontract.ErrResellerProductNotListed) {
			ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", derr)
		return
	}

	if posts, relatedErr := h.loadRelatedPostCards(c.Request.Context(), product.ID); relatedErr == nil {
		decorated.RelatedPosts = posts
	}

	response.Success(c, decorated)
}

func (h *PublicHandler) loadRelatedPostCards(ctx context.Context, productID uint) ([]productpresenter.RelatedPost, error) {
	posts, err := h.relatedPosts.ListPostsForProduct(ctx, productID, publicRelatedPostsLimit)
	if err != nil {
		return nil, err
	}
	result := make([]productpresenter.RelatedPost, 0, len(posts))
	for i := range posts {
		post := &posts[i]
		result = append(result, productpresenter.RelatedPost{
			ID:          post.ID,
			Slug:        post.Slug,
			Type:        post.Type,
			Title:       post.Title,
			Summary:     post.Summary,
			Thumbnail:   post.Thumbnail,
			PublishedAt: post.PublishedAt,
		})
	}
	return result, nil
}

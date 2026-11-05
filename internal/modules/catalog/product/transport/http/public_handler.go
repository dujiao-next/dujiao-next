package producthttp

import (
	"encoding/json"
	"sort"
	"strconv"

	"context"
	"errors"
	"github.com/dujiao-next/internal/shared/jsonmap"
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

// CompareProductReader 是对比接口所需的商品读取能力。
type CompareProductReader interface {
	GetPublicBySlugForTenant(tenant reseller.TenantContext, slug string) (*productdomain.Product, error)
	ListPublicForTenant(tenant reseller.TenantContext, categoryID, search string, page, pageSize int) ([]productdomain.Product, int64, error)
}

// CompareProductStore 按 ID 批量取商品（手动指定对比对象时用）。
type CompareProductStore interface {
	ListByIDs(ids []uint) ([]productdomain.Product, error)
}

// CompareGroupReader 读取维度组（公开侧只需要按 ID / 分类取）。
type CompareGroupReader interface {
	GetByID(id uint) (*productdomain.CompareGroup, error)
	GetByCategory(categoryID uint) (*productdomain.CompareGroup, error)
}

// PublicCompareHandler 输出「哪款 xx 适合你」所需的对比数据。
type PublicCompareHandler struct {
	products CompareProductReader
	store    CompareProductStore
	groups   CompareGroupReader
}

// NewPublicCompareHandler 创建对比 Handler。
func NewPublicCompareHandler(products CompareProductReader, store CompareProductStore, groups CompareGroupReader) *PublicCompareHandler {
	if products == nil || store == nil || groups == nil {
		panic("catalog public compare handler: dependencies are nil")
	}
	return &PublicCompareHandler{products: products, store: store, groups: groups}
}

// CompareDimension 是返回给前台的维度定义。
type CompareDimension struct {
	Key   string       `json:"key"`
	Label jsonmap.JSON `json:"label"`
	Icon  string       `json:"icon,omitempty"`
	Unit  string       `json:"unit,omitempty"`
	Type  string       `json:"type,omitempty"`
	Group string       `json:"group,omitempty"`
	Order int          `json:"order"`
}

// CompareItem 是对比表里的一列。
type CompareItem struct {
	ID          uint                   `json:"id"`
	Slug        string                 `json:"slug"`
	Title       jsonmap.JSON           `json:"title"`
	Image       string                 `json:"image,omitempty"`
	Subtitle    jsonmap.JSON           `json:"subtitle,omitempty"`
	Tagline     jsonmap.JSON           `json:"tagline,omitempty"`
	Badge       jsonmap.JSON           `json:"badge,omitempty"`
	PriceAmount money.Amount           `json:"price_amount"`
	IsCurrent   bool                   `json:"is_current"`
	Values      map[string]interface{} `json:"values"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// ComparePayload 是对比接口响应。
type ComparePayload struct {
	GroupID    uint               `json:"group_id"`
	GroupName  jsonmap.JSON       `json:"group_name,omitempty"`
	Dimensions []CompareDimension `json:"dimensions"`
	Items      []CompareItem      `json:"items"`
	ShowPrice  bool               `json:"show_price"`
	ShowCTA    bool               `json:"show_cta"`
}

// GetProductCompare 返回某商品的对比数据（当前商品 + 对比商品）。
func (h *PublicCompareHandler) GetProductCompare(c *gin.Context) {
	slug := c.Param("slug")
	tenant := reseller.TenantContext{}
	product, err := h.products.GetPublicBySlugForTenant(tenant, slug)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}
	if product == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
		return
	}

	cfg := decodeCompareConfig(product.CompareJSON)
	group := h.resolveGroup(cfg, product.CategoryID)
	if group == nil {
		response.Success(c, nil)
		return
	}

	dimensions := decodeDimensions(group.Dimensions)
	if len(dimensions) == 0 {
		response.Success(c, nil)
		return
	}

	others := h.resolveOthers(cfg, product, group)
	items := make([]CompareItem, 0, len(others)+1)
	items = append(items, buildCompareItem(product, true, cfg))
	for _, other := range others {
		othersPtr := other
		items = append(items, buildCompareItem(&othersPtr, false, decodeCompareConfig(other.CompareJSON)))
	}

	response.Success(c, ComparePayload{
		GroupID:    group.ID,
		GroupName:  group.NameJSON,
		Dimensions: dimensions,
		Items:      items,
		ShowPrice:  cfg["show_price"] != false,
		ShowCTA:    cfg["show_cta"] != false,
	})
}

// GetCompareByIDs 是独立对比页用的接口：?ids=1,2,3
func (h *PublicCompareHandler) GetCompareByIDs(c *gin.Context) {
	raw := strings.Split(c.Query("ids"), ",")
	ids := make([]uint, 0, len(raw))
	for _, item := range raw {
		text := strings.TrimSpace(item)
		if text == "" {
			continue
		}
		value, err := strconv.ParseUint(text, 10, 64)
		if err != nil || value == 0 {
			continue
		}
		ids = append(ids, uint(value))
	}
	if len(ids) < 2 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.compare_ids_required", nil)
		return
	}
	if len(ids) > 4 {
		ids = ids[:4]
	}
	products, err := h.store.ListByIDs(ids)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}
	byID := make(map[uint]productdomain.Product, len(products))
	for _, item := range products {
		byID[item.ID] = item
	}
	ordered := make([]productdomain.Product, 0, len(ids))
	for _, id := range ids {
		if item, ok := byID[id]; ok {
			ordered = append(ordered, item)
		}
	}
	if len(ordered) < 2 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.compare_ids_required", nil)
		return
	}

	var group *productdomain.CompareGroup
	for _, item := range ordered {
		if found := h.resolveGroup(decodeCompareConfig(item.CompareJSON), item.CategoryID); found != nil {
			group = found
			break
		}
	}
	if group == nil {
		response.Success(c, nil)
		return
	}
	dimensions := decodeDimensions(group.Dimensions)
	if len(dimensions) == 0 {
		response.Success(c, nil)
		return
	}

	items := make([]CompareItem, 0, len(ordered))
	for index := range ordered {
		product := ordered[index]
		items = append(items, buildCompareItem(&product, index == 0, decodeCompareConfig(product.CompareJSON)))
	}
	response.Success(c, ComparePayload{
		GroupID:    group.ID,
		GroupName:  group.NameJSON,
		Dimensions: dimensions,
		Items:      items,
		ShowPrice:  true,
		ShowCTA:    true,
	})
}

/* ---------- 内部 ---------- */

func decodeCompareConfig(raw jsonmap.JSON) map[string]interface{} {
	if raw == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}(raw)
}

func (h *PublicCompareHandler) resolveGroup(cfg map[string]interface{}, categoryID uint) *productdomain.CompareGroup {
	if id := toUint(cfg["group_id"]); id > 0 {
		if group, err := h.groups.GetByID(id); err == nil && group != nil && group.IsActive {
			return group
		}
	}
	if categoryID > 0 {
		if group, err := h.groups.GetByCategory(categoryID); err == nil && group != nil {
			return group
		}
	}
	return nil
}

func (h *PublicCompareHandler) resolveOthers(cfg map[string]interface{}, current *productdomain.Product, group *productdomain.CompareGroup) []productdomain.Product {
	limit := toInt(cfg["limit"])
	if limit <= 0 || limit > 4 {
		limit = 2
	}
	mode := stringOf(cfg["mode"])

	if mode == "manual" {
		ids := toUintSlice(cfg["compare_with"])
		if len(ids) > 0 {
			products, err := h.store.ListByIDs(ids)
			if err == nil {
				byID := make(map[uint]productdomain.Product, len(products))
				for _, item := range products {
					byID[item.ID] = item
				}
				out := make([]productdomain.Product, 0, len(ids))
				for _, id := range ids {
					if id == current.ID {
						continue
					}
					if item, ok := byID[id]; ok && item.IsActive {
						out = append(out, item)
					}
					if len(out) >= limit {
						break
					}
				}
				if len(out) > 0 {
					return out
				}
			}
		}
	}

	// 自动模式：同分类上架商品
	tenant := reseller.TenantContext{}
	list, _, err := h.products.ListPublicForTenant(tenant, uintToStr(current.CategoryID), "", 1, limit+1)
	if err != nil {
		return nil
	}
	out := make([]productdomain.Product, 0, limit)
	for _, item := range list {
		if item.ID == current.ID {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func decodeDimensions(raw jsonmap.JSON) []CompareDimension {
	items, ok := raw["items"].([]interface{})
	if !ok {
		// jsonmap 反序列化后可能是 []map 结构，尝试再走一次 JSON
		if encoded, err := json.Marshal(raw["items"]); err == nil {
			_ = json.Unmarshal(encoded, &items)
		}
	}
	out := make([]CompareDimension, 0, len(items))
	for _, entry := range items {
		record, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		if visible, ok := record["visible"].(bool); ok && !visible {
			continue
		}
		label, _ := record["label"].(map[string]interface{})
		out = append(out, CompareDimension{
			Key:   stringOf(record["key"]),
			Label: jsonmap.JSON(label),
			Icon:  stringOf(record["icon"]),
			Unit:  stringOf(record["unit"]),
			Type:  stringOf(record["type"]),
			Group: stringOf(record["group"]),
			Order: toInt(record["order"]),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

func buildCompareItem(product *productdomain.Product, isCurrent bool, cfg map[string]interface{}) CompareItem {
	image := ""
	if len(product.Images) > 0 {
		image = product.Images[0]
	}
	if override := stringOf(cfg["image"]); override != "" {
		image = override
	}
	values := map[string]interface{}{}
	if raw, ok := cfg["values"].(map[string]interface{}); ok {
		values = raw
	}
	subtitle := jsonmap.JSON{}
	if raw, ok := cfg["subtitle"].(map[string]interface{}); ok {
		subtitle = jsonmap.JSON(raw)
	}
	tagline := jsonmap.JSON{}
	if raw, ok := cfg["tagline"].(map[string]interface{}); ok {
		tagline = jsonmap.JSON(raw)
	}
	badge := jsonmap.JSON{}
	if raw, ok := cfg["badge"].(map[string]interface{}); ok {
		badge = jsonmap.JSON(raw)
	}
	details := map[string]interface{}{}
	if raw, ok := cfg["details"].(map[string]interface{}); ok {
		details = raw
	}
	return CompareItem{
		ID:          product.ID,
		Slug:        product.Slug,
		Title:       product.TitleJSON,
		Image:       image,
		Subtitle:    subtitle,
		Tagline:     tagline,
		Badge:       badge,
		PriceAmount: product.PriceAmount,
		IsCurrent:   isCurrent,
		Values:      values,
		Details:     details,
	}
}

func stringOf(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func toInt(value interface{}) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		if n, err := typed.Int64(); err == nil {
			return int(n)
		}
	}
	return 0
}

func toUint(value interface{}) uint {
	switch typed := value.(type) {
	case float64:
		return uint(typed)
	case int:
		return uint(typed)
	case json.Number:
		if n, err := typed.Int64(); err == nil {
			return uint(n)
		}
	}
	return 0
}

func toUintSlice(value interface{}) []uint {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	out := make([]uint, 0, len(items))
	for _, item := range items {
		if id := toUint(item); id > 0 {
			out = append(out, id)
		}
	}
	return out
}

func uintToStr(value uint) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(value), 10)
}

var _ = productcontract.ErrNotFound

package upstreamhttp

import (
	"errors"
	"net/http"
	"time"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	domaincatalog "github.com/dujiao-next/internal/modules/catalog"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// upstreamCategory 上游分类响应格式
type upstreamCategory struct {
	ID        uint         `json:"id"`
	ParentID  uint         `json:"parent_id"`
	Slug      string       `json:"slug"`
	Name      jsonmap.JSON `json:"name"`
	Icon      string       `json:"icon"`
	SortOrder int          `json:"sort_order"`
}

// upstreamProduct 上游商品响应格式
type upstreamProduct struct {
	ID               uint                              `json:"id"`
	Slug             string                            `json:"slug"`
	SeoMeta          jsonmap.JSON                      `json:"seo_meta"`
	Title            jsonmap.JSON                      `json:"title"`
	Description      jsonmap.JSON                      `json:"description"`
	Content          jsonmap.JSON                      `json:"content"`
	Images           jsonslice.Strings                 `json:"images"`
	Tags             jsonslice.Strings                 `json:"tags"`
	PriceAmount      string                            `json:"price_amount"`
	OriginalPrice    string                            `json:"original_price,omitempty"`
	MemberPrice      string                            `json:"member_price,omitempty"`
	AgencyPrice      string                            `json:"agency_price,omitempty"`
	WholesalePrices  productdomain.WholesalePriceTiers `json:"wholesale_prices,omitempty"`
	FulfillmentType  string                            `json:"fulfillment_type"`
	ManualFormSchema jsonmap.JSON                      `json:"manual_form_schema"`
	IsActive         bool                              `json:"is_active"`
	CategoryID       uint                              `json:"category_id"`
	SKUs             []upstreamSKU                     `json:"skus"`
	CreatedAt        time.Time                         `json:"created_at"`
	UpdatedAt        time.Time                         `json:"updated_at"`
}

type upstreamSKU struct {
	ID            uint         `json:"id"`
	SKUCode       string       `json:"sku_code"`
	SpecValues    jsonmap.JSON `json:"spec_values"`
	PriceAmount   string       `json:"price_amount"`
	OriginalPrice string       `json:"original_price,omitempty"`
	MemberPrice   string       `json:"member_price,omitempty"`
	AgencyPrice   string       `json:"agency_price,omitempty"`
	StockStatus   string       `json:"stock_status"`
	StockQuantity int          `json:"stock_quantity"`
	IsActive      bool         `json:"is_active"`
}

// ListCategories GET /api/v1/upstream/categories
func (h *Handler) ListCategories(c *gin.Context) {
	categories, err := h.Categories.List()
	if err != nil {
		logger.Errorw("upstream_list_categories_failed", "error", err)
		errorResponse(c, http.StatusInternalServerError, "internal_error", "failed to list categories")
		return
	}

	items := make([]upstreamCategory, 0, len(categories))
	for _, cat := range categories {
		items = append(items, upstreamCategory{
			ID:        cat.ID,
			ParentID:  cat.ParentID,
			Slug:      cat.Slug,
			Name:      cat.NameJSON,
			Icon:      cat.Icon,
			SortOrder: cat.SortOrder,
		})
	}

	successResponse(c, gin.H{
		"ok":         true,
		"categories": items,
	})
}

// ListProducts GET /api/v1/upstream/products
func (h *Handler) ListProducts(c *gin.Context) {
	page, pageSize := ginutil.ParsePaginationWithBounds(c, "page", "page_size", 50, 50)

	// 是否包含下架商品：下游同步任务用此参数识别上游下架/删除状态
	includeInactive := c.Query("include_inactive") == "true"

	// 解析增量同步时间
	var updatedAfter *time.Time
	if updatedAfterStr := c.Query("updated_after"); updatedAfterStr != "" {
		if t, parseErr := time.Parse(time.RFC3339, updatedAfterStr); parseErr == nil {
			updatedAfter = &t
		}
	}

	products, total, err := h.Products.ListForUpstreamSync(updatedAfter, includeInactive, page, pageSize)
	if err != nil {
		logger.Errorw("upstream_list_products_failed", "error", err)
		errorResponse(c, http.StatusInternalServerError, "internal_error", "failed to list products")
		return
	}

	// 补充自动发货库存计数
	if err := h.Products.ApplyAutoStockCounts(products); err != nil {
		logger.Warnw("upstream_apply_stock_counts_failed", "error", err)
	}

	// 补充上游对接商品的 SKU 库存
	h.applyUpstreamStockToProducts(products)

	// 获取下游用户的会员等级
	userID := getUpstreamUserID(c)
	var memberLevelID uint
	if userID > 0 {
		user, err := h.Users.GetByID(userID)
		if err == nil && user != nil {
			memberLevelID = user.MemberLevelID
		}
	}

	// 批量解析映射商品的真实交付类型
	fulfillmentTypeMap := h.resolveEffectiveFulfillmentTypes(products)

	items := make([]upstreamProduct, 0, len(products))
	for _, p := range products {
		items = append(items, h.toUpstreamProductWithMemberPrice(p, memberLevelID, fulfillmentTypeMap))
	}

	successResponse(c, gin.H{
		"ok":                true,
		"items":             items,
		"total":             total,
		"page":              page,
		"page_size":         pageSize,
		"includes_inactive": includeInactive, // 回声告诉下游本次响应是否包含下架商品
	})
}

// GetProduct GET /api/v1/upstream/products/:id
func (h *Handler) GetProduct(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		errorResponse(c, http.StatusBadRequest, "bad_request", "product id is required")
		return
	}

	product, err := h.Products.GetAdminByID(id)
	if err != nil {
		// 商品被软删除（数据库不存在）→ 保留 product_not_found 向后兼容旧版下游 adapter
		if errors.Is(err, ErrProductNotFound) {
			errorResponse(c, http.StatusNotFound, "product_not_found", "product not found")
			return
		}
		logger.Errorw("upstream_get_product_failed", "id", id, "error", err)
		errorResponse(c, http.StatusInternalServerError, "internal_error", "failed to get product")
		return
	}

	// 商品下架（IsActive=false）：仍返回 200，由下游根据 is_active 字段判断
	// 这样下游可在不下单的前提下感知下架状态、自动同步本地下架

	// 补充自动发货库存计数
	products := []productdomain.Product{*product}
	if err := h.Products.ApplyAutoStockCounts(products); err != nil {
		logger.Warnw("upstream_apply_stock_counts_failed", "error", err)
	}

	// 补充上游对接商品的 SKU 库存
	h.applyUpstreamStockToProducts(products)

	// 获取下游用户的会员等级
	userID := getUpstreamUserID(c)
	var memberLevelID uint
	if userID > 0 {
		user, err := h.Users.GetByID(userID)
		if err == nil && user != nil {
			memberLevelID = user.MemberLevelID
		}
	}

	// 解析映射商品的真实交付类型
	fulfillmentTypeMap := h.resolveEffectiveFulfillmentTypes(products)

	successResponse(c, gin.H{
		"ok":      true,
		"product": h.toUpstreamProductWithMemberPrice(products[0], memberLevelID, fulfillmentTypeMap),
	})
}

// applyUpstreamStockToProducts 为 upstream 类型商品的 SKU 填充上游库存数据
// 从 SKU 映射中读取 UpstreamStock，写入 ProductSKU 的虚拟字段，供 computeSKUStock 使用
func (h *Handler) applyUpstreamStockToProducts(products []productdomain.Product) {
	for i := range products {
		p := &products[i]
		if p.FulfillmentType != constants.FulfillmentTypeUpstream {
			continue
		}
		for j := range p.SKUs {
			sku := &p.SKUs[j]
			mapping, err := h.SKUMappings.GetByLocalSKUID(sku.ID)
			if err != nil || mapping == nil {
				sku.UpstreamStock = 0
				continue
			}
			sku.UpstreamStock = mapping.UpstreamStock
		}
	}
}

// resolveEffectiveFulfillmentTypes 批量解析映射商品的真实交付类型
// 对于映射商品（FulfillmentType="upstream"），返回 ProductMapping 中保存的原始交付类型
func (h *Handler) resolveEffectiveFulfillmentTypes(products []productdomain.Product) map[uint]string {
	result := make(map[uint]string)
	var mappedIDs []uint
	for _, p := range products {
		if p.IsMapped && p.FulfillmentType == constants.FulfillmentTypeUpstream {
			mappedIDs = append(mappedIDs, p.ID)
		}
	}
	if len(mappedIDs) == 0 {
		return result
	}
	mappings, err := h.ProductMappings.ListByLocalProductIDs(mappedIDs)
	if err != nil {
		logger.Warnw("resolve_effective_fulfillment_types_failed", "error", err)
		return result
	}
	for _, m := range mappings {
		ft := m.UpstreamFulfillmentType
		if ft != constants.FulfillmentTypeAuto {
			ft = constants.FulfillmentTypeManual
		}
		result[m.LocalProductID] = ft
	}
	return result
}

func (h *Handler) toUpstreamProductWithMemberPrice(p productdomain.Product, memberLevelID uint, fulfillmentTypeMap map[uint]string) upstreamProduct {
	skus := make([]upstreamSKU, 0, len(p.SKUs))
	for _, s := range p.SKUs {
		if !s.IsActive {
			continue
		}
		stockStatus, stockQuantity := computeSKUStock(p, s)
		si := upstreamSKU{
			ID:            s.ID,
			SKUCode:       s.SKUCode,
			SpecValues:    s.SpecValuesJSON,
			PriceAmount:   s.PriceAmount.StringFixed(2),
			StockStatus:   stockStatus,
			StockQuantity: stockQuantity,
			IsActive:      s.IsActive,
		}
		if s.AgencyPriceAmount.Decimal.GreaterThan(decimal.Zero) {
			si.AgencyPrice = s.AgencyPriceAmount.StringFixed(2)
			si.OriginalPrice = s.PriceAmount.StringFixed(2)
			si.PriceAmount = s.AgencyPriceAmount.StringFixed(2) // API 对接成本价格使用代理价
		} else if memberLevelID > 0 && h.MemberLevels != nil {
			mp, _ := h.MemberLevels.ResolveMemberPrice(memberLevelID, p.ID, s.ID, s.PriceAmount.Decimal)
			if mp.LessThan(s.PriceAmount.Decimal) {
				si.OriginalPrice = si.PriceAmount
				si.MemberPrice = money.FromDecimal(mp).StringFixed(2)
				si.PriceAmount = si.MemberPrice // price_amount 是实际售价（会员价）
			}
		}
		skus = append(skus, si)
	}

	// 映射商品返回原始交付类型，非映射商品返回自身交付类型
	effectiveFulfillmentType := p.FulfillmentType
	if ft, ok := fulfillmentTypeMap[p.ID]; ok {
		effectiveFulfillmentType = ft
	}

	result := upstreamProduct{
		ID:               p.ID,
		Slug:             p.Slug,
		SeoMeta:          p.SeoMetaJSON,
		Title:            p.TitleJSON,
		Description:      p.DescriptionJSON,
		Content:          p.ContentJSON,
		Images:           p.Images,
		Tags:             p.Tags,
		PriceAmount:      p.PriceAmount.StringFixed(2),
		WholesalePrices:  p.WholesalePrices,
		FulfillmentType:  effectiveFulfillmentType,
		ManualFormSchema: p.ManualFormSchemaJSON,
		IsActive:         p.IsActive,
		CategoryID:       p.CategoryID,
		SKUs:             skus,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}

	if p.AgencyPriceAmount.Decimal.GreaterThan(decimal.Zero) {
		result.AgencyPrice = p.AgencyPriceAmount.StringFixed(2)
		result.OriginalPrice = p.PriceAmount.StringFixed(2)
		result.PriceAmount = p.AgencyPriceAmount.StringFixed(2)
	} else if memberLevelID > 0 && h.MemberLevels != nil {
		mp, _ := h.MemberLevels.ResolveMemberPrice(memberLevelID, p.ID, 0, p.PriceAmount.Decimal)
		if mp.LessThan(p.PriceAmount.Decimal) {
			result.OriginalPrice = result.PriceAmount
			result.MemberPrice = money.FromDecimal(mp).StringFixed(2)
			result.PriceAmount = result.MemberPrice
		}
	}

	return result
}

// computeSKUStock 计算 SKU 的库存状态和实际可用量
func computeSKUStock(p productdomain.Product, s productdomain.ProductSKU) (status string, quantity int) {
	var available int64
	switch p.FulfillmentType {
	case constants.FulfillmentTypeManual:
		// ManualStockTotal 已是扣除预占后的剩余库存，不能再次减 ManualStockLocked。
		available = int64(s.ManualStockTotal)
	case constants.FulfillmentTypeUpstream:
		available = int64(s.UpstreamStock)
	default:
		available = s.AutoStockAvailable
	}
	return domaincatalog.UpstreamStockPolicy().Status(available), int(available)
}

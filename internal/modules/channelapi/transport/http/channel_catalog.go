package channelhttp

import (
	"fmt"
	"math"
	"strings"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	domaincatalog "github.com/dujiao-next/internal/modules/catalog"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// GetCategories GET /api/v1/channel/catalog/categories?locale=zh-CN
func (h *Handler) GetCategories(c *gin.Context) {
	locale := c.DefaultQuery("locale", "zh-CN")
	defaultLocale := "zh-CN"

	categories, err := h.CategoryService.ListActive()
	if err != nil {
		logger.Errorw("channel_catalog_list_categories", "error", err)
		respondChannelError(c, 500, 500, "internal_error", "error.internal_error", err)
		return
	}

	type categoryItem struct {
		ID           uint   `json:"id"`
		ParentID     uint   `json:"parent_id"`
		Name         string `json:"name"`
		Icon         string `json:"icon"`
		Slug         string `json:"slug"`
		ProductCount int64  `json:"product_count"`
	}

	// 统计每个分类的直接商品数
	directCounts := make(map[uint]int64, len(categories))
	// 记录哪些 parentID 有子分类
	hasChildren := make(map[uint]struct{})
	for _, cat := range categories {
		count, err := h.CategoryRepo.CountActiveProducts(fmt.Sprintf("%d", cat.ID))
		if err != nil {
			logger.Warnw("channel_catalog_count_products", "category_id", cat.ID, "error", err)
			count = 0
		}
		directCounts[cat.ID] = count
		if cat.ParentID != 0 {
			hasChildren[cat.ParentID] = struct{}{}
		}
	}

	// 构建可见分类列表：
	// - 一级分类：有直接商品 或 有子分类 → 可见
	// - 二级分类：只要父分类可见就一律返回（由客户端决定是否显示空分类）
	visibleParentIDs := make(map[uint]struct{})
	for _, cat := range categories {
		if cat.ParentID == 0 {
			count := directCounts[cat.ID]
			_, hasChild := hasChildren[cat.ID]
			if count > 0 || hasChild {
				visibleParentIDs[cat.ID] = struct{}{}
			}
		}
	}

	var items []categoryItem
	for _, cat := range categories {
		if cat.ParentID == 0 {
			if _, ok := visibleParentIDs[cat.ID]; !ok {
				continue
			}
		} else {
			if _, ok := visibleParentIDs[cat.ParentID]; !ok {
				continue
			}
		}
		items = append(items, categoryItem{
			ID:           cat.ID,
			ParentID:     cat.ParentID,
			Name:         resolveLocalizedJSON(cat.NameJSON, locale, defaultLocale),
			Icon:         cat.Icon,
			Slug:         cat.Slug,
			ProductCount: directCounts[cat.ID],
		})
	}

	respondChannelSuccess(c, gin.H{"items": items})
}

type channelWholesalePrice struct {
	SKUID       uint   `json:"sku_id,omitempty"`
	SKUCode     string `json:"sku_code,omitempty"`
	MinQuantity int    `json:"min_quantity"`
	UnitPrice   string `json:"unit_price"`
}

// resolveChannelMemberPrice 解析渠道目录展示的会员价，口径与下单一致：用户专属价优先，其次会员等级价。
func (h *Handler) resolveChannelMemberPrice(userID, memberLevelID, productID, skuID uint, base decimal.Decimal) (decimal.Decimal, bool) {
	if userID > 0 {
		if price, hit := h.MemberLevelService.ResolveUserPrice(userID, productID, skuID, base); hit {
			return price, true
		}
	}
	if memberLevelID > 0 {
		if price, _ := h.MemberLevelService.ResolveMemberPrice(memberLevelID, productID, skuID, base); price.LessThan(base) {
			return price, true
		}
	}
	return base, false
}

func normalizeChannelWholesalePrices(tiers productdomain.WholesalePriceTiers) []channelWholesalePrice {
	if len(tiers) == 0 {
		return nil
	}
	items := make([]channelWholesalePrice, 0, len(tiers))
	for _, tier := range tiers {
		if tier.MinQuantity <= 0 || tier.UnitPrice.Decimal.LessThanOrEqual(decimal.Zero) {
			continue
		}
		items = append(items, channelWholesalePrice{
			SKUID:       tier.SKUID,
			SKUCode:     tier.SKUCode,
			MinQuantity: tier.MinQuantity,
			UnitPrice:   tier.UnitPrice.String(),
		})
	}
	return items
}

// GetProducts GET /api/v1/channel/catalog/products?locale=zh-CN&category_id=1&page=1&page_size=5
func (h *Handler) GetProducts(c *gin.Context) {
	locale := c.DefaultQuery("locale", "zh-CN")
	defaultLocale := "zh-CN"
	categoryID := c.DefaultQuery("category_id", "")
	page, pageSize := ginutil.ParsePaginationWithBounds(c, "page", "page_size", 5, 20)
	exact := c.DefaultQuery("exact", "") == "1"

	var products []productdomain.Product
	var total int64
	var err error
	if exact {
		products, total, err = h.ProductService.ListPublicExact(categoryID, page, pageSize)
	} else {
		products, total, err = h.ProductService.ListPublic(categoryID, "", page, pageSize)
	}
	if err != nil {
		logger.Errorw("channel_catalog_list_products", "error", err)
		respondChannelError(c, 500, 500, "internal_error", "error.internal_error", err)
		return
	}

	if err := h.ProductService.ApplyAutoStockCounts(products); err != nil {
		logger.Warnw("channel_catalog_apply_stock", "error", err)
	}

	// 批量解析映射商品的真实交付类型，并将上游 SKU 库存写回到本地库存字段
	fulfillmentTypeMap := h.applyUpstreamMappings(products)

	currency, err := h.SettingService.GetSiteCurrency("CNY")
	if err != nil {
		logger.Warnw("channel_catalog_get_currency", "error", err)
		currency = "CNY"
	}

	// 可选：通过 channel_user_id 获取用户会员等级
	var userID, memberLevelID uint
	if cuid := channelUserIDFromQuery(c); cuid != "" {
		user, _, err := h.UserAuthService.ResolveTelegramChannelIdentity(TelegramIdentityInput{
			ChannelUserID: cuid,
		})
		if err == nil && user != nil {
			userID, memberLevelID = user.ID, user.MemberLevelID
		}
	}

	type productItem struct {
		ID                  uint                    `json:"id"`
		Title               string                  `json:"title"`
		Summary             string                  `json:"summary"`
		ImageURL            string                  `json:"image_url"`
		PriceFrom           string                  `json:"price_from"`
		MemberPriceFrom     string                  `json:"member_price_from,omitempty"`
		WholesalePrices     []channelWholesalePrice `json:"wholesale_prices,omitempty"`
		Currency            string                  `json:"currency"`
		StockStatus         string                  `json:"stock_status"`
		StockCount          int64                   `json:"stock_count"`
		StockDisplayMode    string                  `json:"stock_display_mode"`
		StockDisplay        string                  `json:"stock_display"`
		StockRangeMin       *int                    `json:"stock_range_min"`
		StockRangeMax       *int                    `json:"stock_range_max"`
		StockQuantityHidden bool                    `json:"stock_quantity_hidden"`
		CategoryName        string                  `json:"category_name"`
	}

	items := make([]productItem, 0, len(products))
	for _, p := range products {
		title := resolveLocalizedJSON(p.TitleJSON, locale, defaultLocale)
		desc := resolveLocalizedJSON(p.DescriptionJSON, locale, defaultLocale)
		summary := truncate(stripHTML(desc), 100)

		var imageURL string
		if len(p.Images) > 0 {
			imageURL = string(p.Images[0])
		}

		// 映射商品使用真实交付类型计算库存
		ft := p.FulfillmentType
		if eft, ok := fulfillmentTypeMap[p.ID]; ok {
			ft = eft
		}

		stockCount := domaincatalog.StockQuantity(ft, p.AutoStockAvailable, p.ManualStockTotal)
		stockStatus := domaincatalog.StorefrontStockPolicy().Status(stockCount)
		stockDisplay := domaincatalog.StorefrontStockPolicy().Display(p.StockDisplayMode, stockStatus, stockCount)
		item := productItem{
			ID:                  p.ID,
			Title:               title,
			Summary:             summary,
			ImageURL:            imageURL,
			PriceFrom:           p.PriceAmount.String(),
			WholesalePrices:     normalizeChannelWholesalePrices(p.WholesalePrices),
			Currency:            currency,
			StockStatus:         stockStatus,
			StockCount:          stockCount,
			StockDisplayMode:    stockDisplay.Mode,
			StockDisplay:        stockDisplay.Display,
			StockRangeMin:       stockDisplay.RangeMin,
			StockRangeMax:       stockDisplay.RangeMax,
			StockQuantityHidden: stockDisplay.QuantityHidden,
			CategoryName:        resolveLocalizedJSON(p.Category.NameJSON, locale, defaultLocale),
		}

		// 计算会员价
		if (userID > 0 || memberLevelID > 0) && h.MemberLevelService != nil {
			memberPrice, _ := h.resolveChannelMemberPrice(userID, memberLevelID, p.ID, 0, p.PriceAmount.Decimal)
			if memberPrice.LessThan(p.PriceAmount.Decimal) {
				item.MemberPriceFrom = money.FromDecimal(memberPrice).String()
			}
		}

		items = append(items, item)
	}

	totalPages := int64(math.Ceil(float64(total) / float64(pageSize)))

	respondChannelSuccess(c, gin.H{
		"items":      items,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
		"total_page": totalPages,
	})
}

// GetProductDetail GET /api/v1/channel/catalog/products/:id?locale=zh-CN
func (h *Handler) GetProductDetail(c *gin.Context) {
	locale := c.DefaultQuery("locale", "zh-CN")
	defaultLocale := "zh-CN"
	id := c.Param("id")

	product, err := h.ProductRepo.GetByID(id)
	if err != nil {
		logger.Errorw("channel_catalog_get_product", "id", id, "error", err)
		respondChannelError(c, 500, 500, "internal_error", "error.internal_error", err)
		return
	}
	if product == nil || !product.IsActive || !product.Category.IsActive {
		respondChannelError(c, 404, 404, "product_not_found", "error.product_not_found", nil)
		return
	}

	// 计算库存（ApplyAutoStockCounts 接受 []productdomain.Product 并修改 slice 元素）
	stockSlice := []productdomain.Product{*product}
	if err := h.ProductService.ApplyAutoStockCounts(stockSlice); err != nil {
		logger.Warnw("channel_catalog_apply_stock_detail", "error", err)
	}
	fulfillmentTypeMap := h.applyUpstreamMappings(stockSlice)
	*product = stockSlice[0]

	currency, err := h.SettingService.GetSiteCurrency("CNY")
	if err != nil {
		logger.Warnw("channel_catalog_get_currency_detail", "error", err)
		currency = "CNY"
	}

	effectiveFT := product.FulfillmentType
	if eft, ok := fulfillmentTypeMap[product.ID]; ok {
		effectiveFT = eft
	}

	title := resolveLocalizedJSON(product.TitleJSON, locale, defaultLocale)
	description := stripHTML(resolveLocalizedJSON(product.ContentJSON, locale, defaultLocale))

	var imageURL string
	if len(product.Images) > 0 {
		imageURL = string(product.Images[0])
	}

	// 可选：通过 channel_user_id 获取用户会员等级
	var userID, memberLevelID uint
	if cuid := channelUserIDFromQuery(c); cuid != "" {
		user, _, err := h.UserAuthService.ResolveTelegramChannelIdentity(TelegramIdentityInput{
			ChannelUserID: cuid,
		})
		if err == nil && user != nil {
			userID, memberLevelID = user.ID, user.MemberLevelID
		}
	}

	type skuItem struct {
		ID                  uint   `json:"id"`
		SKUCode             string `json:"sku_code"`
		SpecValues          string `json:"spec_values"`
		Price               string `json:"price"`
		MemberPrice         string `json:"member_price,omitempty"`
		StockStatus         string `json:"stock_status"`
		StockCount          int64  `json:"stock_count"`
		StockDisplayMode    string `json:"stock_display_mode"`
		StockDisplay        string `json:"stock_display"`
		StockRangeMin       *int   `json:"stock_range_min"`
		StockRangeMax       *int   `json:"stock_range_max"`
		StockQuantityHidden bool   `json:"stock_quantity_hidden"`
	}

	skus := make([]skuItem, 0, len(product.SKUs))
	for _, sku := range product.SKUs {
		if !sku.IsActive {
			continue
		}
		specValues := resolveLocalizedJSON(sku.SpecValuesJSON, locale, defaultLocale)
		stockCount := domaincatalog.StockQuantity(effectiveFT, sku.AutoStockAvailable, sku.ManualStockTotal)
		stockStatus := domaincatalog.StorefrontStockPolicy().Status(stockCount)
		stockDisplay := domaincatalog.StorefrontStockPolicy().Display(product.StockDisplayMode, stockStatus, stockCount)
		si := skuItem{
			ID:                  sku.ID,
			SKUCode:             sku.SKUCode,
			SpecValues:          specValues,
			Price:               sku.PriceAmount.String(),
			StockStatus:         stockStatus,
			StockCount:          stockCount,
			StockDisplayMode:    stockDisplay.Mode,
			StockDisplay:        stockDisplay.Display,
			StockRangeMin:       stockDisplay.RangeMin,
			StockRangeMax:       stockDisplay.RangeMax,
			StockQuantityHidden: stockDisplay.QuantityHidden,
		}
		if (userID > 0 || memberLevelID > 0) && h.MemberLevelService != nil {
			memberPrice, _ := h.resolveChannelMemberPrice(userID, memberLevelID, product.ID, sku.ID, sku.PriceAmount.Decimal)
			if memberPrice.LessThan(sku.PriceAmount.Decimal) {
				si.MemberPrice = money.FromDecimal(memberPrice).String()
			}
		}
		skus = append(skus, si)
	}

	// 商品级会员价
	var memberPriceFrom string
	if (userID > 0 || memberLevelID > 0) && h.MemberLevelService != nil {
		mp, _ := h.resolveChannelMemberPrice(userID, memberLevelID, product.ID, 0, product.PriceAmount.Decimal)
		if mp.LessThan(product.PriceAmount.Decimal) {
			memberPriceFrom = money.FromDecimal(mp).String()
		}
	}

	stockCount := domaincatalog.StockQuantity(effectiveFT, product.AutoStockAvailable, product.ManualStockTotal)
	stockStatus := domaincatalog.StorefrontStockPolicy().Status(stockCount)
	stockDisplay := domaincatalog.StorefrontStockPolicy().Display(product.StockDisplayMode, stockStatus, stockCount)
	respondChannelSuccess(c, gin.H{
		"id":                    product.ID,
		"title":                 title,
		"description":           description,
		"image_url":             imageURL,
		"price_from":            product.PriceAmount.String(),
		"member_price_from":     memberPriceFrom,
		"wholesale_prices":      normalizeChannelWholesalePrices(product.WholesalePrices),
		"currency":              currency,
		"stock_status":          stockStatus,
		"stock_count":           stockCount,
		"stock_display_mode":    stockDisplay.Mode,
		"stock_display":         stockDisplay.Display,
		"stock_range_min":       stockDisplay.RangeMin,
		"stock_range_max":       stockDisplay.RangeMax,
		"stock_quantity_hidden": stockDisplay.QuantityHidden,
		"category_name":         resolveLocalizedJSON(product.Category.NameJSON, locale, defaultLocale),
		"fulfillment_type":      effectiveFT,
		"min_purchase_quantity": normalizeChannelMinPurchaseQuantity(product.MinPurchaseQuantity),
		"max_purchase_quantity": normalizeChannelMaxPurchaseQuantity(product.MaxPurchaseQuantity),
		"manual_form_schema":    normalizeChannelManualFormSchema(product.ManualFormSchemaJSON, locale, defaultLocale),
		"purchase_note":         "",
		"skus":                  skus,
	})
}

func normalizeChannelManualFormSchema(schema jsonmap.JSON, locale, defaultLocale string) gin.H {
	fieldsRaw, ok := schema["fields"]
	if !ok {
		return gin.H{"fields": []gin.H{}}
	}

	fieldList, ok := fieldsRaw.([]interface{})
	if !ok {
		return gin.H{"fields": []gin.H{}}
	}

	fields := make([]gin.H, 0, len(fieldList))
	for _, rawField := range fieldList {
		fieldMap, ok := rawField.(map[string]interface{})
		if !ok {
			continue
		}

		field := gin.H{}
		if key, ok := fieldMap["key"].(string); ok {
			field["key"] = key
		}
		if typeValue, ok := fieldMap["type"].(string); ok {
			field["type"] = typeValue
		}
		if required, ok := fieldMap["required"].(bool); ok {
			field["required"] = required
		}
		if label := localizedFieldText(fieldMap["label"], locale, defaultLocale); label != "" {
			field["label"] = label
		}
		if placeholder := localizedFieldText(fieldMap["placeholder"], locale, defaultLocale); placeholder != "" {
			field["placeholder"] = placeholder
		}
		if regex, ok := fieldMap["regex"].(string); ok && strings.TrimSpace(regex) != "" {
			field["regex"] = regex
		}
		if minValue, ok := fieldMap["min"]; ok {
			field["min"] = minValue
		}
		if maxValue, ok := fieldMap["max"]; ok {
			field["max"] = maxValue
		}
		if maxLen, ok := fieldMap["max_len"]; ok {
			field["max_len"] = maxLen
		}
		if options, ok := fieldMap["options"].([]string); ok {
			field["options"] = options
		} else if optionsRaw, ok := fieldMap["options"].([]interface{}); ok {
			options := make([]string, 0, len(optionsRaw))
			for _, rawOption := range optionsRaw {
				option := strings.TrimSpace(fmt.Sprintf("%v", rawOption))
				if option == "" || option == "<nil>" {
					continue
				}
				options = append(options, option)
			}
			if len(options) > 0 {
				field["options"] = options
			}
		}

		fields = append(fields, field)
	}

	return gin.H{"fields": fields}
}

func localizedFieldText(raw interface{}, locale, defaultLocale string) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case jsonmap.JSON:
		return strings.TrimSpace(resolveLocalizedJSON(value, locale, defaultLocale))
	case map[string]interface{}:
		return strings.TrimSpace(resolveLocalizedJSON(jsonmap.JSON(value), locale, defaultLocale))
	default:
		text := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if text == "<nil>" {
			return ""
		}
		return text
	}
}

func normalizeChannelMaxPurchaseQuantity(value int) int {
	if value <= 0 {
		return 0
	}
	return value
}

func normalizeChannelMinPurchaseQuantity(value int) int {
	if value <= 0 {
		return 0
	}
	return value
}

// applyUpstreamMappings 一次性完成 upstream 映射商品的处理：
//  1. 批量解析每个映射商品的真实交付类型（auto / manual）；
//  2. 批量拉取 SKU 映射，并将上游库存写回 Product/SKU 的本地库存字段，
//     使下游共享库存策略无需感知 upstream 即可正确工作。
//
// 返回 productID -> displayFulfillmentType 映射，调用方据此决定对外展示的交付类型。
// 必须在 ApplyAutoStockCounts 之后调用。
//
// 降级语义（与 web 端 decorateUpstreamStock 一致）：
//   - 没有 ProductMapping：视作有货（写无限库存），避免脏数据导致误报缺货；
//   - 有 mapping 但所有 SKU 映射均未激活：视作缺货（库存写 0）。
func (h *Handler) applyUpstreamMappings(products []productdomain.Product) map[uint]string {
	ftMap := make(map[uint]string)

	var mappedIDs []uint
	for _, p := range products {
		if p.IsMapped && p.FulfillmentType == constants.FulfillmentTypeUpstream {
			mappedIDs = append(mappedIDs, p.ID)
		}
	}
	if len(mappedIDs) == 0 {
		return ftMap
	}

	mappings, err := h.ProductMappingRepo.ListByLocalProductIDs(mappedIDs)
	if err != nil {
		logger.Warnw("channel_catalog_list_product_mappings", "error", err)
		// 整批查询失败时降级为有货：避免数据库抖动让所有商品全部缺货
		for _, id := range mappedIDs {
			ftMap[id] = constants.FulfillmentTypeManual
		}
		setProductsUnlimited(products, ftMap)
		return ftMap
	}

	mappingByProduct := make(map[uint]*mappingdomain.Mapping, len(mappings))
	mappingIDs := make([]uint, 0, len(mappings))
	for i := range mappings {
		m := &mappings[i]
		mappingByProduct[m.LocalProductID] = m
		mappingIDs = append(mappingIDs, m.ID)
		ft := m.UpstreamFulfillmentType
		if ft != constants.FulfillmentTypeAuto {
			ft = constants.FulfillmentTypeManual
		}
		ftMap[m.LocalProductID] = ft
	}

	// 没有 mapping 记录的 mapped 商品：降级为有货
	for _, id := range mappedIDs {
		if _, ok := mappingByProduct[id]; !ok {
			ftMap[id] = constants.FulfillmentTypeManual
		}
	}

	skuMappings, err := h.SKUMappingRepo.ListByProductMappingIDs(mappingIDs)
	if err != nil {
		logger.Warnw("channel_catalog_list_sku_mappings", "error", err)
		setProductsUnlimited(products, ftMap)
		return ftMap
	}

	// 按 productMappingID 分桶
	skusByMapping := make(map[uint][]*mappingdomain.SKUMapping, len(mappingIDs))
	for i := range skuMappings {
		sm := &skuMappings[i]
		skusByMapping[sm.ProductMappingID] = append(skusByMapping[sm.ProductMappingID], sm)
	}

	for i := range products {
		p := &products[i]
		displayType, ok := ftMap[p.ID]
		if !ok {
			continue
		}

		mapping := mappingByProduct[p.ID]
		if mapping == nil {
			// 无 mapping：写无限库存
			writeProductStock(p, displayType, -1)
			continue
		}

		smByLocal := make(map[uint]*mappingdomain.SKUMapping)
		for _, sm := range skusByMapping[mapping.ID] {
			smByLocal[sm.LocalSKUID] = sm
		}

		hasUnlimited := false
		hasActiveMapping := false
		totalStock := 0

		for j := range p.SKUs {
			sku := &p.SKUs[j]
			sm, ok := smByLocal[sku.ID]
			if !ok || !sm.UpstreamIsActive {
				writeSKUStock(sku, displayType, 0)
				continue
			}
			hasActiveMapping = true
			writeSKUStock(sku, displayType, sm.UpstreamStock)

			if sm.UpstreamStock < 0 {
				hasUnlimited = true
			} else {
				totalStock += sm.UpstreamStock
			}
		}

		switch {
		case !hasActiveMapping:
			writeProductStock(p, displayType, 0)
		case hasUnlimited:
			writeProductStock(p, displayType, -1)
		default:
			writeProductStock(p, displayType, totalStock)
		}
	}

	return ftMap
}

// writeProductStock 按 displayType 把库存值写回 Product 的对应字段。stock<0 表示无限。
func writeProductStock(p *productdomain.Product, displayType string, stock int) {
	if displayType == constants.FulfillmentTypeAuto {
		if stock < 0 {
			p.AutoStockAvailable = -1
		} else {
			p.AutoStockAvailable = int64(stock)
		}
		return
	}
	if stock < 0 {
		p.ManualStockTotal = constants.ManualStockUnlimited
	} else {
		p.ManualStockTotal = stock
	}
}

// writeSKUStock 同 writeProductStock，作用于 SKU。
func writeSKUStock(sku *productdomain.ProductSKU, displayType string, stock int) {
	if displayType == constants.FulfillmentTypeAuto {
		if stock < 0 {
			sku.AutoStockAvailable = -1
		} else {
			sku.AutoStockAvailable = int64(stock)
		}
		return
	}
	if stock < 0 {
		sku.ManualStockTotal = constants.ManualStockUnlimited
	} else {
		sku.ManualStockTotal = stock
	}
}

// setProductsUnlimited 把 ftMap 中列出的所有商品（含其 SKU）置为无限库存。用于查询失败的降级。
func setProductsUnlimited(products []productdomain.Product, ftMap map[uint]string) {
	for i := range products {
		p := &products[i]
		dt, ok := ftMap[p.ID]
		if !ok {
			continue
		}
		writeProductStock(p, dt, -1)
		for j := range p.SKUs {
			writeSKUStock(&p.SKUs[j], dt, -1)
		}
	}
}

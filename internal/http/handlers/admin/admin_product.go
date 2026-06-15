package admin

import (
	"errors"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// GetAdminProducts 获取商品列表 (Admin)
func (h *Handler) GetAdminProducts(c *gin.Context) {
	page, pageSize := shared.ParsePagination(c)
	categoryID := c.Query("category_id")
	search := c.Query("search")
	fulfillmentType := strings.TrimSpace(c.Query("fulfillment_type"))
	stockStatus := c.Query("stock_status")
	if stockStatus == "" {
		stockStatus = c.Query("stock_staus")
	}
	hasWholesalePrices, err := parseWholesaleFilter(c.Query("wholesale"))
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if hasWholesalePrices == nil {
		hasWholesalePrices, err = parseWholesaleFilter(c.Query("has_wholesale_prices"))
		if err != nil {
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
	}

	lowStockThreshold := h.SettingService.GetDashboardLowStockThreshold()
	products, total, err := h.ProductService.ListAdmin(categoryID, search, fulfillmentType, stockStatus, hasWholesalePrices, lowStockThreshold, page, pageSize)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	if err := h.ProductService.ApplyAutoStockCounts(products); err != nil {
		shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	h.applyUpstreamDisplayTypes(products)

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, products, pagination)
}

func parseWholesaleFilter(raw string) (*bool, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "all":
		return nil, nil
	case "1", "true", "yes", "on", "enabled", "has":
		parsed := true
		return &parsed, nil
	case "0", "false", "no", "off", "disabled", "none":
		parsed := false
		return &parsed, nil
	default:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		return &parsed, nil
	}
}

// GetAdminProduct 获取商品详情 (Admin)
func (h *Handler) GetAdminProduct(c *gin.Context) {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	product, err := h.ProductService.GetAdminByID(id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	temp := []models.Product{*product}
	if err := h.ProductService.ApplyAutoStockCounts(temp); err != nil {
		shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}
	*product = temp[0]

	h.applyUpstreamDisplayTypes(temp)
	*product = temp[0]

	response.Success(c, product)
}

// ====================  商品管理  ====================

type ProductSKURequest struct {
	ID               uint                   `json:"id"`
	SKUCode          string                 `json:"sku_code" binding:"required"`
	SpecValuesJSON   map[string]interface{} `json:"spec_values"`
	PriceAmount      float64                `json:"price_amount" binding:"required"`
	CostPriceAmount  float64                `json:"cost_price_amount"`
	ManualStockTotal int                    `json:"manual_stock_total"`
	IsActive         *bool                  `json:"is_active"`
	SortOrder        int                    `json:"sort_order"`
}

type WholesalePriceRequest struct {
	SKUID       uint    `json:"sku_id"`
	SKUCode     string  `json:"sku_code"`
	MinQuantity int     `json:"min_quantity"`
	UnitPrice   float64 `json:"unit_price"`
}

// CreateProductRequest 创建商品请求
type CreateProductRequest struct {
	CategoryID          uint                     `json:"category_id" binding:"required"`
	Slug                string                   `json:"slug" binding:"required"`
	SeoMetaJSON         map[string]interface{}   `json:"seo_meta"`
	TitleJSON           map[string]interface{}   `json:"title" binding:"required"`
	DescriptionJSON     map[string]interface{}   `json:"description"`
	ContentJSON         map[string]interface{}   `json:"content"`
	InstructionsJSON    map[string]interface{}   `json:"instructions"`
	ManualFormSchema    map[string]interface{}   `json:"manual_form_schema"`
	PriceAmount         float64                  `json:"price_amount" binding:"required"`
	CostPriceAmount     float64                  `json:"cost_price_amount"`
	WholesalePrices     *[]WholesalePriceRequest `json:"wholesale_prices"`
	Images              []string                 `json:"images"`
	Tags                []string                 `json:"tags"`
	PurchaseType        string                   `json:"purchase_type"`
	MinPurchaseQuantity *int                     `json:"min_purchase_quantity"`
	MaxPurchaseQuantity *int                     `json:"max_purchase_quantity"`
	StockDisplayMode    string                   `json:"stock_display_mode"`
	FulfillmentType     string                   `json:"fulfillment_type"`
	ManualStockTotal    *int                     `json:"manual_stock_total"`
	SKUs                []ProductSKURequest      `json:"skus"`
	PaymentChannelIDs   []uint                   `json:"payment_channel_ids"`
	IsAffiliateEnabled  *bool                    `json:"is_affiliate_enabled"`
	IsActive            *bool                    `json:"is_active"`
	SortOrder           int                      `json:"sort_order"`
}

// toWholesalePriceInputs 透传「是否提供」语义：请求未携带 wholesale_prices 时返回 nil
// （Update 保留原配置），携带（含空数组）时返回非 nil 指针以整体覆盖。
func toWholesalePriceInputs(items *[]WholesalePriceRequest) *[]service.WholesalePriceInput {
	if items == nil {
		return nil
	}
	result := make([]service.WholesalePriceInput, 0, len(*items))
	for _, item := range *items {
		result = append(result, service.WholesalePriceInput{
			SKUID:       item.SKUID,
			SKUCode:     strings.TrimSpace(item.SKUCode),
			MinQuantity: item.MinQuantity,
			UnitPrice:   decimal.NewFromFloat(item.UnitPrice),
		})
	}
	return &result
}

func toProductSKUInputs(items []ProductSKURequest) []service.ProductSKUInput {
	if len(items) == 0 {
		return nil
	}
	result := make([]service.ProductSKUInput, 0, len(items))
	for _, item := range items {
		result = append(result, service.ProductSKUInput{
			ID:               item.ID,
			SKUCode:          item.SKUCode,
			SpecValuesJSON:   item.SpecValuesJSON,
			PriceAmount:      decimal.NewFromFloat(item.PriceAmount),
			CostPriceAmount:  decimal.NewFromFloat(item.CostPriceAmount),
			ManualStockTotal: item.ManualStockTotal,
			IsActive:         item.IsActive,
			SortOrder:        item.SortOrder,
		})
	}
	return result
}

// CreateProduct 创建商品
func (h *Handler) CreateProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	product, err := h.ProductService.Create(service.CreateProductInput{
		CategoryID:           req.CategoryID,
		Slug:                 req.Slug,
		SeoMetaJSON:          req.SeoMetaJSON,
		TitleJSON:            req.TitleJSON,
		DescriptionJSON:      req.DescriptionJSON,
		ContentJSON:          req.ContentJSON,
		InstructionsJSON:     req.InstructionsJSON,
		ManualFormSchemaJSON: req.ManualFormSchema,
		PriceAmount:          decimal.NewFromFloat(req.PriceAmount),
		CostPriceAmount:      decimal.NewFromFloat(req.CostPriceAmount),
		WholesalePrices:      toWholesalePriceInputs(req.WholesalePrices),
		Images:               req.Images,
		Tags:                 req.Tags,
		PurchaseType:         req.PurchaseType,
		MinPurchaseQuantity:  req.MinPurchaseQuantity,
		MaxPurchaseQuantity:  req.MaxPurchaseQuantity,
		StockDisplayMode:     req.StockDisplayMode,
		FulfillmentType:      req.FulfillmentType,
		ManualStockTotal:     req.ManualStockTotal,
		SKUs:                 toProductSKUInputs(req.SKUs),
		PaymentChannelIDs:    req.PaymentChannelIDs,
		IsAffiliateEnabled:   req.IsAffiliateEnabled,
		IsActive:             req.IsActive,
		SortOrder:            req.SortOrder,
	})
	if err != nil {
		if errors.Is(err, service.ErrSlugExists) {
			shared.RespondError(c, response.CodeBadRequest, "error.slug_exists", nil)
			return
		}
		if errors.Is(err, service.ErrProductPriceInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_price_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrProductPurchaseInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_purchase_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrProductCategoryInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_category_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrFulfillmentInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.fulfillment_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrManualFormSchemaInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.manual_form_schema_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrManualStockInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.manual_stock_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrProductPurchaseLimitInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_purchase_limit_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrProductStockDisplayInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		if errors.Is(err, service.ErrWholesalePriceInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.wholesale_price_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrProductSKUInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		if errors.Is(err, service.ErrProductSKUHasCardSecretStock) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_sku_has_card_secret_stock", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.product_create_failed", err)
		return
	}

	response.Success(c, product)
}

// UpdateProduct 更新商品
func (h *Handler) UpdateProduct(c *gin.Context) {
	id := c.Param("id")

	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	product, err := h.ProductService.Update(id, service.CreateProductInput{
		CategoryID:           req.CategoryID,
		Slug:                 req.Slug,
		SeoMetaJSON:          req.SeoMetaJSON,
		TitleJSON:            req.TitleJSON,
		DescriptionJSON:      req.DescriptionJSON,
		ContentJSON:          req.ContentJSON,
		InstructionsJSON:     req.InstructionsJSON,
		ManualFormSchemaJSON: req.ManualFormSchema,
		PriceAmount:          decimal.NewFromFloat(req.PriceAmount),
		CostPriceAmount:      decimal.NewFromFloat(req.CostPriceAmount),
		WholesalePrices:      toWholesalePriceInputs(req.WholesalePrices),
		Images:               req.Images,
		Tags:                 req.Tags,
		PurchaseType:         req.PurchaseType,
		MinPurchaseQuantity:  req.MinPurchaseQuantity,
		MaxPurchaseQuantity:  req.MaxPurchaseQuantity,
		StockDisplayMode:     req.StockDisplayMode,
		FulfillmentType:      req.FulfillmentType,
		ManualStockTotal:     req.ManualStockTotal,
		SKUs:                 toProductSKUInputs(req.SKUs),
		PaymentChannelIDs:    req.PaymentChannelIDs,
		IsAffiliateEnabled:   req.IsAffiliateEnabled,
		IsActive:             req.IsActive,
		SortOrder:            req.SortOrder,
	})
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrSlugExists) {
			shared.RespondError(c, response.CodeBadRequest, "error.slug_used", nil)
			return
		}
		if errors.Is(err, service.ErrProductPriceInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_price_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrProductPurchaseInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_purchase_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrProductCategoryInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_category_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrFulfillmentInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.fulfillment_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrManualFormSchemaInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.manual_form_schema_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrManualStockInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.manual_stock_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrProductPurchaseLimitInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_purchase_limit_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrProductStockDisplayInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		if errors.Is(err, service.ErrWholesalePriceInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.wholesale_price_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrProductSKUInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		if errors.Is(err, service.ErrProductSKUHasCardSecretStock) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_sku_has_card_secret_stock", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.product_update_failed", err)
		return
	}

	response.Success(c, product)
}

// QuickUpdateProductRequest 快速更新商品请求
type QuickUpdateProductRequest struct {
	IsActive   *bool `json:"is_active"`
	SortOrder  *int  `json:"sort_order"`
	CategoryID *uint `json:"category_id"`
}

type UpdateWholesalePricesRequest struct {
	WholesalePrices *[]WholesalePriceRequest `json:"wholesale_prices" binding:"required"`
}

// UpdateProductWholesalePrices 更新商品批发价阶梯。
func (h *Handler) UpdateProductWholesalePrices(c *gin.Context) {
	id := c.Param("id")

	var req UpdateWholesalePricesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	inputs := toWholesalePriceInputs(req.WholesalePrices)
	if inputs == nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	product, err := h.ProductService.UpdateWholesalePrices(id, *inputs)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrWholesalePriceInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.wholesale_price_invalid", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.product_update_failed", err)
		return
	}

	response.Success(c, product)
}

// QuickUpdateProduct 快速更新商品（状态/排序/分类）
func (h *Handler) QuickUpdateProduct(c *gin.Context) {
	id := c.Param("id")

	var req QuickUpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	fields := make(map[string]interface{})
	if req.IsActive != nil {
		fields["is_active"] = *req.IsActive
	}
	if req.SortOrder != nil {
		fields["sort_order"] = *req.SortOrder
	}
	if req.CategoryID != nil {
		fields["category_id"] = *req.CategoryID
	}
	if len(fields) == 0 {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	product, err := h.ProductService.QuickUpdate(id, fields)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrProductCategoryInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_category_invalid", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.product_update_failed", err)
		return
	}

	response.Success(c, product)
}

// applyUpstreamDisplayTypes 将 upstream 类型商品的 FulfillmentType 替换为上游的实际交付类型，并填充库存字段
func (h *Handler) applyUpstreamDisplayTypes(products []models.Product) {
	var upstreamIDs []uint
	idxMap := make(map[uint]int) // localProductID -> products slice index
	for i := range products {
		if products[i].FulfillmentType == constants.FulfillmentTypeUpstream {
			upstreamIDs = append(upstreamIDs, products[i].ID)
			idxMap[products[i].ID] = i
		}
	}
	if len(upstreamIDs) == 0 {
		return
	}

	mappings, err := h.ProductMappingRepo.ListByLocalProductIDs(upstreamIDs)
	if err != nil || len(mappings) == 0 {
		return
	}

	for _, mp := range mappings {
		idx, ok := idxMap[mp.LocalProductID]
		if !ok {
			continue
		}
		p := &products[idx]

		displayType := mp.UpstreamFulfillmentType
		if displayType != constants.FulfillmentTypeAuto {
			displayType = constants.FulfillmentTypeManual
		}
		p.FulfillmentType = displayType

		// 获取 SKU 映射以填充库存字段
		skuMappings, err := h.SKUMappingRepo.ListByProductMapping(mp.ID)
		if err != nil || len(skuMappings) == 0 {
			continue
		}

		skuMappingByLocal := make(map[uint]*models.SKUMapping, len(skuMappings))
		for i := range skuMappings {
			skuMappingByLocal[skuMappings[i].LocalSKUID] = &skuMappings[i]
		}

		var totalStock int64
		hasUnlimited := false

		for j := range p.SKUs {
			sku := &p.SKUs[j]
			sm, found := skuMappingByLocal[sku.ID]
			if !found || !sm.UpstreamIsActive {
				continue
			}

			if sm.UpstreamStock == -1 {
				hasUnlimited = true
			} else {
				totalStock += int64(sm.UpstreamStock)
			}

			if displayType == constants.FulfillmentTypeAuto {
				sku.AutoStockAvailable = int64(sm.UpstreamStock)
				if sm.UpstreamStock > 0 {
					sku.AutoStockTotal = int64(sm.UpstreamStock)
				}
			} else {
				sku.ManualStockTotal = sm.UpstreamStock
			}
		}

		// 填充商品级汇总库存
		if displayType == constants.FulfillmentTypeAuto {
			if hasUnlimited {
				p.AutoStockAvailable = -1
			} else {
				p.AutoStockAvailable = totalStock
				p.AutoStockTotal = totalStock
			}
		} else {
			if hasUnlimited {
				p.ManualStockTotal = constants.ManualStockUnlimited
			} else {
				p.ManualStockTotal = int(totalStock)
			}
		}
	}
}

// BatchProductActionRequest 商品批量操作请求
type BatchProductActionRequest struct {
	IDs []uint `json:"ids" binding:"required,min=1"`
}

// BatchProductStatusRequest 商品批量状态更新请求
type BatchProductStatusRequest struct {
	IDs      []uint `json:"ids" binding:"required,min=1"`
	IsActive bool   `json:"is_active"`
}

// BatchProductCategoryRequest 商品批量分类更新请求
type BatchProductCategoryRequest struct {
	IDs        []uint `json:"ids" binding:"required,min=1"`
	CategoryID uint   `json:"category_id"`
}

type batchProductFailureItem struct {
	ID        uint   `json:"id"`
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

func productBatchFailureFromError(locale string, id uint, err error) batchProductFailureItem {
	errorCode := "product_update_failed"
	switch {
	case errors.Is(err, service.ErrProductCategoryInvalid):
		errorCode = "product_category_invalid"
	case errors.Is(err, service.ErrNotFound):
		errorCode = "product_not_found"
	}
	return batchProductFailureItem{
		ID:        id,
		ErrorCode: errorCode,
		Message:   i18n.T(locale, "error."+errorCode),
	}
}

// BatchUpdateProductStatus 批量上架/下架
func (h *Handler) BatchUpdateProductStatus(c *gin.Context) {
	var req BatchProductStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	locale := i18n.ResolveLocale(c)
	successCount := 0
	failedItems := make([]batchProductFailureItem, 0)
	for _, id := range req.IDs {
		_, err := h.ProductService.QuickUpdate(strconv.FormatUint(uint64(id), 10), map[string]interface{}{"is_active": req.IsActive})
		if err == nil {
			successCount++
		} else {
			failedItems = append(failedItems, productBatchFailureFromError(locale, id, err))
		}
	}
	response.Success(c, gin.H{"total": len(req.IDs), "success_count": successCount, "failed_items": failedItems})
}

// BatchUpdateProductCategory 批量修改分类
func (h *Handler) BatchUpdateProductCategory(c *gin.Context) {
	var req BatchProductCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	successCount := 0
	for _, id := range req.IDs {
		_, err := h.ProductService.QuickUpdate(strconv.FormatUint(uint64(id), 10), map[string]interface{}{"category_id": req.CategoryID})
		if err == nil {
			successCount++
		}
	}
	response.Success(c, gin.H{"total": len(req.IDs), "success_count": successCount})
}

// BatchDeleteProducts 批量删除商品
func (h *Handler) BatchDeleteProducts(c *gin.Context) {
	var req BatchProductActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	successCount := 0
	var failedIDs []uint
	for _, id := range req.IDs {
		if err := h.ProductService.Delete(strconv.FormatUint(uint64(id), 10)); err == nil {
			successCount++
		} else {
			failedIDs = append(failedIDs, id)
		}
	}
	response.Success(c, gin.H{"total": len(req.IDs), "success_count": successCount, "failed_ids": failedIDs})
}

// DeleteProduct 删除商品
func (h *Handler) DeleteProduct(c *gin.Context) {
	id := c.Param("id")

	if err := h.ProductService.Delete(id); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrProductHasStock) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_has_stock", nil)
			return
		}
		if errors.Is(err, service.ErrProductHasOrderRecord) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_has_order_record", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.product_delete_failed", err)
		return
	}

	response.Success(c, nil)
}

package producthttp

import (
	"errors"
	"strconv"
	"strings"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/i18n"
	productwrite "github.com/dujiao-next/internal/modules/catalog/product/application/write"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/modules/catalog/product/manualform"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ProductQueries 是后台商品列表/详情读取所需的最小用例接口。
type ProductQueries interface {
	ListAdmin(categoryID, search, fulfillmentType, stockStatus string, hasWholesalePrices, isActive *bool, lowStockThreshold int, page, pageSize int) ([]productdomain.Product, int64, error)
	GetAdminByID(id string) (*productdomain.Product, error)
	ApplyAutoStockCounts(products []productdomain.Product) error
}

// ProductWriter 是商品创建与完整更新用例。
type ProductWriter interface {
	Create(input productwrite.CreateProductInput) (*productdomain.Product, error)
	Update(id string, input productwrite.CreateProductInput) (*productdomain.Product, error)
}

// ProductAdminCommands 是商品管理与删除用例。
type ProductAdminCommands interface {
	UpdateWholesalePrices(id string, prices []productdomain.WholesalePriceInput) (*productdomain.Product, error)
	QuickUpdate(id string, fields map[string]interface{}) (*productdomain.Product, error)
	Delete(id string) error
}

// LowStockThresholdProvider 提供仪表盘低库存阈值。
type LowStockThresholdProvider interface {
	GetDashboardLowStockThreshold() int
}

// ProductMappingLookup 用于把 upstream 商品展示为真实交付类型与库存。
type ProductMappingLookup interface {
	ListByLocalProductIDs(productIDs []uint) ([]mappingdomain.Mapping, error)
}

// SKUMappingLookup 读取上游 SKU 库存映射。
type SKUMappingLookup interface {
	ListByProductMapping(productMappingID uint) ([]mappingdomain.SKUMapping, error)
}

// AdminProductHandler 处理后台商品管理请求。
type AdminProductHandler struct {
	products    ProductQueries
	writer      ProductWriter
	admin       ProductAdminCommands
	settings    LowStockThresholdProvider
	mappings    ProductMappingLookup
	skuMappings SKUMappingLookup
}

// NewAdminProductHandler 创建后台商品 Handler。
func NewAdminProductHandler(
	products ProductQueries,
	writer ProductWriter,
	admin ProductAdminCommands,
	settings LowStockThresholdProvider,
	mappings ProductMappingLookup,
	skuMappings SKUMappingLookup,
) *AdminProductHandler {
	if products == nil || writer == nil || admin == nil {
		panic("catalog admin product handler: product use cases are nil")
	}
	return &AdminProductHandler{
		products:    products,
		writer:      writer,
		admin:       admin,
		settings:    settings,
		mappings:    mappings,
		skuMappings: skuMappings,
	}
}

// GetAdminProducts 获取商品列表 (Admin)
func (h *AdminProductHandler) GetAdminProducts(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	categoryID := c.Query("category_id")
	search := c.Query("search")
	fulfillmentType := strings.TrimSpace(c.Query("fulfillment_type"))
	stockStatus := c.Query("stock_status")
	if stockStatus == "" {
		stockStatus = c.Query("stock_staus")
	}
	hasWholesalePrices, err := parseTriStateBoolFilter(c.Query("wholesale"))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if hasWholesalePrices == nil {
		hasWholesalePrices, err = parseTriStateBoolFilter(c.Query("has_wholesale_prices"))
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
	}
	// 三态上架筛选：?is_active=1 仅已上架，?is_active=0 仅已下架，缺省/ all 不限。
	// 与 search / category_id / stock_status / wholesale 之间是 AND 关系，可任意组合。
	isActive, err := parseTriStateBoolFilter(c.Query("is_active"))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	lowStockThreshold := 0
	if h.settings != nil {
		lowStockThreshold = h.settings.GetDashboardLowStockThreshold()
	}
	products, total, err := h.products.ListAdmin(categoryID, search, fulfillmentType, stockStatus, hasWholesalePrices, isActive, lowStockThreshold, page, pageSize)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	if err := h.products.ApplyAutoStockCounts(products); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	h.applyUpstreamDisplayTypes(products)

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, products, pagination)
}

func parseTriStateBoolFilter(raw string) (*bool, error) {
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
func (h *AdminProductHandler) GetAdminProduct(c *gin.Context) {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	product, err := h.products.GetAdminByID(id)
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

	h.applyUpstreamDisplayTypes(temp)
	*product = temp[0]

	response.Success(c, product)
}

// ====================  商品管理  ====================

type ProductSKURequest struct {
	ID                uint                   `json:"id"`
	SKUCode           string                 `json:"sku_code" binding:"required"`
	SpecValuesJSON    map[string]interface{} `json:"spec_values"`
	PriceAmount       float64                `json:"price_amount" binding:"required"`
	CostPriceAmount   float64                `json:"cost_price_amount"`
	AgencyPriceAmount float64                `json:"agency_price_amount"`
	ManualStockTotal  int                    `json:"manual_stock_total"`
	IsActive          *bool                  `json:"is_active"`
	SortOrder         int                    `json:"sort_order"`
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
	AgencyPriceAmount   float64                  `json:"agency_price_amount"`
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
func toWholesalePriceInputs(items *[]WholesalePriceRequest) *[]productdomain.WholesalePriceInput {
	if items == nil {
		return nil
	}
	result := make([]productdomain.WholesalePriceInput, 0, len(*items))
	for _, item := range *items {
		result = append(result, productdomain.WholesalePriceInput{
			SKUID:       item.SKUID,
			SKUCode:     strings.TrimSpace(item.SKUCode),
			MinQuantity: item.MinQuantity,
			UnitPrice:   decimal.NewFromFloat(item.UnitPrice),
		})
	}
	return &result
}

func toProductSKUInputs(items []ProductSKURequest) []productwrite.ProductSKUInput {
	if len(items) == 0 {
		return nil
	}
	result := make([]productwrite.ProductSKUInput, 0, len(items))
	for _, item := range items {
		result = append(result, productwrite.ProductSKUInput{
			ID:                item.ID,
			SKUCode:           item.SKUCode,
			SpecValuesJSON:    item.SpecValuesJSON,
			PriceAmount:       decimal.NewFromFloat(item.PriceAmount),
			CostPriceAmount:   decimal.NewFromFloat(item.CostPriceAmount),
			AgencyPriceAmount: decimal.NewFromFloat(item.AgencyPriceAmount),
			ManualStockTotal:  item.ManualStockTotal,
			IsActive:          item.IsActive,
			SortOrder:         item.SortOrder,
		})
	}
	return result
}

// CreateProduct 创建商品
func (h *AdminProductHandler) CreateProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	product, err := h.writer.Create(productwrite.CreateProductInput{
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
		AgencyPriceAmount:    decimal.NewFromFloat(req.AgencyPriceAmount),
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
		if errors.Is(err, productcontract.ErrSlugExists) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.slug_exists", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductPriceInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_price_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductPurchaseInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_purchase_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductCategoryInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_category_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrFulfillmentInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.fulfillment_invalid", nil)
			return
		}
		if errors.Is(err, manualform.ErrSchemaInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.manual_form_schema_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrManualStockInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.manual_stock_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductPurchaseLimitInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_purchase_limit_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductStockDisplayInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		if errors.Is(err, productdomain.ErrWholesalePriceInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.wholesale_price_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductSKUInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductSKUHasCardSecretStock) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_sku_has_card_secret_stock", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.product_create_failed", err)
		return
	}

	response.Success(c, product)
}

// UpdateProduct 更新商品
func (h *AdminProductHandler) UpdateProduct(c *gin.Context) {
	id := c.Param("id")

	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	product, err := h.writer.Update(id, productwrite.CreateProductInput{
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
		AgencyPriceAmount:    decimal.NewFromFloat(req.AgencyPriceAmount),
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
		if errors.Is(err, productcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		if errors.Is(err, productcontract.ErrSlugExists) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.slug_used", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductPriceInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_price_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductPurchaseInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_purchase_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductCategoryInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_category_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrFulfillmentInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.fulfillment_invalid", nil)
			return
		}
		if errors.Is(err, manualform.ErrSchemaInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.manual_form_schema_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrManualStockInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.manual_stock_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductPurchaseLimitInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_purchase_limit_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductStockDisplayInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		if errors.Is(err, productdomain.ErrWholesalePriceInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.wholesale_price_invalid", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductSKUInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductSKUHasCardSecretStock) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_sku_has_card_secret_stock", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.product_update_failed", err)
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
func (h *AdminProductHandler) UpdateProductWholesalePrices(c *gin.Context) {
	id := c.Param("id")

	var req UpdateWholesalePricesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	inputs := toWholesalePriceInputs(req.WholesalePrices)
	if inputs == nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	product, err := h.admin.UpdateWholesalePrices(id, *inputs)
	if err != nil {
		if errors.Is(err, productcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		if errors.Is(err, productdomain.ErrWholesalePriceInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.wholesale_price_invalid", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.product_update_failed", err)
		return
	}

	response.Success(c, product)
}

// QuickUpdateProduct 快速更新商品（状态/排序/分类）
func (h *AdminProductHandler) QuickUpdateProduct(c *gin.Context) {
	id := c.Param("id")

	var req QuickUpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
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
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	product, err := h.admin.QuickUpdate(id, fields)
	if err != nil {
		if errors.Is(err, productcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductCategoryInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_category_invalid", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.product_update_failed", err)
		return
	}

	response.Success(c, product)
}

// applyUpstreamDisplayTypes 将 upstream 类型商品的 FulfillmentType 替换为上游的实际交付类型，并填充库存字段
func (h *AdminProductHandler) applyUpstreamDisplayTypes(products []productdomain.Product) {
	var upstreamIDs []uint
	idxMap := make(map[uint]int) // localProductID -> products slice index
	for i := range products {
		if products[i].FulfillmentType == constants.FulfillmentTypeUpstream {
			upstreamIDs = append(upstreamIDs, products[i].ID)
			idxMap[products[i].ID] = i
		}
	}
	if len(upstreamIDs) == 0 || h.mappings == nil || h.skuMappings == nil {
		return
	}

	mappings, err := h.mappings.ListByLocalProductIDs(upstreamIDs)
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
		skuMappings, err := h.skuMappings.ListByProductMapping(mp.ID)
		if err != nil || len(skuMappings) == 0 {
			continue
		}

		skuMappingByLocal := make(map[uint]*mappingdomain.SKUMapping, len(skuMappings))
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
	case errors.Is(err, productcontract.ErrProductCategoryInvalid):
		errorCode = "product_category_invalid"
	case errors.Is(err, productcontract.ErrNotFound):
		errorCode = "product_not_found"
	}
	return batchProductFailureItem{
		ID:        id,
		ErrorCode: errorCode,
		Message:   i18n.T(locale, "error."+errorCode),
	}
}

// BatchUpdateProductStatus 批量上架/下架
func (h *AdminProductHandler) BatchUpdateProductStatus(c *gin.Context) {
	var req BatchProductStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	locale := i18n.ResolveLocale(c)
	successCount := 0
	failedItems := make([]batchProductFailureItem, 0)
	for _, id := range req.IDs {
		_, err := h.admin.QuickUpdate(strconv.FormatUint(uint64(id), 10), map[string]interface{}{"is_active": req.IsActive})
		if err == nil {
			successCount++
		} else {
			failedItems = append(failedItems, productBatchFailureFromError(locale, id, err))
		}
	}
	response.Success(c, gin.H{"total": len(req.IDs), "success_count": successCount, "failed_items": failedItems})
}

// BatchUpdateProductCategory 批量修改分类
func (h *AdminProductHandler) BatchUpdateProductCategory(c *gin.Context) {
	var req BatchProductCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	successCount := 0
	for _, id := range req.IDs {
		_, err := h.admin.QuickUpdate(strconv.FormatUint(uint64(id), 10), map[string]interface{}{"category_id": req.CategoryID})
		if err == nil {
			successCount++
		}
	}
	response.Success(c, gin.H{"total": len(req.IDs), "success_count": successCount})
}

// BatchDeleteProducts 批量删除商品
func (h *AdminProductHandler) BatchDeleteProducts(c *gin.Context) {
	var req BatchProductActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	successCount := 0
	var failedIDs []uint
	for _, id := range req.IDs {
		if err := h.admin.Delete(strconv.FormatUint(uint64(id), 10)); err == nil {
			successCount++
		} else {
			failedIDs = append(failedIDs, id)
		}
	}
	response.Success(c, gin.H{"total": len(req.IDs), "success_count": successCount, "failed_ids": failedIDs})
}

// DeleteProduct 删除商品
func (h *AdminProductHandler) DeleteProduct(c *gin.Context) {
	id := c.Param("id")

	if err := h.admin.Delete(id); err != nil {
		if errors.Is(err, productcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductHasStock) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_has_stock", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductHasOrderRecord) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_has_order_record", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.product_delete_failed", err)
		return
	}

	response.Success(c, nil)
}

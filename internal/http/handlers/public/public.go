package public

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	publicConfigCacheKey = "public:config"
	publicConfigCacheTTL = 60 * time.Second
	publicLowStockLimit  = 5
)

// PublicProductView 公共商品响应结构
type PublicProductView struct {
	models.Product
	PromotionID          *uint         `json:"promotion_id,omitempty"`
	PromotionName        string        `json:"promotion_name,omitempty"`
	PromotionType        string        `json:"promotion_type,omitempty"`
	PromotionPriceAmount *models.Money `json:"promotion_price_amount,omitempty"`
	ManualStockAvailable int           `json:"manual_stock_available"`
	AutoStockAvailable   int64         `json:"auto_stock_available"`
	StockStatus          string        `json:"stock_status"`
	IsSoldOut            bool          `json:"is_sold_out"`
}

// GetConfig 获取全局配置
func (h *Handler) GetConfig(c *gin.Context) {
	// 默认配置
	defaults := map[string]interface{}{
		"languages":                        []string{"zh-CN", "zh-TW", "en-US"},
		constants.SettingFieldSiteCurrency: constants.SiteCurrencyDefault,
		"contact": map[string]interface{}{
			"telegram": "https://t.me/dujiaoka",
			"whatsapp": "https://wa.me/1234567890",
		},
		"scripts": make([]interface{}, 0),
	}

	var cached map[string]interface{}
	if hit, err := cache.GetJSON(c.Request.Context(), publicConfigCacheKey, &cached); err == nil && hit {
		response.Success(c, cached)
		return
	}

	data, err := h.SettingService.GetConfig(defaults)
	if err != nil {
		respondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}

	channels, _, err := h.PaymentService.ListChannels(repository.PaymentChannelListFilter{
		Page:       1,
		PageSize:   200,
		ActiveOnly: true,
	})
	if err != nil {
		respondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	publicChannels := make([]map[string]interface{}, 0, len(channels))
	for _, channel := range channels {
		publicChannels = append(publicChannels, map[string]interface{}{
			"id":               channel.ID,
			"name":             channel.Name,
			"provider_type":    channel.ProviderType,
			"channel_type":     channel.ChannelType,
			"interaction_mode": channel.InteractionMode,
			"fee_rate":         channel.FeeRate,
		})
	}
	data["payment_channels"] = publicChannels

	if h.CaptchaService != nil {
		publicCaptcha, captchaErr := h.CaptchaService.GetPublicSetting()
		if captchaErr != nil {
			respondError(c, response.CodeInternal, "error.config_fetch_failed", captchaErr)
			return
		}
		data["captcha"] = publicCaptcha
	}
	telegramAuthConfig := map[string]interface{}{
		"enabled":      false,
		"bot_username": "",
	}
	if h.TelegramAuthService != nil {
		telegramAuthConfig = h.TelegramAuthService.PublicConfig()
	} else if h.Config != nil {
		telegramAuthConfig["enabled"] = h.Config.TelegramAuth.Enabled
		telegramAuthConfig["bot_username"] = strings.TrimSpace(h.Config.TelegramAuth.BotUsername)
	}
	data["telegram_auth"] = telegramAuthConfig

	// 用户注册配置
	userRegConfig := map[string]interface{}{
		"enabled":              true,
		"require_email_verify": true,
	}
	if siteConfig, err := h.SettingService.GetByKey(constants.SettingKeySiteConfig); err == nil && siteConfig != nil {
		if regConfig, ok := siteConfig["user_registration"].(map[string]interface{}); ok {
			if v, ok := regConfig["enabled"].(bool); ok {
				userRegConfig["enabled"] = v
			}
			if v, ok := regConfig["require_email_verify"].(bool); ok {
				userRegConfig["require_email_verify"] = v
			}
		}
	}
	data["user_registration"] = userRegConfig

	_ = cache.SetJSON(c.Request.Context(), publicConfigCacheKey, data, publicConfigCacheTTL)
	response.Success(c, data)
}

// GetProducts 获取商品列表
func (h *Handler) GetProducts(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = normalizePagination(page, pageSize)

	// 获取筛选参数
	categoryID := c.Query("category_id")
	search := strings.TrimSpace(c.Query("search"))

	products, total, err := h.ProductService.ListPublic(categoryID, search, page, pageSize)
	if err != nil {
		respondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	var promotionService *service.PromotionService
	if h.PromotionRepo != nil {
		promotionService = service.NewPromotionService(h.PromotionRepo)
	}
	autoStockMap := make(map[uint]int64)
	if h.CardSecretRepo != nil {
		autoProductIDs := make([]uint, 0)
		for i := range products {
			if strings.TrimSpace(products[i].FulfillmentType) != constants.FulfillmentTypeAuto {
				continue
			}
			if products[i].ID == 0 {
				continue
			}
			autoProductIDs = append(autoProductIDs, products[i].ID)
		}
		if len(autoProductIDs) > 0 {
			counts, countErr := h.CardSecretRepo.CountAvailableByProductIDs(autoProductIDs)
			if countErr != nil {
				respondError(c, response.CodeInternal, "error.product_fetch_failed", countErr)
				return
			}
			autoStockMap = counts
		}
	}

	decorated := make([]PublicProductView, 0, len(products))
	for i := range products {
		item, derr := h.decoratePublicProduct(&products[i], promotionService, autoStockMap)
		if derr != nil {
			respondError(c, response.CodeInternal, "error.product_fetch_failed", derr)
			return
		}
		decorated = append(decorated, item)
	}

	// 统一响应格式
	pagination := response.Pagination{
		Page:      page,
		PageSize:  pageSize,
		Total:     total,
		TotalPage: (total + int64(pageSize) - 1) / int64(pageSize),
	}
	response.SuccessWithPage(c, decorated, pagination)
}

// GetProductBySlug 根据 slug 获取商品详情
func (h *Handler) GetProductBySlug(c *gin.Context) {
	slug := c.Param("slug")

	product, err := h.ProductService.GetPublicBySlug(slug)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			respondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		respondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	var promotionService *service.PromotionService
	if h.PromotionRepo != nil {
		promotionService = service.NewPromotionService(h.PromotionRepo)
	}
	autoStockMap := make(map[uint]int64)
	if h.CardSecretRepo != nil && strings.TrimSpace(product.FulfillmentType) == constants.FulfillmentTypeAuto {
		count, countErr := h.CardSecretRepo.CountAvailable(product.ID, 0)
		if countErr != nil {
			respondError(c, response.CodeInternal, "error.product_fetch_failed", countErr)
			return
		}
		autoStockMap[product.ID] = count
	}

	decorated, derr := h.decoratePublicProduct(product, promotionService, autoStockMap)
	if derr != nil {
		respondError(c, response.CodeInternal, "error.product_fetch_failed", derr)
		return
	}

	response.Success(c, decorated)
}

func (h *Handler) decoratePublicProduct(product *models.Product, promotionService *service.PromotionService, autoStockMap map[uint]int64) (PublicProductView, error) {
	if product == nil {
		return PublicProductView{}, nil
	}

	item := PublicProductView{Product: *product}
	h.decorateProductStock(product, &item, autoStockMap)
	if promotionService == nil {
		return item, nil
	}

	promotion, discountedPrice, err := promotionService.ApplyPromotion(product, 1)
	if err != nil {
		if errors.Is(err, service.ErrPromotionInvalid) {
			return item, nil
		}
		return PublicProductView{}, err
	}
	if promotion == nil {
		return item, nil
	}
	if !discountedPrice.Decimal.LessThan(product.PriceAmount.Decimal) {
		return item, nil
	}

	promotionID := promotion.ID
	item.PromotionID = &promotionID
	item.PromotionName = strings.TrimSpace(promotion.Name)
	item.PromotionType = strings.TrimSpace(promotion.Type)
	item.PromotionPriceAmount = &discountedPrice

	return item, nil
}

func (h *Handler) decorateProductStock(product *models.Product, item *PublicProductView, autoStockMap map[uint]int64) {
	if product == nil || item == nil {
		return
	}

	stockStatus := constants.ProductStockStatusInStock
	manualAvailable := product.ManualStockTotal - product.ManualStockLocked - product.ManualStockSold
	if manualAvailable < 0 {
		manualAvailable = 0
	}

	item.ManualStockAvailable = manualAvailable
	item.AutoStockAvailable = 0
	item.StockStatus = stockStatus
	item.IsSoldOut = false

	fulfillmentType := strings.TrimSpace(product.FulfillmentType)
	if fulfillmentType == "" {
		fulfillmentType = constants.FulfillmentTypeManual
	}

	if fulfillmentType == constants.FulfillmentTypeManual {
		if product.ManualStockTotal <= 0 {
			item.StockStatus = constants.ProductStockStatusUnlimited
			item.IsSoldOut = false
			return
		}

		switch {
		case manualAvailable <= 0:
			item.StockStatus = constants.ProductStockStatusOutOfStock
			item.IsSoldOut = true
		case manualAvailable <= publicLowStockLimit:
			item.StockStatus = constants.ProductStockStatusLowStock
		default:
			item.StockStatus = constants.ProductStockStatusInStock
		}
		return
	}

	autoAvailable := int64(0)
	if autoStockMap != nil {
		autoAvailable = autoStockMap[product.ID]
	}
	item.AutoStockAvailable = autoAvailable

	switch {
	case autoAvailable <= 0:
		item.StockStatus = constants.ProductStockStatusOutOfStock
		item.IsSoldOut = true
	case autoAvailable <= int64(publicLowStockLimit):
		item.StockStatus = constants.ProductStockStatusLowStock
	default:
		item.StockStatus = constants.ProductStockStatusInStock
	}
}

// GetPosts 获取文章/公告列表
func (h *Handler) GetPosts(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = normalizePagination(page, pageSize)

	// 获取类型参数
	postType := c.Query("type") // blog 或 notice

	posts, total, err := h.PostService.ListPublic(postType, page, pageSize)
	if err != nil {
		respondError(c, response.CodeInternal, "error.post_fetch_failed", err)
		return
	}

	// 统一响应格式
	pagination := response.Pagination{
		Page:      page,
		PageSize:  pageSize,
		Total:     total,
		TotalPage: (total + int64(pageSize) - 1) / int64(pageSize),
	}
	response.SuccessWithPage(c, posts, pagination)
}

// GetPostBySlug 根据 slug 获取文章详情
func (h *Handler) GetPostBySlug(c *gin.Context) {
	slug := c.Param("slug")

	post, err := h.PostService.GetPublicBySlug(slug)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			respondError(c, response.CodeNotFound, "error.post_not_found", nil)
			return
		}
		respondError(c, response.CodeInternal, "error.post_fetch_failed", err)
		return
	}

	response.Success(c, post)
}

// GetCategories 获取分类列表
func (h *Handler) GetCategories(c *gin.Context) {
	categories, err := h.CategoryService.List()
	if err != nil {
		respondError(c, response.CodeInternal, "error.category_fetch_failed", err)
		return
	}

	response.Success(c, categories)
}

// CreateGuestOrderRequest 游客下单请求
type CreateGuestOrderRequest struct {
	Email          string                 `json:"email" binding:"required"`
	OrderPassword  string                 `json:"order_password" binding:"required"`
	Items          []OrderItemRequest     `json:"items" binding:"required"`
	CouponCode     string                 `json:"coupon_code"`
	ManualFormData map[string]models.JSON `json:"manual_form_data"`
	CaptchaPayload CaptchaPayloadRequest  `json:"captcha_payload"`
}

// CreateGuestOrder 游客创建订单
func (h *Handler) CreateGuestOrder(c *gin.Context) {
	var req CreateGuestOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if h.CaptchaService != nil {
		if captchaErr := h.CaptchaService.Verify(constants.CaptchaSceneGuestCreateOrder, req.CaptchaPayload.toServicePayload(), c.ClientIP()); captchaErr != nil {
			switch {
			case errors.Is(captchaErr, service.ErrCaptchaRequired):
				respondError(c, response.CodeBadRequest, "error.captcha_required", nil)
				return
			case errors.Is(captchaErr, service.ErrCaptchaInvalid):
				respondError(c, response.CodeBadRequest, "error.captcha_invalid", nil)
				return
			case errors.Is(captchaErr, service.ErrCaptchaConfigInvalid):
				respondError(c, response.CodeInternal, "error.captcha_config_invalid", captchaErr)
				return
			default:
				respondError(c, response.CodeInternal, "error.captcha_verify_failed", captchaErr)
				return
			}
		}
	}
	var items []service.CreateOrderItem
	for _, item := range req.Items {
		items = append(items, service.CreateOrderItem{
			ProductID:       item.ProductID,
			SKUID:           item.SKUID,
			Quantity:        item.Quantity,
			FulfillmentType: item.FulfillmentType,
		})
	}
	order, err := h.OrderService.CreateGuestOrder(service.CreateGuestOrderInput{
		Email:          req.Email,
		OrderPassword:  req.OrderPassword,
		Locale:         i18n.ResolveLocale(c),
		Items:          items,
		CouponCode:     req.CouponCode,
		ClientIP:       c.ClientIP(),
		ManualFormData: req.ManualFormData,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProductSKURequired):
			respondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		case errors.Is(err, service.ErrProductSKUInvalid):
			respondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		case errors.Is(err, service.ErrGuestEmailRequired):
			respondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		case errors.Is(err, service.ErrGuestPasswordRequired):
			respondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		case errors.Is(err, service.ErrInvalidEmail):
			respondError(c, response.CodeBadRequest, "error.email_invalid", nil)
		case errors.Is(err, service.ErrProductPurchaseNotAllowed):
			respondError(c, response.CodeBadRequest, "error.product_purchase_not_allowed", nil)
		case errors.Is(err, service.ErrGuestCouponNotAllowed):
			respondError(c, response.CodeBadRequest, "error.guest_coupon_not_allowed", nil)
		case errors.Is(err, service.ErrInvalidOrderItem):
			respondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		case errors.Is(err, service.ErrInvalidOrderAmount):
			respondError(c, response.CodeBadRequest, "error.order_amount_invalid", nil)
		case errors.Is(err, service.ErrManualStockInsufficient):
			respondError(c, response.CodeBadRequest, "error.manual_stock_insufficient", nil)
		case errors.Is(err, service.ErrCardSecretInsufficient):
			respondError(c, response.CodeBadRequest, "error.card_secret_insufficient", nil)
		case errors.Is(err, service.ErrOrderCurrencyMismatch):
			respondError(c, response.CodeBadRequest, "error.order_currency_mismatch", nil)
		case errors.Is(err, service.ErrProductPriceInvalid):
			respondError(c, response.CodeBadRequest, "error.product_price_invalid", nil)
		case errors.Is(err, service.ErrProductNotAvailable):
			respondError(c, response.CodeBadRequest, "error.product_not_available", nil)
		case errors.Is(err, service.ErrQueueUnavailable):
			respondError(c, response.CodeInternal, "error.queue_unavailable", nil)
		case errors.Is(err, service.ErrManualFormSchemaInvalid):
			respondError(c, response.CodeBadRequest, "error.manual_form_schema_invalid", nil)
		case errors.Is(err, service.ErrManualFormRequiredMissing):
			respondError(c, response.CodeBadRequest, "error.manual_form_required_missing", nil)
		case errors.Is(err, service.ErrManualFormFieldInvalid):
			respondError(c, response.CodeBadRequest, "error.manual_form_field_invalid", nil)
		case errors.Is(err, service.ErrManualFormTypeInvalid):
			respondError(c, response.CodeBadRequest, "error.manual_form_type_invalid", nil)
		case errors.Is(err, service.ErrManualFormOptionInvalid):
			respondError(c, response.CodeBadRequest, "error.manual_form_option_invalid", nil)
		default:
			respondError(c, response.CodeInternal, "error.order_create_failed", err)
		}
		return
	}
	response.Success(c, order)
}

// PreviewGuestOrder 游客订单金额预览
func (h *Handler) PreviewGuestOrder(c *gin.Context) {
	var req CreateGuestOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var items []service.CreateOrderItem
	for _, item := range req.Items {
		items = append(items, service.CreateOrderItem{
			ProductID:       item.ProductID,
			SKUID:           item.SKUID,
			Quantity:        item.Quantity,
			FulfillmentType: item.FulfillmentType,
		})
	}
	preview, err := h.OrderService.PreviewGuestOrder(service.CreateGuestOrderInput{
		Email:          req.Email,
		OrderPassword:  req.OrderPassword,
		Locale:         i18n.ResolveLocale(c),
		Items:          items,
		CouponCode:     req.CouponCode,
		ClientIP:       c.ClientIP(),
		ManualFormData: req.ManualFormData,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProductSKURequired):
			respondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		case errors.Is(err, service.ErrProductSKUInvalid):
			respondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		case errors.Is(err, service.ErrGuestEmailRequired):
			respondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		case errors.Is(err, service.ErrGuestPasswordRequired):
			respondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		case errors.Is(err, service.ErrInvalidEmail):
			respondError(c, response.CodeBadRequest, "error.email_invalid", nil)
		case errors.Is(err, service.ErrProductPurchaseNotAllowed):
			respondError(c, response.CodeBadRequest, "error.product_purchase_not_allowed", nil)
		case errors.Is(err, service.ErrGuestCouponNotAllowed):
			respondError(c, response.CodeBadRequest, "error.guest_coupon_not_allowed", nil)
		case errors.Is(err, service.ErrInvalidOrderItem):
			respondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		case errors.Is(err, service.ErrInvalidOrderAmount):
			respondError(c, response.CodeBadRequest, "error.order_amount_invalid", nil)
		case errors.Is(err, service.ErrManualStockInsufficient):
			respondError(c, response.CodeBadRequest, "error.manual_stock_insufficient", nil)
		case errors.Is(err, service.ErrCardSecretInsufficient):
			respondError(c, response.CodeBadRequest, "error.card_secret_insufficient", nil)
		case errors.Is(err, service.ErrOrderCurrencyMismatch):
			respondError(c, response.CodeBadRequest, "error.order_currency_mismatch", nil)
		case errors.Is(err, service.ErrProductPriceInvalid):
			respondError(c, response.CodeBadRequest, "error.product_price_invalid", nil)
		case errors.Is(err, service.ErrProductNotAvailable):
			respondError(c, response.CodeBadRequest, "error.product_not_available", nil)
		case errors.Is(err, service.ErrCouponInvalid):
			respondError(c, response.CodeBadRequest, "error.coupon_invalid", nil)
		case errors.Is(err, service.ErrCouponNotFound):
			respondError(c, response.CodeBadRequest, "error.coupon_not_found", nil)
		case errors.Is(err, service.ErrCouponInactive):
			respondError(c, response.CodeBadRequest, "error.coupon_inactive", nil)
		case errors.Is(err, service.ErrCouponNotStarted):
			respondError(c, response.CodeBadRequest, "error.coupon_not_started", nil)
		case errors.Is(err, service.ErrCouponExpired):
			respondError(c, response.CodeBadRequest, "error.coupon_expired", nil)
		case errors.Is(err, service.ErrCouponUsageLimit):
			respondError(c, response.CodeBadRequest, "error.coupon_usage_limit", nil)
		case errors.Is(err, service.ErrCouponPerUserLimit):
			respondError(c, response.CodeBadRequest, "error.coupon_per_user_limit", nil)
		case errors.Is(err, service.ErrCouponMinAmount):
			respondError(c, response.CodeBadRequest, "error.coupon_min_amount", nil)
		case errors.Is(err, service.ErrCouponScopeInvalid):
			respondError(c, response.CodeBadRequest, "error.coupon_scope_invalid", nil)
		case errors.Is(err, service.ErrPromotionInvalid):
			respondError(c, response.CodeBadRequest, "error.promotion_invalid", nil)
		case errors.Is(err, service.ErrManualFormSchemaInvalid):
			respondError(c, response.CodeBadRequest, "error.manual_form_schema_invalid", nil)
		case errors.Is(err, service.ErrManualFormRequiredMissing):
			respondError(c, response.CodeBadRequest, "error.manual_form_required_missing", nil)
		case errors.Is(err, service.ErrManualFormFieldInvalid):
			respondError(c, response.CodeBadRequest, "error.manual_form_field_invalid", nil)
		case errors.Is(err, service.ErrManualFormTypeInvalid):
			respondError(c, response.CodeBadRequest, "error.manual_form_type_invalid", nil)
		case errors.Is(err, service.ErrManualFormOptionInvalid):
			respondError(c, response.CodeBadRequest, "error.manual_form_option_invalid", nil)
		default:
			respondError(c, response.CodeInternal, "error.order_create_failed", err)
		}
		return
	}
	response.Success(c, preview)
}

// ListGuestOrders 获取游客订单列表
func (h *Handler) ListGuestOrders(c *gin.Context) {
	email := strings.TrimSpace(c.Query("email"))
	password := strings.TrimSpace(c.Query("order_password"))
	orderNo := strings.TrimSpace(c.Query("order_no"))
	if email == "" {
		respondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		respondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}

	if orderNo != "" {
		order, err := h.OrderService.GetOrderByGuestOrderNo(orderNo, email, password)
		if err != nil {
			if errors.Is(err, service.ErrGuestOrderNotFound) {
				pagination := response.Pagination{
					Page:      1,
					PageSize:  1,
					Total:     0,
					TotalPage: 1,
				}
				response.SuccessWithPage(c, []models.Order{}, pagination)
				return
			}
			respondError(c, response.CodeInternal, "error.order_fetch_failed", err)
			return
		}
		pagination := response.Pagination{
			Page:      1,
			PageSize:  1,
			Total:     1,
			TotalPage: 1,
		}
		response.SuccessWithPage(c, []models.Order{*order}, pagination)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = normalizePagination(page, pageSize)

	orders, total, err := h.OrderService.ListOrdersByGuest(email, password, page, pageSize)
	if err != nil {
		respondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	pagination := response.Pagination{
		Page:      page,
		PageSize:  pageSize,
		Total:     total,
		TotalPage: (total + int64(pageSize) - 1) / int64(pageSize),
	}
	response.SuccessWithPage(c, orders, pagination)
}

// GetGuestOrder 获取游客订单详情
func (h *Handler) GetGuestOrder(c *gin.Context) {
	email := strings.TrimSpace(c.Query("email"))
	password := strings.TrimSpace(c.Query("order_password"))
	if email == "" {
		respondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		respondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || orderID == 0 {
		respondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	order, err := h.OrderService.GetOrderByGuest(uint(orderID), email, password)
	if err != nil {
		if errors.Is(err, service.ErrGuestOrderNotFound) {
			respondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		respondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	response.Success(c, order)
}

// GetGuestOrderByOrderNo 按订单号获取游客订单详情
func (h *Handler) GetGuestOrderByOrderNo(c *gin.Context) {
	email := strings.TrimSpace(c.Query("email"))
	password := strings.TrimSpace(c.Query("order_password"))
	if email == "" {
		respondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		respondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}
	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		respondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	order, err := h.OrderService.GetOrderByGuestOrderNo(orderNo, email, password)
	if err != nil {
		if errors.Is(err, service.ErrGuestOrderNotFound) {
			respondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		respondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	response.Success(c, order)
}

// CreateGuestPaymentRequest 游客发起支付请求
type CreateGuestPaymentRequest struct {
	Email         string `json:"email" binding:"required"`
	OrderPassword string `json:"order_password" binding:"required"`
	OrderID       uint   `json:"order_id" binding:"required"`
	ChannelID     uint   `json:"channel_id" binding:"required"`
}

type LatestGuestPaymentQuery struct {
	Email         string `form:"email" binding:"required"`
	OrderPassword string `form:"order_password" binding:"required"`
	OrderID       uint   `form:"order_id" binding:"required"`
}

// CreateGuestPayment 游客发起支付
func (h *Handler) CreateGuestPayment(c *gin.Context) {
	var req CreateGuestPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.OrderPassword)
	if email == "" {
		respondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		respondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}
	if _, err := h.OrderService.GetOrderByGuest(req.OrderID, email, password); err != nil {
		if errors.Is(err, service.ErrGuestOrderNotFound) {
			respondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		respondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	result, err := h.PaymentService.CreatePayment(service.CreatePaymentInput{
		OrderID:    req.OrderID,
		ChannelID:  req.ChannelID,
		UseBalance: false,
		ClientIP:   c.ClientIP(),
		Context:    c.Request.Context(),
	})
	if err != nil {
		respondPaymentCreateError(c, err)
		return
	}
	resp := gin.H{
		"order_paid":         result.OrderPaid,
		"wallet_paid_amount": result.WalletPaidAmount,
		"online_pay_amount":  result.OnlinePayAmount,
	}
	if result.Payment != nil {
		resp["payment_id"] = result.Payment.ID
		resp["provider_type"] = result.Payment.ProviderType
		resp["channel_type"] = result.Payment.ChannelType
		resp["interaction_mode"] = result.Payment.InteractionMode
		resp["pay_url"] = result.Payment.PayURL
		resp["qr_code"] = result.Payment.QRCode
		resp["expires_at"] = result.Payment.ExpiredAt
	}
	response.Success(c, resp)
}

// CaptureGuestPaymentRequest 游客捕获支付请求。
type CaptureGuestPaymentRequest struct {
	Email         string `json:"email" binding:"required"`
	OrderPassword string `json:"order_password" binding:"required"`
}

// CaptureGuestPayment 游客捕获支付。
func (h *Handler) CaptureGuestPayment(c *gin.Context) {
	paymentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || paymentID == 0 {
		respondError(c, response.CodeBadRequest, "error.payment_invalid", nil)
		return
	}
	var req CaptureGuestPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.OrderPassword)
	if email == "" {
		respondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		respondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}

	payment, err := h.PaymentService.GetPayment(uint(paymentID))
	if err != nil {
		if errors.Is(err, service.ErrPaymentNotFound) {
			respondError(c, response.CodeNotFound, "error.payment_not_found", nil)
			return
		}
		respondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}
	if _, err := h.OrderService.GetOrderByGuest(payment.OrderID, email, password); err != nil {
		if errors.Is(err, service.ErrGuestOrderNotFound) {
			respondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		respondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}

	updated, err := h.PaymentService.CapturePayment(service.CapturePaymentInput{
		PaymentID: uint(paymentID),
		Context:   c.Request.Context(),
	})
	if err != nil {
		respondPaymentCaptureError(c, err)
		return
	}
	response.Success(c, gin.H{
		"payment_id": updated.ID,
		"status":     updated.Status,
	})
}

// GetGuestLatestPayment 获取游客最新待支付记录
func (h *Handler) GetGuestLatestPayment(c *gin.Context) {
	var query LatestGuestPaymentQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		respondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	email := strings.TrimSpace(query.Email)
	password := strings.TrimSpace(query.OrderPassword)
	if email == "" {
		respondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		respondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}

	order, err := h.OrderService.GetOrderByGuest(query.OrderID, email, password)
	if err != nil {
		if errors.Is(err, service.ErrGuestOrderNotFound) {
			respondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		respondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	if order.ParentID != nil {
		respondError(c, response.CodeBadRequest, "error.payment_invalid", nil)
		return
	}
	if order.Status != constants.OrderStatusPendingPayment {
		respondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
		return
	}
	if order.ExpiresAt != nil && !order.ExpiresAt.After(time.Now()) {
		respondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
		return
	}

	payment, err := h.PaymentRepo.GetLatestPendingByOrder(order.ID, time.Now())
	if err != nil {
		respondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}
	if payment == nil {
		respondError(c, response.CodeNotFound, "error.payment_not_found", nil)
		return
	}

	response.Success(c, gin.H{
		"payment_id":       payment.ID,
		"order_id":         payment.OrderID,
		"channel_id":       payment.ChannelID,
		"provider_type":    payment.ProviderType,
		"channel_type":     payment.ChannelType,
		"interaction_mode": payment.InteractionMode,
		"pay_url":          payment.PayURL,
		"qr_code":          payment.QRCode,
		"expires_at":       payment.ExpiredAt,
	})
}

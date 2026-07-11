package public

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/dto"
	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/service"
	"github.com/dujiao-next/internal/version"

	"github.com/gin-gonic/gin"
)

const (
	publicConfigCacheTTL = 60 * time.Second
	publicLowStockLimit  = 5
)

// publicSKUView 内部 SKU 计算结构，用于装饰逻辑
type publicSKUView struct {
	models.ProductSKU
	PromotionPriceAmount *models.Money
	MemberPriceAmount    *models.Money
}

// publicProductView 内部商品计算结构，装饰完成后转换为 dto.ProductResp
type publicProductView struct {
	models.Product
	PromotionID          *uint
	PromotionName        string
	PromotionType        string
	PromotionPriceAmount *models.Money
	PromotionRules       []dto.PromotionRuleResp
	MemberPrices         []dto.MemberLevelPrice
	PublicSKUs           []publicSKUView
	ManualStockAvailable int
	AutoStockAvailable   int64
	StockStatus          string
	IsSoldOut            bool
}

type publicStockDisplayView struct {
	mode           string
	status         string
	display        string
	rangeMin       *int
	rangeMax       *int
	quantityHidden bool
}

// toProductResp 将内部计算结构转换为公共 DTO
func (v *publicProductView) toProductResp() dto.ProductResp {
	mode := normalizePublicStockDisplayMode(v.Product.StockDisplayMode)
	productQuantity := v.productStockQuantity()
	productDisplay := buildPublicStockDisplay(mode, v.StockStatus, productQuantity)

	skus := make([]dto.SKUResp, 0, len(v.PublicSKUs))
	for _, sv := range v.PublicSKUs {
		skuStatus, skuQuantity := v.skuStockState(sv)
		skuDisplay := buildPublicStockDisplay(mode, skuStatus, skuQuantity)
		skus = append(skus, dto.SKUResp{
			ID:                   sv.ID,
			SKUCode:              sv.SKUCode,
			SpecValues:           sv.SpecValuesJSON,
			PriceAmount:          sv.PriceAmount,
			ManualStockTotal:     maskPublicStockInt(mode, sv.ManualStockTotal),
			ManualStockSold:      maskPublicStockSold(mode, sv.ManualStockSold),
			AutoStockAvailable:   maskPublicStockInt64(mode, sv.AutoStockAvailable),
			UpstreamStock:        maskPublicStockInt(mode, sv.UpstreamStock),
			StockStatus:          skuStatus,
			StockDisplayMode:     skuDisplay.mode,
			StockDisplay:         skuDisplay.display,
			StockRangeMin:        skuDisplay.rangeMin,
			StockRangeMax:        skuDisplay.rangeMax,
			StockQuantityHidden:  skuDisplay.quantityHidden,
			IsSoldOut:            skuStatus == constants.ProductStockStatusOutOfStock,
			IsActive:             sv.IsActive,
			PromotionPriceAmount: sv.PromotionPriceAmount,
			MemberPriceAmount:    sv.MemberPriceAmount,
		})
	}

	resp := dto.ProductResp{
		ID:                   v.Product.ID,
		CategoryID:           v.Product.CategoryID,
		Slug:                 v.Product.Slug,
		SeoMeta:              v.Product.SeoMetaJSON,
		Title:                v.Product.TitleJSON,
		Description:          v.Product.DescriptionJSON,
		Content:              v.Product.ContentJSON,
		PriceAmount:          v.Product.PriceAmount,
		WholesalePrices:      dto.NewWholesalePriceRespList(v.Product.WholesalePrices),
		Images:               v.Product.Images,
		Tags:                 v.Product.Tags,
		PurchaseType:         v.Product.PurchaseType,
		MinPurchaseQuantity:  v.Product.MinPurchaseQuantity,
		MaxPurchaseQuantity:  v.Product.MaxPurchaseQuantity,
		StockDisplayMode:     productDisplay.mode,
		StockDisplay:         productDisplay.display,
		StockRangeMin:        productDisplay.rangeMin,
		StockRangeMax:        productDisplay.rangeMax,
		StockQuantityHidden:  productDisplay.quantityHidden,
		FulfillmentType:      v.Product.FulfillmentType,
		ManualFormSchema:     v.Product.ManualFormSchemaJSON,
		ManualStockAvailable: maskPublicStockInt(mode, v.ManualStockAvailable),
		AutoStockAvailable:   maskPublicStockInt64(mode, v.AutoStockAvailable),
		StockStatus:          v.StockStatus,
		IsSoldOut:            v.IsSoldOut,
		PaymentChannelIDs:    service.DecodeChannelIDs(v.Product.PaymentChannelIDs),
		Category:             dto.NewCategoryResp(&v.Product.Category),
		SKUs:                 skus,
		PromotionID:          v.PromotionID,
		PromotionName:        v.PromotionName,
		PromotionType:        v.PromotionType,
		PromotionPriceAmount: v.PromotionPriceAmount,
		PromotionRules:       v.PromotionRules,
		MemberPrices:         v.MemberPrices,
	}
	return resp
}

func normalizePublicStockDisplayMode(raw string) string {
	switch strings.TrimSpace(raw) {
	case constants.ProductStockDisplayStatus:
		return constants.ProductStockDisplayStatus
	case constants.ProductStockDisplayRange:
		return constants.ProductStockDisplayRange
	case constants.ProductStockDisplayHidden:
		return constants.ProductStockDisplayHidden
	default:
		return constants.ProductStockDisplayExact
	}
}

func buildPublicStockDisplay(mode, status string, quantity int64) publicStockDisplayView {
	normalizedMode := normalizePublicStockDisplayMode(mode)
	normalizedStatus := normalizePublicStockStatus(status, quantity)
	view := publicStockDisplayView{
		mode:           normalizedMode,
		status:         normalizedStatus,
		display:        normalizedStatus,
		quantityHidden: normalizedMode != constants.ProductStockDisplayExact,
	}

	switch normalizedMode {
	case constants.ProductStockDisplayRange:
		if normalizedStatus == constants.ProductStockStatusInStock || normalizedStatus == constants.ProductStockStatusLowStock {
			view.display, view.rangeMin, view.rangeMax = publicStockRange(quantity)
		}
	case constants.ProductStockDisplayHidden:
		if normalizedStatus == constants.ProductStockStatusInStock || normalizedStatus == constants.ProductStockStatusLowStock {
			view.display = constants.ProductStockDisplayHidden
		}
	case constants.ProductStockDisplayExact:
		view.quantityHidden = false
		view.display = constants.ProductStockDisplayExact
	}
	return view
}

func normalizePublicStockStatus(status string, quantity int64) string {
	switch status {
	case constants.ProductStockStatusUnlimited:
		return constants.ProductStockStatusUnlimited
	case constants.ProductStockStatusOutOfStock:
		return constants.ProductStockStatusOutOfStock
	case constants.ProductStockStatusLowStock:
		return constants.ProductStockStatusLowStock
	case constants.ProductStockStatusInStock:
		return constants.ProductStockStatusInStock
	}
	switch {
	case quantity == int64(constants.ManualStockUnlimited):
		return constants.ProductStockStatusUnlimited
	case quantity <= 0:
		return constants.ProductStockStatusOutOfStock
	case quantity <= publicLowStockLimit:
		return constants.ProductStockStatusLowStock
	default:
		return constants.ProductStockStatusInStock
	}
}

func publicStockRange(quantity int64) (string, *int, *int) {
	switch {
	case quantity <= 5:
		min, max := 1, 5
		return constants.ProductStockDisplayRange1To5, &min, &max
	case quantity <= 20:
		min, max := 6, 20
		return constants.ProductStockDisplayRange6To20, &min, &max
	case quantity <= 50:
		min, max := 21, 50
		return constants.ProductStockDisplayRange21To50, &min, &max
	case quantity <= 100:
		min, max := 51, 100
		return constants.ProductStockDisplayRange51To100, &min, &max
	default:
		min := 100
		return constants.ProductStockDisplayRange100Plus, &min, nil
	}
}

func maskPublicStockInt(mode string, value int) int {
	if normalizePublicStockDisplayMode(mode) == constants.ProductStockDisplayExact {
		return value
	}
	switch {
	case value == constants.ManualStockUnlimited:
		return constants.ManualStockUnlimited
	case value <= 0:
		return 0
	default:
		return 1
	}
}

func maskPublicStockInt64(mode string, value int64) int64 {
	if normalizePublicStockDisplayMode(mode) == constants.ProductStockDisplayExact {
		return value
	}
	switch {
	case value == int64(constants.ManualStockUnlimited):
		return int64(constants.ManualStockUnlimited)
	case value <= 0:
		return 0
	default:
		return 1
	}
}

func maskPublicStockSold(mode string, value int) int {
	if normalizePublicStockDisplayMode(mode) == constants.ProductStockDisplayExact {
		return value
	}
	return 0
}

func (v *publicProductView) productStockQuantity() int64 {
	if v.StockStatus == constants.ProductStockStatusUnlimited {
		return int64(constants.ManualStockUnlimited)
	}
	fulfillmentType := strings.TrimSpace(v.Product.FulfillmentType)
	if fulfillmentType == constants.FulfillmentTypeAuto {
		return v.AutoStockAvailable
	}
	return int64(v.ManualStockAvailable)
}

func (v *publicProductView) skuStockState(sv publicSKUView) (string, int64) {
	fulfillmentType := strings.TrimSpace(v.Product.FulfillmentType)
	if fulfillmentType == "" {
		fulfillmentType = constants.FulfillmentTypeManual
	}
	var quantity int64
	if fulfillmentType == constants.FulfillmentTypeAuto {
		quantity = sv.AutoStockAvailable
	} else {
		quantity = int64(sv.ManualStockTotal)
	}
	status := normalizePublicStockStatus("", quantity)
	return status, quantity
}

// GetConfig 获取全局配置
func (h *Handler) GetConfig(c *gin.Context) {
	// 默认配置
	defaults := map[string]interface{}{
		"languages":                              append([]string(nil), constants.SupportedLocales...),
		constants.SettingFieldSiteCurrency:       constants.SiteCurrencyDefault,
		constants.SettingFieldStorefrontTemplate: constants.StorefrontTemplateDefault,
		"contact": map[string]interface{}{
			"telegram": "https://t.me/dujiaoka",
			"whatsapp": "https://wa.me/1234567890",
		},
		"scripts": make([]interface{}, 0),
	}

	tenant, _ := service.TenantFromContext(c.Request.Context())
	cacheKey := cache.PublicConfigCacheKey(tenant.ResellerID)

	var cached map[string]interface{}
	if hit, err := cache.GetJSON(c.Request.Context(), cacheKey, &cached); err == nil && hit {
		cached["server_time"] = time.Now().UnixMilli()
		cached["app_version"] = version.Version
		response.Success(c, cached)
		return
	}

	data, err := h.SettingService.GetConfig(defaults)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}

	publicChannels, err := h.PaymentService.GetAvailableChannels(service.AvailablePaymentChannelFilter{
		PaymentType: constants.PaymentTypeOrder,
	})
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	data["payment_channels"] = publicChannels

	// 钱包相关配置
	if h.SettingService != nil {
		walletRechargeChannelIDs := h.SettingService.GetWalletRechargeChannelIDs()
		if len(walletRechargeChannelIDs) > 0 {
			data["wallet_recharge_channel_ids"] = walletRechargeChannelIDs
		}
		if h.SettingService.GetWalletOnlyPayment() {
			data["wallet_only_payment"] = true
		}
	}

	if h.CaptchaService != nil {
		publicCaptcha, captchaErr := h.CaptchaService.GetPublicSetting()
		if captchaErr != nil {
			shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", captchaErr)
			return
		}
		data["captcha"] = publicCaptcha
	}
	telegramAuthConfig := map[string]interface{}{
		"enabled":      false,
		"bot_username": "",
		"mini_app_url": "",
	}
	if h.TelegramAuthService != nil {
		telegramAuthConfig = h.TelegramAuthService.PublicConfig()
	} else if h.Config != nil {
		telegramAuthConfig["enabled"] = h.Config.TelegramAuth.Enabled
		telegramAuthConfig["bot_username"] = strings.TrimSpace(h.Config.TelegramAuth.BotUsername)
		telegramAuthConfig["mini_app_url"] = strings.TrimSpace(h.Config.TelegramAuth.MiniAppURL)
	}
	data["telegram_auth"] = telegramAuthConfig

	affiliateSetting, err := h.SettingService.GetAffiliateSetting()
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	data["affiliate"] = service.AffiliateSettingToMap(affiliateSetting)

	// 邮件与注册配置
	smtpSetting, _ := h.SettingService.GetSMTPSetting(h.Config.Email)
	data["smtp_enabled"] = smtpSetting.Enabled
	registrationEnabled, _ := h.SettingService.GetRegistrationEnabled(true)
	emailVerificationEnabled, _ := h.SettingService.GetEmailVerificationEnabled(true)
	data["registration_enabled"] = registrationEnabled
	data["email_verification_enabled"] = emailVerificationEnabled
	emailDomainPolicy, policyErr := h.SettingService.GetRegistrationEmailDomainPolicy()
	if policyErr != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", policyErr)
		return
	}
	data["email_domain_allowlist_enabled"] = emailDomainPolicy.Enabled
	data["allowed_email_domains"] = emailDomainPolicy.AllowedDomains

	// 导航配置
	navConfigVal, _ := h.SettingService.GetByKey(constants.SettingKeyNavConfig)
	if navConfigVal != nil {
		data["nav_config"] = navConfigVal
	} else {
		data["nav_config"] = map[string]interface{}{
			"builtin":      map[string]interface{}{"blog": true, "notice": true, "about": true},
			"custom_items": make([]interface{}, 0),
		}
	}

	// 首页公告（仅在启用、处于排期内且内容非空时下发）
	if announcement, ok := h.SettingService.GetActiveHomeAnnouncement(); ok {
		data["announcement"] = announcement
	}

	if h.ResellerSiteConfigService != nil {
		overlaid, overlayErr := h.ResellerSiteConfigService.ApplyPublicConfigOverlay(c.Request.Context(), tenant, data)
		if overlayErr != nil {
			shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", overlayErr)
			return
		}
		data = overlaid
	} else if tenant.ResellerID == nil {
		data["tenant"] = map[string]interface{}{"mode": "main", "host": tenant.Host}
	}

	_ = cache.SetJSON(c.Request.Context(), cacheKey, data, publicConfigCacheTTL)
	data["server_time"] = time.Now().UnixMilli()
	data["app_version"] = version.Version
	response.Success(c, data)
}

// GetPublicMemberLevels 获取公共会员等级列表
func (h *Handler) GetPublicMemberLevels(c *gin.Context) {
	levels, err := h.MemberLevelService.ListActiveLevels()
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.member_level_fetch_failed", err)
		return
	}

	views := make([]dto.MemberLevelResp, 0, len(levels))
	for _, l := range levels {
		views = append(views, dto.MemberLevelResp{
			ID:                l.ID,
			Name:              l.NameJSON,
			Slug:              l.Slug,
			Icon:              l.Icon,
			DiscountRate:      l.DiscountRate.Decimal.InexactFloat64(),
			RechargeThreshold: l.RechargeThreshold.Decimal.InexactFloat64(),
			SpendThreshold:    l.SpendThreshold.Decimal.InexactFloat64(),
			IsDefault:         l.IsDefault,
			SortOrder:         l.SortOrder,
		})
	}
	response.Success(c, views)
}

// GetProducts 获取商品列表
func (h *Handler) GetProducts(c *gin.Context) {
	// 获取分页参数
	page, pageSize := shared.ParsePagination(c)

	// 获取筛选参数
	categoryID := c.Query("category_id")
	search := strings.TrimSpace(c.Query("search"))
	tenant := tenantFromRequest(c)

	products, total, err := h.ProductService.ListPublicForTenant(tenant, h.ResellerRepo, categoryID, search, page, pageSize)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	var promotionService *service.PromotionService
	if h.PromotionRepo != nil {
		promotionService = service.NewPromotionService(h.PromotionRepo)
	}

	if err := h.ProductService.ApplyAutoStockCounts(products); err != nil {
		shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	var resellerBatch *service.ResellerDisplayPricingBatch
	if isResellerTenant(tenant) {
		if h.ResellerPricingResolver == nil {
			shared.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		resellerBatch, err = h.ResellerPricingResolver.LoadDisplayPricingBatch(tenant, products)
		if err != nil {
			shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
			return
		}
	}

	decorated := make([]dto.ProductResp, 0, len(products))
	for i := range products {
		item, derr := h.decoratePublicProductForTenant(&products[i], promotionService, tenant, resellerBatch)
		if derr != nil {
			if errors.Is(derr, service.ErrResellerProductNotListed) {
				continue
			}
			shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", derr)
			return
		}
		decorated = append(decorated, item)
	}

	// 统一响应格式
	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, decorated, pagination)
}

// GetProductBySlug 根据 slug 获取商品详情
func (h *Handler) GetProductBySlug(c *gin.Context) {
	slug := c.Param("slug")
	tenant := tenantFromRequest(c)

	product, err := h.ProductService.GetPublicBySlugForTenant(tenant, h.ResellerRepo, slug)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	var promotionService *service.PromotionService
	if h.PromotionRepo != nil {
		promotionService = service.NewPromotionService(h.PromotionRepo)
	}

	temp := []models.Product{*product}
	if err := h.ProductService.ApplyAutoStockCounts(temp); err != nil {
		shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}
	*product = temp[0]

	var resellerBatch *service.ResellerDisplayPricingBatch
	if isResellerTenant(tenant) {
		if h.ResellerPricingResolver == nil {
			shared.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		resellerBatch, err = h.ResellerPricingResolver.LoadDisplayPricingBatch(tenant, []models.Product{*product})
		if err != nil {
			shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
			return
		}
	}

	decorated, derr := h.decoratePublicProductForTenant(product, promotionService, tenant, resellerBatch)
	if derr != nil {
		if errors.Is(derr, service.ErrResellerProductNotListed) {
			shared.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", derr)
		return
	}

	if posts, perr := h.PostService.ListPostsForProduct(product.ID, publicRelatedPostsLimit); perr == nil {
		decorated.RelatedPosts = dto.NewRelatedPostCardList(posts)
	}

	response.Success(c, decorated)
}

const publicRelatedPostsLimit = 6

func tenantFromRequest(c *gin.Context) service.TenantContext {
	if c != nil && c.Request != nil {
		if tenant, ok := service.TenantFromContext(c.Request.Context()); ok {
			return tenant
		}
	}
	return service.MainTenantContext("")
}

func isResellerTenant(tenant service.TenantContext) bool {
	return tenant.ResellerID != nil && !tenant.IsMain && !tenant.Unavailable
}

func isResellerDisplayHiddenError(err error) bool {
	return errors.Is(err, service.ErrResellerProductNotListed) ||
		errors.Is(err, service.ErrResellerPriceBelowBase) ||
		errors.Is(err, service.ErrResellerMarkupExceeded) ||
		errors.Is(err, service.ErrResellerPricingModeInvalid)
}

func (h *Handler) decoratePublicProduct(product *models.Product, promotionService *service.PromotionService, userMemberLevelID ...uint) (dto.ProductResp, error) {
	if product == nil {
		return dto.ProductResp{}, nil
	}

	item := publicProductView{Product: *product}
	displayPrice := resolvePublicDisplayPrice(product)
	displaySKUID := resolvePublicDisplaySKUID(product)
	item.Product.PriceAmount = displayPrice
	h.decorateProductStock(product, &item)

	// 获取所有活动规则用于前端展示
	if promotionService != nil {
		allRules, err := promotionService.GetProductPromotions(product.ID)
		if err == nil && len(allRules) > 0 {
			rules := make([]dto.PromotionRuleResp, 0, len(allRules))
			for _, r := range allRules {
				rules = append(rules, dto.PromotionRuleResp{
					ID:        r.ID,
					Name:      strings.TrimSpace(r.Name),
					Type:      strings.TrimSpace(r.Type),
					Value:     r.Value,
					MinAmount: r.MinAmount,
				})
			}
			item.PromotionRules = rules
		}
	}

	// 附加会员等级价格
	var memberLevelID uint
	if len(userMemberLevelID) > 0 {
		memberLevelID = userMemberLevelID[0]
	}
	if h.Container != nil && h.MemberLevelService != nil {
		levelPrices, _ := h.MemberLevelService.GetLevelPricesByProduct(product.ID)
		if len(levelPrices) > 0 {
			views := make([]dto.MemberLevelPrice, 0, len(levelPrices))
			for _, lp := range levelPrices {
				views = append(views, dto.MemberLevelPrice{
					MemberLevelID: lp.MemberLevelID,
					SKUID:         lp.SKUID,
					PriceAmount:   lp.PriceAmount,
				})
			}
			item.MemberPrices = views
		}
	}

	// 构建 SKU 列表并为每个 active SKU 计算促销价
	skuViews := make([]publicSKUView, 0, len(item.Product.SKUs))
	var displayPromotion *models.Promotion
	var displayPromotionPrice *models.Money

	for _, sku := range item.Product.SKUs {
		sv := publicSKUView{ProductSKU: sku}

		// 计算当前用户的会员价
		if memberLevelID > 0 && h.Container != nil && h.MemberLevelService != nil && sku.IsActive {
			memberPrice, _ := h.MemberLevelService.ResolveMemberPrice(memberLevelID, product.ID, sku.ID, sku.PriceAmount.Decimal)
			if memberPrice.LessThan(sku.PriceAmount.Decimal) {
				mp := models.NewMoneyFromDecimal(memberPrice)
				sv.MemberPriceAmount = &mp
			}
		}

		if promotionService != nil && sku.IsActive {
			priceCarrier := *product
			priceCarrier.PriceAmount = sku.PriceAmount
			promotion, discountedPrice, err := promotionService.ApplyPromotion(&priceCarrier, 1)
			if err != nil && !errors.Is(err, service.ErrPromotionInvalid) {
				return dto.ProductResp{}, err
			}
			if promotion != nil && discountedPrice.Decimal.LessThan(sku.PriceAmount.Decimal) {
				sv.PromotionPriceAmount = &discountedPrice
				if displaySKUID != 0 && sku.ID == displaySKUID {
					displayPromotion = promotion
					cp := discountedPrice
					displayPromotionPrice = &cp
				}
			}
		}
		skuViews = append(skuViews, sv)
	}

	item.PublicSKUs = skuViews

	// 产品级促销信息与展示价保持同一口径，避免列表价与活动价来自不同 SKU。
	if displayPromotion != nil && displayPromotionPrice != nil {
		promotionID := displayPromotion.ID
		item.PromotionID = &promotionID
		item.PromotionName = strings.TrimSpace(displayPromotion.Name)
		item.PromotionType = strings.TrimSpace(displayPromotion.Type)
		item.PromotionPriceAmount = displayPromotionPrice
	}

	return item.toProductResp(), nil
}

func (h *Handler) decoratePublicProductForTenant(
	product *models.Product,
	promotionService *service.PromotionService,
	tenant service.TenantContext,
	resellerBatch *service.ResellerDisplayPricingBatch,
	userMemberLevelID ...uint,
) (dto.ProductResp, error) {
	if !isResellerTenant(tenant) {
		return h.decoratePublicProduct(product, promotionService, userMemberLevelID...)
	}
	if product == nil {
		return dto.ProductResp{}, nil
	}
	if h == nil || h.ResellerPricingResolver == nil {
		return dto.ProductResp{}, service.ErrResellerProductNotListed
	}
	display, err := h.ResellerPricingResolver.ResolveDisplayPrices(tenant, product, resellerBatch)
	if err != nil {
		if isResellerDisplayHiddenError(err) {
			return dto.ProductResp{}, service.ErrResellerProductNotListed
		}
		return dto.ProductResp{}, err
	}
	if display == nil || !display.Visible {
		return dto.ProductResp{}, service.ErrResellerProductNotListed
	}

	productCopy := *product
	productCopy.PriceAmount = display.DisplayPrice
	productCopy.WholesalePrices = nil
	filteredSKUs := make([]models.ProductSKU, 0, len(product.SKUs))
	for _, sku := range product.SKUs {
		if !sku.IsActive || display.HiddenSKUIDs[sku.ID] {
			continue
		}
		if price, ok := display.SKUPrices[sku.ID]; ok {
			sku.PriceAmount = price
		}
		filteredSKUs = append(filteredSKUs, sku)
	}
	if len(product.SKUs) > 0 && len(filteredSKUs) == 0 {
		return dto.ProductResp{}, service.ErrResellerProductNotListed
	}
	productCopy.SKUs = filteredSKUs

	item := publicProductView{Product: productCopy}
	h.decorateProductStock(&productCopy, &item)
	skuViews := make([]publicSKUView, 0, len(productCopy.SKUs))
	for _, sku := range productCopy.SKUs {
		skuViews = append(skuViews, publicSKUView{ProductSKU: sku})
	}
	item.PublicSKUs = skuViews
	return item.toProductResp(), nil
}

func resolvePublicDisplayPrice(product *models.Product) models.Money {
	if product == nil {
		return models.Money{}
	}
	for _, sku := range product.SKUs {
		if !sku.IsActive {
			continue
		}
		return sku.PriceAmount
	}
	return product.PriceAmount
}

func resolvePublicDisplaySKUID(product *models.Product) uint {
	if product == nil {
		return 0
	}
	for _, sku := range product.SKUs {
		if !sku.IsActive {
			continue
		}
		return sku.ID
	}
	return 0
}

func (h *Handler) decorateProductStock(product *models.Product, item *publicProductView) {
	if product == nil || item == nil {
		return
	}

	stockStatus := constants.ProductStockStatusInStock
	manualAvailable := 0

	item.ManualStockAvailable = manualAvailable
	item.AutoStockTotal = 0
	item.AutoStockLocked = 0
	item.AutoStockSold = 0
	item.AutoStockAvailable = 0
	item.StockStatus = stockStatus
	item.IsSoldOut = false

	fulfillmentType := strings.TrimSpace(product.FulfillmentType)
	if fulfillmentType == "" {
		fulfillmentType = constants.FulfillmentTypeManual
	}

	// upstream 类型：根据 SKU 映射中的上游库存判断
	if fulfillmentType == constants.FulfillmentTypeUpstream {
		h.decorateUpstreamStock(product, item)
		return
	}

	if fulfillmentType == constants.FulfillmentTypeManual {
		hasActiveSKU := false
		hasUnlimitedSKU := false
		skuRemaining := 0
		for _, sku := range product.SKUs {
			if !sku.IsActive {
				continue
			}
			hasActiveSKU = true
			if sku.ManualStockTotal == constants.ManualStockUnlimited {
				hasUnlimitedSKU = true
				continue
			}
			if sku.ManualStockTotal > 0 {
				skuRemaining += sku.ManualStockTotal
			}
		}
		if hasActiveSKU {
			if hasUnlimitedSKU {
				item.ManualStockAvailable = constants.ManualStockUnlimited
				item.StockStatus = constants.ProductStockStatusUnlimited
				item.IsSoldOut = false
				return
			}
			manualAvailable = skuRemaining
		} else if product.ManualStockTotal == constants.ManualStockUnlimited {
			item.ManualStockAvailable = constants.ManualStockUnlimited
			item.StockStatus = constants.ProductStockStatusUnlimited
			item.IsSoldOut = false
			return
		} else {
			manualAvailable = product.ManualStockTotal
			if manualAvailable < 0 {
				manualAvailable = 0
			}
		}
		item.ManualStockAvailable = manualAvailable

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
	autoTotal := int64(0)
	autoLocked := int64(0)
	autoSold := int64(0)
	for _, sku := range product.SKUs {
		if !sku.IsActive {
			continue
		}
		autoAvailable += sku.AutoStockAvailable
		autoTotal += sku.AutoStockTotal
		autoLocked += sku.AutoStockLocked
		autoSold += sku.AutoStockSold
	}
	item.AutoStockAvailable = autoAvailable
	item.AutoStockTotal = autoTotal
	item.AutoStockLocked = autoLocked
	item.AutoStockSold = autoSold

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

// decorateUpstreamStock 根据 SKU 映射的上游库存信息填充商品及 SKU 级库存状态
func (h *Handler) decorateUpstreamStock(product *models.Product, item *publicProductView) {
	// 通过本地商品 ID 查找 product mapping
	mapping, err := h.ProductMappingRepo.GetByLocalProductID(product.ID)
	if err != nil || mapping == nil {
		// 没有映射记录，降级为显示有库存（避免误售罄）
		item.Product.FulfillmentType = constants.FulfillmentTypeManual
		item.StockStatus = constants.ProductStockStatusInStock
		item.IsSoldOut = false
		return
	}

	// 根据上游原始交付类型设置展示类型：auto 还是 manual
	displayType := mapping.UpstreamFulfillmentType
	if displayType != constants.FulfillmentTypeAuto {
		displayType = constants.FulfillmentTypeManual
	}
	item.Product.FulfillmentType = displayType

	// 获取该映射下的所有 SKU 映射
	skuMappings, err := h.SKUMappingRepo.ListByProductMapping(mapping.ID)
	if err != nil || len(skuMappings) == 0 {
		item.StockStatus = constants.ProductStockStatusInStock
		item.IsSoldOut = false
		return
	}

	// 按本地 SKU ID 索引映射
	skuMappingByLocal := make(map[uint]*models.SKUMapping, len(skuMappings))
	for i := range skuMappings {
		skuMappingByLocal[skuMappings[i].LocalSKUID] = &skuMappings[i]
	}

	// 填充每个 SKU 的上游库存，同时汇总商品级状态
	hasUnlimited := false
	totalStock := 0
	hasActiveMapping := false

	for i := range item.Product.SKUs {
		sku := &item.Product.SKUs[i]
		sm, ok := skuMappingByLocal[sku.ID]
		if !ok || !sm.UpstreamIsActive {
			sku.UpstreamStock = 0
			continue
		}
		hasActiveMapping = true
		sku.UpstreamStock = sm.UpstreamStock

		// 根据展示类型填充对应的库存字段，让前端详情页的库存判断逻辑正确工作
		if displayType == constants.FulfillmentTypeAuto {
			if sm.UpstreamStock == -1 {
				sku.AutoStockAvailable = -1 // 前端对 auto 类型 -1 不做特殊处理，但总量为负时不限购
			} else {
				sku.AutoStockAvailable = int64(sm.UpstreamStock)
			}
		} else {
			if sm.UpstreamStock == -1 {
				sku.ManualStockTotal = constants.ManualStockUnlimited
			} else {
				sku.ManualStockTotal = sm.UpstreamStock
			}
		}

		if sm.UpstreamStock == -1 {
			hasUnlimited = true
		} else {
			totalStock += sm.UpstreamStock
		}
	}

	if !hasActiveMapping {
		item.StockStatus = constants.ProductStockStatusOutOfStock
		item.IsSoldOut = true
		return
	}

	if hasUnlimited {
		if displayType == constants.FulfillmentTypeAuto {
			item.AutoStockAvailable = -1
		} else {
			item.ManualStockAvailable = constants.ManualStockUnlimited
		}
		item.StockStatus = constants.ProductStockStatusUnlimited
		item.IsSoldOut = false
		return
	}

	if displayType == constants.FulfillmentTypeAuto {
		item.AutoStockAvailable = int64(totalStock)
	} else {
		item.ManualStockAvailable = totalStock
	}

	switch {
	case totalStock <= 0:
		item.StockStatus = constants.ProductStockStatusOutOfStock
		item.IsSoldOut = true
	case totalStock <= publicLowStockLimit:
		item.StockStatus = constants.ProductStockStatusLowStock
	default:
		item.StockStatus = constants.ProductStockStatusInStock
	}
}

// GetPosts 获取文章/公告列表
func (h *Handler) GetPosts(c *gin.Context) {
	// 获取分页参数
	page, pageSize := shared.ParsePagination(c)

	// 获取类型参数
	postType := c.Query("type") // blog 或 notice
	search := c.Query("search")

	posts, total, err := h.PostService.ListPublic(postType, search, page, pageSize)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.post_fetch_failed", err)
		return
	}

	// 统一响应格式
	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, dto.NewPostRespList(posts), pagination)
}

// GetPostBySlug 根据 slug 获取文章详情
func (h *Handler) GetPostBySlug(c *gin.Context) {
	slug := c.Param("slug")

	post, err := h.PostService.GetPublicBySlug(slug)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.post_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.post_fetch_failed", err)
		return
	}

	resp := dto.NewPostResp(post)
	if post.Type == constants.PostTypeBlog {
		products, perr := h.PostService.ListRelatedProducts(post.ID)
		if perr == nil {
			resp.RelatedProducts = dto.NewRelatedProductCardList(products)
		}
	}
	response.Success(c, resp)
}

// GetCategories 获取分类列表
func (h *Handler) GetCategories(c *gin.Context) {
	categories, err := h.CategoryService.ListActive()
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.category_fetch_failed", err)
		return
	}

	response.Success(c, dto.NewCategoryRespList(categories))
}

// CreateGuestOrderRequest 游客下单请求
type CreateGuestOrderRequest struct {
	Email               string                       `json:"email" binding:"required"`
	OrderPassword       string                       `json:"order_password" binding:"required"`
	Items               []OrderItemRequest           `json:"items" binding:"required"`
	CouponCode          string                       `json:"coupon_code"`
	AffiliateCode       string                       `json:"affiliate_code"`
	AffiliateVisitorKey string                       `json:"affiliate_visitor_key"`
	ManualFormData      map[string]models.JSON       `json:"manual_form_data"`
	CaptchaPayload      shared.CaptchaPayloadRequest `json:"captcha_payload"`
}

// CreateGuestOrder 游客创建订单
func (h *Handler) CreateGuestOrder(c *gin.Context) {
	var req CreateGuestOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	if h.CaptchaService != nil {
		if captchaErr := h.CaptchaService.Verify(constants.CaptchaSceneGuestCreateOrder, req.CaptchaPayload.ToServicePayload(), c.ClientIP()); captchaErr != nil {
			switch {
			case errors.Is(captchaErr, service.ErrCaptchaRequired):
				shared.RespondError(c, response.CodeBadRequest, "error.captcha_required", nil)
				return
			case errors.Is(captchaErr, service.ErrCaptchaInvalid):
				shared.RespondError(c, response.CodeBadRequest, "error.captcha_invalid", nil)
				return
			case errors.Is(captchaErr, service.ErrCaptchaConfigInvalid):
				shared.RespondError(c, response.CodeInternal, "error.captcha_config_invalid", captchaErr)
				return
			default:
				shared.RespondError(c, response.CodeInternal, "error.captcha_verify_failed", captchaErr)
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
		Email:               req.Email,
		OrderPassword:       req.OrderPassword,
		Locale:              i18n.ResolveLocale(c),
		Tenant:              tenantFromRequest(c),
		Items:               items,
		CouponCode:          req.CouponCode,
		AffiliateCode:       req.AffiliateCode,
		AffiliateVisitorKey: req.AffiliateVisitorKey,
		ClientIP:            c.ClientIP(),
		ManualFormData:      req.ManualFormData,
	})
	if err != nil {
		respondGuestOrderCreateError(c, err)
		return
	}
	orderDetail := dto.NewOrderDetail(order)
	h.enrichOrderWithAllowedChannels(order, &orderDetail)
	response.Success(c, orderDetail)
}

// CreateGuestOrderAndPayRequest 游客创建订单并发起支付请求
type CreateGuestOrderAndPayRequest struct {
	Email               string                       `json:"email" binding:"required"`
	OrderPassword       string                       `json:"order_password" binding:"required"`
	Items               []OrderItemRequest           `json:"items" binding:"required"`
	CouponCode          string                       `json:"coupon_code"`
	AffiliateCode       string                       `json:"affiliate_code"`
	AffiliateVisitorKey string                       `json:"affiliate_visitor_key"`
	ManualFormData      map[string]models.JSON       `json:"manual_form_data"`
	CaptchaPayload      shared.CaptchaPayloadRequest `json:"captcha_payload"`
	ChannelID           uint                         `json:"channel_id"`
}

// CreateGuestOrderAndPay 游客创建订单并发起支付（合并接口）
func (h *Handler) CreateGuestOrderAndPay(c *gin.Context) {
	var req CreateGuestOrderAndPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	if h.CaptchaService != nil {
		if captchaErr := h.CaptchaService.Verify(constants.CaptchaSceneGuestCreateOrder, req.CaptchaPayload.ToServicePayload(), c.ClientIP()); captchaErr != nil {
			switch {
			case errors.Is(captchaErr, service.ErrCaptchaRequired):
				shared.RespondError(c, response.CodeBadRequest, "error.captcha_required", nil)
				return
			case errors.Is(captchaErr, service.ErrCaptchaInvalid):
				shared.RespondError(c, response.CodeBadRequest, "error.captcha_invalid", nil)
				return
			case errors.Is(captchaErr, service.ErrCaptchaConfigInvalid):
				shared.RespondError(c, response.CodeInternal, "error.captcha_config_invalid", captchaErr)
				return
			default:
				shared.RespondError(c, response.CodeInternal, "error.captcha_verify_failed", captchaErr)
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
		Email:               req.Email,
		OrderPassword:       req.OrderPassword,
		Locale:              i18n.ResolveLocale(c),
		Tenant:              tenantFromRequest(c),
		Items:               items,
		CouponCode:          req.CouponCode,
		AffiliateCode:       req.AffiliateCode,
		AffiliateVisitorKey: req.AffiliateVisitorKey,
		ClientIP:            c.ClientIP(),
		ManualFormData:      req.ManualFormData,
	})
	if err != nil {
		respondGuestOrderCreateError(c, err)
		return
	}
	orderResp := dto.NewOrderDetail(order)
	h.enrichOrderWithAllowedChannels(order, &orderResp)

	// 如果未指定支付渠道，仅返回订单
	if req.ChannelID == 0 {
		response.Success(c, gin.H{
			"order":    orderResp,
			"order_no": order.OrderNo,
		})
		return
	}

	result, err := h.PaymentService.CreatePayment(service.CreatePaymentInput{
		OrderID:    order.ID,
		ChannelID:  req.ChannelID,
		UseBalance: false,
		ClientIP:   c.ClientIP(),
		Context:    c.Request.Context(),
	})
	if err != nil {
		resp := gin.H{
			"order":         orderResp,
			"order_no":      order.OrderNo,
			"payment_error": err.Error(),
		}
		response.Success(c, resp)
		return
	}

	resp := gin.H{
		"order":              orderResp,
		"order_no":           order.OrderNo,
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
		if info := dto.ExtractCryptoWalletInfo(result.Payment.ProviderType, result.Payment.InteractionMode, result.Payment.ProviderPayload); info.HasAny() {
			if info.Address != "" {
				resp["wallet_address"] = info.Address
			}
			if info.ChainAmount != "" {
				resp["chain_amount"] = info.ChainAmount
			}
			if info.Chain != "" {
				resp["chain"] = info.Chain
			}
			if info.TokenID != "" {
				resp["token_id"] = info.TokenID
			}
		}
		resp["expires_at"] = result.Payment.ExpiredAt
	}
	response.Success(c, resp)
}

// PreviewGuestOrder 游客订单金额预览
func (h *Handler) PreviewGuestOrder(c *gin.Context) {
	var req CreateGuestOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
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
		Email:               req.Email,
		OrderPassword:       req.OrderPassword,
		Locale:              i18n.ResolveLocale(c),
		Tenant:              tenantFromRequest(c),
		Items:               items,
		CouponCode:          req.CouponCode,
		AffiliateCode:       req.AffiliateCode,
		AffiliateVisitorKey: req.AffiliateVisitorKey,
		ClientIP:            c.ClientIP(),
		ManualFormData:      req.ManualFormData,
	})
	if err != nil {
		respondGuestOrderPreviewError(c, err)
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
		shared.RespondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}

	if orderNo != "" {
		order, err := h.OrderService.GetOrderByGuestOrderNoForTenant(tenantFromRequest(c), orderNo, email, password)
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
			shared.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
			return
		}
		pagination := response.Pagination{
			Page:      1,
			PageSize:  1,
			Total:     1,
			TotalPage: 1,
		}
		response.SuccessWithPage(c, dto.NewOrderSummaryList([]models.Order{*order}), pagination)
		return
	}

	page, pageSize := shared.ParsePagination(c)

	orders, total, err := h.OrderService.ListOrdersByGuestForTenant(tenantFromRequest(c), email, password, page, pageSize)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, dto.NewOrderSummaryList(orders), pagination)
}

// GetGuestOrderByOrderNo 按订单号获取游客订单详情
func (h *Handler) GetGuestOrderByOrderNo(c *gin.Context) {
	email := strings.TrimSpace(c.Query("email"))
	password := strings.TrimSpace(c.Query("order_password"))
	if email == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}
	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	order, err := h.OrderService.GetOrderByGuestOrderNoForTenant(tenantFromRequest(c), orderNo, email, password)
	if err != nil {
		if errors.Is(err, service.ErrGuestOrderNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	orderDetail := dto.NewOrderDetailTruncated(order)
	h.enrichOrderWithAllowedChannels(order, &orderDetail)
	h.enrichOrderWithRefundRecords(order, &orderDetail)
	response.Success(c, orderDetail)
}

// DownloadGuestFulfillment 下载订单交付内容（游客）
// 支持父订单或子订单的 order_no
func (h *Handler) DownloadGuestFulfillment(c *gin.Context) {
	email := strings.TrimSpace(c.Query("email"))
	password := strings.TrimSpace(c.Query("order_password"))
	if email == "" || password == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	order, err := h.OrderService.GetAnyOrderByGuestOrderNoForTenant(tenantFromRequest(c), orderNo, email, password)
	if err != nil {
		if errors.Is(err, service.ErrGuestOrderNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	if order == nil {
		shared.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
		return
	}
	respondFulfillmentDownload(c, order)
}

// CreateGuestPaymentRequest 游客发起支付请求
type CreateGuestPaymentRequest struct {
	Email         string `json:"email" binding:"required"`
	OrderPassword string `json:"order_password" binding:"required"`
	OrderNo       string `json:"order_no" binding:"required"`
	ChannelID     uint   `json:"channel_id" binding:"required"`
}

type LatestGuestPaymentQuery struct {
	Email         string `form:"email" binding:"required"`
	OrderPassword string `form:"order_password" binding:"required"`
	OrderNo       string `form:"order_no" binding:"required"`
}

// CreateGuestPayment 游客发起支付
func (h *Handler) CreateGuestPayment(c *gin.Context) {
	var req CreateGuestPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.OrderPassword)
	if email == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}
	guestOrder, err := h.OrderService.GetOrderByGuestOrderNoForTenant(tenantFromRequest(c), req.OrderNo, email, password)
	if err != nil {
		if errors.Is(err, service.ErrGuestOrderNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	result, err := h.PaymentService.CreatePayment(service.CreatePaymentInput{
		OrderID:    guestOrder.ID,
		ChannelID:  req.ChannelID,
		UseBalance: false,
		ClientIP:   c.ClientIP(),
		Context:    c.Request.Context(),
	})
	if err != nil {
		respondPaymentCreateError(c, err)
		return
	}
	response.Success(c, dto.NewCreatePaymentResp(result))
}

// CaptureGuestPaymentRequest 游客捕获支付请求。
type CaptureGuestPaymentRequest struct {
	Email         string `json:"email" binding:"required"`
	OrderPassword string `json:"order_password" binding:"required"`
}

// SelectGuestPaymentMethodRequest 游客选择二维码支付的币种和网络。
type SelectGuestPaymentMethodRequest struct {
	Email         string `json:"email" binding:"required"`
	OrderPassword string `json:"order_password" binding:"required"`
	Currency      string `json:"currency" binding:"required"`
	Network       string `json:"network" binding:"required"`
}

// SelectGuestPaymentMethod 为游客订单选择链上付款方式。
func (h *Handler) SelectGuestPaymentMethod(c *gin.Context) {
	paymentID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.payment_invalid", nil)
		return
	}
	var req SelectGuestPaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	payment, err := h.PaymentService.GetPayment(paymentID)
	if err != nil {
		respondPaymentCaptureError(c, err)
		return
	}
	if payment.OrderID == 0 {
		shared.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
		return
	}
	order, err := h.OrderService.GetOrderByGuestForTenant(
		tenantFromRequest(c),
		payment.OrderID,
		strings.TrimSpace(req.Email),
		strings.TrimSpace(req.OrderPassword),
	)
	if err != nil {
		if errors.Is(err, service.ErrGuestOrderNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	updated, err := h.PaymentService.SelectPaymentMethod(service.SelectPaymentMethodInput{
		PaymentID: paymentID,
		Currency:  req.Currency,
		Network:   req.Network,
		Context:   c.Request.Context(),
	})
	if err != nil {
		respondPaymentCaptureError(c, err)
		return
	}
	response.Success(c, dto.NewLatestPaymentResp(updated, order.OrderNo))
}

// CaptureGuestPayment 游客捕获支付。
func (h *Handler) CaptureGuestPayment(c *gin.Context) {
	paymentID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.payment_invalid", nil)
		return
	}
	var req CaptureGuestPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.OrderPassword)
	if email == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}

	payment, err := h.PaymentService.GetPayment(paymentID)
	if err != nil {
		if errors.Is(err, service.ErrPaymentNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.payment_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}
	if _, err := h.OrderService.GetOrderByGuestForTenant(tenantFromRequest(c), payment.OrderID, email, password); err != nil {
		if errors.Is(err, service.ErrGuestOrderNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}

	updated, err := h.PaymentService.CapturePayment(service.CapturePaymentInput{
		PaymentID: paymentID,
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
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	email := strings.TrimSpace(query.Email)
	password := strings.TrimSpace(query.OrderPassword)
	if email == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}

	order, err := h.OrderService.GetOrderByGuestOrderNoForTenant(tenantFromRequest(c), query.OrderNo, email, password)
	if err != nil {
		if errors.Is(err, service.ErrGuestOrderNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	if order.ParentID != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.payment_invalid", nil)
		return
	}
	if order.Status != constants.OrderStatusPendingPayment {
		shared.RespondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
		return
	}
	if order.ExpiresAt != nil && !order.ExpiresAt.After(time.Now()) {
		shared.RespondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
		return
	}

	payment, err := h.PaymentRepo.GetLatestPendingByOrder(order.ID, time.Now())
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}
	if payment == nil {
		shared.RespondError(c, response.CodeNotFound, "error.payment_not_found", nil)
		return
	}

	response.Success(c, dto.NewLatestPaymentResp(payment, order.OrderNo))
}

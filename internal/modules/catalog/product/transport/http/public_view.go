package producthttp

import (
	"errors"
	"strings"

	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productpresenter "github.com/dujiao-next/internal/modules/catalog/product/transport/presenter"

	"github.com/dujiao-next/internal/constants"
	domaincatalog "github.com/dujiao-next/internal/modules/catalog"
	categorypresenter "github.com/dujiao-next/internal/modules/catalog/category/transport/presenter"
	promotioncontract "github.com/dujiao-next/internal/modules/promotion/contract"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/shared/money"
)

// publicSKUView 内部 SKU 计算结构，用于装饰逻辑
type publicSKUView struct {
	productdomain.ProductSKU
	PromotionPriceAmount *money.Amount
	MemberPriceAmount    *money.Amount
}

// publicProductView 内部商品计算结构，装饰完成后转换为 productpresenter.Product
type publicProductView struct {
	productdomain.Product
	PromotionID          *uint
	PromotionName        string
	PromotionType        string
	PromotionPriceAmount *money.Amount
	PromotionRules       []productpresenter.PromotionRule
	MemberPrices         []productpresenter.MemberLevelPrice
	PublicSKUs           []publicSKUView
	ManualStockAvailable int
	AutoStockAvailable   int64
	StockStatus          string
	IsSoldOut            bool
}

// toProductResp 将内部计算结构转换为公共 DTO
func (v *publicProductView) toProductResp() productpresenter.Product {
	mode := domaincatalog.NormalizeStockDisplayMode(v.Product.StockDisplayMode)
	productQuantity := v.productStockQuantity()
	productDisplay := domaincatalog.StorefrontStockPolicy().Display(mode, v.StockStatus, productQuantity)

	skus := make([]productpresenter.SKU, 0, len(v.PublicSKUs))
	for _, sv := range v.PublicSKUs {
		skuStatus, skuQuantity := v.skuStockState(sv)
		skuDisplay := domaincatalog.StorefrontStockPolicy().Display(mode, skuStatus, skuQuantity)
		skus = append(skus, productpresenter.SKU{
			ID:                   sv.ID,
			SKUCode:              sv.SKUCode,
			SpecValues:           sv.SpecValuesJSON,
			PriceAmount:          sv.PriceAmount,
			ManualStockTotal:     domaincatalog.MaskStockInt(mode, sv.ManualStockTotal),
			ManualStockSold:      domaincatalog.MaskSoldCount(mode, sv.ManualStockSold),
			AutoStockAvailable:   domaincatalog.MaskStockInt64(mode, sv.AutoStockAvailable),
			UpstreamStock:        domaincatalog.MaskStockInt(mode, sv.UpstreamStock),
			StockStatus:          skuStatus,
			StockDisplayMode:     skuDisplay.Mode,
			StockDisplay:         skuDisplay.Display,
			StockRangeMin:        skuDisplay.RangeMin,
			StockRangeMax:        skuDisplay.RangeMax,
			StockQuantityHidden:  skuDisplay.QuantityHidden,
			IsSoldOut:            skuStatus == constants.ProductStockStatusOutOfStock,
			IsActive:             sv.IsActive,
			PromotionPriceAmount: sv.PromotionPriceAmount,
			MemberPriceAmount:    sv.MemberPriceAmount,
		})
	}

	resp := productpresenter.Product{
		ID:                   v.Product.ID,
		CategoryID:           v.Product.CategoryID,
		Slug:                 v.Product.Slug,
		SeoMeta:              v.Product.SeoMetaJSON,
		Title:                v.Product.TitleJSON,
		Description:          v.Product.DescriptionJSON,
		Content:              v.Product.ContentJSON,
		Compare:              v.Product.CompareJSON,
		Faq:                  v.Product.FaqJSON,
		InBox:                v.Product.InBoxJSON,
		PriceAmount:          v.Product.PriceAmount,
		WholesalePrices:      productpresenter.WholesalePrices(v.Product.WholesalePrices),
		Images:               v.Product.Images,
		Tags:                 v.Product.Tags,
		PurchaseType:         v.Product.PurchaseType,
		MinPurchaseQuantity:  v.Product.MinPurchaseQuantity,
		MaxPurchaseQuantity:  v.Product.MaxPurchaseQuantity,
		StockDisplayMode:     productDisplay.Mode,
		StockDisplay:         productDisplay.Display,
		StockRangeMin:        productDisplay.RangeMin,
		StockRangeMax:        productDisplay.RangeMax,
		StockQuantityHidden:  productDisplay.QuantityHidden,
		FulfillmentType:      v.Product.FulfillmentType,
		ManualFormSchema:     v.Product.ManualFormSchemaJSON,
		ManualStockAvailable: domaincatalog.MaskStockInt(mode, v.ManualStockAvailable),
		AutoStockAvailable:   domaincatalog.MaskStockInt64(mode, v.AutoStockAvailable),
		StockStatus:          v.StockStatus,
		IsSoldOut:            v.IsSoldOut,
		PaymentChannelIDs:    productdomain.DecodePaymentChannelIDs(v.Product.PaymentChannelIDs),
		Category:             categorypresenter.New(&v.Product.Category),
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

func (v *publicProductView) productStockQuantity() int64 {
	if v.StockStatus == constants.ProductStockStatusUnlimited {
		return int64(constants.ManualStockUnlimited)
	}
	return domaincatalog.StockQuantity(v.Product.FulfillmentType, v.AutoStockAvailable, v.ManualStockAvailable)
}

func (v *publicProductView) skuStockState(sv publicSKUView) (string, int64) {
	fulfillmentType := strings.TrimSpace(v.Product.FulfillmentType)
	if fulfillmentType == "" {
		fulfillmentType = constants.FulfillmentTypeManual
	}
	quantity := domaincatalog.StockQuantity(fulfillmentType, sv.AutoStockAvailable, sv.ManualStockTotal)
	status := domaincatalog.StorefrontStockPolicy().Status(quantity)
	return status, quantity
}

func isResellerDisplayHiddenError(err error) bool {
	return errors.Is(err, productcontract.ErrResellerProductNotListed) ||
		errors.Is(err, reseller.ErrPriceBelowBase) ||
		errors.Is(err, reseller.ErrMarkupExceeded) ||
		errors.Is(err, reseller.ErrPricingModeInvalid)
}

func (h *PublicHandler) decoratePublicProduct(product *productdomain.Product, promotions ProductPromotionDecorator, userMemberLevelID ...uint) (productpresenter.Product, error) {
	if product == nil {
		return productpresenter.Product{}, nil
	}

	item := publicProductView{Product: *product}
	displayPrice := resolvePublicDisplayPrice(product)
	displaySKUID := resolvePublicDisplaySKUID(product)
	item.Product.PriceAmount = displayPrice
	h.decorateProductStock(product, &item)

	// 获取所有活动规则用于前端展示
	if promotions != nil {
		allRules, err := promotions.GetProductPromotions(product.ID)
		if err == nil && len(allRules) > 0 {
			rules := make([]productpresenter.PromotionRule, 0, len(allRules))
			for _, r := range allRules {
				rules = append(rules, productpresenter.PromotionRule{
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
	if h.memberLevels != nil {
		levelPrices, _ := h.memberLevels.GetLevelPricesByProduct(product.ID)
		if len(levelPrices) > 0 {
			views := make([]productpresenter.MemberLevelPrice, 0, len(levelPrices))
			for _, lp := range levelPrices {
				views = append(views, productpresenter.MemberLevelPrice{
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
	var displayPromotion *promotiondomain.Promotion
	var displayPromotionPrice *money.Amount

	for _, sku := range item.Product.SKUs {
		sv := publicSKUView{ProductSKU: sku}

		// 计算当前用户的会员价
		if memberLevelID > 0 && h.memberLevels != nil && sku.IsActive {
			memberPrice, _ := h.memberLevels.ResolveMemberPrice(memberLevelID, product.ID, sku.ID, sku.PriceAmount.Decimal)
			if memberPrice.LessThan(sku.PriceAmount.Decimal) {
				mp := money.FromDecimal(memberPrice)
				sv.MemberPriceAmount = &mp
			}
		}

		if promotions != nil && sku.IsActive {
			priceCarrier := *product
			priceCarrier.PriceAmount = sku.PriceAmount
			promotion, discountedPrice, err := promotions.ApplyPromotion(&priceCarrier, 1)
			if err != nil && !errors.Is(err, promotioncontract.ErrInvalid) {
				return productpresenter.Product{}, err
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

func (h *PublicHandler) decoratePublicProductForTenant(
	product *productdomain.Product,
	promotions ProductPromotionDecorator,
	tenant reseller.TenantContext,
	resellerBatch *reseller.DisplayPricingBatch,
	userMemberLevelID ...uint,
) (productpresenter.Product, error) {
	if !isResellerTenant(tenant) {
		return h.decoratePublicProduct(product, promotions, userMemberLevelID...)
	}
	if product == nil {
		return productpresenter.Product{}, nil
	}
	if h == nil || h.pricer == nil {
		return productpresenter.Product{}, productcontract.ErrResellerProductNotListed
	}
	display, err := h.pricer.ResolveDisplayPrices(tenant, product, resellerBatch)
	if err != nil {
		if isResellerDisplayHiddenError(err) {
			return productpresenter.Product{}, productcontract.ErrResellerProductNotListed
		}
		return productpresenter.Product{}, err
	}
	if display == nil || !display.Visible {
		return productpresenter.Product{}, productcontract.ErrResellerProductNotListed
	}

	productCopy := *product
	productCopy.PriceAmount = display.DisplayPrice
	productCopy.WholesalePrices = nil
	filteredSKUs := make([]productdomain.ProductSKU, 0, len(product.SKUs))
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
		return productpresenter.Product{}, productcontract.ErrResellerProductNotListed
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

func resolvePublicDisplayPrice(product *productdomain.Product) money.Amount {
	if product == nil {
		return money.Amount{}
	}
	for _, sku := range product.SKUs {
		if !sku.IsActive {
			continue
		}
		return sku.PriceAmount
	}
	return product.PriceAmount
}

func resolvePublicDisplaySKUID(product *productdomain.Product) uint {
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

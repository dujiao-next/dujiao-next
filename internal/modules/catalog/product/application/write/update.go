package productwrite

import (
	"strings"

	"github.com/dujiao-next/internal/constants"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/modules/catalog/product/manualform"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// Update 更新商品
func (s *WriteService) Update(id string, input CreateProductInput) (*productdomain.Product, error) {
	priceAmount := input.PriceAmount.Round(2)
	if len(input.SKUs) == 0 && priceAmount.LessThanOrEqual(decimal.Zero) {
		return nil, productcontract.ErrProductPriceInvalid
	}
	product, err := s.products.GetByID(id)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, productcontract.ErrNotFound
	}
	if err := productdomain.ValidateCategoryAssignment(s.categories, input.CategoryID, product.CategoryID, productcontract.ErrProductCategoryInvalid); err != nil {
		return nil, err
	}

	count, err := s.products.CountBySlug(input.Slug, &id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, productcontract.ErrSlugExists
	}

	product.CategoryID = input.CategoryID
	product.Slug = input.Slug
	product.SeoMetaJSON = jsonmap.JSON(input.SeoMetaJSON)
	product.TitleJSON = jsonmap.JSON(input.TitleJSON)
	product.DescriptionJSON = jsonmap.JSON(input.DescriptionJSON)
	product.ContentJSON = jsonmap.JSON(input.ContentJSON)
	product.InstructionsJSON = jsonmap.JSON(input.InstructionsJSON)
	// 对接商品的交付表单由上游同步维护，后台编辑不得清空。
	if !product.IsMapped {
		product.ManualFormSchemaJSON = jsonmap.JSON{}
	}
	product.PriceAmount = money.FromDecimal(priceAmount)
	product.SortOrder = input.SortOrder
	product.Images = jsonslice.Strings(input.Images)
	product.Tags = jsonslice.Strings(input.Tags)
	paymentChannelIDs, err := s.filterAvailablePaymentChannelIDs(input.PaymentChannelIDs)
	if err != nil {
		return nil, err
	}
	product.PaymentChannelIDs = productdomain.EncodePaymentChannelIDs(paymentChannelIDs)
	if input.IsActive != nil {
		product.IsActive = *input.IsActive
	}
	if input.IsAffiliateEnabled != nil {
		product.IsAffiliateEnabled = *input.IsAffiliateEnabled
	}
	rawPurchaseType := strings.TrimSpace(input.PurchaseType)
	if rawPurchaseType == "" {
		rawPurchaseType = product.PurchaseType
	}
	purchaseType := productdomain.NormalizePurchaseType(rawPurchaseType)
	if purchaseType == "" {
		return nil, productcontract.ErrProductPurchaseInvalid
	}
	product.PurchaseType = purchaseType
	if input.MaxPurchaseQuantity != nil {
		product.MaxPurchaseQuantity = productdomain.NormalizePurchaseQuantityLimit(*input.MaxPurchaseQuantity)
	}
	if input.MinPurchaseQuantity != nil {
		product.MinPurchaseQuantity = productdomain.NormalizePurchaseQuantityLimit(*input.MinPurchaseQuantity)
	}
	if product.MinPurchaseQuantity > 0 && product.MaxPurchaseQuantity > 0 && product.MinPurchaseQuantity > product.MaxPurchaseQuantity {
		return nil, productcontract.ErrProductPurchaseLimitInvalid
	}
	stockDisplayMode := productdomain.NormalizeStockDisplayMode(input.StockDisplayMode)
	if stockDisplayMode == "" {
		return nil, productcontract.ErrProductStockDisplayInvalid
	}
	product.StockDisplayMode = stockDisplayMode
	rawFulfillmentType := strings.TrimSpace(input.FulfillmentType)
	if rawFulfillmentType == "" {
		rawFulfillmentType = product.FulfillmentType
	}
	fulfillmentType := productdomain.NormalizeFulfillmentType(rawFulfillmentType)
	if fulfillmentType == "" {
		return nil, productcontract.ErrFulfillmentInvalid
	}
	// 对接商品的真实交付类型必须保持 upstream，后台返回的 auto/manual 仅用于展示。
	if product.IsMapped {
		fulfillmentType = constants.FulfillmentTypeUpstream
	}
	product.FulfillmentType = fulfillmentType
	if fulfillmentType == constants.FulfillmentTypeManual {
		normalizedSchemaJSON, err := manualform.NormalizeSchema(jsonmap.JSON(input.ManualFormSchemaJSON))
		if err != nil {
			return nil, err
		}
		product.ManualFormSchemaJSON = normalizedSchemaJSON
	}

	manualStockTotal := product.ManualStockTotal
	if input.ManualStockTotal != nil {
		manualStockTotal = *input.ManualStockTotal
	}
	if manualStockTotal < constants.ManualStockUnlimited {
		return nil, productcontract.ErrManualStockInvalid
	}

	var normalizedSKUs []normalizedProductSKU
	if len(input.SKUs) > 0 {
		if s.skus == nil {
			return nil, productcontract.ErrProductSKUInvalid
		}
		existingSKUs, listErr := s.skus.ListByProduct(product.ID, false)
		if listErr != nil {
			return nil, listErr
		}
		existingSKUMap := make(map[uint]productdomain.ProductSKU, len(existingSKUs))
		for _, sku := range existingSKUs {
			existingSKUMap[sku.ID] = sku
		}
		var normalizeErr error
		normalizedSKUs, priceAmount, manualStockTotal, normalizeErr = s.normalizeProductSKUInputs(input.SKUs, fulfillmentType, existingSKUMap)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
	}

	agencyPriceAmount := input.AgencyPriceAmount.Round(2)
	if agencyPriceAmount.LessThan(decimal.Zero) {
		return nil, productcontract.ErrProductPriceInvalid
	}

	product.PriceAmount = money.FromDecimal(priceAmount)
	if len(normalizedSKUs) > 0 {
		product.CostPriceAmount = money.FromDecimal(minActiveCostPrice(normalizedSKUs))
		product.AgencyPriceAmount = money.FromDecimal(minActiveAgencyPrice(normalizedSKUs))
	} else {
		product.CostPriceAmount = money.FromDecimal(input.CostPriceAmount.Round(2))
		product.AgencyPriceAmount = money.FromDecimal(agencyPriceAmount)
	}
	product.ManualStockTotal = manualStockTotal

	if err := s.transactions.WithinTransaction(func(repositories TransactionRepositories) error {
		productRepo := repositories.Products
		skuRepo := repositories.SKUs
		cardSecretRepo := repositories.CardSecrets
		if len(normalizedSKUs) > 0 {
			if err := s.applyProductSKUsWithStockGuard(skuRepo, cardSecretRepo, product.ID, fulfillmentType, normalizedSKUs); err != nil {
				return err
			}
		} else if err := s.syncSingleProductSKU(skuRepo, cardSecretRepo, product.ID, fulfillmentType, priceAmount, product.CostPriceAmount.Decimal, product.AgencyPriceAmount.Decimal, product.ManualStockTotal); err != nil {
			return err
		}
		// SKU 落库之后才处理批发价：此时的规格集合才是最终形态。
		var skus []productdomain.ProductSKU
		if skuRepo != nil {
			var err error
			skus, err = skuRepo.ListByProduct(product.ID, false)
			if err != nil {
				return err
			}
		}
		// 仅当请求显式携带批发价字段时才覆盖，省略字段（nil）保留原有配置，
		// 避免不关心批发价的局部更新静默清空已配阶梯。
		if input.WholesalePrices != nil {
			wholesalePrices, err := productdomain.NormalizeWholesalePricesForSKUs(*input.WholesalePrices, skus)
			if err != nil {
				return err
			}
			product.WholesalePrices = wholesalePrices
		} else {
			// 上面可能删掉了规格行。保留的阶梯若仍指向已删除的 SKU，会让后续任何
			// 批发价编辑都因为「引用了不存在的 SKU」而失败，这里顺手清掉悬空阶梯。
			product.WholesalePrices = productdomain.PruneWholesalePricesForSKUs(product.WholesalePrices, skus)
		}
		if err := productRepo.Update(product); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.products.GetByID(id)
}

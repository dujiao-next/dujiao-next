package productwrite

import (
	"strconv"

	"github.com/dujiao-next/internal/constants"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/modules/catalog/product/manualform"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// Create 创建商品
func (s *WriteService) Create(input CreateProductInput) (*productdomain.Product, error) {
	if err := productdomain.ValidateCategoryAssignment(s.categories, input.CategoryID, 0, productcontract.ErrProductCategoryInvalid); err != nil {
		return nil, err
	}

	count, err := s.products.CountBySlug(input.Slug, nil)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, productcontract.ErrSlugExists
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}
	isAffiliateEnabled := false
	if input.IsAffiliateEnabled != nil {
		isAffiliateEnabled = *input.IsAffiliateEnabled
	}
	purchaseType := productdomain.NormalizePurchaseType(input.PurchaseType)
	if purchaseType == "" {
		return nil, productcontract.ErrProductPurchaseInvalid
	}
	fulfillmentType := productdomain.NormalizeFulfillmentType(input.FulfillmentType)
	if fulfillmentType == "" {
		return nil, productcontract.ErrFulfillmentInvalid
	}

	priceAmount := input.PriceAmount.Round(2)
	if len(input.SKUs) == 0 && priceAmount.LessThanOrEqual(decimal.Zero) {
		return nil, productcontract.ErrProductPriceInvalid
	}

	manualStockTotal := 0
	if input.ManualStockTotal != nil {
		manualStockTotal = *input.ManualStockTotal
	}
	if manualStockTotal < constants.ManualStockUnlimited {
		return nil, productcontract.ErrManualStockInvalid
	}
	maxPurchaseQuantity := 0
	if input.MaxPurchaseQuantity != nil {
		maxPurchaseQuantity = productdomain.NormalizePurchaseQuantityLimit(*input.MaxPurchaseQuantity)
	}
	minPurchaseQuantity := 0
	if input.MinPurchaseQuantity != nil {
		minPurchaseQuantity = productdomain.NormalizePurchaseQuantityLimit(*input.MinPurchaseQuantity)
	}
	if minPurchaseQuantity > 0 && maxPurchaseQuantity > 0 && minPurchaseQuantity > maxPurchaseQuantity {
		return nil, productcontract.ErrProductPurchaseLimitInvalid
	}
	stockDisplayMode := productdomain.NormalizeStockDisplayMode(input.StockDisplayMode)
	if stockDisplayMode == "" {
		return nil, productcontract.ErrProductStockDisplayInvalid
	}

	costPriceAmount := input.CostPriceAmount.Round(2)
	agencyPriceAmount := input.AgencyPriceAmount.Round(2)
	if agencyPriceAmount.LessThan(decimal.Zero) {
		return nil, productcontract.ErrProductPriceInvalid
	}
	var wholesaleInputs []productdomain.WholesalePriceInput
	if input.WholesalePrices != nil {
		wholesaleInputs = *input.WholesalePrices
	}

	var normalizedSKUs []normalizedProductSKU
	if len(input.SKUs) > 0 {
		if s.skus == nil {
			return nil, productcontract.ErrProductSKUInvalid
		}
		var normalizeErr error
		normalizedSKUs, priceAmount, manualStockTotal, normalizeErr = s.normalizeProductSKUInputs(input.SKUs, fulfillmentType, nil)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		costPriceAmount = minActiveCostPrice(normalizedSKUs)
		agencyPriceAmount = minActiveAgencyPrice(normalizedSKUs)
	}
	paymentChannelIDs, err := s.filterAvailablePaymentChannelIDs(input.PaymentChannelIDs)
	if err != nil {
		return nil, err
	}

	product := productdomain.Product{
		CategoryID:           input.CategoryID,
		Slug:                 input.Slug,
		SeoMetaJSON:          jsonmap.JSON(input.SeoMetaJSON),
		TitleJSON:            jsonmap.JSON(input.TitleJSON),
		DescriptionJSON:      jsonmap.JSON(input.DescriptionJSON),
		ContentJSON:          jsonmap.JSON(input.ContentJSON),
		InstructionsJSON:     jsonmap.JSON(input.InstructionsJSON),
		ManualFormSchemaJSON: jsonmap.JSON{},
		PriceAmount:          money.FromDecimal(priceAmount),
		CostPriceAmount:      money.FromDecimal(costPriceAmount),
		AgencyPriceAmount:    money.FromDecimal(agencyPriceAmount),
		WholesalePrices:      productdomain.WholesalePriceTiers{},
		Images:               jsonslice.Strings(input.Images),
		Tags:                 jsonslice.Strings(input.Tags),
		PurchaseType:         purchaseType,
		MinPurchaseQuantity:  minPurchaseQuantity,
		MaxPurchaseQuantity:  maxPurchaseQuantity,
		StockDisplayMode:     stockDisplayMode,
		FulfillmentType:      fulfillmentType,
		ManualStockTotal:     manualStockTotal,
		ManualStockLocked:    0,
		ManualStockSold:      0,
		PaymentChannelIDs:    productdomain.EncodePaymentChannelIDs(paymentChannelIDs),
		IsAffiliateEnabled:   isAffiliateEnabled,
		IsActive:             isActive,
		SortOrder:            input.SortOrder,
	}
	if fulfillmentType == constants.FulfillmentTypeManual {
		normalizedSchemaJSON, err := manualform.NormalizeSchema(jsonmap.JSON(input.ManualFormSchemaJSON))
		if err != nil {
			return nil, err
		}
		product.ManualFormSchemaJSON = normalizedSchemaJSON
	}

	if err := s.transactions.WithinTransaction(func(repositories TransactionRepositories) error {
		productRepo := repositories.Products
		skuRepo := repositories.SKUs
		cardSecretRepo := repositories.CardSecrets
		if err := productRepo.Create(&product); err != nil {
			return err
		}
		if len(normalizedSKUs) > 0 {
			if err := s.applyProductSKUsWithStockGuard(skuRepo, cardSecretRepo, product.ID, fulfillmentType, normalizedSKUs); err != nil {
				return err
			}
		} else if err := s.syncSingleProductSKU(skuRepo, cardSecretRepo, product.ID, fulfillmentType, priceAmount, costPriceAmount, agencyPriceAmount, manualStockTotal); err != nil {
			return err
		}
		if input.WholesalePrices != nil {
			var skus []productdomain.ProductSKU
			if skuRepo != nil {
				var err error
				skus, err = skuRepo.ListByProduct(product.ID, false)
				if err != nil {
					return err
				}
			}
			wholesalePrices, err := productdomain.NormalizeWholesalePricesForSKUs(wholesaleInputs, skus)
			if err != nil {
				return err
			}
			product.WholesalePrices = wholesalePrices
			if err := productRepo.QuickUpdate(strconv.FormatUint(uint64(product.ID), 10), map[string]interface{}{"wholesale_prices": wholesalePrices}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.products.GetByID(strconv.FormatUint(uint64(product.ID), 10))
}

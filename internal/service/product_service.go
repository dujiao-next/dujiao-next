package service

import (
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ProductService 商品业务服务
type ProductService struct {
	repo                 repository.ProductRepository
	productSKURepo       repository.ProductSKURepository
	cardSecretRepo       repository.CardSecretRepository
	cardSecretBatchRepo  repository.CardSecretBatchRepository
	categoryRepo         repository.CategoryRepository
	memberLevelPriceRepo repository.MemberLevelPriceRepository
	cartRepo             repository.CartRepository
	productMappingRepo   repository.ProductMappingRepository
	orderRepo            repository.OrderRepository
	paymentChannelRepo   repository.PaymentChannelRepository
}

// NewProductService 创建商品服务
func NewProductService(
	repo repository.ProductRepository,
	productSKURepo repository.ProductSKURepository,
	cardSecretRepo repository.CardSecretRepository,
	cardSecretBatchRepo repository.CardSecretBatchRepository,
	categoryRepo repository.CategoryRepository,
	memberLevelPriceRepo repository.MemberLevelPriceRepository,
	cartRepo repository.CartRepository,
	productMappingRepo repository.ProductMappingRepository,
	orderRepo repository.OrderRepository,
	paymentChannelRepo repository.PaymentChannelRepository,
) *ProductService {
	return &ProductService{
		repo:                 repo,
		productSKURepo:       productSKURepo,
		cardSecretRepo:       cardSecretRepo,
		cardSecretBatchRepo:  cardSecretBatchRepo,
		categoryRepo:         categoryRepo,
		memberLevelPriceRepo: memberLevelPriceRepo,
		cartRepo:             cartRepo,
		productMappingRepo:   productMappingRepo,
		orderRepo:            orderRepo,
		paymentChannelRepo:   paymentChannelRepo,
	}
}

func (s *ProductService) filterAvailablePaymentChannelIDs(ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	uniqueIDs := make([]uint, 0, len(ids))
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		return nil, nil
	}
	if s.paymentChannelRepo == nil {
		return uniqueIDs, nil
	}

	channels, err := s.paymentChannelRepo.ListByIDs(uniqueIDs)
	if err != nil {
		return nil, err
	}
	activeIDs := make(map[uint]struct{}, len(channels))
	for _, channel := range channels {
		if channel.IsActive {
			activeIDs[channel.ID] = struct{}{}
		}
	}

	filtered := make([]uint, 0, len(uniqueIDs))
	for _, id := range uniqueIDs {
		if _, ok := activeIDs[id]; ok {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	return filtered, nil
}

// CreateProductInput 创建/更新商品输入
type CreateProductInput struct {
	CategoryID           uint
	Slug                 string
	SeoMetaJSON          map[string]interface{}
	TitleJSON            map[string]interface{}
	DescriptionJSON      map[string]interface{}
	ContentJSON          map[string]interface{}
	InstructionsJSON     map[string]interface{}
	ManualFormSchemaJSON map[string]interface{}
	PriceAmount          decimal.Decimal
	CostPriceAmount      decimal.Decimal
	// WholesalePrices 为可选字段：nil 表示「不修改」（Update 时保留原有批发价），
	// 非 nil（含空切片）表示以传入内容整体覆盖。Create 时 nil 与空切片等价于无批发价。
	WholesalePrices     *[]WholesalePriceInput
	Images              []string
	Tags                []string
	PurchaseType        string
	MinPurchaseQuantity *int
	MaxPurchaseQuantity *int
	StockDisplayMode    string
	FulfillmentType     string
	ManualStockTotal    *int
	SKUs                []ProductSKUInput
	PaymentChannelIDs   []uint
	IsAffiliateEnabled  *bool
	IsActive            *bool
	SortOrder           int
}

type WholesalePriceInput struct {
	SKUID       uint
	SKUCode     string
	MinQuantity int
	UnitPrice   decimal.Decimal
}

type ProductSKUInput struct {
	ID               uint
	SKUCode          string
	SpecValuesJSON   map[string]interface{}
	PriceAmount      decimal.Decimal
	CostPriceAmount  decimal.Decimal
	ManualStockTotal int
	IsActive         *bool
	SortOrder        int
}

// ListPublic 获取公开商品列表
func (s *ProductService) ListPublic(categoryID, search string, page, pageSize int) ([]models.Product, int64, error) {
	categoryIDs, err := expandPublicCategoryIDs(s.categoryRepo, categoryID)
	if err != nil {
		return nil, 0, err
	}

	filter := repository.ProductListFilter{
		Page:         page,
		PageSize:     pageSize,
		CategoryID:   categoryID,
		CategoryIDs:  categoryIDs,
		Search:       search,
		OnlyActive:   true,
		WithCategory: true,
	}
	return s.repo.List(filter)
}

// ListPublicForTenant 获取当前租户上下文的公开商品列表。
func (s *ProductService) ListPublicForTenant(tenant TenantContext, resellerRepo repository.ResellerRepository, categoryID, search string, page, pageSize int) ([]models.Product, int64, error) {
	if !isResellerOrderContext(tenant) {
		return s.ListPublic(categoryID, search, page, pageSize)
	}
	if tenant.ResellerID == nil || resellerRepo == nil {
		return nil, 0, ErrResellerProductNotListed
	}
	categoryIDs, err := expandPublicCategoryIDs(s.categoryRepo, categoryID)
	if err != nil {
		return nil, 0, err
	}
	hiddenIDs, err := resellerRepo.ListHiddenProductIDs(*tenant.ResellerID)
	if err != nil {
		return nil, 0, err
	}
	filter := repository.ProductListFilter{
		Page:              page,
		PageSize:          pageSize,
		CategoryID:        categoryID,
		CategoryIDs:       categoryIDs,
		Search:            search,
		OnlyActive:        true,
		WithCategory:      true,
		ExcludeProductIDs: hiddenIDs,
	}
	return s.repo.List(filter)
}

// ListForUpstreamSync 上游同步专用：可选包含已下架商品，便于下游识别下架状态
// includeInactive=true 时返回所有未软删商品（含 is_active=false）
func (s *ProductService) ListForUpstreamSync(updatedAfter *time.Time, includeInactive bool, page, pageSize int) ([]models.Product, int64, error) {
	filter := repository.ProductListFilter{
		Page:         page,
		PageSize:     pageSize,
		OnlyActive:   !includeInactive,
		WithCategory: true,
		UpdatedAfter: updatedAfter,
	}
	return s.repo.List(filter)
}

// ListPublicExact 获取公开商品列表（精确匹配分类，不展开父分类）
func (s *ProductService) ListPublicExact(categoryID string, page, pageSize int) ([]models.Product, int64, error) {
	filter := repository.ProductListFilter{
		Page:         page,
		PageSize:     pageSize,
		CategoryID:   categoryID,
		OnlyActive:   true,
		WithCategory: true,
	}
	return s.repo.List(filter)
}

// GetPublicBySlug 获取公开商品详情
func (s *ProductService) GetPublicBySlug(slug string) (*models.Product, error) {
	product, err := s.repo.GetBySlug(slug, true)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, ErrNotFound
	}
	return product, nil
}

// GetPublicBySlugForTenant 获取当前租户上下文的公开商品详情。
func (s *ProductService) GetPublicBySlugForTenant(tenant TenantContext, resellerRepo repository.ResellerRepository, slug string) (*models.Product, error) {
	product, err := s.GetPublicBySlug(slug)
	if err != nil {
		return nil, err
	}
	if !isResellerOrderContext(tenant) {
		return product, nil
	}
	if tenant.ResellerID == nil || resellerRepo == nil {
		return nil, ErrNotFound
	}
	hiddenIDs, err := resellerRepo.ListHiddenProductIDs(*tenant.ResellerID)
	if err != nil {
		return nil, err
	}
	for _, id := range hiddenIDs {
		if id == product.ID {
			return nil, ErrNotFound
		}
	}
	return product, nil
}

// ListAdmin 获取后台商品列表
func (s *ProductService) ListAdmin(categoryID, search, fulfillmentType, stockStatus string, hasWholesalePrices *bool, lowStockThreshold int, page, pageSize int) ([]models.Product, int64, error) {
	filter := repository.ProductListFilter{
		Page:               page,
		PageSize:           pageSize,
		CategoryID:         categoryID,
		Search:             search,
		FulfillmentType:    strings.TrimSpace(fulfillmentType),
		StockStatus:        normalizeStockStatus(stockStatus),
		HasWholesalePrices: hasWholesalePrices,
		LowStockThreshold:  lowStockThreshold,
		OnlyActive:         false,
		WithCategory:       true,
	}
	return s.repo.List(filter)
}

// GetAdminByID 获取后台商品详情
func (s *ProductService) GetAdminByID(id string) (*models.Product, error) {
	product, err := s.repo.GetAdminByID(id)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, ErrNotFound
	}
	return product, nil
}

// Create 创建商品
func (s *ProductService) Create(input CreateProductInput) (*models.Product, error) {
	if err := validateProductCategoryAssignment(s.categoryRepo, input.CategoryID, 0); err != nil {
		return nil, err
	}

	count, err := s.repo.CountBySlug(input.Slug, nil)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSlugExists
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}
	isAffiliateEnabled := false
	if input.IsAffiliateEnabled != nil {
		isAffiliateEnabled = *input.IsAffiliateEnabled
	}
	purchaseType := normalizePurchaseType(input.PurchaseType)
	if purchaseType == "" {
		return nil, ErrProductPurchaseInvalid
	}
	fulfillmentType := normalizeFulfillmentType(input.FulfillmentType)
	if fulfillmentType == "" {
		return nil, ErrFulfillmentInvalid
	}

	priceAmount := input.PriceAmount.Round(2)
	if len(input.SKUs) == 0 && priceAmount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrProductPriceInvalid
	}

	manualStockTotal := 0
	if input.ManualStockTotal != nil {
		manualStockTotal = *input.ManualStockTotal
	}
	if manualStockTotal < constants.ManualStockUnlimited {
		return nil, ErrManualStockInvalid
	}
	maxPurchaseQuantity := 0
	if input.MaxPurchaseQuantity != nil {
		maxPurchaseQuantity = normalizeMaxPurchaseQuantity(*input.MaxPurchaseQuantity)
	}
	minPurchaseQuantity := 0
	if input.MinPurchaseQuantity != nil {
		minPurchaseQuantity = normalizeMinPurchaseQuantity(*input.MinPurchaseQuantity)
	}
	if minPurchaseQuantity > 0 && maxPurchaseQuantity > 0 && minPurchaseQuantity > maxPurchaseQuantity {
		return nil, ErrProductPurchaseLimitInvalid
	}
	stockDisplayMode := normalizeStockDisplayMode(input.StockDisplayMode)
	if stockDisplayMode == "" {
		return nil, ErrProductStockDisplayInvalid
	}

	costPriceAmount := input.CostPriceAmount.Round(2)
	var wholesaleInputs []WholesalePriceInput
	if input.WholesalePrices != nil {
		wholesaleInputs = *input.WholesalePrices
	}

	var normalizedSKUs []normalizedProductSKU
	if len(input.SKUs) > 0 {
		if s.productSKURepo == nil {
			return nil, ErrProductSKUInvalid
		}
		var normalizeErr error
		normalizedSKUs, priceAmount, manualStockTotal, normalizeErr = normalizeProductSKUInputs(input.SKUs, fulfillmentType, nil)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		costPriceAmount = minActiveCostPrice(normalizedSKUs)
	}
	paymentChannelIDs, err := s.filterAvailablePaymentChannelIDs(input.PaymentChannelIDs)
	if err != nil {
		return nil, err
	}

	product := models.Product{
		CategoryID:           input.CategoryID,
		Slug:                 input.Slug,
		SeoMetaJSON:          models.JSON(input.SeoMetaJSON),
		TitleJSON:            models.JSON(input.TitleJSON),
		DescriptionJSON:      models.JSON(input.DescriptionJSON),
		ContentJSON:          models.JSON(input.ContentJSON),
		InstructionsJSON:     models.JSON(input.InstructionsJSON),
		ManualFormSchemaJSON: models.JSON{},
		PriceAmount:          models.NewMoneyFromDecimal(priceAmount),
		CostPriceAmount:      models.NewMoneyFromDecimal(costPriceAmount),
		WholesalePrices:      models.WholesalePriceTiers{},
		Images:               models.StringArray(input.Images),
		Tags:                 models.StringArray(input.Tags),
		PurchaseType:         purchaseType,
		MinPurchaseQuantity:  minPurchaseQuantity,
		MaxPurchaseQuantity:  maxPurchaseQuantity,
		StockDisplayMode:     stockDisplayMode,
		FulfillmentType:      fulfillmentType,
		ManualStockTotal:     manualStockTotal,
		ManualStockLocked:    0,
		ManualStockSold:      0,
		PaymentChannelIDs:    EncodeChannelIDs(paymentChannelIDs),
		IsAffiliateEnabled:   isAffiliateEnabled,
		IsActive:             isActive,
		SortOrder:            input.SortOrder,
	}
	if fulfillmentType == constants.FulfillmentTypeManual {
		_, normalizedSchemaJSON, err := parseManualFormSchema(models.JSON(input.ManualFormSchemaJSON))
		if err != nil {
			return nil, err
		}
		product.ManualFormSchemaJSON = normalizedSchemaJSON
	}

	if err := s.repo.Transaction(func(tx *gorm.DB) error {
		productRepo := s.repo.WithTx(tx)
		var skuRepo repository.ProductSKURepository
		var cardSecretRepo repository.CardSecretRepository
		if s.productSKURepo != nil {
			skuRepo = s.productSKURepo.WithTx(tx)
		}
		if s.cardSecretRepo != nil {
			cardSecretRepo = s.cardSecretRepo.WithTx(tx)
		}
		if err := productRepo.Create(&product); err != nil {
			return err
		}
		if len(normalizedSKUs) > 0 {
			if err := applyProductSKUsWithStockGuard(skuRepo, cardSecretRepo, product.ID, fulfillmentType, normalizedSKUs); err != nil {
				return err
			}
		} else if err := syncSingleProductSKU(skuRepo, product.ID, priceAmount, costPriceAmount, manualStockTotal, true); err != nil {
			return err
		}
		if input.WholesalePrices != nil {
			var skus []models.ProductSKU
			if skuRepo != nil {
				var err error
				skus, err = skuRepo.ListByProduct(product.ID, false)
				if err != nil {
					return err
				}
			}
			wholesalePrices, err := normalizeWholesalePriceInputsForSKUs(wholesaleInputs, skus)
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
	return s.repo.GetByID(strconv.FormatUint(uint64(product.ID), 10))
}

// Update 更新商品
func (s *ProductService) Update(id string, input CreateProductInput) (*models.Product, error) {
	priceAmount := input.PriceAmount.Round(2)
	if len(input.SKUs) == 0 && priceAmount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrProductPriceInvalid
	}
	product, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, ErrNotFound
	}
	if err := validateProductCategoryAssignment(s.categoryRepo, input.CategoryID, product.CategoryID); err != nil {
		return nil, err
	}

	count, err := s.repo.CountBySlug(input.Slug, &id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSlugExists
	}

	product.CategoryID = input.CategoryID
	product.Category = models.Category{}
	product.Slug = input.Slug
	product.SeoMetaJSON = models.JSON(input.SeoMetaJSON)
	product.TitleJSON = models.JSON(input.TitleJSON)
	product.DescriptionJSON = models.JSON(input.DescriptionJSON)
	product.ContentJSON = models.JSON(input.ContentJSON)
	product.InstructionsJSON = models.JSON(input.InstructionsJSON)
	product.ManualFormSchemaJSON = models.JSON{}
	product.PriceAmount = models.NewMoneyFromDecimal(priceAmount)
	product.SortOrder = input.SortOrder
	product.Images = models.StringArray(input.Images)
	product.Tags = models.StringArray(input.Tags)
	paymentChannelIDs, err := s.filterAvailablePaymentChannelIDs(input.PaymentChannelIDs)
	if err != nil {
		return nil, err
	}
	product.PaymentChannelIDs = EncodeChannelIDs(paymentChannelIDs)
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
	purchaseType := normalizePurchaseType(rawPurchaseType)
	if purchaseType == "" {
		return nil, ErrProductPurchaseInvalid
	}
	product.PurchaseType = purchaseType
	if input.MaxPurchaseQuantity != nil {
		product.MaxPurchaseQuantity = normalizeMaxPurchaseQuantity(*input.MaxPurchaseQuantity)
	}
	if input.MinPurchaseQuantity != nil {
		product.MinPurchaseQuantity = normalizeMinPurchaseQuantity(*input.MinPurchaseQuantity)
	}
	if product.MinPurchaseQuantity > 0 && product.MaxPurchaseQuantity > 0 && product.MinPurchaseQuantity > product.MaxPurchaseQuantity {
		return nil, ErrProductPurchaseLimitInvalid
	}
	stockDisplayMode := normalizeStockDisplayMode(input.StockDisplayMode)
	if stockDisplayMode == "" {
		return nil, ErrProductStockDisplayInvalid
	}
	product.StockDisplayMode = stockDisplayMode
	rawFulfillmentType := strings.TrimSpace(input.FulfillmentType)
	if rawFulfillmentType == "" {
		rawFulfillmentType = product.FulfillmentType
	}
	fulfillmentType := normalizeFulfillmentType(rawFulfillmentType)
	if fulfillmentType == "" {
		return nil, ErrFulfillmentInvalid
	}
	// 对接商品的真实交付类型必须保持 upstream，后台返回的 auto/manual 仅用于展示。
	if product.IsMapped {
		fulfillmentType = constants.FulfillmentTypeUpstream
	}
	product.FulfillmentType = fulfillmentType
	if fulfillmentType == constants.FulfillmentTypeManual {
		_, normalizedSchemaJSON, err := parseManualFormSchema(models.JSON(input.ManualFormSchemaJSON))
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
		return nil, ErrManualStockInvalid
	}

	var normalizedSKUs []normalizedProductSKU
	if len(input.SKUs) > 0 {
		if s.productSKURepo == nil {
			return nil, ErrProductSKUInvalid
		}
		existingSKUs, listErr := s.productSKURepo.ListByProduct(product.ID, false)
		if listErr != nil {
			return nil, listErr
		}
		existingSKUMap := make(map[uint]models.ProductSKU, len(existingSKUs))
		for _, sku := range existingSKUs {
			existingSKUMap[sku.ID] = sku
		}
		var normalizeErr error
		normalizedSKUs, priceAmount, manualStockTotal, normalizeErr = normalizeProductSKUInputs(input.SKUs, fulfillmentType, existingSKUMap)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
	}

	product.PriceAmount = models.NewMoneyFromDecimal(priceAmount)
	if len(normalizedSKUs) > 0 {
		product.CostPriceAmount = models.NewMoneyFromDecimal(minActiveCostPrice(normalizedSKUs))
	} else {
		product.CostPriceAmount = models.NewMoneyFromDecimal(input.CostPriceAmount.Round(2))
	}
	product.ManualStockTotal = manualStockTotal

	if err := s.repo.Transaction(func(tx *gorm.DB) error {
		productRepo := s.repo.WithTx(tx)
		var skuRepo repository.ProductSKURepository
		var cardSecretRepo repository.CardSecretRepository
		if s.productSKURepo != nil {
			skuRepo = s.productSKURepo.WithTx(tx)
		}
		if s.cardSecretRepo != nil {
			cardSecretRepo = s.cardSecretRepo.WithTx(tx)
		}
		if len(normalizedSKUs) > 0 {
			if err := applyProductSKUsWithStockGuard(skuRepo, cardSecretRepo, product.ID, fulfillmentType, normalizedSKUs); err != nil {
				return err
			}
		} else if err := syncSingleProductSKU(skuRepo, product.ID, priceAmount, product.CostPriceAmount.Decimal, product.ManualStockTotal, true); err != nil {
			return err
		}
		// 仅当请求显式携带批发价字段时才覆盖，省略字段（nil）保留原有配置，
		// 避免不关心批发价的局部更新静默清空已配阶梯。
		if input.WholesalePrices != nil {
			var skus []models.ProductSKU
			if skuRepo != nil {
				var err error
				skus, err = skuRepo.ListByProduct(product.ID, false)
				if err != nil {
					return err
				}
			}
			wholesalePrices, err := normalizeWholesalePriceInputsForSKUs(*input.WholesalePrices, skus)
			if err != nil {
				return err
			}
			product.WholesalePrices = wholesalePrices
		}
		if err := productRepo.Update(product); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.repo.GetByID(id)
}

func syncSingleProductSKU(skuRepo repository.ProductSKURepository, productID uint, priceAmount decimal.Decimal, costPriceAmount decimal.Decimal, manualStockTotal int, createWhenMissing bool) error {
	if skuRepo == nil || productID == 0 {
		return nil
	}
	skus, err := skuRepo.ListByProduct(productID, false)
	if err != nil {
		return err
	}
	if len(skus) == 0 {
		if !createWhenMissing {
			return nil
		}
		return skuRepo.Create(&models.ProductSKU{
			ProductID:         productID,
			SKUCode:           models.DefaultSKUCode,
			SpecValuesJSON:    models.JSON{},
			PriceAmount:       models.NewMoneyFromDecimal(priceAmount),
			CostPriceAmount:   models.NewMoneyFromDecimal(costPriceAmount),
			ManualStockTotal:  manualStockTotal,
			ManualStockLocked: 0,
			ManualStockSold:   0,
			IsActive:          true,
			SortOrder:         0,
		})
	}
	targetIndex := pickSingleModeTargetSKUIndex(skus)
	if targetIndex < 0 || targetIndex >= len(skus) {
		return ErrProductSKUInvalid
	}

	target := skus[targetIndex]
	target.PriceAmount = models.NewMoneyFromDecimal(priceAmount)
	target.CostPriceAmount = models.NewMoneyFromDecimal(costPriceAmount)
	target.ManualStockTotal = manualStockTotal
	target.IsActive = true
	if strings.TrimSpace(target.SKUCode) == "" {
		target.SKUCode = models.DefaultSKUCode
	}
	if err := skuRepo.Update(&target); err != nil {
		return err
	}

	for i := range skus {
		if i == targetIndex {
			continue
		}
		if err := skuRepo.Delete(skus[i].ID); err != nil {
			return err
		}
	}
	return nil
}

func pickSingleModeTargetSKUIndex(skus []models.ProductSKU) int {
	if len(skus) == 0 {
		return -1
	}
	defaultCode := strings.ToUpper(strings.TrimSpace(models.DefaultSKUCode))

	for i := range skus {
		if !skus[i].IsActive {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(skus[i].SKUCode)) == defaultCode {
			return i
		}
	}
	for i := range skus {
		if skus[i].IsActive {
			return i
		}
	}
	for i := range skus {
		if strings.ToUpper(strings.TrimSpace(skus[i].SKUCode)) == defaultCode {
			return i
		}
	}
	return 0
}

type normalizedProductSKU struct {
	ID               uint
	SKUCode          string
	SpecValuesJSON   models.JSON
	PriceAmount      models.Money
	CostPriceAmount  models.Money
	ManualStockTotal int
	IsActive         bool
	SortOrder        int
}

func normalizeProductSKUInputs(inputs []ProductSKUInput, fulfillmentType string, existingSKUMap map[uint]models.ProductSKU) ([]normalizedProductSKU, decimal.Decimal, int, error) {
	if len(inputs) == 0 {
		return nil, decimal.Zero, 0, ErrProductSKUInvalid
	}
	seenCode := make(map[string]struct{}, len(inputs))
	normalized := make([]normalizedProductSKU, 0, len(inputs))
	hasActive := false
	minActivePrice := decimal.Zero
	manualStockTotal := 0
	hasUnlimitedStock := false

	for _, input := range inputs {
		skuCode := strings.TrimSpace(input.SKUCode)
		if skuCode == "" {
			return nil, decimal.Zero, 0, ErrProductSKUInvalid
		}
		codeKey := strings.ToLower(skuCode)
		if _, exists := seenCode[codeKey]; exists {
			return nil, decimal.Zero, 0, ErrProductSKUInvalid
		}
		seenCode[codeKey] = struct{}{}

		priceAmount := input.PriceAmount.Round(2)
		if priceAmount.LessThanOrEqual(decimal.Zero) {
			return nil, decimal.Zero, 0, ErrProductPriceInvalid
		}
		costPriceAmount := input.CostPriceAmount.Round(2)
		if costPriceAmount.LessThan(decimal.Zero) {
			return nil, decimal.Zero, 0, ErrProductPriceInvalid
		}

		manualTotal := input.ManualStockTotal
		if manualTotal < constants.ManualStockUnlimited {
			return nil, decimal.Zero, 0, ErrManualStockInvalid
		}
		if fulfillmentType != constants.FulfillmentTypeManual {
			manualTotal = 0
		}
		if existingSKUMap != nil && input.ID > 0 {
			_, ok := existingSKUMap[input.ID]
			if !ok {
				return nil, decimal.Zero, 0, ErrProductSKUInvalid
			}
		}

		isActive := true
		if input.IsActive != nil {
			isActive = *input.IsActive
		}
		specValues := models.JSON{}
		if input.SpecValuesJSON != nil {
			specValues = models.JSON(input.SpecValuesJSON)
		}

		normalized = append(normalized, normalizedProductSKU{
			ID:               input.ID,
			SKUCode:          skuCode,
			SpecValuesJSON:   specValues,
			PriceAmount:      models.NewMoneyFromDecimal(priceAmount),
			CostPriceAmount:  models.NewMoneyFromDecimal(costPriceAmount),
			ManualStockTotal: manualTotal,
			IsActive:         isActive,
			SortOrder:        input.SortOrder,
		})

		if isActive {
			if !hasActive || priceAmount.LessThan(minActivePrice) {
				minActivePrice = priceAmount
			}
			hasActive = true
			if fulfillmentType == constants.FulfillmentTypeManual {
				if manualTotal == constants.ManualStockUnlimited {
					hasUnlimitedStock = true
				} else {
					manualStockTotal += manualTotal
				}
			}
		}
	}

	if !hasActive {
		return nil, decimal.Zero, 0, ErrProductSKUInvalid
	}
	if fulfillmentType != constants.FulfillmentTypeManual {
		manualStockTotal = 0
	} else if hasUnlimitedStock {
		manualStockTotal = constants.ManualStockUnlimited
	}
	return normalized, minActivePrice, manualStockTotal, nil
}

// minActiveCostPrice 从已标准化的 SKU 列表中取最低活跃 SKU 的成本价
func minActiveCostPrice(skus []normalizedProductSKU) decimal.Decimal {
	first := true
	min := decimal.Zero
	for _, s := range skus {
		if !s.IsActive {
			continue
		}
		d := s.CostPriceAmount.Decimal
		if first || d.LessThan(min) {
			min = d
			first = false
		}
	}
	return min
}

func applyProductSKUsWithStockGuard(
	skuRepo repository.ProductSKURepository,
	cardSecretRepo repository.CardSecretRepository,
	productID uint,
	fulfillmentType string,
	rows []normalizedProductSKU,
) error {
	if skuRepo == nil || productID == 0 || len(rows) == 0 {
		return ErrProductSKUInvalid
	}
	existingRows, err := skuRepo.ListByProduct(productID, false)
	if err != nil {
		return err
	}
	existingByID := make(map[uint]models.ProductSKU, len(existingRows))
	existingByCode := make(map[string]models.ProductSKU, len(existingRows))
	for _, row := range existingRows {
		existingByID[row.ID] = row
		existingByCode[strings.ToLower(strings.TrimSpace(row.SKUCode))] = row
	}
	if err := ensureAutoSKUCardSecretStockSafe(cardSecretRepo, productID, fulfillmentType, existingRows, rows, existingByID, existingByCode); err != nil {
		return err
	}

	kept := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		if row.ID > 0 {
			existing, ok := existingByID[row.ID]
			if !ok {
				return ErrProductSKUInvalid
			}
			existing.SKUCode = row.SKUCode
			existing.SpecValuesJSON = row.SpecValuesJSON
			existing.PriceAmount = row.PriceAmount
			existing.CostPriceAmount = row.CostPriceAmount
			existing.ManualStockTotal = row.ManualStockTotal
			existing.IsActive = row.IsActive
			existing.SortOrder = row.SortOrder
			if err := skuRepo.Update(&existing); err != nil {
				return err
			}
			kept[existing.ID] = struct{}{}
			existingByCode[strings.ToLower(strings.TrimSpace(existing.SKUCode))] = existing
			continue
		}

		codeKey := strings.ToLower(strings.TrimSpace(row.SKUCode))
		if existing, ok := existingByCode[codeKey]; ok {
			existing.SpecValuesJSON = row.SpecValuesJSON
			existing.PriceAmount = row.PriceAmount
			existing.CostPriceAmount = row.CostPriceAmount
			existing.ManualStockTotal = row.ManualStockTotal
			existing.IsActive = row.IsActive
			existing.SortOrder = row.SortOrder
			if err := skuRepo.Update(&existing); err != nil {
				return err
			}
			kept[existing.ID] = struct{}{}
			continue
		}

		// 清理同 sku_code 的软删除残留，避免唯一索引冲突
		if err := skuRepo.PurgeSoftDeletedByProductAndCode(productID, row.SKUCode); err != nil {
			return err
		}
		item := models.ProductSKU{
			ProductID:         productID,
			SKUCode:           row.SKUCode,
			SpecValuesJSON:    row.SpecValuesJSON,
			PriceAmount:       row.PriceAmount,
			CostPriceAmount:   row.CostPriceAmount,
			ManualStockTotal:  row.ManualStockTotal,
			ManualStockLocked: 0,
			ManualStockSold:   0,
			IsActive:          row.IsActive,
			SortOrder:         row.SortOrder,
		}
		if err := skuRepo.Create(&item); err != nil {
			return err
		}
		kept[item.ID] = struct{}{}
	}

	for _, existing := range existingRows {
		if _, ok := kept[existing.ID]; ok {
			continue
		}
		if err := skuRepo.Delete(existing.ID); err != nil {
			return err
		}
	}
	return nil
}

func ensureAutoSKUCardSecretStockSafe(
	cardSecretRepo repository.CardSecretRepository,
	productID uint,
	fulfillmentType string,
	existingRows []models.ProductSKU,
	rows []normalizedProductSKU,
	existingByID map[uint]models.ProductSKU,
	existingByCode map[string]models.ProductSKU,
) error {
	if cardSecretRepo == nil || productID == 0 || strings.TrimSpace(fulfillmentType) != constants.FulfillmentTypeAuto {
		return nil
	}

	nextActive := make(map[uint]bool, len(existingRows))
	kept := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		if row.ID > 0 {
			existing, ok := existingByID[row.ID]
			if !ok {
				return ErrProductSKUInvalid
			}
			nextActive[existing.ID] = row.IsActive
			kept[existing.ID] = struct{}{}
			continue
		}

		codeKey := strings.ToLower(strings.TrimSpace(row.SKUCode))
		if existing, ok := existingByCode[codeKey]; ok {
			nextActive[existing.ID] = row.IsActive
			kept[existing.ID] = struct{}{}
		}
	}

	for _, existing := range existingRows {
		if _, ok := nextActive[existing.ID]; !ok {
			nextActive[existing.ID] = false
		}
		if _, ok := kept[existing.ID]; !ok {
			nextActive[existing.ID] = false
		}
		if !existing.IsActive || nextActive[existing.ID] {
			continue
		}
		total, available, used, err := cardSecretRepo.CountByProduct(productID, existing.ID)
		if err != nil {
			return err
		}
		outstanding := total - used
		if available > 0 || outstanding > 0 {
			return ErrProductSKUHasCardSecretStock
		}
	}
	return nil
}

func normalizePurchaseType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", constants.ProductPurchaseMember:
		return constants.ProductPurchaseMember
	case constants.ProductPurchaseGuest:
		return constants.ProductPurchaseGuest
	default:
		return ""
	}
}

func normalizeFulfillmentType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", constants.FulfillmentTypeManual:
		return constants.FulfillmentTypeManual
	case constants.FulfillmentTypeAuto:
		return constants.FulfillmentTypeAuto
	case constants.FulfillmentTypeUpstream:
		return constants.FulfillmentTypeUpstream
	default:
		return ""
	}
}

func normalizeStockDisplayMode(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", constants.ProductStockDisplayExact:
		return constants.ProductStockDisplayExact
	case constants.ProductStockDisplayStatus:
		return constants.ProductStockDisplayStatus
	case constants.ProductStockDisplayRange:
		return constants.ProductStockDisplayRange
	case constants.ProductStockDisplayHidden:
		return constants.ProductStockDisplayHidden
	default:
		return ""
	}
}

func normalizeStockStatus(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "all":
		return ""
	case "low", "normal", "unlimited":
		return value
	default:
		return ""
	}
}

func expandPublicCategoryIDs(categoryRepo repository.CategoryRepository, categoryID string) ([]uint, error) {
	normalizedCategoryID := strings.TrimSpace(categoryID)
	if normalizedCategoryID == "" {
		return nil, nil
	}

	parsedCategoryID, err := strconv.ParseUint(normalizedCategoryID, 10, 64)
	if err != nil || parsedCategoryID == 0 {
		return nil, nil
	}
	if categoryRepo == nil {
		return []uint{uint(parsedCategoryID)}, nil
	}

	category, err := categoryRepo.GetByID(normalizedCategoryID)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return []uint{uint(parsedCategoryID)}, nil
	}
	if !category.IsActive {
		return []uint{}, nil
	}
	if category.ParentID > 0 {
		return []uint{category.ID}, nil
	}

	categories, err := categoryRepo.List()
	if err != nil {
		return nil, err
	}

	categoryIDs := []uint{category.ID}
	for _, item := range categories {
		if item.ParentID == category.ID && item.IsActive {
			categoryIDs = append(categoryIDs, item.ID)
		}
	}
	return categoryIDs, nil
}

func validateProductCategoryAssignment(categoryRepo repository.CategoryRepository, categoryID uint, currentCategoryID uint) error {
	if categoryID == 0 || categoryRepo == nil {
		return nil
	}

	categoryIDText := strconv.FormatUint(uint64(categoryID), 10)
	category, err := categoryRepo.GetByID(categoryIDText)
	if err != nil {
		return err
	}
	if category == nil {
		return ErrProductCategoryInvalid
	}

	childCount, err := categoryRepo.CountChildren(categoryIDText)
	if err != nil {
		return err
	}
	if childCount > 0 && categoryID != currentCategoryID {
		return ErrProductCategoryInvalid
	}

	return nil
}

// Delete 删除商品（含前置校验和级联清理）
func (s *ProductService) Delete(id string) error {
	product, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if product == nil {
		return ErrNotFound
	}

	// 事务前校验（避免 SQLite 自锁）

	// 1. 检查是否有可用或预占的卡密库存
	available, err := s.cardSecretRepo.CountAvailable(product.ID, 0)
	if err != nil {
		return err
	}
	if available > 0 {
		return ErrProductHasStock
	}
	reserved, err := s.cardSecretRepo.CountReserved(product.ID, 0)
	if err != nil {
		return err
	}
	if reserved > 0 {
		return ErrProductHasStock
	}

	// 2. 检查是否有成交记录
	orderCount, err := s.orderRepo.CountOrderItemsByProduct(product.ID)
	if err != nil {
		return err
	}
	if orderCount > 0 {
		return ErrProductHasOrderRecord
	}

	// 事务内级联删除
	return s.repo.Transaction(func(tx *gorm.DB) error {
		if err := s.cardSecretRepo.WithTx(tx).DeleteByProduct(product.ID); err != nil {
			return err
		}
		if err := s.cardSecretBatchRepo.WithTx(tx).DeleteByProduct(product.ID); err != nil {
			return err
		}
		if err := s.productSKURepo.WithTx(tx).DeleteByProduct(product.ID); err != nil {
			return err
		}
		if err := s.memberLevelPriceRepo.WithTx(tx).DeleteByProduct(product.ID); err != nil {
			return err
		}
		if err := s.cartRepo.WithTx(tx).DeleteByProduct(product.ID); err != nil {
			return err
		}
		if err := s.productMappingRepo.WithTx(tx).DeleteByLocalProduct(product.ID); err != nil {
			return err
		}
		return s.repo.WithTx(tx).Delete(id)
	})
}

// QuickUpdate 快速更新商品部分字段（如 is_active、sort_order）
func (s *ProductService) QuickUpdate(id string, fields map[string]interface{}) (*models.Product, error) {
	product, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, ErrNotFound
	}
	if isQuickUpdateActivatingProduct(fields) {
		categoryID := product.CategoryID
		if rawCategoryID, ok := fields["category_id"]; ok {
			parsedCategoryID, parseErr := quickUpdateCategoryID(rawCategoryID)
			if parseErr != nil {
				return nil, ErrProductCategoryInvalid
			}
			categoryID = parsedCategoryID
		}
		if err := validateProductActivationCategory(s.categoryRepo, categoryID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.QuickUpdate(id, fields); err != nil {
		return nil, err
	}
	return s.repo.GetByID(id)
}

// UpdateWholesalePrices 更新商品批发价阶梯，不修改商品其他字段。
func (s *ProductService) UpdateWholesalePrices(id string, inputs []WholesalePriceInput) (*models.Product, error) {
	product, err := s.repo.GetAdminByID(id)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, ErrNotFound
	}

	wholesalePrices, err := normalizeWholesalePriceInputsForSKUs(inputs, product.SKUs)
	if err != nil {
		return nil, err
	}
	if err := s.repo.QuickUpdate(id, map[string]interface{}{"wholesale_prices": wholesalePrices}); err != nil {
		return nil, err
	}
	return s.repo.GetAdminByID(id)
}

func isQuickUpdateActivatingProduct(fields map[string]interface{}) bool {
	raw, ok := fields["is_active"]
	if !ok {
		return false
	}
	value, ok := raw.(bool)
	return ok && value
}

func quickUpdateCategoryID(value interface{}) (uint, error) {
	switch v := value.(type) {
	case uint:
		return v, nil
	case uint64:
		return uint(v), nil
	case uint32:
		return uint(v), nil
	case int:
		if v < 0 {
			return 0, ErrProductCategoryInvalid
		}
		return uint(v), nil
	case int64:
		if v < 0 {
			return 0, ErrProductCategoryInvalid
		}
		return uint(v), nil
	case int32:
		if v < 0 {
			return 0, ErrProductCategoryInvalid
		}
		return uint(v), nil
	case float64:
		if v < 0 || v != float64(uint(v)) {
			return 0, ErrProductCategoryInvalid
		}
		return uint(v), nil
	default:
		return 0, ErrProductCategoryInvalid
	}
}

func validateProductActivationCategory(categoryRepo repository.CategoryRepository, categoryID uint) error {
	if categoryID == 0 || categoryRepo == nil {
		return ErrProductCategoryInvalid
	}

	categoryIDText := strconv.FormatUint(uint64(categoryID), 10)
	category, err := categoryRepo.GetByID(categoryIDText)
	if err != nil {
		return err
	}
	if category == nil || !category.IsActive {
		return ErrProductCategoryInvalid
	}

	childCount, err := categoryRepo.CountChildren(categoryIDText)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return ErrProductCategoryInvalid
	}
	return nil
}

// ApplyAutoStockCounts 聚合卡密自动发货库存信息并填充到商品中
func (s *ProductService) ApplyAutoStockCounts(products []models.Product) error {
	var productIDs []uint
	for _, p := range products {
		if p.FulfillmentType == constants.FulfillmentTypeAuto {
			productIDs = append(productIDs, p.ID)
		}
	}
	if len(productIDs) == 0 {
		return nil
	}

	counts, err := s.cardSecretRepo.CountStockByProductIDs(productIDs)
	if err != nil {
		return err
	}

	// map[product_id]map[sku_id]map[status]total
	stockMap := make(map[uint]map[uint]map[string]int64)
	for _, count := range counts {
		if stockMap[count.ProductID] == nil {
			stockMap[count.ProductID] = make(map[uint]map[string]int64)
		}
		if stockMap[count.ProductID][count.SKUID] == nil {
			stockMap[count.ProductID][count.SKUID] = make(map[string]int64)
		}
		stockMap[count.ProductID][count.SKUID][count.Status] = count.Total
	}

	for i := range products {
		if products[i].FulfillmentType != constants.FulfillmentTypeAuto {
			continue
		}
		pMap := stockMap[products[i].ID]
		if pMap == nil {
			continue
		}

		var pAvailable, pLocked, pUsed int64
		for _, statusMap := range pMap {
			pAvailable += statusMap[models.CardSecretStatusAvailable]
			pLocked += statusMap[models.CardSecretStatusReserved]
			pUsed += statusMap[models.CardSecretStatusUsed]
		}
		products[i].AutoStockAvailable = pAvailable
		products[i].AutoStockTotal = pAvailable + pLocked
		products[i].AutoStockLocked = pLocked
		products[i].AutoStockSold = pUsed

		legacyTargetIdx := resolveLegacyStockTargetSKUIndex(products[i].SKUs)
		for j := range products[i].SKUs {
			skuID := products[i].SKUs[j].ID
			statusMap := pMap[skuID]

			available := statusMap[models.CardSecretStatusAvailable]
			locked := statusMap[models.CardSecretStatusReserved]
			used := statusMap[models.CardSecretStatusUsed]

			// 如果 skuID 为 0 的历史卡密存在，优先归并到 DEFAULT SKU。
			// 若不存在 DEFAULT SKU，则回退到首个启用 SKU，避免重复叠加到所有 SKU。
			if j == legacyTargetIdx && pMap[0] != nil {
				available += pMap[0][models.CardSecretStatusAvailable]
				locked += pMap[0][models.CardSecretStatusReserved]
				used += pMap[0][models.CardSecretStatusUsed]
			}

			products[i].SKUs[j].AutoStockAvailable = available
			products[i].SKUs[j].AutoStockTotal = available + locked
			products[i].SKUs[j].AutoStockLocked = locked
			products[i].SKUs[j].AutoStockSold = used
		}
	}
	return nil
}

func resolveLegacyStockTargetSKUIndex(skus []models.ProductSKU) int {
	if len(skus) == 0 {
		return -1
	}

	defaultCode := strings.ToUpper(strings.TrimSpace(models.DefaultSKUCode))
	firstActiveIdx := -1
	for idx := range skus {
		if !skus[idx].IsActive {
			continue
		}
		if firstActiveIdx < 0 {
			firstActiveIdx = idx
		}
		if strings.ToUpper(strings.TrimSpace(skus[idx].SKUCode)) == defaultCode {
			return idx
		}
	}
	if firstActiveIdx >= 0 {
		return firstActiveIdx
	}

	// 防御性回退：没有启用 SKU 时，仍尽量归并到 DEFAULT SKU。
	for idx := range skus {
		if strings.ToUpper(strings.TrimSpace(skus[idx].SKUCode)) == defaultCode {
			return idx
		}
	}
	return 0
}

package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/upstream"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ImportUpstreamProduct 从上游导入商品（克隆为本地商品 + 建立映射）
func (s *ProductMappingService) ImportUpstreamProduct(connectionID uint, upstreamProductID uint, categoryID uint, slug string) (*models.ProductMapping, error) {
	return s.importUpstreamProduct(connectionID, upstreamProductID, categoryID, slug, false, nil)
}

// ImportUpstreamProductWithAutoCategory 从上游导入商品，并按上游分类自动创建/匹配本地分类。
func (s *ProductMappingService) ImportUpstreamProductWithAutoCategory(connectionID uint, upstreamProductID uint, categoryID uint, slug string, autoCreateCategory bool) (*models.ProductMapping, error) {
	return s.importUpstreamProduct(connectionID, upstreamProductID, categoryID, slug, autoCreateCategory, nil)
}

// importUpstreamProduct 内部导入实现。catMap 可由批量入口预先注入以避免 N+1 的上游 ListCategories 调用；
// 为 nil 时在需要时单次拉取。
func (s *ProductMappingService) importUpstreamProduct(connectionID uint, upstreamProductID uint, categoryID uint, slug string, autoCreateCategory bool, catMap map[uint]upstream.UpstreamCategory) (*models.ProductMapping, error) {
	// 检查是否已存在映射
	existing, err := s.mappingRepo.GetByConnectionAndUpstreamID(connectionID, upstreamProductID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrMappingAlreadyExists
	}

	// 获取连接
	conn, err := s.connService.GetByID(connectionID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, ErrConnectionNotFound
	}

	// 获取适配器
	adapter, err := s.connService.GetAdapter(conn)
	if err != nil {
		return nil, err
	}

	// 拉取上游商品
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	upProduct, err := adapter.GetProduct(ctx, upstreamProductID)
	if err != nil {
		// 上游已删除或已下架（旧上游兜底）→ 不允许导入
		if errors.Is(err, upstream.ErrUpstreamProductDeleted) || errors.Is(err, upstream.ErrUpstreamProductUnavailable) {
			return nil, ErrUpstreamProductNotFound
		}
		return nil, fmt.Errorf("fetch upstream product: %w", err)
	}
	if upProduct == nil {
		return nil, ErrUpstreamProductNotFound
	}
	// 新版上游对下架商品返回 200 + is_active=false → 同样禁止导入
	if !upProduct.IsActive {
		return nil, ErrUpstreamProductNotFound
	}

	if autoCreateCategory && categoryID == 0 && upProduct.CategoryID > 0 {
		if catMap == nil {
			fetched, fetchErr := s.fetchUpstreamCategoryMap(ctx, adapter)
			if fetchErr != nil {
				return nil, fmt.Errorf("auto create category: %w", fetchErr)
			}
			catMap = fetched
		}
		category, createErr := s.findOrCreateCategoryFromUpstream(upProduct.CategoryID, catMap)
		if createErr != nil {
			return nil, fmt.Errorf("auto create category: %w", createErr)
		}
		categoryID = category.ID
	}
	if err := validateProductCategoryAssignment(s.categoryRepo, categoryID, 0); err != nil {
		return nil, err
	}

	// 下载图片到本地
	localImages := s.downloadImages(ctx, adapter, upProduct.Images)

	// 下载 Content 中引用的图片
	localContent := s.downloadContentImages(ctx, adapter, upProduct.Content)

	// 确定交付类型：上游商品映射后统一使用 upstream 类型
	fulfillmentType := constants.FulfillmentTypeUpstream

	// 解析价格（先汇率转换，再应用加价比例）
	exchangeRate := conn.ExchangeRate
	markupPercent := conn.PriceMarkupPercent
	roundingMode := conn.PriceRoundingMode

	priceAmount, priceErr := decimal.NewFromString(upProduct.PriceAmount)
	if priceErr != nil {
		logger.Warnw("import_product_price_parse_error",
			"upstream_product_id", upstreamProductID,
			"price_amount", upProduct.PriceAmount,
			"error", priceErr,
		)
		priceAmount = decimal.Zero
	}
	costPriceAmount := convertCurrency(priceAmount, exchangeRate) // 成本价 = 上游价格 × 汇率（本地币种，不含加价）
	priceAmount = CalculateLocalPrice(priceAmount, exchangeRate, markupPercent, roundingMode)
	if priceAmount.LessThanOrEqual(decimal.Zero) && len(upProduct.SKUs) > 0 {
		// 取转换加价后 SKU 最低价
		for _, sku := range upProduct.SKUs {
			skuPrice, _ := decimal.NewFromString(sku.PriceAmount)
			localPrice := CalculateLocalPrice(skuPrice, exchangeRate, markupPercent, roundingMode)
			if localPrice.GreaterThan(decimal.Zero) && (priceAmount.IsZero() || localPrice.LessThan(priceAmount)) {
				priceAmount = localPrice
				costPriceAmount = convertCurrency(skuPrice, exchangeRate)
			}
		}
	}

	// 自动生成 slug（如果未提供）
	if slug == "" {
		slug = fmt.Sprintf("upstream-%d-%d-%d", connectionID, upstreamProductID, time.Now().UnixMilli())
	}

	// 创建本地商品
	product := models.Product{
		CategoryID:           categoryID,
		Slug:                 slug,
		SeoMetaJSON:          upProduct.SeoMeta,
		TitleJSON:            upProduct.Title,
		DescriptionJSON:      upProduct.Description,
		ContentJSON:          localContent,
		ManualFormSchemaJSON: upProduct.ManualFormSchema,
		PriceAmount:          models.NewMoneyFromDecimal(priceAmount.Round(2)),
		CostPriceAmount:      models.NewMoneyFromDecimal(costPriceAmount.Round(2)),
		WholesalePrices:      models.WholesalePriceTiers{},
		Images:               models.StringArray(localImages),
		Tags:                 models.StringArray(upProduct.Tags),
		PurchaseType:         constants.ProductPurchaseMember,
		FulfillmentType:      fulfillmentType,
		ManualStockTotal:     0,
		IsMapped:             true,
		IsActive:             false, // 默认下架，管理员手动上架
		SortOrder:            0,
	}

	var mapping *models.ProductMapping

	// 使用事务一次性创建本地商品、SKU、映射与 SKU 映射，避免留下半成功数据。
	if err := s.productRepo.Transaction(func(tx *gorm.DB) error {
		productRepo := s.productRepo.WithTx(tx)
		mappingRepo := s.mappingRepo.WithTx(tx)
		skuMappingRepo := s.skuMappingRepo.WithTx(tx)
		if err := productRepo.Create(&product); err != nil {
			return fmt.Errorf("create local product: %w", err)
		}

		// 创建 SKU
		skuRepo := s.productSKURepo.WithTx(tx)
		localSKUs := make([]models.ProductSKU, 0, len(upProduct.SKUs))
		for _, upSKU := range upProduct.SKUs {
			skuPrice, skuPriceErr := decimal.NewFromString(upSKU.PriceAmount)
			if skuPriceErr != nil {
				logger.Warnw("import_sku_price_parse_error",
					"upstream_sku_id", upSKU.ID,
					"price_amount", upSKU.PriceAmount,
					"error", skuPriceErr,
				)
				skuPrice = decimal.Zero
			}
			localPrice := CalculateLocalPrice(skuPrice, exchangeRate, markupPercent, roundingMode)
			localSKU := models.ProductSKU{
				ProductID:       product.ID,
				SKUCode:         upSKU.SKUCode,
				SpecValuesJSON:  upSKU.SpecValues,
				PriceAmount:     models.NewMoneyFromDecimal(localPrice.Round(2)),
				CostPriceAmount: models.NewMoneyFromDecimal(convertCurrency(skuPrice, exchangeRate).Round(2)), // 成本价 = 上游价格 × 汇率（本地币种）
				IsActive:        upSKU.IsActive,
				SortOrder:       0,
			}
			if err := skuRepo.Create(&localSKU); err != nil {
				return fmt.Errorf("create local sku: %w", err)
			}
			localSKUs = append(localSKUs, localSKU)
		}

		// 如果没有 SKU，创建默认 SKU
		if len(upProduct.SKUs) == 0 {
			defaultSKU := models.ProductSKU{
				ProductID:      product.ID,
				SKUCode:        models.DefaultSKUCode,
				SpecValuesJSON: models.JSON{},
				PriceAmount:    models.NewMoneyFromDecimal(priceAmount.Round(2)),
				IsActive:       true,
				SortOrder:      0,
			}
			if err := skuRepo.Create(&defaultSKU); err != nil {
				return fmt.Errorf("create default sku: %w", err)
			}
			localSKUs = append(localSKUs, defaultSKU)
		}

		if len(upProduct.WholesalePrices) > 0 {
			wholesalePrices := convertUpstreamWholesalePrices(
				upProduct.WholesalePrices,
				exchangeRate,
				markupPercent,
				roundingMode,
				buildUpstreamWholesaleSKUIndex(localSKUs, upProduct.SKUs, nil),
			)
			product.WholesalePrices = wholesalePrices
			if err := productRepo.QuickUpdate(
				fmt.Sprintf("%d", product.ID),
				map[string]interface{}{"wholesale_prices": wholesalePrices},
			); err != nil {
				return fmt.Errorf("update local product wholesale prices: %w", err)
			}
		}

		// 确定上游原始交付类型（auto/manual）
		upstreamFulfillmentType := upProduct.FulfillmentType
		if upstreamFulfillmentType != constants.FulfillmentTypeAuto {
			upstreamFulfillmentType = constants.FulfillmentTypeManual
		}

		now := time.Now()
		mapping = &models.ProductMapping{
			ConnectionID:            connectionID,
			LocalProductID:          product.ID,
			UpstreamProductID:       upstreamProductID,
			UpstreamFulfillmentType: upstreamFulfillmentType,
			IsActive:                true,
			LastSyncedAt:            &now,
		}
		if err := mappingRepo.Create(mapping); err != nil {
			return fmt.Errorf("create product mapping: %w", err)
		}
		if err := createSKUMappingsWithRepo(skuMappingRepo, mapping.ID, localSKUs, upProduct.SKUs); err != nil {
			return fmt.Errorf("create sku mappings: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return mapping, nil
}

// fetchUpstreamCategoryMap 拉取上游分类列表并构建 ID → 分类映射，供批量导入预取共用。
func (s *ProductMappingService) fetchUpstreamCategoryMap(ctx context.Context, adapter upstream.Adapter) (map[uint]upstream.UpstreamCategory, error) {
	catResult, err := adapter.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	catMap := make(map[uint]upstream.UpstreamCategory, len(catResult.Categories))
	for _, c := range catResult.Categories {
		catMap[c.ID] = c
	}
	return catMap, nil
}

func createSKUMappingsWithRepo(
	skuMappingRepo repository.SKUMappingRepository,
	mappingID uint,
	localSKUs []models.ProductSKU,
	upstreamSKUs []upstream.UpstreamSKU,
) error {
	if skuMappingRepo == nil {
		return nil
	}
	if len(localSKUs) == 0 || len(upstreamSKUs) == 0 {
		return nil
	}

	// 按 SKUCode 匹配
	upstreamByCode := make(map[string]upstream.UpstreamSKU, len(upstreamSKUs))
	for _, us := range upstreamSKUs {
		upstreamByCode[strings.ToLower(strings.TrimSpace(us.SKUCode))] = us
	}

	for _, localSKU := range localSKUs {
		code := strings.ToLower(strings.TrimSpace(localSKU.SKUCode))
		upSKU, ok := upstreamByCode[code]
		if !ok {
			// 如果只有一个 SKU（DEFAULT），匹配第一个上游 SKU
			if len(localSKUs) == 1 && len(upstreamSKUs) == 1 {
				upSKU = upstreamSKUs[0]
			} else {
				continue
			}
		}

		upPrice, _ := decimal.NewFromString(upSKU.PriceAmount)
		now := time.Now()
		skuMapping := &models.SKUMapping{
			ProductMappingID: mappingID,
			LocalSKUID:       localSKU.ID,
			UpstreamSKUID:    upSKU.ID,
			UpstreamPrice:    models.NewMoneyFromDecimal(upPrice.Round(2)),
			UpstreamIsActive: upSKU.IsActive,
			UpstreamStock:    upSKU.StockQuantity,
			StockSyncedAt:    &now,
		}
		if err := skuMappingRepo.Create(skuMapping); err != nil {
			return err
		}
	}

	return nil
}

type upstreamWholesaleSKURef struct {
	ID      uint
	SKUCode string
}

type upstreamWholesaleSKUIndex struct {
	byUpstreamID map[uint]upstreamWholesaleSKURef
	byCode       map[string]upstreamWholesaleSKURef
}

func buildUpstreamWholesaleSKUIndex(localSKUs []models.ProductSKU, upstreamSKUs []upstream.UpstreamSKU, skuMappings []models.SKUMapping) upstreamWholesaleSKUIndex {
	index := upstreamWholesaleSKUIndex{
		byUpstreamID: map[uint]upstreamWholesaleSKURef{},
		byCode:       map[string]upstreamWholesaleSKURef{},
	}
	localByID := make(map[uint]models.ProductSKU, len(localSKUs))
	localByCode := make(map[string]models.ProductSKU, len(localSKUs))
	for _, sku := range localSKUs {
		code := strings.TrimSpace(sku.SKUCode)
		if sku.ID > 0 {
			localByID[sku.ID] = sku
		}
		if code != "" {
			ref := upstreamWholesaleSKURef{ID: sku.ID, SKUCode: code}
			key := strings.ToLower(code)
			localByCode[key] = sku
			index.byCode[key] = ref
		}
	}

	for _, mapping := range skuMappings {
		localSKU, ok := localByID[mapping.LocalSKUID]
		if !ok {
			continue
		}
		code := strings.TrimSpace(localSKU.SKUCode)
		index.byUpstreamID[mapping.UpstreamSKUID] = upstreamWholesaleSKURef{ID: localSKU.ID, SKUCode: code}
	}

	for _, upSKU := range upstreamSKUs {
		if _, ok := index.byUpstreamID[upSKU.ID]; ok {
			continue
		}
		if localSKU, ok := localByCode[strings.ToLower(strings.TrimSpace(upSKU.SKUCode))]; ok {
			index.byUpstreamID[upSKU.ID] = upstreamWholesaleSKURef{ID: localSKU.ID, SKUCode: strings.TrimSpace(localSKU.SKUCode)}
			continue
		}
		if len(localSKUs) == 1 && len(upstreamSKUs) == 1 {
			localSKU := localSKUs[0]
			index.byUpstreamID[upSKU.ID] = upstreamWholesaleSKURef{ID: localSKU.ID, SKUCode: strings.TrimSpace(localSKU.SKUCode)}
		}
	}
	return index
}

func convertUpstreamWholesalePrices(tiers models.WholesalePriceTiers, exchangeRate, markupPercent decimal.Decimal, roundingMode string, indexes ...upstreamWholesaleSKUIndex) models.WholesalePriceTiers {
	if len(tiers) == 0 {
		return models.WholesalePriceTiers{}
	}
	index := upstreamWholesaleSKUIndex{}
	if len(indexes) > 0 {
		index = indexes[0]
	}
	converted := make([]WholesalePriceInput, 0, len(tiers))
	skipped := 0
	for idx, tier := range tiers {
		if tier.MinQuantity <= 0 || tier.UnitPrice.Decimal.LessThanOrEqual(decimal.Zero) {
			skipped++
			logger.Warnw("convert_upstream_wholesale_price_invalid",
				"index", idx,
				"min_quantity", tier.MinQuantity,
				"unit_price", tier.UnitPrice.String(),
			)
			continue
		}
		localPrice := CalculateLocalPrice(tier.UnitPrice.Decimal, exchangeRate, markupPercent, roundingMode)
		if localPrice.LessThanOrEqual(decimal.Zero) {
			skipped++
			logger.Warnw("convert_upstream_wholesale_price_invalid",
				"index", idx,
				"min_quantity", tier.MinQuantity,
				"unit_price", tier.UnitPrice.String(),
				"local_price", localPrice.String(),
			)
			continue
		}
		localSKUID, localSKUCode, ok := resolveUpstreamWholesaleTierScope(tier, index)
		if !ok {
			skipped++
			logger.Warnw("convert_upstream_wholesale_price_sku_unmapped",
				"index", idx,
				"upstream_sku_id", tier.SKUID,
				"sku_code", tier.SKUCode,
				"min_quantity", tier.MinQuantity,
			)
			continue
		}
		converted = append(converted, WholesalePriceInput{
			SKUID:       localSKUID,
			SKUCode:     localSKUCode,
			MinQuantity: tier.MinQuantity,
			UnitPrice:   localPrice,
		})
	}
	normalized, err := normalizeWholesalePriceInputs(converted)
	if err != nil {
		logger.Warnw("convert_upstream_wholesale_prices_failed",
			"error", err,
			"tier_count", len(tiers),
			"valid_tier_count", len(converted),
			"skipped_tier_count", skipped,
		)
		return models.WholesalePriceTiers{}
	}
	if skipped > 0 {
		logger.Warnw("convert_upstream_wholesale_prices_skipped_invalid",
			"tier_count", len(tiers),
			"valid_tier_count", len(converted),
			"skipped_tier_count", skipped,
		)
	}
	return normalized
}

func resolveUpstreamWholesaleTierScope(tier models.WholesalePriceTier, index upstreamWholesaleSKUIndex) (uint, string, bool) {
	hasIndex := len(index.byCode) > 0 || len(index.byUpstreamID) > 0
	skuCode := strings.TrimSpace(tier.SKUCode)
	if skuCode != "" {
		if ref, ok := index.byCode[strings.ToLower(skuCode)]; ok {
			if tier.SKUID > 0 {
				if idRef, idOK := index.byUpstreamID[tier.SKUID]; idOK && idRef.ID != ref.ID {
					return 0, "", false
				}
			}
			return ref.ID, strings.TrimSpace(ref.SKUCode), true
		}
		if hasIndex {
			return 0, "", false
		}
		return 0, skuCode, true
	}
	if tier.SKUID > 0 {
		if ref, ok := index.byUpstreamID[tier.SKUID]; ok {
			return ref.ID, strings.TrimSpace(ref.SKUCode), true
		}
		return 0, "", false
	}
	return 0, "", true
}

// downloadImages 下载上游图片到本地
func (s *ProductMappingService) downloadImages(ctx context.Context, adapter upstream.Adapter, images []string) []string {
	var localImages []string
	for _, img := range images {
		if strings.TrimSpace(img) == "" {
			continue
		}
		localPath, err := adapter.DownloadImage(ctx, img)
		if err != nil {
			// 下载失败保留原始 URL
			localImages = append(localImages, img)
			continue
		}
		if s.mediaService != nil {
			s.mediaService.RecordLocalFile(localPath, "upstream")
		}
		localImages = append(localImages, localPath)
	}
	return localImages
}

// downloadContentImages 下载多语言 Content 中的图片并替换 URL
func (s *ProductMappingService) downloadContentImages(ctx context.Context, adapter upstream.Adapter, content models.JSON) models.JSON {
	if len(content) == 0 {
		return content
	}

	// models.JSON 是 map[string]interface{}，值为各语言的 Markdown 文本
	imgRegex := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)|<img[^>]+src=["']([^"']+)["']`)
	downloaded := make(map[string]string) // originalURL -> localPath

	// 第一遍：收集所有唯一图片 URL
	for _, val := range content {
		text, ok := val.(string)
		if !ok || text == "" {
			continue
		}
		matches := imgRegex.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			url := m[1]
			if url == "" {
				url = m[2]
			}
			if url == "" || strings.HasPrefix(url, "/uploads/") {
				continue
			}
			downloaded[url] = "" // 占位
		}
	}

	if len(downloaded) == 0 {
		return content
	}

	// 下载图片
	for url := range downloaded {
		localPath, err := adapter.DownloadImage(ctx, url)
		if err != nil {
			downloaded[url] = url // 失败保留原始
		} else {
			if s.mediaService != nil {
				s.mediaService.RecordLocalFile(localPath, "upstream")
			}
			downloaded[url] = localPath
		}
	}

	// 第二遍：替换所有语言文本中的 URL
	result := make(models.JSON, len(content))
	for lang, val := range content {
		text, ok := val.(string)
		if !ok {
			result[lang] = val
			continue
		}
		for original, local := range downloaded {
			if original != local {
				text = strings.ReplaceAll(text, original, local)
			}
		}
		result[lang] = text
	}

	return result
}

// ListUpstreamProducts 通过连接代理拉取上游商品列表（分页）
func (s *ProductMappingService) ListUpstreamProducts(connectionID uint, page, pageSize int) (*upstream.ProductListResult, error) {
	conn, err := s.connService.GetByID(connectionID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, ErrConnectionNotFound
	}

	adapter, err := s.connService.GetAdapter(conn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return adapter.ListProducts(ctx, upstream.ListProductsOpts{
		Page:     page,
		PageSize: pageSize,
	})
}

// ListUpstreamCategories 通过连接代理拉取上游分类列表
// 返回 (categories, supported, error)，supported 为 false 表示上游不支持分类 API
func (s *ProductMappingService) ListUpstreamCategories(connectionID uint) ([]upstream.UpstreamCategory, bool, error) {
	conn, err := s.connService.GetByID(connectionID)
	if err != nil {
		return nil, false, err
	}
	if conn == nil {
		return nil, false, ErrConnectionNotFound
	}

	adapter, err := s.connService.GetAdapter(conn)
	if err != nil {
		return nil, false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := adapter.ListCategories(ctx)
	if err != nil {
		return nil, false, err
	}

	return result.Categories, result.Supported, nil
}

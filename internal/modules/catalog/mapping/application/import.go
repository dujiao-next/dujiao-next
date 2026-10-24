package application

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/upstream"

	"github.com/shopspring/decimal"
)

var importSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// pickImportSlug 依次尝试 上游 slug → slug-上游ID → upstream-连接ID-上游ID-毫秒时间戳。
func (s *Service) pickImportSlug(upstreamSlug string, connectionID, upstreamProductID uint) (string, error) {
	upstreamSlug = strings.ToLower(strings.TrimSpace(upstreamSlug))
	if len(upstreamSlug) <= 100 && importSlugPattern.MatchString(upstreamSlug) {
		for _, candidate := range []string{upstreamSlug, fmt.Sprintf("%s-%d", upstreamSlug, upstreamProductID)} {
			count, err := s.products.CountBySlug(candidate, nil)
			if err != nil {
				return "", err
			}
			if count == 0 {
				return candidate, nil
			}
		}
	}
	return fmt.Sprintf("upstream-%d-%d-%d", connectionID, upstreamProductID, time.Now().UnixMilli()), nil
}

// ImportUpstreamProduct 从上游导入商品（克隆为本地商品 + 建立映射）
func (s *Service) ImportUpstreamProduct(connectionID uint, upstreamProductID uint, categoryID uint, slug string) (*mappingdomain.Mapping, error) {
	return s.importUpstreamProduct(connectionID, upstreamProductID, categoryID, slug, false, nil)
}

// ImportUpstreamProductWithAutoCategory 从上游导入商品，并按上游分类自动创建/匹配本地分类。
func (s *Service) ImportUpstreamProductWithAutoCategory(connectionID uint, upstreamProductID uint, categoryID uint, slug string, autoCreateCategory bool) (*mappingdomain.Mapping, error) {
	return s.importUpstreamProduct(connectionID, upstreamProductID, categoryID, slug, autoCreateCategory, nil)
}

// importUpstreamProduct 内部导入实现。catMap 可由批量入口预先注入以避免 N+1 的上游 ListCategories 调用；
// 为 nil 时在需要时单次拉取。
func (s *Service) importUpstreamProduct(connectionID uint, upstreamProductID uint, categoryID uint, slug string, autoCreateCategory bool, catMap map[uint]upstream.UpstreamCategory) (*mappingdomain.Mapping, error) {
	// 检查是否已存在映射
	existing, err := s.mappings.GetByConnectionAndUpstreamID(connectionID, upstreamProductID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, mappingcontract.ErrMappingAlreadyExists
	}

	// 获取连接
	conn, err := s.connections.GetByID(connectionID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, siteconnectioncontract.ErrNotFound
	}

	// 获取适配器
	adapter, err := s.connections.GetAdapter(conn)
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
			return nil, mappingcontract.ErrUpstreamProductNotFound
		}
		return nil, fmt.Errorf("fetch upstream product: %w", err)
	}
	if upProduct == nil {
		return nil, mappingcontract.ErrUpstreamProductNotFound
	}
	// 新版上游对下架商品返回 200 + is_active=false → 同样禁止导入
	if !upProduct.IsActive {
		return nil, mappingcontract.ErrUpstreamProductNotFound
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
	if err := productdomain.ValidateCategoryAssignment(s.categories, categoryID, 0, productcontract.ErrProductCategoryInvalid); err != nil {
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

	// 未指定 slug 时优先沿用上游 slug，不可用再回退自动生成
	if slug == "" {
		slug, err = s.pickImportSlug(upProduct.Slug, connectionID, upstreamProductID)
		if err != nil {
			return nil, err
		}
	}

	// 创建本地商品
	product := productdomain.Product{
		CategoryID:           categoryID,
		Slug:                 slug,
		SeoMetaJSON:          upProduct.SeoMeta,
		TitleJSON:            upProduct.Title,
		DescriptionJSON:      upProduct.Description,
		ContentJSON:          localContent,
		ManualFormSchemaJSON: upProduct.ManualFormSchema,
		PriceAmount:          money.FromDecimal(priceAmount.Round(2)),
		CostPriceAmount:      money.FromDecimal(costPriceAmount.Round(2)),
		WholesalePrices:      productdomain.WholesalePriceTiers{},
		Images:               jsonslice.Strings(localImages),
		Tags:                 jsonslice.Strings(upProduct.Tags),
		PurchaseType:         constants.ProductPurchaseMember,
		FulfillmentType:      fulfillmentType,
		ManualStockTotal:     0,
		IsMapped:             true,
		IsActive:             false, // 默认下架，管理员手动上架
		SortOrder:            0,
	}

	var mapping *mappingdomain.Mapping

	// 使用事务一次性创建本地商品、SKU、映射与 SKU 映射，避免留下半成功数据。
	if err := s.transactions.WithinTransaction(func(repos mappingcontract.ImportRepositories) error {
		if err := repos.Products.Create(&product); err != nil {
			return fmt.Errorf("create local product: %w", err)
		}

		// 创建 SKU
		localSKUs := make([]productdomain.ProductSKU, 0, len(upProduct.SKUs))
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
			localSKU := productdomain.ProductSKU{
				ProductID:       product.ID,
				SKUCode:         upSKU.SKUCode,
				SpecValuesJSON:  upSKU.SpecValues,
				PriceAmount:     money.FromDecimal(localPrice.Round(2)),
				CostPriceAmount: money.FromDecimal(convertCurrency(skuPrice, exchangeRate).Round(2)), // 成本价 = 上游价格 × 汇率（本地币种）
				IsActive:        upSKU.IsActive,
				SortOrder:       0,
			}
			if err := repos.SKUs.Create(&localSKU); err != nil {
				return fmt.Errorf("create local sku: %w", err)
			}
			localSKUs = append(localSKUs, localSKU)
		}

		// 如果没有 SKU，创建默认 SKU
		if len(upProduct.SKUs) == 0 {
			defaultSKU := productdomain.ProductSKU{
				ProductID:      product.ID,
				SKUCode:        productdomain.DefaultSKUCode,
				SpecValuesJSON: jsonmap.JSON{},
				PriceAmount:    money.FromDecimal(priceAmount.Round(2)),
				IsActive:       true,
				SortOrder:      0,
			}
			if err := repos.SKUs.Create(&defaultSKU); err != nil {
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
			if err := repos.Products.QuickUpdate(
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
		mapping = &mappingdomain.Mapping{
			ConnectionID:            connectionID,
			LocalProductID:          product.ID,
			UpstreamProductID:       upstreamProductID,
			UpstreamFulfillmentType: upstreamFulfillmentType,
			IsActive:                true,
			LastSyncedAt:            &now,
		}
		if err := repos.Mappings.Create(mapping); err != nil {
			return fmt.Errorf("create product mapping: %w", err)
		}
		if err := createSKUMappings(repos.SKUMappings, mapping.ID, localSKUs, upProduct.SKUs); err != nil {
			return fmt.Errorf("create sku mappings: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return mapping, nil
}

// fetchUpstreamCategoryMap 拉取上游分类列表并构建 ID → 分类映射，供批量导入预取共用。
func (s *Service) fetchUpstreamCategoryMap(ctx context.Context, adapter upstream.Adapter) (map[uint]upstream.UpstreamCategory, error) {
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

func createSKUMappings(
	skuMappings mappingcontract.ImportTxSKUMappingRepository,
	mappingID uint,
	localSKUs []productdomain.ProductSKU,
	upstreamSKUs []upstream.UpstreamSKU,
) error {
	if skuMappings == nil {
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
		skuMapping := &mappingdomain.SKUMapping{
			ProductMappingID: mappingID,
			LocalSKUID:       localSKU.ID,
			UpstreamSKUID:    upSKU.ID,
			UpstreamPrice:    money.FromDecimal(upPrice.Round(2)),
			UpstreamIsActive: upSKU.IsActive,
			UpstreamStock:    upSKU.StockQuantity,
			StockSyncedAt:    &now,
		}
		if err := skuMappings.Create(skuMapping); err != nil {
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

func buildUpstreamWholesaleSKUIndex(localSKUs []productdomain.ProductSKU, upstreamSKUs []upstream.UpstreamSKU, skuMappings []mappingdomain.SKUMapping) upstreamWholesaleSKUIndex {
	index := upstreamWholesaleSKUIndex{
		byUpstreamID: map[uint]upstreamWholesaleSKURef{},
		byCode:       map[string]upstreamWholesaleSKURef{},
	}
	localByID := make(map[uint]productdomain.ProductSKU, len(localSKUs))
	localByCode := make(map[string]productdomain.ProductSKU, len(localSKUs))
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

func convertUpstreamWholesalePrices(tiers productdomain.WholesalePriceTiers, exchangeRate, markupPercent decimal.Decimal, roundingMode string, indexes ...upstreamWholesaleSKUIndex) productdomain.WholesalePriceTiers {
	if len(tiers) == 0 {
		return productdomain.WholesalePriceTiers{}
	}
	index := upstreamWholesaleSKUIndex{}
	if len(indexes) > 0 {
		index = indexes[0]
	}
	converted := make([]productdomain.WholesalePriceInput, 0, len(tiers))
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
		converted = append(converted, productdomain.WholesalePriceInput{
			SKUID:       localSKUID,
			SKUCode:     localSKUCode,
			MinQuantity: tier.MinQuantity,
			UnitPrice:   localPrice,
		})
	}
	normalized, err := productdomain.NormalizeWholesalePrices(converted)
	if err != nil {
		logger.Warnw("convert_upstream_wholesale_prices_failed",
			"error", err,
			"tier_count", len(tiers),
			"valid_tier_count", len(converted),
			"skipped_tier_count", skipped,
		)
		return productdomain.WholesalePriceTiers{}
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

func resolveUpstreamWholesaleTierScope(tier productdomain.WholesalePriceTier, index upstreamWholesaleSKUIndex) (uint, string, bool) {
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
func (s *Service) downloadImages(ctx context.Context, adapter upstream.Adapter, images []string) []string {
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
		s.recordUpstreamMedia(ctx, localPath)
		localImages = append(localImages, localPath)
	}
	return localImages
}

// downloadContentImages 下载多语言 Content 中的图片并替换 URL
func (s *Service) downloadContentImages(ctx context.Context, adapter upstream.Adapter, content jsonmap.JSON) jsonmap.JSON {
	if len(content) == 0 {
		return content
	}

	// jsonmap.JSON 是 map[string]interface{}，值为各语言的 Markdown 文本
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
			s.recordUpstreamMedia(ctx, localPath)
			downloaded[url] = localPath
		}
	}

	// 第二遍：替换所有语言文本中的 URL
	result := make(jsonmap.JSON, len(content))
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

func (s *Service) recordUpstreamMedia(ctx context.Context, localPath string) {
	s.media.RecordLocalFile(ctx, localPath, "upstream")
}

// ListUpstreamProducts 通过连接代理拉取上游商品列表（分页）
func (s *Service) ListUpstreamProducts(connectionID uint, page, pageSize int) (*upstream.ProductListResult, error) {
	conn, err := s.connections.GetByID(connectionID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, siteconnectioncontract.ErrNotFound
	}

	adapter, err := s.connections.GetAdapter(conn)
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
func (s *Service) ListUpstreamCategories(connectionID uint) ([]upstream.UpstreamCategory, bool, error) {
	conn, err := s.connections.GetByID(connectionID)
	if err != nil {
		return nil, false, err
	}
	if conn == nil {
		return nil, false, siteconnectioncontract.ErrNotFound
	}

	adapter, err := s.connections.GetAdapter(conn)
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

// ListUpstreamCategoryCounts 拉取上游全部商品并按分类统计数量。
// 管理端只加载首页商品用于预览，不代表分类下商品的真实数量，所以这里遍历全部分页。
// ctx 取自请求：管理端关闭弹窗或切换连接后，剩余分页不再继续拉取。
func (s *Service) ListUpstreamCategoryCounts(ctx context.Context, connectionID uint) (map[uint]int, int, error) {
	conn, err := s.connections.GetByID(connectionID)
	if err != nil {
		return nil, 0, err
	}
	if conn == nil {
		return nil, 0, siteconnectioncontract.ErrNotFound
	}

	adapter, err := s.connections.GetAdapter(conn)
	if err != nil {
		return nil, 0, err
	}

	products, err := listAllUpstreamProducts(ctx, adapter)
	if err != nil {
		return nil, 0, err
	}
	counts := make(map[uint]int)
	for _, p := range products {
		counts[p.CategoryID]++
	}
	return counts, len(products), nil
}

// maxUpstreamProductPages 全量翻页的页数上限（每页 50 条，即 5 万条商品）。
// 上游若忽略 page 参数且 total 异常，翻页条件永远不满足，靠它兜底防止死循环。
const maxUpstreamProductPages = 1000

// listAllUpstreamProducts 逐页拉取上游全部商品。上游 page_size 上限为 50。
func listAllUpstreamProducts(ctx context.Context, adapter upstream.Adapter) ([]upstream.UpstreamProduct, error) {
	const pageSize = 50
	var products []upstream.UpstreamProduct
	for page := 1; page <= maxUpstreamProductPages; page++ {
		pageCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		result, err := adapter.ListProducts(pageCtx, upstream.ListProductsOpts{
			Page:     page,
			PageSize: pageSize,
		})
		cancel()
		if err != nil {
			return nil, fmt.Errorf("fetch upstream products page %d: %w", page, err)
		}
		products = append(products, result.Items...)
		if len(result.Items) < pageSize || page*pageSize >= result.Total {
			return products, nil
		}
	}
	return nil, fmt.Errorf("upstream products exceed %d pages", maxUpstreamProductPages)
}

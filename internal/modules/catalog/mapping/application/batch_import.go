package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"

	categoryapp "github.com/dujiao-next/internal/modules/catalog/category/application"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/upstream"
)

// acquireBatchImportLock 同一连接同时只允许一个批量导入：大批量导入常超过 HTTP 超时，
// 管理员重试时后台仍在跑上一批，并发导入会产生重复商品。
func acquireBatchImportLock(connectionID uint) (func(), error) {
	ctx := context.Background()
	key := fmt.Sprintf("mapping:batch_import:%d", connectionID)
	locked, err := cache.SetNX(ctx, key, "1", 2*time.Hour)
	if err != nil {
		// Redis 不可用时降级为不加锁
		return func() {}, nil
	}
	if !locked {
		return nil, mappingcontract.ErrBatchImportInProgress
	}
	return func() { _ = cache.Del(ctx, key) }, nil
}

// BatchUpstreamProductImportOutcome 单个商品批量导入结果（供 handler 组装响应）
type BatchUpstreamProductImportOutcome struct {
	UpstreamProductID         uint
	UpstreamProductExternalID string
	Mapping                   *mappingdomain.Mapping
	Err                       error
}

// BatchImportUpstreamProducts 批量按 ID 导入上游商品。
// 当 autoCreateCategory 为 true 且未指定本地分类时，会一次性预拉取上游分类列表，
// 避免逐个商品都触发一次 ListCategories（N+1）。
func (s *Service) BatchImportUpstreamProducts(
	connectionID uint,
	upstreamProductIDs []uint,
	categoryID uint,
	autoCreateCategory bool,
) ([]BatchUpstreamProductImportOutcome, error) {
	if len(upstreamProductIDs) == 0 {
		return nil, nil
	}
	release, err := acquireBatchImportLock(connectionID)
	if err != nil {
		return nil, err
	}
	defer release()

	var catMap map[uint]upstream.UpstreamCategory
	if autoCreateCategory && categoryID == 0 {
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
		fetched, fetchErr := s.fetchUpstreamCategoryMap(ctx, adapter)
		if fetchErr != nil {
			return nil, fmt.Errorf("prefetch upstream categories: %w", fetchErr)
		}
		catMap = fetched
	}

	outcomes := make([]BatchUpstreamProductImportOutcome, 0, len(upstreamProductIDs))
	for _, id := range upstreamProductIDs {
		mapping, importErr := s.importUpstreamProduct(connectionID, id, categoryID, "", autoCreateCategory, catMap)
		outcomes = append(outcomes, BatchUpstreamProductImportOutcome{
			UpstreamProductID: id,
			Mapping:           mapping,
			Err:               importErr,
		})
	}
	return outcomes, nil
}

// BatchImportUpstreamProductsByExternalID 批量导入使用字符串外部 ID 的上游商品。
// 外部 ID 原样传递和落库，不转换为 uint。
func (s *Service) BatchImportUpstreamProductsByExternalID(
	connectionID uint,
	upstreamProductExternalIDs []string,
	categoryID uint,
) ([]BatchUpstreamProductImportOutcome, error) {
	if len(upstreamProductExternalIDs) == 0 {
		return nil, nil
	}
	release, err := acquireBatchImportLock(connectionID)
	if err != nil {
		return nil, err
	}
	defer release()

	outcomes := make([]BatchUpstreamProductImportOutcome, 0, len(upstreamProductExternalIDs))
	for _, externalID := range upstreamProductExternalIDs {
		externalID = strings.TrimSpace(externalID)
		mapping, importErr := s.ImportUpstreamProductByExternalID(connectionID, externalID, categoryID, "")
		outcomes = append(outcomes, BatchUpstreamProductImportOutcome{
			UpstreamProductExternalID: externalID,
			Mapping:                   mapping,
			Err:                       importErr,
		})
	}
	return outcomes, nil
}

// BatchImportByCategoryResult 按分类批量导入结果
type BatchImportByCategoryResult struct {
	Total        int    `json:"total"`
	SuccessCount int    `json:"success_count"`
	CategoryID   uint   `json:"category_id"`
	CategoryName string `json:"category_name,omitempty"`
	Errors       []struct {
		UpstreamProductID uint   `json:"upstream_product_id"`
		Error             string `json:"error"`
	} `json:"errors,omitempty"`
}

// BatchImportByCategory 按上游分类批量导入商品
func (s *Service) BatchImportByCategory(
	connectionID uint,
	upstreamCategoryID uint,
	autoCreateCategory bool,
	localCategoryID uint,
) (*BatchImportByCategoryResult, error) {
	release, err := acquireBatchImportLock(connectionID)
	if err != nil {
		return nil, err
	}
	defer release()
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

	// 分页拉取上游所有商品，筛选属于目标分类的
	allProducts, err := listAllUpstreamProducts(context.Background(), adapter)
	if err != nil {
		return nil, err
	}
	var targetProducts []upstream.UpstreamProduct
	for _, p := range allProducts {
		if p.CategoryID == upstreamCategoryID {
			targetProducts = append(targetProducts, p)
		}
	}

	if len(targetProducts) == 0 {
		return &BatchImportByCategoryResult{Total: 0, SuccessCount: 0}, nil
	}

	// 确定本地分类 ID
	categoryID := localCategoryID
	categoryName := ""

	if autoCreateCategory && categoryID == 0 {
		// 拉取上游分类列表用于自动创建
		catCtx, catCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer catCancel()
		catResult, catErr := adapter.ListCategories(catCtx)
		if catErr != nil {
			return nil, fmt.Errorf("fetch upstream categories: %w", catErr)
		}
		catMap := make(map[uint]upstream.UpstreamCategory)
		for _, c := range catResult.Categories {
			catMap[c.ID] = c
		}
		cat, createErr := s.findOrCreateCategoryFromUpstream(upstreamCategoryID, catMap)
		if createErr != nil {
			return nil, fmt.Errorf("auto create category: %w", createErr)
		}
		categoryID = cat.ID
		if nameMap, ok := cat.NameJSON["zh-CN"]; ok {
			if n, ok := nameMap.(string); ok {
				categoryName = n
			}
		}
	}

	// 逐个导入
	result := &BatchImportByCategoryResult{
		Total:        len(targetProducts),
		CategoryID:   categoryID,
		CategoryName: categoryName,
	}
	for _, p := range targetProducts {
		_, importErr := s.ImportUpstreamProduct(connectionID, p.ID, categoryID, "")
		if importErr != nil {
			if errors.Is(importErr, mappingcontract.ErrMappingAlreadyExists) {
				result.SuccessCount++ // 已映射的算成功
				continue
			}
			result.Errors = append(result.Errors, struct {
				UpstreamProductID uint   `json:"upstream_product_id"`
				Error             string `json:"error"`
			}{
				UpstreamProductID: p.ID,
				Error:             importErr.Error(),
			})
		} else {
			result.SuccessCount++
		}
	}

	return result, nil
}

// findOrCreateCategoryFromUpstream 根据上游分类信息查找或创建本地分类
func (s *Service) findOrCreateCategoryFromUpstream(
	upstreamCategoryID uint, catMap map[uint]upstream.UpstreamCategory,
) (*categorydomain.Category, error) {
	target, ok := catMap[upstreamCategoryID]
	if !ok {
		return nil, fmt.Errorf("upstream category %d not found", upstreamCategoryID)
	}

	// 如果上游分类有父分类，先确保父分类存在
	var localParentID uint
	if target.ParentID > 0 {
		if parent, parentOK := catMap[target.ParentID]; parentOK {
			parentCat, parentErr := s.findOrCreateLocalCategory(parent.Slug, parent.Name, 0)
			if parentErr != nil {
				return nil, fmt.Errorf("create parent category: %w", parentErr)
			}
			localParentID = parentCat.ID
		}
	}

	return s.findOrCreateLocalCategory(target.Slug, target.Name, localParentID)
}

// findOrCreateLocalCategory 按 slug 查找或创建本地分类
func (s *Service) findOrCreateLocalCategory(slug string, nameJSON jsonmap.JSON, parentID uint) (*categorydomain.Category, error) {
	// 先查找是否已存在同 slug 分类
	existing, err := s.categories.GetBySlug(slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	deleted, err := s.categories.GetBySlugUnscoped(slug)
	if err != nil {
		return nil, err
	}
	if deleted != nil {
		deleted.ParentID = parentID
		deleted.NameJSON = nameJSON
		deleted.IsActive = true
		if err := s.categories.Restore(deleted); err != nil {
			return nil, err
		}
		return deleted, nil
	}

	// 不存在，创建新分类
	if s.categoryCreator == nil {
		return nil, fmt.Errorf("category service not available")
	}

	cat, err := s.categoryCreator.Create(categoryapp.UpsertInput{
		ParentID: parentID,
		Slug:     slug,
		NameJSON: map[string]interface{}(nameJSON),
	})
	if err != nil {
		// slug 冲突，追加后缀重试
		if errors.Is(err, categoryapp.ErrSlugExists) {
			for i := 2; i <= 10; i++ {
				suffixedSlug := fmt.Sprintf("%s-%d", slug, i)
				cat, err = s.categoryCreator.Create(categoryapp.UpsertInput{
					ParentID: parentID,
					Slug:     suffixedSlug,
					NameJSON: map[string]interface{}(nameJSON),
				})
				if err == nil {
					return cat, nil
				}
				if !errors.Is(err, categoryapp.ErrSlugExists) {
					return nil, err
				}
			}
			return nil, fmt.Errorf("slug conflict after retries: %s", slug)
		}
		return nil, err
	}
	return cat, nil
}

type autoCategoryDefinition struct {
	Slug          string
	NameJSON      jsonmap.JSON
	TitleKeywords []string
	Keywords      []string
}

var autoCategoryDefinitions = []autoCategoryDefinition{
	{
		Slug: "ai-api",
		NameJSON: jsonmap.JSON{
			"zh-CN": "AI 工具",
			"zh-TW": "AI 工具",
			"en-US": "AI Tools",
		},
		TitleKeywords: []string{
			"chatgpt", "claude", "openai", "gemini", "midjourney", "sora", "cursor", "copilot", "codex", "elevenlabs", "muse ai", "framer ai", "gpt", "llm", "ai api",
		},
		Keywords: []string{
			"人工智能", "生成式", "ai api",
		},
	},
	{
		Slug: "streaming",
		NameJSON: jsonmap.JSON{
			"zh-CN": "流媒体会员",
			"zh-TW": "串流會員",
			"en-US": "Streaming",
		},
		TitleKeywords: []string{
			"netflix", "spotify", "youtube", "apple tv", "amazon prime", "prime video", "disney", "hbo", "shahid", "scribd", "crunchyroll", "stream", "流媒体", "串流",
		},
		Keywords: []string{
			"netflix", "spotify", "youtube", "apple tv", "amazon prime", "prime video", "disney", "hbo", "shahid", "scribd", "crunchyroll", "stream", "流媒体", "串流",
		},
	},
	{
		Slug: "email-accounts",
		NameJSON: jsonmap.JSON{
			"zh-CN": "邮箱与账号",
			"zh-TW": "電郵與帳號",
			"en-US": "Email & Accounts",
		},
		TitleKeywords: []string{
			"email", "gmail", "outlook", "hotmail", "mailbox", "mails", "邮箱", "電郵", "账号", "帳號",
		},
		Keywords: []string{
			"email", "gmail", "outlook", "hotmail", "邮箱", "帳號", "電郵",
		},
	},
	{
		Slug: "vpn-proxy",
		NameJSON: jsonmap.JSON{
			"zh-CN": "网络工具",
			"zh-TW": "網路工具",
			"en-US": "VPN & Proxy",
		},
		TitleKeywords: []string{
			"vpn", "proxy", "v2ray", "trojan", "wireguard", "shadowsocks", "hma", "nord", "expressvpn", "rotating proxy", "节点", "代理",
		},
		Keywords: []string{
			"vpn", "proxy", "v2ray", "trojan", "wireguard", "shadowsocks", "节点", "代理",
		},
	},
	{
		Slug: "software",
		NameJSON: jsonmap.JSON{
			"zh-CN": "软件与激活",
			"zh-TW": "軟體與啟用",
			"en-US": "Software & Licenses",
		},
		TitleKeywords: []string{
			"windows", "office", "adobe", "canva", "capcut", "autodesk", "jetbrains", "figma", "notion", "miro", "avira", "ilovepdf", "wispr", "software", "license", "activation", "steam", "game", "软件", "軟體", "激活", "游戏",
		},
		Keywords: []string{
			"windows", "office", "adobe", "canva", "software", "license", "activation", "激活", "软件", "軟體", "steam", "game", "游戏",
		},
	},
	{
		Slug: "developer-tools",
		NameJSON: jsonmap.JSON{
			"zh-CN": "开发者工具",
			"zh-TW": "開發者工具",
			"en-US": "Developer Tools",
		},
		TitleKeywords: []string{
			"github", "gitlab", "npm", "docker", "developer", "开发", "開發", "代码", "程式",
		},
		Keywords: []string{
			"github", "gitlab", "npm", "docker", "developer", "开发", "開發", "代码", "程式",
		},
	},
	{
		Slug: "other",
		NameJSON: jsonmap.JSON{
			"zh-CN": "其他商品",
			"zh-TW": "其他商品",
			"en-US": "Other",
		},
	},
}

func appendCategorySearchText(builder *strings.Builder, value interface{}) {
	switch typed := value.(type) {
	case nil:
		return
	case string:
		builder.WriteString(strings.ToLower(typed))
		builder.WriteByte(' ')
	case []string:
		for _, item := range typed {
			appendCategorySearchText(builder, item)
		}
	case []interface{}:
		for _, item := range typed {
			appendCategorySearchText(builder, item)
		}
	case jsonmap.JSON:
		for _, item := range typed {
			appendCategorySearchText(builder, item)
		}
	case map[string]interface{}:
		for _, item := range typed {
			appendCategorySearchText(builder, item)
		}
	default:
		builder.WriteString(strings.ToLower(fmt.Sprint(typed)))
		builder.WriteByte(' ')
	}
}

func classifyAutoCategory(values ...interface{}) autoCategoryDefinition {
	// The upstream title is the first value for all import/sync paths. Titles
	// are the strongest signal: descriptions often mention unrelated products
	// in warranty or activation instructions (for example Apple TV mentioning
	// Gemini), which must not change the product's category.
	titleText := ""
	if len(values) > 0 {
		var title strings.Builder
		appendCategorySearchText(&title, values[0])
		titleText = title.String()
	}
	for _, definition := range autoCategoryDefinitions {
		if definition.Slug == "other" {
			continue
		}
		for _, keyword := range definition.TitleKeywords {
			if strings.Contains(titleText, strings.ToLower(keyword)) {
				return definition
			}
		}
	}

	var search strings.Builder
	for _, value := range values {
		appendCategorySearchText(&search, value)
	}
	text := search.String()
	best := autoCategoryDefinitions[len(autoCategoryDefinitions)-1]
	bestScore := 0
	for _, definition := range autoCategoryDefinitions {
		if definition.Slug == "other" {
			continue
		}
		score := 0
		for _, keyword := range definition.Keywords {
			if strings.Contains(text, strings.ToLower(keyword)) {
				// Longer, more specific phrases outweigh incidental mentions.
				score += 10 + len([]rune(keyword))
			}
		}
		if score > bestScore {
			best = definition
			bestScore = score
		}
	}
	return best
}

func isAutoCategorySlug(slug string) bool {
	for _, definition := range autoCategoryDefinitions {
		if definition.Slug == slug {
			return true
		}
	}
	return false
}

// reclassifyAutoCategoryForProduct updates only categories that were created
// by the automatic classifier. A manually selected category remains untouched.
func (s *Service) reclassifyAutoCategoryForProduct(product *productdomain.Product, values ...interface{}) (bool, error) {
	if s == nil || product == nil || s.categories == nil || s.categoryCreator == nil {
		return false, nil
	}
	if product.CategoryID != 0 {
		current, err := s.categories.GetByID(fmt.Sprintf("%d", product.CategoryID))
		if err != nil {
			return false, err
		}
		if current == nil || !isAutoCategorySlug(current.Slug) {
			return false, nil
		}
	}
	targetID, err := s.ensureAutoCategory(values...)
	if err != nil {
		return false, err
	}
	if targetID == 0 || targetID == product.CategoryID {
		return false, nil
	}
	product.CategoryID = targetID
	return true, nil
}

func (s *Service) ensureAutoCategory(values ...interface{}) (uint, error) {
	if s == nil || s.categories == nil || s.categoryCreator == nil {
		return 0, nil
	}
	definition := classifyAutoCategory(values...)
	s.categoryMu.Lock()
	defer s.categoryMu.Unlock()
	category, err := s.findOrCreateLocalCategory(definition.Slug, definition.NameJSON, 0)
	if err != nil {
		return 0, err
	}
	if category == nil {
		return 0, nil
	}
	return category.ID, nil
}

func (s *Service) ensureAutoCategoryForUpstreamProduct(product *upstream.UpstreamProduct) (uint, error) {
	if product == nil {
		return 0, nil
	}
	return s.ensureAutoCategory(product.Title, product.Description, product.Content, product.Tags)
}

func (s *Service) ensureAutoCategoryForLocalProduct(product *productdomain.Product) (uint, error) {
	if product == nil {
		return 0, nil
	}
	return s.ensureAutoCategory(product.TitleJSON, product.DescriptionJSON, product.ContentJSON, product.Tags, product.SeoMetaJSON)
}

// AutoCategorizeUncategorized 为尚未分配分类的映射商品创建/匹配本地分类。
// 管理员已经手动选择分类的商品不会被覆盖。
func (s *Service) AutoCategorizeUncategorized(mappings []mappingdomain.Mapping) int {
	if s == nil || s.products == nil || s.categories == nil || s.categoryCreator == nil {
		return 0
	}
	categorized := 0
	for _, mapping := range mappings {
		if mapping.LocalProductID == 0 {
			continue
		}
		product, err := s.products.GetByID(fmt.Sprintf("%d", mapping.LocalProductID))
		if err != nil || product == nil || product.CategoryID != 0 {
			continue
		}
		categoryID, categoryErr := s.ensureAutoCategoryForLocalProduct(product)
		if categoryErr != nil {
			logger.Warnw("auto_categorize_product_failed", "local_product_id", mapping.LocalProductID, "error", categoryErr)
			continue
		}
		if categoryID == 0 {
			continue
		}
		product.CategoryID = categoryID
		if updateErr := s.products.Update(product); updateErr != nil {
			logger.Warnw("auto_categorize_product_update_failed", "local_product_id", mapping.LocalProductID, "category_id", categoryID, "error", updateErr)
			continue
		}
		categorized++
	}
	return categorized
}

// RepairProdSellerCatalog repairs the derived category of existing ProdSeller
// mappings without making an upstream request. It is intentionally limited to
// auto-managed categories; an operator-selected category is never overwritten.
// The method is idempotent and is used once during startup so products imported
// before the title-first classifier was introduced are repaired as well.
func (s *Service) RepairProdSellerCatalog(connectionID uint) (int, error) {
	if s == nil || s.mappings == nil || s.products == nil {
		return 0, nil
	}
	mappings, err := s.mappings.ListActiveByConnection(connectionID)
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, mapping := range mappings {
		if mapping.LocalProductID == 0 {
			continue
		}
		product, err := s.products.GetByID(fmt.Sprintf("%d", mapping.LocalProductID))
		if err != nil {
			return updated, err
		}
		if product == nil {
			continue
		}
		changed, err := s.reclassifyAutoCategoryForProduct(
			product,
			product.TitleJSON,
			product.DescriptionJSON,
			product.ContentJSON,
			product.Tags,
			product.SeoMetaJSON,
		)
		if err != nil {
			return updated, err
		}
		if !changed {
			continue
		}
		if err := s.products.Update(product); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

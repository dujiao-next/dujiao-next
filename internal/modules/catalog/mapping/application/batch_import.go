package application

import (
	"context"
	"errors"
	"fmt"
	"github.com/dujiao-next/internal/cache"
	"time"

	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"

	categoryapp "github.com/dujiao-next/internal/modules/catalog/category/application"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

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
	UpstreamProductID uint
	Mapping           *mappingdomain.Mapping
	Err               error
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

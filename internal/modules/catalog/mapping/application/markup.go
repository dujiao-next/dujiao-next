package application

import (
	"context"
	"strconv"
	"time"

	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/upstream"
)

// ReapplyMarkup 对指定连接的所有映射商品重新应用加价规则。
// 会访问上游（补齐分类 ID、拉分类树），耗时可能到分钟级，不要放在保存请求里同步调用。
func (s *Service) ReapplyMarkup(connectionID uint) (int, error) {
	conn, err := s.connections.GetByID(connectionID)
	if err != nil {
		return 0, err
	}
	if conn == nil {
		return 0, siteconnectioncontract.ErrNotFound
	}

	// 配了分类加价时，先补齐旧映射缺失的上游分类 ID，再取分类树用于继承。
	// 任一步拿不到就整体放弃改价：拿不到父分类只能按默认加价算，
	// 会把「父分类 50%」的商品写成 0%，比不改更糟。
	var categoryParents map[uint]uint
	if len(conn.CategoryMarkups) > 0 {
		adapter, adapterErr := s.connections.GetAdapter(conn)
		if adapterErr != nil {
			logger.Warnw("reapply_markup_get_adapter_failed", "connection_id", connectionID, "error", adapterErr)
			return 0, mappingcontract.ErrUpstreamCategoryUnavailable
		}
		mappings, listErr := s.mappings.ListActiveByConnection(connectionID)
		if listErr != nil {
			return 0, listErr
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if !s.backfillUpstreamCategoryIDs(ctx, adapter, connectionID, mappings) {
			return 0, mappingcontract.ErrUpstreamCategoryUnavailable
		}
		parents, ok := s.upstreamCategoryParents(ctx, adapter, conn)
		if !ok {
			return 0, mappingcontract.ErrUpstreamCategoryUnavailable
		}
		categoryParents = parents
	}

	return s.reapplyMarkupPrices(conn, categoryParents, false)
}

// ReapplyMarkupLocal 只用本地已知的分类 ID 重算，不访问上游。
// 供连接保存这种必须快速返回的路径使用：拿不到父分类的商品只按分类自身配置算；
// 分类 ID 为 0 的旧映射在配了分类加价时直接跳过（无法判断它该用哪个分类的加价，
// 按默认价写回就是错价），留待「重新应用加价」补齐。
func (s *Service) ReapplyMarkupLocal(connectionID uint) (int, error) {
	conn, err := s.connections.GetByID(connectionID)
	if err != nil {
		return 0, err
	}
	if conn == nil {
		return 0, siteconnectioncontract.ErrNotFound
	}
	return s.reapplyMarkupPrices(conn, nil, true)
}

// reapplyMarkupPrices 按解析好的分类加价重算本地售价。
// skipUnknownCategory=true 时跳过上游分类 ID 为 0 的映射（见 ReapplyMarkupLocal 说明）。
func (s *Service) reapplyMarkupPrices(conn *siteconnectiondomain.Connection, categoryParents map[uint]uint, skipUnknownCategory bool) (int, error) {
	mappings, err := s.mappings.ListActiveByConnection(conn.ID)
	if err != nil {
		return 0, err
	}
	hasCategoryMarkups := len(conn.CategoryMarkups) > 0

	updated := 0
	for _, mapping := range mappings {
		if skipUnknownCategory && hasCategoryMarkups && mapping.UpstreamCategoryID == 0 {
			continue
		}
		// 没有分类树（本地重算路径）且该分类需要向父分类继承时，算出来的加价可能是错的：
		// 跳过它，留给「重新应用加价」拉完分类树再算。
		if categoryParents == nil && hasCategoryMarkups &&
			categoryMarkupsNeedParents(conn.CategoryMarkups, mapping.UpstreamCategoryID) {
			continue
		}
		skuMappings, err := s.skuMappings.ListByProductMapping(mapping.ID)
		if err != nil {
			continue
		}

		markupPercent := ResolveCategoryMarkup(conn.PriceMarkupPercent, conn.CategoryMarkups, mapping.UpstreamCategoryID, categoryParents)
		for _, sm := range skuMappings {
			newLocalPrice := CalculateLocalPrice(sm.UpstreamPrice.Decimal, conn.ExchangeRate, markupPercent, conn.PriceRoundingMode)
			localSKU, err := s.skus.GetByID(sm.LocalSKUID)
			if err != nil || localSKU == nil {
				continue
			}
			localSKU.PriceAmount = money.FromDecimal(newLocalPrice.Round(2))
			localSKU.CostPriceAmount = money.FromDecimal(convertCurrency(sm.UpstreamPrice.Decimal, conn.ExchangeRate).Round(2)) // 成本价 = 上游价格 × 汇率（本地币种）
			_ = s.skus.Update(localSKU)
		}

		// 更新 Product.PriceAmount
		localProduct, err := s.products.GetByID(strconv.FormatUint(uint64(mapping.LocalProductID), 10))
		if err == nil && localProduct != nil {
			s.recalcProductPrice(localProduct)
			updated++
		}
	}

	return updated, nil
}

// backfillUpstreamCategoryIDs 为还没有上游分类 ID 的旧映射补齐分类 ID（就地修改 mappings）。
// 一次遍历上游全部商品；没有需要补的映射时不发请求。上游本身未分类的商品拿到的仍是 0，
// 之后每次重算会再拉一遍，但只发生在配置了分类加价的连接上。
// 返回 false 表示补齐失败（拿不到上游商品列表），调用方必须放弃这一轮改价。
func (s *Service) backfillUpstreamCategoryIDs(ctx context.Context, adapter upstream.Adapter, connectionID uint, mappings []mappingdomain.Mapping) bool {
	needs := false
	for i := range mappings {
		if mappings[i].UpstreamCategoryID == 0 {
			needs = true
			break
		}
	}
	if !needs {
		return true
	}
	products, err := listAllUpstreamProducts(ctx, adapter)
	if err != nil {
		logger.Warnw("backfill_upstream_category_ids_failed", "connection_id", connectionID, "error", err)
		return false
	}
	categoryByProduct := make(map[uint]uint, len(products))
	for _, p := range products {
		categoryByProduct[p.ID] = p.CategoryID
	}
	for i := range mappings {
		if mappings[i].UpstreamCategoryID != 0 {
			continue
		}
		categoryID := categoryByProduct[mappings[i].UpstreamProductID]
		if categoryID == 0 {
			continue
		}
		mappings[i].UpstreamCategoryID = categoryID
		// 只更新这一列：Update 走 db.Save 会整行写回，若这期间定时同步改了
		// 下架状态或 last_synced_at，整行写回会把它们覆盖回旧值。
		if err := s.mappings.UpdateUpstreamCategoryID(mappings[i].ID, categoryID); err != nil {
			logger.Warnw("backfill_upstream_category_id_update_failed",
				"mapping_id", mappings[i].ID, "error", err)
		}
	}
	return true
}

// recalcProductPrice 重新计算商品基准价格和成本价为最低活跃 SKU 价格
func (s *Service) recalcProductPrice(product *productdomain.Product) {
	allSKUs, err := s.skus.ListByProduct(product.ID, true)
	if err != nil || len(allSKUs) == 0 {
		return
	}
	minPrice := allSKUs[0].PriceAmount.Decimal
	minCostPrice := allSKUs[0].CostPriceAmount.Decimal
	for _, sku := range allSKUs[1:] {
		if sku.PriceAmount.Decimal.LessThan(minPrice) {
			minPrice = sku.PriceAmount.Decimal
		}
		if sku.CostPriceAmount.Decimal.LessThan(minCostPrice) {
			minCostPrice = sku.CostPriceAmount.Decimal
		}
	}
	product.PriceAmount = money.FromDecimal(minPrice.Round(2))
	product.CostPriceAmount = money.FromDecimal(minCostPrice.Round(2))
	_ = s.products.Update(product)
}

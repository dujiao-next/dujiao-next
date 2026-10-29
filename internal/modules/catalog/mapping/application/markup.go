package application

import (
	"context"
	"strconv"
	"time"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/upstream"
)

// ReapplyMarkup 对指定连接的所有映射商品重新应用加价规则
func (s *Service) ReapplyMarkup(connectionID uint) (int, error) {
	conn, err := s.connections.GetByID(connectionID)
	if err != nil {
		return 0, err
	}
	if conn == nil {
		return 0, siteconnectioncontract.ErrNotFound
	}

	mappings, err := s.mappings.ListActiveByConnection(connectionID)
	if err != nil {
		return 0, err
	}

	// 配置了分类加价时，先补齐旧映射缺失的上游分类 ID，并取上游分类树用于继承。
	// 两步都是尽力而为：失败只记告警，对应商品回落到连接默认加价。
	var categoryParents map[uint]uint
	if len(conn.CategoryMarkups) > 0 {
		adapter, adapterErr := s.connections.GetAdapter(conn)
		if adapterErr != nil {
			logger.Warnw("reapply_markup_get_adapter_failed", "connection_id", connectionID, "error", adapterErr)
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			s.backfillUpstreamCategoryIDs(ctx, adapter, connectionID, mappings)
			categoryParents = s.upstreamCategoryParents(ctx, adapter, conn)
		}
	}

	updated := 0
	for _, mapping := range mappings {
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
func (s *Service) backfillUpstreamCategoryIDs(ctx context.Context, adapter upstream.Adapter, connectionID uint, mappings []mappingdomain.Mapping) {
	needs := false
	for i := range mappings {
		if mappings[i].UpstreamCategoryID == 0 {
			needs = true
			break
		}
	}
	if !needs {
		return
	}
	products, err := listAllUpstreamProducts(ctx, adapter)
	if err != nil {
		logger.Warnw("backfill_upstream_category_ids_failed", "connection_id", connectionID, "error", err)
		return
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
		if err := s.mappings.Update(&mappings[i]); err != nil {
			logger.Warnw("backfill_upstream_category_id_update_failed",
				"mapping_id", mappings[i].ID, "error", err)
		}
	}
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

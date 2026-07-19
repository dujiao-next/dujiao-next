package service

import (
	"strconv"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
)

// ReapplyMarkup 对指定连接的所有映射商品重新应用加价规则
func (s *ProductMappingService) ReapplyMarkup(connectionID uint) (int, error) {
	conn, err := s.connService.GetByID(connectionID)
	if err != nil {
		return 0, err
	}
	if conn == nil {
		return 0, ErrConnectionNotFound
	}
	if conn.Protocol == constants.ConnectionProtocolGenericWebhook {
		return 0, ErrProtocolCapabilityUnsupported
	}

	mappings, err := s.mappingRepo.ListActiveByConnection(connectionID)
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, mapping := range mappings {
		skuMappings, err := s.skuMappingRepo.ListByProductMapping(mapping.ID)
		if err != nil {
			continue
		}

		for _, sm := range skuMappings {
			newLocalPrice := CalculateLocalPrice(sm.UpstreamPrice.Decimal, conn.ExchangeRate, conn.PriceMarkupPercent, conn.PriceRoundingMode)
			localSKU, err := s.productSKURepo.GetByID(sm.LocalSKUID)
			if err != nil || localSKU == nil {
				continue
			}
			localSKU.PriceAmount = models.NewMoneyFromDecimal(newLocalPrice.Round(2))
			localSKU.CostPriceAmount = models.NewMoneyFromDecimal(convertCurrency(sm.UpstreamPrice.Decimal, conn.ExchangeRate).Round(2)) // 成本价 = 上游价格 × 汇率（本地币种）
			_ = s.productSKURepo.Update(localSKU)
		}

		// 更新 Product.PriceAmount
		localProduct, err := s.productRepo.GetByID(strconv.FormatUint(uint64(mapping.LocalProductID), 10))
		if err == nil && localProduct != nil {
			s.recalcProductPrice(localProduct)
			updated++
		}
	}

	return updated, nil
}

// recalcProductPrice 重新计算商品基准价格和成本价为最低活跃 SKU 价格
func (s *ProductMappingService) recalcProductPrice(product *models.Product) {
	allSKUs, err := s.productSKURepo.ListByProduct(product.ID, true)
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
	product.PriceAmount = models.NewMoneyFromDecimal(minPrice.Round(2))
	product.CostPriceAmount = models.NewMoneyFromDecimal(minCostPrice.Round(2))
	_ = s.productRepo.Update(product)
}

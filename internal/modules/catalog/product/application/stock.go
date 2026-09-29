package productapplication

import (
	"strings"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
)

// ApplyAutoStockCounts 聚合卡密自动发货库存信息并填充到商品中
func (s *Service) ApplyAutoStockCounts(products []productdomain.Product) error {
	var productIDs []uint
	for _, p := range products {
		if p.FulfillmentType == constants.FulfillmentTypeAuto {
			productIDs = append(productIDs, p.ID)
		}
	}
	if len(productIDs) == 0 {
		return nil
	}

	counts, err := s.stock.CountStockByProductIDs(productIDs)
	if err != nil {
		return err
	}

	// map[product_id]map[sku_id]map[status]total
	stockMap := make(map[uint]map[uint]map[string]int64)
	// 有可用循环卡密的 SKU 视为无限库存：map[product_id]map[sku_id]bool
	loopMap := make(map[uint]map[uint]bool)
	for _, count := range counts {
		if count.IsLoop && count.Status == cardsecretdomain.StatusAvailable {
			if loopMap[count.ProductID] == nil {
				loopMap[count.ProductID] = make(map[uint]bool)
			}
			loopMap[count.ProductID][count.SKUID] = true
			continue
		}
		if stockMap[count.ProductID] == nil {
			stockMap[count.ProductID] = make(map[uint]map[string]int64)
		}
		if stockMap[count.ProductID][count.SKUID] == nil {
			stockMap[count.ProductID][count.SKUID] = make(map[string]int64)
		}
		stockMap[count.ProductID][count.SKUID][count.Status] += count.Total
	}

	for i := range products {
		if products[i].FulfillmentType != constants.FulfillmentTypeAuto {
			continue
		}
		pMap := stockMap[products[i].ID]
		pLoop := loopMap[products[i].ID]
		if pMap == nil && pLoop == nil {
			continue
		}

		var pAvailable, pLocked, pUsed int64
		for _, statusMap := range pMap {
			pAvailable += statusMap[cardsecretdomain.StatusAvailable]
			pLocked += statusMap[cardsecretdomain.StatusReserved]
			pUsed += statusMap[cardsecretdomain.StatusUsed]
		}
		products[i].AutoStockAvailable = pAvailable
		products[i].AutoStockTotal = pAvailable + pLocked
		products[i].AutoStockLocked = pLocked
		products[i].AutoStockSold = pUsed
		if len(pLoop) > 0 {
			products[i].AutoStockAvailable = -1
		}

		legacyTargetIdx := resolveLegacyStockTargetSKUIndex(products[i].SKUs)
		for j := range products[i].SKUs {
			skuID := products[i].SKUs[j].ID
			statusMap := pMap[skuID]

			available := statusMap[cardsecretdomain.StatusAvailable]
			locked := statusMap[cardsecretdomain.StatusReserved]
			used := statusMap[cardsecretdomain.StatusUsed]

			// 如果 skuID 为 0 的历史卡密存在，优先归并到 DEFAULT SKU。
			// 若不存在 DEFAULT SKU，则回退到首个启用 SKU，避免重复叠加到所有 SKU。
			if j == legacyTargetIdx && pMap[0] != nil {
				available += pMap[0][cardsecretdomain.StatusAvailable]
				locked += pMap[0][cardsecretdomain.StatusReserved]
				used += pMap[0][cardsecretdomain.StatusUsed]
			}

			products[i].SKUs[j].AutoStockAvailable = available
			products[i].SKUs[j].AutoStockTotal = available + locked
			products[i].SKUs[j].AutoStockLocked = locked
			products[i].SKUs[j].AutoStockSold = used
			if pLoop[skuID] || (j == legacyTargetIdx && pLoop[0]) {
				products[i].SKUs[j].AutoStockAvailable = -1
			}
		}
	}
	return nil
}

func resolveLegacyStockTargetSKUIndex(skus []productdomain.ProductSKU) int {
	if len(skus) == 0 {
		return -1
	}

	defaultCode := strings.ToUpper(strings.TrimSpace(productdomain.DefaultSKUCode))
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

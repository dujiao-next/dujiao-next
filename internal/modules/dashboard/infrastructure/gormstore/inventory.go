package gormstore

import (
	"strings"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	dashboard "github.com/dujiao-next/internal/modules/dashboard/contract"

	"gorm.io/gorm"
)

func resolveDashboardManualAvailableStock(product productdomain.Product) (int64, bool) {
	activeSKUs := activeProductSKUs(product.SKUs)
	if len(activeSKUs) == 0 {
		if product.ManualStockTotal == constants.ManualStockUnlimited {
			return 0, true
		}
		available := int64(product.ManualStockTotal)
		if available < 0 {
			available = 0
		}
		return available, false
	}

	total := int64(0)
	for _, sku := range activeSKUs {
		if sku.ManualStockTotal == constants.ManualStockUnlimited {
			return 0, true
		}
		available := int64(sku.ManualStockTotal)
		if available < 0 {
			available = 0
		}
		total += available
	}
	return total, false
}

// loopSecretProductSet 返回存在可用循环卡密的商品。循环卡密可无限发货，
// 这些商品与手动无限库存一样不参与库存预警统计。
func (r *Store) loopSecretProductSet(productIDs []uint) (map[uint]bool, error) {
	result := make(map[uint]bool)
	if len(productIDs) == 0 {
		return result, nil
	}
	var ids []uint
	if err := r.db.Model(&cardsecretdomain.Secret{}).
		Where("product_id IN ? AND status = ? AND is_loop = ? AND deleted_at IS NULL", productIDs, cardsecretdomain.StatusAvailable, true).
		Distinct().
		Pluck("product_id", &ids).Error; err != nil {
		return nil, err
	}
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

func excludeProductIDs(ids []uint, excluded map[uint]bool) []uint {
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if !excluded[id] {
			result = append(result, id)
		}
	}
	return result
}

// GetStockStats 获取库存总览统计
func (r *Store) GetStockStats(lowStockThreshold int64) (dashboard.StockStatsRow, error) {
	result := dashboard.StockStatsRow{}

	products := make([]productdomain.Product, 0)
	if err := r.db.
		Preload("SKUs", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL AND is_active = ?", true).Order("sort_order DESC, created_at ASC")
		}).
		Where("deleted_at IS NULL AND is_active = ?", true).
		Find(&products).Error; err != nil {
		return result, err
	}

	autoProductIDs := make([]uint, 0)
	allActiveSKUIDs := make([]uint, 0)
	autoProductActiveSKUs := make(map[uint][]uint) // product_id -> active sku_ids
	for _, product := range products {
		fulfillmentType := strings.TrimSpace(product.FulfillmentType)
		if fulfillmentType == constants.FulfillmentTypeAuto {
			autoProductIDs = append(autoProductIDs, product.ID)
			skuIDs := make([]uint, 0, len(product.SKUs))
			for _, sku := range activeProductSKUs(product.SKUs) {
				skuIDs = append(skuIDs, sku.ID)
				allActiveSKUIDs = append(allActiveSKUIDs, sku.ID)
			}
			autoProductActiveSKUs[product.ID] = skuIDs
			continue
		}
		if fulfillmentType != constants.FulfillmentTypeManual {
			continue
		}
		available, unlimited := resolveDashboardManualAvailableStock(product)
		if unlimited {
			continue
		}
		result.ManualAvailableUnits += available
		switch classifyInventoryAlertType(available, lowStockThreshold) {
		case constants.NotificationAlertTypeOutOfStockProducts:
			result.OutOfStockProducts += 1
		case constants.NotificationAlertTypeLowStockProducts:
			result.LowStockProducts += 1
		}
		// 手动交付 SKU 维度统计
		activeSKUs := activeProductSKUs(product.SKUs)
		for _, sku := range activeSKUs {
			if sku.ManualStockTotal == constants.ManualStockUnlimited {
				continue
			}
			skuAvail := int64(sku.ManualStockTotal)
			if skuAvail < 0 {
				skuAvail = 0
			}
			switch classifyInventoryAlertType(skuAvail, lowStockThreshold) {
			case constants.NotificationAlertTypeOutOfStockProducts:
				result.OutOfStockSKUs += 1
			case constants.NotificationAlertTypeLowStockProducts:
				result.LowStockSKUs += 1
			}
		}
	}

	loopProducts, err := r.loopSecretProductSet(autoProductIDs)
	if err != nil {
		return result, err
	}
	autoProductIDs = excludeProductIDs(autoProductIDs, loopProducts)
	for productID := range loopProducts {
		delete(autoProductActiveSKUs, productID)
	}

	if len(autoProductIDs) == 0 {
		return result, nil
	}

	// 按 product_id + sku_id 分组查询，仅统计启用 SKU 和 sku_id=0（遗留）的卡密
	type countRow struct {
		ProductID uint
		SKUID     uint `gorm:"column:sku_id"`
		Total     int64
	}
	var rows []countRow
	query := r.db.Model(&cardsecretdomain.Secret{}).
		Select("product_id, sku_id, COUNT(*) as total").
		Where("product_id IN ? AND status = ? AND deleted_at IS NULL", autoProductIDs, cardsecretdomain.StatusAvailable)
	if len(allActiveSKUIDs) > 0 {
		query = query.Where("sku_id = 0 OR sku_id IN ?", allActiveSKUIDs)
	} else {
		query = query.Where("sku_id = 0")
	}
	if err := query.Group("product_id, sku_id").Scan(&rows).Error; err != nil {
		return result, err
	}

	// 按商品聚合总库存，同时按 SKU 统计
	productAvailableMap := make(map[uint]int64)
	skuAvailableMap := make(map[uint]map[uint]int64) // product_id -> sku_id -> total
	for _, item := range rows {
		productAvailableMap[item.ProductID] += item.Total
		result.AutoAvailableSecrets += item.Total
		if skuAvailableMap[item.ProductID] == nil {
			skuAvailableMap[item.ProductID] = make(map[uint]int64)
		}
		skuAvailableMap[item.ProductID][item.SKUID] = item.Total
	}

	// 商品级别统计
	for _, productID := range autoProductIDs {
		available := productAvailableMap[productID]
		switch classifyInventoryAlertType(available, lowStockThreshold) {
		case constants.NotificationAlertTypeOutOfStockProducts:
			result.OutOfStockProducts += 1
		case constants.NotificationAlertTypeLowStockProducts:
			result.LowStockProducts += 1
		}
	}

	// SKU 级别统计
	for productID, skuIDs := range autoProductActiveSKUs {
		skuMap := skuAvailableMap[productID]
		legacyTargetSKUID := uint(0)
		if len(skuIDs) > 0 {
			legacyTargetSKUID = skuIDs[0] // 简化处理：sku_id=0 库存归入第一个启用 SKU
		}
		for _, skuID := range skuIDs {
			skuAvail := int64(0)
			if skuMap != nil {
				skuAvail = skuMap[skuID]
			}
			if skuID == legacyTargetSKUID && skuMap != nil {
				skuAvail += skuMap[0]
			}
			switch classifyInventoryAlertType(skuAvail, lowStockThreshold) {
			case constants.NotificationAlertTypeOutOfStockProducts:
				result.OutOfStockSKUs += 1
			case constants.NotificationAlertTypeLowStockProducts:
				result.LowStockSKUs += 1
			}
		}
	}

	return result, nil
}

// GetInventoryAlertItems 获取库存异常明细
func (r *Store) GetInventoryAlertItems(lowStockThreshold int64) ([]dashboard.InventoryAlertRow, error) {
	products := make([]productdomain.Product, 0)
	if err := r.db.
		Preload("SKUs", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL AND is_active = ?", true).Order("sort_order DESC, created_at ASC")
		}).
		Where("deleted_at IS NULL AND is_active = ?", true).
		Order("sort_order DESC, created_at DESC").
		Find(&products).Error; err != nil {
		return nil, err
	}

	autoProductIDs := make([]uint, 0)
	upstreamProductIDs := make([]uint, 0)
	for _, product := range products {
		switch strings.TrimSpace(product.FulfillmentType) {
		case constants.FulfillmentTypeAuto:
			autoProductIDs = append(autoProductIDs, product.ID)
		case constants.FulfillmentTypeUpstream:
			upstreamProductIDs = append(upstreamProductIDs, product.ID)
		}
	}

	loopProducts, err := r.loopSecretProductSet(autoProductIDs)
	if err != nil {
		return nil, err
	}
	autoProductIDs = excludeProductIDs(autoProductIDs, loopProducts)

	autoAvailableMap := make(map[uint]map[uint]int64)
	if len(autoProductIDs) > 0 {
		type countRow struct {
			ProductID uint
			SKUID     uint `gorm:"column:sku_id"`
			Total     int64
		}
		rows := make([]countRow, 0)
		if err := r.db.Model(&cardsecretdomain.Secret{}).
			Select("product_id, sku_id, COUNT(*) as total").
			Where("product_id IN ? AND status = ? AND deleted_at IS NULL", autoProductIDs, cardsecretdomain.StatusAvailable).
			Group("product_id, sku_id").
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			if autoAvailableMap[row.ProductID] == nil {
				autoAvailableMap[row.ProductID] = make(map[uint]int64)
			}
			autoAvailableMap[row.ProductID][row.SKUID] = row.Total
		}
	}

	upstreamMappingsByProduct := make(map[uint][]interface{})
	if len(upstreamProductIDs) > 0 {
		type skuMappingRow struct {
			ProductMappingID uint `gorm:"column:product_mapping_id"`
			LocalSKUID       uint `gorm:"column:local_sku_id"`
			UpstreamStock    int  `gorm:"column:upstream_stock"`
			UpstreamIsActive bool `gorm:"column:upstream_is_active"`
		}
		var mappingRows []skuMappingRow
		if err := r.db.Model(&mappingdomain.SKUMapping{}).
			Select("sku_mappings.product_mapping_id, sku_mappings.local_sku_id, sku_mappings.upstream_stock, sku_mappings.upstream_is_active").
			Joins("JOIN product_mappings ON product_mappings.id = sku_mappings.product_mapping_id").
			Where("product_mappings.local_product_id IN ? AND product_mappings.deleted_at IS NULL AND sku_mappings.deleted_at IS NULL", upstreamProductIDs).
			Scan(&mappingRows).Error; err != nil {
			return nil, err
		}

		productMappingToProduct := make(map[uint]uint)
		var productMappings []mappingdomain.Mapping
		if err := r.db.Model(&mappingdomain.Mapping{}).
			Select("id, local_product_id").
			Where("local_product_id IN ? AND deleted_at IS NULL", upstreamProductIDs).
			Find(&productMappings).Error; err != nil {
			return nil, err
		}
		for _, pm := range productMappings {
			productMappingToProduct[pm.ID] = pm.LocalProductID
		}

		for _, row := range mappingRows {
			productID := productMappingToProduct[row.ProductMappingID]
			if productID == 0 {
				continue
			}
			upstreamMappingsByProduct[productID] = append(upstreamMappingsByProduct[productID], map[string]interface{}{
				"local_sku_id":       row.LocalSKUID,
				"upstream_stock":     row.UpstreamStock,
				"upstream_is_active": row.UpstreamIsActive,
			})
		}
	}

	result := make([]dashboard.InventoryAlertRow, 0)
	for _, product := range products {
		switch strings.TrimSpace(product.FulfillmentType) {
		case constants.FulfillmentTypeAuto:
			if loopProducts[product.ID] {
				continue
			}
			result = append(result, collectAutoInventoryAlertRows(product, autoAvailableMap[product.ID], lowStockThreshold)...)
		case constants.FulfillmentTypeManual:
			result = append(result, collectManualInventoryAlertRows(product, lowStockThreshold)...)
		case constants.FulfillmentTypeUpstream:
			result = append(result, collectUpstreamInventoryAlertRows(product, upstreamMappingsByProduct[product.ID], lowStockThreshold)...)
		}
	}
	return result, nil
}

func collectManualInventoryAlertRows(product productdomain.Product, lowStockThreshold int64) []dashboard.InventoryAlertRow {
	result := make([]dashboard.InventoryAlertRow, 0)
	activeSKUs := activeProductSKUs(product.SKUs)
	if len(activeSKUs) == 0 {
		if product.ManualStockTotal == constants.ManualStockUnlimited {
			return result
		}
		available := int64(product.ManualStockTotal)
		if available < 0 {
			available = 0
		}
		if alertType := classifyInventoryAlertType(available, lowStockThreshold); alertType != "" {
			result = append(result, dashboard.InventoryAlertRow{
				ProductID:        product.ID,
				ProductTitleJSON: product.TitleJSON,
				FulfillmentType:  constants.FulfillmentTypeManual,
				AlertType:        alertType,
				AvailableStock:   available,
			})
		}
		return result
	}

	for _, sku := range activeSKUs {
		if sku.ManualStockTotal == constants.ManualStockUnlimited {
			continue
		}
		available := int64(sku.ManualStockTotal)
		if available < 0 {
			available = 0
		}
		if alertType := classifyInventoryAlertType(available, lowStockThreshold); alertType != "" {
			result = append(result, dashboard.InventoryAlertRow{
				ProductID:         product.ID,
				SKUID:             sku.ID,
				ProductTitleJSON:  product.TitleJSON,
				SKUCode:           strings.TrimSpace(sku.SKUCode),
				SKUSpecValuesJSON: sku.SpecValuesJSON,
				FulfillmentType:   constants.FulfillmentTypeManual,
				AlertType:         alertType,
				AvailableStock:    available,
			})
		}
	}
	return result
}

func collectUpstreamInventoryAlertRows(product productdomain.Product, skuMappings []interface{}, lowStockThreshold int64) []dashboard.InventoryAlertRow {
	result := make([]dashboard.InventoryAlertRow, 0)
	activeSKUs := activeProductSKUs(product.SKUs)

	// 构建 SKU 映射查找表
	type skuMappingData struct {
		LocalSKUID       uint
		UpstreamStock    int
		UpstreamIsActive bool
	}
	mappingByLocalSKU := make(map[uint]skuMappingData)
	for _, item := range skuMappings {
		if m, ok := item.(map[string]interface{}); ok {
			localSKUID, _ := m["local_sku_id"].(uint)
			upstreamStock, _ := m["upstream_stock"].(int)
			upstreamIsActive, _ := m["upstream_is_active"].(bool)
			if localSKUID > 0 {
				mappingByLocalSKU[localSKUID] = skuMappingData{
					LocalSKUID:       localSKUID,
					UpstreamStock:    upstreamStock,
					UpstreamIsActive: upstreamIsActive,
				}
			}
		}
	}

	// 没有启用 SKU 的情况
	if len(activeSKUs) == 0 {
		totalStock := int64(0)
		for _, mapping := range mappingByLocalSKU {
			if !mapping.UpstreamIsActive {
				continue
			}
			stock := int64(mapping.UpstreamStock)
			if stock < 0 {
				stock = 0
			}
			totalStock += stock
		}
		if alertType := classifyInventoryAlertType(totalStock, lowStockThreshold); alertType != "" {
			result = append(result, dashboard.InventoryAlertRow{
				ProductID:        product.ID,
				ProductTitleJSON: product.TitleJSON,
				FulfillmentType:  constants.FulfillmentTypeUpstream,
				AlertType:        alertType,
				AvailableStock:   totalStock,
			})
		}
		return result
	}

	// 有启用 SKU 的情况
	for _, sku := range activeSKUs {
		mapping, hasMapp := mappingByLocalSKU[sku.ID]
		if !hasMapp || !mapping.UpstreamIsActive {
			continue
		}
		available := int64(mapping.UpstreamStock)
		if available < 0 {
			available = 0
		}
		if alertType := classifyInventoryAlertType(available, lowStockThreshold); alertType != "" {
			result = append(result, dashboard.InventoryAlertRow{
				ProductID:         product.ID,
				SKUID:             sku.ID,
				ProductTitleJSON:  product.TitleJSON,
				SKUCode:           strings.TrimSpace(sku.SKUCode),
				SKUSpecValuesJSON: sku.SpecValuesJSON,
				FulfillmentType:   constants.FulfillmentTypeUpstream,
				AlertType:         alertType,
				AvailableStock:    available,
			})
		}
	}
	return result
}

func collectAutoInventoryAlertRows(product productdomain.Product, availableMap map[uint]int64, lowStockThreshold int64) []dashboard.InventoryAlertRow {
	result := make([]dashboard.InventoryAlertRow, 0)
	activeSKUs := activeProductSKUs(product.SKUs)
	totalAvailable := int64(0)
	activeSKUSet := make(map[uint]struct{}, len(activeSKUs))
	for _, sku := range activeSKUs {
		activeSKUSet[sku.ID] = struct{}{}
	}
	legacyInactiveAvailable := int64(0)
	for skuID, total := range availableMap {
		totalAvailable += total
		if skuID == 0 {
			continue
		}
		if _, ok := activeSKUSet[skuID]; ok {
			continue
		}
		legacyInactiveAvailable += total
	}
	if len(activeSKUs) == 0 {
		if alertType := classifyInventoryAlertType(totalAvailable, lowStockThreshold); alertType != "" {
			result = append(result, dashboard.InventoryAlertRow{
				ProductID:        product.ID,
				ProductTitleJSON: product.TitleJSON,
				FulfillmentType:  constants.FulfillmentTypeAuto,
				AlertType:        alertType,
				AvailableStock:   totalAvailable,
			})
		}
		return result
	}

	legacyTargetIdx := resolveDashboardLegacyStockTargetSKUIndex(activeSKUs)
	hasPositiveActive := false
	for idx, sku := range activeSKUs {
		available := availableMap[sku.ID]
		if idx == legacyTargetIdx {
			available += availableMap[0]
		}
		if len(activeSKUs) == 1 {
			available += legacyInactiveAvailable
		}
		if available > 0 {
			hasPositiveActive = true
		}
		if alertType := classifyInventoryAlertType(available, lowStockThreshold); alertType != "" {
			result = append(result, dashboard.InventoryAlertRow{
				ProductID:         product.ID,
				SKUID:             sku.ID,
				ProductTitleJSON:  product.TitleJSON,
				SKUCode:           strings.TrimSpace(sku.SKUCode),
				SKUSpecValuesJSON: sku.SpecValuesJSON,
				FulfillmentType:   constants.FulfillmentTypeAuto,
				AlertType:         alertType,
				AvailableStock:    available,
			})
		}
	}
	if hasPositiveActive || legacyInactiveAvailable <= 0 {
		return result
	}
	if fallbackType := classifyInventoryAlertType(totalAvailable, lowStockThreshold); fallbackType != "" {
		return []dashboard.InventoryAlertRow{
			{
				ProductID:        product.ID,
				ProductTitleJSON: product.TitleJSON,
				FulfillmentType:  constants.FulfillmentTypeAuto,
				AlertType:        fallbackType,
				AvailableStock:   totalAvailable,
			},
		}
	}
	return result
}

func activeProductSKUs(items []productdomain.ProductSKU) []productdomain.ProductSKU {
	result := make([]productdomain.ProductSKU, 0, len(items))
	for _, item := range items {
		if !item.IsActive {
			continue
		}
		result = append(result, item)
	}
	return result
}

func classifyInventoryAlertType(available int64, lowStockThreshold int64) string {
	switch {
	case available <= 0:
		return constants.NotificationAlertTypeOutOfStockProducts
	case available <= lowStockThreshold:
		return constants.NotificationAlertTypeLowStockProducts
	default:
		return ""
	}
}

func resolveDashboardLegacyStockTargetSKUIndex(skus []productdomain.ProductSKU) int {
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
	return firstActiveIdx
}

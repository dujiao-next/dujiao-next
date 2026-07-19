package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ensureManualStockRemainingMigration 将历史“总量库存”迁移为“剩余库存”语义，仅执行一次。
func ensureManualStockRemainingMigration() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}

	var marker Setting
	if err := DB.First(&marker, "key = ?", manualStockRemainingMigrationSettingKey).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else if migrationDone(marker.ValueJSON) {
		return nil
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Product{}).
			Where("manual_stock_total >= ?", manualStockUnlimitedValue+1).
			Update("manual_stock_total",
				gorm.Expr("CASE WHEN (manual_stock_total - manual_stock_locked - manual_stock_sold) < 0 THEN 0 ELSE (manual_stock_total - manual_stock_locked - manual_stock_sold) END")).
			Error; err != nil {
			return err
		}

		if err := tx.Model(&ProductSKU{}).
			Where("manual_stock_total >= ?", manualStockUnlimitedValue+1).
			Update("manual_stock_total",
				gorm.Expr("CASE WHEN (manual_stock_total - manual_stock_locked - manual_stock_sold) < 0 THEN 0 ELSE (manual_stock_total - manual_stock_locked - manual_stock_sold) END")).
			Error; err != nil {
			return err
		}

		marker := Setting{
			Key: manualStockRemainingMigrationSettingKey,
			ValueJSON: JSON{
				"done":        true,
				"migrated_at": time.Now().UTC().Format(time.RFC3339),
			},
		}
		return tx.Save(&marker).Error
	})
}

func migrationDone(value JSON) bool {
	if len(value) == 0 {
		return false
	}
	done, ok := value["done"]
	if !ok {
		return false
	}
	flag, ok := done.(bool)
	return ok && flag
}

// ensureOrderItemOriginalPriceMigration 为历史订单项回填原价快照。
// 历史数据没有真实原价，只能以当时已记录的 unit_price/total_price 作为兼容回填。
func ensureOrderItemOriginalPriceMigration() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}

	var marker Setting
	if err := DB.First(&marker, "key = ?", orderItemOriginalPriceMigrationKey).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else if migrationDone(marker.ValueJSON) {
		return nil
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&OrderItem{}).
			Where("original_unit_price = 0").
			Update("original_unit_price", gorm.Expr("unit_price")).
			Error; err != nil {
			return err
		}
		if err := tx.Model(&OrderItem{}).
			Where("original_total_price = 0").
			Update("original_total_price", gorm.Expr("total_price")).
			Error; err != nil {
			return err
		}

		marker := Setting{
			Key: orderItemOriginalPriceMigrationKey,
			ValueJSON: JSON{
				"done":        true,
				"migrated_at": time.Now().UTC().Format(time.RFC3339),
			},
		}
		return tx.Save(&marker).Error
	})
}

// migrateCartSKUUniqueIndex 迁移购物车唯一索引为 user_id + product_id + sku_id 维度。
func migrateCartSKUUniqueIndex() error {
	migrator := DB.Migrator()

	// 历史唯一索引会阻止同一商品不同 SKU 共存，迁移时必须移除。
	if migrator.HasIndex(&CartItem{}, "idx_cart_user_product") {
		if err := migrator.DropIndex(&CartItem{}, "idx_cart_user_product"); err != nil {
			return err
		}
	}

	if !migrator.HasIndex(&CartItem{}, "idx_cart_user_product_sku") {
		if err := migrator.CreateIndex(&CartItem{}, "idx_cart_user_product_sku"); err != nil {
			return err
		}
	}
	return nil
}

// migrateProductMappingLocalProductIndex replaces the legacy unique index so
// a soft-deleted product mapping can be created again for the same product.
func migrateProductMappingLocalProductIndex() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}

	var marker Setting
	if err := DB.First(&marker, "key = ?", productMappingLocalProductIndexMigrationKey).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else if migrationDone(marker.ValueJSON) {
		return nil
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		const indexName = "idx_product_mappings_local_product_id"
		migrator := tx.Migrator()
		if migrator.HasIndex(&ProductMapping{}, indexName) {
			if err := migrator.DropIndex(&ProductMapping{}, indexName); err != nil {
				return err
			}
		}
		if err := migrator.CreateIndex(&ProductMapping{}, indexName); err != nil {
			return err
		}

		return tx.Save(&Setting{
			Key: productMappingLocalProductIndexMigrationKey,
			ValueJSON: JSON{
				"done":        true,
				"migrated_at": time.Now().UTC().Format(time.RFC3339),
			},
		}).Error
	})
}

// ensureProductSKUMigration 执行 SKU 迁移：补默认 SKU、回填 sku_id、完整性校验。
// 迁移完成后写入幂等标记，后续启动跳过。
func ensureProductSKUMigration() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}

	// 检查迁移标记，已完成则跳过
	var marker Setting
	if err := DB.First(&marker, "key = ?", skuMigrationSettingKey).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else if migrationDone(marker.ValueJSON) {
		return nil
	}

	if err := ensureDefaultProductSKUs(); err != nil {
		return err
	}

	skuMap, err := buildProductSKUMap()
	if err != nil {
		return err
	}

	if err := backfillLegacySKUID(skuMap); err != nil {
		return err
	}

	if err := validateSKUMigrationIntegrity(); err != nil {
		return err
	}

	// 迁移完成，写入标记
	doneMarker := Setting{
		Key: skuMigrationSettingKey,
		ValueJSON: JSON{
			"done":        true,
			"migrated_at": time.Now().UTC().Format(time.RFC3339),
		},
	}
	return DB.Save(&doneMarker).Error
}

// ensureDefaultProductSKUs 为每个历史商品补一条 DEFAULT SKU。
func ensureDefaultProductSKUs() error {
	var products []Product
	if err := DB.Unscoped().
		Select("id, price_amount, manual_stock_total, manual_stock_locked, manual_stock_sold, is_active").
		Find(&products).Error; err != nil {
		return err
	}
	if len(products) == 0 {
		return nil
	}

	type skuProductRow struct {
		ProductID uint
	}
	var existing []skuProductRow
	if err := DB.Unscoped().Model(&ProductSKU{}).
		Select("DISTINCT product_id").
		Scan(&existing).Error; err != nil {
		return err
	}
	existingMap := make(map[uint]struct{}, len(existing))
	for _, row := range existing {
		existingMap[row.ProductID] = struct{}{}
	}

	createRows := make([]ProductSKU, 0)
	for _, product := range products {
		if _, ok := existingMap[product.ID]; ok {
			continue
		}
		createRows = append(createRows, ProductSKU{
			ProductID:         product.ID,
			SKUCode:           DefaultSKUCode,
			SpecValuesJSON:    JSON{},
			PriceAmount:       product.PriceAmount,
			ManualStockTotal:  product.ManualStockTotal,
			ManualStockLocked: product.ManualStockLocked,
			ManualStockSold:   product.ManualStockSold,
			IsActive:          product.IsActive,
		})
	}

	if len(createRows) == 0 {
		return nil
	}

	return DB.Create(&createRows).Error
}

// buildProductSKUMap 构建 product_id -> sku_id 映射，优先选择 DEFAULT SKU。
func buildProductSKUMap() (map[uint]uint, error) {
	type skuRow struct {
		ID        uint
		ProductID uint
		SKUCode   string
	}
	var rows []skuRow
	if err := DB.Unscoped().Model(&ProductSKU{}).
		Select("id, product_id, sku_code").
		Order("id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[uint]uint, len(rows))
	for _, row := range rows {
		if row.ProductID == 0 || row.ID == 0 {
			continue
		}
		current, exists := result[row.ProductID]
		if !exists {
			result[row.ProductID] = row.ID
			continue
		}
		if strings.EqualFold(strings.TrimSpace(row.SKUCode), DefaultSKUCode) {
			result[row.ProductID] = row.ID
			continue
		}
		if current == 0 {
			result[row.ProductID] = row.ID
		}
	}
	return result, nil
}

// backfillLegacySKUID 回填历史 order/cart/card_secret 数据的 sku_id。
func backfillLegacySKUID(productToSKU map[uint]uint) error {
	if len(productToSKU) == 0 {
		return nil
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		for productID, skuID := range productToSKU {
			if productID == 0 || skuID == 0 {
				continue
			}

			if err := tx.Unscoped().Model(&OrderItem{}).
				Where("product_id = ? AND sku_id = 0", productID).
				Update("sku_id", skuID).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Model(&CartItem{}).
				Where("product_id = ? AND sku_id = 0", productID).
				Update("sku_id", skuID).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Model(&CardSecret{}).
				Where("product_id = ? AND sku_id = 0", productID).
				Update("sku_id", skuID).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Model(&CardSecretBatch{}).
				Where("product_id = ? AND sku_id = 0", productID).
				Update("sku_id", skuID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// validateSKUMigrationIntegrity 校验迁移完整性，避免半迁移状态继续运行。
func validateSKUMigrationIntegrity() error {
	type pendingCheck struct {
		name  string
		query func() (int64, error)
	}

	checks := []pendingCheck{
		{
			name: "order_items",
			query: func() (int64, error) {
				var count int64
				err := DB.Model(&OrderItem{}).Where("sku_id = 0").Count(&count).Error
				return count, err
			},
		},
		{
			name: "cart_items",
			query: func() (int64, error) {
				var count int64
				err := DB.Model(&CartItem{}).Where("sku_id = 0").Count(&count).Error
				return count, err
			},
		},
		{
			name: "card_secrets",
			query: func() (int64, error) {
				var count int64
				err := DB.Model(&CardSecret{}).Where("sku_id = 0").Count(&count).Error
				return count, err
			},
		},
		{
			name: "card_secret_batches",
			query: func() (int64, error) {
				var count int64
				err := DB.Model(&CardSecretBatch{}).Where("sku_id = 0").Count(&count).Error
				return count, err
			},
		},
	}

	for _, check := range checks {
		count, err := check.query()
		if err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("sku migration incomplete: %s still has %d records with sku_id=0", check.name, count)
		}
	}

	var missingProducts int64
	if err := DB.Raw(`
SELECT COUNT(1) FROM (
	SELECT p.id
	FROM products p
	LEFT JOIN product_skus s ON s.product_id = p.id AND s.deleted_at IS NULL
	WHERE p.deleted_at IS NULL
	GROUP BY p.id
	HAVING COUNT(s.id) = 0
) t
`).Scan(&missingProducts).Error; err != nil {
		return err
	}
	if missingProducts > 0 {
		return fmt.Errorf("sku migration incomplete: %d products still have no sku", missingProducts)
	}

	return nil
}

// ensurePaymentProviderBepusdtRenameMigration 把 payment_channels 表里历史
// provider_type='epusdt'（实际是 BEpusdt 适配器）改名为 'bepusdt'，仅执行一次。
// 通过 settings 表写 marker 保证幂等：一旦标记 done，后续启动跳过；
// 即使用户后续新建真 epusdt 渠道也不会被误改。
func ensurePaymentProviderBepusdtRenameMigration() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}

	var marker Setting
	if err := DB.First(&marker, "key = ?", paymentProviderBepusdtRenameMigrationSettingKey).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else if migrationDone(marker.ValueJSON) {
		return nil
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"UPDATE payment_channels SET provider_type = ? WHERE provider_type = ?",
			"bepusdt", "epusdt",
		).Error; err != nil {
			return err
		}

		marker := Setting{
			Key: paymentProviderBepusdtRenameMigrationSettingKey,
			ValueJSON: JSON{
				"done":        true,
				"migrated_at": time.Now().UTC().Format(time.RFC3339),
			},
		}
		return tx.Save(&marker).Error
	})
}

// ensureCategoryParentMigration 兼容历史单层分类数据，统一将空 parent_id 视为 0。
func ensureCategoryParentMigration() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}

	var marker Setting
	if err := DB.First(&marker, "key = ?", categoryParentMigrationSettingKey).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else if migrationDone(marker.ValueJSON) {
		return nil
	}

	if !DB.Migrator().HasColumn(&Category{}, "parent_id") {
		return nil
	}
	if err := DB.Model(&Category{}).Where("parent_id IS NULL").Update("parent_id", 0).Error; err != nil {
		return err
	}

	doneMarker := Setting{
		Key: categoryParentMigrationSettingKey,
		ValueJSON: JSON{
			"done":        true,
			"migrated_at": time.Now().UTC().Format(time.RFC3339),
		},
	}
	return DB.Save(&doneMarker).Error
}

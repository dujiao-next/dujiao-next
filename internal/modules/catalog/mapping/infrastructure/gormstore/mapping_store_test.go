package gormstore

import (
	"fmt"
	"testing"
	"time"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/constants"
	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupMappingStoreTest(t *testing.T) (*MappingStore, *SKUMappingStore, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:mapping_store_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&siteconnectiondomain.Connection{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
	); err != nil {
		t.Fatalf("migrate mapping models failed: %v", err)
	}
	defaultCategory := categorydomain.Category{
		ID:       1,
		Slug:     "default-test-category",
		NameJSON: jsonmap.JSON{"zh-CN": "default"},
		IsActive: true,
	}
	if err := db.Create(&defaultCategory).Error; err != nil {
		t.Fatalf("seed default category failed: %v", err)
	}
	return NewMappingStore(db), NewSKUMappingStore(db), db
}

func createMappedProduct(t *testing.T, db *gorm.DB, slug, title string) *productdomain.Product {
	t.Helper()
	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            slug,
		TitleJSON:       jsonmap.JSON{"zh-CN": title},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeUpstream,
		IsMapped:        true,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	return product
}

func createMapping(t *testing.T, store *MappingStore, connectionID, localProductID, upstreamProductID uint, upstreamStatus string, isActive bool) *mappingdomain.Mapping {
	t.Helper()
	mapping := &mappingdomain.Mapping{
		ConnectionID:      connectionID,
		LocalProductID:    localProductID,
		UpstreamProductID: upstreamProductID,
		UpstreamStatus:    upstreamStatus,
		IsActive:          true,
	}
	if err := store.Create(mapping); err != nil {
		t.Fatalf("create mapping failed: %v", err)
	}
	// IsActive 带 default:true 标签，Create 会忽略 false 零值，停用需走 Update
	if !isActive {
		mapping.IsActive = false
		if err := store.Update(mapping); err != nil {
			t.Fatalf("deactivate mapping failed: %v", err)
		}
	}
	return mapping
}

func TestMappingStoreListFiltersAndPaginates(t *testing.T) {
	store, _, db := setupMappingStoreTest(t)

	rechargeCard := createMappedProduct(t, db, "recharge-card", "上游充值卡")
	plainProduct := createMappedProduct(t, db, "plain-product", "普通商品")
	otherSite := createMappedProduct(t, db, "other-site", "另一站点商品")

	createMapping(t, store, 1, rechargeCard.ID, 1001, mappingdomain.UpstreamStatusActive, true)
	inactive := createMapping(t, store, 1, plainProduct.ID, 1002, mappingdomain.UpstreamStatusInactive, false)
	createMapping(t, store, 2, otherSite.ID, 2001, mappingdomain.UpstreamStatusActive, true)

	if _, total, err := store.List(mappingcontract.ListFilter{}); err != nil || total != 3 {
		t.Fatalf("list all: total=%d err=%v, want 3", total, err)
	}
	if _, total, err := store.List(mappingcontract.ListFilter{ConnectionID: 1}); err != nil || total != 2 {
		t.Fatalf("list by connection: total=%d err=%v, want 2", total, err)
	}
	rows, total, err := store.List(mappingcontract.ListFilter{UpstreamStatus: mappingdomain.UpstreamStatusInactive})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].ID != inactive.ID {
		t.Fatalf("list by upstream status: rows=%d total=%d err=%v, want the inactive mapping", len(rows), total, err)
	}
	if _, total, err := store.List(mappingcontract.ListFilter{ProductStatus: "active"}); err != nil || total != 2 {
		t.Fatalf("list by product status active: total=%d err=%v, want 2", total, err)
	}
	if _, total, err := store.List(mappingcontract.ListFilter{ProductStatus: "inactive"}); err != nil || total != 1 {
		t.Fatalf("list by product status inactive: total=%d err=%v, want 1", total, err)
	}
	rows, total, err = store.List(mappingcontract.ListFilter{Search: "充值"})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].LocalProductID != rechargeCard.ID {
		t.Fatalf("list by search: rows=%d total=%d err=%v, want the recharge card mapping", len(rows), total, err)
	}

	rows, total, err = store.List(mappingcontract.ListFilter{Page: 1, PageSize: 2})
	if err != nil || total != 3 || len(rows) != 2 {
		t.Fatalf("list page 1: rows=%d total=%d err=%v, want 2 rows of 3", len(rows), total, err)
	}
	rows, total, err = store.List(mappingcontract.ListFilter{Page: 2, PageSize: 2})
	if err != nil || total != 3 || len(rows) != 1 {
		t.Fatalf("list page 2: rows=%d total=%d err=%v, want 1 row of 3", len(rows), total, err)
	}
}

func TestMappingStoreDeleteByLocalProductRemovesSKUMappings(t *testing.T) {
	store, skuStore, db := setupMappingStoreTest(t)

	removed := createMappedProduct(t, db, "removed-product", "待删除商品")
	kept := createMappedProduct(t, db, "kept-product", "保留商品")
	removedMapping := createMapping(t, store, 1, removed.ID, 1001, mappingdomain.UpstreamStatusActive, true)
	keptMapping := createMapping(t, store, 1, kept.ID, 1002, mappingdomain.UpstreamStatusActive, true)

	for i, mappingID := range []uint{removedMapping.ID, removedMapping.ID, keptMapping.ID} {
		if err := skuStore.Create(&mappingdomain.SKUMapping{
			ProductMappingID: mappingID,
			LocalSKUID:       uint(100 + i),
			UpstreamSKUID:    uint(200 + i),
		}); err != nil {
			t.Fatalf("create sku mapping failed: %v", err)
		}
	}

	if err := store.DeleteByLocalProduct(removed.ID); err != nil {
		t.Fatalf("delete by local product failed: %v", err)
	}

	if mapping, err := store.GetByLocalProductID(removed.ID); err != nil || mapping != nil {
		t.Fatalf("removed mapping should be gone: mapping=%v err=%v", mapping, err)
	}
	if rows, err := skuStore.ListByProductMapping(removedMapping.ID); err != nil || len(rows) != 0 {
		t.Fatalf("removed sku mappings should be gone: rows=%d err=%v", len(rows), err)
	}
	var deletedMapping mappingdomain.Mapping
	if err := db.Where("id = ? AND deleted_at IS NOT NULL", removedMapping.ID).First(&deletedMapping).Error; err != nil {
		t.Fatalf("mapping must remain persisted with deleted_at: %v", err)
	}
	var deletedSKUCount int64
	if err := db.Model(&mappingdomain.SKUMapping{}).
		Where("product_mapping_id = ? AND deleted_at IS NOT NULL", removedMapping.ID).
		Count(&deletedSKUCount).Error; err != nil {
		t.Fatalf("count soft-deleted sku mappings: %v", err)
	}
	if deletedSKUCount != 2 {
		t.Fatalf("soft-deleted sku mappings = %d, want 2", deletedSKUCount)
	}
	if mapping, err := store.GetByLocalProductID(kept.ID); err != nil || mapping == nil {
		t.Fatalf("kept mapping should survive: mapping=%v err=%v", mapping, err)
	}
	if rows, err := skuStore.ListByProductMapping(keptMapping.ID); err != nil || len(rows) != 1 {
		t.Fatalf("kept sku mapping should survive: rows=%d err=%v", len(rows), err)
	}
}

func TestMappingStoreDeleteHidesMappingFromEveryReadPath(t *testing.T) {
	store, _, db := setupMappingStoreTest(t)
	product := createMappedProduct(t, db, "single-delete", "单条删除")
	mapping := createMapping(t, store, 9, product.ID, 9001, mappingdomain.UpstreamStatusActive, true)

	if err := store.Delete(mapping.ID); err != nil {
		t.Fatalf("delete mapping: %v", err)
	}

	if got, err := store.GetByID(mapping.ID); err != nil || got != nil {
		t.Fatalf("GetByID after delete = %#v, %v; want nil, nil", got, err)
	}
	if got, err := store.GetByConnectionAndUpstreamID(mapping.ConnectionID, mapping.UpstreamProductID); err != nil || got != nil {
		t.Fatalf("GetByConnectionAndUpstreamID after delete = %#v, %v; want nil, nil", got, err)
	}
	if rows, total, err := store.List(mappingcontract.ListFilter{}); err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("List after delete = rows %d total %d err %v; want empty", len(rows), total, err)
	}
	if rows, err := store.ListActiveByConnection(mapping.ConnectionID); err != nil || len(rows) != 0 {
		t.Fatalf("ListActiveByConnection after delete = rows %d err %v; want empty", len(rows), err)
	}
	if ids, err := store.ListUpstreamIDsByConnection(mapping.ConnectionID); err != nil || len(ids) != 0 {
		t.Fatalf("ListUpstreamIDsByConnection after delete = ids %v err %v; want empty", ids, err)
	}
}

func TestMappingStoreExternalIDIsFullAndUniquePerConnection(t *testing.T) {
	store, _, db := setupMappingStoreTest(t)
	firstProduct := createMappedProduct(t, db, "external-one", "外部一")
	secondProduct := createMappedProduct(t, db, "external-two", "外部二")
	longID := "6a2ff705dbe65bbecc0d3d9d"
	first := &mappingdomain.Mapping{
		ConnectionID:              1,
		LocalProductID:            firstProduct.ID,
		UpstreamProductExternalID: longID,
		UpstreamStatus:            mappingdomain.UpstreamStatusActive,
		IsActive:                  true,
	}
	if err := store.Create(first); err != nil {
		t.Fatalf("create external mapping failed: %v", err)
	}
	if got, err := store.GetByConnectionAndExternalID(1, longID); err != nil || got == nil || got.UpstreamProductExternalID != longID {
		t.Fatalf("external lookup = %#v err=%v", got, err)
	}
	if err := store.Create(&mappingdomain.Mapping{
		ConnectionID:              1,
		LocalProductID:            secondProduct.ID,
		UpstreamProductExternalID: longID,
		UpstreamStatus:            mappingdomain.UpstreamStatusActive,
		IsActive:                  true,
	}); err == nil {
		t.Fatal("duplicate external ID on one connection must be rejected")
	}
	otherConnection := createMappedProduct(t, db, "external-three", "外部三")
	if err := store.Create(&mappingdomain.Mapping{
		ConnectionID:              2,
		LocalProductID:            otherConnection.ID,
		UpstreamProductExternalID: longID,
		UpstreamStatus:            mappingdomain.UpstreamStatusActive,
		IsActive:                  true,
	}); err != nil {
		t.Fatalf("same external ID on another connection should be allowed: %v", err)
	}
	ids, err := store.ListUpstreamExternalIDsByConnection(1)
	if err != nil || len(ids) != 1 || ids[0] != longID {
		t.Fatalf("external IDs = %#v err=%v", ids, err)
	}
}

package gormstore

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/cart/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCartRepositoryTest(t *testing.T) (*Store, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:cart_repository_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&productdomain.Product{}, &productdomain.ProductSKU{}, &domain.Item{}); err != nil {
		t.Fatalf("migrate cart item failed: %v", err)
	}
	return New(db), db
}

func TestStoreListByUserLoadsProductAndSKUAndHidesDeletedItems(t *testing.T) {
	repo, db := setupCartRepositoryTest(t)
	now := time.Now()
	product := productdomain.Product{
		ID:              888,
		CategoryID:      1,
		Slug:            "cart-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "购物车商品"},
		FulfillmentType: "manual",
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := productdomain.ProductSKU{
		ID:        101,
		ProductID: product.ID,
		SKUCode:   "CART-SKU",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	item := &domain.Item{
		UserID:          10001,
		ProductID:       product.ID,
		SKUID:           sku.ID,
		Quantity:        2,
		FulfillmentType: "manual",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := repo.Upsert(item); err != nil {
		t.Fatalf("upsert item failed: %v", err)
	}

	items, err := repo.ListByUser(item.UserID)
	if err != nil {
		t.Fatalf("list cart items failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("cart items want 1 got %d", len(items))
	}
	if items[0].Product == nil || items[0].Product.ID != product.ID {
		t.Fatalf("product association was not loaded: %#v", items[0].Product)
	}
	if items[0].SKU == nil || items[0].SKU.ID != sku.ID {
		t.Fatalf("sku association was not loaded: %#v", items[0].SKU)
	}

	if err := repo.DeleteByUserProductSKU(item.UserID, item.ProductID, item.SKUID); err != nil {
		t.Fatalf("delete item failed: %v", err)
	}
	items, err = repo.ListByUser(item.UserID)
	if err != nil {
		t.Fatalf("list after delete failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("deleted cart item must be hidden, got %d rows", len(items))
	}
	var deleted domain.Item
	if err := db.Where("id = ?", item.ID).First(&deleted).Error; err != nil {
		t.Fatalf("load deleted row failed: %v", err)
	}
	if deleted.DeletedAt == nil {
		t.Fatal("deleted_at was not persisted")
	}
}

func TestCartRepositoryUpsertUsesProductAndSKUDimension(t *testing.T) {
	repo, db := setupCartRepositoryTest(t)
	now := time.Now()

	first := &domain.Item{
		UserID:          10001,
		ProductID:       888,
		SKUID:           101,
		Quantity:        1,
		FulfillmentType: "manual",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	second := &domain.Item{
		UserID:          10001,
		ProductID:       888,
		SKUID:           102,
		Quantity:        2,
		FulfillmentType: "manual",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := repo.Upsert(first); err != nil {
		t.Fatalf("upsert first sku failed: %v", err)
	}
	if err := repo.Upsert(second); err != nil {
		t.Fatalf("upsert second sku failed: %v", err)
	}

	var count int64
	if err := db.Model(&domain.Item{}).
		Where("user_id = ? AND product_id = ? AND deleted_at IS NULL", first.UserID, first.ProductID).
		Count(&count).Error; err != nil {
		t.Fatalf("count cart items failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("cart rows want 2 got %d", count)
	}

	first.Quantity = 5
	first.UpdatedAt = now.Add(time.Minute)
	if err := repo.Upsert(first); err != nil {
		t.Fatalf("update first sku failed: %v", err)
	}

	var gotFirst domain.Item
	if err := db.Where("user_id = ? AND product_id = ? AND sku_id = ?", first.UserID, first.ProductID, first.SKUID).First(&gotFirst).Error; err != nil {
		t.Fatalf("query first sku row failed: %v", err)
	}
	if gotFirst.Quantity != 5 {
		t.Fatalf("first sku quantity want 5 got %d", gotFirst.Quantity)
	}

	var gotSecond domain.Item
	if err := db.Where("user_id = ? AND product_id = ? AND sku_id = ?", second.UserID, second.ProductID, second.SKUID).First(&gotSecond).Error; err != nil {
		t.Fatalf("query second sku row failed: %v", err)
	}
	if gotSecond.Quantity != 2 {
		t.Fatalf("second sku quantity should keep 2 got %d", gotSecond.Quantity)
	}
}

func TestCartRepositoryUpsertRevivesRemovedItem(t *testing.T) {
	repo, _ := setupCartRepositoryTest(t)
	now := time.Now()
	add := func(quantity int) {
		t.Helper()
		item := &domain.Item{UserID: 7, ProductID: 1, SKUID: 2, Quantity: quantity, FulfillmentType: "manual", CreatedAt: now, UpdatedAt: now}
		if err := repo.Upsert(item); err != nil {
			t.Fatalf("upsert failed: %v", err)
		}
	}

	add(3)
	if err := repo.DeleteByUserProductSKU(7, 1, 2); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	add(1)

	items, err := repo.ListByUser(7)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(items) != 1 || items[0].Quantity != 1 {
		t.Fatalf("expected one revived item with quantity 1, got %+v", items)
	}
}

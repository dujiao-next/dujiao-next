package gormstore

import (
	"github.com/dujiao-next/internal/shared/jsonmap"
	"reflect"
	"testing"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
)

func TestSKUMappingStoreListByProductMappingIDs(t *testing.T) {
	_, skuStore, _ := setupMappingStoreTest(t)

	for i, mappingID := range []uint{11, 11, 22} {
		if err := skuStore.Create(&mappingdomain.SKUMapping{
			ProductMappingID: mappingID,
			LocalSKUID:       uint(100 + i),
			UpstreamSKUID:    uint(200 + i),
		}); err != nil {
			t.Fatalf("create sku mapping failed: %v", err)
		}
	}

	if rows, err := skuStore.ListByProductMappingIDs(nil); err != nil || rows != nil {
		t.Fatalf("empty input should short-circuit: rows=%v err=%v", rows, err)
	}
	rows, err := skuStore.ListByProductMappingIDs([]uint{11})
	if err != nil || len(rows) != 2 {
		t.Fatalf("list by mapping ids: rows=%d err=%v, want 2", len(rows), err)
	}
}

func TestSKUMappingStoreGetByLocalSKUIDReturnsNilWhenMissing(t *testing.T) {
	_, skuStore, _ := setupMappingStoreTest(t)

	mapping, err := skuStore.GetByLocalSKUID(999)
	if err != nil || mapping != nil {
		t.Fatalf("missing sku mapping should be nil without error: mapping=%v err=%v", mapping, err)
	}
}

func TestSKUMappingStoreDeleteHidesRowsAndPersistsMarker(t *testing.T) {
	_, skuStore, db := setupMappingStoreTest(t)
	mapping := &mappingdomain.SKUMapping{
		ProductMappingID: 77,
		LocalSKUID:       88,
		UpstreamSKUID:    99,
	}
	if err := skuStore.Create(mapping); err != nil {
		t.Fatalf("create sku mapping: %v", err)
	}
	if err := skuStore.DeleteByProductMapping(mapping.ProductMappingID); err != nil {
		t.Fatalf("delete sku mapping: %v", err)
	}
	if got, err := skuStore.GetByLocalSKUID(mapping.LocalSKUID); err != nil || got != nil {
		t.Fatalf("GetByLocalSKUID after delete = %#v, %v; want nil, nil", got, err)
	}
	var persisted mappingdomain.SKUMapping
	if err := db.Where("id = ?", mapping.ID).First(&persisted).Error; err != nil {
		t.Fatalf("load persisted sku mapping: %v", err)
	}
	if persisted.DeletedAt == nil {
		t.Fatal("DeleteByProductMapping must persist deleted_at")
	}
}

func TestSKUMappingStorePersistsUpstreamMetadata(t *testing.T) {
	_, store, _ := setupMappingStoreTest(t)
	mapping := &mappingdomain.SKUMapping{ProductMappingID: 1, LocalSKUID: 81, UpstreamSKUID: 91, UpstreamMetadata: jsonmap.JSON{"code": "product-code", "race": "race-a", "sku": "sku-a"}}
	if err := store.Create(mapping); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetByLocalSKUID(81)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || !reflect.DeepEqual(loaded.UpstreamMetadata, mapping.UpstreamMetadata) {
		t.Fatalf("metadata not persisted: %#v", loaded)
	}
	loaded.UpstreamMetadata = jsonmap.JSON{"code": "product-code", "race": "race-b", "sku": "sku-b"}
	if err := store.Update(loaded); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.GetByLocalSKUID(81)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded == nil || !reflect.DeepEqual(reloaded.UpstreamMetadata, loaded.UpstreamMetadata) {
		t.Fatalf("metadata not updated: %#v", reloaded)
	}
}

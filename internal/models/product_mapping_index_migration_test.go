package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMigrateProductMappingLocalProductIndexAllowsRebind(t *testing.T) {
	dsn := fmt.Sprintf("file:product_mapping_index_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Setting{}, &ProductMapping{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	const indexName = "idx_product_mappings_local_product_id"
	if db.Migrator().HasIndex(&ProductMapping{}, indexName) {
		if err := db.Migrator().DropIndex(&ProductMapping{}, indexName); err != nil {
			t.Fatalf("drop current index: %v", err)
		}
	}
	if err := db.Exec("CREATE UNIQUE INDEX " + indexName + " ON product_mappings(local_product_id)").Error; err != nil {
		t.Fatalf("create legacy unique index: %v", err)
	}

	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })
	if err := migrateProductMappingLocalProductIndex(); err != nil {
		t.Fatalf("migrate index: %v", err)
	}
	if err := migrateProductMappingLocalProductIndex(); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}

	first := ProductMapping{ConnectionID: 1, LocalProductID: 100, UpstreamProductID: 100}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first mapping: %v", err)
	}
	if err := db.Delete(&first).Error; err != nil {
		t.Fatalf("soft delete first mapping: %v", err)
	}
	second := ProductMapping{ConnectionID: 1, LocalProductID: 100, UpstreamProductID: 100}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create replacement mapping: %v", err)
	}
}

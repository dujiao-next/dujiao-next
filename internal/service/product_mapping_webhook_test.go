package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestBindGenericWebhookProduct(t *testing.T) {
	dsn := fmt.Sprintf("file:generic_webhook_binding_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.SiteConnection{}, &models.Product{}, &models.ProductSKU{},
		&models.ProductMapping{}, &models.SKUMapping{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-app-secret", t.TempDir())
	conn, err := connSvc.Create(CreateConnectionInput{
		Name:        "webhook",
		BaseURL:     "https://provider.example.com/orders",
		ApiKey:      "token",
		Protocol:    constants.ConnectionProtocolGenericWebhook,
		CallbackURL: "https://shop.example.com/api/v1/upstream/generic-webhook/callback",
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	product := &models.Product{
		Slug:            "webhook-product",
		TitleJSON:       models.JSON{"zh-CN": "Webhook Product"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	skus := []models.ProductSKU{
		{ProductID: product.ID, SKUCode: "A", PriceAmount: product.PriceAmount, IsActive: true},
		{ProductID: product.ID, SKUCode: "B", PriceAmount: product.PriceAmount, IsActive: false},
	}
	if err := db.Create(&skus).Error; err != nil {
		t.Fatalf("create skus: %v", err)
	}
	if err := db.Model(&models.ProductSKU{}).Where("id = ?", skus[1].ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("disable sku: %v", err)
	}

	svc := NewProductMappingService(
		repository.NewProductMappingRepository(db), repository.NewSKUMappingRepository(db),
		repository.NewProductRepository(db), repository.NewProductSKURepository(db),
		repository.NewCategoryRepository(db), connSvc,
	)
	mapping, err := svc.BindGenericWebhookProduct(conn.ID, product.ID)
	if err != nil {
		t.Fatalf("BindGenericWebhookProduct: %v", err)
	}
	if mapping.UpstreamProductID != product.ID || mapping.UpstreamFulfillmentType != constants.FulfillmentTypeManual {
		t.Fatalf("unexpected mapping: %#v", mapping)
	}

	skuMappings, err := svc.GetSKUMappings(mapping.ID)
	if err != nil || len(skuMappings) != 1 {
		t.Fatalf("unexpected sku mappings: %#v err=%v", skuMappings, err)
	}
	if skuMappings[0].UpstreamSKUID != skus[0].ID || skuMappings[0].UpstreamStock != -1 {
		t.Fatalf("unexpected sku mapping: %#v", skuMappings[0])
	}
	refreshed, err := repository.NewProductRepository(db).GetAdminByID(fmt.Sprintf("%d", product.ID))
	if err != nil || refreshed.FulfillmentType != constants.FulfillmentTypeUpstream || !refreshed.IsMapped || refreshed.IsActive {
		t.Fatalf("unexpected bound product: %#v err=%v", refreshed, err)
	}
	if err := svc.SyncProduct(mapping.ID); !errors.Is(err, ErrProtocolCapabilityUnsupported) {
		t.Fatalf("expected sync capability error, got %v", err)
	}
	if _, err := svc.BindGenericWebhookProduct(conn.ID, product.ID); !errors.Is(err, ErrMappingAlreadyExists) {
		t.Fatalf("expected duplicate mapping error, got %v", err)
	}
}

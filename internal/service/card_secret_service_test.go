package service

import (
	"bytes"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupCardSecretServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:card_secret_service_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Product{},
		&models.ProductSKU{},
		&models.CardSecretBatch{},
		&models.CardSecret{},
		&models.CardSecretExport{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	models.DB = db
	return db
}

func newCardSecretCSVFileHeader(t *testing.T, content string) *multipart.FileHeader {
	t.Helper()
	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "card-secrets.csv")
	if err != nil {
		t.Fatalf("create csv form file failed: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write csv form file failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close csv multipart writer failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/admin/card-secrets/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		t.Fatalf("parse multipart form failed: %v", err)
	}
	_, fileHeader, err := req.FormFile("file")
	if err != nil {
		t.Fatalf("read multipart file header failed: %v", err)
	}
	return fileHeader
}

func TestCreateCardSecretBatchAutoMultiSKURequiresExplicitSKU(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)

	product := &models.Product{
		CategoryID:      1,
		Slug:            "card-secret-product-default",
		TitleJSON:       models.JSON{"zh-CN": "卡密商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}
	otherSKU := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     "PRO",
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(30)),
		IsActive:    true,
	}
	if err := db.Create(otherSKU).Error; err != nil {
		t.Fatalf("create other sku failed: %v", err)
	}

	svc := NewCardSecretService(
		repository.NewCardSecretRepository(db),
		repository.NewCardSecretBatchRepository(db),
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
	)

	batch, created, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"AAA-001", "AAA-002"},
		Source:    constants.CardSecretSourceManual,
		AdminID:   1,
	})
	if err != ErrProductSKURequired {
		t.Fatalf("create card secret batch error want %v got %v", ErrProductSKURequired, err)
	}
	if batch != nil || created != 0 {
		t.Fatalf("batch should not be created when sku is omitted for auto multi-sku product")
	}
}

func TestCreateCardSecretBatchAutoSingleActiveFallsBackToOnlyActiveSKU(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)

	product := &models.Product{
		CategoryID:      1,
		Slug:            "card-secret-product-single-active",
		TitleJSON:       models.JSON{"zh-CN": "卡密商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		IsActive:    false,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}
	if err := db.Model(defaultSKU).Update("is_active", false).Error; err != nil {
		t.Fatalf("disable default sku failed: %v", err)
	}
	onlyActiveSKU := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     "PRO",
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(30)),
		IsActive:    true,
	}
	if err := db.Create(onlyActiveSKU).Error; err != nil {
		t.Fatalf("create active sku failed: %v", err)
	}

	svc := NewCardSecretService(
		repository.NewCardSecretRepository(db),
		repository.NewCardSecretBatchRepository(db),
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
	)

	batch, created, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"AAA-101", "AAA-102"},
		Source:    constants.CardSecretSourceManual,
		AdminID:   1,
	})
	if err != nil {
		t.Fatalf("create card secret batch failed: %v", err)
	}
	if created != 2 {
		t.Fatalf("created count want 2 got %d", created)
	}
	if batch.SKUID != onlyActiveSKU.ID {
		t.Fatalf("batch sku_id want active %d got %d", onlyActiveSKU.ID, batch.SKUID)
	}
}

func TestCreateCardSecretBatchAlwaysDeduplicatesInput(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)

	product := &models.Product{
		CategoryID:      1,
		Slug:            "card-secret-deduplicate-option",
		TitleJSON:       models.JSON{"zh-CN": "卡密去重商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}

	svc := NewCardSecretService(
		repository.NewCardSecretRepository(db),
		repository.NewCardSecretBatchRepository(db),
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
	)

	defaultBatch, created, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"DEDUP-DEFAULT-001", "DEDUP-DEFAULT-001", "DEDUP-DEFAULT-002"},
		BatchNo:   "DEDUP-DEFAULT",
		Source:    constants.CardSecretSourceManual,
	})
	if err != nil {
		t.Fatalf("create default deduplicate batch failed: %v", err)
	}
	if created != 2 || defaultBatch.TotalCount != 2 {
		t.Fatalf("default deduplicate want created=2 total=2 got created=%d total=%d", created, defaultBatch.TotalCount)
	}

	duplicateBatch, created, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"DEDUP-SECOND-001", "DEDUP-SECOND-001", "DEDUP-SECOND-002"},
		BatchNo:   "DEDUP-SECOND",
		Source:    constants.CardSecretSourceManual,
	})
	if err != nil {
		t.Fatalf("create second deduplicated batch failed: %v", err)
	}
	if created != 2 || duplicateBatch.TotalCount != 2 {
		t.Fatalf("forced deduplicate want created=2 total=2 got created=%d total=%d", created, duplicateBatch.TotalCount)
	}

	items, total, err := svc.ListCardSecrets(ListCardSecretInput{
		ProductID: product.ID,
		BatchID:   duplicateBatch.ID,
		Page:      1,
		PageSize:  20,
	})
	if err != nil {
		t.Fatalf("list duplicate batch failed: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("deduplicated batch list want total=2 len=2 got total=%d len=%d", total, len(items))
	}
	repeated := 0
	for _, item := range items {
		if item.Secret == "DEDUP-SECOND-001" {
			repeated++
		}
	}
	if repeated != 1 {
		t.Fatalf("expected repeated secret to be stored once, got %d", repeated)
	}
}

func TestImportCardSecretCSVAlwaysDeduplicatesInput(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)

	product := &models.Product{
		CategoryID:      1,
		Slug:            "card-secret-csv-deduplicate-option",
		TitleJSON:       models.JSON{"zh-CN": "CSV 卡密去重商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}

	svc := NewCardSecretService(
		repository.NewCardSecretRepository(db),
		repository.NewCardSecretBatchRepository(db),
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
	)

	batch, created, err := svc.ImportCardSecretCSV(ImportCardSecretCSVInput{
		ProductID: product.ID,
		File:      newCardSecretCSVFileHeader(t, "secret\nCSV-DUP-001\nCSV-DUP-001\nCSV-DUP-002\n"),
		BatchNo:   "CSV-DEDUP",
	})
	if err != nil {
		t.Fatalf("import csv with duplicate secrets failed: %v", err)
	}
	if created != 2 || batch.TotalCount != 2 {
		t.Fatalf("csv forced deduplicate want created=2 total=2 got created=%d total=%d", created, batch.TotalCount)
	}

	items, total, err := svc.ListCardSecrets(ListCardSecretInput{
		ProductID: product.ID,
		BatchID:   batch.ID,
		Page:      1,
		PageSize:  20,
	})
	if err != nil {
		t.Fatalf("list csv duplicate batch failed: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("csv deduplicated batch list want total=2 len=2 got total=%d len=%d", total, len(items))
	}
	repeated := 0
	for _, item := range items {
		if item.Secret == "CSV-DUP-001" {
			repeated++
		}
	}
	if repeated != 1 {
		t.Fatalf("expected csv repeated secret to be stored once, got %d", repeated)
	}
}

func TestCreateCardSecretBatchRequiresConfirmationAndRefreshesDuplicates(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)
	product := &models.Product{
		CategoryID:      1,
		Slug:            "card-secret-overwrite-duplicates",
		TitleJSON:       models.JSON{"zh-CN": "覆盖导入商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		IsActive:    true,
	}
	if err := db.Create(sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	svc := NewCardSecretService(
		repository.NewCardSecretRepository(db),
		repository.NewCardSecretBatchRepository(db),
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
	)

	originalBatch, _, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		SKUID:     sku.ID,
		Secrets:   []string{"OVERWRITE-AVAILABLE", "OVERWRITE-USED"},
		BatchNo:   "ORIGINAL",
	})
	if err != nil {
		t.Fatalf("create original batch failed: %v", err)
	}
	oldImportedAt := time.Now().Add(-24 * time.Hour).Truncate(time.Second)
	orderID := uint(42)
	if err := db.Model(&models.CardSecret{}).
		Where("product_id = ? AND sku_id = ?", product.ID, sku.ID).
		Updates(map[string]interface{}{"created_at": oldImportedAt, "updated_at": oldImportedAt}).Error; err != nil {
		t.Fatalf("seed old import time failed: %v", err)
	}
	if err := db.Model(&models.CardSecret{}).
		Where("product_id = ? AND sku_id = ? AND secret = ?", product.ID, sku.ID, "OVERWRITE-USED").
		Updates(map[string]interface{}{"status": models.CardSecretStatusUsed, "order_id": orderID}).Error; err != nil {
		t.Fatalf("seed used card secret failed: %v", err)
	}

	batch, imported, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		SKUID:     sku.ID,
		Secrets:   []string{"OVERWRITE-AVAILABLE", "OVERWRITE-USED", "OVERWRITE-NEW"},
		BatchNo:   "REIMPORT",
	})
	var duplicateErr *CardSecretDuplicatesError
	if !errors.As(err, &duplicateErr) {
		t.Fatalf("expected duplicate confirmation error, got %v", err)
	}
	if batch != nil || imported != 0 {
		t.Fatalf("duplicate preview must not import data")
	}
	if strings.Join(duplicateErr.Secrets, ",") != "OVERWRITE-AVAILABLE,OVERWRITE-USED" {
		t.Fatalf("unexpected duplicate list: %+v", duplicateErr.Secrets)
	}
	var batchCount int64
	if err := db.Model(&models.CardSecretBatch{}).Count(&batchCount).Error; err != nil {
		t.Fatalf("count batches failed: %v", err)
	}
	if batchCount != 1 {
		t.Fatalf("duplicate preview should not create a batch, got %d", batchCount)
	}
	var secretCountBeforeOverwrite int64
	if err := db.Model(&models.CardSecret{}).
		Where("product_id = ? AND sku_id = ?", product.ID, sku.ID).
		Count(&secretCountBeforeOverwrite).Error; err != nil {
		t.Fatalf("count secrets after duplicate preview failed: %v", err)
	}
	if secretCountBeforeOverwrite != 2 {
		t.Fatalf("duplicate preview must not partially import fresh secrets, got %d rows", secretCountBeforeOverwrite)
	}

	reimportBatch, imported, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID:           product.ID,
		SKUID:               sku.ID,
		Secrets:             []string{"OVERWRITE-AVAILABLE", "OVERWRITE-USED", "OVERWRITE-NEW"},
		BatchNo:             "REIMPORT",
		OverwriteDuplicates: true,
	})
	if err != nil {
		t.Fatalf("overwrite duplicate import failed: %v", err)
	}
	if imported != 3 || reimportBatch.TotalCount != 3 || reimportBatch.ID == originalBatch.ID {
		t.Fatalf("unexpected overwrite result: batch=%+v imported=%d", reimportBatch, imported)
	}
	rows, total, err := svc.ListCardSecrets(ListCardSecretInput{
		ProductID: product.ID,
		SKUID:     sku.ID,
		BatchID:   reimportBatch.ID,
		Page:      1,
		PageSize:  10,
	})
	if err != nil {
		t.Fatalf("list overwritten batch failed: %v", err)
	}
	if total != 3 || len(rows) != 3 {
		t.Fatalf("overwrite should keep three unique rows, total=%d len=%d", total, len(rows))
	}
	for _, row := range rows {
		if !row.CreatedAt.After(oldImportedAt) {
			t.Fatalf("import time was not refreshed: %+v", row)
		}
		if row.Secret == "OVERWRITE-USED" {
			if row.Status != models.CardSecretStatusUsed || row.OrderID == nil || *row.OrderID != orderID {
				t.Fatalf("used status and order relation must be preserved: %+v", row)
			}
		}
	}
	var secretCount int64
	if err := db.Model(&models.CardSecret{}).Where("product_id = ? AND sku_id = ?", product.ID, sku.ID).Count(&secretCount).Error; err != nil {
		t.Fatalf("count secrets failed: %v", err)
	}
	if secretCount != 3 {
		t.Fatalf("overwrite import must not create duplicate rows, got %d", secretCount)
	}
}

func TestCardSecretServiceSupportsBatchTargetOperations(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)

	product := &models.Product{
		CategoryID:      1,
		Slug:            "card-secret-batch-ops",
		TitleJSON:       models.JSON{"zh-CN": "卡密批次商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(50)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(50)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}

	secretRepo := repository.NewCardSecretRepository(db)
	svc := NewCardSecretService(
		secretRepo,
		repository.NewCardSecretBatchRepository(db),
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
	)

	batchA, created, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"BATCH-A-001", "BATCH-A-002"},
		Source:    constants.CardSecretSourceManual,
	})
	if err != nil {
		t.Fatalf("create batch A failed: %v", err)
	}
	if created != 2 {
		t.Fatalf("batch A created want 2 got %d", created)
	}

	batchB, created, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"BATCH-B-001"},
		Source:    constants.CardSecretSourceManual,
	})
	if err != nil {
		t.Fatalf("create batch B failed: %v", err)
	}
	if created != 1 {
		t.Fatalf("batch B created want 1 got %d", created)
	}

	rows, total, err := svc.ListCardSecrets(ListCardSecretInput{
		ProductID: product.ID,
		BatchID:   batchA.ID,
		Page:      1,
		PageSize:  20,
	})
	if err != nil {
		t.Fatalf("list card secrets by batch failed: %v", err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("list by batch A want total=2 len=2 got total=%d len=%d", total, len(rows))
	}
	for _, row := range rows {
		if row.BatchID == nil || *row.BatchID != batchA.ID {
			t.Fatalf("expected batch A id %d got %+v", batchA.ID, row.BatchID)
		}
	}

	affected, err := svc.BatchUpdateCardSecretStatus(nil, batchA.ID, ListCardSecretInput{}, models.CardSecretStatusUsed)
	if err != nil {
		t.Fatalf("batch update status by batch id failed: %v", err)
	}
	if affected != 2 {
		t.Fatalf("batch update affected want 2 got %d", affected)
	}

	batchAIDs, err := secretRepo.ListIDsByBatchID(batchA.ID)
	if err != nil {
		t.Fatalf("list batch A ids failed: %v", err)
	}
	batchASecrets, err := secretRepo.ListByIDs(batchAIDs)
	if err != nil {
		t.Fatalf("list batch A secrets failed: %v", err)
	}
	for _, row := range batchASecrets {
		if row.Status != models.CardSecretStatusUsed {
			t.Fatalf("batch A status want used got %s", row.Status)
		}
	}

	batchBIDs, err := secretRepo.ListIDsByBatchID(batchB.ID)
	if err != nil {
		t.Fatalf("list batch B ids failed: %v", err)
	}
	batchBSecrets, err := secretRepo.ListByIDs(batchBIDs)
	if err != nil {
		t.Fatalf("list batch B secrets failed: %v", err)
	}
	if len(batchBSecrets) != 1 || batchBSecrets[0].Status != models.CardSecretStatusAvailable {
		t.Fatalf("batch B status should remain available, got %+v", batchBSecrets)
	}

	content, contentType, err := svc.ExportCardSecrets(nil, batchA.ID, ListCardSecretInput{}, constants.ExportFormatTXT)
	if err != nil {
		t.Fatalf("export batch A secrets failed: %v", err)
	}
	if contentType != "text/plain; charset=utf-8" {
		t.Fatalf("export content type mismatch: %s", contentType)
	}
	exported := string(content)
	if !strings.Contains(exported, "BATCH-A-001") || !strings.Contains(exported, "BATCH-A-002") {
		t.Fatalf("exported content missing batch A secrets: %s", exported)
	}
	if strings.Contains(exported, "BATCH-B-001") {
		t.Fatalf("exported content should not contain batch B secret: %s", exported)
	}

	deleted, err := svc.BatchDeleteCardSecrets(nil, batchB.ID, ListCardSecretInput{})
	if err != nil {
		t.Fatalf("delete batch B secrets failed: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("delete batch B affected want 1 got %d", deleted)
	}

	batchBIDs, err = secretRepo.ListIDsByBatchID(batchB.ID)
	if err != nil {
		t.Fatalf("reload batch B ids failed: %v", err)
	}
	if len(batchBIDs) != 0 {
		t.Fatalf("batch B ids want empty got %v", batchBIDs)
	}
}

func TestExportCardSecretsWithEmptyFilterExportsCurrentResults(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)

	product := &models.Product{
		CategoryID:      1,
		Slug:            "card-secret-export-empty-filter",
		TitleJSON:       models.JSON{"zh-CN": "卡密导出商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	defaultSKU := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}

	svc := NewCardSecretService(
		repository.NewCardSecretRepository(db),
		repository.NewCardSecretBatchRepository(db),
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
	)
	if _, _, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"EMPTY-FILTER-001", "EMPTY-FILTER-002"},
		BatchNo:   "EMPTY-FILTER",
		Source:    constants.CardSecretSourceManual,
	}); err != nil {
		t.Fatalf("create card secret batch failed: %v", err)
	}

	content, contentType, err := svc.ExportCardSecrets(nil, 0, ListCardSecretInput{}, constants.ExportFormatTXT)
	if err != nil {
		t.Fatalf("export with empty filter failed: %v", err)
	}
	if contentType != "text/plain; charset=utf-8" {
		t.Fatalf("content type want text/plain got %s", contentType)
	}
	exported := string(content)
	if !strings.Contains(exported, "EMPTY-FILTER-001") || !strings.Contains(exported, "EMPTY-FILTER-002") {
		t.Fatalf("exported content missing current results: %s", exported)
	}
}

func TestCardSecretServiceSupportsKeywordAndBatchNoFilters(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)

	product := &models.Product{
		CategoryID:      1,
		Slug:            "card-secret-search",
		TitleJSON:       models.JSON{"zh-CN": "卡密搜索商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(30)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(30)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}

	svc := NewCardSecretService(
		repository.NewCardSecretRepository(db),
		repository.NewCardSecretBatchRepository(db),
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
	)

	if _, _, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"AAA-SEARCH-001", "AAA-SEARCH-002"},
		BatchNo:   "BATCH-SEARCH-A",
		Source:    constants.CardSecretSourceManual,
	}); err != nil {
		t.Fatalf("create batch A failed: %v", err)
	}
	if _, _, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"BBB-KEEP-001"},
		BatchNo:   "BATCH-SEARCH-B",
		Source:    constants.CardSecretSourceManual,
	}); err != nil {
		t.Fatalf("create batch B failed: %v", err)
	}

	items, total, err := svc.ListCardSecrets(ListCardSecretInput{
		ProductID: product.ID,
		Secret:    "SEARCH-002",
		Page:      1,
		PageSize:  20,
	})
	if err != nil {
		t.Fatalf("filter by secret failed: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Secret != "AAA-SEARCH-002" {
		t.Fatalf("filter by secret mismatch, total=%d items=%+v", total, items)
	}

	items, total, err = svc.ListCardSecrets(ListCardSecretInput{
		ProductID: product.ID,
		BatchNo:   "SEARCH-A",
		Page:      1,
		PageSize:  20,
	})
	if err != nil {
		t.Fatalf("filter by batch no failed: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("filter by batch no want total=2 len=2 got total=%d len=%d", total, len(items))
	}
}

func TestCardSecretServiceListBatchesReturnsRealtimeCounts(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)

	product := &models.Product{
		CategoryID:      1,
		Slug:            "card-secret-batch-summary",
		TitleJSON:       models.JSON{"zh-CN": "卡密批次统计商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(88)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(88)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}

	svc := NewCardSecretService(
		repository.NewCardSecretRepository(db),
		repository.NewCardSecretBatchRepository(db),
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
	)

	batchA, _, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"SUMMARY-A-001", "SUMMARY-A-002"},
		BatchNo:   "SUMMARY-A",
		Source:    constants.CardSecretSourceManual,
	})
	if err != nil {
		t.Fatalf("create batch A failed: %v", err)
	}
	batchB, _, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"SUMMARY-B-001"},
		BatchNo:   "SUMMARY-B",
		Source:    constants.CardSecretSourceManual,
	})
	if err != nil {
		t.Fatalf("create batch B failed: %v", err)
	}

	rows, err := repository.NewCardSecretRepository(db).ListIDs(repository.CardSecretListFilter{
		ProductID: product.ID,
		BatchID:   batchA.ID,
	})
	if err != nil {
		t.Fatalf("list batch A ids failed: %v", err)
	}
	if _, err := svc.BatchUpdateCardSecretStatus(rows[:1], 0, ListCardSecretInput{}, models.CardSecretStatusReserved); err != nil {
		t.Fatalf("mark batch A reserved failed: %v", err)
	}
	if _, err := svc.BatchUpdateCardSecretStatus(rows[1:], 0, ListCardSecretInput{}, models.CardSecretStatusUsed); err != nil {
		t.Fatalf("mark batch A used failed: %v", err)
	}
	if _, err := svc.BatchDeleteCardSecrets(nil, batchB.ID, ListCardSecretInput{}); err != nil {
		t.Fatalf("delete batch B failed: %v", err)
	}

	summaries, total, err := svc.ListBatches(product.ID, defaultSKU.ID, 1, 20)
	if err != nil {
		t.Fatalf("list batches failed: %v", err)
	}
	if total != 2 || len(summaries) != 2 {
		t.Fatalf("list batches want total=2 len=2 got total=%d len=%d", total, len(summaries))
	}

	for _, summary := range summaries {
		switch summary.BatchNo {
		case "SUMMARY-A":
			if summary.TotalCount != 2 || summary.AvailableCount != 0 || summary.ReservedCount != 1 || summary.UsedCount != 1 {
				t.Fatalf("summary A mismatch: %+v", summary)
			}
		case "SUMMARY-B":
			if summary.TotalCount != 0 || summary.AvailableCount != 0 || summary.ReservedCount != 0 || summary.UsedCount != 0 {
				t.Fatalf("summary B mismatch: %+v", summary)
			}
		default:
			t.Fatalf("unexpected batch summary: %+v", summary)
		}
	}
}

func TestExportAvailableCardSecretsMarksUsed(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)
	product := &models.Product{
		CategoryID:      1,
		Slug:            "export-available-used",
		TitleJSON:       models.JSON{"zh-CN": "出库导出商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(88)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(88)),
		IsActive:    true,
	}
	if err := db.Create(sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	svc := NewCardSecretService(
		repository.NewCardSecretRepository(db),
		repository.NewCardSecretBatchRepository(db),
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
	)
	batch, _, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"EXP-A-001", "EXP-A-002", "EXP-A-003"},
		BatchNo:   "EXP-A",
		Source:    constants.CardSecretSourceManual,
	})
	if err != nil {
		t.Fatalf("create batch failed: %v", err)
	}
	if err := db.Model(&models.CardSecret{}).
		Where("product_id = ? AND secret = ?", product.ID, "EXP-A-001").
		Update("order_id", 99).Error; err != nil {
		t.Fatalf("seed stale order id failed: %v", err)
	}

	result, err := svc.ExportAvailableCardSecrets(ExportAvailableCardSecretInput{
		ProductID: product.ID,
		SKUID:     sku.ID,
		BatchID:   batch.ID,
		Limit:     2,
		Format:    constants.ExportFormatTXT,
		AdminID:   9,
	})
	if err != nil {
		t.Fatalf("export available failed: %v", err)
	}
	if result.Count != 2 || strings.TrimSpace(string(result.Content)) != "EXP-A-001\nEXP-A-002" {
		t.Fatalf("unexpected export result: count=%d content=%q", result.Count, string(result.Content))
	}
	if result.Record == nil || result.Record.ID == 0 || result.Record.AdminID != 9 {
		t.Fatalf("expected export record, got %+v", result.Record)
	}
	records, total, err := svc.ListCardSecretExports(ListCardSecretExportInput{ID: result.Record.ID, Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(records) != 1 || records[0].Count != 2 {
		t.Fatalf("expected queryable export record, rows=%+v total=%d err=%v", records, total, err)
	}

	rows, _, err := svc.ListCardSecrets(ListCardSecretInput{
		ProductID: product.ID,
		SKUID:     sku.ID,
		BatchID:   batch.ID,
		Page:      1,
		PageSize:  10,
	})
	if err != nil {
		t.Fatalf("list secrets failed: %v", err)
	}
	statusBySecret := map[string]string{}
	for _, row := range rows {
		statusBySecret[row.Secret] = row.Status
		if row.Secret != "EXP-A-003" && (row.ExportID == nil || *row.ExportID != result.Record.ID) {
			t.Fatalf("expected exported secret linked to record: %+v", row)
		}
		if row.Secret != "EXP-A-003" && row.OrderID != nil {
			t.Fatalf("expected exported secret order link cleared: %+v", row)
		}
	}
	if statusBySecret["EXP-A-001"] != models.CardSecretStatusUsed ||
		statusBySecret["EXP-A-002"] != models.CardSecretStatusUsed ||
		statusBySecret["EXP-A-003"] != models.CardSecretStatusAvailable {
		t.Fatalf("unexpected statuses: %+v", statusBySecret)
	}
}

func TestExportAvailableCardSecretsDeletesAfterExport(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)
	product := &models.Product{
		CategoryID:      1,
		Slug:            "export-available-delete",
		TitleJSON:       models.JSON{"zh-CN": "导出删除商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(88)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := &models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(88)),
		IsActive:    true,
	}
	if err := db.Create(sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	svc := NewCardSecretService(
		repository.NewCardSecretRepository(db),
		repository.NewCardSecretBatchRepository(db),
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
	)
	batch, _, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID: product.ID,
		Secrets:   []string{"EXP-D-001", "EXP-D-002"},
		BatchNo:   "EXP-D",
		Source:    constants.CardSecretSourceManual,
	})
	if err != nil {
		t.Fatalf("create batch failed: %v", err)
	}

	result, err := svc.ExportAvailableCardSecrets(ExportAvailableCardSecretInput{
		ProductID:         product.ID,
		SKUID:             sku.ID,
		BatchID:           batch.ID,
		Limit:             1,
		Format:            constants.ExportFormatTXT,
		DeleteAfterExport: true,
		AdminID:           10,
	})
	if err != nil {
		t.Fatalf("export available with delete failed: %v", err)
	}
	if result.Count != 1 || strings.TrimSpace(string(result.Content)) != "EXP-D-001" {
		t.Fatalf("unexpected export result: count=%d content=%q", result.Count, string(result.Content))
	}
	if result.Record == nil || !result.Record.DeleteAfterExport || result.Record.AdminID != 10 {
		t.Fatalf("expected delete export record, got %+v", result.Record)
	}

	rows, _, err := svc.ListCardSecrets(ListCardSecretInput{
		ProductID: product.ID,
		SKUID:     sku.ID,
		BatchID:   batch.ID,
		Page:      1,
		PageSize:  10,
	})
	if err != nil {
		t.Fatalf("list secrets failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Secret != "EXP-D-002" || rows[0].Status != models.CardSecretStatusAvailable {
		t.Fatalf("unexpected remaining rows: %+v", rows)
	}
}

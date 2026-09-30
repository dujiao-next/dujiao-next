package integrationtest

import (
	"bytes"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	"github.com/dujiao-next/internal/constants"
	cardsecretapp "github.com/dujiao-next/internal/modules/cardsecret/application"
	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	cardsecretgormstore "github.com/dujiao-next/internal/modules/cardsecret/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type CreateCardSecretBatchInput = cardsecretapp.CreateCardSecretBatchInput
type ImportCardSecretCSVInput = cardsecretapp.ImportCardSecretCSVInput
type ListCardSecretInput = cardsecretapp.ListCardSecretInput
type ExportAvailableCardSecretInput = cardsecretapp.ExportAvailableCardSecretInput

var (
	ErrProductSKURequired     = cardsecretapp.ErrProductSKURequired
	ErrProductSKUInvalid      = cardsecretapp.ErrProductSKUInvalid
	ErrNotFound               = cardsecretapp.ErrNotFound
	ErrCardSecretInvalid      = cardsecretapp.ErrInvalid
	ErrCardSecretInsufficient = cardsecretapp.ErrInsufficient
)

func NewCardSecretService(
	secrets *cardsecretgormstore.Store,
	batches *cardsecretgormstore.BatchStore,
	products productcontract.Repository,
	productSKUs productcontract.SKURepository,
) *cardsecretapp.Service {
	return cardsecretapp.NewService(cardsecretapp.ServiceOptions{
		Secrets:      secrets,
		Batches:      batches,
		Transactions: secrets,
		Products:     products,
		ProductSKUs:  productSKUs,
	})
}

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
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&cardsecretdomain.Batch{},
		&cardsecretdomain.Secret{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	gormdb.DB = db
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

	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            "card-secret-product-default",
		TitleJSON:       jsonmap.JSON{"zh-CN": "卡密商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(20)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(20)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}
	otherSKU := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     "PRO",
		PriceAmount: money.FromDecimal(decimal.NewFromInt(30)),
		IsActive:    true,
	}
	if err := db.Create(otherSKU).Error; err != nil {
		t.Fatalf("create other sku failed: %v", err)
	}

	svc := NewCardSecretService(
		cardsecretgormstore.New(db),
		cardsecretgormstore.NewBatch(db),
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
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

	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            "card-secret-product-single-active",
		TitleJSON:       jsonmap.JSON{"zh-CN": "卡密商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(20)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(20)),
		IsActive:    false,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}
	if err := db.Model(defaultSKU).Update("is_active", false).Error; err != nil {
		t.Fatalf("disable default sku failed: %v", err)
	}
	onlyActiveSKU := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     "PRO",
		PriceAmount: money.FromDecimal(decimal.NewFromInt(30)),
		IsActive:    true,
	}
	if err := db.Create(onlyActiveSKU).Error; err != nil {
		t.Fatalf("create active sku failed: %v", err)
	}

	svc := NewCardSecretService(
		cardsecretgormstore.New(db),
		cardsecretgormstore.NewBatch(db),
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
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

func TestCreateCardSecretBatchDeduplicateOption(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)

	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            "card-secret-deduplicate-option",
		TitleJSON:       jsonmap.JSON{"zh-CN": "卡密去重商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(20)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(20)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}

	svc := NewCardSecretService(
		cardsecretgormstore.New(db),
		cardsecretgormstore.NewBatch(db),
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
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

	keepDuplicates := false
	duplicateBatch, created, err := svc.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID:   product.ID,
		Secrets:     []string{"DEDUP-OFF-001", "DEDUP-OFF-001", "DEDUP-OFF-002"},
		BatchNo:     "DEDUP-OFF",
		Source:      constants.CardSecretSourceManual,
		Deduplicate: &keepDuplicates,
	})
	if err != nil {
		t.Fatalf("create keep duplicate batch failed: %v", err)
	}
	if created != 3 || duplicateBatch.TotalCount != 3 {
		t.Fatalf("keep duplicate want created=3 total=3 got created=%d total=%d", created, duplicateBatch.TotalCount)
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
	if total != 3 || len(items) != 3 {
		t.Fatalf("duplicate batch list want total=3 len=3 got total=%d len=%d", total, len(items))
	}
	repeated := 0
	for _, item := range items {
		if item.Secret == "DEDUP-OFF-001" {
			repeated++
		}
	}
	if repeated != 2 {
		t.Fatalf("expected repeated secret to be stored twice, got %d", repeated)
	}
}

func TestImportCardSecretCSVKeepsDuplicatesWhenDeduplicateDisabled(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)

	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            "card-secret-csv-deduplicate-option",
		TitleJSON:       jsonmap.JSON{"zh-CN": "CSV 卡密去重商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(20)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(20)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}

	svc := NewCardSecretService(
		cardsecretgormstore.New(db),
		cardsecretgormstore.NewBatch(db),
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
	)

	keepDuplicates := false
	batch, created, err := svc.ImportCardSecretCSV(ImportCardSecretCSVInput{
		ProductID:   product.ID,
		File:        newCardSecretCSVFileHeader(t, "secret\nCSV-DUP-001\nCSV-DUP-001\nCSV-DUP-002\n"),
		BatchNo:     "CSV-DEDUP-OFF",
		Deduplicate: &keepDuplicates,
	})
	if err != nil {
		t.Fatalf("import csv with duplicate secrets failed: %v", err)
	}
	if created != 3 || batch.TotalCount != 3 {
		t.Fatalf("csv keep duplicate want created=3 total=3 got created=%d total=%d", created, batch.TotalCount)
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
	if total != 3 || len(items) != 3 {
		t.Fatalf("csv duplicate batch list want total=3 len=3 got total=%d len=%d", total, len(items))
	}
	repeated := 0
	for _, item := range items {
		if item.Secret == "CSV-DUP-001" {
			repeated++
		}
	}
	if repeated != 2 {
		t.Fatalf("expected csv repeated secret to be stored twice, got %d", repeated)
	}
}

func TestCardSecretServiceSupportsBatchTargetOperations(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)

	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            "card-secret-batch-ops",
		TitleJSON:       jsonmap.JSON{"zh-CN": "卡密批次商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(50)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(50)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}

	secretRepo := cardsecretgormstore.New(db)
	svc := NewCardSecretService(
		secretRepo,
		cardsecretgormstore.NewBatch(db),
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
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

	affected, err := svc.BatchUpdateCardSecretStatus(nil, batchA.ID, ListCardSecretInput{}, cardsecretdomain.StatusUsed)
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
		if row.Status != cardsecretdomain.StatusUsed {
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
	if len(batchBSecrets) != 1 || batchBSecrets[0].Status != cardsecretdomain.StatusAvailable {
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
	var softDeleted cardsecretdomain.Secret
	if err := db.First(&softDeleted, batchBIDs[0]).Error; err != nil {
		t.Fatalf("load soft-deleted batch B secret failed: %v", err)
	}
	if softDeleted.DeletedAt == nil {
		t.Fatal("batch delete must preserve the row with deleted_at set")
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

	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            "card-secret-export-empty-filter",
		TitleJSON:       jsonmap.JSON{"zh-CN": "卡密导出商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(20)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	defaultSKU := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(20)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}

	svc := NewCardSecretService(
		cardsecretgormstore.New(db),
		cardsecretgormstore.NewBatch(db),
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
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

	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            "card-secret-search",
		TitleJSON:       jsonmap.JSON{"zh-CN": "卡密搜索商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(30)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(30)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}

	svc := NewCardSecretService(
		cardsecretgormstore.New(db),
		cardsecretgormstore.NewBatch(db),
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
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

	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            "card-secret-batch-summary",
		TitleJSON:       jsonmap.JSON{"zh-CN": "卡密批次统计商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(88)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(88)),
		IsActive:    true,
	}
	if err := db.Create(defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}

	svc := NewCardSecretService(
		cardsecretgormstore.New(db),
		cardsecretgormstore.NewBatch(db),
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
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

	rows, err := cardsecretgormstore.New(db).ListIDs(cardsecretcontract.ListFilter{
		ProductID: product.ID,
		BatchID:   batchA.ID,
	})
	if err != nil {
		t.Fatalf("list batch A ids failed: %v", err)
	}
	if _, err := svc.BatchUpdateCardSecretStatus(rows[:1], 0, ListCardSecretInput{}, cardsecretdomain.StatusReserved); err != nil {
		t.Fatalf("mark batch A reserved failed: %v", err)
	}
	if _, err := svc.BatchUpdateCardSecretStatus(rows[1:], 0, ListCardSecretInput{}, cardsecretdomain.StatusUsed); err != nil {
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
	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            "export-available-used",
		TitleJSON:       jsonmap.JSON{"zh-CN": "出库导出商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(88)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(88)),
		IsActive:    true,
	}
	if err := db.Create(sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	svc := NewCardSecretService(
		cardsecretgormstore.New(db),
		cardsecretgormstore.NewBatch(db),
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
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

	result, err := svc.ExportAvailableCardSecrets(ExportAvailableCardSecretInput{
		ProductID: product.ID,
		SKUID:     sku.ID,
		BatchID:   batch.ID,
		Limit:     2,
		Format:    constants.ExportFormatTXT,
	})
	if err != nil {
		t.Fatalf("export available failed: %v", err)
	}
	if result.Count != 2 || strings.TrimSpace(string(result.Content)) != "EXP-A-001\nEXP-A-002" {
		t.Fatalf("unexpected export result: count=%d content=%q", result.Count, string(result.Content))
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
	}
	if statusBySecret["EXP-A-001"] != cardsecretdomain.StatusUsed ||
		statusBySecret["EXP-A-002"] != cardsecretdomain.StatusUsed ||
		statusBySecret["EXP-A-003"] != cardsecretdomain.StatusAvailable {
		t.Fatalf("unexpected statuses: %+v", statusBySecret)
	}
}

func TestExportAvailableCardSecretsDeletesAfterExport(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)
	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            "export-available-delete",
		TitleJSON:       jsonmap.JSON{"zh-CN": "导出删除商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(88)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(88)),
		IsActive:    true,
	}
	if err := db.Create(sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	svc := NewCardSecretService(
		cardsecretgormstore.New(db),
		cardsecretgormstore.NewBatch(db),
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
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
	})
	if err != nil {
		t.Fatalf("export available with delete failed: %v", err)
	}
	if result.Count != 1 || strings.TrimSpace(string(result.Content)) != "EXP-D-001" {
		t.Fatalf("unexpected export result: count=%d content=%q", result.Count, string(result.Content))
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
	if len(rows) != 1 || rows[0].Secret != "EXP-D-002" || rows[0].Status != cardsecretdomain.StatusAvailable {
		t.Fatalf("unexpected remaining rows: %+v", rows)
	}
}

// 出库导出不应带走循环卡密（循环卡密可无限发货，导出后标记已用会让商品变成售罄）。
func TestExportAvailableCardSecretsSkipsLoopSecrets(t *testing.T) {
	db := setupCardSecretServiceTestDB(t)
	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            "export-available-loop",
		TitleJSON:       jsonmap.JSON{"zh-CN": "循环卡密导出商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(88)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(88)),
		IsActive:    true,
	}
	if err := db.Create(sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	svc := NewCardSecretService(
		cardsecretgormstore.New(db),
		cardsecretgormstore.NewBatch(db),
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
	)
	// 循环卡密先录入，ID 更小；若导出不排除它会被优先取走
	for _, input := range []CreateCardSecretBatchInput{
		{ProductID: product.ID, Secrets: []string{"EXP-LOOP"}, BatchNo: "EXP-LOOP", Source: constants.CardSecretSourceManual, IsLoop: true},
		{ProductID: product.ID, Secrets: []string{"EXP-NORMAL"}, BatchNo: "EXP-NORMAL", Source: constants.CardSecretSourceManual},
	} {
		if _, _, err := svc.CreateCardSecretBatch(input); err != nil {
			t.Fatalf("create batch failed: %v", err)
		}
	}

	if _, err := svc.ExportAvailableCardSecrets(ExportAvailableCardSecretInput{
		ProductID: product.ID, SKUID: sku.ID, Limit: 2, Format: constants.ExportFormatTXT,
	}); !errors.Is(err, cardsecretapp.ErrInsufficient) {
		t.Fatalf("export 2 with only 1 normal secret want ErrInsufficient, got %v", err)
	}
	result, err := svc.ExportAvailableCardSecrets(ExportAvailableCardSecretInput{
		ProductID: product.ID, SKUID: sku.ID, Limit: 1, Format: constants.ExportFormatTXT,
	})
	if err != nil {
		t.Fatalf("export available failed: %v", err)
	}
	if strings.TrimSpace(string(result.Content)) != "EXP-NORMAL" {
		t.Fatalf("export should only include normal secret, got %q", string(result.Content))
	}

	rows, _, err := svc.ListCardSecrets(ListCardSecretInput{ProductID: product.ID, SKUID: sku.ID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list secrets failed: %v", err)
	}
	for _, row := range rows {
		if row.Secret == "EXP-LOOP" && row.Status != cardsecretdomain.StatusAvailable {
			t.Fatalf("loop secret should stay available, got %s", row.Status)
		}
	}
}

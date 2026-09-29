package integrationtest

import (
	"testing"
	"time"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	cardsecretgormstore "github.com/dujiao-next/internal/modules/cardsecret/infrastructure/gormstore"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
)

func TestApplyAutoStockCounts_LegacyStockPrefersDefaultSKU(t *testing.T) {
	svc, db := newAutoStockProductService(t)
	productID := uint(3001)
	defaultSKUID := uint(101)
	otherSKUID := uint(102)

	insertCardSecrets(t, db, productID, 0, cardsecretdomain.StatusAvailable, 2)
	insertCardSecrets(t, db, productID, 0, cardsecretdomain.StatusReserved, 1)
	insertCardSecrets(t, db, productID, 0, cardsecretdomain.StatusUsed, 1)
	insertCardSecrets(t, db, productID, defaultSKUID, cardsecretdomain.StatusAvailable, 3)
	insertCardSecrets(t, db, productID, otherSKUID, cardsecretdomain.StatusAvailable, 4)
	counts, err := cardsecretgormstore.New(db).CountStockByProductIDs([]uint{productID})
	if err != nil {
		t.Fatalf("count stock by product ids failed: %v", err)
	}
	if len(counts) != 5 {
		t.Fatalf("expected 5 grouped stock rows, got %d", len(counts))
	}
	bySKUAndStatus := make(map[uint]map[string]int64)
	for _, row := range counts {
		if bySKUAndStatus[row.SKUID] == nil {
			bySKUAndStatus[row.SKUID] = make(map[string]int64)
		}
		bySKUAndStatus[row.SKUID][row.Status] = row.Total
	}
	if bySKUAndStatus[0][cardsecretdomain.StatusAvailable] != 2 ||
		bySKUAndStatus[0][cardsecretdomain.StatusReserved] != 1 ||
		bySKUAndStatus[0][cardsecretdomain.StatusUsed] != 1 {
		t.Fatalf("unexpected legacy sku(0) rows: %+v", bySKUAndStatus[0])
	}
	if bySKUAndStatus[defaultSKUID][cardsecretdomain.StatusAvailable] != 3 {
		t.Fatalf("unexpected default sku rows: %+v", bySKUAndStatus[defaultSKUID])
	}
	if bySKUAndStatus[otherSKUID][cardsecretdomain.StatusAvailable] != 4 {
		t.Fatalf("unexpected other sku rows: %+v", bySKUAndStatus[otherSKUID])
	}

	products := []productdomain.Product{
		{
			ID:              productID,
			FulfillmentType: constants.FulfillmentTypeAuto,
			SKUs: []productdomain.ProductSKU{
				{
					ID:       otherSKUID,
					SKUCode:  "B",
					IsActive: true,
				},
				{
					ID:       defaultSKUID,
					SKUCode:  productdomain.DefaultSKUCode,
					IsActive: true,
				},
			},
		},
	}

	if err := svc.Read.ApplyAutoStockCounts(products); err != nil {
		t.Fatalf("apply auto stock counts failed: %v", err)
	}

	got := products[0]
	if got.AutoStockAvailable != 9 {
		t.Fatalf("expected product auto available=9, got %d", got.AutoStockAvailable)
	}
	if got.AutoStockLocked != 1 {
		t.Fatalf("expected product auto locked=1, got %d", got.AutoStockLocked)
	}
	if got.AutoStockSold != 1 {
		t.Fatalf("expected product auto sold=1, got %d", got.AutoStockSold)
	}
	if got.AutoStockTotal != 10 {
		t.Fatalf("expected product auto total=10, got %d", got.AutoStockTotal)
	}

	if got.SKUs[0].AutoStockAvailable != 4 {
		t.Fatalf("expected other sku auto available=4, got %d", got.SKUs[0].AutoStockAvailable)
	}
	if got.SKUs[0].AutoStockLocked != 0 || got.SKUs[0].AutoStockSold != 0 {
		t.Fatalf("expected other sku locked/sold to remain 0, got locked=%d sold=%d", got.SKUs[0].AutoStockLocked, got.SKUs[0].AutoStockSold)
	}

	if got.SKUs[1].AutoStockAvailable != 5 {
		t.Fatalf("expected default sku auto available=5, got %d", got.SKUs[1].AutoStockAvailable)
	}
	if got.SKUs[1].AutoStockLocked != 1 {
		t.Fatalf("expected default sku auto locked=1, got %d", got.SKUs[1].AutoStockLocked)
	}
	if got.SKUs[1].AutoStockSold != 1 {
		t.Fatalf("expected default sku auto sold=1, got %d", got.SKUs[1].AutoStockSold)
	}
}

func TestApplyAutoStockCounts_LoopSecretMarksSKUUnlimited(t *testing.T) {
	svc, db := newAutoStockProductService(t)
	productID := uint(3101)
	loopSKUID := uint(201)
	normalSKUID := uint(202)

	insertCardSecrets(t, db, productID, loopSKUID, cardsecretdomain.StatusAvailable, 2)
	insertCardSecrets(t, db, productID, normalSKUID, cardsecretdomain.StatusAvailable, 4)
	now := time.Now()
	for _, row := range []cardsecretdomain.Secret{
		{ProductID: productID, SKUID: loopSKUID, Secret: "LOOP", Status: cardsecretdomain.StatusAvailable, IsLoop: true},
		// 已停用的循环卡密不生效，按普通状态计数
		{ProductID: productID, SKUID: normalSKUID, Secret: "LOOP-DISABLED", Status: cardsecretdomain.StatusUsed, IsLoop: true},
	} {
		row.CreatedAt, row.UpdatedAt = now, now
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create loop secret failed: %v", err)
		}
	}

	products := []productdomain.Product{{
		ID:              productID,
		FulfillmentType: constants.FulfillmentTypeAuto,
		SKUs: []productdomain.ProductSKU{
			{ID: loopSKUID, SKUCode: productdomain.DefaultSKUCode, IsActive: true},
			{ID: normalSKUID, SKUCode: "B", IsActive: true},
		},
	}}
	if err := svc.Read.ApplyAutoStockCounts(products); err != nil {
		t.Fatalf("apply auto stock counts failed: %v", err)
	}

	got := products[0]
	if got.AutoStockAvailable != -1 {
		t.Fatalf("expected product auto available=-1, got %d", got.AutoStockAvailable)
	}
	if got.SKUs[0].AutoStockAvailable != -1 {
		t.Fatalf("expected loop sku auto available=-1, got %d", got.SKUs[0].AutoStockAvailable)
	}
	if got.SKUs[1].AutoStockAvailable != 4 {
		t.Fatalf("expected normal sku auto available=4, got %d", got.SKUs[1].AutoStockAvailable)
	}
	if got.SKUs[1].AutoStockSold != 1 {
		t.Fatalf("expected disabled loop secret counted as sold=1, got %d", got.SKUs[1].AutoStockSold)
	}
}

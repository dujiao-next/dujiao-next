package integrationtest

import (
	"strconv"
	"strings"
	"testing"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	productwrite "github.com/dujiao-next/internal/modules/catalog/product/application/write"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestProductServiceUpdateRejectsDisablingAutoSKUWithCardSecretStock(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	category := categorydomain.Category{
		Slug:     "auto-card-secret-category",
		NameJSON: jsonmap.JSON{"zh-CN": "auto-card-secret-category"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "auto-card-secret-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "auto-card-secret-product"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	stockSKU := productdomain.ProductSKU{
		ProductID:      product.ID,
		SKUCode:        "SKU-STOCK",
		SpecValuesJSON: jsonmap.JSON{"zh-CN": "有库存"},
		PriceAmount:    money.FromDecimal(decimal.NewFromInt(10)),
		IsActive:       true,
		SortOrder:      2,
	}
	spareSKU := productdomain.ProductSKU{
		ProductID:      product.ID,
		SKUCode:        "SKU-SPARE",
		SpecValuesJSON: jsonmap.JSON{"zh-CN": "无库存"},
		PriceAmount:    money.FromDecimal(decimal.NewFromInt(10)),
		IsActive:       true,
		SortOrder:      1,
	}
	if err := db.Create(&stockSKU).Error; err != nil {
		t.Fatalf("create stock sku failed: %v", err)
	}
	if err := db.Create(&spareSKU).Error; err != nil {
		t.Fatalf("create spare sku failed: %v", err)
	}

	insertCardSecrets(t, db, product.ID, stockSKU.ID, cardsecretdomain.StatusAvailable, 1)

	_, err := svc.Write.Update(strconv.FormatUint(uint64(product.ID), 10), productwrite.CreateProductInput{
		CategoryID:      category.ID,
		Slug:            product.Slug,
		TitleJSON:       map[string]interface{}{"zh-CN": "auto-card-secret-product"},
		PriceAmount:     decimal.NewFromInt(10),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		SKUs: []productwrite.ProductSKUInput{
			{
				ID:             stockSKU.ID,
				SKUCode:        stockSKU.SKUCode,
				SpecValuesJSON: map[string]interface{}{"zh-CN": "有库存"},
				PriceAmount:    decimal.NewFromInt(10),
				IsActive: func() *bool {
					value := false
					return &value
				}(),
				SortOrder: 2,
			},
			{
				ID:             spareSKU.ID,
				SKUCode:        spareSKU.SKUCode,
				SpecValuesJSON: map[string]interface{}{"zh-CN": "无库存"},
				PriceAmount:    decimal.NewFromInt(10),
				IsActive: func() *bool {
					value := true
					return &value
				}(),
				SortOrder: 1,
			},
		},
		IsActive: func() *bool {
			value := true
			return &value
		}(),
	})
	if err != productcontract.ErrProductSKUHasCardSecretStock {
		t.Fatalf("update product error want %v got %v", productcontract.ErrProductSKUHasCardSecretStock, err)
	}
}

// 复现 issue #344：后台删光全部规格后保存，商品必须真正回落为单规格。
//
// 之前的实现会在收到空 SKUs 时"复活"一行已存在的规格行，用户重新打开商品就看到规格
// 又回来了，表现为"关不掉 SKU 规格配置"。这里走完整的 Update 路径（含 db.Save 关联
// 级联），确认落库后只剩一行 DEFAULT，并且反复保存保持稳定。
func TestProductServiceUpdateWithEmptySKUsCollapsesToSingleSpec(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	category := categorydomain.Category{
		Slug:     "collapse-single-spec-category",
		NameJSON: jsonmap.JSON{"zh-CN": "collapse-single-spec-category"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "collapse-single-spec-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "collapse-single-spec-product"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	defaultSKU := productdomain.ProductSKU{
		ProductID:      product.ID,
		SKUCode:        productdomain.DefaultSKUCode,
		SpecValuesJSON: jsonmap.JSON{},
		PriceAmount:    money.FromDecimal(decimal.NewFromInt(10)),
		IsActive:       true,
		SortOrder:      0,
	}
	extraSKU := productdomain.ProductSKU{
		ProductID:      product.ID,
		SKUCode:        "SKU-EXTRA",
		SpecValuesJSON: jsonmap.JSON{"zh-CN": "规格一"},
		PriceAmount:    money.FromDecimal(decimal.NewFromInt(20)),
		IsActive:       true,
		SortOrder:      1,
	}
	if err := db.Create(&defaultSKU).Error; err != nil {
		t.Fatalf("create default sku failed: %v", err)
	}
	if err := db.Create(&extraSKU).Error; err != nil {
		t.Fatalf("create extra sku failed: %v", err)
	}

	productID := strconv.FormatUint(uint64(product.ID), 10)
	buildInput := func() productwrite.CreateProductInput {
		return productwrite.CreateProductInput{
			CategoryID:      category.ID,
			Slug:            product.Slug,
			TitleJSON:       map[string]interface{}{"zh-CN": "collapse-single-spec-product"},
			PriceAmount:     decimal.NewFromInt(10),
			PurchaseType:    constants.ProductPurchaseMember,
			FulfillmentType: constants.FulfillmentTypeAuto,
			SKUs:            nil, // 后台把规格全删了
			IsActive: func() *bool {
				value := true
				return &value
			}(),
		}
	}

	// 连续保存两次：既验证首次收敛，也验证重开后再次保存不会把规格带回来。
	for round := 1; round <= 2; round++ {
		if _, err := svc.Write.Update(productID, buildInput()); err != nil {
			t.Fatalf("round %d: update product failed: %v", round, err)
		}

		stored, err := productgormstore.NewProductStore(db).GetAdminByID(productID)
		if err != nil {
			t.Fatalf("round %d: reload product failed: %v", round, err)
		}
		if stored == nil {
			t.Fatalf("round %d: product disappeared", round)
		}
		if len(stored.SKUs) != 1 {
			t.Fatalf("round %d: expected exactly one sku after collapsing, got %d: %+v", round, len(stored.SKUs), stored.SKUs)
		}
		if !strings.EqualFold(stored.SKUs[0].SKUCode, productdomain.DefaultSKUCode) {
			t.Fatalf("round %d: expected surviving sku code DEFAULT, got %q", round, stored.SKUs[0].SKUCode)
		}
		if !stored.SKUs[0].IsActive {
			t.Fatalf("round %d: expected surviving DEFAULT sku to be active: %+v", round, stored.SKUs[0])
		}
	}

	// 规格表里也不能留下软删除之外的残留行。
	var rowCount int64
	if err := db.Model(&productdomain.ProductSKU{}).
		Where("product_id = ? AND deleted_at IS NULL", product.ID).
		Count(&rowCount).Error; err != nil {
		t.Fatalf("count sku rows failed: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected exactly one live sku row in db, got %d", rowCount)
	}
}

// 后台打开编辑框后有买家下单预占库存，管理员只改标题保存（未提交库存）时，
// SKU 的实时库存不能被表单里的旧值覆盖，否则会超卖。
func TestProductServiceUpdateWithoutStockKeepsReservedSKUStock(t *testing.T) {
	svc, db := newProductServiceForTest(t)
	category := categorydomain.Category{Slug: "keep-stock-category", NameJSON: jsonmap.JSON{"zh-CN": "keep-stock"}}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	product := productdomain.Product{
		CategoryID:       category.ID,
		Slug:             "keep-stock-product",
		TitleJSON:        jsonmap.JSON{"zh-CN": "keep-stock-product"},
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(10)),
		PurchaseType:     constants.ProductPurchaseMember,
		FulfillmentType:  constants.FulfillmentTypeManual,
		ManualStockTotal: 10,
		IsActive:         true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	// 单规格 DEFAULT 与多规格各验证一次
	single := productdomain.ProductSKU{ProductID: product.ID, SKUCode: productdomain.DefaultSKUCode, PriceAmount: money.FromDecimal(decimal.NewFromInt(10)), ManualStockTotal: 7, ManualStockLocked: 3, IsActive: true}
	if err := db.Create(&single).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	active := true
	update := func(skus []productwrite.ProductSKUInput) {
		t.Helper()
		if _, err := svc.Write.Update(strconv.FormatUint(uint64(product.ID), 10), productwrite.CreateProductInput{
			CategoryID:      category.ID,
			Slug:            product.Slug,
			TitleJSON:       map[string]interface{}{"zh-CN": "renamed"},
			PriceAmount:     decimal.NewFromInt(10),
			PurchaseType:    constants.ProductPurchaseMember,
			FulfillmentType: constants.FulfillmentTypeManual,
			SKUs:            skus,
			IsActive:        &active,
		}); err != nil {
			t.Fatalf("update product failed: %v", err)
		}
	}
	update(nil)
	var reloaded productdomain.ProductSKU
	db.First(&reloaded, single.ID)
	if reloaded.ManualStockTotal != 7 {
		t.Fatalf("single spec: expected live stock 7 kept, got %d", reloaded.ManualStockTotal)
	}

	update([]productwrite.ProductSKUInput{{ID: single.ID, SKUCode: "A", PriceAmount: decimal.NewFromInt(10), IsActive: &active}})
	db.First(&reloaded, single.ID)
	if reloaded.ManualStockTotal != 7 {
		t.Fatalf("multi spec: expected live stock 7 kept, got %d", reloaded.ManualStockTotal)
	}
}

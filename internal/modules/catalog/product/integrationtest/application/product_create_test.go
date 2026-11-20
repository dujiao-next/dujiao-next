package integrationtest

import (
	"errors"
	"testing"

	productwrite "github.com/dujiao-next/internal/modules/catalog/product/application/write"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestProductServiceCreateRejectsParentCategoryWithChildren(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	parent := categorydomain.Category{
		Slug:     "games",
		NameJSON: jsonmap.JSON{"zh-CN": "games"},
	}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent category failed: %v", err)
	}
	child := categorydomain.Category{
		ParentID: parent.ID,
		Slug:     "steam",
		NameJSON: jsonmap.JSON{"zh-CN": "steam"},
	}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child category failed: %v", err)
	}

	_, err := svc.Write.Create(productwrite.CreateProductInput{
		CategoryID:      parent.ID,
		Slug:            "invalid-parent-product",
		TitleJSON:       map[string]interface{}{"zh-CN": "invalid-parent-product"},
		PriceAmount:     decimal.NewFromInt(10),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		ManualStockTotal: func() *int {
			value := 1
			return &value
		}(),
	})
	if err != productcontract.ErrProductCategoryInvalid {
		t.Fatalf("expected productcontract.ErrProductCategoryInvalid, got %v", err)
	}
}

func TestProductServiceCreateFiltersUnavailablePaymentChannels(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	category := categorydomain.Category{
		Slug:     "payment-channel-category",
		NameJSON: jsonmap.JSON{"zh-CN": "payment-channel-category"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	activeChannel := createProductTestPaymentChannel(t, db, "Active", true, false)
	inactiveChannel := createProductTestPaymentChannel(t, db, "Inactive", false, false)
	deletedChannel := createProductTestPaymentChannel(t, db, "Deleted", true, true)

	product, err := svc.Write.Create(productwrite.CreateProductInput{
		CategoryID:        category.ID,
		Slug:              "payment-channel-create",
		TitleJSON:         map[string]interface{}{"zh-CN": "payment-channel-create"},
		PriceAmount:       decimal.NewFromInt(10),
		PurchaseType:      constants.ProductPurchaseMember,
		FulfillmentType:   constants.FulfillmentTypeAuto,
		PaymentChannelIDs: []uint{deletedChannel.ID, inactiveChannel.ID, activeChannel.ID},
		IsActive: func() *bool {
			value := true
			return &value
		}(),
	})
	if err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	got := productdomain.DecodePaymentChannelIDs(product.PaymentChannelIDs)
	if len(got) != 1 || got[0] != activeChannel.ID {
		t.Fatalf("expected only active payment channel %d, got %v", activeChannel.ID, got)
	}
}

func TestProductServiceCreateRejectsInvalidPurchaseLimits(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	cat := categorydomain.Category{Slug: "test-purchase-limit", NameJSON: jsonmap.JSON{"zh-CN": "test"}}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	intPtr := func(v int) *int { return &v }

	_, err := svc.Write.Create(productwrite.CreateProductInput{
		CategoryID:          cat.ID,
		Slug:                "invalid-limit-product",
		TitleJSON:           map[string]interface{}{"zh-CN": "invalid-limit-product"},
		PriceAmount:         decimal.NewFromInt(10),
		PurchaseType:        constants.ProductPurchaseMember,
		FulfillmentType:     constants.FulfillmentTypeManual,
		ManualStockTotal:    intPtr(1),
		MinPurchaseQuantity: intPtr(10),
		MaxPurchaseQuantity: intPtr(5),
	})
	if !errors.Is(err, productcontract.ErrProductPurchaseLimitInvalid) {
		t.Fatalf("expected productcontract.ErrProductPurchaseLimitInvalid, got %v", err)
	}
}

func TestProductServiceCreateRollsBackProductAndSKUWhenWholesaleValidationFails(t *testing.T) {
	svc, db := newProductServiceForTest(t)
	category := categorydomain.Category{
		Slug:     "write-rollback-category",
		NameJSON: jsonmap.JSON{"zh-CN": "write-rollback-category"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	_, err := svc.Write.Create(productwrite.CreateProductInput{
		CategoryID:      category.ID,
		Slug:            "write-rollback-product",
		TitleJSON:       map[string]interface{}{"zh-CN": "write-rollback-product"},
		PriceAmount:     decimal.NewFromInt(10),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		WholesalePrices: &[]productdomain.WholesalePriceInput{{
			MinQuantity: 0,
			UnitPrice:   decimal.NewFromInt(8),
		}},
	})
	if !errors.Is(err, productdomain.ErrWholesalePriceInvalid) {
		t.Fatalf("expected productdomain.ErrWholesalePriceInvalid, got %v", err)
	}

	var productCount int64
	if err := db.Model(&productdomain.Product{}).Where("slug = ?", "write-rollback-product").Count(&productCount).Error; err != nil {
		t.Fatalf("count products: %v", err)
	}
	if productCount != 0 {
		t.Fatalf("transaction must roll back product, got %d rows", productCount)
	}
	var skuCount int64
	if err := db.Model(&productdomain.ProductSKU{}).Count(&skuCount).Error; err != nil {
		t.Fatalf("count product SKUs: %v", err)
	}
	if skuCount != 0 {
		t.Fatalf("transaction must roll back default SKU, got %d rows", skuCount)
	}
}

// 创建商品时返利比例的默认值：未携带字段存 0（不参与返利），显式 null 才跟随全站，数值按请求覆盖。
// API 脚本和仍发送旧字段 is_affiliate_enabled 的客户端不会带 affiliate_rate_percent，不能因此悄悄跟随全站比例。
func TestProductServiceCreateAffiliateRateDefaults(t *testing.T) {
	svc, db := newProductServiceForTest(t)
	category := categorydomain.Category{
		Slug:     "affiliate-default-category",
		NameJSON: jsonmap.JSON{"zh-CN": "affiliate-default-category"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	create := func(t *testing.T, slug string, rate *money.Amount, set bool) *productdomain.Product {
		t.Helper()
		product, err := svc.Write.Create(productwrite.CreateProductInput{
			CategoryID:              category.ID,
			Slug:                    slug,
			TitleJSON:               map[string]interface{}{"zh-CN": slug},
			PriceAmount:             decimal.NewFromInt(10),
			PurchaseType:            constants.ProductPurchaseMember,
			FulfillmentType:         constants.FulfillmentTypeAuto,
			AffiliateRatePercent:    rate,
			AffiliateRatePercentSet: set,
		})
		if err != nil {
			t.Fatalf("create product %s failed: %v", slug, err)
		}
		return product
	}

	if got := create(t, "affiliate-default-missing", nil, false); got.AffiliateRatePercent == nil || !got.AffiliateRatePercent.IsZero() {
		t.Fatalf("missing field should default to 0, got %+v", got.AffiliateRatePercent)
	}
	if got := create(t, "affiliate-default-null", nil, true); got.AffiliateRatePercent != nil {
		t.Fatalf("explicit null should follow the site-wide rate, got %+v", got.AffiliateRatePercent)
	}
	custom := money.FromDecimal(decimal.NewFromInt(25))
	if got := create(t, "affiliate-default-custom", &custom, true); got.AffiliateRatePercent == nil || !got.AffiliateRatePercent.Equal(decimal.NewFromInt(25)) {
		t.Fatalf("explicit rate should be stored, got %+v", got.AffiliateRatePercent)
	}
}

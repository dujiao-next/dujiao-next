package service

import (
	"encoding/json"
	"errors"
	"fmt"
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

type wholesaleOrderFixture struct {
	db      *gorm.DB
	svc     *OrderService
	product models.Product
	sku     models.ProductSKU
	user    models.User
}

func setupWholesaleOrderFixture(t *testing.T, name string, wholesalePrices models.WholesalePriceTiers, promotionPercent *decimal.Decimal, memberRate *decimal.Decimal) wholesaleOrderFixture {
	t.Helper()

	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", name, time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Category{},
		&models.Product{},
		&models.ProductSKU{},
		&models.Promotion{},
		&models.Coupon{},
		&models.CouponUsage{},
		&models.User{},
		&models.MemberLevel{},
		&models.MemberLevelPrice{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := models.Category{
		Slug:      name + "-category",
		NameJSON:  models.JSON{"zh-CN": "测试分类"},
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	var user models.User
	if memberRate != nil {
		level := models.MemberLevel{
			NameJSON:     models.JSON{"zh-CN": "批发会员"},
			Slug:         name + "-level",
			DiscountRate: models.NewMoneyFromDecimal(*memberRate),
			IsActive:     true,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := db.Create(&level).Error; err != nil {
			t.Fatalf("create member level failed: %v", err)
		}
		user = models.User{
			Email:         name + "@example.com",
			PasswordHash:  "hash",
			Status:        "active",
			MemberLevelID: level.ID,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("create user failed: %v", err)
		}
	}

	product := models.Product{
		CategoryID:      category.ID,
		Slug:            name + "-product",
		TitleJSON:       models.JSON{"zh-CN": "批发测试商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		WholesalePrices: wholesalePrices,
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	sku := models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	if promotionPercent != nil {
		promotion := models.Promotion{
			Name:       name + "-promotion",
			ScopeType:  constants.ScopeTypeProduct,
			ScopeRefID: product.ID,
			Type:       constants.PromotionTypePercent,
			Value:      models.NewMoneyFromDecimal(*promotionPercent),
			MinAmount:  models.NewMoneyFromDecimal(decimal.Zero),
			IsActive:   true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := db.Create(&promotion).Error; err != nil {
			t.Fatalf("create promotion failed: %v", err)
		}
	}

	userRepo := repository.NewUserRepository(db)
	levelRepo := repository.NewMemberLevelRepository(db)
	priceRepo := repository.NewMemberLevelPriceRepository(db)
	svc := NewOrderService(OrderServiceOptions{
		UserRepo:           userRepo,
		ProductRepo:        repository.NewProductRepository(db),
		ProductSKURepo:     repository.NewProductSKURepository(db),
		PromotionRepo:      repository.NewPromotionRepository(db),
		CouponRepo:         repository.NewCouponRepository(db),
		CouponUsageRepo:    repository.NewCouponUsageRepository(db),
		MemberLevelService: NewMemberLevelService(levelRepo, priceRepo, userRepo),
		ExpireMinutes:      15,
	})

	return wholesaleOrderFixture{db: db, svc: svc, product: product, sku: sku, user: user}
}

func createWholesaleOrderCouponFixture(t *testing.T, db *gorm.DB, productIDs []uint, disabledWholesalePrice bool) models.Coupon {
	t.Helper()

	scopeIDs, err := json.Marshal(productIDs)
	if err != nil {
		t.Fatalf("marshal coupon scope failed: %v", err)
	}
	coupon := models.Coupon{
		Code:                   "STACK10",
		Type:                   constants.CouponTypePercent,
		Value:                  models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		MinAmount:              models.NewMoneyFromDecimal(decimal.Zero),
		MaxDiscount:            models.NewMoneyFromDecimal(decimal.Zero),
		ScopeType:              constants.ScopeTypeProduct,
		ScopeRefIDs:            string(scopeIDs),
		DisabledWholesalePrice: disabledWholesalePrice,
		IsActive:               true,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}
	if err := db.Create(&coupon).Error; err != nil {
		t.Fatalf("create coupon failed: %v", err)
	}
	return coupon
}

func TestBuildOrderResultPrefersWholesaleOverPromotion(t *testing.T) {
	wholesalePrices := models.WholesalePriceTiers{
		{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
	}
	promotionPercent := decimal.NewFromInt(10) // 活动价 90，批发价 80 更便宜。
	fixture := setupWholesaleOrderFixture(t, "wholesale_over_promotion", wholesalePrices, &promotionPercent, nil)

	result, err := fixture.svc.buildOrderResult(orderCreateParams{
		Items: []CreateOrderItem{{ProductID: fixture.product.ID, SKUID: fixture.sku.ID, Quantity: 5}},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}

	if !result.OriginalAmount.Equal(decimal.NewFromInt(500)) {
		t.Fatalf("expected original amount 500, got %s", result.OriginalAmount.String())
	}
	if !result.WholesaleDiscountAmount.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("expected wholesale discount 100, got %s", result.WholesaleDiscountAmount.String())
	}
	if !result.PromotionDiscountAmount.IsZero() {
		t.Fatalf("expected promotion discount 0, got %s", result.PromotionDiscountAmount.String())
	}
	if !result.TotalAmount.Equal(decimal.NewFromInt(400)) {
		t.Fatalf("expected total 400, got %s", result.TotalAmount.String())
	}
	item := result.Plans[0].Item
	if item.UnitPrice.String() != "80.00" || item.WholesaleDiscount.String() != "100.00" || item.PromotionDiscount.String() != "0.00" {
		t.Fatalf("unexpected item price result: unit=%s wholesale=%s promotion=%s", item.UnitPrice.String(), item.WholesaleDiscount.String(), item.PromotionDiscount.String())
	}
}

func TestBuildOrderResultPrefersPromotionOverWholesale(t *testing.T) {
	wholesalePrices := models.WholesalePriceTiers{
		{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
	}
	promotionPercent := decimal.NewFromInt(30) // 活动价 70，批发价 80 不生效。
	fixture := setupWholesaleOrderFixture(t, "promotion_over_wholesale", wholesalePrices, &promotionPercent, nil)

	result, err := fixture.svc.buildOrderResult(orderCreateParams{
		Items: []CreateOrderItem{{ProductID: fixture.product.ID, SKUID: fixture.sku.ID, Quantity: 5}},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}

	if !result.PromotionDiscountAmount.Equal(decimal.NewFromInt(150)) {
		t.Fatalf("expected promotion discount 150, got %s", result.PromotionDiscountAmount.String())
	}
	if !result.WholesaleDiscountAmount.IsZero() {
		t.Fatalf("expected wholesale discount 0, got %s", result.WholesaleDiscountAmount.String())
	}
	if !result.TotalAmount.Equal(decimal.NewFromInt(350)) {
		t.Fatalf("expected total 350, got %s", result.TotalAmount.String())
	}
	item := result.Plans[0].Item
	if item.UnitPrice.String() != "70.00" || item.PromotionDiscount.String() != "150.00" || item.WholesaleDiscount.String() != "0.00" {
		t.Fatalf("unexpected item price result: unit=%s wholesale=%s promotion=%s", item.UnitPrice.String(), item.WholesaleDiscount.String(), item.PromotionDiscount.String())
	}
}

func TestBuildOrderResultAppliesCouponAfterBestPromotionOrWholesalePrice(t *testing.T) {
	tests := []struct {
		name              string
		promotionPercent  decimal.Decimal
		wantPromotion     decimal.Decimal
		wantWholesale     decimal.Decimal
		wantCoupon        decimal.Decimal
		wantTotal         decimal.Decimal
		wantUnitPrice     string
		wantCouponPerItem string
	}{
		{
			name:              "wholesale wins before coupon",
			promotionPercent:  decimal.NewFromInt(10),
			wantPromotion:     decimal.Zero,
			wantWholesale:     decimal.NewFromInt(100),
			wantCoupon:        decimal.NewFromInt(40),
			wantTotal:         decimal.NewFromInt(360),
			wantUnitPrice:     "80.00",
			wantCouponPerItem: "40.00",
		},
		{
			name:              "promotion wins before coupon",
			promotionPercent:  decimal.NewFromInt(30),
			wantPromotion:     decimal.NewFromInt(150),
			wantWholesale:     decimal.Zero,
			wantCoupon:        decimal.NewFromInt(35),
			wantTotal:         decimal.NewFromInt(315),
			wantUnitPrice:     "70.00",
			wantCouponPerItem: "35.00",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wholesalePrices := models.WholesalePriceTiers{
				{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
			}
			fixture := setupWholesaleOrderFixture(t, strings.ReplaceAll(tc.name, " ", "_"), wholesalePrices, &tc.promotionPercent, nil)
			coupon := models.Coupon{
				Code:        "STACK10",
				Type:        constants.CouponTypePercent,
				Value:       models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
				MinAmount:   models.NewMoneyFromDecimal(decimal.Zero),
				MaxDiscount: models.NewMoneyFromDecimal(decimal.Zero),
				ScopeType:   constants.ScopeTypeProduct,
				ScopeRefIDs: fmt.Sprintf("[%d]", fixture.product.ID),
				IsActive:    true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			if err := fixture.db.Create(&coupon).Error; err != nil {
				t.Fatalf("create coupon failed: %v", err)
			}

			result, err := fixture.svc.buildOrderResult(orderCreateParams{
				CouponCode: "STACK10",
				Items: []CreateOrderItem{
					{ProductID: fixture.product.ID, SKUID: fixture.sku.ID, Quantity: 5},
				},
			})
			if err != nil {
				t.Fatalf("buildOrderResult failed: %v", err)
			}

			if !result.PromotionDiscountAmount.Equal(tc.wantPromotion) {
				t.Fatalf("expected promotion discount %s, got %s", tc.wantPromotion.String(), result.PromotionDiscountAmount.String())
			}
			if !result.WholesaleDiscountAmount.Equal(tc.wantWholesale) {
				t.Fatalf("expected wholesale discount %s, got %s", tc.wantWholesale.String(), result.WholesaleDiscountAmount.String())
			}
			if !result.DiscountAmount.Equal(tc.wantCoupon) {
				t.Fatalf("expected coupon discount %s, got %s", tc.wantCoupon.String(), result.DiscountAmount.String())
			}
			if !result.TotalAmount.Equal(tc.wantTotal) {
				t.Fatalf("expected total %s, got %s", tc.wantTotal.String(), result.TotalAmount.String())
			}
			if len(result.Plans) != 1 {
				t.Fatalf("expected one plan, got %d", len(result.Plans))
			}
			item := result.Plans[0].Item
			if item.UnitPrice.String() != tc.wantUnitPrice || item.CouponDiscount.String() != tc.wantCouponPerItem {
				t.Fatalf("unexpected item result: unit=%s coupon=%s", item.UnitPrice.String(), item.CouponDiscount.String())
			}
		})
	}
}

func TestBuildOrderResultRejectsCouponWhenDisabledForAllWholesaleItems(t *testing.T) {
	wholesalePrices := models.WholesalePriceTiers{
		{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
	}
	promotionPercent := decimal.NewFromInt(10) // 活动价 90，批发价 80 更便宜并实际生效。
	fixture := setupWholesaleOrderFixture(t, "coupon_disabled_all_wholesale", wholesalePrices, &promotionPercent, nil)
	createWholesaleOrderCouponFixture(t, fixture.db, []uint{fixture.product.ID}, true)

	_, err := fixture.svc.buildOrderResult(orderCreateParams{
		CouponCode: "STACK10",
		Items: []CreateOrderItem{
			{ProductID: fixture.product.ID, SKUID: fixture.sku.ID, Quantity: 5},
		},
	})
	if !errors.Is(err, ErrCouponWholesaleDisabled) {
		t.Fatalf("expected ErrCouponWholesaleDisabled, got %v", err)
	}
}

func TestBuildOrderResultExcludesWholesaleItemsWhenCouponDisabledWholesalePrice(t *testing.T) {
	wholesalePrices := models.WholesalePriceTiers{
		{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
	}
	promotionPercent := decimal.NewFromInt(10) // 商品 A 批发价 80 胜出，应被优惠券排除。
	fixture := setupWholesaleOrderFixture(t, "coupon_disabled_mixed_wholesale", wholesalePrices, &promotionPercent, nil)
	now := time.Now()
	productB := models.Product{
		CategoryID:      fixture.product.CategoryID,
		Slug:            "coupon-disabled-mixed-product-b",
		TitleJSON:       models.JSON{"zh-CN": "非批发商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := fixture.db.Create(&productB).Error; err != nil {
		t.Fatalf("create product b failed: %v", err)
	}
	skuB := models.ProductSKU{
		ProductID:   productB.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := fixture.db.Create(&skuB).Error; err != nil {
		t.Fatalf("create sku b failed: %v", err)
	}
	createWholesaleOrderCouponFixture(t, fixture.db, []uint{fixture.product.ID, productB.ID}, true)

	result, err := fixture.svc.buildOrderResult(orderCreateParams{
		CouponCode: "STACK10",
		Items: []CreateOrderItem{
			{ProductID: fixture.product.ID, SKUID: fixture.sku.ID, Quantity: 5},
			{ProductID: productB.ID, SKUID: skuB.ID, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}
	if !result.WholesaleDiscountAmount.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("expected wholesale discount 100, got %s", result.WholesaleDiscountAmount.String())
	}
	if !result.DiscountAmount.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("expected coupon discount only from product b = 10, got %s", result.DiscountAmount.String())
	}
	if !result.TotalAmount.Equal(decimal.NewFromInt(490)) {
		t.Fatalf("expected total 490, got %s", result.TotalAmount.String())
	}

	byProduct := make(map[uint]models.OrderItem, len(result.Plans))
	for _, plan := range result.Plans {
		byProduct[plan.Item.ProductID] = plan.Item
	}
	wholesaleItem := byProduct[fixture.product.ID]
	if wholesaleItem.CouponDiscount.String() != "0.00" {
		t.Fatalf("expected wholesale item coupon discount 0, got %s", wholesaleItem.CouponDiscount.String())
	}
	regularItem := byProduct[productB.ID]
	if regularItem.CouponDiscount.String() != "10.00" {
		t.Fatalf("expected regular item coupon discount 10, got %s", regularItem.CouponDiscount.String())
	}
}

func TestBuildOrderResultAppliesMemberDiscountAfterWholesale(t *testing.T) {
	wholesalePrices := models.WholesalePriceTiers{
		{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
	}
	memberRate := decimal.NewFromInt(80) // 批发价 80 后再打 8 折，最终 64。
	fixture := setupWholesaleOrderFixture(t, "member_after_wholesale", wholesalePrices, nil, &memberRate)

	result, err := fixture.svc.buildOrderResult(orderCreateParams{
		UserID: fixture.user.ID,
		Items:  []CreateOrderItem{{ProductID: fixture.product.ID, SKUID: fixture.sku.ID, Quantity: 5}},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}

	if !result.WholesaleDiscountAmount.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("expected wholesale discount 100, got %s", result.WholesaleDiscountAmount.String())
	}
	if !result.MemberDiscountAmount.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("expected member discount 80, got %s", result.MemberDiscountAmount.String())
	}
	if !result.TotalAmount.Equal(decimal.NewFromInt(320)) {
		t.Fatalf("expected total 320, got %s", result.TotalAmount.String())
	}
	item := result.Plans[0].Item
	if item.UnitPrice.String() != "64.00" || item.WholesaleDiscount.String() != "100.00" || item.MemberDiscount.String() != "80.00" {
		t.Fatalf("unexpected item price result: unit=%s wholesale=%s member=%s", item.UnitPrice.String(), item.WholesaleDiscount.String(), item.MemberDiscount.String())
	}
}

func TestBuildOrderResultMatchesWholesaleByProductQuantityAcrossSKUs(t *testing.T) {
	wholesalePrices := models.WholesalePriceTiers{
		{MinQuantity: 10, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
	}
	fixture := setupWholesaleOrderFixture(t, "wholesale_across_skus", wholesalePrices, nil, nil)

	skuB := models.ProductSKU{
		ProductID:   fixture.product.ID,
		SKUCode:     "SKU-B",
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := fixture.db.Create(&skuB).Error; err != nil {
		t.Fatalf("create second sku failed: %v", err)
	}

	result, err := fixture.svc.buildOrderResult(orderCreateParams{
		Items: []CreateOrderItem{
			{ProductID: fixture.product.ID, SKUID: fixture.sku.ID, Quantity: 6},
			{ProductID: fixture.product.ID, SKUID: skuB.ID, Quantity: 6},
		},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}

	if !result.OriginalAmount.Equal(decimal.NewFromInt(1200)) {
		t.Fatalf("expected original amount 1200, got %s", result.OriginalAmount.String())
	}
	if !result.WholesaleDiscountAmount.Equal(decimal.NewFromInt(240)) {
		t.Fatalf("expected wholesale discount 240, got %s", result.WholesaleDiscountAmount.String())
	}
	if !result.TotalAmount.Equal(decimal.NewFromInt(960)) {
		t.Fatalf("expected total 960, got %s", result.TotalAmount.String())
	}
	if len(result.Plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(result.Plans))
	}
	for _, plan := range result.Plans {
		if plan.Item.UnitPrice.String() != "80.00" || plan.Item.WholesaleDiscount.String() != "120.00" {
			t.Fatalf("unexpected item price result: sku=%d unit=%s wholesale=%s", plan.Item.SKUID, plan.Item.UnitPrice.String(), plan.Item.WholesaleDiscount.String())
		}
	}
}

// TestBuildOrderResultSkipsWholesaleForCheaperSKU 验证同商品多 SKU 底价不同时的语义：
// 批发门槛按商品总量判定（共享门槛），但批发单价只对「底价高于档位价」的 SKU 生效；
// 底价已低于档位价的 SKU 不会被批发价拉高，各行只计算自己的优惠。
func TestBuildOrderResultSkipsWholesaleForCheaperSKU(t *testing.T) {
	wholesalePrices := models.WholesalePriceTiers{
		{MinQuantity: 10, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
	}
	// 默认 SKU 底价 100（高于档位价 80），新增 SKU 底价 50（低于档位价 80）。
	fixture := setupWholesaleOrderFixture(t, "wholesale_skip_cheaper_sku", wholesalePrices, nil, nil)

	cheaperSKU := models.ProductSKU{
		ProductID:   fixture.product.ID,
		SKUCode:     "SKU-CHEAP",
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(50)),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := fixture.db.Create(&cheaperSKU).Error; err != nil {
		t.Fatalf("create cheaper sku failed: %v", err)
	}

	// 6 + 6 = 12 ≥ 10，命中门槛。
	result, err := fixture.svc.buildOrderResult(orderCreateParams{
		Items: []CreateOrderItem{
			{ProductID: fixture.product.ID, SKUID: fixture.sku.ID, Quantity: 6},
			{ProductID: fixture.product.ID, SKUID: cheaperSKU.ID, Quantity: 6},
		},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}

	// 原价 = 100*6 + 50*6 = 900；批发优惠仅来自底价 100 的 SKU：(100-80)*6 = 120。
	if !result.OriginalAmount.Equal(decimal.NewFromInt(900)) {
		t.Fatalf("expected original amount 900, got %s", result.OriginalAmount.String())
	}
	if !result.WholesaleDiscountAmount.Equal(decimal.NewFromInt(120)) {
		t.Fatalf("expected wholesale discount 120, got %s", result.WholesaleDiscountAmount.String())
	}
	// 实付 = 80*6 + 50*6 = 780。
	if !result.TotalAmount.Equal(decimal.NewFromInt(780)) {
		t.Fatalf("expected total 780, got %s", result.TotalAmount.String())
	}

	bySKU := make(map[uint]models.OrderItem, len(result.Plans))
	for _, plan := range result.Plans {
		bySKU[plan.Item.SKUID] = plan.Item
	}
	high := bySKU[fixture.sku.ID]
	if high.UnitPrice.String() != "80.00" || high.WholesaleDiscount.String() != "120.00" {
		t.Fatalf("expected high-base SKU to use wholesale 80: unit=%s wholesale=%s", high.UnitPrice.String(), high.WholesaleDiscount.String())
	}
	cheap := bySKU[cheaperSKU.ID]
	if cheap.UnitPrice.String() != "50.00" || cheap.WholesaleDiscount.String() != "0.00" {
		t.Fatalf("expected cheaper SKU to keep base 50 without wholesale: unit=%s wholesale=%s", cheap.UnitPrice.String(), cheap.WholesaleDiscount.String())
	}
}

func TestBuildOrderResultAppliesDifferentWholesalePricesPerSKU(t *testing.T) {
	fixture := setupWholesaleOrderFixture(t, "wholesale_per_sku", nil, nil, nil)
	skuB := models.ProductSKU{
		ProductID:   fixture.product.ID,
		SKUCode:     "SKU-B",
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := fixture.db.Create(&skuB).Error; err != nil {
		t.Fatalf("create second sku failed: %v", err)
	}

	wholesalePrices := models.WholesalePriceTiers{
		{SKUID: fixture.sku.ID, SKUCode: fixture.sku.SKUCode, MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(70))},
		{SKUID: skuB.ID, SKUCode: skuB.SKUCode, MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(60))},
	}
	if err := fixture.db.Model(&models.Product{}).Where("id = ?", fixture.product.ID).Update("wholesale_prices", wholesalePrices).Error; err != nil {
		t.Fatalf("update wholesale prices failed: %v", err)
	}

	result, err := fixture.svc.buildOrderResult(orderCreateParams{
		Items: []CreateOrderItem{
			{ProductID: fixture.product.ID, SKUID: fixture.sku.ID, Quantity: 5},
			{ProductID: fixture.product.ID, SKUID: skuB.ID, Quantity: 5},
		},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}

	if !result.OriginalAmount.Equal(decimal.NewFromInt(1000)) {
		t.Fatalf("expected original amount 1000, got %s", result.OriginalAmount.String())
	}
	if !result.WholesaleDiscountAmount.Equal(decimal.NewFromInt(350)) {
		t.Fatalf("expected wholesale discount 350, got %s", result.WholesaleDiscountAmount.String())
	}
	if !result.TotalAmount.Equal(decimal.NewFromInt(650)) {
		t.Fatalf("expected total 650, got %s", result.TotalAmount.String())
	}
	bySKU := make(map[uint]models.OrderItem, len(result.Plans))
	for _, plan := range result.Plans {
		bySKU[plan.Item.SKUID] = plan.Item
	}
	if item := bySKU[fixture.sku.ID]; item.UnitPrice.String() != "70.00" || item.WholesaleDiscount.String() != "150.00" {
		t.Fatalf("unexpected default sku result: unit=%s wholesale=%s", item.UnitPrice.String(), item.WholesaleDiscount.String())
	}
	if item := bySKU[skuB.ID]; item.UnitPrice.String() != "60.00" || item.WholesaleDiscount.String() != "200.00" {
		t.Fatalf("unexpected skuB result: unit=%s wholesale=%s", item.UnitPrice.String(), item.WholesaleDiscount.String())
	}
}

func TestBuildOrderResultSKUWholesaleDoesNotFallbackToUniversalTier(t *testing.T) {
	fixture := setupWholesaleOrderFixture(t, "wholesale_sku_no_fallback", nil, nil, nil)
	skuB := models.ProductSKU{
		ProductID:   fixture.product.ID,
		SKUCode:     "SKU-B",
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := fixture.db.Create(&skuB).Error; err != nil {
		t.Fatalf("create second sku failed: %v", err)
	}

	wholesalePrices := models.WholesalePriceTiers{
		{MinQuantity: 10, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
		{SKUID: fixture.sku.ID, SKUCode: fixture.sku.SKUCode, MinQuantity: 10, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(70))},
	}
	if err := fixture.db.Model(&models.Product{}).Where("id = ?", fixture.product.ID).Update("wholesale_prices", wholesalePrices).Error; err != nil {
		t.Fatalf("update wholesale prices failed: %v", err)
	}

	result, err := fixture.svc.buildOrderResult(orderCreateParams{
		Items: []CreateOrderItem{
			{ProductID: fixture.product.ID, SKUID: fixture.sku.ID, Quantity: 6},
			{ProductID: fixture.product.ID, SKUID: skuB.ID, Quantity: 6},
		},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}

	if !result.OriginalAmount.Equal(decimal.NewFromInt(1200)) {
		t.Fatalf("expected original amount 1200, got %s", result.OriginalAmount.String())
	}
	if !result.WholesaleDiscountAmount.Equal(decimal.NewFromInt(120)) {
		t.Fatalf("expected wholesale discount 120, got %s", result.WholesaleDiscountAmount.String())
	}
	if !result.TotalAmount.Equal(decimal.NewFromInt(1080)) {
		t.Fatalf("expected total 1080, got %s", result.TotalAmount.String())
	}
	bySKU := make(map[uint]models.OrderItem, len(result.Plans))
	for _, plan := range result.Plans {
		bySKU[plan.Item.SKUID] = plan.Item
	}
	if item := bySKU[fixture.sku.ID]; item.UnitPrice.String() != "100.00" || item.WholesaleDiscount.String() != "0.00" {
		t.Fatalf("expected specific SKU to skip universal fallback: unit=%s wholesale=%s", item.UnitPrice.String(), item.WholesaleDiscount.String())
	}
	if item := bySKU[skuB.ID]; item.UnitPrice.String() != "80.00" || item.WholesaleDiscount.String() != "120.00" {
		t.Fatalf("expected skuB to use universal tier: unit=%s wholesale=%s", item.UnitPrice.String(), item.WholesaleDiscount.String())
	}
}

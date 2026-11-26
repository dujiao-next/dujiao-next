package application

import (
	"errors"
	"fmt"
	"testing"
	"time"

	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"

	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"

	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	coupongormstore "github.com/dujiao-next/internal/modules/coupon/infrastructure/gormstore"
	memberlevelapp "github.com/dujiao-next/internal/modules/memberlevel/application"
	memberlevelgormstore "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	promotiongormstore "github.com/dujiao-next/internal/modules/promotion/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type orderPurchaseQuantityLimitFixture struct {
	dsnPrefix       string
	categorySlug    string
	productSlug     string
	minQuantity     int
	maxQuantity     int
	requestQuantity int
	expectedErr     error
}

func assertBuildOrderResultRejectsPurchaseQuantity(t *testing.T, fixture orderPurchaseQuantityLimitFixture) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", fixture.dsnPrefix, time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&categorydomain.Category{}, &productdomain.Product{}, &productdomain.ProductSKU{}, &promotiondomain.Promotion{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := categorydomain.Category{
		Slug:      fixture.categorySlug,
		NameJSON:  jsonmap.JSON{"zh-CN": "测试分类"},
		SortOrder: 0,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := productdomain.Product{
		CategoryID:          category.ID,
		Slug:                fixture.productSlug,
		TitleJSON:           jsonmap.JSON{"zh-CN": "测试商品"},
		PriceAmount:         money.FromDecimal(decimal.NewFromInt(10)),
		PurchaseType:        constants.ProductPurchaseMember,
		FulfillmentType:     constants.FulfillmentTypeManual,
		MinPurchaseQuantity: fixture.minQuantity,
		MaxPurchaseQuantity: fixture.maxQuantity,
		IsActive:            true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	sku := productdomain.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           productdomain.DefaultSKUCode,
		PriceAmount:       money.FromDecimal(decimal.NewFromInt(10)),
		IsActive:          true,
		ManualStockTotal:  constants.ManualStockUnlimited,
		ManualStockLocked: 0,
		ManualStockSold:   0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		ProductStore:    productgormstore.NewProductStore(db),
		ProductSKUStore: productgormstore.NewSKUStore(db),
		PromotionRepo:   promotiongormstore.New(db),
		ExpireMinutes:   15,
	})

	_, err = svc.buildOrderResult(orderCreateParams{
		UserID: 1,
		Items: []CreateOrderItem{
			{
				ProductID: product.ID,
				SKUID:     sku.ID,
				Quantity:  fixture.requestQuantity,
			},
		},
	})
	if !errors.Is(err, fixture.expectedErr) {
		t.Fatalf("expected %v, got: %v", fixture.expectedErr, err)
	}
}

func TestBuildOrderResultRejectsZeroPromotionPrice(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_promo_zero_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&categorydomain.Category{}, &productdomain.Product{}, &productdomain.ProductSKU{}, &promotiondomain.Promotion{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := categorydomain.Category{
		Slug:      "test-category",
		NameJSON:  jsonmap.JSON{"zh-CN": "测试分类"},
		SortOrder: 0,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "test-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "测试商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	sku := productdomain.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           productdomain.DefaultSKUCode,
		PriceAmount:       money.FromDecimal(decimal.NewFromInt(10)),
		IsActive:          true,
		ManualStockTotal:  constants.ManualStockUnlimited,
		ManualStockLocked: 0,
		ManualStockSold:   0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	promotion := promotiondomain.Promotion{
		Name:       "test-100-percent",
		ScopeType:  constants.ScopeTypeProduct,
		ScopeRefID: product.ID,
		Type:       constants.PromotionTypePercent,
		Value:      money.FromDecimal(decimal.NewFromInt(100)),
		MinAmount:  money.FromDecimal(decimal.Zero),
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := db.Create(&promotion).Error; err != nil {
		t.Fatalf("create promotion failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		ProductStore:    productgormstore.NewProductStore(db),
		ProductSKUStore: productgormstore.NewSKUStore(db),
		PromotionRepo:   promotiongormstore.New(db),
		ExpireMinutes:   15,
	})

	_, err = svc.buildOrderResult(orderCreateParams{
		UserID: 1,
		Items: []CreateOrderItem{
			{
				ProductID: product.ID,
				SKUID:     sku.ID,
				Quantity:  1,
			},
		},
	})
	if !errors.Is(err, ErrProductPriceInvalid) {
		t.Fatalf("expected product price invalid, got: %v", err)
	}
}

func TestPreviewOrderAppliesMemberDiscountForManualProductBeforeFormCompleted(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_manual_member_preview_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&promotiondomain.Promotion{},
		&userdomain.User{},
		&memberleveldomain.MemberLevel{},
		&memberleveldomain.MemberLevelPrice{},
		&memberleveldomain.UserProductPrice{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := categorydomain.Category{
		Slug:      "manual-member-preview-category",
		NameJSON:  jsonmap.JSON{"zh-CN": "测试分类"},
		SortOrder: 0,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	level := memberleveldomain.MemberLevel{
		NameJSON:     jsonmap.JSON{"zh-CN": "金牌会员"},
		Slug:         "gold",
		DiscountRate: money.FromDecimal(decimal.NewFromInt(80)),
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&level).Error; err != nil {
		t.Fatalf("create member level failed: %v", err)
	}
	user := userdomain.User{
		Email:         "manual-preview@example.com",
		PasswordHash:  "hash",
		Status:        "active",
		MemberLevelID: level.ID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "manual-member-preview-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "人工发货商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		ManualFormSchemaJSON: jsonmap.JSON{
			"fields": []interface{}{
				map[string]interface{}{
					"key":      "account",
					"type":     "text",
					"required": true,
					"label":    map[string]interface{}{"zh-CN": "账号"},
				},
			},
		},
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	sku := productdomain.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           productdomain.DefaultSKUCode,
		PriceAmount:       money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:          true,
		ManualStockTotal:  constants.ManualStockUnlimited,
		ManualStockLocked: 0,
		ManualStockSold:   0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	levelRepo := memberlevelgormstore.NewLevelStore(db)
	priceRepo := memberlevelgormstore.NewPriceStore(db)
	userRepo := userstore.New(db)
	svc := NewOrderService(OrderServiceOptions{
		UserStore:          userRepo,
		ProductStore:       productgormstore.NewProductStore(db),
		ProductSKUStore:    productgormstore.NewSKUStore(db),
		PromotionRepo:      promotiongormstore.New(db),
		MemberLevelService: memberlevelapp.NewService(levelRepo, priceRepo, memberlevelgormstore.NewUserStore(db), memberlevelgormstore.NewUserPriceStore(db)),
		ExpireMinutes:      15,
	})

	preview, err := svc.PreviewOrder(CreateOrderInput{
		UserID: user.ID,
		Items: []CreateOrderItem{
			{
				ProductID: product.ID,
				SKUID:     sku.ID,
				Quantity:  2,
			},
		},
	})
	if err != nil {
		t.Fatalf("preview order failed: %v", err)
	}

	expectedOriginal := decimal.NewFromInt(200)
	expectedMemberDiscount := decimal.NewFromInt(40)
	expectedTotal := decimal.NewFromInt(160)
	if !preview.OriginalAmount.Decimal.Equal(expectedOriginal) {
		t.Fatalf("expected original amount %s, got: %s", expectedOriginal.String(), preview.OriginalAmount.String())
	}
	if !preview.MemberDiscountAmount.Decimal.Equal(expectedMemberDiscount) {
		t.Fatalf("expected member discount amount %s, got: %s", expectedMemberDiscount.String(), preview.MemberDiscountAmount.String())
	}
	if !preview.TotalAmount.Decimal.Equal(expectedTotal) {
		t.Fatalf("expected total amount %s, got: %s", expectedTotal.String(), preview.TotalAmount.String())
	}
}

func TestBuildOrderResultStacksPromotionAndMemberDiscount(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_stack_promo_member_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&promotiondomain.Promotion{},
		&userdomain.User{},
		&memberleveldomain.MemberLevel{},
		&memberleveldomain.MemberLevelPrice{},
		&memberleveldomain.UserProductPrice{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := categorydomain.Category{
		Slug:      "stack-promo-member-category",
		NameJSON:  jsonmap.JSON{"zh-CN": "测试分类"},
		SortOrder: 0,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	level := memberleveldomain.MemberLevel{
		NameJSON:     jsonmap.JSON{"zh-CN": "金牌会员"},
		Slug:         "stack-gold",
		DiscountRate: money.FromDecimal(decimal.NewFromInt(80)),
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&level).Error; err != nil {
		t.Fatalf("create member level failed: %v", err)
	}
	user := userdomain.User{
		Email:         "stack-promo-member@example.com",
		PasswordHash:  "hash",
		Status:        "active",
		MemberLevelID: level.ID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "stack-promo-member-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "叠加优惠商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	promotion := promotiondomain.Promotion{
		Name:       "test-10-percent",
		ScopeType:  constants.ScopeTypeProduct,
		ScopeRefID: product.ID,
		Type:       constants.PromotionTypePercent,
		Value:      money.FromDecimal(decimal.NewFromInt(10)),
		MinAmount:  money.FromDecimal(decimal.Zero),
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := db.Create(&promotion).Error; err != nil {
		t.Fatalf("create promotion failed: %v", err)
	}

	levelRepo := memberlevelgormstore.NewLevelStore(db)
	priceRepo := memberlevelgormstore.NewPriceStore(db)
	userRepo := userstore.New(db)
	svc := NewOrderService(OrderServiceOptions{
		UserStore:          userRepo,
		ProductStore:       productgormstore.NewProductStore(db),
		ProductSKUStore:    productgormstore.NewSKUStore(db),
		PromotionRepo:      promotiongormstore.New(db),
		MemberLevelService: memberlevelapp.NewService(levelRepo, priceRepo, memberlevelgormstore.NewUserStore(db), memberlevelgormstore.NewUserPriceStore(db)),
		ExpireMinutes:      15,
	})

	result, err := svc.buildOrderResult(orderCreateParams{
		UserID: user.ID,
		Items: []CreateOrderItem{
			{
				ProductID: product.ID,
				SKUID:     sku.ID,
				Quantity:  2,
			},
		},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}

	expectedOriginal := decimal.NewFromInt(200)
	expectedPromotion := decimal.NewFromInt(20)
	expectedMemberDiscount := decimal.NewFromInt(36)
	expectedTotal := decimal.NewFromInt(144)
	if !result.OriginalAmount.Equal(expectedOriginal) {
		t.Fatalf("expected original amount %s, got: %s", expectedOriginal.String(), result.OriginalAmount.String())
	}
	if !result.PromotionDiscountAmount.Equal(expectedPromotion) {
		t.Fatalf("expected promotion discount amount %s, got: %s", expectedPromotion.String(), result.PromotionDiscountAmount.String())
	}
	if !result.MemberDiscountAmount.Equal(expectedMemberDiscount) {
		t.Fatalf("expected member discount amount %s, got: %s", expectedMemberDiscount.String(), result.MemberDiscountAmount.String())
	}
	if !result.TotalAmount.Equal(expectedTotal) {
		t.Fatalf("expected total amount %s, got: %s", expectedTotal.String(), result.TotalAmount.String())
	}
	if len(result.Plans) != 1 {
		t.Fatalf("expected one plan, got %d", len(result.Plans))
	}
	item := result.Plans[0].Item
	if item.OriginalUnitPrice.String() != "100.00" {
		t.Fatalf("expected original unit price 100.00, got %s", item.OriginalUnitPrice.String())
	}
	if item.OriginalTotalPrice.String() != "200.00" {
		t.Fatalf("expected original total price 200.00, got %s", item.OriginalTotalPrice.String())
	}
	if item.UnitPrice.String() != "72.00" {
		t.Fatalf("expected final unit price 72.00, got %s", item.UnitPrice.String())
	}
	if item.TotalPrice.String() != "144.00" {
		t.Fatalf("expected final total price 144.00, got %s", item.TotalPrice.String())
	}
}

// 专属折扣价按 SKU 原价打折，再与活动价取低：折扣更低时采纳折扣，
// 并且像一口价一样把活动价差额清零，保持「原价 - 优惠 = 实付」。
func TestBuildOrderResultAppliesUserDiscountRate(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_user_discount_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&promotiondomain.Promotion{},
		&userdomain.User{},
		&memberleveldomain.MemberLevel{},
		&memberleveldomain.MemberLevelPrice{},
		&memberleveldomain.UserProductPrice{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := categorydomain.Category{
		Slug:      "user-discount-category",
		NameJSON:  jsonmap.JSON{"zh-CN": "测试分类"},
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	user := userdomain.User{
		Email:        "user-discount@example.com",
		PasswordHash: "hash",
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "user-discount-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "专属折扣商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	promotion := promotiondomain.Promotion{
		Name:       "user-discount-10-percent",
		ScopeType:  constants.ScopeTypeProduct,
		ScopeRefID: product.ID,
		Type:       constants.PromotionTypePercent,
		Value:      money.FromDecimal(decimal.NewFromInt(10)),
		MinAmount:  money.FromDecimal(decimal.Zero),
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := db.Create(&promotion).Error; err != nil {
		t.Fatalf("create promotion failed: %v", err)
	}
	// 八折专属价：80 < 活动价 90，应当胜出。
	if err := db.Create(&memberleveldomain.UserProductPrice{
		UserID:       user.ID,
		ProductID:    product.ID,
		SKUID:        sku.ID,
		DiscountRate: money.FromDecimal(decimal.NewFromInt(80)),
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("create user discount failed: %v", err)
	}

	levelRepo := memberlevelgormstore.NewLevelStore(db)
	priceRepo := memberlevelgormstore.NewPriceStore(db)
	svc := NewOrderService(OrderServiceOptions{
		UserStore:          userstore.New(db),
		ProductStore:       productgormstore.NewProductStore(db),
		ProductSKUStore:    productgormstore.NewSKUStore(db),
		PromotionRepo:      promotiongormstore.New(db),
		MemberLevelService: memberlevelapp.NewService(levelRepo, priceRepo, memberlevelgormstore.NewUserStore(db), memberlevelgormstore.NewUserPriceStore(db)),
		ExpireMinutes:      15,
	})

	result, err := svc.buildOrderResult(orderCreateParams{
		UserID: user.ID,
		Items:  []CreateOrderItem{{ProductID: product.ID, SKUID: sku.ID, Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}

	if !result.OriginalAmount.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("expected original amount 200, got: %s", result.OriginalAmount.String())
	}
	if !result.PromotionDiscountAmount.IsZero() {
		t.Fatalf("expected promotion discount cleared by exclusive price, got: %s", result.PromotionDiscountAmount.String())
	}
	if !result.MemberDiscountAmount.Equal(decimal.NewFromInt(40)) {
		t.Fatalf("expected member discount amount 40, got: %s", result.MemberDiscountAmount.String())
	}
	if !result.TotalAmount.Equal(decimal.NewFromInt(160)) {
		t.Fatalf("expected total amount 160, got: %s", result.TotalAmount.String())
	}
	if len(result.Plans) != 1 {
		t.Fatalf("expected one plan, got %d", len(result.Plans))
	}
	if got := result.Plans[0].Item.UnitPrice.String(); got != "80.00" {
		t.Fatalf("expected final unit price 80.00, got %s", got)
	}
}

// 折扣价仍高于活动价时，活动价必须继续生效，用户不能因为设了专属折扣而多付。
func TestBuildOrderResultKeepsCheaperPromotionOverUserDiscount(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_user_discount_promo_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&promotiondomain.Promotion{},
		&userdomain.User{},
		&memberleveldomain.MemberLevel{},
		&memberleveldomain.MemberLevelPrice{},
		&memberleveldomain.UserProductPrice{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := categorydomain.Category{
		Slug:      "user-discount-promo-category",
		NameJSON:  jsonmap.JSON{"zh-CN": "测试分类"},
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	user := userdomain.User{
		Email:        "user-discount-promo@example.com",
		PasswordHash: "hash",
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "user-discount-promo-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "活动更低的商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	if err := db.Create(&promotiondomain.Promotion{
		Name:       "user-discount-promo-50-percent",
		ScopeType:  constants.ScopeTypeProduct,
		ScopeRefID: product.ID,
		Type:       constants.PromotionTypePercent,
		Value:      money.FromDecimal(decimal.NewFromInt(50)),
		MinAmount:  money.FromDecimal(decimal.Zero),
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("create promotion failed: %v", err)
	}
	// 95 折专属价：折后 95 高于活动价 50，必须让活动价胜出。
	if err := db.Create(&memberleveldomain.UserProductPrice{
		UserID:       user.ID,
		ProductID:    product.ID,
		SKUID:        sku.ID,
		DiscountRate: money.FromDecimal(decimal.NewFromInt(95)),
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("create user discount failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		UserStore:          userstore.New(db),
		ProductStore:       productgormstore.NewProductStore(db),
		ProductSKUStore:    productgormstore.NewSKUStore(db),
		PromotionRepo:      promotiongormstore.New(db),
		MemberLevelService: memberlevelapp.NewService(memberlevelgormstore.NewLevelStore(db), memberlevelgormstore.NewPriceStore(db), memberlevelgormstore.NewUserStore(db), memberlevelgormstore.NewUserPriceStore(db)),
		ExpireMinutes:      15,
	})

	result, err := svc.buildOrderResult(orderCreateParams{
		UserID: user.ID,
		Items:  []CreateOrderItem{{ProductID: product.ID, SKUID: sku.ID, Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}
	if got := result.Plans[0].Item.UnitPrice.String(); got != "50.00" {
		t.Fatalf("expected cheaper promotion to win, unit price = %s", got)
	}
	if !result.TotalAmount.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("expected total 100, got %s", result.TotalAmount.String())
	}
}

func TestBuildOrderResultRejectsProductMaxPurchaseQuantityExceeded(t *testing.T) {
	assertBuildOrderResultRejectsPurchaseQuantity(t, orderPurchaseQuantityLimitFixture{
		dsnPrefix:       "order_service_purchase_limit",
		categorySlug:    "test-category-limit",
		productSlug:     "test-product-limit",
		maxQuantity:     2,
		requestQuantity: 3,
		expectedErr:     ErrProductMaxPurchaseExceeded,
	})
}

func TestBuildOrderResultRejectsProductMinPurchaseQuantityNotMet(t *testing.T) {
	assertBuildOrderResultRejectsPurchaseQuantity(t, orderPurchaseQuantityLimitFixture{
		dsnPrefix:       "order_service_purchase_min",
		categorySlug:    "test-category-min-limit",
		productSlug:     "test-product-min-limit",
		minQuantity:     3,
		requestQuantity: 2,
		expectedErr:     ErrProductMinPurchaseNotMet,
	})
}

func TestBuildOrderResultOriginalAmountBeforePromotion(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_promo_original_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&categorydomain.Category{}, &productdomain.Product{}, &productdomain.ProductSKU{}, &promotiondomain.Promotion{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := categorydomain.Category{
		Slug:      "test-category-original",
		NameJSON:  jsonmap.JSON{"zh-CN": "测试分类"},
		SortOrder: 0,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "test-product-original",
		TitleJSON:       jsonmap.JSON{"zh-CN": "测试商品"},
		PriceAmount:     money.FromDecimal(decimal.RequireFromString("59.90")),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	sku := productdomain.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           productdomain.DefaultSKUCode,
		PriceAmount:       money.FromDecimal(decimal.RequireFromString("59.90")),
		IsActive:          true,
		ManualStockTotal:  constants.ManualStockUnlimited,
		ManualStockLocked: 0,
		ManualStockSold:   0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	promotion := promotiondomain.Promotion{
		Name:       "test-20-percent",
		ScopeType:  constants.ScopeTypeProduct,
		ScopeRefID: product.ID,
		Type:       constants.PromotionTypePercent,
		Value:      money.FromDecimal(decimal.NewFromInt(20)),
		MinAmount:  money.FromDecimal(decimal.Zero),
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := db.Create(&promotion).Error; err != nil {
		t.Fatalf("create promotion failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		ProductStore:    productgormstore.NewProductStore(db),
		ProductSKUStore: productgormstore.NewSKUStore(db),
		PromotionRepo:   promotiongormstore.New(db),
		ExpireMinutes:   15,
	})

	result, err := svc.buildOrderResult(orderCreateParams{
		UserID: 1,
		Items: []CreateOrderItem{
			{
				ProductID: product.ID,
				SKUID:     sku.ID,
				Quantity:  2,
			},
		},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}

	expectedOriginal := decimal.RequireFromString("119.80")
	expectedPromotion := decimal.RequireFromString("23.96")
	expectedTotal := decimal.RequireFromString("95.84")

	if !result.OriginalAmount.Equal(expectedOriginal) {
		t.Fatalf("expected original amount %s, got: %s", expectedOriginal.String(), result.OriginalAmount.String())
	}
	if !result.PromotionDiscountAmount.Equal(expectedPromotion) {
		t.Fatalf("expected promotion discount amount %s, got: %s", expectedPromotion.String(), result.PromotionDiscountAmount.String())
	}
	if !result.DiscountAmount.Equal(decimal.Zero) {
		t.Fatalf("expected coupon discount amount 0, got: %s", result.DiscountAmount.String())
	}
	if !result.TotalAmount.Equal(expectedTotal) {
		t.Fatalf("expected total amount %s, got: %s", expectedTotal.String(), result.TotalAmount.String())
	}
}

func TestBuildOrderResultRejectsZeroTotalAmountAfterCoupon(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_coupon_zero_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&categorydomain.Category{}, &productdomain.Product{}, &productdomain.ProductSKU{}, &coupondomain.Coupon{}, &coupondomain.CouponUsage{}, &promotiondomain.Promotion{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := categorydomain.Category{
		Slug:      "test-category-coupon",
		NameJSON:  jsonmap.JSON{"zh-CN": "测试分类"},
		SortOrder: 0,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "test-product-coupon",
		TitleJSON:       jsonmap.JSON{"zh-CN": "测试商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	sku := productdomain.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           productdomain.DefaultSKUCode,
		PriceAmount:       money.FromDecimal(decimal.NewFromInt(10)),
		IsActive:          true,
		ManualStockTotal:  constants.ManualStockUnlimited,
		ManualStockLocked: 0,
		ManualStockSold:   0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	coupon := coupondomain.Coupon{
		Code:        "FREE10",
		Type:        constants.CouponTypeFixed,
		Value:       money.FromDecimal(decimal.NewFromInt(10)),
		MinAmount:   money.FromDecimal(decimal.Zero),
		MaxDiscount: money.FromDecimal(decimal.Zero),
		ScopeType:   constants.ScopeTypeProduct,
		ScopeRefIDs: fmt.Sprintf("[%d]", product.ID),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&coupon).Error; err != nil {
		t.Fatalf("create coupon failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		ProductStore:     productgormstore.NewProductStore(db),
		ProductSKUStore:  productgormstore.NewSKUStore(db),
		CouponStore:      coupongormstore.New(db),
		CouponUsageStore: coupongormstore.NewUsageStore(db),
		PromotionRepo:    promotiongormstore.New(db),
		ExpireMinutes:    15,
	})

	_, err = svc.buildOrderResult(orderCreateParams{
		UserID:     1,
		CouponCode: "FREE10",
		Items: []CreateOrderItem{
			{
				ProductID: product.ID,
				SKUID:     sku.ID,
				Quantity:  1,
			},
		},
	})
	if !errors.Is(err, ErrInvalidOrderAmount) {
		t.Fatalf("expected invalid order amount, got: %v", err)
	}
}

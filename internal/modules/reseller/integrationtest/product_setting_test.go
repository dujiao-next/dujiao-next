package integrationtest

import (
	"fmt"
	"testing"
	"time"

	resellergormstore "github.com/dujiao-next/internal/modules/reseller/infrastructure/gormstore"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func openResellerProductSettingServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:reseller_product_setting_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&resellerdomain.Profile{},
		&resellerdomain.ProductSetting{},
	); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return db
}

func seedResellerProductSettingServiceData(t *testing.T, db *gorm.DB) (userdomain.User, resellerdomain.Profile, productdomain.Product, []productdomain.ProductSKU) {
	t.Helper()
	user := userdomain.User{Email: fmt.Sprintf("setting-service-%d@example.test", time.Now().UnixNano()), PasswordHash: "hash", Status: constants.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	profile := resellerdomain.Profile{
		UserID:               user.ID,
		Status:               resellerdomain.ProfileStatusActive,
		DefaultMarkupPercent: money.FromDecimal(decimal.RequireFromString("10.00")),
		MaxMarkupPercent:     money.FromDecimal(decimal.RequireFromString("40.00")),
		SettlementStatus:     resellerdomain.SettlementStatusNormal,
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	category := categorydomain.Category{Slug: "service-category", NameJSON: jsonmap.JSON{"zh-CN": "分类"}, IsActive: true}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "service-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "服务商品", "zh-TW": "服務商品", "en-US": "Service Product"},
		PriceAmount:     money.FromDecimal(decimal.RequireFromString("100.00")),
		CostPriceAmount: money.FromDecimal(decimal.RequireFromString("80.00")),
		IsActive:        true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	skus := []productdomain.ProductSKU{
		{ProductID: product.ID, SKUCode: "A", PriceAmount: money.FromDecimal(decimal.RequireFromString("100.00")), CostPriceAmount: money.FromDecimal(decimal.RequireFromString("80.00")), IsActive: true},
		{ProductID: product.ID, SKUCode: "B", PriceAmount: money.FromDecimal(decimal.RequireFromString("200.00")), CostPriceAmount: money.FromDecimal(decimal.RequireFromString("120.00")), IsActive: true},
	}
	if err := db.Create(&skus).Error; err != nil {
		t.Fatalf("create skus failed: %v", err)
	}
	return user, profile, product, skus
}

func newResellerProductSettingServiceForTest(db *gorm.DB) *ResellerProductSettingService {
	return NewResellerProductSettingService(
		resellergormstore.New(db),
		resellergormstore.New(db),
		productgormstore.NewProductStore(db),
	)
}

func TestResellerProductSettingServiceUserSaveValidatesAndReturnsEffectivePrices(t *testing.T) {
	db := openResellerProductSettingServiceTestDB(t)
	user, _, product, skus := seedResellerProductSettingServiceData(t, db)
	svc := newResellerProductSettingServiceForTest(db)

	detail, err := svc.SaveUserProductSettings(user.ID, product.ID, ResellerProductSettingSaveInput{
		Settings: []ResellerProductSettingInput{
			{SKUID: 0, IsListed: true, PricingMode: resellerdomain.PricingModeMarkupPercent, MarkupPercent: decimal.RequireFromString("20.00")},
			{SKUID: skus[0].ID, IsListed: true, PricingMode: resellerdomain.PricingModeFixedPrice, FixedPriceAmount: decimal.RequireFromString("130.00")},
		},
	})
	if err != nil {
		t.Fatalf("save user settings failed: %v", err)
	}
	if detail == nil || detail.Product.ID != product.ID || len(detail.Settings) != 2 {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	effective := detail.EffectiveBySKUID[skus[0].ID]
	if effective.StringFixed(2) != "130.00" {
		t.Fatalf("sku override effective price want 130.00 got %s", effective.StringFixed(2))
	}
	fallback := detail.EffectiveBySKUID[skus[1].ID]
	if fallback.StringFixed(2) != "240.00" {
		t.Fatalf("product markup effective price want 240.00 got %s", fallback.StringFixed(2))
	}
}

func TestResellerProductSettingServicePersistsFirstHiddenSKUSetting(t *testing.T) {
	db := openResellerProductSettingServiceTestDB(t)
	user, profile, product, skus := seedResellerProductSettingServiceData(t, db)
	svc := newResellerProductSettingServiceForTest(db)

	detail, err := svc.SaveUserProductSettings(user.ID, product.ID, ResellerProductSettingSaveInput{
		Settings: []ResellerProductSettingInput{
			{SKUID: skus[0].ID, IsListed: false, PricingMode: resellerdomain.PricingModeInherit},
		},
	})
	if err != nil {
		t.Fatalf("save hidden sku setting failed: %v", err)
	}
	if detail == nil {
		t.Fatal("expected detail after hidden save")
	}

	var row resellerdomain.ProductSetting
	if err := db.Where("reseller_id = ? AND product_id = ? AND sku_id = ?", profile.ID, product.ID, skus[0].ID).
		First(&row).Error; err != nil {
		t.Fatalf("fetch hidden sku setting failed: %v", err)
	}
	if row.IsListed {
		t.Fatalf("expected first hidden sku save to persist is_listed=false, got true: %+v", row)
	}
}

func TestResellerProductSettingServiceRejectsPriceBelowBase(t *testing.T) {
	db := openResellerProductSettingServiceTestDB(t)
	user, _, product, skus := seedResellerProductSettingServiceData(t, db)
	svc := newResellerProductSettingServiceForTest(db)

	_, err := svc.SaveUserProductSettings(user.ID, product.ID, ResellerProductSettingSaveInput{
		Settings: []ResellerProductSettingInput{
			{SKUID: skus[0].ID, IsListed: true, PricingMode: resellerdomain.PricingModeFixedPrice, FixedPriceAmount: decimal.RequireFromString("99.99")},
		},
	})
	if err != ErrResellerPriceBelowBase {
		t.Fatalf("expected ErrResellerPriceBelowBase, got %v", err)
	}
}

func TestResellerProductSettingServiceRejectsMarkupExceeded(t *testing.T) {
	db := openResellerProductSettingServiceTestDB(t)
	user, _, product, skus := seedResellerProductSettingServiceData(t, db)
	svc := newResellerProductSettingServiceForTest(db)

	_, err := svc.SaveUserProductSettings(user.ID, product.ID, ResellerProductSettingSaveInput{
		Settings: []ResellerProductSettingInput{
			{SKUID: skus[0].ID, IsListed: true, PricingMode: resellerdomain.PricingModeFixedPrice, FixedPriceAmount: decimal.RequireFromString("150.00")},
		},
	})
	if err != ErrResellerMarkupExceeded {
		t.Fatalf("expected ErrResellerMarkupExceeded, got %v", err)
	}
}

func TestResellerProductSettingServiceRejectsBatchWithoutPartialWrites(t *testing.T) {
	db := openResellerProductSettingServiceTestDB(t)
	user, profile, product, skus := seedResellerProductSettingServiceData(t, db)
	svc := newResellerProductSettingServiceForTest(db)

	_, err := svc.SaveUserProductSettings(user.ID, product.ID, ResellerProductSettingSaveInput{
		Settings: []ResellerProductSettingInput{
			{SKUID: skus[0].ID, IsListed: true, PricingMode: resellerdomain.PricingModeFixedPrice, FixedPriceAmount: decimal.RequireFromString("130.00")},
			{SKUID: skus[1].ID, IsListed: true, PricingMode: resellerdomain.PricingModeFixedPrice, FixedPriceAmount: decimal.RequireFromString("99.99")},
		},
	})
	if err != ErrResellerPriceBelowBase {
		t.Fatalf("expected ErrResellerPriceBelowBase, got %v", err)
	}

	var count int64
	if err := db.Model(&resellerdomain.ProductSetting{}).
		Where("reseller_id = ? AND product_id = ?", profile.ID, product.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("count settings failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no partial writes after rejected batch, got %d settings", count)
	}
}

func findPreviewItem(items []ResellerProductSettingPreviewItem, skuID uint) *ResellerProductSettingPreviewItem {
	for i := range items {
		if items[i].SKUID == skuID {
			return &items[i]
		}
	}
	return nil
}

func TestResellerProductSettingServicePreviewMatchesSaveSemantics(t *testing.T) {
	db := openResellerProductSettingServiceTestDB(t)
	user, profile, product, skus := seedResellerProductSettingServiceData(t, db)
	svc := newResellerProductSettingServiceForTest(db)

	items, err := svc.PreviewUserProductSettings(user.ID, product.ID, ResellerProductSettingSaveInput{
		Settings: []ResellerProductSettingInput{
			{SKUID: 0, IsListed: true, PricingMode: resellerdomain.PricingModeMarkupPercent, MarkupPercent: decimal.RequireFromString("20.00")},
			{SKUID: skus[0].ID, IsListed: true, PricingMode: resellerdomain.PricingModeFixedPrice, FixedPriceAmount: decimal.RequireFromString("130.00")},
			{SKUID: skus[1].ID, IsListed: true, PricingMode: resellerdomain.PricingModeInherit},
		},
	})
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}

	skuA := findPreviewItem(items, skus[0].ID)
	if skuA == nil || !skuA.Valid || skuA.EffectivePrice.StringFixed(2) != "130.00" {
		t.Fatalf("sku A preview want valid 130.00, got %+v", skuA)
	}
	skuB := findPreviewItem(items, skus[1].ID)
	if skuB == nil || !skuB.Valid || skuB.EffectivePrice.StringFixed(2) != "240.00" {
		t.Fatalf("sku B inherit preview want valid 240.00, got %+v", skuB)
	}

	// 预览不得落库。
	var count int64
	if err := db.Model(&resellerdomain.ProductSetting{}).Where("reseller_id = ?", profile.ID).Count(&count).Error; err != nil {
		t.Fatalf("count settings failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("preview must not persist settings, found %d rows", count)
	}
}

func TestResellerProductSettingServicePreviewFlagsInvalidRules(t *testing.T) {
	db := openResellerProductSettingServiceTestDB(t)
	user, _, product, skus := seedResellerProductSettingServiceData(t, db)
	svc := newResellerProductSettingServiceForTest(db)

	// 低于基准价 → price_invalid；超出封顶加价 → markup_exceeded。
	items, err := svc.PreviewUserProductSettings(user.ID, product.ID, ResellerProductSettingSaveInput{
		Settings: []ResellerProductSettingInput{
			{SKUID: skus[0].ID, IsListed: true, PricingMode: resellerdomain.PricingModeFixedPrice, FixedPriceAmount: decimal.RequireFromString("99.99")},
			{SKUID: skus[1].ID, IsListed: true, PricingMode: resellerdomain.PricingModeFixedPrice, FixedPriceAmount: decimal.RequireFromString("300.00")},
		},
	})
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	skuA := findPreviewItem(items, skus[0].ID)
	if skuA == nil || skuA.Valid || skuA.ErrorCode != "price_invalid" {
		t.Fatalf("sku A want invalid price_invalid, got %+v", skuA)
	}
	skuB := findPreviewItem(items, skus[1].ID)
	if skuB == nil || skuB.Valid || skuB.ErrorCode != "markup_exceeded" {
		t.Fatalf("sku B want invalid markup_exceeded, got %+v", skuB)
	}
}

func TestResellerProductSettingServiceRequiresActiveProfile(t *testing.T) {
	db := openResellerProductSettingServiceTestDB(t)
	user, profile, product, _ := seedResellerProductSettingServiceData(t, db)
	if err := db.Model(&resellerdomain.Profile{}).Where("id = ?", profile.ID).Update("status", resellerdomain.ProfileStatusPendingReview).Error; err != nil {
		t.Fatalf("update profile failed: %v", err)
	}
	svc := newResellerProductSettingServiceForTest(db)
	_, _, err := svc.ListUserProductSettings(user.ID, ResellerProductSettingUserListInput{Page: 1, PageSize: 20})
	if err != ErrResellerProfileInactive {
		t.Fatalf("expected inactive profile, got %v product=%d", err, product.ID)
	}
}

func TestResellerProductSettingServiceAgencyPriceAndMasterPriceLimit(t *testing.T) {
	db := openResellerProductSettingServiceTestDB(t)
	user, _, _, _ := seedResellerProductSettingServiceData(t, db)
	svc := newResellerProductSettingServiceForTest(db)

	now := time.Now()
	category := categorydomain.Category{Slug: "agency-cat", NameJSON: jsonmap.JSON{"zh-CN": "代理测试"}, IsActive: true}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	// 商品主站售价 100.00，代理价 60.00
	product := productdomain.Product{
		CategoryID:        category.ID,
		Slug:              "agency-product",
		TitleJSON:         jsonmap.JSON{"zh-CN": "代理测试商品"},
		PriceAmount:       money.FromDecimal(decimal.RequireFromString("100.00")),
		AgencyPriceAmount: money.FromDecimal(decimal.RequireFromString("60.00")),
		IsActive:          true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := productdomain.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           "AGENCY-SKU",
		PriceAmount:       money.FromDecimal(decimal.RequireFromString("100.00")),
		AgencyPriceAmount: money.FromDecimal(decimal.RequireFromString("60.00")),
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	// 1. 预览：售价低于主站原售价 100.00（例如设置 80.00），标记为 price_below_master
	preview, err := svc.PreviewUserProductSettings(user.ID, product.ID, ResellerProductSettingSaveInput{
		Settings: []ResellerProductSettingInput{
			{SKUID: sku.ID, IsListed: true, PricingMode: resellerdomain.PricingModeFixedPrice, FixedPriceAmount: decimal.RequireFromString("80.00")},
		},
	})
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	item := findPreviewItem(preview, sku.ID)
	if item == nil || item.Valid || item.ErrorCode != "price_below_master" {
		t.Fatalf("expected price_below_master for price below master, got: %+v", item)
	}
	// 底价必须为代理价 60.00
	if !item.BasePrice.Equal(decimal.RequireFromString("60.00")) {
		t.Fatalf("expected base price 60.00, got: %s", item.BasePrice)
	}

	// 2. 保存：设置 80.00 < 100.00 保存时应直接拒绝报错 ErrResellerPriceBelowMasterPrice
	_, err = svc.SaveUserProductSettings(user.ID, product.ID, ResellerProductSettingSaveInput{
		Settings: []ResellerProductSettingInput{
			{SKUID: sku.ID, IsListed: true, PricingMode: resellerdomain.PricingModeFixedPrice, FixedPriceAmount: decimal.RequireFromString("80.00")},
		},
	})
	if err == nil {
		t.Fatalf("expected error when saving price below master price, got nil")
	}

	// 3. 继承模式：底价为 60.00，生效价继承主站售价 100.00
	detail, err := svc.SaveUserProductSettings(user.ID, product.ID, ResellerProductSettingSaveInput{
		Settings: []ResellerProductSettingInput{
			{SKUID: sku.ID, IsListed: true, PricingMode: resellerdomain.PricingModeInherit},
		},
	})
	if err != nil {
		t.Fatalf("save inherit mode failed: %v", err)
	}
	if effective := detail.EffectiveBySKUID[sku.ID]; !effective.Equal(decimal.RequireFromString("100.00")) {
		t.Fatalf("expected effective price 100.00, got %s", effective)
	}
}

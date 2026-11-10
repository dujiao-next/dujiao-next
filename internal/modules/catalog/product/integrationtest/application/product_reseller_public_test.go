package integrationtest

import (
	"fmt"
	"testing"
	"time"

	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	resellergormstore "github.com/dujiao-next/internal/modules/reseller/infrastructure/gormstore"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	cardsecretgormstore "github.com/dujiao-next/internal/modules/cardsecret/infrastructure/gormstore"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	catalogproductbootstrap "github.com/dujiao-next/internal/bootstrap/catalogproduct"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	cartdomain "github.com/dujiao-next/internal/modules/cart/domain"
	cartgormstore "github.com/dujiao-next/internal/modules/cart/infrastructure/gormstore"
	categorygormstore "github.com/dujiao-next/internal/modules/catalog/category/infrastructure/gormstore"
	mappinggormstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	memberlevelgormstore "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func newProductServiceForResellerPublicTest(t *testing.T) (catalogproductbootstrap.Services, *resellergormstore.Store, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:product_reseller_public_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&cardsecretdomain.Secret{},
		&cardsecretdomain.Batch{},
		&memberleveldomain.MemberLevelPrice{},
		&memberleveldomain.UserProductPrice{},
		&cartdomain.Item{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&paymentdomain.PaymentChannel{},
		&resellerdomain.Profile{},
		&resellerdomain.ProductSetting{},
	); err != nil {
		t.Fatalf("auto migrate product reseller public tables failed: %v", err)
	}

	svc := catalogproductbootstrap.New(catalogproductbootstrap.Dependencies{
		Products:          productgormstore.NewProductStore(db),
		SKUs:              productgormstore.NewSKUStore(db),
		CardSecrets:       cardsecretgormstore.New(db),
		CardSecretBatches: cardsecretgormstore.NewBatch(db),
		Categories:        categorygormstore.NewCategoryStore(db),
		MemberLevelPrices: memberlevelgormstore.NewPriceStore(db),
		UserPrices:        memberlevelgormstore.NewUserPriceStore(db),
		Carts:             cartgormstore.New(db),
		ProductMappings:   mappinggormstore.NewMappingStore(db),
		Orders:            ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"),
		PaymentChannels:   paymentgormstore.NewChannelStore(db),
	})
	return svc, resellergormstore.New(db), db
}

func seedResellerPublicProduct(t *testing.T, db *gorm.DB, categoryID uint, slug string, skuCount int) (productdomain.Product, []productdomain.ProductSKU) {
	t.Helper()
	product := productdomain.Product{
		CategoryID:      categoryID,
		Slug:            slug,
		TitleJSON:       jsonmap.JSON{"zh-CN": slug},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
		SortOrder:       10,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product %s failed: %v", slug, err)
	}
	skus := make([]productdomain.ProductSKU, 0, skuCount)
	for i := 0; i < skuCount; i++ {
		sku := productdomain.ProductSKU{
			ProductID:        product.ID,
			SKUCode:          fmt.Sprintf("%s-sku-%d", slug, i+1),
			PriceAmount:      money.FromDecimal(decimal.NewFromInt(int64(100 + i))),
			CostPriceAmount:  money.FromDecimal(decimal.NewFromInt(50)),
			ManualStockTotal: constants.ManualStockUnlimited,
			IsActive:         true,
			SortOrder:        skuCount - i,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		if err := db.Create(&sku).Error; err != nil {
			t.Fatalf("create sku %s failed: %v", sku.SKUCode, err)
		}
		skus = append(skus, sku)
	}
	return product, skus
}

func createResellerPublicSetting(t *testing.T, db *gorm.DB, setting resellerdomain.ProductSetting) resellerdomain.ProductSetting {
	t.Helper()
	wantListed := setting.IsListed
	if err := db.Create(&setting).Error; err != nil {
		t.Fatalf("create reseller product setting failed: %v", err)
	}
	if !wantListed {
		if err := db.Model(&resellerdomain.ProductSetting{}).
			Where("id = ?", setting.ID).
			Update("is_listed", false).Error; err != nil {
			t.Fatalf("force reseller product setting hidden failed: %v", err)
		}
		setting.IsListed = false
	}
	return setting
}

func TestProductServiceListPublicForTenantExcludesResellerHiddenProductsBeforePagination(t *testing.T) {
	svc, resellerRepo, db := newProductServiceForResellerPublicTest(t)
	category := categorydomain.Category{Slug: "reseller-public", NameJSON: jsonmap.JSON{"zh-CN": "reseller-public"}, IsActive: true}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	owner := userdomain.User{Email: "reseller-public-owner@example.com", PasswordHash: "hash"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner failed: %v", err)
	}
	profile := resellerdomain.Profile{UserID: owner.ID, Status: resellerdomain.ProfileStatusActive}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}

	visible, _ := seedResellerPublicProduct(t, db, category.ID, "visible", 1)
	productHidden, _ := seedResellerPublicProduct(t, db, category.ID, "product-hidden", 1)
	allSKUHidden, allHiddenSKUs := seedResellerPublicProduct(t, db, category.ID, "all-sku-hidden", 2)
	partialSKUHidden, partialHiddenSKUs := seedResellerPublicProduct(t, db, category.ID, "partial-sku-hidden", 2)

	createResellerPublicSetting(t, db, resellerdomain.ProductSetting{ResellerID: profile.ID, ProductID: productHidden.ID, SKUID: 0, IsListed: false, PricingMode: resellerdomain.PricingModeInherit})
	for _, sku := range allHiddenSKUs {
		createResellerPublicSetting(t, db, resellerdomain.ProductSetting{ResellerID: profile.ID, ProductID: allSKUHidden.ID, SKUID: sku.ID, IsListed: false, PricingMode: resellerdomain.PricingModeInherit})
	}
	createResellerPublicSetting(t, db, resellerdomain.ProductSetting{ResellerID: profile.ID, ProductID: partialSKUHidden.ID, SKUID: partialHiddenSKUs[0].ID, IsListed: false, PricingMode: resellerdomain.PricingModeInherit})

	tenant := reseller.ResellerTenantContext("shop.example.test", profile.ID, owner.ID, "shop.example.test")
	rows, total, err := svc.Read.ListPublicForTenant(tenant, resellerRepo, "", "", 1, 20)
	if err != nil {
		t.Fatalf("ListPublicForTenant failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total=2 after excluding hidden products before pagination, got %d", total)
	}
	got := map[uint]bool{}
	for _, product := range rows {
		got[product.ID] = true
	}
	if !got[visible.ID] || !got[partialSKUHidden.ID] || got[productHidden.ID] || got[allSKUHidden.ID] {
		t.Fatalf("unexpected products after reseller filtering: %+v", got)
	}
}

func TestProductServiceGetPublicBySlugForTenantRejectsHiddenProduct(t *testing.T) {
	svc, resellerRepo, db := newProductServiceForResellerPublicTest(t)
	category := categorydomain.Category{Slug: "reseller-detail", NameJSON: jsonmap.JSON{"zh-CN": "reseller-detail"}, IsActive: true}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	owner := userdomain.User{Email: "reseller-detail-owner@example.com", PasswordHash: "hash"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner failed: %v", err)
	}
	profile := resellerdomain.Profile{UserID: owner.ID, Status: resellerdomain.ProfileStatusActive}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	hidden, _ := seedResellerPublicProduct(t, db, category.ID, "hidden-detail", 1)
	createResellerPublicSetting(t, db, resellerdomain.ProductSetting{ResellerID: profile.ID, ProductID: hidden.ID, SKUID: 0, IsListed: false, PricingMode: resellerdomain.PricingModeInherit})

	tenant := reseller.ResellerTenantContext("shop.example.test", profile.ID, owner.ID, "shop.example.test")
	_, err := svc.Read.GetPublicBySlugForTenant(tenant, resellerRepo, hidden.Slug)
	if err != productcontract.ErrNotFound {
		t.Fatalf("expected productcontract.ErrNotFound for hidden detail, got %v", err)
	}
}

package integrationtest

import (
	"fmt"
	"testing"
	"time"

	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	cardsecretgormstore "github.com/dujiao-next/internal/modules/cardsecret/infrastructure/gormstore"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	catalogproductbootstrap "github.com/dujiao-next/internal/bootstrap/catalogproduct"
	cartdomain "github.com/dujiao-next/internal/modules/cart/domain"
	cartgormstore "github.com/dujiao-next/internal/modules/cart/infrastructure/gormstore"
	categorygormstore "github.com/dujiao-next/internal/modules/catalog/category/infrastructure/gormstore"
	mappinggormstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	memberlevelgormstore "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newAutoStockProductService(t *testing.T) (catalogproductbootstrap.Services, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:product_auto_stock_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&cardsecretdomain.Secret{}); err != nil {
		t.Fatalf("auto migrate card secret failed: %v", err)
	}
	secretRepo := cardsecretgormstore.New(db)
	return catalogproductbootstrap.New(catalogproductbootstrap.Dependencies{CardSecrets: secretRepo}), db
}

func insertCardSecrets(t *testing.T, db *gorm.DB, productID, skuID uint, status string, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		row := cardsecretdomain.Secret{
			ProductID: productID,
			SKUID:     skuID,
			Secret:    fmt.Sprintf("secret-%d-%d-%s-%d", productID, skuID, status, i),
			Status:    status,
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create card secret failed: %v", err)
		}
	}
}

func newProductServiceForTest(t *testing.T) (catalogproductbootstrap.Services, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:product_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&categorydomain.Category{}, &productdomain.Product{}, &productdomain.ProductSKU{}, &cardsecretdomain.Secret{}, &cardsecretdomain.Batch{}, &memberleveldomain.MemberLevelPrice{}, &memberleveldomain.UserProductPrice{}, &cartdomain.Item{}, &mappingdomain.Mapping{}, &mappingdomain.SKUMapping{}, &orderdomain.Order{}, &orderdomain.OrderItem{}, &paymentdomain.PaymentChannel{}); err != nil {
		t.Fatalf("auto migrate product service tables failed: %v", err)
	}

	return catalogproductbootstrap.New(catalogproductbootstrap.Dependencies{
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
	}), db
}

func createProductTestPaymentChannel(t *testing.T, db *gorm.DB, name string, active bool, deleted bool) paymentdomain.PaymentChannel {
	t.Helper()

	channel := paymentdomain.PaymentChannel{
		Name:            name,
		ProviderType:    "official",
		ChannelType:     "wechat",
		InteractionMode: "qr",
		IsActive:        active,
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create payment channel failed: %v", err)
	}
	if !active {
		if err := db.Model(&channel).Update("is_active", false).Error; err != nil {
			t.Fatalf("disable payment channel failed: %v", err)
		}
		channel.IsActive = false
	}
	if deleted {
		if err := db.Delete(&channel).Error; err != nil {
			t.Fatalf("delete payment channel failed: %v", err)
		}
	}
	return channel
}

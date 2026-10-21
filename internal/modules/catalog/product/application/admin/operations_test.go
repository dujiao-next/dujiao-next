package productadmin

import (
	"errors"
	"testing"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/shopspring/decimal"
)

func TestAdminServiceDeleteStopsBeforeTransactionWhenStockExists(t *testing.T) {
	transactions := &unitOfWorkStub{}
	service := NewAdminService(Options{
		Products:     &productRepositoryStub{product: &productdomain.Product{ID: 7}},
		CardSecrets:  stockRepositoryStub{available: 1},
		Orders:       orderRepositoryStub{},
		Transactions: transactions,
	})

	if err := service.Delete("7"); !errors.Is(err, productcontract.ErrProductHasStock) {
		t.Fatalf("expected product stock error, got %v", err)
	}
	if transactions.called {
		t.Fatal("delete transaction must not start while card-secret stock exists")
	}
}

func TestAdminServiceDeleteUsesAllCascadePorts(t *testing.T) {
	deleted := make(map[string]int)
	repositories := DeleteRepositories{
		Products:          productDeleteStub{deleted: deleted},
		CardSecrets:       productRelationDeleteStub{name: "card_secrets", deleted: deleted},
		CardSecretBatches: productRelationDeleteStub{name: "card_secret_batches", deleted: deleted},
		SKUs:              productRelationDeleteStub{name: "skus", deleted: deleted},
		MemberLevelPrices: productRelationDeleteStub{name: "member_level_prices", deleted: deleted},
		Carts:             productRelationDeleteStub{name: "carts", deleted: deleted},
		ProductMappings:   productMappingDeleteStub{deleted: deleted},
	}
	transactions := &unitOfWorkStub{repositories: repositories}
	service := NewAdminService(Options{
		Products:     &productRepositoryStub{product: &productdomain.Product{ID: 9}},
		CardSecrets:  stockRepositoryStub{},
		Orders:       orderRepositoryStub{},
		Transactions: transactions,
	})

	if err := service.Delete("9"); err != nil {
		t.Fatalf("delete product failed: %v", err)
	}
	if !transactions.called {
		t.Fatal("expected cascade to run inside the unit of work")
	}
	for _, name := range []string{"product", "card_secrets", "card_secret_batches", "skus", "member_level_prices", "carts", "product_mappings"} {
		if deleted[name] != 1 {
			t.Errorf("expected %s delete once, got %d", name, deleted[name])
		}
	}
}

func TestAdminServiceQuickUpdateValidatesActivationCategory(t *testing.T) {
	products := &productRepositoryStub{product: &productdomain.Product{ID: 11, CategoryID: 3}}
	service := NewAdminService(Options{
		Products:   products,
		Categories: categoryRepositoryStub{categories: map[string]*categorydomain.Category{"5": {ID: 5, IsActive: true}}},
	})

	updated, err := service.QuickUpdate("11", map[string]interface{}{
		"category_id": float64(5),
		"is_active":   true,
	})
	if err != nil {
		t.Fatalf("quick update failed: %v", err)
	}
	if updated == nil || products.quickUpdates != 1 {
		t.Fatalf("expected one persisted quick update, got product=%+v updates=%d", updated, products.quickUpdates)
	}

	_, err = service.QuickUpdate("11", map[string]interface{}{
		"category_id": 5.5,
		"is_active":   true,
	})
	if !errors.Is(err, productcontract.ErrProductCategoryInvalid) {
		t.Fatalf("expected invalid category error, got %v", err)
	}
	if products.quickUpdates != 1 {
		t.Fatalf("invalid category must not be persisted, got %d updates", products.quickUpdates)
	}
}

func TestAdminServiceQuickUpdateAllowsMappedProductWithoutCategory(t *testing.T) {
	products := &productRepositoryStub{product: &productdomain.Product{
		ID:       12,
		IsMapped: true,
	}}
	service := NewAdminService(Options{Products: products})

	if _, err := service.QuickUpdate("12", map[string]interface{}{"is_active": true}); err != nil {
		t.Fatalf("mapped upstream product without category should activate: %v", err)
	}
	if products.quickUpdates != 1 {
		t.Fatalf("expected one persisted quick update, got %d", products.quickUpdates)
	}
}

func TestAdminServiceQuickUpdateStillRejectsManualProductWithoutCategory(t *testing.T) {
	products := &productRepositoryStub{product: &productdomain.Product{ID: 13}}
	service := NewAdminService(Options{Products: products})

	if _, err := service.QuickUpdate("13", map[string]interface{}{"is_active": true}); !errors.Is(err, productcontract.ErrProductCategoryInvalid) {
		t.Fatalf("expected invalid category error, got %v", err)
	}
	if products.quickUpdates != 0 {
		t.Fatalf("manual product must not be activated without category, got %d updates", products.quickUpdates)
	}
}

func TestAdminServiceUpdateWholesalePricesCanonicalizesSKU(t *testing.T) {
	products := &productRepositoryStub{adminProduct: &productdomain.Product{
		ID: 13,
		SKUs: []productdomain.ProductSKU{{
			ID:       21,
			SKUCode:  "SKU-A",
			IsActive: true,
		}},
	}}
	service := NewAdminService(Options{Products: products})

	updated, err := service.UpdateWholesalePrices("13", []productdomain.WholesalePriceInput{{
		SKUCode:     "SKU-A",
		MinQuantity: 5,
		UnitPrice:   decimal.NewFromInt(80),
	}})
	if err != nil {
		t.Fatalf("update wholesale prices failed: %v", err)
	}
	if updated == nil || products.quickUpdates != 1 {
		t.Fatalf("expected one persisted wholesale update, got product=%+v updates=%d", updated, products.quickUpdates)
	}
	tiers, ok := products.lastFields["wholesale_prices"].(productdomain.WholesalePriceTiers)
	if !ok || len(tiers) != 1 {
		t.Fatalf("expected one normalized wholesale tier, got %#v", products.lastFields["wholesale_prices"])
	}
	if tiers[0].SKUID != 21 || tiers[0].SKUCode != "SKU-A" {
		t.Fatalf("expected canonical SKU identity, got %+v", tiers[0])
	}
	if len(products.lastFields) != 1 {
		t.Fatalf("wholesale update must only touch one field, got %+v", products.lastFields)
	}
}

type productRepositoryStub struct {
	product      *productdomain.Product
	adminProduct *productdomain.Product
	quickUpdates int
	lastFields   map[string]interface{}
}

func (repository *productRepositoryStub) GetByID(string) (*productdomain.Product, error) {
	return repository.product, nil
}

func (repository *productRepositoryStub) GetAdminByID(string) (*productdomain.Product, error) {
	return repository.adminProduct, nil
}

func (repository *productRepositoryStub) QuickUpdate(_ string, fields map[string]interface{}) error {
	repository.quickUpdates++
	repository.lastFields = fields
	if tiers, ok := fields["wholesale_prices"].(productdomain.WholesalePriceTiers); ok && repository.adminProduct != nil {
		repository.adminProduct.WholesalePrices = tiers
	}
	return nil
}

type categoryRepositoryStub struct {
	categories map[string]*categorydomain.Category
	children   map[string]int64
}

func (repository categoryRepositoryStub) GetByID(id string) (*categorydomain.Category, error) {
	return repository.categories[id], nil
}

func (repository categoryRepositoryStub) CountChildren(id string) (int64, error) {
	return repository.children[id], nil
}

type stockRepositoryStub struct {
	available int64
	reserved  int64
}

func (repository stockRepositoryStub) CountAvailable(uint, uint) (int64, error) {
	return repository.available, nil
}

func (repository stockRepositoryStub) CountReserved(uint, uint) (int64, error) {
	return repository.reserved, nil
}

type orderRepositoryStub struct {
	count int64
}

func (repository orderRepositoryStub) CountOrderItemsByProduct(uint) (int64, error) {
	return repository.count, nil
}

type unitOfWorkStub struct {
	repositories DeleteRepositories
	called       bool
}

func (unit *unitOfWorkStub) WithinTransaction(fn func(DeleteRepositories) error) error {
	unit.called = true
	return fn(unit.repositories)
}

type productDeleteStub struct {
	deleted map[string]int
}

func (repository productDeleteStub) Delete(string) error {
	repository.deleted["product"]++
	return nil
}

type productRelationDeleteStub struct {
	name    string
	deleted map[string]int
}

func (repository productRelationDeleteStub) DeleteByProduct(uint) error {
	repository.deleted[repository.name]++
	return nil
}

type productMappingDeleteStub struct {
	deleted map[string]int
}

func (repository productMappingDeleteStub) DeleteByLocalProduct(uint) error {
	repository.deleted["product_mappings"]++
	return nil
}

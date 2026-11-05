package productdomain

import (
	"errors"
	"testing"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/constants"
)

func TestValidatePurchaseQuantityKeepsLimitSemantics(t *testing.T) {
	product := &Product{MinPurchaseQuantity: 2, MaxPurchaseQuantity: 5}
	tests := []struct {
		name     string
		quantity int
		wantErr  error
	}{
		{name: "non-positive", quantity: 0, wantErr: ErrPurchaseQuantityInvalid},
		{name: "below minimum", quantity: 1, wantErr: ErrMinPurchaseNotMet},
		{name: "minimum boundary", quantity: 2},
		{name: "maximum boundary", quantity: 5},
		{name: "above maximum", quantity: 6, wantErr: ErrMaxPurchaseExceeded},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidatePurchaseQuantity(product, test.quantity)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ValidatePurchaseQuantity(%d) want %v got %v", test.quantity, test.wantErr, err)
			}
		})
	}
}

func TestNormalizePurchaseQuantityLimitDisablesNonPositiveValues(t *testing.T) {
	for _, value := range []int{-1, 0} {
		if got := NormalizePurchaseQuantityLimit(value); got != 0 {
			t.Fatalf("NormalizePurchaseQuantityLimit(%d) want 0 got %d", value, got)
		}
	}
	if got := NormalizePurchaseQuantityLimit(3); got != 3 {
		t.Fatalf("NormalizePurchaseQuantityLimit(3) want 3 got %d", got)
	}
}

func TestManualSKUStockPolicyPreservesLegacyDefaultFallback(t *testing.T) {
	product := &Product{SKUs: []ProductSKU{
		{SKUCode: DefaultSKUCode, IsActive: true},
		{SKUCode: "SECOND", IsActive: true},
	}}
	legacyDefault := &ProductSKU{SKUCode: DefaultSKUCode, ManualStockTotal: -2}
	if !ShouldEnforceManualSKUStock(product, legacyDefault) {
		t.Fatal("legacy DEFAULT SKU must enforce stock when multiple active SKUs exist")
	}
	if got := ManualSKUAvailable(legacyDefault); got != 0 {
		t.Fatalf("legacy negative stock want 0 available got %d", got)
	}

	unlimited := &ProductSKU{SKUCode: "SECOND", ManualStockTotal: constants.ManualStockUnlimited}
	if ShouldEnforceManualSKUStock(product, unlimited) {
		t.Fatal("unlimited SKU must not enforce manual stock")
	}
	if got := ManualSKUAvailable(unlimited); got <= 0 {
		t.Fatalf("unlimited SKU must expose a positive sentinel, got %d", got)
	}
}

func TestProductConfigurationNormalizersKeepDefaultsAndRejectUnknownValues(t *testing.T) {
	if got := NormalizePurchaseType(""); got != constants.ProductPurchaseMember {
		t.Fatalf("empty purchase type want %q got %q", constants.ProductPurchaseMember, got)
	}
	if got := NormalizePurchaseType(constants.ProductPurchaseGuest); got != constants.ProductPurchaseGuest {
		t.Fatalf("guest purchase type want %q got %q", constants.ProductPurchaseGuest, got)
	}
	if got := NormalizePurchaseType("invalid"); got != "" {
		t.Fatalf("invalid purchase type must be rejected, got %q", got)
	}

	if got := NormalizeFulfillmentType(""); got != constants.FulfillmentTypeManual {
		t.Fatalf("empty fulfillment type want %q got %q", constants.FulfillmentTypeManual, got)
	}
	if got := NormalizeFulfillmentType(constants.FulfillmentTypeUpstream); got != constants.FulfillmentTypeUpstream {
		t.Fatalf("upstream fulfillment type want %q got %q", constants.FulfillmentTypeUpstream, got)
	}
	if got := NormalizeFulfillmentType("invalid"); got != "" {
		t.Fatalf("invalid fulfillment type must be rejected, got %q", got)
	}

	if got := NormalizeStockDisplayMode(""); got != constants.ProductStockDisplayExact {
		t.Fatalf("empty stock display mode want %q got %q", constants.ProductStockDisplayExact, got)
	}
	if got := NormalizeStockDisplayMode(constants.ProductStockDisplayHidden); got != constants.ProductStockDisplayHidden {
		t.Fatalf("hidden stock mode want %q got %q", constants.ProductStockDisplayHidden, got)
	}
	if got := NormalizeStockDisplayMode("invalid"); got != "" {
		t.Fatalf("invalid stock display mode must be rejected, got %q", got)
	}
}

type categoryAssignmentRepositoryStub struct {
	category *categorydomain.Category
	children int64
}

func (stub categoryAssignmentRepositoryStub) GetByID(string) (*categorydomain.Category, error) {
	return stub.category, nil
}

func (stub categoryAssignmentRepositoryStub) CountChildren(string) (int64, error) {
	return stub.children, nil
}

func TestValidateCategoryAssignmentPreservesCompatibilityError(t *testing.T) {
	compatibilityError := errors.New("legacy category invalid")
	repository := categoryAssignmentRepositoryStub{
		category: &categorydomain.Category{ID: 10},
		children: 1,
	}

	if err := ValidateCategoryAssignment(repository, 10, 0, compatibilityError); !errors.Is(err, compatibilityError) {
		t.Fatalf("new parent assignment want compatibility error, got %v", err)
	}
	if err := ValidateCategoryAssignment(repository, 10, 10, compatibilityError); err != nil {
		t.Fatalf("keeping current parent assignment must remain valid, got %v", err)
	}
}

func TestPruneWholesalePricesForSKUs(t *testing.T) {
	tiers := WholesalePriceTiers{
		{SKUID: 7, SKUCode: "A", MinQuantity: 10}, // 编码被改为 A2：保留并刷新编码
		{SKUID: 8, SKUCode: "B", MinQuantity: 10}, // SKU 已删除：丢弃
		{SKUCode: "legacy", MinQuantity: 10},      // 仅按编码引用且编码已不存在：丢弃
		{SKUCode: "c", MinQuantity: 10},           // 仅按编码引用且编码存在（大小写不敏感）：保留
		{MinQuantity: 20},                         // 商品级阶梯：保留
	}
	skus := []ProductSKU{{ID: 7, SKUCode: "A2"}, {ID: 9, SKUCode: "C"}}

	got := PruneWholesalePricesForSKUs(tiers, skus)
	if len(got) != 3 {
		t.Fatalf("expected 3 tiers kept, got %d: %+v", len(got), got)
	}
	if got[0].SKUID != 7 || got[0].SKUCode != "A2" {
		t.Fatalf("expected renamed tier refreshed to A2, got %+v", got[0])
	}
	if got[1].SKUCode != "c" || got[2].SKUID != 0 || got[2].SKUCode != "" {
		t.Fatalf("unexpected kept tiers: %+v", got)
	}
}

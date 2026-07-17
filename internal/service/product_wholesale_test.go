package service

import (
	"errors"
	"testing"

	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/upstream"

	"github.com/shopspring/decimal"
)

func TestNormalizeWholesalePriceInputsSortsTiers(t *testing.T) {
	tiers, err := normalizeWholesalePriceInputs([]WholesalePriceInput{
		{MinQuantity: 10, UnitPrice: decimal.NewFromInt(70)},
		{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
	})
	if err != nil {
		t.Fatalf("normalizeWholesalePriceInputs returned error: %v", err)
	}
	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %d", len(tiers))
	}
	if tiers[0].MinQuantity != 5 || tiers[0].UnitPrice.String() != "80.00" {
		t.Fatalf("unexpected first tier: %+v", tiers[0])
	}
	if tiers[1].MinQuantity != 10 || tiers[1].UnitPrice.String() != "70.00" {
		t.Fatalf("unexpected second tier: %+v", tiers[1])
	}
}

func TestNormalizeWholesalePriceInputsRejectsInvalidTiers(t *testing.T) {
	tests := []struct {
		name   string
		inputs []WholesalePriceInput
	}{
		{
			name:   "zero quantity",
			inputs: []WholesalePriceInput{{MinQuantity: 0, UnitPrice: decimal.NewFromInt(80)}},
		},
		{
			name:   "zero price",
			inputs: []WholesalePriceInput{{MinQuantity: 5, UnitPrice: decimal.Zero}},
		},
		{
			name: "duplicate quantity",
			inputs: []WholesalePriceInput{
				{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
				{MinQuantity: 5, UnitPrice: decimal.NewFromInt(70)},
			},
		},
		{
			name: "higher tier more expensive",
			inputs: []WholesalePriceInput{
				{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
				{MinQuantity: 10, UnitPrice: decimal.NewFromInt(90)},
			},
		},
		{
			name: "higher tier equal price",
			inputs: []WholesalePriceInput{
				{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
				{MinQuantity: 10, UnitPrice: decimal.NewFromInt(80)},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeWholesalePriceInputs(tc.inputs)
			if !errors.Is(err, ErrWholesalePriceInvalid) {
				t.Fatalf("expected ErrWholesalePriceInvalid, got %v", err)
			}
		})
	}
}

func TestNormalizeWholesalePriceInputsRejectsDuplicateCanonicalSKUScope(t *testing.T) {
	_, err := normalizeWholesalePriceInputs([]WholesalePriceInput{
		{SKUID: 5, SKUCode: "SKU-A", MinQuantity: 10, UnitPrice: decimal.NewFromInt(80)},
		{SKUCode: "SKU-A", MinQuantity: 10, UnitPrice: decimal.NewFromInt(70)},
	})
	if !errors.Is(err, ErrWholesalePriceInvalid) {
		t.Fatalf("expected ErrWholesalePriceInvalid, got %v", err)
	}
}

func TestNormalizeWholesalePriceInputsRejectsNonDecreasingCanonicalSKUScope(t *testing.T) {
	_, err := normalizeWholesalePriceInputs([]WholesalePriceInput{
		{SKUID: 5, SKUCode: "SKU-A", MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
		{SKUCode: "SKU-A", MinQuantity: 10, UnitPrice: decimal.NewFromInt(90)},
	})
	if !errors.Is(err, ErrWholesalePriceInvalid) {
		t.Fatalf("expected ErrWholesalePriceInvalid, got %v", err)
	}
}

func TestNormalizeWholesalePriceInputsAllowsSameQuantityForDifferentSKUs(t *testing.T) {
	tiers, err := normalizeWholesalePriceInputs([]WholesalePriceInput{
		{SKUID: 2, SKUCode: "SKU-B", MinQuantity: 5, UnitPrice: decimal.NewFromInt(70)},
		{SKUID: 1, SKUCode: "SKU-A", MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
		{MinQuantity: 5, UnitPrice: decimal.NewFromInt(90)},
	})
	if err != nil {
		t.Fatalf("normalizeWholesalePriceInputs returned error: %v", err)
	}
	if len(tiers) != 3 {
		t.Fatalf("expected 3 tiers, got %d", len(tiers))
	}
	if tiers[0].SKUID != 0 || tiers[0].MinQuantity != 5 {
		t.Fatalf("expected universal tier first, got %+v", tiers[0])
	}
	if tiers[1].SKUID != 1 || tiers[1].SKUCode != "SKU-A" {
		t.Fatalf("expected SKU-A tier second, got %+v", tiers[1])
	}
	if tiers[2].SKUID != 2 || tiers[2].SKUCode != "SKU-B" {
		t.Fatalf("expected SKU-B tier third, got %+v", tiers[2])
	}
}

func TestConvertUpstreamWholesalePricesRemapsUpstreamSKUScope(t *testing.T) {
	tiers := convertUpstreamWholesalePrices(models.WholesalePriceTiers{
		{SKUID: 201, MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
	}, decimal.NewFromInt(1), decimal.Zero, "none", buildUpstreamWholesaleSKUIndex(
		[]models.ProductSKU{{ID: 11, SKUCode: "SKU-A"}},
		[]upstream.UpstreamSKU{{ID: 201, SKUCode: "SKU-A"}},
		nil,
	))

	if len(tiers) != 1 {
		t.Fatalf("expected 1 tier, got %d", len(tiers))
	}
	if tiers[0].SKUID != 11 || tiers[0].SKUCode != "SKU-A" {
		t.Fatalf("expected upstream SKU scope to be remapped, got %+v", tiers[0])
	}
}

func TestConvertUpstreamWholesalePricesDropsUnmappedUpstreamSKUID(t *testing.T) {
	tiers := convertUpstreamWholesalePrices(models.WholesalePriceTiers{
		{SKUID: 201, MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
	}, decimal.NewFromInt(1), decimal.Zero, "none")

	if len(tiers) != 0 {
		t.Fatalf("expected unmapped upstream sku_id tier to be dropped, got %+v", tiers)
	}
}

func TestResolveWholesaleUnitPriceMatchesBestTier(t *testing.T) {
	product := &models.Product{
		WholesalePrices: models.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
			{MinQuantity: 10, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(70))},
		},
	}

	unitPrice, discount, matched := ResolveWholesaleUnitPrice(product, decimal.NewFromInt(100), 12)
	if !matched {
		t.Fatalf("expected wholesale tier to match")
	}
	if !unitPrice.Equal(decimal.NewFromInt(70)) {
		t.Fatalf("expected unit price 70, got %s", unitPrice.String())
	}
	if !discount.Equal(decimal.NewFromInt(360)) {
		t.Fatalf("expected discount 360, got %s", discount.String())
	}
}

func TestResolveWholesaleUnitPriceDoesNotMatchBelowQuantity(t *testing.T) {
	product := &models.Product{
		WholesalePrices: models.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
		},
	}

	unitPrice, discount, matched := ResolveWholesaleUnitPrice(product, decimal.NewFromInt(100), 4)
	if matched {
		t.Fatalf("expected no wholesale tier to match")
	}
	if !unitPrice.Equal(decimal.NewFromInt(100)) || !discount.IsZero() {
		t.Fatalf("unexpected price result: unit=%s discount=%s", unitPrice.String(), discount.String())
	}
}

func TestResolveWholesaleUnitPriceForSKUPrefersSpecificTier(t *testing.T) {
	product := &models.Product{
		WholesalePrices: models.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
			{SKUID: 11, SKUCode: "SKU-A", MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(70))},
		},
	}

	unitPrice, discount, matched := ResolveWholesaleUnitPriceForSKU(product, decimal.NewFromInt(100), 11, "SKU-A", 12, 6)
	if !matched {
		t.Fatalf("expected SKU specific wholesale tier to match")
	}
	if !unitPrice.Equal(decimal.NewFromInt(70)) {
		t.Fatalf("expected unit price 70, got %s", unitPrice.String())
	}
	if !discount.Equal(decimal.NewFromInt(180)) {
		t.Fatalf("expected discount 180, got %s", discount.String())
	}
}

func TestResolveWholesaleUnitPriceForSKURequiresIDAndCodeToMatch(t *testing.T) {
	product := &models.Product{
		WholesalePrices: models.WholesalePriceTiers{
			{SKUID: 11, SKUCode: "SKU-A", MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(70))},
		},
	}

	if _, _, matched := ResolveWholesaleUnitPriceForSKU(product, decimal.NewFromInt(100), 11, "SKU-B", 6, 6); matched {
		t.Fatalf("expected no match when sku_id matches but sku_code differs")
	}
	if _, _, matched := ResolveWholesaleUnitPriceForSKU(product, decimal.NewFromInt(100), 12, "SKU-A", 6, 6); matched {
		t.Fatalf("expected no match when sku_code matches but sku_id differs")
	}
}

func TestResolveWholesaleUnitPriceForSKUDoesNotFallbackWhenSpecificTierExists(t *testing.T) {
	product := &models.Product{
		WholesalePrices: models.WholesalePriceTiers{
			{MinQuantity: 10, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
			{SKUID: 11, SKUCode: "SKU-A", MinQuantity: 10, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(70))},
		},
	}

	unitPrice, discount, matched := ResolveWholesaleUnitPriceForSKU(product, decimal.NewFromInt(100), 11, "SKU-A", 12, 6)
	if matched {
		t.Fatalf("expected no match because SKU specific threshold uses current SKU quantity")
	}
	if !unitPrice.Equal(decimal.NewFromInt(100)) || !discount.IsZero() {
		t.Fatalf("unexpected price result: unit=%s discount=%s", unitPrice.String(), discount.String())
	}
}

func TestResolveWholesaleUnitPriceForSKUUsesProductQuantityForUniversalTier(t *testing.T) {
	product := &models.Product{
		WholesalePrices: models.WholesalePriceTiers{
			{MinQuantity: 10, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
		},
	}

	unitPrice, discount, matched := ResolveWholesaleUnitPriceForSKU(product, decimal.NewFromInt(100), 12, "SKU-B", 12, 6)
	if !matched {
		t.Fatalf("expected universal wholesale tier to match by product quantity")
	}
	if !unitPrice.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("expected unit price 80, got %s", unitPrice.String())
	}
	if !discount.Equal(decimal.NewFromInt(120)) {
		t.Fatalf("expected discount 120, got %s", discount.String())
	}
}

// TestResolveWholesaleUnitPricePicksCheapestTierForLegacyData 验证即便历史脏数据
// 存在非单调阶梯（高门槛档单价反而更高），选档也按单价最低者成交，避免「买更多反而更贵」。
func TestResolveWholesaleUnitPricePicksCheapestTierForLegacyData(t *testing.T) {
	product := &models.Product{
		WholesalePrices: models.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
			{MinQuantity: 10, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(90))},
		},
	}

	// 购买 10 件时两档均满足门槛，应取更便宜的 80 而非门槛更高的 90。
	unitPrice, discount, matched := ResolveWholesaleUnitPrice(product, decimal.NewFromInt(100), 10)
	if !matched {
		t.Fatalf("expected wholesale tier to match")
	}
	if !unitPrice.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("expected unit price 80, got %s", unitPrice.String())
	}
	if !discount.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("expected discount 200, got %s", discount.String())
	}
}

func TestResolveWholesaleUnitPriceIgnoresHigherTierPrice(t *testing.T) {
	product := &models.Product{
		WholesalePrices: models.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(120))},
		},
	}

	unitPrice, discount, matched := ResolveWholesaleUnitPrice(product, decimal.NewFromInt(100), 5)
	if matched {
		t.Fatalf("expected higher wholesale price to be ignored")
	}
	if !unitPrice.Equal(decimal.NewFromInt(100)) || !discount.IsZero() {
		t.Fatalf("unexpected price result: unit=%s discount=%s", unitPrice.String(), discount.String())
	}
}

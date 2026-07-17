package admin

import (
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestBuildCreateCouponInputFromRequestMapsFieldsAndParsesTimes(t *testing.T) {
	isActive := false
	disabledWholesalePrice := true
	perItemDiscount := true
	req := CreateCouponRequest{
		Code:                   "SAVE10",
		Type:                   "fixed",
		Value:                  12.345,
		MinAmount:              99.994,
		MaxDiscount:            5.678,
		UsageLimit:             100,
		PerUserLimit:           2,
		DisabledWholesalePrice: &disabledWholesalePrice,
		PerItemDiscount:        &perItemDiscount,
		PaymentRoles:           []string{"balance", "online"},
		MemberLevels:           []uint{1, 3},
		ScopeRefIDs:            []uint{10, 20},
		StartsAt:               "2026-06-01T10:00:00Z",
		EndsAt:                 "2026-06-02T10:00:00Z",
		IsActive:               &isActive,
	}

	input, err := buildCreateCouponInputFromRequest(req)
	if err != nil {
		t.Fatalf("buildCreateCouponInputFromRequest: %v", err)
	}
	if input.Code != req.Code || input.Type != req.Type {
		t.Fatalf("basic fields mismatch: %#v", input)
	}
	if !input.Value.Decimal.Equal(decimal.RequireFromString("12.35")) {
		t.Fatalf("value mismatch: %s", input.Value.String())
	}
	if !input.MinAmount.Decimal.Equal(decimal.RequireFromString("99.99")) {
		t.Fatalf("min_amount mismatch: %s", input.MinAmount.String())
	}
	if !input.MaxDiscount.Decimal.Equal(decimal.RequireFromString("5.68")) {
		t.Fatalf("max_discount mismatch: %s", input.MaxDiscount.String())
	}
	if input.UsageLimit != req.UsageLimit || input.PerUserLimit != req.PerUserLimit {
		t.Fatalf("limit fields mismatch: %#v", input)
	}
	if input.DisabledWholesalePrice == nil || *input.DisabledWholesalePrice != disabledWholesalePrice {
		t.Fatalf("disabled_wholesale_price mismatch: %#v", input.DisabledWholesalePrice)
	}
	if input.PerItemDiscount == nil || *input.PerItemDiscount != perItemDiscount {
		t.Fatalf("per_item_discount mismatch: %#v", input.PerItemDiscount)
	}
	if !reflect.DeepEqual(input.PaymentRoles, req.PaymentRoles) {
		t.Fatalf("payment_roles mismatch: %#v", input.PaymentRoles)
	}
	if !reflect.DeepEqual(input.MemberLevels, req.MemberLevels) {
		t.Fatalf("member_levels mismatch: %#v", input.MemberLevels)
	}
	if !reflect.DeepEqual(input.ScopeRefIDs, req.ScopeRefIDs) {
		t.Fatalf("scope_ref_ids mismatch: %#v", input.ScopeRefIDs)
	}
	if input.StartsAt == nil || input.StartsAt.Format(time.RFC3339) != req.StartsAt {
		t.Fatalf("starts_at mismatch: %#v", input.StartsAt)
	}
	if input.EndsAt == nil || input.EndsAt.Format(time.RFC3339) != req.EndsAt {
		t.Fatalf("ends_at mismatch: %#v", input.EndsAt)
	}
	if input.IsActive == nil || *input.IsActive != isActive {
		t.Fatalf("is_active mismatch: %#v", input.IsActive)
	}
}

func TestBuildUpdateCouponInputFromRequestMapsFieldsAndParsesTimes(t *testing.T) {
	isActive := true
	disabledWholesalePrice := true
	perItemDiscount := true
	req := CreateCouponRequest{
		Code:                   "SAVE20",
		Type:                   "percent",
		Value:                  20,
		MinAmount:              50,
		MaxDiscount:            30,
		UsageLimit:             200,
		PerUserLimit:           1,
		DisabledWholesalePrice: &disabledWholesalePrice,
		PerItemDiscount:        &perItemDiscount,
		PaymentRoles:           []string{"online"},
		MemberLevels:           []uint{2},
		ScopeRefIDs:            []uint{30},
		StartsAt:               "2026-07-01T10:00:00Z",
		EndsAt:                 "2026-07-02T10:00:00Z",
		IsActive:               &isActive,
	}

	input, err := buildUpdateCouponInputFromRequest(req)
	if err != nil {
		t.Fatalf("buildUpdateCouponInputFromRequest: %v", err)
	}
	if input.Code != req.Code || input.Type != req.Type {
		t.Fatalf("basic fields mismatch: %#v", input)
	}
	if !input.Value.Decimal.Equal(decimal.RequireFromString("20.00")) {
		t.Fatalf("value mismatch: %s", input.Value.String())
	}
	if !reflect.DeepEqual(input.ScopeRefIDs, req.ScopeRefIDs) {
		t.Fatalf("scope_ref_ids mismatch: %#v", input.ScopeRefIDs)
	}
	if input.DisabledWholesalePrice == nil || *input.DisabledWholesalePrice != disabledWholesalePrice {
		t.Fatalf("disabled_wholesale_price mismatch: %#v", input.DisabledWholesalePrice)
	}
	if input.PerItemDiscount == nil || *input.PerItemDiscount != perItemDiscount {
		t.Fatalf("per_item_discount mismatch: %#v", input.PerItemDiscount)
	}
	if input.StartsAt == nil || input.StartsAt.Format(time.RFC3339) != req.StartsAt {
		t.Fatalf("starts_at mismatch: %#v", input.StartsAt)
	}
	if input.EndsAt == nil || input.EndsAt.Format(time.RFC3339) != req.EndsAt {
		t.Fatalf("ends_at mismatch: %#v", input.EndsAt)
	}
	if input.IsActive == nil || *input.IsActive != isActive {
		t.Fatalf("is_active mismatch: %#v", input.IsActive)
	}
}

func TestBuildCreateCouponInputFromRequestRejectsInvalidTime(t *testing.T) {
	_, err := buildCreateCouponInputFromRequest(CreateCouponRequest{
		Code:     "SAVE10",
		Type:     "fixed",
		Value:    10,
		StartsAt: "not-a-time",
	})
	if err == nil {
		t.Fatalf("expected invalid time error")
	}
}

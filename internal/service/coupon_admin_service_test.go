package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func newCouponAdminServiceForTest(t *testing.T) (*CouponAdminService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:coupon_admin_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Coupon{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return NewCouponAdminService(repository.NewCouponRepository(db)), db
}

func validCouponAdminInput(code string) CreateCouponInput {
	return CreateCouponInput{
		Code:        code,
		Type:        constants.CouponTypePercent,
		Value:       models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		MinAmount:   models.NewMoneyFromDecimal(decimal.Zero),
		MaxDiscount: models.NewMoneyFromDecimal(decimal.Zero),
		ScopeRefIDs: []uint{1},
	}
}

func TestCouponAdminServiceCreateDefaultsDisabledWholesalePriceFalse(t *testing.T) {
	svc, _ := newCouponAdminServiceForTest(t)

	coupon, err := svc.Create(validCouponAdminInput("DEFAULT_WHOLESALE"))
	if err != nil {
		t.Fatalf("create coupon failed: %v", err)
	}
	if coupon.DisabledWholesalePrice {
		t.Fatalf("disabled_wholesale_price should default to false")
	}
	if coupon.PerItemDiscount {
		t.Fatalf("per_item_discount should default to false")
	}
}

func TestCouponAdminServiceCreateAndUpdateDisabledWholesalePrice(t *testing.T) {
	svc, _ := newCouponAdminServiceForTest(t)

	disabled := true
	input := validCouponAdminInput("DISABLED_WHOLESALE")
	input.DisabledWholesalePrice = &disabled
	coupon, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create coupon failed: %v", err)
	}
	if !coupon.DisabledWholesalePrice {
		t.Fatalf("disabled_wholesale_price should be true after create")
	}

	updateInput := UpdateCouponInput(validCouponAdminInput("DISABLED_WHOLESALE_RENAMED"))
	updated, err := svc.Update(coupon.ID, updateInput)
	if err != nil {
		t.Fatalf("update coupon failed: %v", err)
	}
	if !updated.DisabledWholesalePrice {
		t.Fatalf("disabled_wholesale_price should be preserved when update input omits it")
	}

	disabled = false
	updateInput.DisabledWholesalePrice = &disabled
	updated, err = svc.Update(coupon.ID, updateInput)
	if err != nil {
		t.Fatalf("update coupon failed: %v", err)
	}
	if updated.DisabledWholesalePrice {
		t.Fatalf("disabled_wholesale_price should be false after explicit update")
	}
}

func TestCouponAdminServiceCreateAndUpdatePerItemDiscount(t *testing.T) {
	svc, _ := newCouponAdminServiceForTest(t)

	enabled := true
	input := validCouponAdminInput("PER_ITEM")
	input.Type = constants.CouponTypeFixed
	input.PerItemDiscount = &enabled
	coupon, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create coupon failed: %v", err)
	}
	if !coupon.PerItemDiscount {
		t.Fatalf("per_item_discount should be true after fixed coupon create")
	}

	updateInput := UpdateCouponInput(validCouponAdminInput("PER_ITEM_RENAMED"))
	updateInput.Type = constants.CouponTypeFixed
	updated, err := svc.Update(coupon.ID, updateInput)
	if err != nil {
		t.Fatalf("update coupon failed: %v", err)
	}
	if !updated.PerItemDiscount {
		t.Fatalf("per_item_discount should be preserved when update input omits it")
	}

	enabled = false
	updateInput.PerItemDiscount = &enabled
	updated, err = svc.Update(coupon.ID, updateInput)
	if err != nil {
		t.Fatalf("update coupon failed: %v", err)
	}
	if updated.PerItemDiscount {
		t.Fatalf("per_item_discount should be false after explicit update")
	}
}

func TestCouponAdminServiceIgnoresPerItemDiscountForPercentCoupon(t *testing.T) {
	svc, _ := newCouponAdminServiceForTest(t)

	enabled := true
	input := validCouponAdminInput("PER_ITEM_PERCENT")
	input.PerItemDiscount = &enabled
	coupon, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create coupon failed: %v", err)
	}
	if coupon.PerItemDiscount {
		t.Fatalf("per_item_discount should be false for percent coupon")
	}

	updateInput := UpdateCouponInput(validCouponAdminInput("PER_ITEM_PERCENT_RENAMED"))
	updateInput.PerItemDiscount = &enabled
	updated, err := svc.Update(coupon.ID, updateInput)
	if err != nil {
		t.Fatalf("update coupon failed: %v", err)
	}
	if updated.PerItemDiscount {
		t.Fatalf("per_item_discount should be false after percent coupon update")
	}
}

package integrationtest

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	couponapp "github.com/dujiao-next/internal/modules/coupon/application"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"
	coupongormstore "github.com/dujiao-next/internal/modules/coupon/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func newCouponServiceForTest(t *testing.T) (*couponapp.Service, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:coupon_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&coupondomain.Coupon{}, &coupondomain.CouponUsage{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	couponRepo := coupongormstore.New(db)
	usageRepo := coupongormstore.NewUsageStore(db)
	return couponapp.NewService(couponRepo, usageRepo), db
}

func createCouponFixture(t *testing.T, db *gorm.DB, coupon coupondomain.Coupon) coupondomain.Coupon {
	t.Helper()
	if err := db.Create(&coupon).Error; err != nil {
		t.Fatalf("create coupon fixture failed: %v", err)
	}
	return coupon
}

func TestCouponServiceApplyCoupon_RespectsPaymentRoleAndMemberLevel(t *testing.T) {
	svc, db := newCouponServiceForTest(t)
	now := time.Now()
	items := []couponcontract.EligibilityItem{
		{
			ProductID:  100,
			Quantity:   1,
			TotalPrice: money.FromDecimal(decimal.NewFromInt(100)),
		},
	}
	subtotal := money.FromDecimal(decimal.NewFromInt(100))

	testCases := []struct {
		name          string
		code          string
		roles         jsonslice.Strings
		memberLevels  jsonslice.Uints
		isGuest       bool
		memberLevelID uint
		expectErr     error
	}{
		{
			name:          "no restrictions allows guest",
			code:          "NO_LIMIT",
			isGuest:       true,
			memberLevelID: 0,
		},
		{
			name:          "member-only coupon blocks guest",
			code:          "MEMBER_ONLY",
			roles:         jsonslice.Strings{constants.PaymentRoleMember},
			isGuest:       true,
			memberLevelID: 0,
			expectErr:     couponcontract.ErrPaymentRoleMemberOnly,
		},
		{
			name:          "guest-only coupon blocks member",
			code:          "GUEST_ONLY",
			roles:         jsonslice.Strings{constants.PaymentRoleGuest},
			isGuest:       false,
			memberLevelID: 1,
			expectErr:     couponcontract.ErrPaymentRoleGuestOnly,
		},
		{
			name:          "member-level limited coupon blocks other levels",
			code:          "VIP2_ONLY",
			memberLevels:  jsonslice.Uints{2},
			isGuest:       false,
			memberLevelID: 1,
			expectErr:     couponcontract.ErrMemberLevelNotAllowed,
		},
		{
			name:          "member-level limited coupon allows matching level",
			code:          "VIP3_OK",
			memberLevels:  jsonslice.Uints{3},
			isGuest:       false,
			memberLevelID: 3,
		},
		{
			name:          "combined restrictions allow matching member",
			code:          "MEMBER_VIP5",
			roles:         jsonslice.Strings{constants.PaymentRoleMember},
			memberLevels:  jsonslice.Uints{5},
			isGuest:       false,
			memberLevelID: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = createCouponFixture(t, db, coupondomain.Coupon{
				Code:         tc.code,
				Type:         constants.CouponTypeFixed,
				Value:        money.FromDecimal(decimal.NewFromInt(10)),
				MinAmount:    money.FromDecimal(decimal.Zero),
				MaxDiscount:  money.FromDecimal(decimal.Zero),
				ScopeType:    constants.ScopeTypeProduct,
				ScopeRefIDs:  "[100]",
				IsActive:     true,
				PaymentRoles: tc.roles,
				MemberLevels: tc.memberLevels,
				StartsAt:     &now,
			})

			_, _, err := svc.ApplyCoupon(subtotal, tc.code, 0, items, tc.isGuest, tc.memberLevelID)
			if tc.expectErr == nil && err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if tc.expectErr != nil && err != tc.expectErr {
				t.Fatalf("expected %v, got %v", tc.expectErr, err)
			}
		})
	}
}

func TestCouponServiceApplyCouponFixedPerItemDiscount(t *testing.T) {
	svc, db := newCouponServiceForTest(t)
	items := []couponcontract.EligibilityItem{
		{
			ProductID:  100,
			Quantity:   3,
			TotalPrice: money.FromDecimal(decimal.NewFromInt(375)),
		},
	}
	subtotal := money.FromDecimal(decimal.NewFromInt(375))

	_ = createCouponFixture(t, db, coupondomain.Coupon{
		Code:            "FIXED_ONCE",
		Type:            constants.CouponTypeFixed,
		Value:           money.FromDecimal(decimal.NewFromInt(5)),
		MinAmount:       money.FromDecimal(decimal.Zero),
		MaxDiscount:     money.FromDecimal(decimal.Zero),
		ScopeType:       constants.ScopeTypeProduct,
		ScopeRefIDs:     "[100]",
		PerItemDiscount: false,
		IsActive:        true,
	})
	discount, _, err := svc.ApplyCoupon(subtotal, "FIXED_ONCE", 0, items, false, 0)
	if err != nil {
		t.Fatalf("apply fixed once coupon failed: %v", err)
	}
	if !discount.Decimal.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("expected fixed once discount 5, got %s", discount.String())
	}

	_ = createCouponFixture(t, db, coupondomain.Coupon{
		Code:            "FIXED_PER_ITEM",
		Type:            constants.CouponTypeFixed,
		Value:           money.FromDecimal(decimal.NewFromInt(5)),
		MinAmount:       money.FromDecimal(decimal.Zero),
		MaxDiscount:     money.FromDecimal(decimal.Zero),
		ScopeType:       constants.ScopeTypeProduct,
		ScopeRefIDs:     "[100]",
		PerItemDiscount: true,
		IsActive:        true,
	})
	discount, _, err = svc.ApplyCoupon(subtotal, "FIXED_PER_ITEM", 0, items, false, 0)
	if err != nil {
		t.Fatalf("apply fixed per item coupon failed: %v", err)
	}
	if !discount.Decimal.Equal(decimal.NewFromInt(15)) {
		t.Fatalf("expected fixed per item discount 15, got %s", discount.String())
	}
}

func TestCouponServiceApplyCouponFixedPerItemDiscountRespectsMaxDiscount(t *testing.T) {
	svc, db := newCouponServiceForTest(t)
	items := []couponcontract.EligibilityItem{
		{
			ProductID:  100,
			Quantity:   3,
			TotalPrice: money.FromDecimal(decimal.NewFromInt(375)),
		},
	}
	subtotal := money.FromDecimal(decimal.NewFromInt(375))

	_ = createCouponFixture(t, db, coupondomain.Coupon{
		Code:            "FIXED_PER_ITEM_CAP",
		Type:            constants.CouponTypeFixed,
		Value:           money.FromDecimal(decimal.NewFromInt(5)),
		MinAmount:       money.FromDecimal(decimal.Zero),
		MaxDiscount:     money.FromDecimal(decimal.NewFromInt(10)),
		ScopeType:       constants.ScopeTypeProduct,
		ScopeRefIDs:     "[100]",
		PerItemDiscount: true,
		IsActive:        true,
	})

	discount, _, err := svc.ApplyCoupon(subtotal, "FIXED_PER_ITEM_CAP", 0, items, false, 0)
	if err != nil {
		t.Fatalf("apply fixed per item coupon failed: %v", err)
	}
	if !discount.Decimal.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("expected max discount cap 10, got %s", discount.String())
	}
}

func TestCouponServiceApplyCouponFixedPerItemDiscountExcludesWholesaleItems(t *testing.T) {
	svc, db := newCouponServiceForTest(t)
	items := []couponcontract.EligibilityItem{
		{
			ProductID:         100,
			Quantity:          5,
			TotalPrice:        money.FromDecimal(decimal.NewFromInt(600)),
			WholesaleDiscount: money.FromDecimal(decimal.NewFromInt(25)),
		},
		{
			ProductID:  101,
			Quantity:   2,
			TotalPrice: money.FromDecimal(decimal.NewFromInt(250)),
		},
	}
	subtotal := money.FromDecimal(decimal.NewFromInt(850))

	_ = createCouponFixture(t, db, coupondomain.Coupon{
		Code:                   "FIXED_PER_ITEM_NO_WHOLESALE",
		Type:                   constants.CouponTypeFixed,
		Value:                  money.FromDecimal(decimal.NewFromInt(5)),
		MinAmount:              money.FromDecimal(decimal.Zero),
		MaxDiscount:            money.FromDecimal(decimal.Zero),
		ScopeType:              constants.ScopeTypeProduct,
		ScopeRefIDs:            "[100,101]",
		DisabledWholesalePrice: true,
		PerItemDiscount:        true,
		IsActive:               true,
	})

	discount, _, err := svc.ApplyCoupon(subtotal, "FIXED_PER_ITEM_NO_WHOLESALE", 0, items, false, 0)
	if err != nil {
		t.Fatalf("apply fixed per item coupon failed: %v", err)
	}
	if !discount.Decimal.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("expected only non-wholesale quantity discount 10, got %s", discount.String())
	}
}

func TestCouponServiceApplyCouponPercentIgnoresPerItemDiscount(t *testing.T) {
	svc, db := newCouponServiceForTest(t)
	items := []couponcontract.EligibilityItem{
		{
			ProductID:  100,
			Quantity:   3,
			TotalPrice: money.FromDecimal(decimal.NewFromInt(300)),
		},
	}
	subtotal := money.FromDecimal(decimal.NewFromInt(300))

	_ = createCouponFixture(t, db, coupondomain.Coupon{
		Code:            "PERCENT_PER_ITEM_IGNORED",
		Type:            constants.CouponTypePercent,
		Value:           money.FromDecimal(decimal.NewFromInt(10)),
		MinAmount:       money.FromDecimal(decimal.Zero),
		MaxDiscount:     money.FromDecimal(decimal.Zero),
		ScopeType:       constants.ScopeTypeProduct,
		ScopeRefIDs:     "[100]",
		PerItemDiscount: true,
		IsActive:        true,
	})

	discount, _, err := svc.ApplyCoupon(subtotal, "PERCENT_PER_ITEM_IGNORED", 0, items, false, 0)
	if err != nil {
		t.Fatalf("apply percent coupon failed: %v", err)
	}
	if !discount.Decimal.Equal(decimal.NewFromInt(30)) {
		t.Fatalf("expected percent discount 30, got %s", discount.String())
	}
}

func TestCouponServiceApplyCouponRespectsScopeSKUIDs(t *testing.T) {
	svc, db := newCouponServiceForTest(t)
	items := []couponcontract.EligibilityItem{
		{ProductID: 100, SKUID: 1, Quantity: 1, TotalPrice: money.FromDecimal(decimal.NewFromInt(100))},
		{ProductID: 100, SKUID: 2, Quantity: 1, TotalPrice: money.FromDecimal(decimal.NewFromInt(200))},
	}
	subtotal := money.FromDecimal(decimal.NewFromInt(300))

	// 未限制规格：商品下全部规格参与，10% of 300 = 30。
	_ = createCouponFixture(t, db, coupondomain.Coupon{
		Code:        "SKU_ALL",
		Type:        constants.CouponTypePercent,
		Value:       money.FromDecimal(decimal.NewFromInt(10)),
		ScopeType:   constants.ScopeTypeProduct,
		ScopeRefIDs: "[100]",
		IsActive:    true,
	})
	discount, _, err := svc.ApplyCoupon(subtotal, "SKU_ALL", 0, items, false, 0)
	if err != nil {
		t.Fatalf("apply coupon without sku scope failed: %v", err)
	}
	if !discount.Decimal.Equal(decimal.NewFromInt(30)) {
		t.Fatalf("expected discount 30, got %s", discount.String())
	}

	// 限定规格 2：只对 200 打折，10% = 20。
	_ = createCouponFixture(t, db, coupondomain.Coupon{
		Code:        "SKU_ONE",
		Type:        constants.CouponTypePercent,
		Value:       money.FromDecimal(decimal.NewFromInt(10)),
		ScopeType:   constants.ScopeTypeProduct,
		ScopeRefIDs: "[100]",
		ScopeSKUIDs: "[2]",
		IsActive:    true,
	})
	discount, _, err = svc.ApplyCoupon(subtotal, "SKU_ONE", 0, items, false, 0)
	if err != nil {
		t.Fatalf("apply coupon with sku scope failed: %v", err)
	}
	if !discount.Decimal.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("expected sku-scoped discount 20, got %s", discount.String())
	}

	// 限定规格 3：订单里没有该规格，应判定不适用。
	_ = createCouponFixture(t, db, coupondomain.Coupon{
		Code:        "SKU_MISS",
		Type:        constants.CouponTypePercent,
		Value:       money.FromDecimal(decimal.NewFromInt(10)),
		ScopeType:   constants.ScopeTypeProduct,
		ScopeRefIDs: "[100]",
		ScopeSKUIDs: "[3]",
		IsActive:    true,
	})
	if _, _, err = svc.ApplyCoupon(subtotal, "SKU_MISS", 0, items, false, 0); !errors.Is(err, couponcontract.ErrScopeInvalid) {
		t.Fatalf("expected ErrScopeInvalid for unmatched sku, got %v", err)
	}
}

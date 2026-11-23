package integrationtest

import (
	"errors"
	"testing"

	affiliateapp "github.com/dujiao-next/internal/modules/affiliate/application"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	affiliategormstore "github.com/dujiao-next/internal/modules/affiliate/infrastructure/gormstore"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	"github.com/dujiao-next/internal/testkit/memorysettings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type rateProductReader struct{ products []productdomain.Product }

func (r rateProductReader) ListByIDs([]uint) ([]productdomain.Product, error) { return r.products, nil }

type rateSKUReader struct{ skus []productdomain.ProductSKU }

func (r rateSKUReader) ListByIDs([]uint) ([]productdomain.ProductSKU, error) { return r.skus, nil }

func (r rateSKUReader) ListByProduct(uint, bool) ([]productdomain.ProductSKU, error) {
	return r.skus, nil
}

func ratePtr(v string) *money.Amount {
	a := money.FromDecimal(decimal.RequireFromString(v))
	return &a
}

func amountOf(v string) money.Amount { return money.FromDecimal(decimal.RequireFromString(v)) }

// 商品级 / SKU 级返利比例经 HandleOrderPaid 落到佣金记录上的端到端行为。
func TestHandleOrderPaidResolvesPerItemAffiliateRate(t *testing.T) {
	tests := []struct {
		name       string
		globalRate float64
		products   []productdomain.Product
		skus       []productdomain.ProductSKU
		items      []orderdomain.OrderItem
		wantNone   bool
		wantBase   string
		wantAmount string
		wantRate   string
	}{
		{
			name:       "single product inherits global rate",
			globalRate: 10,
			products:   []productdomain.Product{{ID: 1}},
			items:      []orderdomain.OrderItem{{ProductID: 1, TotalPrice: amountOf("100")}},
			wantBase:   "100.00", wantAmount: "10.00", wantRate: "10.00",
		},
		{
			name:       "mixed rates are summed per item and stored as weighted average",
			globalRate: 10,
			products:   []productdomain.Product{{ID: 1}, {ID: 2, AffiliateRatePercent: ratePtr("30")}},
			items: []orderdomain.OrderItem{
				{ProductID: 1, TotalPrice: amountOf("100")},
				{ProductID: 2, TotalPrice: amountOf("100")},
			},
			wantBase: "200.00", wantAmount: "40.00", wantRate: "20.00",
		},
		{
			// 全站比例为 0、只靠商品级比例运营：不能因为全站为 0 就整单不生成佣金。
			name:       "global rate zero still pays product override",
			globalRate: 0,
			products:   []productdomain.Product{{ID: 1, AffiliateRatePercent: ratePtr("20")}},
			items:      []orderdomain.OrderItem{{ProductID: 1, TotalPrice: amountOf("100")}},
			wantBase:   "100.00", wantAmount: "20.00", wantRate: "20.00",
		},
		{
			name:       "product zero excludes only that item",
			globalRate: 10,
			products:   []productdomain.Product{{ID: 1}, {ID: 2, AffiliateRatePercent: ratePtr("0")}},
			items: []orderdomain.OrderItem{
				{ProductID: 1, TotalPrice: amountOf("100")},
				{ProductID: 2, TotalPrice: amountOf("500")},
			},
			wantBase: "100.00", wantAmount: "10.00", wantRate: "10.00",
		},
		{
			name:       "every item excluded creates no commission",
			globalRate: 10,
			products:   []productdomain.Product{{ID: 1, AffiliateRatePercent: ratePtr("0")}},
			items:      []orderdomain.OrderItem{{ProductID: 1, TotalPrice: amountOf("100")}},
			wantNone:   true,
		},
		{
			name:       "sku rate beats product rate",
			globalRate: 10,
			products:   []productdomain.Product{{ID: 1, AffiliateRatePercent: ratePtr("10")}},
			skus:       []productdomain.ProductSKU{{ID: 11, ProductID: 1, AffiliateRatePercent: ratePtr("50")}},
			items:      []orderdomain.OrderItem{{ProductID: 1, SKUID: 11, TotalPrice: amountOf("100")}},
			wantBase:   "100.00", wantAmount: "50.00", wantRate: "50.00",
		},
		{
			name:       "sku zero excludes despite product rate",
			globalRate: 10,
			products:   []productdomain.Product{{ID: 1, AffiliateRatePercent: ratePtr("30")}},
			skus:       []productdomain.ProductSKU{{ID: 11, ProductID: 1, AffiliateRatePercent: ratePtr("0")}},
			items:      []orderdomain.OrderItem{{ProductID: 1, SKUID: 11, TotalPrice: amountOf("100")}},
			wantNone:   true,
		},
		{
			name:       "sku without override falls back to product rate",
			globalRate: 10,
			products:   []productdomain.Product{{ID: 1, AffiliateRatePercent: ratePtr("30")}},
			skus:       []productdomain.ProductSKU{{ID: 11, ProductID: 1}},
			items:      []orderdomain.OrderItem{{ProductID: 1, SKUID: 11, TotalPrice: amountOf("100")}},
			wantBase:   "100.00", wantAmount: "30.00", wantRate: "30.00",
		},
		{
			// SKU 已被后台删除时订单项仍能算佣金，退回商品级比例而不是报错。
			name:       "deleted sku falls back to product rate",
			globalRate: 10,
			products:   []productdomain.Product{{ID: 1, AffiliateRatePercent: ratePtr("30")}},
			items:      []orderdomain.OrderItem{{ProductID: 1, SKUID: 99, TotalPrice: amountOf("100")}},
			wantBase:   "100.00", wantAmount: "30.00", wantRate: "30.00",
		},
		{
			name:       "coupon discount reduces the commission base",
			globalRate: 10,
			products:   []productdomain.Product{{ID: 1, AffiliateRatePercent: ratePtr("20")}},
			items:      []orderdomain.OrderItem{{ProductID: 1, TotalPrice: amountOf("100"), CouponDiscount: amountOf("40")}},
			wantBase:   "60.00", wantAmount: "12.00", wantRate: "20.00",
		},
		{
			// 金额不是整数时，记录里的比例不能被取整后的佣金带偏（9.99 × 10% 曾显示 10.01%）。
			name:       "fractional amount keeps exact rate",
			globalRate: 10,
			products:   []productdomain.Product{{ID: 1, AffiliateRatePercent: ratePtr("10")}},
			items:      []orderdomain.OrderItem{{ProductID: 1, TotalPrice: amountOf("9.99")}},
			wantBase:   "9.99", wantAmount: "1.00", wantRate: "10.00",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, db, _ := setupAffiliateServiceTest(t)
			if err := db.AutoMigrate(&affiliatedomain.Commission{}); err != nil {
				t.Fatalf("migrate commission: %v", err)
			}
			promoter := createAffiliateTestUser(t, db, "promoter@example.com")
			profile := createAffiliateTestProfile(t, db, promoter.ID, "RATE01", constants.AffiliateProfileStatusActive)

			order := &orderdomain.Order{
				ID: 10, UserID: promoter.ID + 1, Status: constants.OrderStatusPaid, AffiliateProfileID: &profile.ID,
				Items: tc.items,
			}
			store := affiliategormstore.New(db)
			settings := settingsapp.NewService(memorysettings.New())
			if _, err := settings.UpdateAffiliateSetting(settingsintegration.AffiliateSetting{Enabled: true, CommissionRate: tc.globalRate}); err != nil {
				t.Fatalf("init affiliate setting: %v", err)
			}
			svc := affiliateapp.NewService(store, userstore.New(db), staticOrderReader{order}, rateProductReader{tc.products}, rateSKUReader{tc.skus}, settings)

			if err := svc.HandleOrderPaid(order.ID); err != nil {
				t.Fatalf("HandleOrderPaid: %v", err)
			}
			got, err := store.GetCommissionByOrderAndProfile(order.ID, profile.ID, constants.AffiliateCommissionTypeOrder)
			if err != nil {
				t.Fatalf("load commission: %v", err)
			}
			if tc.wantNone {
				if got != nil {
					t.Fatalf("expected no commission, got amount=%s", got.CommissionAmount.String())
				}
				return
			}
			if got == nil {
				t.Fatal("expected a commission, got none")
			}
			if got.BaseAmount.String() != tc.wantBase {
				t.Errorf("base = %s, want %s", got.BaseAmount.String(), tc.wantBase)
			}
			if got.CommissionAmount.String() != tc.wantAmount {
				t.Errorf("commission = %s, want %s", got.CommissionAmount.String(), tc.wantAmount)
			}
			if got.RatePercent.String() != tc.wantRate {
				t.Errorf("rate = %s, want %s", got.RatePercent.String(), tc.wantRate)
			}
		})
	}
}

func TestResolveAffiliateRate(t *testing.T) {
	global := decimal.RequireFromString("10")
	tests := []struct {
		name     string
		sku      *money.Amount
		product  *money.Amount
		global   decimal.Decimal
		wantRate string
	}{
		{"nothing configured inherits global", nil, nil, global, "10"},
		{"product overrides global", nil, ratePtr("25"), global, "25"},
		{"sku overrides product", ratePtr("40"), ratePtr("25"), global, "40"},
		{"sku overrides global when product inherits", ratePtr("5"), nil, global, "5"},
		{"product zero disables despite global", nil, ratePtr("0"), global, "0"},
		{"sku zero disables despite product and global", ratePtr("0"), ratePtr("25"), global, "0"},
		{"product zero disables despite sku override", ratePtr("20"), ratePtr("0"), global, "0"},
		{"sku zero is not the same as unset", ratePtr("0"), nil, global, "0"},
		{"global zero with product override still pays", nil, ratePtr("20"), decimal.Zero, "20"},
		{"global zero and nothing else pays nothing", nil, nil, decimal.Zero, "0"},
		{"over 100 is clamped", nil, ratePtr("250"), global, "100"},
		{"negative is clamped to zero", nil, ratePtr("-5"), global, "0"},
		{"rounded to two decimals", nil, nil, decimal.RequireFromString("12.345"), "12.35"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rate := affiliateapp.ResolveAffiliateRate(tc.sku, tc.product, tc.global)
			if !rate.Equal(decimal.RequireFromString(tc.wantRate)) {
				t.Fatalf("rate = %s, want %s", rate, tc.wantRate)
			}
		})
	}
}

// 推广员看到的比例必须与结算使用同一套解析：SKU → 商品 → 全站，且只对已开通、未停用的推广员开放。
func TestGetProductRates(t *testing.T) {
	setup := func(t *testing.T, globalRate float64, enabled bool, profileStatus string, products []productdomain.Product, skus []productdomain.ProductSKU) (*affiliateapp.Service, uint) {
		t.Helper()
		_, db, _ := setupAffiliateServiceTest(t)
		user := createAffiliateTestUser(t, db, "rates@example.com")
		if profileStatus != "" {
			createAffiliateTestProfile(t, db, user.ID, "RATES001", profileStatus)
		}
		settings := settingsapp.NewService(memorysettings.New())
		if _, err := settings.UpdateAffiliateSetting(settingsintegration.AffiliateSetting{Enabled: enabled, CommissionRate: globalRate}); err != nil {
			t.Fatalf("init affiliate setting: %v", err)
		}
		svc := affiliateapp.NewService(affiliategormstore.New(db), userstore.New(db), staticOrderReader{}, rateProductReader{products}, rateSKUReader{skus}, settings)
		return svc, user.ID
	}
	active := constants.AffiliateProfileStatusActive

	t.Run("resolves sku then product then global", func(t *testing.T) {
		svc, uid := setup(t, 10, true, active,
			[]productdomain.Product{{ID: 1, IsActive: true, AffiliateRatePercent: ratePtr("30")}},
			[]productdomain.ProductSKU{
				{ID: 11, ProductID: 1},
				{ID: 12, ProductID: 1, AffiliateRatePercent: ratePtr("50")},
				{ID: 13, ProductID: 1, AffiliateRatePercent: ratePtr("0")},
			})
		got, err := svc.GetProductRates(uid, 1)
		if err != nil {
			t.Fatalf("GetProductRates: %v", err)
		}
		if got.RatePercent.String() != "30.00" {
			t.Errorf("product rate = %s, want 30.00", got.RatePercent.String())
		}
		want := map[uint]string{11: "30.00", 12: "50.00", 13: "0.00"}
		if len(got.SKUs) != len(want) {
			t.Fatalf("sku rates = %d, want %d", len(got.SKUs), len(want))
		}
		for _, sku := range got.SKUs {
			if sku.RatePercent.String() != want[sku.SKUID] {
				t.Errorf("sku %d rate = %s, want %s", sku.SKUID, sku.RatePercent.String(), want[sku.SKUID])
			}
		}
	})

	t.Run("product without override inherits global", func(t *testing.T) {
		svc, uid := setup(t, 12.5, true, active, []productdomain.Product{{ID: 1, IsActive: true}}, nil)
		got, err := svc.GetProductRates(uid, 1)
		if err != nil || got.RatePercent.String() != "12.50" {
			t.Fatalf("rate = %s err=%v, want 12.50", got.RatePercent.String(), err)
		}
	})

	t.Run("disabled profile is rejected", func(t *testing.T) {
		svc, uid := setup(t, 10, true, constants.AffiliateProfileStatusDisabled, []productdomain.Product{{ID: 1, IsActive: true}}, nil)
		if _, err := svc.GetProductRates(uid, 1); !errors.Is(err, affiliateapp.ErrNotOpened) {
			t.Fatalf("err = %v, want ErrNotOpened", err)
		}
	})

	t.Run("user without affiliate profile is rejected", func(t *testing.T) {
		svc, uid := setup(t, 10, true, "", []productdomain.Product{{ID: 1, IsActive: true}}, nil)
		if _, err := svc.GetProductRates(uid, 1); !errors.Is(err, affiliateapp.ErrNotOpened) {
			t.Fatalf("err = %v, want ErrNotOpened", err)
		}
	})

	t.Run("site affiliate switched off is rejected", func(t *testing.T) {
		svc, uid := setup(t, 10, false, active, []productdomain.Product{{ID: 1, IsActive: true}}, nil)
		if _, err := svc.GetProductRates(uid, 1); !errors.Is(err, affiliateapp.ErrDisabled) {
			t.Fatalf("err = %v, want ErrDisabled", err)
		}
	})

	t.Run("inactive or missing product is not found", func(t *testing.T) {
		svc, uid := setup(t, 10, true, active, []productdomain.Product{{ID: 1, IsActive: false}}, nil)
		if _, err := svc.GetProductRates(uid, 1); !errors.Is(err, affiliateapp.ErrNotFound) {
			t.Fatalf("inactive: err = %v, want ErrNotFound", err)
		}
		svc, uid = setup(t, 10, true, active, nil, nil)
		if _, err := svc.GetProductRates(uid, 1); !errors.Is(err, affiliateapp.ErrNotFound) {
			t.Fatalf("missing: err = %v, want ErrNotFound", err)
		}
	})
}

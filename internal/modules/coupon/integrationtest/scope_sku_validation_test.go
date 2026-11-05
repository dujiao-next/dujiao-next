package integrationtest

import (
	"errors"
	"testing"

	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
)

// fakeSKUDirectory 以固定映射模拟「规格 ID -> 所属商品 ID」。
type fakeSKUDirectory map[uint]uint

func (f fakeSKUDirectory) ResolveSKUProducts(ids []uint) (map[uint]uint, error) {
	result := make(map[uint]uint, len(ids))
	for _, id := range ids {
		if productID, ok := f[id]; ok {
			result[id] = productID
		}
	}
	return result, nil
}

func TestCouponAdminServiceValidatesScopeSKUsBelongToProducts(t *testing.T) {
	svc, _ := newCouponAdminServiceForTest(t)
	svc.WithSKUDirectory(fakeSKUDirectory{11: 1, 12: 1, 21: 2})

	// 规格属于所选商品：允许。
	ok := validCouponAdminInput("SKU_OK")
	ok.ScopeRefIDs = []uint{1}
	ok.ScopeSKUIDs = []uint{11, 12}
	created, err := svc.Create(ok)
	if err != nil {
		t.Fatalf("create with owned skus failed: %v", err)
	}
	if created.ScopeSKUIDs != "[11,12]" {
		t.Fatalf("scope_sku_ids want [11,12] got %q", created.ScopeSKUIDs)
	}

	// 规格属于别的商品：拒绝。
	foreign := validCouponAdminInput("SKU_FOREIGN")
	foreign.ScopeRefIDs = []uint{1}
	foreign.ScopeSKUIDs = []uint{21}
	if _, err := svc.Create(foreign); !errors.Is(err, couponcontract.ErrScopeInvalid) {
		t.Fatalf("foreign sku want ErrScopeInvalid got %v", err)
	}

	// 规格不存在：拒绝。
	missing := validCouponAdminInput("SKU_MISSING")
	missing.ScopeRefIDs = []uint{1}
	missing.ScopeSKUIDs = []uint{999}
	if _, err := svc.Create(missing); !errors.Is(err, couponcontract.ErrScopeInvalid) {
		t.Fatalf("missing sku want ErrScopeInvalid got %v", err)
	}

	// 不限规格：不触发校验。
	none := validCouponAdminInput("SKU_NONE")
	none.ScopeRefIDs = []uint{1}
	if _, err := svc.Create(none); err != nil {
		t.Fatalf("create without skus failed: %v", err)
	}
}

// 限制规格时，每个所选商品都必须至少选中一个规格，
// 否则该商品下的订单永远匹配不上，前端提示也会误导管理员。
func TestCouponAdminServiceRequiresEachProductCoveredBySKUs(t *testing.T) {
	svc, _ := newCouponAdminServiceForTest(t)
	svc.WithSKUDirectory(fakeSKUDirectory{11: 1, 12: 1, 21: 2, 22: 2})

	// 商品 1/2 都覆盖：允许。
	covered := validCouponAdminInput("SKU_COVERED")
	covered.ScopeRefIDs = []uint{1, 2}
	covered.ScopeSKUIDs = []uint{11, 22}
	if _, err := svc.Create(covered); err != nil {
		t.Fatalf("create with covered skus failed: %v", err)
	}

	// 商品 2 没有选规格：拒绝。
	partial := validCouponAdminInput("SKU_PARTIAL")
	partial.ScopeRefIDs = []uint{1, 2}
	partial.ScopeSKUIDs = []uint{11}
	if _, err := svc.Create(partial); !errors.Is(err, couponcontract.ErrScopeInvalid) {
		t.Fatalf("uncovered product want ErrScopeInvalid got %v", err)
	}
}

package architecture

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestOrderServiceTestsAreSplitByResponsibility(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	applicationDirectory := filepath.Join(repositoryRoot, "internal", "modules", "order", "application")
	integrationDirectory := filepath.Join(repositoryRoot, "internal", "modules", "order", "integrationtest", "application")
	for _, legacyPath := range []string{
		filepath.Join(repositoryRoot, "internal", "service", "order_service_test.go"),
		filepath.Join(applicationDirectory, "order_service_test.go"),
	} {
		if _, err := os.Stat(legacyPath); err == nil {
			t.Fatalf("monolithic order_service_test.go must stay removed: %s", legacyPath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", legacyPath, err)
		}
	}

	expected := map[string][]string{
		filepath.Join(applicationDirectory, "order_service_helpers_test.go"): {
			"TestMergeCreateOrderItems", "TestMergeCreateOrderItemsConflict",
			"TestApplyCouponDiscountToItems", "TestResolveManualFormSubmissionPreferOrderItemKey",
			"TestResolveManualFormSubmissionFallbackLegacyProductKey",
		},
		filepath.Join(integrationDirectory, "order_service_cancel_test.go"): {
			"TestCancelExpiredOrderExpiresPendingPayments", "setupCancelPaymentTestDB",
			"newPendingOrderForCancel", "newPaymentForOrder", "TestCancelOrderExpiresPendingPayments",
			"TestUpdateOrderStatusAdminCancelExpiresPendingPaymentsSingleOrder",
			"TestCancelExpiredOrderExpiresPaymentsForParentAndChildren",
		},
		filepath.Join(applicationDirectory, "order_service_status_test.go"): {
			"TestCalcParentStatus", "TestCalcParentStatusAllRefunded", "TestCalcParentStatusPartiallyRefunded",
			"TestExpectedRefundStatus", "TestResolvedParentStatusPrefersOwnRefund",
			"TestIsTransitionAllowedRefunded", "TestUpdateOrderStatusParentToPartiallyRefundedSyncsChildren",
			"TestUpdateOrderStatusRejectsCancelingChildOrder",
			"TestUpdateOrderStatusRejectsCancelingPaidParent",
			"TestUpdateOrderStatusRejectsManualPaidTransition",
			"TestCanCompleteParentOrder", "TestCanCompleteParentOrderRejectInvalidStatus",
			"TestCanCompleteParentOrderRejectInvalidChild",
		},
		filepath.Join(applicationDirectory, "order_service_pricing_test.go"): {
			"assertBuildOrderResultRejectsPurchaseQuantity", "TestBuildOrderResultRejectsZeroPromotionPrice",
			"TestPreviewOrderAppliesMemberDiscountForManualProductBeforeFormCompleted",
			"TestBuildOrderResultStacksPromotionAndMemberDiscount",
			"TestBuildOrderResultRejectsProductMaxPurchaseQuantityExceeded",
			"TestBuildOrderResultRejectsProductMinPurchaseQuantityNotMet",
			"TestBuildOrderResultOriginalAmountBeforePromotion",
			"TestBuildOrderResultRejectsZeroTotalAmountAfterCoupon",
		},
	}

	for file, want := range expected {
		parsed := parseProductionGoFile(t, file)
		got := declaredFunctionNames(parsed)
		sort.Strings(want)
		sort.Strings(got)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s function ownership mismatch\nwant: %v\ngot:  %v", filepath.Base(file), want, got)
		}
	}
}

func TestOrderServiceTestFixtureLivesWithPricingTests(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	applicationDirectory := filepath.Join(repositoryRoot, "internal", "modules", "order", "application")
	integrationDirectory := filepath.Join(repositoryRoot, "internal", "modules", "order", "integrationtest", "application")
	files := []string{
		filepath.Join(applicationDirectory, "order_service_helpers_test.go"),
		filepath.Join(integrationDirectory, "order_service_cancel_test.go"),
		filepath.Join(applicationDirectory, "order_service_status_test.go"),
		filepath.Join(applicationDirectory, "order_service_pricing_test.go"),
	}

	owners := make(map[string]string)
	for _, file := range files {
		parsed := parseProductionGoFile(t, file)
		for _, typeName := range declaredTypeNames(parsed) {
			if previous, exists := owners[typeName]; exists {
				t.Fatalf("type %s declared in both %s and %s", typeName, filepath.Base(previous), filepath.Base(file))
			}
			owners[typeName] = file
		}
	}
	if got := filepath.Base(owners["orderPurchaseQuantityLimitFixture"]); got != "order_service_pricing_test.go" {
		t.Fatalf("orderPurchaseQuantityLimitFixture must live in pricing tests, got %q", got)
	}
}

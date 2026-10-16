package architecture

import (
	"go/ast"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestPaymentServiceImplementationIsSplitByResponsibility(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	serviceDirectory := filepath.Join(repositoryRoot, "internal", "modules", "payment", "application")
	expected := map[string][]string{
		"payment_service.go": {
			"SetProcurementService", "SetDownstreamCallbackService", "SetMemberLevelService",
			"NewPaymentService", "ListPayments", "GetPayment", "ListChannels", "GetChannel",
			"paymentLogger",
		},
		"payment_service_create.go": {"hasProviderResult", "CreatePayment"},
		"payment_service_recharge.go": {
			"CreateWalletRechargePayment", "generateWalletRechargeNo",
			"ExpireWalletRechargePayment", "canExpireWalletRechargePayment",
		},
		"payment_service_provider.go": {
			"detachOutboundRequestContext",
			"shouldUseGatewayOrderNo", "buildGatewayOrderNo", "resolveGatewayOrderNo",
			"resolveProviderOrderNo", "matchesBusinessOrderNo", "buildPaymentReturnQuery",
			"applyProviderPayment", "TestChannelSecurity", "ValidateChannel", "resolveTenantReturnURL",
			"tenantReturnPath", "resolveTokenPayOrderUserKey",
		},
		"payment_service_rules.go": {
			"normalizeOrderAmount", "calculatePaymentAmounts", "paymentCoveredOrderAmount", "paymentExchangeRate", "paymentOriginalAmount", "pickFirstNonEmpty",
			"shouldMarkFulfilling", "shouldUseCNYPaymentCurrency", "validatePaymentAmountForChannel",
			"validatePaymentCurrencyForChannel", "resolveExpireMinutes", "normalizePaymentStatus",
			"isPaymentStatusValid", "shouldAutoFulfill", "isOrderFullyAutoFulfill",
			"buildOrderSubject",
		},
		"payment_service_channel_rules.go": {
			"computeProductChannelIntersection",
			"validateProductPaymentChannel", "validateWalletRechargeChannel",
			"GetAllowedChannelsForProducts", "GetWalletRechargeChannels", "GetAllowedChannelIDsForOrder",
			"GetAvailableChannels", "matchesChannelAmount", "matchesChannelRole",
			"matchesChannelMemberLevel", "matchesChannelPaymentType",
			"validateOrderChannelEligibility", "validateWalletChannelEligibility",
		},
	}

	for file, want := range expected {
		parsed := parseProductionGoFile(t, filepath.Join(serviceDirectory, file))
		got := declaredFunctionNames(parsed)
		sort.Strings(want)
		sort.Strings(got)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s function ownership mismatch\nwant: %v\ngot:  %v", file, want, got)
		}
	}
}

func TestPaymentServiceInputTypesLiveWithTheirResponsibilities(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	serviceDirectory := filepath.Join(repositoryRoot, "internal", "modules", "payment", "application")
	expectedOwner := map[string]string{
		"PaymentService":                    "payment_service.go",
		"PaymentServiceOptions":             "payment_service.go",
		"CreatePaymentInput":                "payment_service_create.go",
		"CreatePaymentResult":               "payment_service_create.go",
		"CreateWalletRechargePaymentInput":  "payment_service_recharge.go",
		"CreateWalletRechargePaymentResult": "payment_service_recharge.go",
		"PaymentCallbackInput":              "payment_service_callback.go",
		"CapturePaymentInput":               "payment_service_capture.go",
		"WebhookCallbackInput":              "payment_service_webhook.go",
	}
	actualOwner := make(map[string][]string, len(expectedOwner))
	files := []string{
		"payment_service.go", "payment_service_create.go", "payment_service_recharge.go",
		"payment_service_callback.go", "payment_service_capture.go", "payment_service_webhook.go",
	}

	for _, file := range files {
		parsed := parseProductionGoFile(t, filepath.Join(serviceDirectory, file))
		for _, typeName := range declaredTypeNames(parsed) {
			if _, tracked := expectedOwner[typeName]; tracked {
				actualOwner[typeName] = append(actualOwner[typeName], file)
			}
		}
	}

	for typeName, wantFile := range expectedOwner {
		gotFiles := actualOwner[typeName]
		if len(gotFiles) != 1 || gotFiles[0] != wantFile {
			t.Errorf("%s ownership mismatch: want [%s], got %v", typeName, wantFile, gotFiles)
		}
	}
}

func declaredTypeNames(parsed *ast.File) []string {
	types := make([]string, 0)
	for _, declaration := range parsed.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok.String() != "type" {
			continue
		}
		for _, spec := range general.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if ok {
				types = append(types, typeSpec.Name.Name)
			}
		}
	}
	return types
}

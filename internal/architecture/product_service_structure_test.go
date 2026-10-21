package architecture

import (
	"go/ast"
	"os"
	"path/filepath"
	"testing"
)

func TestProductApplicationServicesAreExplicitlyComposed(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	bootstrapFile := filepath.Join(repositoryRoot, "internal", "bootstrap", "catalogproduct", "services.go")
	assertFileDeclaresTypes(t, bootstrapFile, []string{"Dependencies", "Services"})
	assertFileDeclaresFunctions(t, bootstrapFile, []string{"New"})
	publicHTTPFile := filepath.Join(repositoryRoot, "internal", "bootstrap", "catalogproduct", "public_http.go")
	assertFileDeclaresTypes(t, publicHTTPFile, []string{"PublicHTTPDependencies"})
	assertFileDeclaresFunctions(t, publicHTTPFile, []string{"NewPublicHTTP"})

	for _, relativePath := range []string{
		"internal/service/product_service.go",
		"internal/service/product_application_compat.go",
	} {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		if _, err := os.Stat(path); err == nil {
			t.Errorf("deleted Product facade path was recreated: %s", relativePath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", relativePath, err)
		}
	}

	testHelper := parseProductionGoFile(t, filepath.Join(
		repositoryRoot,
		"internal", "modules", "catalog", "product", "integrationtest", "application", "product_test_helpers_test.go",
	))
	for _, typeName := range declaredTypeNames(testHelper) {
		if typeName == "ProductService" {
			t.Fatal("Product integration tests must use explicit Read/Admin/Write services, not a ProductService facade")
		}
	}
	for _, functionName := range declaredFunctionNames(testHelper) {
		if functionName == "NewProductService" {
			t.Fatal("Product integration tests must not recreate the deleted ProductService constructor")
		}
	}
}

func TestProductServiceTestsAreSplitByResponsibility(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	testDirectory := filepath.Join(repositoryRoot, "internal", "modules", "catalog", "product", "integrationtest", "application")
	legacyPath := filepath.Join(repositoryRoot, "internal", "service", "product_service_test.go")
	if _, err := os.Stat(legacyPath); err == nil {
		t.Fatalf("product_service_test.go must be replaced by responsibility-focused test files")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat product_service_test.go: %v", err)
	}

	expectedOwner := map[string]string{
		"TestProductServiceUpdateRejectsDisablingAutoSKUWithCardSecretStock":  "product_sku_test.go",
		"TestApplyAutoStockCounts_LegacyStockPrefersDefaultSKU":               "product_stock_test.go",
		"TestProductServiceListPublicIncludesChildProductsForParentCategory":  "product_query_test.go",
		"TestProductServiceListPublicSortOrderDescending":                     "product_query_test.go",
		"TestProductServiceListPublicSortsSKUsDescending":                     "product_query_test.go",
		"TestProductServiceGetAdminByIDIncludesInactiveSKUs":                  "product_query_test.go",
		"TestProductServiceCreateRejectsParentCategoryWithChildren":           "product_create_test.go",
		"TestProductServiceCreateFiltersUnavailablePaymentChannels":           "product_create_test.go",
		"TestProductServiceCreateRejectsInvalidPurchaseLimits":                "product_create_test.go",
		"TestProductServiceUpdateKeepsMappedProductFulfillmentUpstream":       "product_update_test.go",
		"TestProductServiceUpdateFiltersUnavailablePaymentChannels":           "product_update_test.go",
		"TestProductServiceUpdateRejectsInvalidPurchaseLimits":                "product_update_test.go",
		"TestProductServiceQuickUpdateAllowsMappedActivationWithoutCategory":  "product_admin_test.go",
		"TestProductServiceQuickUpdateRejectsManualActivationWithoutCategory": "product_admin_test.go",
		"TestProductServiceDeleteCascade":                                     "product_admin_test.go",
		"TestProductServiceDeleteRollsBackCascadeWhenProductDeleteFails":      "product_admin_test.go",
		"TestProductServiceUpdateWholesalePricesOptionalSemantics":            "product_wholesale_test.go",
		"TestProductServiceUpdateWholesalePricesOnlyTouchesWholesaleField":    "product_wholesale_test.go",
		"TestProductServiceUpdateWholesalePricesClearsTiers":                  "product_wholesale_test.go",
		"TestProductServiceUpdateWholesalePricesRejectsInvalidInputs":         "product_wholesale_test.go",
		"TestProductServiceUpdateWholesalePricesValidatesSKUBelonging":        "product_wholesale_test.go",
		"TestProductServiceUpdateWholesalePricesReturnsNotFound":              "product_wholesale_test.go",
	}

	actualOwners := make(map[string][]string, len(expectedOwner))
	for _, file := range []string{
		"product_sku_test.go",
		"product_stock_test.go",
		"product_query_test.go",
		"product_create_test.go",
		"product_update_test.go",
		"product_admin_test.go",
		"product_wholesale_test.go",
	} {
		parsed := parseProductionGoFile(t, filepath.Join(testDirectory, file))
		for _, function := range declaredFunctionNames(parsed) {
			if _, tracked := expectedOwner[function]; tracked {
				actualOwners[function] = append(actualOwners[function], file)
			}
		}
	}

	for function, wantFile := range expectedOwner {
		gotFiles := actualOwners[function]
		if len(gotFiles) != 1 || gotFiles[0] != wantFile {
			t.Errorf("%s ownership mismatch: want [%s], got %v", function, wantFile, gotFiles)
		}
	}
}

func declaredFunctionNames(parsed *ast.File) []string {
	functions := make([]string, 0)
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		functions = append(functions, function.Name.Name)
	}
	return functions
}

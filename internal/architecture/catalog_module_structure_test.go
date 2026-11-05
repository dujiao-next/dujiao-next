package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestCatalogStockConsumersUseSharedPolicy(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	consumers := []string{
		"internal/modules/catalog/product/transport/http/public_view.go",
		"internal/modules/catalog/product/transport/http/public_stock.go",
		"internal/modules/channelapi/transport/http/channel_catalog.go",
		"internal/modules/upstreamapi/transport/http/upstream_catalog.go",
	}
	forbiddenFunctions := map[string]struct{}{
		"normalizePublicStockDisplayMode":    {},
		"buildPublicStockDisplay":            {},
		"normalizePublicStockStatus":         {},
		"publicStockRange":                   {},
		"maskPublicStockInt":                 {},
		"maskPublicStockInt64":               {},
		"maskPublicStockSold":                {},
		"computeStockCount":                  {},
		"normalizeChannelStockDisplayMode":   {},
		"buildChannelStockDisplay":           {},
		"normalizeChannelStockDisplayStatus": {},
		"channelStockRange":                  {},
		"computeStockStatus":                 {},
	}

	for _, relativePath := range consumers {
		relativePath := relativePath
		t.Run(filepath.Base(relativePath), func(t *testing.T) {
			path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", relativePath, err)
			}

			importsCatalog := false
			for _, imported := range parsed.Imports {
				importPath, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					t.Fatalf("unquote import in %s: %v", relativePath, err)
				}
				if importPath == moduleImportPath+"/internal/modules/catalog" {
					importsCatalog = true
				}
			}
			if !importsCatalog {
				t.Errorf("%s must consume the shared catalog stock policy", relativePath)
			}

			for _, declaration := range parsed.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if _, forbidden := forbiddenFunctions[function.Name.Name]; forbidden {
					t.Errorf("%s redeclares legacy stock policy helper %s", relativePath, function.Name.Name)
				}
			}
		})
	}
}

func TestCatalogCategoryImplementationLivesInBoundedContextDirectories(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	catalogRoot := filepath.Join(repositoryRoot, "internal", "modules", "catalog")
	categoryRoot := filepath.Join(catalogRoot, "category")
	domainRoot := filepath.Join(categoryRoot, "domain")
	contractRoot := filepath.Join(categoryRoot, "contract")
	applicationRoot := filepath.Join(categoryRoot, "application")
	storeRoot := filepath.Join(categoryRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(categoryRoot, "transport", "http")
	presenterRoot := filepath.Join(categoryRoot, "transport", "presenter")
	integrationRoot := filepath.Join(categoryRoot, "integrationtest")

	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "category.go"), []string{"Category"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "repository.go"), []string{"Repository"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service", "UpsertInput"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"CategoryStore"})
	assertFileDeclaresFunctions(t, filepath.Join(storeRoot, "store.go"), []string{"NewCategoryStore"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"CategoryService", "AdminCategoryHandler", "CreateCategoryRequest", "PatchCategoryActiveRequest",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{
		"RegisterPublicRoutes", "RegisterAdminRoutes",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "public_handler.go"), []string{
		"PublicQueries", "PublicHandler",
	})
	assertFileDeclaresTypes(t, filepath.Join(presenterRoot, "category.go"), []string{"Category"})
	assertFileDeclaresFunctions(t, filepath.Join(presenterRoot, "category.go"), []string{"New", "List"})

	assertDirectoryGoFileBudget(t, catalogRoot, 2)
	assertDirectoryGoFileBudget(t, domainRoot, 1)
	assertDirectoryGoFileBudget(t, contractRoot, 1)
	assertDirectoryGoFileBudget(t, applicationRoot, 1)
	assertDirectoryGoFileBudget(t, storeRoot, 2)
	assertDirectoryGoFileBudget(t, transportRoot, 3)
	assertDirectoryGoFileBudget(t, presenterRoot, 2)
	assertDirectoryGoFileBudget(t, integrationRoot, 1)

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "catalog.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "catalog_view.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "catalog_stock.go"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy public catalog handler must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy public catalog handler: %v", err)
		}
	}
}

func TestCatalogCategoryLegacyFlatFilesStayRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	patterns := []string{
		filepath.Join(repositoryRoot, "internal", "models", "category.go"),
		filepath.Join(repositoryRoot, "internal", "modules", "catalog", "category_service.go"),
		filepath.Join(repositoryRoot, "internal", "modules", "catalog", "store", "gormstore", "category_store*.go"),
		filepath.Join(repositoryRoot, "internal", "transport", "http", "catalog", "admin_category_handler.go"),
		filepath.Join(repositoryRoot, "internal", "integration", "catalog", "category_service_test.go"),
		filepath.Join(repositoryRoot, "internal", "service", "category_service*.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "category_repository*.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_category*.go"),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		if len(matches) != 0 {
			t.Errorf("legacy Catalog category files must stay removed: %v", matches)
		}
	}
}

func TestCatalogProductImplementationLivesInNestedBoundedContext(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	productRoot := filepath.Join(repositoryRoot, "internal", "modules", "catalog", "product")
	contractRoot := filepath.Join(productRoot, "contract")
	applicationRoot := filepath.Join(productRoot, "application")
	adminRoot := filepath.Join(applicationRoot, "admin")
	writeRoot := filepath.Join(applicationRoot, "write")
	domainRoot := filepath.Join(productRoot, "domain")
	manualFormRoot := filepath.Join(productRoot, "manualform")
	storeRoot := filepath.Join(productRoot, "store", "gormstore")
	transportRoot := filepath.Join(productRoot, "transport", "http")
	presenterRoot := filepath.Join(productRoot, "transport", "presenter")
	integrationRoot := filepath.Join(productRoot, "integrationtest")
	integrationApplicationRoot := filepath.Join(integrationRoot, "application")
	integrationTransportRoot := filepath.Join(integrationRoot, "transport")
	sharedGORMRoot := filepath.Join(repositoryRoot, "internal", "persistence", "gormutil")

	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "repository.go"), []string{
		"ListFilter", "Repository", "SKURepository",
	})
	if _, err := os.Stat(filepath.Join(contractRoot, "errors.go")); err != nil {
		t.Fatalf("catalog product contract errors.go must exist: %v", err)
	}
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "query.go"), []string{
		"ProductRepository", "CategoryRepository", "HiddenProductRepository", "StockCounter",
		"Options", "Service",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "query.go"), []string{
		"NewService", "ListPublic", "ListPublicForTenant", "ListForUpstreamSync", "ListPublicExact",
		"GetPublicBySlug", "GetPublicBySlugForTenant", "ListAdmin", "GetAdminByID",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "stock.go"), []string{
		"ApplyAutoStockCounts", "resolveLegacyStockTargetSKUIndex",
	})
	assertFileDeclaresTypes(t, filepath.Join(adminRoot, "service.go"), []string{
		"ProductRepository", "CategoryRepository", "CardSecretStockRepository", "OrderHistoryRepository",
		"ProductDeleteRepository", "CardSecretDeleteRepository", "CardSecretBatchDeleteRepository",
		"SKUDeleteRepository", "MemberLevelPriceDeleteRepository", "CartDeleteRepository",
		"ProductMappingDeleteRepository", "DeleteRepositories", "UnitOfWork", "Options", "AdminService",
	})
	assertFileDeclaresFunctions(t, filepath.Join(adminRoot, "service.go"), []string{"NewAdminService"})
	assertFileDeclaresFunctions(t, filepath.Join(adminRoot, "operations.go"), []string{
		"Delete", "QuickUpdate", "UpdateWholesalePrices", "isActivatingProduct",
		"categoryIDFromValue", "validateActivationCategory",
	})
	assertFileDeclaresFunctions(t, filepath.Join(adminRoot, "operations_test.go"), []string{
		"TestAdminServiceDeleteStopsBeforeTransactionWhenStockExists",
		"TestAdminServiceDeleteUsesAllCascadePorts",
		"TestAdminServiceQuickUpdateValidatesActivationCategory",
		"TestAdminServiceUpdateWholesalePricesCanonicalizesSKU",
	})
	assertFileDeclaresTypes(t, filepath.Join(writeRoot, "service.go"), []string{
		"ProductRepository", "SKURepository", "CategoryRepository", "PaymentChannelStoresitory",
		"CardSecretStockRepository", "TransactionRepositories", "UnitOfWork", "Options",
		"WriteService", "CreateProductInput", "ProductSKUInput",
	})
	assertFileDeclaresFunctions(t, filepath.Join(writeRoot, "service.go"), []string{"NewWriteService"})
	assertFileDeclaresFunctions(t, filepath.Join(writeRoot, "create.go"), []string{"Create"})
	assertFileDeclaresFunctions(t, filepath.Join(writeRoot, "update.go"), []string{"Update"})
	assertFileDeclaresFunctions(t, filepath.Join(writeRoot, "skus.go"), []string{
		"syncSingleProductSKU", "pickSingleModeTargetSKUIndex", "normalizeProductSKUInputs",
		"minActiveCostPrice", "applyProductSKUsWithStockGuard", "ensureAutoSKUCardSecretStockSafe",
	})
	assertFileDeclaresFunctions(t, filepath.Join(writeRoot, "skus_test.go"), []string{
		"TestSyncSingleProductSKUMultipleRowsKeepsSingleActive",
		"TestSyncSingleProductSKUNoActivePrefersDefaultCode",
	})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "pricing.go"), []string{"WholesalePriceInput"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "product.go"), []string{"Product"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "sku.go"), []string{"ProductSKU"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "wholesale_price.go"), []string{
		"WholesalePriceTier", "WholesalePriceTiers",
	})
	assertFileDeclaresFunctions(t, filepath.Join(domainRoot, "pricing.go"), []string{
		"NormalizeWholesalePrices", "NormalizeWholesalePricesForSKUs",
		"ResolveWholesaleUnitPrice", "ResolveWholesaleUnitPriceForSKU",
	})
	assertFileDeclaresFunctions(t, filepath.Join(domainRoot, "purchase_policy.go"), []string{
		"NormalizePurchaseType", "NormalizeFulfillmentType", "NormalizeStockDisplayMode",
		"ValidateCategoryAssignment", "NormalizePurchaseQuantityLimit", "ValidatePurchaseQuantity",
	})
	assertFileDeclaresFunctions(t, filepath.Join(domainRoot, "inventory_policy.go"), []string{
		"ManualSKUAvailable", "ShouldEnforceManualSKUStock",
	})
	assertFileDeclaresFunctions(t, filepath.Join(domainRoot, "payment_channels.go"), []string{
		"DecodePaymentChannelIDs", "EncodePaymentChannelIDs",
	})
	assertFileDeclaresFunctions(t, filepath.Join(manualFormRoot, "schema.go"), []string{
		"ValidateAndNormalize", "NormalizeSchema",
	})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "product_store.go"), []string{"ProductStore"})
	assertFileDeclaresFunctions(t, filepath.Join(storeRoot, "product_store.go"), []string{"NewProductStore"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "sku_store.go"), []string{"SKUStore"})
	assertFileDeclaresFunctions(t, filepath.Join(storeRoot, "sku_store.go"), []string{"NewSKUStore"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"ProductQueries", "ProductWriter", "ProductAdminCommands", "AdminProductHandler",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "public_handler.go"), []string{
		"PublicProductQueries", "PublicHandler",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{
		"RegisterPublicRoutes", "RegisterAdminRoutes",
	})
	assertFileDeclaresTypes(t, filepath.Join(presenterRoot, "product.go"), []string{
		"Product", "RelatedPost", "WholesalePrice", "SKU", "PromotionRule", "MemberLevelPrice",
	})
	assertFileDeclaresFunctions(t, filepath.Join(presenterRoot, "product.go"), []string{"WholesalePrices"})

	assertDirectoryGoFileBudget(t, productRoot, 0)
	assertDirectoryGoFileBudget(t, contractRoot, 2)
	assertDirectoryGoFileBudget(t, applicationRoot, 4)
	assertDirectoryGoFileBudget(t, adminRoot, 4)
	assertDirectoryGoFileBudget(t, writeRoot, 6)
	assertDirectoryGoFileBudget(t, domainRoot, 9)
	assertDirectoryGoFileBudget(t, manualFormRoot, 2)
	assertDirectoryGoFileBudget(t, storeRoot, 4)
	assertDirectoryGoFileBudget(t, transportRoot, 8)
	assertDirectoryGoFileBudget(t, presenterRoot, 1)
	assertDirectoryGoFileBudget(t, integrationRoot, 0)
	assertDirectoryGoFileBudget(t, integrationApplicationRoot, 9)
	assertDirectoryGoFileBudget(t, integrationTransportRoot, 2)
	assertDirectoryGoFileBudget(t, sharedGORMRoot, 3)
}

func TestCatalogMappingImplementationLivesInBoundedContext(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	mappingRoot := filepath.Join(repositoryRoot, "internal", "modules", "catalog", "mapping")
	domainRoot := filepath.Join(mappingRoot, "domain")
	contractRoot := filepath.Join(mappingRoot, "contract")
	applicationRoot := filepath.Join(mappingRoot, "application")
	storeRoot := filepath.Join(mappingRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(mappingRoot, "transport", "http")
	integrationRoot := filepath.Join(mappingRoot, "integrationtest")

	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "mapping.go"), []string{"Mapping", "SKUMapping"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "repository.go"), []string{
		"ListFilter", "MappingRepository", "SKUMappingRepository", "ProductRepository", "SKURepository",
		"CategoryRepository",
		"ImportTxProductRepository", "ImportTxSKURepository", "ImportTxMappingRepository", "ImportTxSKUMappingRepository",
		"ImportRepositories", "UnitOfWork",
	})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{
		"ConnectionProvider", "MediaRecorder", "CategoryCreator", "SettingsProvider",
	})
	if _, err := os.Stat(filepath.Join(contractRoot, "errors.go")); err != nil {
		t.Fatalf("catalog mapping contract errors.go must exist: %v", err)
	}
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Options", "Service"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{
		"NewService", "SetCategoryCreator", "SetSettings",
		"GetByID", "List", "SetActive", "Delete", "GetSKUMappings", "GetMappedUpstreamIDs",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "import.go"), []string{
		"ImportUpstreamProduct", "ImportUpstreamProductWithAutoCategory", "importUpstreamProduct",
		"createSKUMappings", "ListUpstreamProducts", "ListUpstreamCategories",
	})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "batch_import.go"), []string{
		"BatchUpstreamProductImportOutcome", "BatchImportByCategoryResult",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "batch_import.go"), []string{
		"BatchImportUpstreamProducts", "BatchImportByCategory",
		"findOrCreateCategoryFromUpstream", "findOrCreateLocalCategory",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "sync.go"), []string{
		"SyncProduct", "SyncAllStock", "SyncConnectionStock", "EnsureUpstreamStockForOrder",
		"markUpstreamUnavailable", "computeFullSyncInterval",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "markup.go"), []string{
		"ReapplyMarkup", "recalcProductPrice",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "pricing.go"), []string{
		"CalculateLocalPrice", "CalculateMarkedUpPrice", "convertCurrency",
	})

	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "mapping_store.go"), []string{"MappingStore"})
	assertFileDeclaresFunctions(t, filepath.Join(storeRoot, "mapping_store.go"), []string{"NewMappingStore"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "sku_mapping_store.go"), []string{"SKUMappingStore"})
	assertFileDeclaresFunctions(t, filepath.Join(storeRoot, "sku_mapping_store.go"), []string{"NewSKUMappingStore"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{"ProductMappingService", "AdminHandler"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_handler.go"), []string{"NewAdminHandler"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})

	assertDirectoryGoFileBudget(t, mappingRoot, 0)
	assertDirectoryGoFileBudget(t, domainRoot, 1)
	assertDirectoryGoFileBudget(t, contractRoot, 3)
	assertDirectoryGoFileBudget(t, applicationRoot, 11)
	assertDirectoryGoFileBudget(t, storeRoot, 4)
	assertDirectoryGoFileBudget(t, transportRoot, 2)
	assertDirectoryGoFileBudget(t, integrationRoot, 1)
}

func TestCatalogMappingLegacyRepositoryFilesStayRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	legacyFiles := []string{
		"internal/repository/product_mapping_repository.go",
		"internal/repository/product_mapping_repository_test.go",
		"internal/repository/sku_mapping_repository.go",
		"internal/repository/sku_mapping_repository_test.go",
	}
	for _, relativePath := range legacyFiles {
		if matches, err := filepath.Glob(filepath.Join(repositoryRoot, relativePath)); err != nil {
			t.Fatalf("glob %s: %v", relativePath, err)
		} else if len(matches) != 0 {
			t.Errorf("legacy Catalog mapping repository file must stay removed: %v", matches)
		}
	}
}

func TestCatalogMappingLegacyFlatFilesStayRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	legacyFiles := []string{
		"internal/service/product_mapping_import.go",
		"internal/service/product_mapping_batch_import.go",
		"internal/service/product_mapping_sync.go",
		"internal/service/product_mapping_markup.go",
		"internal/service/price_markup.go",
		"internal/service/price_markup_test.go",
	}
	for _, relativePath := range legacyFiles {
		if matches, err := filepath.Glob(filepath.Join(repositoryRoot, relativePath)); err != nil {
			t.Fatalf("glob %s: %v", relativePath, err)
		} else if len(matches) != 0 {
			t.Errorf("legacy Catalog mapping flat file must stay removed: %v", matches)
		}
	}
}

func TestCatalogProductLegacyRepositoryFilesStayRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	legacyFiles := []string{
		"internal/repository/product_repository.go",
		"internal/repository/product_repository_test.go",
		"internal/repository/product_sku_repository.go",
		"internal/repository/product_sku_repository_test.go",
	}
	for _, relativePath := range legacyFiles {
		if matches, err := filepath.Glob(filepath.Join(repositoryRoot, relativePath)); err != nil {
			t.Fatalf("glob %s: %v", relativePath, err)
		} else if len(matches) != 0 {
			t.Errorf("legacy Catalog Product repository file must stay removed: %v", matches)
		}
	}
}

func TestCatalogProductLegacyFlatFilesStayRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	legacyFiles := []string{
		"internal/models/product.go",
		"internal/models/product_sku.go",
		"internal/models/wholesale_price.go",
		"internal/modules/catalog/product/errors.go",
		"internal/modules/catalog/product/ports.go",
		"internal/dto/product.go",
		"internal/transport/http/catalog/admin_product_handler.go",
		"internal/transport/http/catalog/admin_product_handler_integration_test.go",
		"internal/transport/http/catalog/public_handler.go",
		"internal/transport/http/catalog/public_view.go",
		"internal/transport/http/catalog/public_stock.go",
		"internal/transport/http/catalog/public_price_integration_test.go",
		"internal/transport/http/catalog/public_price_test.go",
		"internal/transport/http/catalog/public_related_posts_test.go",
		"internal/transport/http/catalog/public_stock_test.go",
		"internal/service/product_create.go",
		"internal/service/product_admin.go",
		"internal/service/product_query.go",
		"internal/service/product_rules.go",
		"internal/service/product_sku.go",
		"internal/service/product_stock.go",
		"internal/service/product_update.go",
		"internal/service/product_wholesale.go",
		"internal/service/product_purchase_limit.go",
		"internal/service/sku_stock_policy.go",
		"internal/service/manual_form_validator.go",
		"internal/service/manual_form_validator_test.go",
		"internal/http/handlers/admin/admin_product.go",
		"internal/http/handlers/admin/admin_product_test.go",
		"internal/http/handlers/admin/admin_product_mapping.go",
		"internal/http/handlers/admin/admin_product_mapping_test.go",
		"internal/transport/http/catalog/admin_product_mapping_handler.go",
		"internal/transport/http/catalog/admin_product_mapping_handler_integration_test.go",
		"internal/transport/http/catalog/routes.go",
	}
	for _, relativePath := range legacyFiles {
		if matches, err := filepath.Glob(filepath.Join(repositoryRoot, relativePath)); err != nil {
			t.Fatalf("glob %s: %v", relativePath, err)
		} else if len(matches) != 0 {
			t.Errorf("legacy Catalog Product flat file must stay removed: %v", matches)
		}
	}
}

package architecture

import (
	"go/ast"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type packageFileBudget struct {
	production int
	total      int
}

// Test-only architecture assertions intentionally share one package so they can
// reuse AST helpers. Production packages have no file-budget exceptions.
var packageFileBudgetOverrides = map[string]packageFileBudget{
	"internal/architecture": {production: 0, total: 51},
}

// completedMigrationPaths are deleted compatibility-free entry points. Once a
// bounded context reaches this list, recreating its former horizontal package
// is an architecture regression rather than an allowed transitional change.
var completedMigrationPaths = []string{
	"internal/dto",
	"internal/http",
	"internal/integration",
	"internal/models",
	"internal/provider",
	"internal/repository",
	"internal/router",
	"internal/service",
	"internal/transport",
	"internal/wiring",
	"internal/worker",
	"internal/payment",
	"internal/persistence/reseller",
	"internal/modules/reseller/accounting_balance.go",
	"internal/modules/reseller/accounting_profit.go",
	"internal/modules/reseller/accounting_query.go",
	"internal/modules/reseller/accounting_query_test.go",
	"internal/modules/reseller/accounting_refund.go",
	"internal/modules/reseller/accounting_types.go",
	"internal/modules/reseller/accounting_withdraw.go",
	"internal/modules/reseller/accounting_withdraw_test.go",
	"internal/modules/reseller/domain_resolver.go",
	"internal/modules/reseller/domain_resolver_test.go",
	"internal/modules/reseller/errors.go",
	"internal/modules/reseller/management.go",
	"internal/modules/reseller/operations.go",
	"internal/modules/reseller/operations_test.go",
	"internal/modules/reseller/operations_types.go",
	"internal/modules/reseller/order_query.go",
	"internal/modules/reseller/order_query_test.go",
	"internal/modules/reseller/order_types.go",
	"internal/modules/reseller/ports.go",
	"internal/modules/reseller/pricing_rules.go",
	"internal/modules/reseller/pricing_types.go",
	"internal/modules/reseller/pricing_types_test.go",
	"internal/modules/reseller/product_setting.go",
	"internal/modules/reseller/site_config.go",
	"internal/modules/reseller/tenant.go",
	"internal/modules/reseller/tenant_test.go",
	"internal/modules/wallet/errors.go",
	"internal/modules/wallet/ports.go",
	"internal/modules/wallet/query.go",
	"internal/modules/wallet/store",
	"internal/modules/procurement/service.go",
	"internal/modules/procurement/create.go",
	"internal/modules/procurement/submit.go",
	"internal/modules/procurement/callback.go",
	"internal/modules/procurement/poll.go",
	"internal/modules/procurement/query.go",
	"internal/modules/procurement/manual.go",
	"internal/modules/procurement/rules_test.go",
	"internal/modules/procurement/store",
	"internal/modules/procurement/infrastructure/orderlifecycle",
	"internal/modules/reconciliation/service.go",
	"internal/modules/reconciliation/execute.go",
	"internal/modules/reconciliation/query.go",
	"internal/modules/reconciliation/service_test.go",
	"internal/modules/reconciliation/store",
	"internal/modules/reporting/window.go",
	"internal/modules/reporting/window_test.go",
	"internal/modules/orderrisk/service.go",
	"internal/modules/orderrisk/service_test.go",
	"internal/modules/affiliate/errors.go",
	"internal/modules/affiliate/types.go",
	"internal/modules/catalog/category_service.go",
	"internal/modules/catalog/store/gormstore/category_store.go",
	"internal/modules/catalog/store/gormstore/category_store_test.go",
	"internal/modules/catalog/product/errors.go",
	"internal/modules/catalog/product/ports.go",
	"internal/modules/siteconnection/errors.go",
	"internal/modules/siteconnection/service.go",
	"internal/modules/siteconnection/types.go",
	"internal/modules/catalog/mapping/batch_import.go",
	"internal/modules/catalog/mapping/batch_import_test.go",
	"internal/modules/catalog/mapping/import.go",
	"internal/modules/catalog/mapping/import_test.go",
	"internal/modules/catalog/mapping/markup.go",
	"internal/modules/catalog/mapping/pricing.go",
	"internal/modules/catalog/mapping/pricing_test.go",
	"internal/modules/catalog/mapping/service.go",
	"internal/modules/catalog/mapping/service_test.go",
	"internal/modules/catalog/mapping/sync.go",
	"internal/modules/catalog/mapping/sync_test.go",
	"internal/modules/catalog/mapping/store",
	"internal/modules/memberlevel/ports.go",
	"internal/modules/memberlevel/service.go",
	"internal/modules/memberlevel/store",
	"internal/modules/promotion/admin_service.go",
	"internal/modules/promotion/errors.go",
	"internal/modules/promotion/ports.go",
	"internal/modules/promotion/service.go",
	"internal/modules/promotion/store",
	"internal/modules/coupon/admin_service.go",
	"internal/modules/coupon/errors.go",
	"internal/modules/coupon/ports.go",
	"internal/modules/coupon/service.go",
	"internal/modules/coupon/store",
	"internal/modules/giftcard/errors.go",
	"internal/modules/giftcard/export.go",
	"internal/modules/giftcard/generate.go",
	"internal/modules/giftcard/helpers.go",
	"internal/modules/giftcard/helpers_test.go",
	"internal/modules/giftcard/manage.go",
	"internal/modules/giftcard/ports.go",
	"internal/modules/giftcard/service.go",
	"internal/modules/giftcard/types.go",
	"internal/modules/giftcard/store",
	"internal/modules/apicredential/ports.go",
	"internal/modules/apicredential/service.go",
	"internal/modules/apicredential/store",
	"internal/modules/auditlog/authz_service.go",
	"internal/modules/auditlog/ports.go",
	"internal/modules/auditlog/service_test.go",
	"internal/modules/auditlog/store",
	"internal/modules/auditlog/user_login_service.go",
	"internal/modules/notification/alert.go",
	"internal/modules/notification/dedupe.go",
	"internal/modules/notification/errors.go",
	"internal/modules/notification/inventory_alert.go",
	"internal/modules/notification/log_service.go",
	"internal/modules/notification/log_service_test.go",
	"internal/modules/notification/order_format.go",
	"internal/modules/notification/payment_order_alert.go",
	"internal/modules/notification/ports.go",
	"internal/modules/notification/send.go",
	"internal/modules/notification/service.go",
	"internal/modules/notification/store",
	"internal/modules/notification/template.go",
	"internal/modules/notification/test_variables.go",
	"internal/modules/content/banner_service.go",
	"internal/modules/content/errors.go",
	"internal/modules/content/media_service.go",
	"internal/modules/content/ports.go",
	"internal/modules/content/post_category_service.go",
	"internal/modules/content/post_service.go",
	"internal/modules/content/query.go",
	"internal/modules/content/service_test.go",
	"internal/modules/content/system.go",
	"internal/modules/content/store",
	"internal/modules/content/filestore",
	"internal/modules/dashboard/errors.go",
	"internal/modules/dashboard/ports.go",
	"internal/modules/dashboard/service.go",
	"internal/modules/dashboard/service_test.go",
	"internal/modules/dashboard/types.go",
	"internal/modules/dashboard/store",
	"internal/modules/cart/service.go",
	"internal/adgateway",
	"internal/modules/adproxy/service.go",
	"internal/modules/adproxy/types.go",
	"internal/modules/compliance/service.go",
	"internal/modules/compliance/types.go",
	"internal/modules/compliance/errors.go",
	"internal/modules/sitemap/service.go",
	"internal/modules/upload/service.go",
	"internal/modules/upload/service_test.go",
	"internal/modules/captcha/service.go",
	"internal/modules/captcha/errors.go",
	"internal/bootstrap/captchahttp",
	"internal/modules/telegram/notify.go",
	"internal/modules/telegram/notify_test.go",
	"internal/modules/telegram/errors.go",
	"internal/modules/downstreamcallback/service.go",
	"internal/modules/cardsecret/service.go",
	"internal/modules/cardsecret/import.go",
	"internal/modules/cardsecret/manage.go",
	"internal/modules/cardsecret/export.go",
	"internal/modules/cardsecret/stats.go",
	"internal/modules/cardsecret/store",
}

func TestCompletedMigrationPathsStayDeleted(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	for _, relativePath := range completedMigrationPaths {
		t.Run(strings.ReplaceAll(relativePath, "/", "_"), func(t *testing.T) {
			absolutePath := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
			if _, err := os.Stat(absolutePath); err == nil {
				t.Fatalf("completed migration path was recreated: %s", relativePath)
			} else if !os.IsNotExist(err) {
				t.Fatalf("stat completed migration path %s: %v", relativePath, err)
			}
		})
	}
}

func TestSettingsModuleRootContainsNoGoFiles(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	settingsRoot := filepath.Join(repositoryRoot, "internal", "modules", "settings")
	production, total := countDirectGoFiles(t, settingsRoot)
	if production != 0 || total != 0 {
		t.Fatalf("settings module root must remain structural only, got production=%d total=%d", production, total)
	}
}

func TestSharedMoneyOwnsMonetaryValueObject(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moneyRoot := filepath.Join(repositoryRoot, "internal", "shared", "money")
	assertFileDeclaresTypes(t, filepath.Join(moneyRoot, "amount.go"), []string{"Amount"})
	assertFileDeclaresFunctions(t, filepath.Join(moneyRoot, "amount.go"), []string{"FromDecimal"})
	assertDirectoryGoFileBudget(t, moneyRoot, 2)
}

func TestSharedPasswordPolicyOwnsStrengthValidation(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	policyRoot := filepath.Join(repositoryRoot, "internal", "shared", "passwordpolicy")
	assertFileDeclaresTypes(t, filepath.Join(policyRoot, "policy.go"), []string{"Policy"})
	assertFileDeclaresFunctions(t, filepath.Join(policyRoot, "policy.go"), []string{"Validate"})
	assertDirectoryGoFileBudget(t, policyRoot, 2)
}

func TestAffiliateModuleOwnsDomainAndPersistence(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	domainRoot := filepath.Join(repositoryRoot, "internal", "modules", "affiliate", "domain")
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "profile.go"), []string{"Profile"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "click.go"), []string{"Click"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "commission.go"), []string{"Commission"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "withdraw_request.go"), []string{"WithdrawRequest"})
	assertDirectoryGoFileBudget(t, domainRoot, 5)

	storeRoot := filepath.Join(repositoryRoot, "internal", "modules", "affiliate", "infrastructure", "gormstore")
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertDirectoryGoFileBudget(t, storeRoot, 2)

	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "affiliate")
	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("affiliate module root must remain structural only, got production=%d total=%d", production, total)
	}
}

func TestDatabaseBootstrapIsSeparatedFromPlatformConnection(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	connectionRoot := filepath.Join(repositoryRoot, "internal", "platform", "database", "gormdb")
	migrationRoot := filepath.Join(repositoryRoot, "internal", "bootstrap", "database", "migrations")

	assertFileDeclaresTypes(t, filepath.Join(connectionRoot, "db.go"), []string{"DBPoolConfig"})
	assertFileDeclaresFunctions(t, filepath.Join(connectionRoot, "db.go"), []string{"InitDB"})
	assertFileDeclaresFunctions(t, filepath.Join(migrationRoot, "registry.go"), []string{"AutoMigrate"})
	assertDirectoryGoFileBudget(t, connectionRoot, 2)
	assertDirectoryGoFileBudget(t, migrationRoot, 4)
}

func TestNoNewCompatibilityOrLegacyProductionFiles(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	internalRoot := filepath.Join(repositoryRoot, "internal")

	err := filepath.WalkDir(internalRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		name := strings.ToLower(entry.Name())
		if !strings.Contains(name, "compat") && !strings.Contains(name, "legacy") && !strings.Contains(name, "facade") {
			return nil
		}

		relativePath, err := filepath.Rel(repositoryRoot, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		t.Errorf("compatibility or legacy production file is forbidden: %s", relativePath)
		return nil
	})
	if err != nil {
		t.Fatalf("inspect production Go files: %v", err)
	}
}

func TestProductionCodeContainsNoTypeAliases(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	internalRoot := filepath.Join(repositoryRoot, "internal")

	err := filepath.WalkDir(internalRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		parsed := parseProductionGoFile(t, path)
		ast.Inspect(parsed, func(node ast.Node) bool {
			typeSpec, ok := node.(*ast.TypeSpec)
			if ok && typeSpec.Assign.IsValid() {
				relativePath, err := filepath.Rel(repositoryRoot, path)
				if err != nil {
					t.Fatalf("resolve type alias path: %v", err)
				}
				t.Errorf(
					"production type alias %s is forbidden; depend on its owning package or declare an owned type: %s",
					typeSpec.Name.Name,
					filepath.ToSlash(relativePath),
				)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("inspect production Go aliases: %v", err)
	}
}

func TestGoPackagesStayWithinFileBudgets(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	internalRoot := filepath.Join(repositoryRoot, "internal")

	err := filepath.WalkDir(internalRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() || entry.Name() == "testdata" {
			return nil
		}

		production, total := countDirectGoFiles(t, path)
		if total == 0 {
			return nil
		}

		relativePath, err := filepath.Rel(repositoryRoot, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)

		budget := packageFileBudget{production: 12, total: 20}
		if override, ok := packageFileBudgetOverrides[relativePath]; ok {
			budget = override
		}

		if production > budget.production || total > budget.total {
			t.Errorf(
				"Go package %s exceeds its migration budget: production=%d/%d total=%d/%d",
				relativePath,
				production,
				budget.production,
				total,
				budget.total,
			)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect Go package budgets: %v", err)
	}
}

func countDirectGoFiles(t *testing.T, directory string) (production int, total int) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read %s: %v", directory, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		total++
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			production++
		}
	}
	return production, total
}

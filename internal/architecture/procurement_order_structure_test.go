package architecture

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestProcurementUsesCompleteVerticalLayout(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "procurement")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	domainRoot := filepath.Join(moduleRoot, "domain")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	orderReaderRoot := filepath.Join(moduleRoot, "infrastructure", "orderreader")
	mappingReaderRoot := filepath.Join(moduleRoot, "infrastructure", "mappingreader")
	upstreamGatewayRoot := filepath.Join(moduleRoot, "infrastructure", "upstreamgateway")
	queueAdapterRoot := filepath.Join(moduleRoot, "infrastructure", "queueadapter")
	notificationAdapterRoot := filepath.Join(moduleRoot, "infrastructure", "notificationadapter")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")
	integrationTestRoot := filepath.Join(moduleRoot, "integrationtest")

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("procurement module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 8)
	assertDirectoryGoFileBudget(t, contractRoot, 3)
	assertDirectoryGoFileBudget(t, domainRoot, 1)
	assertDirectoryGoFileBudget(t, storeRoot, 2)
	assertDirectoryGoFileBudget(t, orderReaderRoot, 1)
	assertDirectoryGoFileBudget(t, mappingReaderRoot, 1)
	assertDirectoryGoFileBudget(t, upstreamGatewayRoot, 1)
	assertDirectoryGoFileBudget(t, queueAdapterRoot, 1)
	assertDirectoryGoFileBudget(t, notificationAdapterRoot, 1)
	assertDirectoryGoFileBudget(t, transportRoot, 2)
	assertDirectoryGoFileBudget(t, integrationTestRoot, 6)

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service", "Options"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{
		"Repository", "OrderRepository", "ProductMappingReader", "SKUMappingReader", "ConnectionProvider",
		"Enqueuer", "OrderLifecycle", "DownstreamCallbackEnqueuer", "BotFulfillmentNotifier", "FailureNotifier",
	})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "types.go"), []string{
		"ListFilter", "Fulfillment", "CreateOrderRequest", "CreateOrderResult", "UpstreamOrder", "UpstreamConnection", "UseCase",
	})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "order.go"), []string{"Order", "LocalOrder", "LocalOrderItem"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertFileDeclaresTypes(t, filepath.Join(orderReaderRoot, "reader.go"), []string{"Reader"})
	assertFileDeclaresTypes(t, filepath.Join(upstreamGatewayRoot, "gateway.go"), []string{"Gateway"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "lifecycle.go"), []string{"Lifecycle"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{"AdminHandler", "Service"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})

	expected := map[string][]string{
		"service.go": {"NewService"},
		"create.go":  {"CreateForOrder", "createProcurementForSingleOrder", "hasUpstreamItems"},
		"submit.go": {
			"SubmitToUpstream", "markProcurementError", "rejectProcurement",
			"rollbackLocalOrderOnProcurementFailure", "notifyProcurementFailure", "handleSubmitFailure",
			"isRetryableErrorCode", "parseRetryIntervals", "recordUpstreamOrderAfterStatusChanged",
		},
		"callback.go": {"HandleUpstreamCallback", "createUpstreamFulfillment", "isUpstreamTransitionAllowed"},
		"poll.go":     {"PollUpstreamStatus", "requeuePoll", "SyncAcceptedOrders", "syncAcceptedOrder", "requeueStalledSubmissions", "mapProcurementUpstreamStatus"},
		"query.go": {
			"GetByID", "GetByLocalOrderNo", "List", "StatsByStatus", "FillParentOrderNo", "fillParentOrderNos",
			"applyProcurementLocalRefundedAmountFallback", "shouldSyncUpstreamRefundStatus",
			"normalizeProcurementUpstreamStatus", "buildUpstreamRefundRecords", "parseUpstreamRefundRecordCreatedAt",
			"fillUpstreamRefundRecordsForProcurementOrder", "isPositiveUpstreamRefundAmount",
			"fillUpstreamRefundRecordsForProcurementOrders",
		},
		"manual.go": {"RetryManual", "CancelManual"},
	}
	for file, want := range expected {
		parsed := parseProductionGoFile(t, filepath.Join(applicationRoot, file))
		got := declaredFunctionNames(parsed)
		sort.Strings(want)
		sort.Strings(got)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s function ownership mismatch\nwant: %v\ngot:  %v", file, want, got)
		}
	}

	for _, forbiddenImport := range []string{
		moduleImportPath + "/internal/models",
		moduleImportPath + "/internal/queue",
		moduleImportPath + "/internal/upstream",
		moduleImportPath + "/internal/repository",
		moduleImportPath + "/internal/modules/siteconnection",
		moduleImportPath + "/internal/modules/notification",
		moduleImportPath + "/internal/modules/catalog/mapping",
		"github.com/gin-gonic/gin",
		"gorm.io/gorm",
	} {
		assertProductionImportsAbsent(t, applicationRoot, forbiddenImport)
	}
	for _, root := range []string{contractRoot, domainRoot} {
		assertProductionImportsAbsent(t, root, moduleImportPath+"/internal/models")
		assertProductionImportsAbsent(t, root, moduleImportPath+"/internal/repository")
		assertProductionImportsAbsent(t, root, "github.com/gin-gonic/gin")
		assertProductionImportsAbsent(t, root, "gorm.io/gorm")
	}
	assertProductionImportsAbsent(t, transportRoot, moduleImportPath+"/internal/models")

	for _, relativePath := range legacyProcurementPaths() {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		if _, err := os.Stat(path); err == nil {
			t.Errorf("legacy procurement path must not return: %s", relativePath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", relativePath, err)
		}
	}
}

func legacyProcurementPaths() []string {
	return []string{
		"internal/models/procurement_order.go",
		"internal/repository/procurement_order_repository.go",
		"internal/http/handlers/admin/admin_procurement_order.go",
		"internal/service/procurement_order_service.go",
		"internal/service/procurement_order_create.go",
		"internal/service/procurement_order_submit.go",
		"internal/service/procurement_order_callback.go",
		"internal/service/procurement_order_poll.go",
		"internal/service/procurement_order_query.go",
		"internal/service/procurement_order_manual.go",
		"internal/service/procurement_order_lifecycle.go",
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
		"internal/transport/http/procurement",
		"internal/integration/procurement",
	}
}

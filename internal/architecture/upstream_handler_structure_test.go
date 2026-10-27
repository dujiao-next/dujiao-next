package architecture

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestUpstreamHandlerImplementationIsSplitByResource(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	handlerDirectory := filepath.Join(
		repositoryRoot,
		"internal", "modules", "upstreamapi", "transport", "http",
	)
	expected := map[string][]string{
		"upstream_handler.go": {
			"New", "getUpstreamUserID", "getUpstreamCredentialID", "successResponse", "errorResponse",
		},
		"upstream_ping.go": {"Ping"},
		"upstream_catalog.go": {
			"ListCategories", "ListProducts", "GetProduct", "applyUpstreamStockToProducts",
			"resolveEffectiveFulfillmentTypes", "toUpstreamProductWithMemberPrice", "resolveCatalogPrice", "computeSKUStock",
		},
		"upstream_order.go":    {"CreateOrder", "GetOrder", "CancelOrder", "mapOrderErrorToResponse"},
		"upstream_callback.go": {"HandleCallback", "Read", "mapCallbackStatus", "validateCallbackURL"},
	}

	for file, want := range expected {
		parsed := parseProductionGoFile(t, filepath.Join(handlerDirectory, file))
		got := declaredFunctionNames(parsed)
		sort.Strings(want)
		sort.Strings(got)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s function ownership mismatch\nwant: %v\ngot:  %v", file, want, got)
		}
	}
	assertDirectoryGoFileBudget(t, handlerDirectory, 8)
	legacyFiles, err := filepath.Glob(filepath.Join(repositoryRoot, "internal", "http", "handlers", "upstream", "*.go"))
	if err != nil {
		t.Fatalf("list legacy upstream handlers: %v", err)
	}
	if len(legacyFiles) != 0 {
		t.Fatalf("legacy upstream handlers must stay removed: %v", legacyFiles)
	}
	if _, err := os.Stat(filepath.Join(handlerDirectory, "routes.go")); err != nil {
		t.Fatalf("upstream transport routes missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repositoryRoot, "internal", "router", "upstream_adapter.go")); err == nil {
		t.Fatal("upstream composition adapters belong in internal/bootstrap/upstreamapi, not internal/router")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy upstream router adapter: %v", err)
	}
	wiringDirectory := filepath.Join(repositoryRoot, "internal", "bootstrap", "upstreamapi")
	if _, err := os.Stat(filepath.Join(wiringDirectory, "wiring.go")); err != nil {
		t.Fatalf("upstream wiring missing: %v", err)
	}
	assertDirectoryGoFileBudget(t, wiringDirectory, 3)
}

func TestUpstreamHandlerTypesLiveWithTheirResources(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	handlerDirectory := filepath.Join(
		repositoryRoot,
		"internal", "modules", "upstreamapi", "transport", "http",
	)
	expectedOwner := map[string]string{
		"Handler":            "upstream_handler.go",
		"upstreamCategory":   "upstream_catalog.go",
		"upstreamProduct":    "upstream_catalog.go",
		"upstreamSKU":        "upstream_catalog.go",
		"createOrderRequest": "upstream_order.go",
		"callbackPayload":    "upstream_callback.go",
		"bodyBuf":            "upstream_callback.go",
	}
	actualOwner := make(map[string][]string, len(expectedOwner))
	files := []string{
		"upstream_handler.go", "upstream_ping.go", "upstream_catalog.go",
		"upstream_order.go", "upstream_callback.go",
	}

	for _, file := range files {
		parsed := parseProductionGoFile(t, filepath.Join(handlerDirectory, file))
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

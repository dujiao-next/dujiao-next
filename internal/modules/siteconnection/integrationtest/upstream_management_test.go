package siteconnection_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	mappingstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	siteconnectionstore "github.com/dujiao-next/internal/modules/siteconnection/infrastructure/gormstore"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestUpstreamManagementKeepsLastSuccessfulBalanceAndCountsMappedProducts(t *testing.T) {
	var upstreamResponse atomic.Value
	upstreamResponse.Store(`{"ok":true,"balance":"25.75","currency":"CNY"}`)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/upstream/ping" {
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body := upstreamResponse.Load().(string)
		if body == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, body)
	}))
	defer upstream.Close()

	db, err := gorm.Open(sqlite.Open("file:upstream_management?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&siteconnectiondomain.Connection{}, &siteconnectiondomain.BalanceSnapshot{}); err != nil {
		t.Fatal(err)
	}
	store := siteconnectionstore.New(db)
	connections := siteconnectionapp.NewService(store, "test-app-key", t.TempDir())
	conn, err := connections.Create(siteconnectionapp.CreateInput{
		Name: "Test supplier", BaseURL: upstream.URL, ApiKey: "test-key", ApiSecret: "test-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := connections.SetStatus(conn.ID, constants.ConnectionStatusActive); err != nil {
		t.Fatal(err)
	}
	synced := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{sql: "CREATE TABLE products (id INTEGER PRIMARY KEY, is_active BOOLEAN, deleted_at DATETIME)"},
		{sql: "CREATE TABLE product_mappings (id INTEGER PRIMARY KEY, connection_id INTEGER, local_product_id INTEGER, last_synced_at DATETIME, deleted_at DATETIME)"},
		{sql: "INSERT INTO products VALUES (1, true, NULL), (2, false, NULL), (3, true, ?) ", args: []any{synced}},
		{sql: "INSERT INTO product_mappings VALUES (1, ?, 1, ?, NULL), (2, ?, 2, NULL, NULL), (3, ?, 3, ?, NULL)",
			args: []any{conn.ID, synced, conn.ID, conn.ID, time.Now().UTC()}},
	} {
		if err := db.Exec(statement.sql, statement.args...).Error; err != nil {
			t.Fatal(err)
		}
	}

	management := siteconnectionapp.NewManagement(connections, store, mappingstore.NewMappingStore(db))
	first, err := management.Refresh(t.Context(), conn.ID)
	if err != nil || first.State != "ok" || first.Risk != "low" || first.Amount != "25.75" || first.Threshold != "50" {
		t.Fatalf("unexpected initial balance: %+v, %v", first, err)
	}
	configured, err := management.SetThreshold(conn.ID, "20", "CNY")
	if err != nil || configured.Risk != "normal" || configured.Threshold != "20" {
		t.Fatalf("unexpected threshold result: %+v, %v", configured, err)
	}
	expireCooldown := func() {
		t.Helper()
		if err := db.Model(&siteconnectiondomain.BalanceSnapshot{}).Where("connection_id = ?", conn.ID).
			Update("checked_at", time.Now().Add(-time.Minute)).Error; err != nil {
			t.Fatal(err)
		}
	}
	expireCooldown()
	upstreamResponse.Store("")
	failed, err := management.Refresh(t.Context(), conn.ID)
	if err != nil || failed.State != "error" || failed.Amount != first.Amount || failed.Threshold != "20" ||
		failed.LastSuccessAt == nil || !failed.LastSuccessAt.Equal(*first.LastSuccessAt) {
		t.Fatalf("failed query lost successful balance or threshold: %+v, %v", failed, err)
	}
	expireCooldown()
	upstreamResponse.Store(`{"ok":true,"balance":"0.00","currency":"CNY"}`)
	zero, err := management.Refresh(t.Context(), conn.ID)
	if err != nil || zero.State != "ok" || zero.Risk != "empty" || zero.Amount != "0.00" {
		t.Fatalf("zero balance was not distinguished from failure: %+v, %v", zero, err)
	}
	overview, err := management.Overview()
	if err != nil || len(overview) != 1 || overview[0].MappedProducts != 2 || overview[0].ListedProducts != 1 ||
		overview[0].LastSyncedAt == nil || !overview[0].LastSyncedAt.Equal(synced) {
		t.Fatalf("incorrect mapped/listed product counts: %+v, %v", overview, err)
	}
	if overview[0].Balance.Amount != zero.Amount {
		t.Fatalf("overview did not load the saved snapshot: %+v", overview[0].Balance)
	}
}

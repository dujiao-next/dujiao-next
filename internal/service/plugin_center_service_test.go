package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/pluginhost"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPluginCenterServiceForTest(t *testing.T) (*PluginCenterService, repository.PluginRepository, *SettingService) {
	t.Helper()

	dsn := fmt.Sprintf("file:plugin_center_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Setting{},
		&models.Plugin{},
		&models.PluginVersion{},
		&models.PluginRuntimeLog{},
		&models.PluginRouteRegistry{},
		&models.PluginPageRegistry{},
		&models.PluginEventSubscription{},
	); err != nil {
		t.Fatalf("auto migrate plugin center tables failed: %v", err)
	}

	pluginRepo := repository.NewPluginRepository(db)
	settingSvc := NewSettingService(repository.NewSettingRepository(db))
	svc := &PluginCenterService{
		repo:           pluginRepo,
		settingService: settingSvc,
		runtimeRoot:    t.TempDir(),
		marketRoot:     t.TempDir(),
		loaded:         make(map[string]*pluginLoadedRuntime),
		paymentDrivers: make(map[string]pluginhost.PaymentDriver),
		builtinFeeds:   make(map[string]func() (*MarketFeed, error)),
	}
	return svc, pluginRepo, settingSvc
}

func TestPluginCenterBuiltinOfficialFeedMergesStaticAndDynamicItems(t *testing.T) {
	svc, _, _ := newPluginCenterServiceForTest(t)

	staticFeed := &MarketFeed{
		Items: []MarketFeedItem{
			{PluginID: "demo-feature", Version: "1.0.0", Name: "演示功能插件"},
		},
	}
	content, err := json.MarshalIndent(staticFeed, "", "  ")
	if err != nil {
		t.Fatalf("marshal static feed failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(svc.marketRoot, "official.json"), content, 0o644); err != nil {
		t.Fatalf("write static official feed failed: %v", err)
	}

	svc.SetBuiltinFeedProvider("official", func() (*MarketFeed, error) {
		return &MarketFeed{
			Items: []MarketFeedItem{
				{PluginID: "telegram-suite", Version: "1.0.3", Name: "Telegram Bot 插件"},
			},
		}, nil
	})

	items, err := svc.fetchMarketFeed(models.PluginMarketRegistry{
		ID:        "official",
		IndexURL:  "builtin://official",
		IsBuiltIn: true,
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("fetch builtin official feed failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 merged market items, got %d", len(items))
	}
	if items[0].PluginID != "demo-feature" {
		t.Fatalf("expected first item demo-feature, got %s", items[0].PluginID)
	}
	if items[1].PluginID != "telegram-suite" {
		t.Fatalf("expected second item telegram-suite, got %s", items[1].PluginID)
	}

	snapshot, err := os.ReadFile(filepath.Join(svc.marketRoot, "official.json"))
	if err != nil {
		t.Fatalf("read merged official feed snapshot failed: %v", err)
	}
	var merged MarketFeed
	if err := json.Unmarshal(snapshot, &merged); err != nil {
		t.Fatalf("unmarshal merged official feed snapshot failed: %v", err)
	}
	if len(merged.Items) != 2 {
		t.Fatalf("expected merged official snapshot to contain 2 items, got %d", len(merged.Items))
	}
}

func seedPluginRuntimeMetadata(t *testing.T, repo repository.PluginRepository, pluginID string) {
	t.Helper()
	if err := repo.ReplaceRouteRegistries(pluginID, []models.PluginRouteRegistry{
		{Scope: "public", Path: "/hello", Methods: models.StringArray{"GET"}},
	}); err != nil {
		t.Fatalf("seed route registries failed: %v", err)
	}
	if err := repo.ReplacePageRegistries(pluginID, []models.PluginPageRegistry{
		{Scope: "admin", RoutePath: "/plugins/demo", Title: "演示插件"},
	}); err != nil {
		t.Fatalf("seed page registries failed: %v", err)
	}
	if err := repo.ReplaceEventSubscriptions(pluginID, []models.PluginEventSubscription{
		{EventType: "order.created"},
	}); err != nil {
		t.Fatalf("seed event subscriptions failed: %v", err)
	}
}

func TestPluginCenterReloadRuntimeFinalizesDisabledPlugin(t *testing.T) {
	svc, repo, _ := newPluginCenterServiceForTest(t)
	pluginID := "demo-disabled"

	if err := repo.SavePlugin(&models.Plugin{
		ID:             pluginID,
		Name:           "演示插件",
		Type:           "feature",
		Status:         models.PluginStatusDisabled,
		CurrentVersion: "1.0.0",
		PendingVersion: "1.0.0",
		IsEnabled:      false,
		NeedsRestart:   true,
		LastError:      "old error",
	}); err != nil {
		t.Fatalf("save plugin failed: %v", err)
	}
	if err := repo.SavePluginVersion(&models.PluginVersion{
		PluginID:    pluginID,
		Version:     "1.0.0",
		Status:      models.PluginStatusInstalled,
		InstallPath: "/tmp/demo-disabled/1.0.0",
		IsActive:    true,
	}); err != nil {
		t.Fatalf("save plugin version failed: %v", err)
	}
	seedPluginRuntimeMetadata(t, repo, pluginID)

	if err := svc.ReloadRuntime(); err != nil {
		t.Fatalf("reload runtime failed: %v", err)
	}

	pluginItem, err := repo.GetPlugin(pluginID)
	if err != nil {
		t.Fatalf("get plugin failed: %v", err)
	}
	if pluginItem == nil {
		t.Fatalf("expected plugin to remain after disable reload")
	}
	if pluginItem.Status != models.PluginStatusDisabled {
		t.Fatalf("expected status %s, got %s", models.PluginStatusDisabled, pluginItem.Status)
	}
	if pluginItem.NeedsRestart {
		t.Fatalf("expected needs_restart to be false after reload")
	}
	if pluginItem.LastError != "" {
		t.Fatalf("expected last_error to be cleared, got %q", pluginItem.LastError)
	}

	routes, err := repo.ListRouteRegistries(pluginID)
	if err != nil {
		t.Fatalf("list route registries failed: %v", err)
	}
	if len(routes) != 0 {
		t.Fatalf("expected route registries to be cleared, got %d", len(routes))
	}
	pages, err := repo.ListPageRegistries(pluginID)
	if err != nil {
		t.Fatalf("list page registries failed: %v", err)
	}
	if len(pages) != 0 {
		t.Fatalf("expected page registries to be cleared, got %d", len(pages))
	}
	events, err := repo.ListEventSubscriptions(pluginID)
	if err != nil {
		t.Fatalf("list event subscriptions failed: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected event subscriptions to be cleared, got %d", len(events))
	}
}

func TestPluginCenterReloadRuntimeFinalizesRemovedPlugin(t *testing.T) {
	svc, repo, settingSvc := newPluginCenterServiceForTest(t)
	pluginID := "demo-removed"

	if err := repo.SavePlugin(&models.Plugin{
		ID:             pluginID,
		Name:           "演示插件",
		Type:           "feature",
		Status:         models.PluginStatusRemovePendingRestart,
		CurrentVersion: "1.0.0",
		PendingVersion: "1.0.0",
		IsEnabled:      false,
		NeedsRestart:   true,
	}); err != nil {
		t.Fatalf("save plugin failed: %v", err)
	}
	if err := repo.SavePluginVersion(&models.PluginVersion{
		PluginID:    pluginID,
		Version:     "1.0.0",
		Status:      models.PluginStatusInstalled,
		InstallPath: "/tmp/demo-removed/1.0.0",
		IsActive:    true,
	}); err != nil {
		t.Fatalf("save plugin version failed: %v", err)
	}
	if err := repo.CreateRuntimeLog(&models.PluginRuntimeLog{
		PluginID:      pluginID,
		PluginVersion: "1.0.0",
		Level:         "warn",
		EventType:     "plugin_removed_pending_reload",
		Message:       "等待重载清理",
	}); err != nil {
		t.Fatalf("create runtime log failed: %v", err)
	}
	seedPluginRuntimeMetadata(t, repo, pluginID)
	if _, err := settingSvc.Update(svc.configKey(pluginID), map[string]interface{}{"greeting": "hello"}); err != nil {
		t.Fatalf("save plugin config failed: %v", err)
	}

	if err := svc.ReloadRuntime(); err != nil {
		t.Fatalf("reload runtime failed: %v", err)
	}

	pluginItem, err := repo.GetPlugin(pluginID)
	if err != nil {
		t.Fatalf("get plugin failed: %v", err)
	}
	if pluginItem != nil {
		t.Fatalf("expected plugin to be removed after reload")
	}

	versions, err := repo.ListPluginVersions(pluginID)
	if err != nil {
		t.Fatalf("list plugin versions failed: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("expected plugin versions to be removed, got %d", len(versions))
	}

	logs, total, err := repo.ListRuntimeLogs(repository.PluginLogListFilter{
		Page:     1,
		PageSize: 20,
		PluginID: pluginID,
	})
	if err != nil {
		t.Fatalf("list runtime logs failed: %v", err)
	}
	if total != 0 || len(logs) != 0 {
		t.Fatalf("expected runtime logs to be removed, got total=%d len=%d", total, len(logs))
	}

	routes, err := repo.ListRouteRegistries(pluginID)
	if err != nil {
		t.Fatalf("list route registries failed: %v", err)
	}
	if len(routes) != 0 {
		t.Fatalf("expected route registries to be removed, got %d", len(routes))
	}
	pages, err := repo.ListPageRegistries(pluginID)
	if err != nil {
		t.Fatalf("list page registries failed: %v", err)
	}
	if len(pages) != 0 {
		t.Fatalf("expected page registries to be removed, got %d", len(pages))
	}
	events, err := repo.ListEventSubscriptions(pluginID)
	if err != nil {
		t.Fatalf("list event subscriptions failed: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected event subscriptions to be removed, got %d", len(events))
	}

	configValue, err := settingSvc.GetByKey(svc.configKey(pluginID))
	if err != nil {
		t.Fatalf("get plugin config failed: %v", err)
	}
	if configValue != nil {
		t.Fatalf("expected plugin config to be removed, got %#v", configValue)
	}
}

type stubScopedPluginModule struct {
	lastScope        string
	lastRoutePattern string
	lastPath         string
	unloaded         bool
}

func (m *stubScopedPluginModule) OnLoad(ctx context.Context) error {
	_ = ctx
	return nil
}

func (m *stubScopedPluginModule) HandlePublic(ctx context.Context, req *pluginhost.HTTPRequest) (*pluginhost.HTTPResponse, error) {
	_ = ctx
	return &pluginhost.HTTPResponse{StatusCode: 200, Data: map[string]interface{}{"scope": "public", "path": req.Path}}, nil
}

func (m *stubScopedPluginModule) HandleAdmin(ctx context.Context, req *pluginhost.HTTPRequest) (*pluginhost.HTTPResponse, error) {
	_ = ctx
	return &pluginhost.HTTPResponse{StatusCode: 200, Data: map[string]interface{}{"scope": "admin", "path": req.Path}}, nil
}

func (m *stubScopedPluginModule) Handle(ctx context.Context, req *pluginhost.HTTPRequest) (*pluginhost.HTTPResponse, error) {
	_ = ctx
	m.lastScope = req.Scope
	m.lastRoutePattern = req.RoutePattern
	m.lastPath = req.Path
	return &pluginhost.HTTPResponse{StatusCode: 200, Data: map[string]interface{}{"scope": req.Scope, "route_pattern": req.RoutePattern, "path": req.Path}}, nil
}

func (m *stubScopedPluginModule) OnUnload(ctx context.Context) error {
	_ = ctx
	m.unloaded = true
	return nil
}

func TestPluginCenterDispatchMountedRouteUsesScopedModule(t *testing.T) {
	svc, _, _ := newPluginCenterServiceForTest(t)
	module := &stubScopedPluginModule{}
	svc.loaded["telegram-suite"] = &pluginLoadedRuntime{
		plugin:  &models.Plugin{ID: "telegram-suite"},
		version: &models.PluginVersion{PluginID: "telegram-suite", Version: "1.0.0"},
		module:  module,
		routeItems: []models.PluginRouteRegistry{
			{PluginID: "telegram-suite", Scope: pluginhost.ScopeAuth, Path: "/telegram/login", Methods: models.StringArray{"POST"}},
		},
	}

	if !svc.HasMountedRoute(pluginhost.ScopeAuth, "/telegram/login", "POST") {
		t.Fatalf("expected mounted auth route to exist")
	}
	if svc.HasMountedRoute(pluginhost.ScopeAuth, "/telegram/login", "GET") {
		t.Fatalf("expected GET method to be rejected for mounted auth route")
	}

	resp, err := svc.DispatchMounted(pluginhost.ScopeAuth, "/telegram/login", &pluginhost.HTTPRequest{
		Scope:        pluginhost.ScopeAuth,
		Method:       "POST",
		RoutePattern: "/telegram/login",
		Path:         "/telegram/login",
	})
	if err != nil {
		t.Fatalf("dispatch mounted route failed: %v", err)
	}
	if resp == nil || resp.StatusCode != 200 {
		t.Fatalf("expected mounted route response status 200, got %#v", resp)
	}
	if module.lastScope != pluginhost.ScopeAuth {
		t.Fatalf("expected scope %s, got %s", pluginhost.ScopeAuth, module.lastScope)
	}
	if module.lastRoutePattern != "/telegram/login" {
		t.Fatalf("expected route pattern /telegram/login, got %s", module.lastRoutePattern)
	}
}

func TestPluginCenterListMountedPagesFiltersByScope(t *testing.T) {
	svc, _, _ := newPluginCenterServiceForTest(t)
	svc.loaded["telegram-suite"] = &pluginLoadedRuntime{
		plugin:  &models.Plugin{ID: "telegram-suite"},
		version: &models.PluginVersion{PluginID: "telegram-suite", Version: "1.0.0"},
		pageItems: []models.PluginPageRegistry{
			{PluginID: "telegram-suite", Scope: pluginhost.ScopeAdmin, RoutePath: "/telegram-bot", SortOrder: 20},
			{PluginID: "telegram-suite", Scope: pluginhost.ScopePublic, RoutePath: "/telegram-widget", SortOrder: 10},
			{PluginID: "telegram-suite", Scope: pluginhost.ScopeAdmin, RoutePath: "/telegram-bot/settings", SortOrder: 30},
		},
	}

	items := svc.ListMountedPages(pluginhost.ScopeAdmin)
	if len(items) != 2 {
		t.Fatalf("expected 2 admin mounted pages, got %d", len(items))
	}
	if items[0].RoutePath != "/telegram-bot" {
		t.Fatalf("expected first admin page /telegram-bot, got %s", items[0].RoutePath)
	}
	if items[1].RoutePath != "/telegram-bot/settings" {
		t.Fatalf("expected second admin page /telegram-bot/settings, got %s", items[1].RoutePath)
	}
}

func TestPluginCenterReloadRuntimeShutsDownPreviousRuntime(t *testing.T) {
	svc, repo, _ := newPluginCenterServiceForTest(t)
	module := &stubScopedPluginModule{}
	svc.loaded["legacy"] = &pluginLoadedRuntime{
		plugin:     &models.Plugin{ID: "legacy"},
		version:    &models.PluginVersion{PluginID: "legacy", Version: "1.0.0"},
		module:     module,
		loadCtx:    context.Background(),
		cancel:     func() {},
		routeItems: []models.PluginRouteRegistry{},
	}

	if err := repo.SavePlugin(&models.Plugin{
		ID:             "disabled-noop",
		Name:           "禁用插件",
		Type:           "feature",
		Status:         models.PluginStatusDisabled,
		CurrentVersion: "1.0.0",
		PendingVersion: "1.0.0",
		IsEnabled:      false,
		NeedsRestart:   true,
	}); err != nil {
		t.Fatalf("save plugin failed: %v", err)
	}
	if err := repo.SavePluginVersion(&models.PluginVersion{
		PluginID:    "disabled-noop",
		Version:     "1.0.0",
		Status:      models.PluginStatusInstalled,
		InstallPath: "/tmp/disabled-noop/1.0.0",
		IsActive:    true,
	}); err != nil {
		t.Fatalf("save plugin version failed: %v", err)
	}

	if err := svc.ReloadRuntime(); err != nil {
		t.Fatalf("reload runtime failed: %v", err)
	}
	if !module.unloaded {
		t.Fatalf("expected previous runtime module to be unloaded")
	}
}

func TestRuntimePluginHostMainDBAccessRequiresPermission(t *testing.T) {
	svc, _, _ := newPluginCenterServiceForTest(t)
	mainDB, err := gorm.Open(sqlite.Open("file:plugin_center_main_db_access?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open main db failed: %v", err)
	}
	svc.hostExtensions = map[string]interface{}{
		"main_gorm_db":   mainDB,
		"main_db_driver": "postgres",
	}

	host := &runtimePluginHost{
		service: svc,
		plugin: &models.Plugin{
			ID:          "demo-no-main-db",
			Permissions: models.StringArray{pluginhost.PermissionDB},
		},
		version: &models.PluginVersion{PluginID: "demo-no-main-db", Version: "1.0.0"},
	}

	if got := host.GetMainDBDriver(); got != "" {
		t.Fatalf("expected empty main db driver without permission, got %q", got)
	}
	if host.GetMainDB() != nil {
		t.Fatalf("expected nil main db handle without permission")
	}
}

func TestRuntimePluginHostMainDBAccessWithPermission(t *testing.T) {
	svc, _, _ := newPluginCenterServiceForTest(t)
	mainDB, err := gorm.Open(sqlite.Open("file:plugin_center_main_db_allowed?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open main db failed: %v", err)
	}
	svc.hostExtensions = map[string]interface{}{
		"main_gorm_db":   mainDB,
		"main_db_driver": "postgres",
	}

	host := &runtimePluginHost{
		service: svc,
		plugin: &models.Plugin{
			ID:          "demo-main-db",
			Permissions: models.StringArray{pluginhost.PermissionDB, pluginhost.PermissionDBMain},
		},
		version: &models.PluginVersion{PluginID: "demo-main-db", Version: "1.0.0"},
	}

	if got := host.GetMainDBDriver(); got != "postgres" {
		t.Fatalf("expected main db driver postgres, got %q", got)
	}
	if host.GetMainDB() != mainDB {
		t.Fatalf("expected main db handle to be exposed")
	}
}

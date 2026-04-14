package service

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	stdplugin "plugin"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/pluginhost"
)

const (
	pluginConfigKeyPrefix     = "plugin_config:"
	pluginMarketBuiltinPrefix = "builtin://"
	hostPluginAPIVersion      = "v1"
	pluginDefaultEntrySymbol  = "PluginEntrypoint"
)

type pluginLoadedRuntime struct {
	plugin     *models.Plugin
	version    *models.PluginVersion
	module     pluginhost.Module
	routeItems []models.PluginRouteRegistry
	pageItems  []models.PluginPageRegistry
	loadCtx    context.Context
	cancel     context.CancelFunc
}

// PluginCenterService 插件中心服务。
type PluginCenterService struct {
	repo           repository.PluginRepository
	settingService *SettingService
	runtimeRoot    string
	marketRoot     string
	mu             sync.RWMutex
	loaded         map[string]*pluginLoadedRuntime
	paymentDrivers map[string]pluginhost.PaymentDriver
	hostExtensions map[string]interface{}
	builtinFeeds   map[string]func() (*MarketFeed, error)
	httpClient     *http.Client
}

// PluginUploadResult 本地上传结果。
type PluginUploadResult struct {
	Plugin   *models.Plugin        `json:"plugin"`
	Version  *models.PluginVersion `json:"version"`
	Manifest pluginhost.Manifest   `json:"manifest"`
}

// PluginDetail 插件详情。
type PluginDetail struct {
	Plugin        *models.Plugin                   `json:"plugin"`
	Versions      []models.PluginVersion           `json:"versions"`
	Routes        []models.PluginRouteRegistry     `json:"routes"`
	Pages         []models.PluginPageRegistry      `json:"pages"`
	Events        []models.PluginEventSubscription `json:"events"`
	RuntimeLoaded bool                             `json:"runtime_loaded"`
	Config        models.JSON                      `json:"config"`
}

// MarketFeed 在线插件库元数据结构。
type MarketFeed struct {
	Items []MarketFeedItem `json:"items"`
}

// MarketFeedItem 在线插件库单项。
type MarketFeedItem struct {
	PluginID       string                 `json:"plugin_id"`
	Name           string                 `json:"name"`
	Author         string                 `json:"author"`
	Type           string                 `json:"type"`
	Version        string                 `json:"version"`
	Summary        string                 `json:"summary"`
	Description    string                 `json:"description"`
	Icon           string                 `json:"icon"`
	Cover          string                 `json:"cover"`
	HostAPIVersion string                 `json:"host_api_version"`
	GoVersion      string                 `json:"go_version"`
	BuildTarget    string                 `json:"build_target"`
	Permissions    []string               `json:"permissions"`
	DownloadURL    string                 `json:"download_url"`
	Checksum       string                 `json:"checksum"`
	ReviewStatus   string                 `json:"review_status"`
	Changelog      string                 `json:"changelog"`
	ConfigSchema   map[string]interface{} `json:"config_schema"`
	Meta           map[string]interface{} `json:"meta"`
}

// NewPluginCenterService 创建插件中心服务。
func NewPluginCenterService(repo repository.PluginRepository, settingService *SettingService) *PluginCenterService {
	workdir, _ := os.Getwd()
	runtimeRoot := filepath.Join(workdir, "runtime", "plugins")
	marketRoot := filepath.Join(workdir, "plugin-market")
	svc := &PluginCenterService{
		repo:           repo,
		settingService: settingService,
		runtimeRoot:    runtimeRoot,
		marketRoot:     marketRoot,
		loaded:         make(map[string]*pluginLoadedRuntime),
		paymentDrivers: make(map[string]pluginhost.PaymentDriver),
		hostExtensions: make(map[string]interface{}),
		builtinFeeds:   make(map[string]func() (*MarketFeed, error)),
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
	_ = os.MkdirAll(filepath.Join(runtimeRoot, "_uploads"), 0o755)
	_ = os.MkdirAll(filepath.Join(runtimeRoot, "_tmp"), 0o755)
	_ = os.MkdirAll(marketRoot, 0o755)
	if err := svc.EnsureBuiltinRegistries(); err != nil {
		logger.Warnw("plugin_market_builtin_registry_init_failed", "error", err)
	}
	return svc
}

// SetHostExtension 注册宿主扩展上下文，供特权内置插件读取。
func (s *PluginCenterService) SetHostExtension(name string, value interface{}) {
	if s == nil || strings.TrimSpace(name) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hostExtensions == nil {
		s.hostExtensions = make(map[string]interface{})
	}
	s.hostExtensions[strings.TrimSpace(name)] = value
}

// SetBuiltinFeedProvider 注册内置插件库的动态 feed 提供器。
func (s *PluginCenterService) SetBuiltinFeedProvider(name string, provider func() (*MarketFeed, error)) {
	if s == nil || strings.TrimSpace(name) == "" || provider == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.builtinFeeds == nil {
		s.builtinFeeds = make(map[string]func() (*MarketFeed, error))
	}
	s.builtinFeeds[strings.TrimSpace(name)] = provider
}

func (s *PluginCenterService) configKey(pluginID string) string {
	return pluginConfigKeyPrefix + strings.TrimSpace(pluginID)
}

func (s *PluginCenterService) EnsureBuiltinRegistries() error {
	if s == nil || s.repo == nil {
		return nil
	}
	builtins := []*models.PluginMarketRegistry{
		{ID: "official", Name: "官方库", Description: "平台维护与审核的官方插件库", SourceType: "builtin", IndexURL: pluginMarketBuiltinPrefix + "official", IsBuiltIn: true, IsEnabled: true, SortOrder: 10, LastSyncStatus: "idle"},
		{ID: "public", Name: "公共市场", Description: "开发者提交并通过审核的公共插件市场", SourceType: "builtin", IndexURL: pluginMarketBuiltinPrefix + "public", IsBuiltIn: true, IsEnabled: true, SortOrder: 20, LastSyncStatus: "idle"},
	}
	for _, item := range builtins {
		existing, err := s.repo.GetMarketRegistry(item.ID)
		if err != nil {
			return err
		}
		if existing == nil {
			if err := s.repo.UpsertMarketRegistry(item); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *PluginCenterService) ListPlugins(filter repository.PluginListFilter) ([]models.Plugin, int64, error) {
	if s == nil || s.repo == nil {
		return []models.Plugin{}, 0, nil
	}
	return s.repo.ListPlugins(filter)
}

func (s *PluginCenterService) GetPluginDetail(pluginID string) (*PluginDetail, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	pluginID = strings.TrimSpace(pluginID)
	item, err := s.repo.GetPlugin(pluginID)
	if err != nil || item == nil {
		return nil, err
	}
	versions, _ := s.repo.ListPluginVersions(pluginID)
	routes, _ := s.repo.ListRouteRegistries(pluginID)
	pages, _ := s.repo.ListPageRegistries(pluginID)
	events, _ := s.repo.ListEventSubscriptions(pluginID)
	configValue, _ := s.settingService.GetByKey(s.configKey(pluginID))
	s.mu.RLock()
	_, loaded := s.loaded[pluginID]
	s.mu.RUnlock()
	return &PluginDetail{Plugin: item, Versions: versions, Routes: routes, Pages: pages, Events: events, RuntimeLoaded: loaded, Config: configValue}, nil
}

func (s *PluginCenterService) GetPluginConfig(pluginID string) (models.JSON, error) {
	if s == nil || s.settingService == nil {
		return models.JSON{}, nil
	}
	value, err := s.settingService.GetByKey(s.configKey(pluginID))
	if value == nil {
		return models.JSON{}, err
	}
	return value, err
}

func (s *PluginCenterService) UpdatePluginConfig(pluginID string, value map[string]interface{}) (models.JSON, error) {
	if s == nil || s.settingService == nil {
		return models.JSON{}, nil
	}
	return s.settingService.Update(s.configKey(pluginID), value)
}

func (s *PluginCenterService) UploadArchive(file *multipart.FileHeader) (*PluginUploadResult, error) {
	if s == nil || file == nil {
		return nil, errors.New("plugin archive is required")
	}
	safeName := sanitizeFilename(file.Filename)
	packagePath := filepath.Join(s.runtimeRoot, "_uploads", fmt.Sprintf("%d-%s", time.Now().UnixNano(), safeName))
	if err := copyMultipartFile(file, packagePath); err != nil {
		return nil, err
	}
	manifest, checksum, err := s.readArchiveManifest(packagePath)
	if err != nil {
		return nil, err
	}
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}
	pluginItem, err := s.repo.GetPlugin(manifest.ID)
	if err != nil {
		return nil, err
	}
	if pluginItem == nil {
		pluginItem = &models.Plugin{ID: manifest.ID}
	}
	pluginItem.Name = manifest.Name
	pluginItem.Type = manifest.Type
	pluginItem.Author = strings.TrimSpace(manifest.Author)
	pluginItem.Summary = strings.TrimSpace(manifest.Summary)
	pluginItem.Description = strings.TrimSpace(manifest.Description)
	pluginItem.Source = "local"
	pluginItem.Status = models.PluginStatusUploaded
	pluginItem.HostAPIVersion = strings.TrimSpace(manifest.HostAPIVersion)
	pluginItem.GoVersion = strings.TrimSpace(manifest.GoVersion)
	pluginItem.BuildTarget = strings.TrimSpace(manifest.BuildTarget)
	pluginItem.EntrySymbol = pickFirstNonEmpty(strings.TrimSpace(manifest.EntrySymbol), pluginDefaultEntrySymbol)
	pluginItem.Permissions = normalizeStringArray(manifest.Permissions)
	pluginItem.ConfigSchema = mapToJSON(manifest.ConfigSchema)
	pluginItem.PendingVersion = strings.TrimSpace(manifest.Version)
	pluginItem.NeedsRestart = false
	if err := s.repo.SavePlugin(pluginItem); err != nil {
		return nil, err
	}
	versionItem, err := s.repo.GetPluginVersion(manifest.ID, manifest.Version)
	if err != nil {
		return nil, err
	}
	if versionItem == nil {
		versionItem = &models.PluginVersion{PluginID: manifest.ID, Version: manifest.Version}
	}
	versionItem.Status = models.PluginStatusUploaded
	versionItem.PackagePath = packagePath
	versionItem.Checksum = checksum
	versionItem.ManifestJSON = manifestToJSON(manifest)
	versionItem.MetaJSON = models.JSON{"original_filename": file.Filename}
	versionItem.IsActive = pluginItem.CurrentVersion == manifest.Version && pluginItem.IsEnabled
	if err := s.repo.SavePluginVersion(versionItem); err != nil {
		return nil, err
	}
	s.recordRuntimeLog(manifest.ID, manifest.Version, "info", "plugin_uploaded", "插件包上传成功", models.JSON{"filename": file.Filename, "checksum": checksum})
	return &PluginUploadResult{Plugin: pluginItem, Version: versionItem, Manifest: manifest}, nil
}

func (s *PluginCenterService) InstallUploaded(pluginID, version string) (*PluginDetail, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("plugin service unavailable")
	}
	pluginID = strings.TrimSpace(pluginID)
	item, err := s.repo.GetPlugin(pluginID)
	if err != nil || item == nil {
		return nil, errors.New("插件不存在")
	}
	versionItem, err := s.resolveInstallVersion(pluginID, version)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(versionItem.PackagePath) == "" {
		return nil, errors.New("插件包不存在")
	}
	installPath := filepath.Join(s.runtimeRoot, pluginID, versionItem.Version)
	if err := os.RemoveAll(installPath); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(installPath, 0o755); err != nil {
		return nil, err
	}
	if err := extractArchive(versionItem.PackagePath, installPath); err != nil {
		return nil, err
	}
	pluginDataDir := filepath.Join(s.runtimeRoot, pluginID, "data")
	pluginLogDir := filepath.Join(s.runtimeRoot, pluginID, "logs")
	if err := os.MkdirAll(pluginDataDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(pluginLogDir, 0o755); err != nil {
		return nil, err
	}
	sqlitePath := filepath.Join(pluginDataDir, "plugin.db")
	if err := s.initPluginSQLite(sqlitePath); err != nil {
		return nil, err
	}
	if err := applyPluginMigrations(sqlitePath, filepath.Join(installPath, "migrations")); err != nil {
		return nil, err
	}
	versionItem.InstallPath = installPath
	versionItem.Status = models.PluginStatusInstalled
	if err := s.repo.SavePluginVersion(versionItem); err != nil {
		return nil, err
	}
	item.Status = models.PluginStatusInstalled
	item.PendingVersion = versionItem.Version
	if err := s.repo.SavePlugin(item); err != nil {
		return nil, err
	}
	s.recordRuntimeLog(pluginID, versionItem.Version, "info", "plugin_installed", "插件版本安装完成", models.JSON{"install_path": installPath})
	return s.GetPluginDetail(pluginID)
}

func (s *PluginCenterService) InstallFromMarket(registryID, pluginID, version string) (*PluginDetail, error) {
	marketItem, err := s.repo.GetMarketItem(strings.TrimSpace(registryID), strings.TrimSpace(pluginID), strings.TrimSpace(version))
	if err != nil {
		return nil, err
	}
	if marketItem == nil {
		return nil, errors.New("在线插件不存在")
	}
	packagePath, err := s.downloadMarketPackage(marketItem)
	if err != nil {
		return nil, err
	}
	manifest, checksum, err := s.readArchiveManifest(packagePath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(marketItem.Checksum) != "" && !strings.EqualFold(strings.TrimSpace(marketItem.Checksum), checksum) {
		return nil, errors.New("插件包校验失败")
	}
	pluginItem, err := s.repo.GetPlugin(manifest.ID)
	if err != nil {
		return nil, err
	}
	if pluginItem == nil {
		pluginItem = &models.Plugin{ID: manifest.ID}
	}
	pluginItem.Name = manifest.Name
	pluginItem.Type = manifest.Type
	pluginItem.Author = manifest.Author
	pluginItem.Summary = manifest.Summary
	pluginItem.Description = manifest.Description
	pluginItem.Icon = marketItem.Icon
	pluginItem.Cover = marketItem.Cover
	pluginItem.Source = registryID
	pluginItem.Status = models.PluginStatusUploaded
	pluginItem.ReviewStatus = pickFirstNonEmpty(marketItem.ReviewStatus, "approved")
	pluginItem.HostAPIVersion = manifest.HostAPIVersion
	pluginItem.GoVersion = manifest.GoVersion
	pluginItem.BuildTarget = manifest.BuildTarget
	pluginItem.EntrySymbol = pickFirstNonEmpty(manifest.EntrySymbol, pluginDefaultEntrySymbol)
	pluginItem.Permissions = normalizeStringArray(manifest.Permissions)
	pluginItem.ConfigSchema = mapToJSON(manifest.ConfigSchema)
	pluginItem.PendingVersion = manifest.Version
	if err := s.repo.SavePlugin(pluginItem); err != nil {
		return nil, err
	}
	versionItem := &models.PluginVersion{PluginID: manifest.ID, Version: manifest.Version, Status: models.PluginStatusUploaded, PackagePath: packagePath, Checksum: checksum, ManifestJSON: manifestToJSON(manifest), MetaJSON: models.JSON{"registry_id": registryID, "download_url": marketItem.DownloadURL}}
	if err := s.repo.SavePluginVersion(versionItem); err != nil {
		return nil, err
	}
	return s.InstallUploaded(manifest.ID, manifest.Version)
}

func (s *PluginCenterService) EnablePlugin(pluginID, version string) (*PluginDetail, error) {
	item, err := s.repo.GetPlugin(strings.TrimSpace(pluginID))
	if err != nil || item == nil {
		return nil, errors.New("插件不存在")
	}
	versionItem, err := s.resolveInstalledVersion(item.ID, version)
	if err != nil {
		return nil, err
	}
	versions, _ := s.repo.ListPluginVersions(item.ID)
	for _, candidate := range versions {
		candidate.IsActive = candidate.Version == versionItem.Version
		if candidate.ID != 0 {
			_ = s.repo.SavePluginVersion(&candidate)
		}
	}
	item.IsEnabled = true
	item.CurrentVersion = versionItem.Version
	item.PendingVersion = versionItem.Version
	item.Status = models.PluginStatusUpgradePendingRestart
	item.NeedsRestart = true
	if err := s.repo.SavePlugin(item); err != nil {
		return nil, err
	}
	s.recordRuntimeLog(item.ID, versionItem.Version, "info", "plugin_enabled_pending_reload", "插件已启用，等待重载生效", nil)
	return s.GetPluginDetail(item.ID)
}

func (s *PluginCenterService) DisablePlugin(pluginID string) (*PluginDetail, error) {
	item, err := s.repo.GetPlugin(strings.TrimSpace(pluginID))
	if err != nil || item == nil {
		return nil, errors.New("插件不存在")
	}
	item.IsEnabled = false
	item.Status = models.PluginStatusDisabled
	item.PendingVersion = item.CurrentVersion
	item.NeedsRestart = true
	if err := s.repo.SavePlugin(item); err != nil {
		return nil, err
	}
	s.recordRuntimeLog(item.ID, item.CurrentVersion, "info", "plugin_disabled_pending_reload", "插件已禁用，等待重载生效", nil)
	return s.GetPluginDetail(item.ID)
}

func (s *PluginCenterService) RollbackPlugin(pluginID string) (*PluginDetail, error) {
	item, err := s.repo.GetPlugin(strings.TrimSpace(pluginID))
	if err != nil || item == nil {
		return nil, errors.New("插件不存在")
	}
	versions, err := s.repo.ListPluginVersions(item.ID)
	if err != nil {
		return nil, err
	}
	if len(versions) < 2 {
		return nil, errors.New("没有可回滚的历史版本")
	}
	current := strings.TrimSpace(item.CurrentVersion)
	target := ""
	for _, versionItem := range versions {
		if versionItem.Version != current && versionItem.Status != models.PluginStatusUploaded {
			target = versionItem.Version
			break
		}
	}
	if target == "" {
		return nil, errors.New("没有可回滚的历史版本")
	}
	return s.EnablePlugin(item.ID, target)
}

func (s *PluginCenterService) RemovePlugin(pluginID string, purge bool) error {
	item, err := s.repo.GetPlugin(strings.TrimSpace(pluginID))
	if err != nil || item == nil {
		return errors.New("插件不存在")
	}
	s.mu.Lock()
	delete(s.loaded, item.ID)
	delete(s.paymentDrivers, item.ID)
	s.mu.Unlock()
	item.IsEnabled = false
	item.Status = models.PluginStatusRemovePendingRestart
	item.NeedsRestart = true
	if err := s.repo.SavePlugin(item); err != nil {
		return err
	}
	if purge {
		_ = os.RemoveAll(filepath.Join(s.runtimeRoot, item.ID))
	}
	s.recordRuntimeLog(item.ID, item.CurrentVersion, "warn", "plugin_removed_pending_reload", "插件已标记移除，等待重载完成清理", models.JSON{"purge": purge})
	return nil
}

func (s *PluginCenterService) clearPluginRuntimeRegistry(pluginID string) error {
	if s == nil || s.repo == nil || strings.TrimSpace(pluginID) == "" {
		return nil
	}
	if err := s.repo.ReplaceRouteRegistries(pluginID, nil); err != nil {
		return err
	}
	if err := s.repo.ReplacePageRegistries(pluginID, nil); err != nil {
		return err
	}
	return s.repo.ReplaceEventSubscriptions(pluginID, nil)
}

func (s *PluginCenterService) finalizeDisabledPlugin(item *models.Plugin) error {
	if s == nil || s.repo == nil || item == nil {
		return nil
	}
	if err := s.clearPluginRuntimeRegistry(item.ID); err != nil {
		return err
	}
	item.Status = models.PluginStatusDisabled
	item.NeedsRestart = false
	item.LastError = ""
	return s.repo.SavePlugin(item)
}

func (s *PluginCenterService) finalizeRemovedPlugin(pluginID string) error {
	if s == nil || s.repo == nil || strings.TrimSpace(pluginID) == "" {
		return nil
	}
	if err := s.clearPluginRuntimeRegistry(pluginID); err != nil {
		return err
	}
	if s.settingService != nil {
		if err := s.settingService.Delete(s.configKey(pluginID)); err != nil {
			return err
		}
	}
	return s.repo.DeletePlugin(pluginID)
}

func (s *PluginCenterService) ListPluginLogs(filter repository.PluginLogListFilter) ([]models.PluginRuntimeLog, int64, error) {
	if s == nil || s.repo == nil {
		return []models.PluginRuntimeLog{}, 0, nil
	}
	return s.repo.ListRuntimeLogs(filter)
}

func (s *PluginCenterService) ListRegistries() ([]models.PluginMarketRegistry, error) {
	if s == nil || s.repo == nil {
		return []models.PluginMarketRegistry{}, nil
	}
	return s.repo.ListMarketRegistries()
}

func (s *PluginCenterService) RefreshMarket() error {
	if s == nil || s.repo == nil {
		return nil
	}
	registries, err := s.repo.ListMarketRegistries()
	if err != nil {
		return err
	}
	for _, registryItem := range registries {
		if !registryItem.IsEnabled {
			continue
		}
		items, err := s.fetchMarketFeed(registryItem)
		now := time.Now()
		if err != nil {
			registryItem.LastSyncAt = &now
			registryItem.LastSyncStatus = "failed"
			registryItem.LastSyncMessage = err.Error()
			_ = s.repo.UpsertMarketRegistry(&registryItem)
			s.recordRuntimeLog(registryItem.ID, "", "error", "market_refresh_failed", "在线插件库同步失败", models.JSON{"registry_id": registryItem.ID, "error": err.Error()})
			continue
		}
		cacheItems := make([]models.PluginMarketCache, 0, len(items))
		for _, item := range items {
			cacheItems = append(cacheItems, models.PluginMarketCache{
				RegistryID:     registryItem.ID,
				PluginID:       strings.TrimSpace(item.PluginID),
				Version:        strings.TrimSpace(item.Version),
				Name:           strings.TrimSpace(item.Name),
				Author:         strings.TrimSpace(item.Author),
				Type:           strings.TrimSpace(item.Type),
				Summary:        strings.TrimSpace(item.Summary),
				Description:    strings.TrimSpace(item.Description),
				Icon:           strings.TrimSpace(item.Icon),
				Cover:          strings.TrimSpace(item.Cover),
				HostAPIVersion: strings.TrimSpace(item.HostAPIVersion),
				GoVersion:      strings.TrimSpace(item.GoVersion),
				BuildTarget:    strings.TrimSpace(item.BuildTarget),
				Permissions:    normalizeStringArray(item.Permissions),
				DownloadURL:    strings.TrimSpace(item.DownloadURL),
				Checksum:       strings.TrimSpace(item.Checksum),
				ReviewStatus:   pickFirstNonEmpty(strings.TrimSpace(item.ReviewStatus), "approved"),
				Changelog:      strings.TrimSpace(item.Changelog),
				ConfigSchema:   mapToJSON(item.ConfigSchema),
				MetaJSON:       mapToJSON(item.Meta),
				SyncedAt:       now,
			})
		}
		if err := s.repo.ReplaceMarketCache(registryItem.ID, cacheItems); err != nil {
			return err
		}
		registryItem.LastSyncAt = &now
		registryItem.LastSyncStatus = "ok"
		registryItem.LastSyncMessage = fmt.Sprintf("同步 %d 个插件版本", len(cacheItems))
		if err := s.repo.UpsertMarketRegistry(&registryItem); err != nil {
			return err
		}
	}
	return nil
}

func (s *PluginCenterService) ListMarketItems(filter repository.PluginMarketListFilter) ([]models.PluginMarketCache, int64, error) {
	return s.repo.ListMarketItems(filter)
}

func (s *PluginCenterService) GetMarketItem(registryID, pluginID string) (*models.PluginMarketCache, []models.PluginMarketCache, error) {
	item, err := s.repo.GetMarketItem(registryID, pluginID, "")
	if err != nil || item == nil {
		return nil, nil, err
	}
	versions, err := s.repo.ListMarketVersions(registryID, pluginID)
	return item, versions, err
}

func (s *PluginCenterService) ReloadRuntime() error {
	if s == nil || s.repo == nil {
		return nil
	}
	items, _, err := s.repo.ListPlugins(repository.PluginListFilter{Page: 1, PageSize: 500})
	if err != nil {
		return err
	}
	loaded := make(map[string]*pluginLoadedRuntime)
	paymentDrivers := make(map[string]pluginhost.PaymentDriver)
	for _, item := range items {
		if !item.IsEnabled {
			if item.Status == models.PluginStatusRemovePendingRestart {
				if err := s.finalizeRemovedPlugin(item.ID); err != nil {
					return err
				}
				continue
			}
			if item.NeedsRestart || item.Status == models.PluginStatusDisabled {
				if err := s.finalizeDisabledPlugin(&item); err != nil {
					return err
				}
			}
			continue
		}
		if strings.TrimSpace(item.CurrentVersion) == "" {
			continue
		}
		versionItem, err := s.repo.GetPluginVersion(item.ID, item.CurrentVersion)
		if err != nil || versionItem == nil {
			continue
		}
		runtimeItem, driver, err := s.loadPlugin(&item, versionItem)
		if err != nil {
			now := time.Now()
			item.Status = models.PluginStatusLoadFailed
			item.LastError = err.Error()
			item.LastLoadedAt = &now
			item.NeedsRestart = false
			_ = s.repo.SavePlugin(&item)
			s.recordRuntimeLog(item.ID, versionItem.Version, "error", "plugin_load_failed", "插件装载失败", models.JSON{"error": err.Error()})
			continue
		}
		now := time.Now()
		item.Status = models.PluginStatusEnabled
		item.LastError = ""
		item.LastLoadedAt = &now
		item.NeedsRestart = false
		_ = s.repo.SavePlugin(&item)
		loaded[item.ID] = runtimeItem
		if driver != nil {
			paymentDrivers[item.ID] = driver
		}
		s.recordRuntimeLog(item.ID, versionItem.Version, "info", "plugin_loaded", "插件装载成功", nil)
	}
	var previous map[string]*pluginLoadedRuntime
	s.mu.Lock()
	previous = s.loaded
	s.loaded = loaded
	s.paymentDrivers = paymentDrivers
	s.mu.Unlock()
	s.shutdownLoadedRuntimes(previous)
	return nil
}

func (s *PluginCenterService) Dispatch(scope, pluginID string, req *pluginhost.HTTPRequest) (*pluginhost.HTTPResponse, error) {
	if s == nil {
		return nil, errors.New("plugin service unavailable")
	}
	s.mu.RLock()
	item, ok := s.loaded[strings.TrimSpace(pluginID)]
	s.mu.RUnlock()
	if !ok || item == nil || item.module == nil {
		return nil, errors.New("插件未加载或未启用")
	}
	return s.dispatchRuntime(strings.TrimSpace(scope), item, req)
}

// HasMountedRoute 判断当前是否有已启用插件接管指定作用域和路由。
func (s *PluginCenterService) HasMountedRoute(scope, routePattern, method string) bool {
	if s == nil {
		return false
	}
	scope = normalizeScope(scope)
	routePattern = normalizePluginPath(routePattern)
	method = strings.ToUpper(strings.TrimSpace(method))
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.loaded {
		if item == nil {
			continue
		}
		for _, routeItem := range item.routeItems {
			if routeMatches(routeItem, scope, routePattern, method) {
				return true
			}
		}
	}
	return false
}

// ListMountedPages 返回当前已加载插件登记的运行时页面。
func (s *PluginCenterService) ListMountedPages(scope string) []models.PluginPageRegistry {
	if s == nil {
		return []models.PluginPageRegistry{}
	}
	filterScope := strings.TrimSpace(scope)
	if filterScope != "" {
		filterScope = normalizeScope(filterScope)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]models.PluginPageRegistry, 0)
	for _, item := range s.loaded {
		if item == nil {
			continue
		}
		for _, pageItem := range item.pageItems {
			if filterScope != "" && normalizeScope(pageItem.Scope) != filterScope {
				continue
			}
			items = append(items, pageItem)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].SortOrder == items[j].SortOrder {
			if items[i].RoutePath == items[j].RoutePath {
				return items[i].PluginID < items[j].PluginID
			}
			return items[i].RoutePath < items[j].RoutePath
		}
		return items[i].SortOrder < items[j].SortOrder
	})
	return items
}

// DispatchMounted 根据挂载路由分发到对应插件。
func (s *PluginCenterService) DispatchMounted(scope, routePattern string, req *pluginhost.HTTPRequest) (*pluginhost.HTTPResponse, error) {
	if s == nil {
		return nil, errors.New("plugin service unavailable")
	}
	scope = normalizeScope(scope)
	routePattern = normalizePluginPath(routePattern)
	method := ""
	if req != nil {
		req.Scope = scope
		req.RoutePattern = routePattern
		method = req.Method
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.loaded {
		if item == nil {
			continue
		}
		for _, routeItem := range item.routeItems {
			if !routeMatches(routeItem, scope, routePattern, method) {
				continue
			}
			return s.dispatchRuntime(scope, item, req)
		}
	}
	return nil, errors.New("未找到接管该路由的插件")
}

func (s *PluginCenterService) loadPlugin(item *models.Plugin, versionItem *models.PluginVersion) (*pluginLoadedRuntime, pluginhost.PaymentDriver, error) {
	pluginFile := filepath.Join(versionItem.InstallPath, "plugin.so")
	if stat, err := os.Stat(pluginFile); err != nil || stat.IsDir() {
		return nil, nil, errors.New("plugin.so 不存在")
	}
	opened, err := stdplugin.Open(pluginFile)
	if err != nil {
		return nil, nil, err
	}
	entrySymbol := pickFirstNonEmpty(strings.TrimSpace(item.EntrySymbol), pluginDefaultEntrySymbol)
	symbol, err := opened.Lookup(entrySymbol)
	if err != nil {
		return nil, nil, err
	}
	entry, err := castPluginEntrypoint(symbol)
	if err != nil {
		return nil, nil, err
	}
	host := &runtimePluginHost{service: s, plugin: item, version: versionItem}
	module, err := entry(host)
	if err != nil {
		return nil, nil, err
	}
	if module == nil {
		return nil, nil, errors.New("插件入口未返回模块实例")
	}
	loadCtx, cancel := context.WithCancel(context.Background())
	if err := module.OnLoad(loadCtx); err != nil {
		cancel()
		return nil, nil, err
	}
	if err := s.repo.ReplaceRouteRegistries(item.ID, host.routeItems); err != nil {
		cancel()
		return nil, nil, err
	}
	if err := s.repo.ReplacePageRegistries(item.ID, host.pageItems); err != nil {
		cancel()
		return nil, nil, err
	}
	if err := s.repo.ReplaceEventSubscriptions(item.ID, host.eventItems); err != nil {
		cancel()
		return nil, nil, err
	}
	routeItems := append([]models.PluginRouteRegistry(nil), host.routeItems...)
	for index := range routeItems {
		routeItems[index].PluginID = item.ID
	}
	pageItems := append([]models.PluginPageRegistry(nil), host.pageItems...)
	for index := range pageItems {
		pageItems[index].PluginID = item.ID
	}
	return &pluginLoadedRuntime{
		plugin:     item,
		version:    versionItem,
		module:     module,
		routeItems: routeItems,
		pageItems:  pageItems,
		loadCtx:    loadCtx,
		cancel:     cancel,
	}, host.paymentDriver, nil
}

func (s *PluginCenterService) resolveInstallVersion(pluginID, version string) (*models.PluginVersion, error) {
	if strings.TrimSpace(version) != "" {
		item, err := s.repo.GetPluginVersion(pluginID, version)
		if err != nil || item == nil {
			return nil, errors.New("插件版本不存在")
		}
		return item, nil
	}
	item, err := s.repo.GetLatestPluginVersion(pluginID)
	if err != nil || item == nil {
		return nil, errors.New("插件版本不存在")
	}
	return item, nil
}

func (s *PluginCenterService) resolveInstalledVersion(pluginID, version string) (*models.PluginVersion, error) {
	item, err := s.resolveInstallVersion(pluginID, version)
	if err != nil {
		return nil, err
	}
	if item.Status == models.PluginStatusUploaded {
		return nil, errors.New("插件版本尚未安装")
	}
	return item, nil
}

func (s *PluginCenterService) recordRuntimeLog(pluginID, version, level, eventType, message string, details models.JSON) {
	if s == nil || s.repo == nil || strings.TrimSpace(pluginID) == "" {
		return
	}
	item := &models.PluginRuntimeLog{PluginID: strings.TrimSpace(pluginID), PluginVersion: strings.TrimSpace(version), Level: normalizeLogLevel(level), EventType: strings.TrimSpace(eventType), Message: strings.TrimSpace(message), DetailsJSON: details, CreatedAt: time.Now()}
	if err := s.repo.CreateRuntimeLog(item); err != nil {
		logger.Warnw("plugin_runtime_log_persist_failed", "plugin_id", pluginID, "error", err)
	}
	logFile := filepath.Join(s.runtimeRoot, pluginID, "logs", "plugin.log")
	_ = os.MkdirAll(filepath.Dir(logFile), 0o755)
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s [%s] %s %s\n", time.Now().Format(time.RFC3339), strings.ToUpper(level), eventType, message)
}

func (s *PluginCenterService) readArchiveManifest(path string) (pluginhost.Manifest, string, error) {
	manifestBytes, checksum, err := readArchiveFile(path, "plugin.json")
	if err != nil {
		return pluginhost.Manifest{}, "", err
	}
	var manifest pluginhost.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return pluginhost.Manifest{}, "", err
	}
	return manifest, checksum, nil
}

func (s *PluginCenterService) fetchMarketFeed(registryItem models.PluginMarketRegistry) ([]MarketFeedItem, error) {
	urlValue := strings.TrimSpace(registryItem.IndexURL)
	if strings.HasPrefix(urlValue, pluginMarketBuiltinPrefix) {
		name := strings.TrimPrefix(urlValue, pluginMarketBuiltinPrefix)
		return s.loadBuiltinMarketFeed(name)
	}
	resp, err := s.httpClient.Get(urlValue)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("registry http status %d", resp.StatusCode)
	}
	var feed MarketFeed
	if err := json.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}
	return feed.Items, nil
}

func (s *PluginCenterService) loadBuiltinMarketFeed(name string) ([]MarketFeedItem, error) {
	baseFeed, baseErr := s.readBuiltinMarketFeedFile(name)
	provider := s.getBuiltinFeedProvider(name)
	if provider == nil {
		if baseErr != nil {
			return nil, baseErr
		}
		return baseFeed.Items, nil
	}

	dynamicFeed, err := provider()
	if err != nil {
		if baseErr == nil {
			return baseFeed.Items, nil
		}
		return nil, err
	}
	merged := mergeMarketFeeds(baseFeed, dynamicFeed)
	_ = s.writeBuiltinMarketFeedFile(name, merged)
	return merged.Items, nil
}

func (s *PluginCenterService) getBuiltinFeedProvider(name string) func() (*MarketFeed, error) {
	if s == nil || strings.TrimSpace(name) == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.builtinFeeds[strings.TrimSpace(name)]
}

func (s *PluginCenterService) readBuiltinMarketFeedFile(name string) (*MarketFeed, error) {
	content, err := os.ReadFile(filepath.Join(s.marketRoot, name+".json"))
	if err != nil {
		return nil, err
	}
	var feed MarketFeed
	if err := json.Unmarshal(content, &feed); err != nil {
		return nil, err
	}
	return &feed, nil
}

func (s *PluginCenterService) writeBuiltinMarketFeedFile(name string, feed *MarketFeed) error {
	if s == nil || strings.TrimSpace(name) == "" || feed == nil {
		return nil
	}
	content, err := json.MarshalIndent(feed, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.marketRoot, strings.TrimSpace(name)+".json"), content, 0o644)
}

func mergeMarketFeeds(baseFeed, dynamicFeed *MarketFeed) *MarketFeed {
	merged := &MarketFeed{Items: make([]MarketFeedItem, 0)}
	indexMap := make(map[string]int)
	appendItems := func(items []MarketFeedItem) {
		for _, item := range items {
			key := strings.TrimSpace(item.PluginID) + "@" + strings.TrimSpace(item.Version)
			if key == "@" {
				continue
			}
			if index, ok := indexMap[key]; ok {
				merged.Items[index] = item
				continue
			}
			indexMap[key] = len(merged.Items)
			merged.Items = append(merged.Items, item)
		}
	}
	if baseFeed != nil {
		appendItems(baseFeed.Items)
	}
	if dynamicFeed != nil {
		appendItems(dynamicFeed.Items)
	}
	return merged
}

func (s *PluginCenterService) downloadMarketPackage(item *models.PluginMarketCache) (string, error) {
	if item == nil {
		return "", errors.New("market item is nil")
	}
	downloadURL := strings.TrimSpace(item.DownloadURL)
	if downloadURL == "" {
		return "", errors.New("下载地址为空")
	}
	target := filepath.Join(s.runtimeRoot, "_uploads", fmt.Sprintf("market-%s-%s.zip", item.PluginID, item.Version))
	if strings.HasPrefix(downloadURL, pluginMarketBuiltinPrefix) {
		localName := strings.TrimPrefix(downloadURL, pluginMarketBuiltinPrefix)
		src := filepath.Join(s.marketRoot, localName)
		input, err := os.ReadFile(src)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(target, input, 0o644); err != nil {
			return "", err
		}
		return target, nil
	}
	resp, err := s.httpClient.Get(downloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download failed: http %d", resp.StatusCode)
	}
	file, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", err
	}
	return target, nil
}

func (s *PluginCenterService) initPluginSQLite(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	_, _ = db.Exec("PRAGMA journal_mode=WAL")
	_, _ = db.Exec("PRAGMA busy_timeout=5000")
	_, _ = db.Exec("PRAGMA synchronous=NORMAL")
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS plugin_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL)")
	return err
}

type runtimePluginHost struct {
	service       *PluginCenterService
	plugin        *models.Plugin
	version       *models.PluginVersion
	pageItems     []models.PluginPageRegistry
	routeItems    []models.PluginRouteRegistry
	eventItems    []models.PluginEventSubscription
	paymentDriver pluginhost.PaymentDriver
}

func (h *runtimePluginHost) GetPluginID() string { return h.plugin.ID }
func (h *runtimePluginHost) GetVersion() string  { return h.version.Version }
func (h *runtimePluginHost) GetDataDir() string {
	return filepath.Join(h.service.runtimeRoot, h.plugin.ID, "data")
}
func (h *runtimePluginHost) GetAssetsDir() string {
	return filepath.Join(h.version.InstallPath, "assets")
}
func (h *runtimePluginHost) GetLogDir() string {
	return filepath.Join(h.service.runtimeRoot, h.plugin.ID, "logs")
}
func (h *runtimePluginHost) GetSQLitePath() string { return filepath.Join(h.GetDataDir(), "plugin.db") }
func (h *runtimePluginHost) GetMainDBDriver() string {
	if !h.canAccessMainDB() {
		return ""
	}
	if h == nil || h.service == nil {
		return ""
	}
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()
	if raw, ok := h.service.hostExtensions["main_db_driver"].(string); ok {
		return strings.TrimSpace(raw)
	}
	return ""
}
func (h *runtimePluginHost) GetMainDB() interface{} {
	if !h.canAccessMainDB() {
		return nil
	}
	if h == nil || h.service == nil {
		return nil
	}
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()
	return h.service.hostExtensions["main_gorm_db"]
}
func (h *runtimePluginHost) ReadConfig() (map[string]interface{}, error) {
	value, err := h.service.GetPluginConfig(h.plugin.ID)
	if value == nil {
		return map[string]interface{}{}, err
	}
	return value, err
}
func (h *runtimePluginHost) SaveConfig(value map[string]interface{}) error {
	_, err := h.service.UpdatePluginConfig(h.plugin.ID, value)
	return err
}
func (h *runtimePluginHost) Log(level string, eventType string, message string, details map[string]interface{}) {
	h.service.recordRuntimeLog(h.plugin.ID, h.version.Version, level, eventType, message, mapToJSON(details))
}
func (h *runtimePluginHost) RegisterPage(item pluginhost.PageRegistration) error {
	h.pageItems = append(h.pageItems, models.PluginPageRegistry{Scope: normalizeScope(item.Scope), RoutePath: normalizePluginPath(item.RoutePath), Title: strings.TrimSpace(item.Title), SortOrder: item.SortOrder, MetaJSON: mapToJSON(item.Meta)})
	return nil
}
func (h *runtimePluginHost) RegisterRoute(item pluginhost.RouteRegistration) error {
	h.routeItems = append(h.routeItems, models.PluginRouteRegistry{Scope: normalizeScope(item.Scope), Path: normalizePluginPath(item.Path), Methods: normalizeStringArray(item.Methods), MetaJSON: mapToJSON(item.Meta)})
	return nil
}
func (h *runtimePluginHost) RegisterEventSubscription(item pluginhost.EventSubscription) error {
	h.eventItems = append(h.eventItems, models.PluginEventSubscription{EventType: strings.TrimSpace(item.EventType), MetaJSON: mapToJSON(item.Meta)})
	return nil
}
func (h *runtimePluginHost) RegisterPaymentDriver(driver pluginhost.PaymentDriver) error {
	h.paymentDriver = driver
	return nil
}
func (h *runtimePluginHost) GetExtension(name string) interface{} {
	if h == nil || h.service == nil || strings.TrimSpace(name) == "" {
		return nil
	}
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()
	return h.service.hostExtensions[strings.TrimSpace(name)]
}

func (h *runtimePluginHost) canAccessMainDB() bool {
	if h == nil || h.plugin == nil {
		return false
	}
	return stringArrayContains(h.plugin.Permissions, pluginhost.PermissionDBMain) ||
		stringArrayContains(h.plugin.Permissions, pluginhost.PermissionPrivileged)
}

func stringArrayContains(items []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}

func castPluginEntrypoint(symbol interface{}) (pluginhost.Entrypoint, error) {
	switch v := symbol.(type) {
	case pluginhost.Entrypoint:
		return v, nil
	case *pluginhost.Entrypoint:
		return *v, nil
	case func(pluginhost.Host) (pluginhost.Module, error):
		return pluginhost.Entrypoint(v), nil
	case *func(pluginhost.Host) (pluginhost.Module, error):
		return pluginhost.Entrypoint(*v), nil
	default:
		return nil, errors.New("插件入口符号类型不匹配")
	}
}

func validateManifest(manifest pluginhost.Manifest) error {
	if strings.TrimSpace(manifest.ID) == "" {
		return errors.New("plugin.json 缺少 id")
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return errors.New("plugin.json 缺少 name")
	}
	if strings.TrimSpace(manifest.Type) == "" {
		return errors.New("plugin.json 缺少 type")
	}
	switch strings.TrimSpace(manifest.Type) {
	case pluginhost.TypeTheme, pluginhost.TypePayment, pluginhost.TypeFeature:
	default:
		return errors.New("plugin.json type 不支持")
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return errors.New("plugin.json 缺少 version")
	}
	if strings.TrimSpace(manifest.BuildTarget) == "" {
		manifest.BuildTarget = "linux/amd64"
	}
	if !strings.EqualFold(strings.TrimSpace(manifest.BuildTarget), "linux/amd64") {
		return errors.New("当前版本仅支持 linux/amd64 插件")
	}
	if strings.TrimSpace(manifest.EntrySymbol) == "" {
		manifest.EntrySymbol = pluginDefaultEntrySymbol
	}
	return nil
}

func readArchiveFile(path string, targetName string) ([]byte, string, error) {
	checksum, err := fileSHA256(path)
	if err != nil {
		return nil, "", err
	}
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".zip") {
		reader, err := zip.OpenReader(path)
		if err != nil {
			return nil, "", err
		}
		defer reader.Close()
		for _, file := range reader.File {
			if filepath.Base(file.Name) != targetName {
				continue
			}
			handle, err := file.Open()
			if err != nil {
				return nil, "", err
			}
			defer handle.Close()
			data, err := io.ReadAll(handle)
			return data, checksum, err
		}
		return nil, "", errors.New("插件包缺少 plugin.json")
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		file, err := os.Open(path)
		if err != nil {
			return nil, "", err
		}
		defer file.Close()
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, "", err
		}
		defer gzReader.Close()
		tarReader := tar.NewReader(gzReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, "", err
			}
			if filepath.Base(header.Name) != targetName {
				continue
			}
			data, err := io.ReadAll(tarReader)
			return data, checksum, err
		}
		return nil, "", errors.New("插件包缺少 plugin.json")
	}
	return nil, "", errors.New("当前仅支持 zip 或 tar.gz 插件包")
}

func extractArchive(archivePath, targetDir string) error {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".zip") {
		reader, err := zip.OpenReader(archivePath)
		if err != nil {
			return err
		}
		defer reader.Close()
		for _, file := range reader.File {
			if err := extractZipFile(file, targetDir); err != nil {
				return err
			}
		}
		return nil
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		file, err := os.Open(archivePath)
		if err != nil {
			return err
		}
		defer file.Close()
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gzReader.Close()
		tarReader := tar.NewReader(gzReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if err := extractTarEntry(header, tarReader, targetDir); err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New("当前仅支持 zip 或 tar.gz 插件包")
}

func extractZipFile(file *zip.File, targetDir string) error {
	relPath := filepath.Clean(file.Name)
	if relPath == "." || strings.Contains(relPath, "..") {
		return errors.New("非法插件包路径")
	}
	dest := filepath.Join(targetDir, relPath)
	if file.FileInfo().IsDir() {
		return os.MkdirAll(dest, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	writer, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
	if err != nil {
		return err
	}
	defer writer.Close()
	_, err = io.Copy(writer, reader)
	return err
}

func extractTarEntry(header *tar.Header, reader io.Reader, targetDir string) error {
	relPath := filepath.Clean(header.Name)
	if relPath == "." || strings.Contains(relPath, "..") {
		return errors.New("非法插件包路径")
	}
	dest := filepath.Join(targetDir, relPath)
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(dest, 0o755)
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		writer, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
		if err != nil {
			return err
		}
		defer writer.Close()
		_, err = io.Copy(writer, reader)
		return err
	default:
		return nil
	}
}

func applyPluginMigrations(sqlitePath, migrationDir string) error {
	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".sql") {
			continue
		}
		files = append(files, filepath.Join(migrationDir, entry.Name()))
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil
	}
	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return err
	}
	defer db.Close()
	_, _ = db.Exec("CREATE TABLE IF NOT EXISTS plugin_schema_migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL)")
	for _, file := range files {
		name := filepath.Base(file)
		var existing string
		err := db.QueryRow("SELECT name FROM plugin_schema_migrations WHERE name = ?", name).Scan(&existing)
		if err == nil && existing == name {
			continue
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %w", name, err)
		}
		if _, err := db.Exec("INSERT INTO plugin_schema_migrations(name, applied_at) VALUES(?, ?)", name, time.Now().Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func copyMultipartFile(file *multipart.FileHeader, target string) error {
	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	output, err := os.Create(target)
	if err != nil {
		return err
	}
	defer output.Close()
	_, err = io.Copy(output, source)
	return err
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	name = strings.ReplaceAll(name, " ", "-")
	if name == "" {
		return fmt.Sprintf("plugin-%d.zip", time.Now().UnixNano())
	}
	return name
}

func manifestToJSON(manifest pluginhost.Manifest) models.JSON {
	bytes, err := json.Marshal(manifest)
	if err != nil {
		return models.JSON{}
	}
	result := models.JSON{}
	_ = json.Unmarshal(bytes, &result)
	return result
}

func mapToJSON(value map[string]interface{}) models.JSON {
	if len(value) == 0 {
		return models.JSON{}
	}
	return models.JSON(value)
}

func normalizeStringArray(values []string) models.StringArray {
	result := make(models.StringArray, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func normalizeScope(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case pluginhost.ScopeAdmin:
		return pluginhost.ScopeAdmin
	case pluginhost.ScopeAuth:
		return pluginhost.ScopeAuth
	case pluginhost.ScopeUser:
		return pluginhost.ScopeUser
	case pluginhost.ScopeChannel:
		return pluginhost.ScopeChannel
	default:
		return pluginhost.ScopePublic
	}
}

func routeMatches(item models.PluginRouteRegistry, scope, routePattern, method string) bool {
	if normalizeScope(item.Scope) != normalizeScope(scope) {
		return false
	}
	if normalizePluginPath(item.Path) != normalizePluginPath(routePattern) {
		return false
	}
	if len(item.Methods) == 0 || strings.TrimSpace(method) == "" {
		return true
	}
	for _, itemMethod := range item.Methods {
		if strings.EqualFold(strings.TrimSpace(itemMethod), method) {
			return true
		}
	}
	return false
}

func (s *PluginCenterService) dispatchRuntime(scope string, item *pluginLoadedRuntime, req *pluginhost.HTTPRequest) (*pluginhost.HTTPResponse, error) {
	if item == nil || item.module == nil {
		return nil, errors.New("插件未加载或未启用")
	}
	if scoped, ok := item.module.(pluginhost.ScopedModule); ok {
		return scoped.Handle(context.Background(), req)
	}
	if scope == pluginhost.ScopeAdmin {
		return item.module.HandleAdmin(context.Background(), req)
	}
	if scope == pluginhost.ScopePublic {
		return item.module.HandlePublic(context.Background(), req)
	}
	return nil, fmt.Errorf("插件未声明 %s 作用域处理能力", scope)
}

func (s *PluginCenterService) shutdownLoadedRuntimes(items map[string]*pluginLoadedRuntime) {
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.cancel != nil {
			item.cancel()
		}
		if lifecycle, ok := item.module.(pluginhost.LifecycleModule); ok {
			_ = lifecycle.OnUnload(context.Background())
		}
	}
}

func normalizePluginPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "/"
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return "/" + trimmed
}

func normalizeLogLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "warn", "warning":
		return "warn"
	case "error":
		return "error"
	default:
		return "info"
	}
}

func resolveBuiltinURL(base string) string {
	if base == "" {
		return ""
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}
	return parsed.String()
}

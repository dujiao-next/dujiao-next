package service

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/shopspring/decimal"
)

var (
	// ErrOnlinePluginCenterNotFound 在线插件中心记录不存在。
	ErrOnlinePluginCenterNotFound = errors.New("online plugin center record not found")
	// ErrOnlinePluginCenterInvalid 在线插件中心输入参数无效。
	ErrOnlinePluginCenterInvalid = errors.New("online plugin center invalid input")
	// ErrOnlinePluginCenterPublisherInUse 发布者仍有关联插件。
	ErrOnlinePluginCenterPublisherInUse = errors.New("online plugin center publisher in use")
)

// OnlinePluginPublisherInput 发布者写入参数。
type OnlinePluginPublisherInput struct {
	PublisherCode string
	Name          string
	ContactEmail  string
	Status        string
	IsOfficial    *bool
	Meta          map[string]interface{}
}

// OnlinePluginCatalogPluginInput 在线插件主记录写入参数。
type OnlinePluginCatalogPluginInput struct {
	PluginID    string
	Slug        string
	PublisherID uint
	Name        string
	Summary     string
	Description string
	PluginType  string
	BillingMode string
	LicenseMode string
	Status      string
	IsOfficial  *bool
	IsPublic    *bool
	IconURL     string
	CoverURL    string
	HomepageURL string
	SourceURL   string
	Tags        []string
	Meta        map[string]interface{}
}

// OnlinePluginVersionInput 在线插件版本写入参数。
type OnlinePluginVersionInput struct {
	Version            string
	ReleaseChannel     string
	PackageStorageKey  string
	PackageDownloadURL string
	ChecksumSHA256     string
	PackageSizeBytes   int64
	HostAPIVersion     string
	BuildTarget        string
	GoVersion          string
	Permissions        []string
	ConfigSchema       map[string]interface{}
	ChangelogMD        string
	ReviewStatus       string
	PublishedAt        *time.Time
	Meta               map[string]interface{}
}

// OnlinePluginPlanInput 在线插件套餐写入参数。
type OnlinePluginPlanInput struct {
	PlanCode       string
	PlanName       string
	BillingMode    string
	LicenseMode    string
	PriceAmount    string
	PriceCurrency  string
	DurationDays   *int
	MaxSites       int
	MaxActivations int
	FeatureFlags   map[string]interface{}
	Status         string
	SortOrder      int
	Meta           map[string]interface{}
}

// OnlinePluginCenterPublicListFilter 公开市场查询过滤器。
type OnlinePluginCenterPublicListFilter struct {
	Page        int
	PageSize    int
	Keyword     string
	PluginType  string
	BillingMode string
	Scope       string
}

// OnlinePluginCenterPluginSummary 在线插件概要。
type OnlinePluginCenterPluginSummary struct {
	Plugin        *models.PluginMarketCatalogPlugin `json:"plugin"`
	Publisher     *models.PluginMarketPublisher     `json:"publisher,omitempty"`
	LatestVersion *models.PluginMarketVersion       `json:"latest_version,omitempty"`
}

// OnlinePluginCenterPluginDetail 在线插件详情。
type OnlinePluginCenterPluginDetail struct {
	Plugin        *models.PluginMarketCatalogPlugin `json:"plugin"`
	Publisher     *models.PluginMarketPublisher     `json:"publisher,omitempty"`
	Versions      []models.PluginMarketVersion      `json:"versions"`
	Plans         []models.PluginMarketPlan         `json:"plans"`
	LatestVersion *models.PluginMarketVersion       `json:"latest_version,omitempty"`
}

// OnlinePluginCenterDownloadInfo 下载信息。
type OnlinePluginCenterDownloadInfo struct {
	PluginID       string    `json:"plugin_id"`
	Version        string    `json:"version"`
	DownloadURL    string    `json:"download_url"`
	ChecksumSHA256 string    `json:"checksum_sha256"`
	BuildTarget    string    `json:"build_target"`
	HostAPIVersion string    `json:"host_api_version"`
	PublishedAt    time.Time `json:"published_at"`
}

// OnlinePluginCenterService 在线插件中心平台服务。
type OnlinePluginCenterService struct {
	repo repository.PluginMarketPlatformRepository
}

// NewOnlinePluginCenterService 创建在线插件中心服务。
func NewOnlinePluginCenterService(repo repository.PluginMarketPlatformRepository) *OnlinePluginCenterService {
	return &OnlinePluginCenterService{repo: repo}
}

func (s *OnlinePluginCenterService) ListPublishers() ([]models.PluginMarketPublisher, error) {
	if s == nil || s.repo == nil {
		return []models.PluginMarketPublisher{}, nil
	}
	return s.repo.ListMarketPublishers()
}

func (s *OnlinePluginCenterService) CreatePublisher(input OnlinePluginPublisherInput) (*models.PluginMarketPublisher, error) {
	if s == nil || s.repo == nil {
		return nil, ErrOnlinePluginCenterInvalid
	}
	code := normalizeOnlinePluginCode(input.PublisherCode)
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" {
		return nil, ErrOnlinePluginCenterInvalid
	}
	status := normalizePublisherStatus(input.Status)
	item := &models.PluginMarketPublisher{
		PublisherCode: code,
		Name:          name,
		ContactEmail:  strings.TrimSpace(input.ContactEmail),
		Status:        status,
		IsOfficial:    normalizeBool(input.IsOfficial, false),
		MetaJSON:      normalizeJSON(input.Meta),
	}
	if err := s.repo.SaveMarketPublisher(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *OnlinePluginCenterService) UpdatePublisher(id uint, input OnlinePluginPublisherInput) (*models.PluginMarketPublisher, error) {
	if s == nil || s.repo == nil || id == 0 {
		return nil, ErrOnlinePluginCenterInvalid
	}
	item, err := s.repo.GetMarketPublisher(id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrOnlinePluginCenterNotFound
	}
	code := normalizeOnlinePluginCode(input.PublisherCode)
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" {
		return nil, ErrOnlinePluginCenterInvalid
	}
	item.PublisherCode = code
	item.Name = name
	item.ContactEmail = strings.TrimSpace(input.ContactEmail)
	item.Status = normalizePublisherStatus(input.Status)
	item.IsOfficial = normalizeBool(input.IsOfficial, item.IsOfficial)
	item.MetaJSON = normalizeJSON(input.Meta)
	if err := s.repo.SaveMarketPublisher(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *OnlinePluginCenterService) DeletePublisher(id uint) error {
	if s == nil || s.repo == nil || id == 0 {
		return ErrOnlinePluginCenterInvalid
	}
	count, err := s.repo.CountPluginsByPublisher(id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrOnlinePluginCenterPublisherInUse
	}
	return s.repo.DeleteMarketPublisher(id)
}

func (s *OnlinePluginCenterService) ListPlugins(filter repository.PluginMarketCenterPluginListFilter) ([]OnlinePluginCenterPluginSummary, int64, error) {
	if s == nil || s.repo == nil {
		return []OnlinePluginCenterPluginSummary{}, 0, nil
	}
	items, total, err := s.repo.ListCatalogPlugins(filter)
	if err != nil {
		return nil, 0, err
	}
	summaries, err := s.buildPluginSummaries(items, false)
	if err != nil {
		return nil, 0, err
	}
	return summaries, total, nil
}

func (s *OnlinePluginCenterService) GetPluginDetail(pluginID string) (*OnlinePluginCenterPluginDetail, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	item, err := s.repo.GetCatalogPluginByPluginID(strings.TrimSpace(pluginID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	publisher, err := s.repo.GetMarketPublisher(item.PublisherID)
	if err != nil {
		return nil, err
	}
	versions, err := s.repo.ListPluginVersions(item.PluginID)
	if err != nil {
		return nil, err
	}
	plans, err := s.repo.ListPluginPlans(item.PluginID)
	if err != nil {
		return nil, err
	}
	return &OnlinePluginCenterPluginDetail{
		Plugin:        item,
		Publisher:     publisher,
		Versions:      versions,
		Plans:         plans,
		LatestVersion: pickLatestVersion(versions, false),
	}, nil
}

func (s *OnlinePluginCenterService) CreatePlugin(input OnlinePluginCatalogPluginInput) (*models.PluginMarketCatalogPlugin, error) {
	if s == nil || s.repo == nil {
		return nil, ErrOnlinePluginCenterInvalid
	}
	item, err := s.buildCatalogPlugin(nil, input)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveCatalogPlugin(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *OnlinePluginCenterService) UpdatePlugin(pluginID string, input OnlinePluginCatalogPluginInput) (*models.PluginMarketCatalogPlugin, error) {
	if s == nil || s.repo == nil {
		return nil, ErrOnlinePluginCenterInvalid
	}
	existing, err := s.repo.GetCatalogPluginByPluginID(strings.TrimSpace(pluginID))
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrOnlinePluginCenterNotFound
	}
	item, err := s.buildCatalogPlugin(existing, input)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveCatalogPlugin(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *OnlinePluginCenterService) DeletePlugin(pluginID string) error {
	if s == nil || s.repo == nil || strings.TrimSpace(pluginID) == "" {
		return ErrOnlinePluginCenterInvalid
	}
	return s.repo.DeleteCatalogPlugin(strings.TrimSpace(pluginID))
}

func (s *OnlinePluginCenterService) CreateVersion(pluginID string, input OnlinePluginVersionInput) (*models.PluginMarketVersion, error) {
	if s == nil || s.repo == nil {
		return nil, ErrOnlinePluginCenterInvalid
	}
	pluginItem, err := s.repo.GetCatalogPluginByPluginID(strings.TrimSpace(pluginID))
	if err != nil {
		return nil, err
	}
	if pluginItem == nil {
		return nil, ErrOnlinePluginCenterNotFound
	}
	item, err := s.buildPluginVersion(nil, pluginItem.PluginID, input)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SavePluginVersion(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *OnlinePluginCenterService) UpdateVersion(id uint, input OnlinePluginVersionInput) (*models.PluginMarketVersion, error) {
	if s == nil || s.repo == nil || id == 0 {
		return nil, ErrOnlinePluginCenterInvalid
	}
	existing, err := s.repo.GetPluginVersion(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrOnlinePluginCenterNotFound
	}
	item, err := s.buildPluginVersion(existing, existing.PluginID, input)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SavePluginVersion(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *OnlinePluginCenterService) DeleteVersion(id uint) error {
	if s == nil || s.repo == nil || id == 0 {
		return ErrOnlinePluginCenterInvalid
	}
	return s.repo.DeletePluginVersion(id)
}

func (s *OnlinePluginCenterService) CreatePlan(pluginID string, input OnlinePluginPlanInput) (*models.PluginMarketPlan, error) {
	if s == nil || s.repo == nil {
		return nil, ErrOnlinePluginCenterInvalid
	}
	pluginItem, err := s.repo.GetCatalogPluginByPluginID(strings.TrimSpace(pluginID))
	if err != nil {
		return nil, err
	}
	if pluginItem == nil {
		return nil, ErrOnlinePluginCenterNotFound
	}
	item, err := s.buildPluginPlan(nil, pluginItem.PluginID, input)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SavePluginPlan(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *OnlinePluginCenterService) UpdatePlan(id uint, input OnlinePluginPlanInput) (*models.PluginMarketPlan, error) {
	if s == nil || s.repo == nil || id == 0 {
		return nil, ErrOnlinePluginCenterInvalid
	}
	existing, err := s.repo.GetPluginPlan(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrOnlinePluginCenterNotFound
	}
	item, err := s.buildPluginPlan(existing, existing.PluginID, input)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SavePluginPlan(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *OnlinePluginCenterService) DeletePlan(id uint) error {
	if s == nil || s.repo == nil || id == 0 {
		return ErrOnlinePluginCenterInvalid
	}
	return s.repo.DeletePluginPlan(id)
}

func (s *OnlinePluginCenterService) ListPublicPlugins(filter OnlinePluginCenterPublicListFilter) ([]OnlinePluginCenterPluginSummary, int64, error) {
	if s == nil || s.repo == nil {
		return []OnlinePluginCenterPluginSummary{}, 0, nil
	}
	repoFilter := repository.PluginMarketCenterPluginListFilter{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		Keyword:     filter.Keyword,
		Status:      "published",
		PluginType:  filter.PluginType,
		BillingMode: filter.BillingMode,
	}
	scope := strings.TrimSpace(strings.ToLower(filter.Scope))
	switch scope {
	case "official":
		value := true
		repoFilter.IsOfficial = &value
	case "public":
		value := true
		repoFilter.IsPublic = &value
	}
	items, total, err := s.repo.ListCatalogPlugins(repoFilter)
	if err != nil {
		return nil, 0, err
	}
	summaries, err := s.buildPluginSummaries(items, true)
	if err != nil {
		return nil, 0, err
	}
	return summaries, total, nil
}

func (s *OnlinePluginCenterService) GetPublicPluginDetail(pluginID string) (*OnlinePluginCenterPluginDetail, error) {
	detail, err := s.GetPluginDetail(pluginID)
	if err != nil || detail == nil || detail.Plugin == nil {
		return detail, err
	}
	if detail.Plugin.Status != "published" {
		return nil, nil
	}
	detail.Versions = filterPublishedVersions(detail.Versions)
	detail.Plans = filterActivePlans(detail.Plans)
	detail.LatestVersion = pickLatestVersion(detail.Versions, true)
	return detail, nil
}

func (s *OnlinePluginCenterService) BuildPublicFeed(scope string) (*MarketFeed, error) {
	items, _, err := s.ListPublicPlugins(OnlinePluginCenterPublicListFilter{
		Page:     1,
		PageSize: 500,
		Scope:    scope,
	})
	if err != nil {
		return nil, err
	}
	feedItems := make([]MarketFeedItem, 0, len(items))
	for _, item := range items {
		if item.Plugin == nil || item.LatestVersion == nil {
			continue
		}
		version := item.LatestVersion
		if strings.TrimSpace(version.PackageDownloadURL) == "" {
			continue
		}
		feedItems = append(feedItems, MarketFeedItem{
			PluginID:       item.Plugin.PluginID,
			Name:           item.Plugin.Name,
			Author:         publisherName(item.Publisher),
			Type:           item.Plugin.PluginType,
			Version:        version.Version,
			Summary:        item.Plugin.Summary,
			Description:    item.Plugin.Description,
			Icon:           item.Plugin.IconURL,
			Cover:          item.Plugin.CoverURL,
			HostAPIVersion: version.HostAPIVersion,
			GoVersion:      version.GoVersion,
			BuildTarget:    version.BuildTarget,
			Permissions:    append([]string(nil), version.PermissionsJSON...),
			DownloadURL:    version.PackageDownloadURL,
			Checksum:       version.ChecksumSHA256,
			ReviewStatus:   version.ReviewStatus,
			Changelog:      version.ChangelogMD,
			ConfigSchema:   cloneJSON(version.ConfigSchemaJSON),
			Meta:           mergeJSON(item.Plugin.MetaJSON, version.MetaJSON),
		})
	}
	return &MarketFeed{Items: feedItems}, nil
}

func (s *OnlinePluginCenterService) GetPublicVersions(pluginID string) ([]models.PluginMarketVersion, error) {
	if s == nil || s.repo == nil {
		return []models.PluginMarketVersion{}, nil
	}
	pluginItem, err := s.repo.GetCatalogPluginByPluginID(strings.TrimSpace(pluginID))
	if err != nil || pluginItem == nil || pluginItem.Status != "published" {
		return []models.PluginMarketVersion{}, err
	}
	items, err := s.repo.ListPluginVersions(pluginItem.PluginID)
	if err != nil {
		return nil, err
	}
	return filterPublishedVersions(items), nil
}

func (s *OnlinePluginCenterService) GetPublicPlans(pluginID string) ([]models.PluginMarketPlan, error) {
	if s == nil || s.repo == nil {
		return []models.PluginMarketPlan{}, nil
	}
	pluginItem, err := s.repo.GetCatalogPluginByPluginID(strings.TrimSpace(pluginID))
	if err != nil || pluginItem == nil || pluginItem.Status != "published" {
		return []models.PluginMarketPlan{}, err
	}
	items, err := s.repo.ListPluginPlans(pluginItem.PluginID)
	if err != nil {
		return nil, err
	}
	return filterActivePlans(items), nil
}

func (s *OnlinePluginCenterService) GetPublicDownloadInfo(pluginID, version string) (*OnlinePluginCenterDownloadInfo, error) {
	versions, err := s.GetPublicVersions(pluginID)
	if err != nil {
		return nil, err
	}
	selected := pickMatchingVersion(versions, version)
	if selected == nil || strings.TrimSpace(selected.PackageDownloadURL) == "" {
		return nil, nil
	}
	publishedAt := time.Now().UTC()
	if selected.PublishedAt != nil {
		publishedAt = selected.PublishedAt.UTC()
	}
	return &OnlinePluginCenterDownloadInfo{
		PluginID:       strings.TrimSpace(pluginID),
		Version:        selected.Version,
		DownloadURL:    selected.PackageDownloadURL,
		ChecksumSHA256: selected.ChecksumSHA256,
		BuildTarget:    selected.BuildTarget,
		HostAPIVersion: selected.HostAPIVersion,
		PublishedAt:    publishedAt,
	}, nil
}

func (s *OnlinePluginCenterService) buildCatalogPlugin(existing *models.PluginMarketCatalogPlugin, input OnlinePluginCatalogPluginInput) (*models.PluginMarketCatalogPlugin, error) {
	pluginID := normalizeOnlinePluginCode(input.PluginID)
	slug := normalizeOnlinePluginSlug(input.Slug)
	name := strings.TrimSpace(input.Name)
	if pluginID == "" || slug == "" || name == "" || input.PublisherID == 0 {
		return nil, ErrOnlinePluginCenterInvalid
	}
	publisher, err := s.repo.GetMarketPublisher(input.PublisherID)
	if err != nil {
		return nil, err
	}
	if publisher == nil {
		return nil, ErrOnlinePluginCenterInvalid
	}
	item := existing
	if item == nil {
		item = &models.PluginMarketCatalogPlugin{}
	}
	item.PluginID = pluginID
	item.Slug = slug
	item.PublisherID = input.PublisherID
	item.Name = name
	item.Summary = strings.TrimSpace(input.Summary)
	item.Description = strings.TrimSpace(input.Description)
	item.PluginType = normalizePluginType(input.PluginType)
	item.BillingMode = normalizeBillingMode(input.BillingMode)
	item.LicenseMode = normalizeBillingMode(input.LicenseMode)
	item.Status = normalizePluginStatus(input.Status)
	item.IsOfficial = normalizeBool(input.IsOfficial, item.IsOfficial)
	item.IsPublic = normalizeBool(input.IsPublic, existing == nil || item.IsPublic)
	item.IconURL = strings.TrimSpace(input.IconURL)
	item.CoverURL = strings.TrimSpace(input.CoverURL)
	item.HomepageURL = strings.TrimSpace(input.HomepageURL)
	item.SourceURL = strings.TrimSpace(input.SourceURL)
	item.TagsJSON = normalizeUniqueStringArray(input.Tags)
	item.MetaJSON = normalizeJSON(input.Meta)
	return item, nil
}

func (s *OnlinePluginCenterService) buildPluginVersion(existing *models.PluginMarketVersion, pluginID string, input OnlinePluginVersionInput) (*models.PluginMarketVersion, error) {
	version := strings.TrimSpace(input.Version)
	if strings.TrimSpace(pluginID) == "" || version == "" {
		return nil, ErrOnlinePluginCenterInvalid
	}
	reviewStatus := normalizeVersionReviewStatus(input.ReviewStatus)
	releaseChannel := strings.TrimSpace(strings.ToLower(input.ReleaseChannel))
	if releaseChannel == "" {
		releaseChannel = "stable"
	}
	if (reviewStatus == "approved" || reviewStatus == "published") && strings.TrimSpace(input.PackageDownloadURL) == "" {
		return nil, ErrOnlinePluginCenterInvalid
	}
	item := existing
	if item == nil {
		item = &models.PluginMarketVersion{}
	}
	item.PluginID = strings.TrimSpace(pluginID)
	item.Version = version
	item.ReleaseChannel = releaseChannel
	item.PackageStorageKey = strings.TrimSpace(input.PackageStorageKey)
	item.PackageDownloadURL = strings.TrimSpace(input.PackageDownloadURL)
	item.ChecksumSHA256 = strings.TrimSpace(strings.ToLower(input.ChecksumSHA256))
	item.PackageSizeBytes = input.PackageSizeBytes
	item.HostAPIVersion = strings.TrimSpace(input.HostAPIVersion)
	item.BuildTarget = strings.TrimSpace(input.BuildTarget)
	item.GoVersion = strings.TrimSpace(input.GoVersion)
	item.PermissionsJSON = normalizeUniqueStringArray(input.Permissions)
	item.ConfigSchemaJSON = normalizeJSON(input.ConfigSchema)
	item.ChangelogMD = strings.TrimSpace(input.ChangelogMD)
	item.ReviewStatus = reviewStatus
	item.MetaJSON = normalizeJSON(input.Meta)
	if input.PublishedAt != nil {
		publishedAt := input.PublishedAt.UTC()
		item.PublishedAt = &publishedAt
	} else if reviewStatus == "published" && item.PublishedAt == nil {
		now := time.Now().UTC()
		item.PublishedAt = &now
	} else if reviewStatus != "published" && reviewStatus != "approved" {
		item.PublishedAt = nil
	}
	return item, nil
}

func (s *OnlinePluginCenterService) buildPluginPlan(existing *models.PluginMarketPlan, pluginID string, input OnlinePluginPlanInput) (*models.PluginMarketPlan, error) {
	planCode := normalizeOnlinePluginCode(input.PlanCode)
	planName := strings.TrimSpace(input.PlanName)
	if strings.TrimSpace(pluginID) == "" || planCode == "" || planName == "" {
		return nil, ErrOnlinePluginCenterInvalid
	}
	priceDecimal, err := decimal.NewFromString(strings.TrimSpace(defaultString(input.PriceAmount, "0")))
	if err != nil {
		return nil, ErrOnlinePluginCenterInvalid
	}
	duration := input.DurationDays
	billingMode := normalizeBillingMode(input.BillingMode)
	licenseMode := normalizeBillingMode(input.LicenseMode)
	switch billingMode {
	case "annual":
		if duration == nil || *duration <= 0 {
			defaultDays := 365
			duration = &defaultDays
		}
	case "free", "perpetual":
		duration = nil
	}
	item := existing
	if item == nil {
		item = &models.PluginMarketPlan{}
	}
	item.PluginID = strings.TrimSpace(pluginID)
	item.PlanCode = planCode
	item.PlanName = planName
	item.BillingMode = billingMode
	item.LicenseMode = licenseMode
	item.PriceAmount = models.NewMoneyFromDecimal(priceDecimal)
	item.PriceCurrency = strings.ToUpper(strings.TrimSpace(defaultString(input.PriceCurrency, "CNY")))
	item.DurationDays = duration
	item.MaxSites = defaultInt(input.MaxSites, 1)
	item.MaxActivations = defaultInt(input.MaxActivations, 1)
	item.FeatureFlagsJSON = normalizeJSON(input.FeatureFlags)
	item.Status = normalizePlanStatus(input.Status)
	item.SortOrder = input.SortOrder
	item.MetaJSON = normalizeJSON(input.Meta)
	return item, nil
}

func (s *OnlinePluginCenterService) buildPluginSummaries(items []models.PluginMarketCatalogPlugin, publicOnly bool) ([]OnlinePluginCenterPluginSummary, error) {
	if len(items) == 0 {
		return []OnlinePluginCenterPluginSummary{}, nil
	}
	publisherIDs := make([]uint, 0, len(items))
	seenPublisher := make(map[uint]struct{}, len(items))
	for _, item := range items {
		if item.PublisherID == 0 {
			continue
		}
		if _, exists := seenPublisher[item.PublisherID]; exists {
			continue
		}
		seenPublisher[item.PublisherID] = struct{}{}
		publisherIDs = append(publisherIDs, item.PublisherID)
	}
	publisherMap := make(map[uint]*models.PluginMarketPublisher, len(publisherIDs))
	publishers, err := s.repo.ListMarketPublishersByIDs(publisherIDs)
	if err != nil {
		return nil, err
	}
	for i := range publishers {
		item := publishers[i]
		copied := item
		publisherMap[item.ID] = &copied
	}

	summaries := make([]OnlinePluginCenterPluginSummary, 0, len(items))
	for i := range items {
		item := items[i]
		versions, err := s.repo.ListPluginVersions(item.PluginID)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, OnlinePluginCenterPluginSummary{
			Plugin:        &item,
			Publisher:     publisherMap[item.PublisherID],
			LatestVersion: pickLatestVersion(versions, publicOnly),
		})
	}
	return summaries, nil
}

func pickLatestVersion(items []models.PluginMarketVersion, onlyPublished bool) *models.PluginMarketVersion {
	for i := range items {
		if onlyPublished && !isPublishedVersion(items[i]) {
			continue
		}
		item := items[i]
		return &item
	}
	return nil
}

func pickMatchingVersion(items []models.PluginMarketVersion, version string) *models.PluginMarketVersion {
	if len(items) == 0 {
		return nil
	}
	target := strings.TrimSpace(version)
	if target == "" {
		return pickLatestVersion(items, true)
	}
	for i := range items {
		if strings.TrimSpace(items[i].Version) != target {
			continue
		}
		item := items[i]
		return &item
	}
	return nil
}

func filterPublishedVersions(items []models.PluginMarketVersion) []models.PluginMarketVersion {
	filtered := make([]models.PluginMarketVersion, 0, len(items))
	for _, item := range items {
		if isPublishedVersion(item) {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		left := filtered[i].CreatedAt
		right := filtered[j].CreatedAt
		if filtered[i].PublishedAt != nil {
			left = filtered[i].PublishedAt.UTC()
		}
		if filtered[j].PublishedAt != nil {
			right = filtered[j].PublishedAt.UTC()
		}
		return left.After(right)
	})
	return filtered
}

func filterActivePlans(items []models.PluginMarketPlan) []models.PluginMarketPlan {
	filtered := make([]models.PluginMarketPlan, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Status), "active") {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].SortOrder == filtered[j].SortOrder {
			return filtered[i].ID > filtered[j].ID
		}
		return filtered[i].SortOrder > filtered[j].SortOrder
	})
	return filtered
}

func isPublishedVersion(item models.PluginMarketVersion) bool {
	status := strings.TrimSpace(strings.ToLower(item.ReviewStatus))
	return status == "approved" || status == "published"
}

func normalizeOnlinePluginCode(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, " ", "-")
	builder := strings.Builder{}
	lastDash := false
	for _, r := range value {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if !valid {
			continue
		}
		if r == '-' {
			if lastDash {
				continue
			}
			lastDash = true
		} else {
			lastDash = false
		}
		builder.WriteRune(r)
	}
	return strings.Trim(builder.String(), "-_")
}

func normalizeOnlinePluginSlug(raw string) string {
	value := normalizeOnlinePluginCode(raw)
	return strings.ReplaceAll(value, "_", "-")
}

func normalizePublisherStatus(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "disabled":
		return "disabled"
	default:
		return "active"
	}
}

func normalizePluginType(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "payment", "theme":
		return strings.TrimSpace(strings.ToLower(raw))
	default:
		return "feature"
	}
}

func normalizeBillingMode(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "annual", "perpetual":
		return strings.TrimSpace(strings.ToLower(raw))
	default:
		return "free"
	}
}

func normalizePluginStatus(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "pending_review", "published", "hidden", "archived":
		return strings.TrimSpace(strings.ToLower(raw))
	default:
		return "draft"
	}
}

func normalizeVersionReviewStatus(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "pending_review", "approved", "rejected", "published":
		return strings.TrimSpace(strings.ToLower(raw))
	default:
		return "draft"
	}
}

func normalizePlanStatus(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "disabled":
		return "disabled"
	default:
		return "active"
	}
}

func normalizeJSON(value map[string]interface{}) models.JSON {
	if value == nil {
		return models.JSON{}
	}
	cloned := make(models.JSON, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}

func cloneJSON(value models.JSON) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}

func mergeJSON(left, right models.JSON) map[string]interface{} {
	merged := cloneJSON(left)
	for key, item := range right {
		merged[key] = item
	}
	return merged
}

func normalizeUniqueStringArray(values []string) models.StringArray {
	items := make(models.StringArray, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func normalizeBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func publisherName(item *models.PluginMarketPublisher) string {
	if item == nil {
		return ""
	}
	return strings.TrimSpace(item.Name)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

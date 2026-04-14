package admin

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

type adminPluginMarketPublisherRequest struct {
	PublisherCode string                 `json:"publisher_code" binding:"required"`
	Name          string                 `json:"name" binding:"required"`
	ContactEmail  string                 `json:"contact_email"`
	Status        string                 `json:"status"`
	IsOfficial    *bool                  `json:"is_official"`
	Meta          map[string]interface{} `json:"meta"`
}

type adminPluginMarketCatalogPluginRequest struct {
	PluginID    string                 `json:"plugin_id" binding:"required"`
	Slug        string                 `json:"slug" binding:"required"`
	PublisherID uint                   `json:"publisher_id" binding:"required"`
	Name        string                 `json:"name" binding:"required"`
	Summary     string                 `json:"summary"`
	Description string                 `json:"description"`
	PluginType  string                 `json:"plugin_type"`
	BillingMode string                 `json:"billing_mode"`
	LicenseMode string                 `json:"license_mode"`
	Status      string                 `json:"status"`
	IsOfficial  *bool                  `json:"is_official"`
	IsPublic    *bool                  `json:"is_public"`
	IconURL     string                 `json:"icon_url"`
	CoverURL    string                 `json:"cover_url"`
	HomepageURL string                 `json:"homepage_url"`
	SourceURL   string                 `json:"source_url"`
	Tags        []string               `json:"tags"`
	Meta        map[string]interface{} `json:"meta"`
}

type adminPluginMarketVersionRequest struct {
	Version            string                 `json:"version" binding:"required"`
	ReleaseChannel     string                 `json:"release_channel"`
	PackageStorageKey  string                 `json:"package_storage_key"`
	PackageDownloadURL string                 `json:"package_download_url"`
	ChecksumSHA256     string                 `json:"checksum_sha256"`
	PackageSizeBytes   int64                  `json:"package_size_bytes"`
	HostAPIVersion     string                 `json:"host_api_version"`
	BuildTarget        string                 `json:"build_target"`
	GoVersion          string                 `json:"go_version"`
	Permissions        []string               `json:"permissions"`
	ConfigSchema       map[string]interface{} `json:"config_schema"`
	ChangelogMD        string                 `json:"changelog_md"`
	ReviewStatus       string                 `json:"review_status"`
	PublishedAt        string                 `json:"published_at"`
	Meta               map[string]interface{} `json:"meta"`
}

type adminPluginMarketPlanRequest struct {
	PlanCode       string                 `json:"plan_code" binding:"required"`
	PlanName       string                 `json:"plan_name" binding:"required"`
	BillingMode    string                 `json:"billing_mode"`
	LicenseMode    string                 `json:"license_mode"`
	PriceAmount    string                 `json:"price_amount"`
	PriceCurrency  string                 `json:"price_currency"`
	DurationDays   *int                   `json:"duration_days"`
	MaxSites       int                    `json:"max_sites"`
	MaxActivations int                    `json:"max_activations"`
	FeatureFlags   map[string]interface{} `json:"feature_flags"`
	Status         string                 `json:"status"`
	SortOrder      int                    `json:"sort_order"`
	Meta           map[string]interface{} `json:"meta"`
}

// ListPluginMarketCenterPublishers 获取在线插件中心发布者列表。
func (h *Handler) ListPluginMarketCenterPublishers(c *gin.Context) {
	items, err := h.OnlinePluginCenterService.ListPublishers()
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "获取在线插件发布者失败", err)
		return
	}
	response.Success(c, items)
}

// CreatePluginMarketCenterPublisher 创建在线插件发布者。
func (h *Handler) CreatePluginMarketCenterPublisher(c *gin.Context) {
	var req adminPluginMarketPublisherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	item, err := h.OnlinePluginCenterService.CreatePublisher(service.OnlinePluginPublisherInput(req))
	if err != nil {
		respondPluginMarketCenterError(c, err, "创建在线插件发布者失败")
		return
	}
	response.Success(c, item)
}

// UpdatePluginMarketCenterPublisher 更新在线插件发布者。
func (h *Handler) UpdatePluginMarketCenterPublisher(c *gin.Context) {
	var req adminPluginMarketPublisherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	item, err := h.OnlinePluginCenterService.UpdatePublisher(parseUintParam(c.Param("id")), service.OnlinePluginPublisherInput(req))
	if err != nil {
		respondPluginMarketCenterError(c, err, "更新在线插件发布者失败")
		return
	}
	response.Success(c, item)
}

// DeletePluginMarketCenterPublisher 删除在线插件发布者。
func (h *Handler) DeletePluginMarketCenterPublisher(c *gin.Context) {
	if err := h.OnlinePluginCenterService.DeletePublisher(parseUintParam(c.Param("id"))); err != nil {
		respondPluginMarketCenterError(c, err, "删除在线插件发布者失败")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// ListPluginMarketCenterPlugins 获取在线插件中心插件列表。
func (h *Handler) ListPluginMarketCenterPlugins(c *gin.Context) {
	page, pageSize := shared.NormalizePagination(parseIntDefault(c.Query("page"), 1), parseIntDefault(c.Query("page_size"), 20))
	items, total, err := h.OnlinePluginCenterService.ListPlugins(repository.PluginMarketCenterPluginListFilter{
		Page:        page,
		PageSize:    pageSize,
		Keyword:     strings.TrimSpace(c.Query("keyword")),
		Status:      strings.TrimSpace(c.Query("status")),
		PluginType:  strings.TrimSpace(c.Query("plugin_type")),
		BillingMode: strings.TrimSpace(c.Query("billing_mode")),
		PublisherID: parseUintParam(c.Query("publisher_id")),
	})
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "获取在线插件列表失败", err)
		return
	}
	response.SuccessWithPage(c, items, response.BuildPagination(page, pageSize, total))
}

// GetPluginMarketCenterPlugin 获取在线插件详情。
func (h *Handler) GetPluginMarketCenterPlugin(c *gin.Context) {
	item, err := h.OnlinePluginCenterService.GetPluginDetail(strings.TrimSpace(c.Param("plugin_id")))
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "获取在线插件详情失败", err)
		return
	}
	if item == nil {
		shared.RespondErrorWithMsg(c, response.CodeNotFound, "在线插件不存在", nil)
		return
	}
	response.Success(c, item)
}

// CreatePluginMarketCenterPlugin 创建在线插件记录。
func (h *Handler) CreatePluginMarketCenterPlugin(c *gin.Context) {
	var req adminPluginMarketCatalogPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	item, err := h.OnlinePluginCenterService.CreatePlugin(service.OnlinePluginCatalogPluginInput(req))
	if err != nil {
		respondPluginMarketCenterError(c, err, "创建在线插件失败")
		return
	}
	response.Success(c, item)
}

// UpdatePluginMarketCenterPlugin 更新在线插件记录。
func (h *Handler) UpdatePluginMarketCenterPlugin(c *gin.Context) {
	var req adminPluginMarketCatalogPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	item, err := h.OnlinePluginCenterService.UpdatePlugin(strings.TrimSpace(c.Param("plugin_id")), service.OnlinePluginCatalogPluginInput(req))
	if err != nil {
		respondPluginMarketCenterError(c, err, "更新在线插件失败")
		return
	}
	response.Success(c, item)
}

// DeletePluginMarketCenterPlugin 删除在线插件记录。
func (h *Handler) DeletePluginMarketCenterPlugin(c *gin.Context) {
	if err := h.OnlinePluginCenterService.DeletePlugin(strings.TrimSpace(c.Param("plugin_id"))); err != nil {
		respondPluginMarketCenterError(c, err, "删除在线插件失败")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// CreatePluginMarketCenterVersion 创建在线插件版本。
func (h *Handler) CreatePluginMarketCenterVersion(c *gin.Context) {
	var req adminPluginMarketVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	publishedAt, err := parseAdminRFC3339Time(req.PublishedAt)
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "发布时间格式错误，请使用 RFC3339", err)
		return
	}
	item, err := h.OnlinePluginCenterService.CreateVersion(strings.TrimSpace(c.Param("plugin_id")), service.OnlinePluginVersionInput{
		Version:            req.Version,
		ReleaseChannel:     req.ReleaseChannel,
		PackageStorageKey:  req.PackageStorageKey,
		PackageDownloadURL: req.PackageDownloadURL,
		ChecksumSHA256:     req.ChecksumSHA256,
		PackageSizeBytes:   req.PackageSizeBytes,
		HostAPIVersion:     req.HostAPIVersion,
		BuildTarget:        req.BuildTarget,
		GoVersion:          req.GoVersion,
		Permissions:        req.Permissions,
		ConfigSchema:       req.ConfigSchema,
		ChangelogMD:        req.ChangelogMD,
		ReviewStatus:       req.ReviewStatus,
		PublishedAt:        publishedAt,
		Meta:               req.Meta,
	})
	if err != nil {
		respondPluginMarketCenterError(c, err, "创建在线插件版本失败")
		return
	}
	response.Success(c, item)
}

// UpdatePluginMarketCenterVersion 更新在线插件版本。
func (h *Handler) UpdatePluginMarketCenterVersion(c *gin.Context) {
	var req adminPluginMarketVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	publishedAt, err := parseAdminRFC3339Time(req.PublishedAt)
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "发布时间格式错误，请使用 RFC3339", err)
		return
	}
	item, err := h.OnlinePluginCenterService.UpdateVersion(parseUintParam(c.Param("id")), service.OnlinePluginVersionInput{
		Version:            req.Version,
		ReleaseChannel:     req.ReleaseChannel,
		PackageStorageKey:  req.PackageStorageKey,
		PackageDownloadURL: req.PackageDownloadURL,
		ChecksumSHA256:     req.ChecksumSHA256,
		PackageSizeBytes:   req.PackageSizeBytes,
		HostAPIVersion:     req.HostAPIVersion,
		BuildTarget:        req.BuildTarget,
		GoVersion:          req.GoVersion,
		Permissions:        req.Permissions,
		ConfigSchema:       req.ConfigSchema,
		ChangelogMD:        req.ChangelogMD,
		ReviewStatus:       req.ReviewStatus,
		PublishedAt:        publishedAt,
		Meta:               req.Meta,
	})
	if err != nil {
		respondPluginMarketCenterError(c, err, "更新在线插件版本失败")
		return
	}
	response.Success(c, item)
}

// DeletePluginMarketCenterVersion 删除在线插件版本。
func (h *Handler) DeletePluginMarketCenterVersion(c *gin.Context) {
	if err := h.OnlinePluginCenterService.DeleteVersion(parseUintParam(c.Param("id"))); err != nil {
		respondPluginMarketCenterError(c, err, "删除在线插件版本失败")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// CreatePluginMarketCenterPlan 创建在线插件套餐。
func (h *Handler) CreatePluginMarketCenterPlan(c *gin.Context) {
	var req adminPluginMarketPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	item, err := h.OnlinePluginCenterService.CreatePlan(strings.TrimSpace(c.Param("plugin_id")), service.OnlinePluginPlanInput{
		PlanCode:       req.PlanCode,
		PlanName:       req.PlanName,
		BillingMode:    req.BillingMode,
		LicenseMode:    req.LicenseMode,
		PriceAmount:    req.PriceAmount,
		PriceCurrency:  req.PriceCurrency,
		DurationDays:   req.DurationDays,
		MaxSites:       req.MaxSites,
		MaxActivations: req.MaxActivations,
		FeatureFlags:   req.FeatureFlags,
		Status:         req.Status,
		SortOrder:      req.SortOrder,
		Meta:           req.Meta,
	})
	if err != nil {
		respondPluginMarketCenterError(c, err, "创建在线插件套餐失败")
		return
	}
	response.Success(c, item)
}

// UpdatePluginMarketCenterPlan 更新在线插件套餐。
func (h *Handler) UpdatePluginMarketCenterPlan(c *gin.Context) {
	var req adminPluginMarketPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	item, err := h.OnlinePluginCenterService.UpdatePlan(parseUintParam(c.Param("id")), service.OnlinePluginPlanInput{
		PlanCode:       req.PlanCode,
		PlanName:       req.PlanName,
		BillingMode:    req.BillingMode,
		LicenseMode:    req.LicenseMode,
		PriceAmount:    req.PriceAmount,
		PriceCurrency:  req.PriceCurrency,
		DurationDays:   req.DurationDays,
		MaxSites:       req.MaxSites,
		MaxActivations: req.MaxActivations,
		FeatureFlags:   req.FeatureFlags,
		Status:         req.Status,
		SortOrder:      req.SortOrder,
		Meta:           req.Meta,
	})
	if err != nil {
		respondPluginMarketCenterError(c, err, "更新在线插件套餐失败")
		return
	}
	response.Success(c, item)
}

// DeletePluginMarketCenterPlan 删除在线插件套餐。
func (h *Handler) DeletePluginMarketCenterPlan(c *gin.Context) {
	if err := h.OnlinePluginCenterService.DeletePlan(parseUintParam(c.Param("id"))); err != nil {
		respondPluginMarketCenterError(c, err, "删除在线插件套餐失败")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func respondPluginMarketCenterError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrOnlinePluginCenterNotFound):
		shared.RespondErrorWithMsg(c, response.CodeNotFound, "记录不存在", nil)
	case errors.Is(err, service.ErrOnlinePluginCenterInvalid):
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "参数无效，请检查后重试", nil)
	case errors.Is(err, service.ErrOnlinePluginCenterPublisherInUse):
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "发布者下仍有关联插件，暂时不能删除", nil)
	default:
		shared.RespondErrorWithMsg(c, response.CodeInternal, fallback, err)
	}
}

func parseAdminRFC3339Time(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	normalized := parsed.UTC()
	return &normalized, nil
}

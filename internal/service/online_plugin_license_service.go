package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/google/uuid"
)

const onlinePluginLicenseHeartbeatSeconds = 300

var (
	// ErrOnlinePluginLicenseNotFound 授权记录不存在。
	ErrOnlinePluginLicenseNotFound = errors.New("online plugin license not found")
	// ErrOnlinePluginLicenseInvalid 授权参数无效。
	ErrOnlinePluginLicenseInvalid = errors.New("online plugin license invalid input")
	// ErrOnlinePluginLicenseConflict 授权绑定冲突。
	ErrOnlinePluginLicenseConflict = errors.New("online plugin license binding conflict")
	// ErrOnlinePluginLicenseAlreadyActivated 授权已被其他实例占用。
	ErrOnlinePluginLicenseAlreadyActivated = errors.New("online plugin license already activated")
	// ErrOnlinePluginLicenseTokenInvalid 激活令牌无效。
	ErrOnlinePluginLicenseTokenInvalid = errors.New("online plugin license activation token invalid")
	// ErrOnlinePluginLicenseInactive 授权当前不可用。
	ErrOnlinePluginLicenseInactive = errors.New("online plugin license inactive")
)

// OnlinePluginLicenseAdminInput 后台授权写入参数。
type OnlinePluginLicenseAdminInput struct {
	PluginID        string
	PlanCode        string
	CustomerID      uint
	OrderID         uint
	LicenseID       string
	LicenseKey      string
	LicenseMode     string
	Status          string
	BoundDomain     string
	BoundServerIP   string
	ExpireAt        *time.Time
	GraceDeadlineAt *time.Time
	FeatureFlags    map[string]interface{}
	Meta            map[string]interface{}
}

// OnlinePluginLicenseActivateInput 首次激活请求。
type OnlinePluginLicenseActivateInput struct {
	PluginID        string
	LicenseKey      string
	InstallID       string
	HostFingerprint string
	PrimaryDomain   string
	ServerIP        string
	CurrentVersion  string
}

// OnlinePluginLicenseValidateInput 手动校验请求。
type OnlinePluginLicenseValidateInput struct {
	PluginID        string
	LicenseKey      string
	ActivationToken string
	InstallID       string
	PrimaryDomain   string
	ServerIP        string
	CurrentVersion  string
	RuntimeLoaded   bool
}

// OnlinePluginLicenseHeartbeatInput 心跳请求。
type OnlinePluginLicenseHeartbeatInput struct {
	PluginID        string
	ActivationToken string
	InstallID       string
	PrimaryDomain   string
	ServerIP        string
	CurrentVersion  string
	RuntimeLoaded   bool
}

// OnlinePluginLicenseListFilter 后台列表过滤器。
type OnlinePluginLicenseListFilter struct {
	Page        int
	PageSize    int
	Keyword     string
	PluginID    string
	Status      string
	LicenseMode string
}

// OnlinePluginLicenseSummary 授权概要。
type OnlinePluginLicenseSummary struct {
	License          *models.PluginLicense             `json:"license"`
	Plugin           *models.PluginMarketCatalogPlugin `json:"plugin,omitempty"`
	Plan             *models.PluginMarketPlan          `json:"plan,omitempty"`
	ActiveActivation *models.PluginLicenseActivation   `json:"active_activation,omitempty"`
}

// OnlinePluginLicenseDetail 授权详情。
type OnlinePluginLicenseDetail struct {
	License          *models.PluginLicense             `json:"license"`
	Plugin           *models.PluginMarketCatalogPlugin `json:"plugin,omitempty"`
	Plan             *models.PluginMarketPlan          `json:"plan,omitempty"`
	Activations      []models.PluginLicenseActivation  `json:"activations"`
	RecentHeartbeats []models.PluginLicenseHeartbeat   `json:"recent_heartbeats"`
}

// OnlinePluginLicenseDecisionResponse 授权决策结果。
type OnlinePluginLicenseDecisionResponse struct {
	LicenseID                 string      `json:"license_id"`
	PluginID                  string      `json:"plugin_id"`
	LicenseStatus             string      `json:"license_status"`
	LicenseMode               string      `json:"license_mode"`
	BoundDomain               string      `json:"bound_domain"`
	BoundServerIP             string      `json:"bound_server_ip"`
	MatchedDomain             bool        `json:"matched_domain"`
	MatchedServerIP           bool        `json:"matched_server_ip"`
	ExpireAt                  *time.Time  `json:"expire_at,omitempty"`
	GraceDeadlineAt           *time.Time  `json:"grace_deadline_at,omitempty"`
	FeatureFlags              models.JSON `json:"feature_flags"`
	ActivationToken           string      `json:"activation_token,omitempty"`
	EnforcementMode           string      `json:"enforcement_mode"`
	Message                   string      `json:"message"`
	NextHeartbeatAfterSeconds int         `json:"next_heartbeat_after_seconds"`
}

// OnlinePluginLicenseService 在线授权中心服务。
type OnlinePluginLicenseService struct {
	repo repository.PluginLicensePlatformRepository
}

// NewOnlinePluginLicenseService 创建在线授权中心服务。
func NewOnlinePluginLicenseService(repo repository.PluginLicensePlatformRepository) *OnlinePluginLicenseService {
	return &OnlinePluginLicenseService{repo: repo}
}

func (s *OnlinePluginLicenseService) ListLicenses(filter OnlinePluginLicenseListFilter) ([]OnlinePluginLicenseSummary, int64, error) {
	if s == nil || s.repo == nil {
		return []OnlinePluginLicenseSummary{}, 0, nil
	}
	items, total, err := s.repo.ListLicenses(repository.PluginLicenseListFilter{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		Keyword:     strings.TrimSpace(filter.Keyword),
		PluginID:    normalizeOnlinePluginCode(filter.PluginID),
		Status:      normalizeLicenseStatusOptional(filter.Status),
		LicenseMode: normalizeBillingModeOptional(filter.LicenseMode),
	})
	if err != nil {
		return nil, 0, err
	}
	summaries, err := s.buildLicenseSummaries(items)
	if err != nil {
		return nil, 0, err
	}
	return summaries, total, nil
}

func (s *OnlinePluginLicenseService) GetLicenseDetail(licenseID string) (*OnlinePluginLicenseDetail, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	license, err := s.repo.GetLicenseByLicenseID(strings.TrimSpace(licenseID))
	if err != nil {
		return nil, err
	}
	if license == nil {
		return nil, nil
	}
	pluginItem, err := s.repo.GetCatalogPluginByPluginID(license.PluginID)
	if err != nil {
		return nil, err
	}
	planItem, err := s.repo.GetPluginPlan(license.PlanID)
	if err != nil {
		return nil, err
	}
	activations, err := s.repo.ListLicenseActivations(license.LicenseID)
	if err != nil {
		return nil, err
	}
	heartbeats, err := s.repo.ListRecentLicenseHeartbeats(license.LicenseID, 20)
	if err != nil {
		return nil, err
	}
	return &OnlinePluginLicenseDetail{
		License:          license,
		Plugin:           pluginItem,
		Plan:             planItem,
		Activations:      activations,
		RecentHeartbeats: heartbeats,
	}, nil
}

func (s *OnlinePluginLicenseService) CreateLicense(input OnlinePluginLicenseAdminInput) (*models.PluginLicense, error) {
	if s == nil || s.repo == nil {
		return nil, ErrOnlinePluginLicenseInvalid
	}
	license, err := s.buildLicense(nil, input)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveLicense(license); err != nil {
		return nil, err
	}
	return license, nil
}

func (s *OnlinePluginLicenseService) UpdateLicense(licenseID string, input OnlinePluginLicenseAdminInput) (*models.PluginLicense, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(licenseID) == "" {
		return nil, ErrOnlinePluginLicenseInvalid
	}
	existing, err := s.repo.GetLicenseByLicenseID(strings.TrimSpace(licenseID))
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrOnlinePluginLicenseNotFound
	}
	license, err := s.buildLicense(existing, input)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveLicense(license); err != nil {
		return nil, err
	}
	return license, nil
}

func (s *OnlinePluginLicenseService) ActivateLicense(input OnlinePluginLicenseActivateInput) (*OnlinePluginLicenseDecisionResponse, error) {
	if s == nil || s.repo == nil {
		return nil, ErrOnlinePluginLicenseInvalid
	}
	license, domain, serverIP, err := s.resolveLicenseByKey(input.PluginID, input.LicenseKey, input.PrimaryDomain, input.ServerIP)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	state := deriveLicenseRuntimeStatus(license, now)
	if state == "revoked" || state == "suspended" || state == "expired" {
		return nil, ErrOnlinePluginLicenseInactive
	}
	if err := s.ensureBindingAvailable(license, domain, serverIP, now); err != nil {
		return nil, err
	}
	sameInstall, err := s.repo.GetLicenseActivationByInstallID(license.LicenseID, strings.TrimSpace(input.InstallID))
	if err != nil {
		return nil, err
	}
	activeActivation, err := s.repo.GetLatestActiveActivation(license.LicenseID)
	if err != nil {
		return nil, err
	}
	if activeActivation != nil && strings.TrimSpace(activeActivation.InstallID) != "" && strings.TrimSpace(activeActivation.InstallID) != strings.TrimSpace(input.InstallID) {
		return nil, ErrOnlinePluginLicenseAlreadyActivated
	}

	token := generateActivationToken()
	activation := sameInstall
	if activation == nil {
		activation = &models.PluginLicenseActivation{}
	}
	nowCopy := now
	activation.LicenseID = license.LicenseID
	activation.SiteID = domain
	activation.InstallID = strings.TrimSpace(input.InstallID)
	activation.HostFingerprint = strings.TrimSpace(input.HostFingerprint)
	activation.ReportedDomain = domain
	activation.ReportedIP = serverIP
	activation.ValidatedDomain = license.BoundDomain
	activation.ValidatedServerIP = license.BoundServerIP
	activation.ActivationToken = token
	activation.Status = "active"
	activation.ActivatedAt = now
	activation.LastHeartbeatAt = &nowCopy
	activation.LastSeenAt = &nowCopy
	if err := s.repo.SaveLicenseActivation(activation); err != nil {
		return nil, err
	}

	license.ActivationSecretHash = hashToken(token)
	license.ActivatedAt = &nowCopy
	license.LastValidatedAt = &nowCopy
	if state == "grace" {
		license.Status = "grace"
	} else {
		license.Status = "active"
	}
	if err := s.repo.SaveLicense(license); err != nil {
		return nil, err
	}

	decision := s.buildDecision(license, domain, serverIP, now)
	decision.ActivationToken = token
	decision.Message = "授权激活成功"

	if err := s.repo.CreateLicenseHeartbeat(&models.PluginLicenseHeartbeat{
		LicenseID:         license.LicenseID,
		ActivationID:      activation.ID,
		ReportedDomain:    domain,
		ReportedIP:        serverIP,
		ReportedVersion:   strings.TrimSpace(input.CurrentVersion),
		RuntimeLoaded:     true,
		MatchedDomain:     decision.MatchedDomain,
		MatchedServerIP:   decision.MatchedServerIP,
		LicenseStatus:     decision.LicenseStatus,
		EnforcementAction: decision.EnforcementMode,
		Message:           decision.Message,
		CreatedAt:         now,
	}); err != nil {
		return nil, err
	}
	return decision, nil
}

func (s *OnlinePluginLicenseService) ValidateLicense(input OnlinePluginLicenseValidateInput) (*OnlinePluginLicenseDecisionResponse, error) {
	return s.evaluateLicense(input.PluginID, input.LicenseKey, input.ActivationToken, input.InstallID, input.PrimaryDomain, input.ServerIP, input.CurrentVersion, input.RuntimeLoaded, false)
}

func (s *OnlinePluginLicenseService) ReportHeartbeat(input OnlinePluginLicenseHeartbeatInput) (*OnlinePluginLicenseDecisionResponse, error) {
	return s.evaluateLicense(input.PluginID, "", input.ActivationToken, input.InstallID, input.PrimaryDomain, input.ServerIP, input.CurrentVersion, input.RuntimeLoaded, true)
}

func (s *OnlinePluginLicenseService) buildLicense(existing *models.PluginLicense, input OnlinePluginLicenseAdminInput) (*models.PluginLicense, error) {
	pluginID := normalizeOnlinePluginCode(input.PluginID)
	if pluginID == "" {
		return nil, ErrOnlinePluginLicenseInvalid
	}
	pluginItem, err := s.repo.GetCatalogPluginByPluginID(pluginID)
	if err != nil {
		return nil, err
	}
	if pluginItem == nil {
		return nil, ErrOnlinePluginLicenseInvalid
	}
	planItem, err := s.resolvePlan(pluginID, input.PlanCode, existing)
	if err != nil {
		return nil, err
	}

	item := existing
	if item == nil {
		item = &models.PluginLicense{}
		item.IssuedAt = time.Now().UTC()
	}
	item.PluginID = pluginID
	if planItem != nil {
		item.PlanID = planItem.ID
	}
	item.CustomerID = input.CustomerID
	item.OrderID = input.OrderID
	item.LicenseID = normalizeLicenseID(defaultString(input.LicenseID, item.LicenseID))
	if item.LicenseID == "" {
		item.LicenseID = generateLicenseID()
	}
	item.LicenseKey = normalizeLicenseKey(defaultString(input.LicenseKey, item.LicenseKey))
	if item.LicenseKey == "" {
		item.LicenseKey = generateLicenseKey()
	}
	mode := normalizeBillingModeOptional(input.LicenseMode)
	if mode == "" {
		if planItem != nil {
			mode = normalizeBillingModeOptional(planItem.LicenseMode)
		}
		if mode == "" {
			mode = normalizeBillingModeOptional(pluginItem.LicenseMode)
		}
	}
	if mode == "" {
		mode = "free"
	}
	item.LicenseMode = mode
	item.Status = normalizeLicenseStatus(defaultString(input.Status, item.Status))
	item.BoundDomain = normalizeDomain(input.BoundDomain)
	item.BoundServerIP = normalizeServerIP(input.BoundServerIP)
	item.FeatureFlagsJSON = normalizeJSON(input.FeatureFlags)
	item.MetaJSON = normalizeJSON(input.Meta)
	item.ExpireAt = normalizeLicenseExpireAt(mode, input.ExpireAt, planItem)
	item.GraceDeadlineAt = normalizeOptionalTime(input.GraceDeadlineAt)
	if (item.Status == "active" || item.Status == "grace") && item.ActivatedAt == nil && item.BoundDomain != "" && item.BoundServerIP != "" {
		now := time.Now().UTC()
		item.ActivatedAt = &now
	}
	return item, nil
}

func (s *OnlinePluginLicenseService) resolvePlan(pluginID, planCode string, existing *models.PluginLicense) (*models.PluginMarketPlan, error) {
	trimmedCode := normalizeOnlinePluginCode(planCode)
	switch {
	case trimmedCode != "":
		item, err := s.repo.GetPluginPlanByCode(pluginID, trimmedCode)
		if err != nil {
			return nil, err
		}
		if item == nil {
			return nil, ErrOnlinePluginLicenseInvalid
		}
		return item, nil
	case existing != nil && existing.PlanID > 0:
		return s.repo.GetPluginPlan(existing.PlanID)
	default:
		return nil, nil
	}
}

func (s *OnlinePluginLicenseService) buildLicenseSummaries(items []models.PluginLicense) ([]OnlinePluginLicenseSummary, error) {
	summaries := make([]OnlinePluginLicenseSummary, 0, len(items))
	for i := range items {
		item := items[i]
		pluginItem, err := s.repo.GetCatalogPluginByPluginID(item.PluginID)
		if err != nil {
			return nil, err
		}
		planItem, err := s.repo.GetPluginPlan(item.PlanID)
		if err != nil {
			return nil, err
		}
		activation, err := s.repo.GetLatestActiveActivation(item.LicenseID)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, OnlinePluginLicenseSummary{
			License:          &item,
			Plugin:           pluginItem,
			Plan:             planItem,
			ActiveActivation: activation,
		})
	}
	return summaries, nil
}

func (s *OnlinePluginLicenseService) resolveLicenseByKey(pluginID, licenseKey, domain, serverIP string) (*models.PluginLicense, string, string, error) {
	normalizedPluginID := normalizeOnlinePluginCode(pluginID)
	normalizedKey := normalizeLicenseKey(licenseKey)
	normalizedDomain := normalizeDomain(domain)
	normalizedServerIP := normalizeServerIP(serverIP)
	if normalizedPluginID == "" || normalizedKey == "" || normalizedDomain == "" || normalizedServerIP == "" {
		return nil, "", "", ErrOnlinePluginLicenseInvalid
	}
	license, err := s.repo.GetLicenseByLicenseKey(normalizedKey)
	if err != nil {
		return nil, "", "", err
	}
	if license == nil || license.PluginID != normalizedPluginID {
		return nil, "", "", ErrOnlinePluginLicenseNotFound
	}
	return license, normalizedDomain, normalizedServerIP, nil
}

func (s *OnlinePluginLicenseService) ensureBindingAvailable(license *models.PluginLicense, domain, serverIP string, now time.Time) error {
	if license == nil || domain == "" || serverIP == "" {
		return ErrOnlinePluginLicenseInvalid
	}
	if license.BoundDomain == "" {
		license.BoundDomain = domain
	}
	if license.BoundServerIP == "" {
		license.BoundServerIP = serverIP
	}
	if license.BoundDomain != domain || license.BoundServerIP != serverIP {
		return ErrOnlinePluginLicenseConflict
	}
	items, err := s.repo.ListBindingConflictLicenses(license.PluginID, domain, serverIP, license.LicenseID)
	if err != nil {
		return err
	}
	for i := range items {
		status := deriveLicenseRuntimeStatus(&items[i], now)
		if !isBlockingLicenseStatus(status) {
			continue
		}
		if items[i].BoundDomain == domain || items[i].BoundServerIP == serverIP {
			return ErrOnlinePluginLicenseConflict
		}
	}
	return nil
}

func (s *OnlinePluginLicenseService) evaluateLicense(pluginID, licenseKey, activationToken, installID, domain, serverIP, currentVersion string, runtimeLoaded bool, persistHeartbeat bool) (*OnlinePluginLicenseDecisionResponse, error) {
	if s == nil || s.repo == nil {
		return nil, ErrOnlinePluginLicenseInvalid
	}
	now := time.Now().UTC()
	normalizedPluginID := normalizeOnlinePluginCode(pluginID)
	normalizedDomain := normalizeDomain(domain)
	normalizedServerIP := normalizeServerIP(serverIP)
	normalizedInstallID := strings.TrimSpace(installID)
	if normalizedPluginID == "" || normalizedDomain == "" || normalizedServerIP == "" {
		return nil, ErrOnlinePluginLicenseInvalid
	}

	var (
		license    *models.PluginLicense
		activation *models.PluginLicenseActivation
		err        error
	)
	switch {
	case strings.TrimSpace(activationToken) != "":
		activation, err = s.repo.GetLicenseActivationByToken(strings.TrimSpace(activationToken))
		if err != nil {
			return nil, err
		}
		if activation == nil {
			return nil, ErrOnlinePluginLicenseTokenInvalid
		}
		license, err = s.repo.GetLicenseByLicenseID(activation.LicenseID)
		if err != nil {
			return nil, err
		}
		if license == nil {
			return nil, ErrOnlinePluginLicenseNotFound
		}
	case strings.TrimSpace(licenseKey) != "":
		license, _, _, err = s.resolveLicenseByKey(normalizedPluginID, licenseKey, normalizedDomain, normalizedServerIP)
		if err != nil {
			return nil, err
		}
		if normalizedInstallID != "" {
			activation, err = s.repo.GetLicenseActivationByInstallID(license.LicenseID, normalizedInstallID)
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, ErrOnlinePluginLicenseInvalid
	}
	if license.PluginID != normalizedPluginID {
		return nil, ErrOnlinePluginLicenseNotFound
	}

	decision := s.buildDecision(license, normalizedDomain, normalizedServerIP, now)
	if activation != nil {
		nowCopy := now
		activation.SiteID = pickFirstNonEmpty(activation.SiteID, normalizedDomain)
		activation.InstallID = pickFirstNonEmpty(activation.InstallID, normalizedInstallID)
		activation.ReportedDomain = normalizedDomain
		activation.ReportedIP = normalizedServerIP
		activation.LastSeenAt = &nowCopy
		if decision.MatchedDomain && decision.MatchedServerIP && decision.EnforcementMode != "disable" {
			activation.Status = "active"
			activation.ValidatedDomain = license.BoundDomain
			activation.ValidatedServerIP = license.BoundServerIP
			activation.LastHeartbeatAt = &nowCopy
		} else if decision.MatchedDomain && decision.MatchedServerIP {
			activation.Status = "revoked"
		} else {
			activation.Status = "mismatch"
		}
		if err := s.repo.SaveLicenseActivation(activation); err != nil {
			return nil, err
		}
	}

	nowCopy := now
	license.LastValidatedAt = &nowCopy
	license.Status = decision.LicenseStatus
	if err := s.repo.SaveLicense(license); err != nil {
		return nil, err
	}

	if persistHeartbeat {
		heartbeat := &models.PluginLicenseHeartbeat{
			LicenseID:         license.LicenseID,
			ReportedDomain:    normalizedDomain,
			ReportedIP:        normalizedServerIP,
			ReportedVersion:   strings.TrimSpace(currentVersion),
			RuntimeLoaded:     runtimeLoaded,
			MatchedDomain:     decision.MatchedDomain,
			MatchedServerIP:   decision.MatchedServerIP,
			LicenseStatus:     decision.LicenseStatus,
			EnforcementAction: decision.EnforcementMode,
			Message:           decision.Message,
			CreatedAt:         now,
		}
		if activation != nil {
			heartbeat.ActivationID = activation.ID
		}
		if err := s.repo.CreateLicenseHeartbeat(heartbeat); err != nil {
			return nil, err
		}
	}
	return decision, nil
}

func (s *OnlinePluginLicenseService) buildDecision(license *models.PluginLicense, reportedDomain, reportedServerIP string, now time.Time) *OnlinePluginLicenseDecisionResponse {
	status := deriveLicenseRuntimeStatus(license, now)
	matchedDomain := license != nil && strings.TrimSpace(license.BoundDomain) != "" && strings.EqualFold(strings.TrimSpace(license.BoundDomain), strings.TrimSpace(reportedDomain))
	matchedServerIP := license != nil && strings.TrimSpace(license.BoundServerIP) != "" && strings.TrimSpace(license.BoundServerIP) == strings.TrimSpace(reportedServerIP)
	enforcement := "ok"
	message := "授权校验通过"
	switch {
	case !matchedDomain && strings.TrimSpace(license.BoundDomain) != "":
		enforcement = "disable"
		message = "授权域名不匹配"
	case !matchedServerIP && strings.TrimSpace(license.BoundServerIP) != "":
		enforcement = "disable"
		message = "授权服务器 IP 不匹配"
	case status == "grace":
		enforcement = "warn"
		message = "授权处于宽限期，请尽快续费或完成处理"
	case status == "expired":
		enforcement = "disable"
		message = "授权已过期"
	case status == "revoked":
		enforcement = "disable"
		message = "授权已被撤销"
	case status == "suspended":
		enforcement = "disable"
		message = "授权已被暂停"
	case status == "pending":
		enforcement = "warn"
		message = "授权尚未激活绑定"
	}
	result := &OnlinePluginLicenseDecisionResponse{
		LicenseID:                 license.LicenseID,
		PluginID:                  license.PluginID,
		LicenseStatus:             status,
		LicenseMode:               license.LicenseMode,
		BoundDomain:               license.BoundDomain,
		BoundServerIP:             license.BoundServerIP,
		MatchedDomain:             matchedDomain || strings.TrimSpace(license.BoundDomain) == "",
		MatchedServerIP:           matchedServerIP || strings.TrimSpace(license.BoundServerIP) == "",
		ExpireAt:                  normalizeOptionalTime(license.ExpireAt),
		GraceDeadlineAt:           normalizeOptionalTime(license.GraceDeadlineAt),
		FeatureFlags:              license.FeatureFlagsJSON,
		EnforcementMode:           enforcement,
		Message:                   message,
		NextHeartbeatAfterSeconds: onlinePluginLicenseHeartbeatSeconds,
	}
	return result
}

func normalizeLicenseStatus(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "pending", "active", "grace", "expired", "revoked", "suspended":
		return value
	default:
		if value == "" {
			return "pending"
		}
		return "pending"
	}
}

func normalizeLicenseStatusOptional(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	return normalizeLicenseStatus(value)
}

func normalizeBillingModeOptional(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	return normalizeBillingMode(value)
}

func normalizeLicenseID(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	return strings.ToLower(strings.ReplaceAll(value, " ", "-"))
}

func normalizeLicenseKey(raw string) string {
	value := strings.ToUpper(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func normalizeDomain(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimSuffix(value, "/")
	if idx := strings.Index(value, "/"); idx >= 0 {
		value = value[:idx]
	}
	return value
}

func normalizeServerIP(raw string) string {
	return strings.TrimSpace(raw)
}

func normalizeLicenseExpireAt(mode string, raw *time.Time, plan *models.PluginMarketPlan) *time.Time {
	switch normalizeBillingMode(mode) {
	case "annual":
		if raw != nil {
			return normalizeOptionalTime(raw)
		}
		days := 365
		if plan != nil && plan.DurationDays != nil && *plan.DurationDays > 0 {
			days = *plan.DurationDays
		}
		expires := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)
		return &expires
	case "free", "perpetual":
		return nil
	default:
		return normalizeOptionalTime(raw)
	}
}

func normalizeOptionalTime(raw *time.Time) *time.Time {
	if raw == nil {
		return nil
	}
	normalized := raw.UTC()
	return &normalized
}

func deriveLicenseRuntimeStatus(item *models.PluginLicense, now time.Time) string {
	if item == nil {
		return "pending"
	}
	status := normalizeLicenseStatus(item.Status)
	switch status {
	case "revoked", "suspended":
		return status
	}
	if item.ExpireAt != nil && now.After(item.ExpireAt.UTC()) {
		if item.GraceDeadlineAt != nil && now.Before(item.GraceDeadlineAt.UTC()) {
			return "grace"
		}
		return "expired"
	}
	if status == "grace" {
		if item.GraceDeadlineAt != nil && now.Before(item.GraceDeadlineAt.UTC()) {
			return "grace"
		}
		return "active"
	}
	return status
}

func isBlockingLicenseStatus(status string) bool {
	switch normalizeLicenseStatus(status) {
	case "pending", "active", "grace", "suspended":
		return true
	default:
		return false
	}
}

func generateLicenseID() string {
	return "lic_" + strings.ReplaceAll(strings.ToLower(uuid.NewString()), "-", "")
}

func generateLicenseKey() string {
	id := strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))
	if len(id) < 24 {
		return "LIC-" + id
	}
	return fmt.Sprintf("LIC-%s-%s-%s", id[0:8], id[8:16], id[16:24])
}

func generateActivationToken() string {
	return "act_" + strings.ReplaceAll(strings.ToLower(uuid.NewString()+uuid.NewString()), "-", "")
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

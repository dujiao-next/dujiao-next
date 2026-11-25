package adminhttp

import (
	"context"
	"strings"
	"time"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	resellermodule "github.com/dujiao-next/internal/modules/reseller/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	dto "github.com/dujiao-next/internal/modules/reseller/transport/http/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type AdminManagementService interface {
	ApproveProfile(ctx context.Context, adminID, profileID uint, input resellermodule.ResellerApproveInput) (*resellermodule.ResellerApproveResult, error)
	RejectProfile(adminID, profileID uint, reason string) (*resellerdomain.Profile, error)
	DisableProfile(adminID, profileID uint, reason string) (*resellerdomain.Profile, error)
	RestoreProfile(adminID, profileID uint) (*resellerdomain.Profile, error)
	UpdateProfileOperationalConfig(adminID, profileID uint, input resellermodule.ResellerProfileUpdateInput) (*resellerdomain.Profile, error)
	AssignSystemSubdomain(ctx context.Context, adminID, profileID uint, input resellermodule.ResellerSystemDomainInput) (*resellerdomain.Domain, error)
	ApproveDomain(ctx context.Context, adminID, domainID uint) (*resellerdomain.Domain, error)
	DisableDomain(ctx context.Context, adminID, domainID uint) (*resellerdomain.Domain, error)
	SetPrimaryDomain(ctx context.Context, adminID, domainID uint) (*resellerdomain.Domain, error)
}

type AuditRecorder interface {
	Record(input auditlogapp.AuthzRecord) error
}

type ProfileDirectory interface {
	ListProfiles(filter resellercontract.ProfileListFilter) ([]resellerdomain.Profile, int64, error)
	ListDomains(filter resellercontract.DomainListFilter) ([]resellerdomain.Domain, int64, error)
}

type DomainHTTPSProvisioner interface {
	Setup(context.Context, uint) (string, error)
}

type AdminManagementHandler struct {
	management  AdminManagementService
	directory   ProfileDirectory
	audit       AuditRecorder
	domainHTTPS DomainHTTPSProvisioner
}

func NewAdminManagementHandler(management AdminManagementService, directory ProfileDirectory, audit AuditRecorder, domainHTTPS DomainHTTPSProvisioner) *AdminManagementHandler {
	if management == nil || directory == nil {
		panic("reseller admin management handler: required dependency is nil")
	}
	return &AdminManagementHandler{management: management, directory: directory, audit: audit, domainHTTPS: domainHTTPS}
}

type profileApproveRequest struct {
	DefaultMarkupPercent string `json:"default_markup_percent"`
	MaxMarkupPercent     string `json:"max_markup_percent"`
}

type profileReasonRequest struct {
	Reason string `json:"reason"`
}

type profileUpdateRequest struct {
	DefaultMarkupPercent string `json:"default_markup_percent"`
	MaxMarkupPercent     string `json:"max_markup_percent"`
	SettlementStatus     string `json:"settlement_status"`
	Reason               string `json:"reason"`
}

type systemDomainRequest struct {
	Subdomain string `json:"subdomain"`
	Domain    string `json:"domain"`
}

// ListProfiles 管理端分销商资料列表。
func (h *AdminManagementHandler) ListProfiles(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	userID, _ := ginutil.ParseQueryUint(c.Query("user_id"), false)
	rows, total, err := h.directory.ListProfiles(resellercontract.ProfileListFilter{
		Page:             page,
		PageSize:         pageSize,
		UserID:           userID,
		Status:           strings.TrimSpace(c.Query("status")),
		SettlementStatus: strings.TrimSpace(c.Query("settlement_status")),
		Keyword:          strings.TrimSpace(c.Query("keyword")),
		CreatedFrom:      parseTimePointer(c.Query("created_from")),
		CreatedTo:        parseTimePointer(c.Query("created_to")),
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, pageSize, total))
}

// ListDomains 管理端分销域名列表。
func (h *AdminManagementHandler) ListDomains(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	resellerID, _ := ginutil.ParseQueryUint(c.Query("reseller_id"), false)
	userID, _ := ginutil.ParseQueryUint(c.Query("user_id"), false)
	rows, total, err := h.directory.ListDomains(resellercontract.DomainListFilter{
		Page:               page,
		PageSize:           pageSize,
		ResellerID:         resellerID,
		UserID:             userID,
		Domain:             strings.TrimSpace(c.Query("domain")),
		Type:               strings.TrimSpace(c.Query("type")),
		Status:             strings.TrimSpace(c.Query("status")),
		VerificationStatus: strings.TrimSpace(c.Query("verification_status")),
		Keyword:            strings.TrimSpace(c.Query("keyword")),
		CreatedFrom:        parseTimePointer(c.Query("created_from")),
		CreatedTo:          parseTimePointer(c.Query("created_to")),
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, pageSize, total))
}

// ApproveProfile 审核通过分销商资料。
func (h *AdminManagementHandler) ApproveProfile(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req profileApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	defaultMarkup, err := parseOptionalDecimal(req.DefaultMarkupPercent)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	maxMarkup, err := parseOptionalDecimal(req.MaxMarkupPercent)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	result, err := h.management.ApproveProfile(c.Request.Context(), adminID, id, resellermodule.ResellerApproveInput{
		DefaultMarkupPercent: defaultMarkup,
		MaxMarkupPercent:     maxMarkup,
	})
	if err != nil {
		respondAdminManagementError(c, err)
		return
	}
	h.recordAudit(c, "reseller_profile_approve", "/admin/resellers/profiles/:id/approve", gin.H{
		"profile_id":  id,
		"reseller_id": id,
		"next_status": resellerdomain.ProfileStatusActive,
	})
	var systemDomain any
	if result.SystemDomain != nil {
		systemDomain = dto.NewResellerDomainResp(result.SystemDomain)
	}
	response.Success(c, gin.H{"profile": result.Profile, "system_domain": systemDomain})
}

// UpdateProfile 更新分销商运营配置。
func (h *AdminManagementHandler) UpdateProfile(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req profileUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	defaultMarkup, err := parseOptionalDecimal(req.DefaultMarkupPercent)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	maxMarkup, err := parseOptionalDecimal(req.MaxMarkupPercent)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	row, err := h.management.UpdateProfileOperationalConfig(adminID, id, resellermodule.ResellerProfileUpdateInput{
		DefaultMarkupPercent: defaultMarkup,
		MaxMarkupPercent:     maxMarkup,
		SettlementStatus:     req.SettlementStatus,
		Reason:               req.Reason,
	})
	if err != nil {
		respondAdminManagementError(c, err)
		return
	}
	h.recordAudit(c, "reseller_profile_update", "/admin/resellers/profiles/:id", gin.H{
		"profile_id":             id,
		"reseller_id":            id,
		"default_markup_percent": row.DefaultMarkupPercent.String(),
		"max_markup_percent":     row.MaxMarkupPercent.String(),
		"settlement_status":      row.SettlementStatus,
		"reason":                 strings.TrimSpace(req.Reason),
	})
	response.Success(c, dto.NewAdminResellerProfileResp(row))
}

// AssignSystemDomain 为分销商分配或编辑系统二级域名。
func (h *AdminManagementHandler) AssignSystemDomain(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req systemDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	rawSubdomain := strings.TrimSpace(req.Subdomain)
	if rawSubdomain == "" {
		rawSubdomain = strings.TrimSpace(req.Domain)
	}
	row, err := h.management.AssignSystemSubdomain(c.Request.Context(), adminID, id, resellermodule.ResellerSystemDomainInput{
		Subdomain: rawSubdomain,
	})
	if err != nil {
		respondAdminManagementError(c, err)
		return
	}
	h.recordAudit(c, "reseller_profile_system_domain_update", "/admin/resellers/profiles/:id/system-domain", gin.H{
		"profile_id":  id,
		"reseller_id": id,
		"domain_id":   row.ID,
		"domain":      row.Domain,
	})
	response.Success(c, dto.NewResellerDomainResp(row))
}

// RejectProfile 拒绝分销商申请。
func (h *AdminManagementHandler) RejectProfile(c *gin.Context) {
	h.handleProfileReasonAction(c, "reseller_profile_reject", "/admin/resellers/profiles/:id/reject", h.management.RejectProfile)
}

// DisableProfile 禁用分销商资料。
func (h *AdminManagementHandler) DisableProfile(c *gin.Context) {
	h.handleProfileReasonAction(c, "reseller_profile_disable", "/admin/resellers/profiles/:id/disable", h.management.DisableProfile)
}

// RestoreProfile 恢复分销商资料。
func (h *AdminManagementHandler) RestoreProfile(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	row, err := h.management.RestoreProfile(adminID, id)
	if err != nil {
		respondAdminManagementError(c, err)
		return
	}
	h.recordAudit(c, "reseller_profile_restore", "/admin/resellers/profiles/:id/restore", gin.H{
		"profile_id":  id,
		"reseller_id": id,
		"next_status": row.Status,
	})
	response.Success(c, row)
}

func (h *AdminManagementHandler) SetupDomainHTTPS(c *gin.Context) {
	if _, ok := ginutil.GetAdminID(c); !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil || id == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if h.domainHTTPS == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.reseller_https_unavailable", nil)
		return
	}
	result, err := h.domainHTTPS.Setup(c.Request.Context(), id)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.reseller_https_unavailable", err)
		return
	}
	switch result {
	case "ready":
		h.recordAudit(c, "reseller_domain_setup_https", "/admin/resellers/domains/:id/setup-https", gin.H{"domain_id": id})
		response.Success(c, gin.H{"https_ready": true})
	case "dns":
		ginutil.RespondError(c, response.CodeBadRequest, "error.reseller_https_dns", nil)
	case "conflict":
		ginutil.RespondError(c, response.CodeBadRequest, "error.reseller_https_conflict", nil)
	case "busy":
		ginutil.RespondError(c, response.CodeBadRequest, "error.reseller_https_busy", nil)
	case "invalid":
		ginutil.RespondError(c, response.CodeBadRequest, "error.reseller_domain_invalid", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.reseller_https_failed", nil)
	}
}

// ApproveDomain 审核通过自定义域名。
func (h *AdminManagementHandler) ApproveDomain(c *gin.Context) {
	h.handleDomainAction(c, "reseller_domain_approve", "/admin/resellers/domains/:id/approve", h.management.ApproveDomain)
}

// DisableDomain 禁用域名。
func (h *AdminManagementHandler) DisableDomain(c *gin.Context) {
	h.handleDomainAction(c, "reseller_domain_disable", "/admin/resellers/domains/:id/disable", h.management.DisableDomain)
}

// SetPrimaryDomain 将已启用且已验证域名设为主域名。
func (h *AdminManagementHandler) SetPrimaryDomain(c *gin.Context) {
	h.handleDomainAction(c, "reseller_domain_set_primary", "/admin/resellers/domains/:id/set-primary", h.management.SetPrimaryDomain)
}

func (h *AdminManagementHandler) handleProfileReasonAction(
	c *gin.Context,
	action string,
	object string,
	fn func(adminID, profileID uint, reason string) (*resellerdomain.Profile, error),
) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	if fn == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.save_failed", nil)
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req profileReasonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	row, err := fn(adminID, id, req.Reason)
	if err != nil {
		respondAdminManagementError(c, err)
		return
	}
	h.recordAudit(c, action, object, gin.H{
		"profile_id":  id,
		"reseller_id": id,
		"next_status": row.Status,
	})
	response.Success(c, row)
}

func (h *AdminManagementHandler) handleDomainAction(
	c *gin.Context,
	action string,
	object string,
	fn func(ctx context.Context, adminID, domainID uint) (*resellerdomain.Domain, error),
) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	if fn == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.save_failed", nil)
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	row, err := fn(c.Request.Context(), adminID, id)
	if err != nil {
		respondAdminManagementError(c, err)
		return
	}
	h.recordAudit(c, action, object, gin.H{
		"domain_id":   id,
		"reseller_id": row.ResellerID,
		"domain":      row.Domain,
		"next_status": row.Status,
	})
	response.Success(c, dto.NewResellerDomainResp(row))
}

func (h *AdminManagementHandler) recordAudit(c *gin.Context, action string, object string, detail gin.H) {
	if h == nil || h.audit == nil {
		return
	}
	method := "POST"
	if c.Request != nil && c.Request.Method != "" {
		method = c.Request.Method
	}
	_ = h.audit.Record(auditlogapp.AuthzRecord{
		OperatorAdminID:  c.GetUint("admin_id"),
		OperatorUsername: c.GetString("username"),
		Action:           action,
		Object:           object,
		Method:           method,
		RequestID:        strings.TrimSpace(c.GetString("request_id")),
		Detail:           jsonmap.JSON(detail),
	})
}

func parseOptionalDecimal(raw string) (decimal.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(raw)
}

func parseTimePointer(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return &parsed
	}
	return nil
}

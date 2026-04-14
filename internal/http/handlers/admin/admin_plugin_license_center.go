package admin

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

type adminPluginLicenseRequest struct {
	PluginID        string                 `json:"plugin_id" binding:"required"`
	PlanCode        string                 `json:"plan_code"`
	CustomerID      uint                   `json:"customer_id"`
	OrderID         uint                   `json:"order_id"`
	LicenseID       string                 `json:"license_id"`
	LicenseKey      string                 `json:"license_key"`
	LicenseMode     string                 `json:"license_mode"`
	Status          string                 `json:"status"`
	BoundDomain     string                 `json:"bound_domain"`
	BoundServerIP   string                 `json:"bound_server_ip"`
	ExpireAt        string                 `json:"expire_at"`
	GraceDeadlineAt string                 `json:"grace_deadline_at"`
	FeatureFlags    map[string]interface{} `json:"feature_flags"`
	Meta            map[string]interface{} `json:"meta"`
}

// ListPluginLicenseCenterLicenses 获取授权列表。
func (h *Handler) ListPluginLicenseCenterLicenses(c *gin.Context) {
	page, pageSize := shared.NormalizePagination(parseIntDefault(c.Query("page"), 1), parseIntDefault(c.Query("page_size"), 20))
	items, total, err := h.OnlinePluginLicenseService.ListLicenses(service.OnlinePluginLicenseListFilter{
		Page:        page,
		PageSize:    pageSize,
		Keyword:     strings.TrimSpace(c.Query("keyword")),
		PluginID:    strings.TrimSpace(c.Query("plugin_id")),
		Status:      strings.TrimSpace(c.Query("status")),
		LicenseMode: strings.TrimSpace(c.Query("license_mode")),
	})
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "获取插件授权列表失败", err)
		return
	}
	response.SuccessWithPage(c, items, response.BuildPagination(page, pageSize, total))
}

// GetPluginLicenseCenterLicense 获取授权详情。
func (h *Handler) GetPluginLicenseCenterLicense(c *gin.Context) {
	item, err := h.OnlinePluginLicenseService.GetLicenseDetail(strings.TrimSpace(c.Param("license_id")))
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "获取插件授权详情失败", err)
		return
	}
	if item == nil {
		shared.RespondErrorWithMsg(c, response.CodeNotFound, "插件授权不存在", nil)
		return
	}
	response.Success(c, item)
}

// CreatePluginLicenseCenterLicense 创建授权。
func (h *Handler) CreatePluginLicenseCenterLicense(c *gin.Context) {
	var req adminPluginLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	expireAt, err := parseAdminRFC3339Time(req.ExpireAt)
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "到期时间格式错误，请使用 RFC3339", err)
		return
	}
	graceDeadlineAt, err := parseAdminRFC3339Time(req.GraceDeadlineAt)
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "宽限截止时间格式错误，请使用 RFC3339", err)
		return
	}
	item, err := h.OnlinePluginLicenseService.CreateLicense(service.OnlinePluginLicenseAdminInput{
		PluginID:        req.PluginID,
		PlanCode:        req.PlanCode,
		CustomerID:      req.CustomerID,
		OrderID:         req.OrderID,
		LicenseID:       req.LicenseID,
		LicenseKey:      req.LicenseKey,
		LicenseMode:     req.LicenseMode,
		Status:          req.Status,
		BoundDomain:     req.BoundDomain,
		BoundServerIP:   req.BoundServerIP,
		ExpireAt:        expireAt,
		GraceDeadlineAt: graceDeadlineAt,
		FeatureFlags:    req.FeatureFlags,
		Meta:            req.Meta,
	})
	if err != nil {
		respondPluginLicenseCenterError(c, err, "创建插件授权失败")
		return
	}
	response.Success(c, item)
}

// UpdatePluginLicenseCenterLicense 更新授权。
func (h *Handler) UpdatePluginLicenseCenterLicense(c *gin.Context) {
	var req adminPluginLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	expireAt, err := parseAdminRFC3339Time(req.ExpireAt)
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "到期时间格式错误，请使用 RFC3339", err)
		return
	}
	graceDeadlineAt, err := parseAdminRFC3339Time(req.GraceDeadlineAt)
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "宽限截止时间格式错误，请使用 RFC3339", err)
		return
	}
	item, err := h.OnlinePluginLicenseService.UpdateLicense(strings.TrimSpace(c.Param("license_id")), service.OnlinePluginLicenseAdminInput{
		PluginID:        req.PluginID,
		PlanCode:        req.PlanCode,
		CustomerID:      req.CustomerID,
		OrderID:         req.OrderID,
		LicenseID:       req.LicenseID,
		LicenseKey:      req.LicenseKey,
		LicenseMode:     req.LicenseMode,
		Status:          req.Status,
		BoundDomain:     req.BoundDomain,
		BoundServerIP:   req.BoundServerIP,
		ExpireAt:        expireAt,
		GraceDeadlineAt: graceDeadlineAt,
		FeatureFlags:    req.FeatureFlags,
		Meta:            req.Meta,
	})
	if err != nil {
		respondPluginLicenseCenterError(c, err, "更新插件授权失败")
		return
	}
	response.Success(c, item)
}

func respondPluginLicenseCenterError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrOnlinePluginLicenseNotFound):
		shared.RespondErrorWithMsg(c, response.CodeNotFound, "插件授权不存在", nil)
	case errors.Is(err, service.ErrOnlinePluginLicenseInvalid):
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "插件授权参数无效，请检查后重试", nil)
	case errors.Is(err, service.ErrOnlinePluginLicenseConflict):
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "域名或服务器 IP 已被其他有效授权占用", nil)
	case errors.Is(err, service.ErrOnlinePluginLicenseAlreadyActivated):
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "该授权已被其他安装实例占用", nil)
	default:
		shared.RespondErrorWithMsg(c, response.CodeInternal, fallback, err)
	}
}

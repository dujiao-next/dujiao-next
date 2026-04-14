package public

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

type publicPluginLicenseActivateRequest struct {
	PluginID        string `json:"plugin_id" binding:"required"`
	LicenseKey      string `json:"license_key" binding:"required"`
	InstallID       string `json:"install_id" binding:"required"`
	HostFingerprint string `json:"host_fingerprint"`
	PrimaryDomain   string `json:"primary_domain" binding:"required"`
	ServerIP        string `json:"server_ip" binding:"required"`
	CurrentVersion  string `json:"current_version"`
}

type publicPluginLicenseValidateRequest struct {
	PluginID        string `json:"plugin_id" binding:"required"`
	LicenseKey      string `json:"license_key"`
	ActivationToken string `json:"activation_token"`
	InstallID       string `json:"install_id"`
	PrimaryDomain   string `json:"primary_domain" binding:"required"`
	ServerIP        string `json:"server_ip" binding:"required"`
	CurrentVersion  string `json:"current_version"`
	RuntimeLoaded   *bool  `json:"runtime_loaded"`
}

type publicPluginLicenseHeartbeatRequest struct {
	PluginID        string `json:"plugin_id" binding:"required"`
	ActivationToken string `json:"activation_token"`
	InstallID       string `json:"install_id"`
	PrimaryDomain   string `json:"primary_domain" binding:"required"`
	ServerIP        string `json:"server_ip" binding:"required"`
	CurrentVersion  string `json:"current_version"`
	RuntimeLoaded   *bool  `json:"runtime_loaded"`
}

// ActivatePluginLicense 首次激活插件授权。
func (h *Handler) ActivatePluginLicense(c *gin.Context) {
	var req publicPluginLicenseActivateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	item, err := h.OnlinePluginLicenseService.ActivateLicense(service.OnlinePluginLicenseActivateInput{
		PluginID:        req.PluginID,
		LicenseKey:      req.LicenseKey,
		InstallID:       req.InstallID,
		HostFingerprint: req.HostFingerprint,
		PrimaryDomain:   req.PrimaryDomain,
		ServerIP:        req.ServerIP,
		CurrentVersion:  req.CurrentVersion,
	})
	if err != nil {
		respondPluginLicensePublicError(c, err, "激活插件授权失败")
		return
	}
	response.Success(c, item)
}

// ValidatePluginLicense 手动校验插件授权。
func (h *Handler) ValidatePluginLicense(c *gin.Context) {
	var req publicPluginLicenseValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	token := pickPluginLicenseFirstNonEmpty(req.ActivationToken, bearerTokenFromHeader(c.GetHeader("Authorization")))
	item, err := h.OnlinePluginLicenseService.ValidateLicense(service.OnlinePluginLicenseValidateInput{
		PluginID:        req.PluginID,
		LicenseKey:      req.LicenseKey,
		ActivationToken: token,
		InstallID:       req.InstallID,
		PrimaryDomain:   req.PrimaryDomain,
		ServerIP:        req.ServerIP,
		CurrentVersion:  req.CurrentVersion,
		RuntimeLoaded:   req.RuntimeLoaded == nil || *req.RuntimeLoaded,
	})
	if err != nil {
		respondPluginLicensePublicError(c, err, "校验插件授权失败")
		return
	}
	response.Success(c, item)
}

// HeartbeatPluginLicense 上报插件授权心跳。
func (h *Handler) HeartbeatPluginLicense(c *gin.Context) {
	var req publicPluginLicenseHeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	token := pickPluginLicenseFirstNonEmpty(req.ActivationToken, bearerTokenFromHeader(c.GetHeader("Authorization")))
	item, err := h.OnlinePluginLicenseService.ReportHeartbeat(service.OnlinePluginLicenseHeartbeatInput{
		PluginID:        req.PluginID,
		ActivationToken: token,
		InstallID:       req.InstallID,
		PrimaryDomain:   req.PrimaryDomain,
		ServerIP:        req.ServerIP,
		CurrentVersion:  req.CurrentVersion,
		RuntimeLoaded:   req.RuntimeLoaded == nil || *req.RuntimeLoaded,
	})
	if err != nil {
		respondPluginLicensePublicError(c, err, "上报插件授权心跳失败")
		return
	}
	response.Success(c, item)
}

func respondPluginLicensePublicError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrOnlinePluginLicenseNotFound):
		shared.RespondErrorWithMsg(c, response.CodeNotFound, "授权不存在或插件不匹配", nil)
	case errors.Is(err, service.ErrOnlinePluginLicenseInvalid):
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "授权请求参数无效", nil)
	case errors.Is(err, service.ErrOnlinePluginLicenseConflict):
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "域名或服务器 IP 与授权绑定不一致", nil)
	case errors.Is(err, service.ErrOnlinePluginLicenseAlreadyActivated):
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "该授权已被其他安装实例占用", nil)
	case errors.Is(err, service.ErrOnlinePluginLicenseTokenInvalid):
		shared.RespondErrorWithMsg(c, response.CodeUnauthorized, "激活令牌无效或已失效", nil)
	case errors.Is(err, service.ErrOnlinePluginLicenseInactive):
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, "授权当前不可用", nil)
	default:
		shared.RespondErrorWithMsg(c, response.CodeInternal, fallback, err)
	}
}

func bearerTokenFromHeader(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return value
}

func pickPluginLicenseFirstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

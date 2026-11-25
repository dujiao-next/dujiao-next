package adminapitokenhttp

import (
	"errors"
	"time"

	tokenapp "github.com/dujiao-next/internal/modules/identity/adminapitoken/application"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

type AuditEvent struct {
	OperatorID uint
	Username   string
	AdminID    uint
	TokenID    uint
	Action     string
	RequestID  string
}

type Handler struct {
	service *tokenapp.Service
	audit   func(AuditEvent)
}

func NewHandler(service *tokenapp.Service, audit func(AuditEvent)) *Handler {
	return &Handler{service: service, audit: audit}
}

func targetAdmin(c *gin.Context) (uint, bool) {
	c.Header("Cache-Control", "no-store")
	if c.GetUint("admin_api_token_id") != 0 {
		ginutil.RespondError(c, response.CodeForbidden, "error.forbidden", nil)
		return 0, false
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil || id == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_id_invalid", nil)
		return 0, false
	}
	return id, true
}

func (h *Handler) List(c *gin.Context) {
	id, ok := targetAdmin(c)
	if !ok {
		return
	}
	items, err := h.service.List(id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, items)
}

func (h *Handler) Create(c *gin.Context) {
	id, ok := targetAdmin(c)
	if !ok {
		return
	}
	var req struct {
		Name        string    `json:"name" binding:"required"`
		ExpiresAt   time.Time `json:"expires_at" binding:"required"`
		IPWhitelist []string  `json:"ip_whitelist"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_api_token_input", nil)
		return
	}
	token, secret, err := h.service.Create(id, req.Name, req.ExpiresAt, req.IPWhitelist)
	if err != nil {
		respondError(c, err)
		return
	}
	h.record(c, id, token.ID, "admin_api_token_create")
	response.Success(c, gin.H{"token": token, "secret": secret})
}

func (h *Handler) Revoke(c *gin.Context) {
	id, ok := targetAdmin(c)
	if !ok {
		return
	}
	tokenID, err := ginutil.ParseParamUint(c, "token_id")
	if err != nil || tokenID == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if err := h.service.Revoke(id, tokenID); err != nil {
		respondError(c, err)
		return
	}
	h.record(c, id, tokenID, "admin_api_token_revoke")
	response.Success(c, nil)
}

func (h *Handler) Rotate(c *gin.Context) {
	id, ok := targetAdmin(c)
	if !ok {
		return
	}
	tokenID, err := ginutil.ParseParamUint(c, "token_id")
	if err != nil || tokenID == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	token, secret, err := h.service.Rotate(id, tokenID)
	if err != nil {
		respondError(c, err)
		return
	}
	h.record(c, id, tokenID, "admin_api_token_rotate")
	response.Success(c, gin.H{"token": token, "secret": secret})
}

func (h *Handler) record(c *gin.Context, adminID, tokenID uint, action string) {
	if h.audit != nil {
		h.audit(AuditEvent{
			OperatorID: c.GetUint("admin_id"), Username: c.GetString("username"),
			AdminID: adminID, TokenID: tokenID, Action: action, RequestID: c.GetString("request_id"),
		})
	}
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, tokenapp.ErrIPWhitelist):
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_api_token_ip_whitelist", nil)
	case errors.Is(err, tokenapp.ErrInput):
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_api_token_input", nil)
	case errors.Is(err, tokenapp.ErrSubAccount):
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_api_token_subaccount", nil)
	case errors.Is(err, tokenapp.ErrNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.admin_api_token_not_found", nil)
	case errors.Is(err, tokenapp.ErrRotation):
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_api_token_rotation", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.internal_error", err)
	}
}

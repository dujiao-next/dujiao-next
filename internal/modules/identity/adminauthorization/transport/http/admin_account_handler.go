package adminauthzhttp

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dujiao-next/internal/logger"
	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

const protectedSuperAdminUsername = "admin"

type authzCreateAdminPayload struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	IsSuper  *bool  `json:"is_super"`
}

type authzUpdateAdminPayload struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
	IsSuper  *bool   `json:"is_super"`
}

// CreateAuthzAdmin 创建管理员
func (h *AdminHandler) CreateAuthzAdmin(c *gin.Context) {
	var req authzCreateAdminPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	username, err := normalizeAdminUsername(req.Username)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_username_invalid", err)
		return
	}
	if rejectNonSuperEscalation(c, (req.IsSuper != nil && *req.IsSuper) || isProtectedSuperAdminUsername(username)) {
		return
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.password_weak", nil)
		return
	}

	existing, err := h.admins.GetByUsername(username)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.admin_create_failed", err)
		return
	}
	if existing != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_username_exists", nil)
		return
	}

	if err := h.passwords.ValidatePassword(password); err != nil {
		if errors.Is(err, ErrWeakPassword) {
			respondWeakPassword(c, err)
			return
		}
		ginutil.RespondError(c, response.CodeBadRequest, "error.password_weak", err)
		return
	}

	hash, err := h.passwords.HashPassword(password)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.admin_create_failed", err)
		return
	}

	isSuper := req.IsSuper != nil && *req.IsSuper
	if strings.EqualFold(username, protectedSuperAdminUsername) {
		isSuper = true
	}

	admin := &admindomain.Admin{
		Username:     username,
		PasswordHash: hash,
		IsSuper:      isSuper,
	}
	if err := h.admins.Create(admin); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.admin_create_failed", err)
		return
	}

	_ = h.authState.SetAdminAuthState(c.Request.Context(), admin)

	h.recordAuthzAudit(c, auditlogapp.AuthzRecord{
		OperatorAdminID:  c.GetUint("admin_id"),
		OperatorUsername: strings.TrimSpace(c.GetString("username")),
		TargetAdminID:    &admin.ID,
		TargetUsername:   admin.Username,
		Action:           "admin_create",
		RequestID:        strings.TrimSpace(c.GetString("request_id")),
		Detail: jsonmap.JSON{
			"target_admin_id": admin.ID,
			"target_username": admin.Username,
			"is_super":        admin.IsSuper,
		},
	})

	logger.Infow("admin_authz_admin_created",
		"operator_admin_id", c.GetUint("admin_id"),
		"target_admin_id", admin.ID,
		"target_username", admin.Username,
		"is_super", admin.IsSuper,
	)

	response.Success(c, admin)
}

// UpdateAuthzAdmin 更新管理员
func (h *AdminHandler) UpdateAuthzAdmin(c *gin.Context) {
	adminID, ok := parseAdminIDParam(c)
	if !ok {
		return
	}

	admin, err := h.admins.GetByID(adminID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.admin_update_failed", err)
		return
	}
	if admin == nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_id_invalid", nil)
		return
	}

	var req authzUpdateAdminPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if rejectNonSuperEscalation(c, admin.IsSuper ||
		(req.IsSuper != nil && *req.IsSuper) ||
		(req.Username != nil && isProtectedSuperAdminUsername(*req.Username))) {
		return
	}

	updatedFields := make([]string, 0, 3)

	if req.Username != nil {
		normalizedUsername, err := normalizeAdminUsername(*req.Username)
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.admin_username_invalid", err)
			return
		}
		if normalizedUsername != admin.Username {
			existing, err := h.admins.GetByUsername(normalizedUsername)
			if err != nil {
				ginutil.RespondError(c, response.CodeInternal, "error.admin_update_failed", err)
				return
			}
			if existing != nil && existing.ID != admin.ID {
				ginutil.RespondError(c, response.CodeBadRequest, "error.admin_username_exists", nil)
				return
			}
			admin.Username = normalizedUsername
			updatedFields = append(updatedFields, "username")
		}
	}

	if req.IsSuper != nil {
		nextIsSuper := *req.IsSuper
		if strings.EqualFold(strings.TrimSpace(admin.Username), protectedSuperAdminUsername) {
			nextIsSuper = true
		}
		if admin.IsSuper != nextIsSuper {
			admin.IsSuper = nextIsSuper
			updatedFields = append(updatedFields, "is_super")
		}
	}

	if req.Password != nil {
		password := strings.TrimSpace(*req.Password)
		if password == "" {
			ginutil.RespondError(c, response.CodeBadRequest, "error.password_weak", nil)
			return
		}
		if err := h.passwords.ValidatePassword(password); err != nil {
			if errors.Is(err, ErrWeakPassword) {
				respondWeakPassword(c, err)
				return
			}
			ginutil.RespondError(c, response.CodeBadRequest, "error.password_weak", err)
			return
		}
		hash, err := h.passwords.HashPassword(password)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.admin_update_failed", err)
			return
		}
		admin.PasswordHash = hash
		now := time.Now()
		admin.TokenVersion++
		admin.TokenInvalidBefore = &now
		updatedFields = append(updatedFields, "password")
	}

	if len(updatedFields) == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	if err := h.admins.Update(admin); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.admin_update_failed", err)
		return
	}
	_ = h.authState.SetAdminAuthState(c.Request.Context(), admin)

	sort.Strings(updatedFields)
	if c.GetUint("admin_id") == admin.ID {
		c.Set("admin_is_super", admin.IsSuper)
	}

	h.recordAuthzAudit(c, auditlogapp.AuthzRecord{
		OperatorAdminID:  c.GetUint("admin_id"),
		OperatorUsername: strings.TrimSpace(c.GetString("username")),
		TargetAdminID:    &admin.ID,
		TargetUsername:   admin.Username,
		Action:           "admin_update",
		RequestID:        strings.TrimSpace(c.GetString("request_id")),
		Detail: jsonmap.JSON{
			"target_admin_id": admin.ID,
			"target_username": admin.Username,
			"updated_fields":  updatedFields,
			"is_super":        admin.IsSuper,
		},
	})

	logger.Infow("admin_authz_admin_updated",
		"operator_admin_id", c.GetUint("admin_id"),
		"target_admin_id", admin.ID,
		"target_username", admin.Username,
		"updated_fields", updatedFields,
	)

	response.Success(c, admin)
}

// DeleteAuthzAdmin 删除管理员
func (h *AdminHandler) DeleteAuthzAdmin(c *gin.Context) {
	adminID, ok := parseAdminIDParam(c)
	if !ok {
		return
	}

	admin, err := h.admins.GetByID(adminID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.admin_delete_failed", err)
		return
	}
	if admin == nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_id_invalid", nil)
		return
	}
	if c.GetUint("admin_id") == adminID {
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_delete_self_forbidden", nil)
		return
	}
	if rejectNonSuperEscalation(c, admin.IsSuper) {
		return
	}
	if strings.EqualFold(strings.TrimSpace(admin.Username), protectedSuperAdminUsername) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_delete_protected", nil)
		return
	}

	count, err := h.admins.Count()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.admin_delete_failed", err)
		return
	}
	if count <= 1 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_delete_last_forbidden", nil)
		return
	}

	if err := h.authz.SetAdminRoles(adminID, []string{}); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.admin_delete_failed", err)
		return
	}
	if err := h.admins.Delete(adminID); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.admin_delete_failed", err)
		return
	}
	_ = h.authState.DelAdminAuthState(c.Request.Context(), adminID)

	h.recordAuthzAudit(c, auditlogapp.AuthzRecord{
		OperatorAdminID:  c.GetUint("admin_id"),
		OperatorUsername: strings.TrimSpace(c.GetString("username")),
		TargetAdminID:    &adminID,
		TargetUsername:   admin.Username,
		Action:           "admin_delete",
		RequestID:        strings.TrimSpace(c.GetString("request_id")),
		Detail: jsonmap.JSON{
			"target_admin_id": adminID,
			"target_username": admin.Username,
		},
	})

	logger.Infow("admin_authz_admin_deleted",
		"operator_admin_id", c.GetUint("admin_id"),
		"target_admin_id", adminID,
		"target_username", admin.Username,
	)

	response.Success(c, nil)
}

// rejectNonSuperEscalation 拦截非超管越权：RBAC 只管能不能进接口，授予超管、改动超管账号
// 以及占用保留用户名（启动时会被自动提权为超管）必须由超管本人操作。
func rejectNonSuperEscalation(c *gin.Context, touchesSuper bool) bool {
	if !touchesSuper || ginutil.IsSuperAdmin(c) {
		return false
	}
	ginutil.RespondError(c, response.CodeForbidden, "error.forbidden", nil)
	return true
}

func isProtectedSuperAdminUsername(username string) bool {
	return strings.EqualFold(strings.TrimSpace(username), protectedSuperAdminUsername)
}

func normalizeAdminUsername(username string) (string, error) {
	trimmed := strings.TrimSpace(username)
	if trimmed == "" {
		return "", fmt.Errorf("username is required")
	}
	if strings.ContainsAny(trimmed, " \t\r\n") {
		return "", fmt.Errorf("username contains whitespace")
	}
	length := len([]rune(trimmed))
	if length < 3 || length > 64 {
		return "", fmt.Errorf("username length out of range")
	}
	return trimmed, nil
}

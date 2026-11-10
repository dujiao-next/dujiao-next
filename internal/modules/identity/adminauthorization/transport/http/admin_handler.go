package adminauthzhttp

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/dujiao-next/internal/authz"
	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/logger"
	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

var ErrWeakPassword = errors.New("weak password")

// WeakPasswordError 携带可本地化的弱密码策略详情。
type WeakPasswordError struct {
	key  string
	args []interface{}
}

func NewWeakPasswordError(key string, args ...interface{}) error {
	if key == "" {
		return ErrWeakPassword
	}
	return WeakPasswordError{key: key, args: args}
}

func (e WeakPasswordError) Error() string { return e.key }

func (e WeakPasswordError) Is(target error) bool { return target == ErrWeakPassword }

func (e WeakPasswordError) Key() string { return e.key }

func (e WeakPasswordError) Args() []interface{} { return e.args }

// Policy 权限策略快照。
type Policy struct {
	Subject string `json:"subject"`
	Object  string `json:"object"`
	Action  string `json:"action"`
}

// AuthzRoleItem 是角色列表的管理端响应；immutable 由后端内置角色种子计算，
// 避免前端维护第二份角色名单。
type AuthzRoleItem struct {
	Role      string `json:"role"`
	Immutable bool   `json:"immutable"`
}

// RolePolicyService 角色与策略管理端口。
type RolePolicyService interface {
	GetAdminRoles(adminID uint) ([]string, error)
	GetAdminPolicies(adminID uint) ([]Policy, error)
	ListRoles() ([]string, error)
	EnsureRole(role string) (string, error)
	DeleteRole(role string) error
	GetRolePolicies(role string) ([]Policy, error)
	GrantRolePolicy(role, object, action string) error
	RevokeRolePolicy(role, object, action string) error
	SetAdminRoles(adminID uint, roles []string) error
}

// AdminDirectory 管理员目录端口。
type AdminDirectory interface {
	List() ([]admindomain.Admin, error)
	GetByID(id uint) (*admindomain.Admin, error)
	GetByUsername(username string) (*admindomain.Admin, error)
	Create(admin *admindomain.Admin) error
	Update(admin *admindomain.Admin) error
	Delete(id uint) error
	Count() (int64, error)
}

// PasswordService 管理员密码校验与哈希端口。
type PasswordService interface {
	ValidatePassword(password string) error
	HashPassword(password string) (string, error)
}

// AuthStateCache 管理员鉴权状态缓存端口。
type AuthStateCache interface {
	SetAdminAuthState(ctx context.Context, admin *admindomain.Admin) error
	DelAdminAuthState(ctx context.Context, adminID uint) error
}

// AuditRecorder 权限审计端口。
type AuditRecorder interface {
	Record(input auditlogapp.AuthzRecord) error
}

// AdminHandler 处理后台权限管理 HTTP。
type AdminHandler struct {
	authz     RolePolicyService
	admins    AdminDirectory
	passwords PasswordService
	authState AuthStateCache
	audit     AuditRecorder
}

func NewAdminHandler(
	authz RolePolicyService,
	admins AdminDirectory,
	passwords PasswordService,
	authState AuthStateCache,
	audit AuditRecorder,
) *AdminHandler {
	if authz == nil || admins == nil || passwords == nil || authState == nil {
		panic("admin authz handler: required dependency is nil")
	}
	return &AdminHandler{
		authz:     authz,
		admins:    admins,
		passwords: passwords,
		authState: authState,
		audit:     audit,
	}
}

func respondWeakPassword(c *gin.Context, err error) {
	if perr, ok := err.(interface {
		Key() string
		Args() []interface{}
	}); ok {
		msg := i18n.Sprintf(i18n.ResolveLocale(c), perr.Key(), perr.Args()...)
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, msg, nil)
		return
	}
	ginutil.RespondError(c, response.CodeBadRequest, "error.password_weak", nil)
}

type authzRolePayload struct {
	Role string `json:"role" binding:"required"`
}

type authzPolicyPayload struct {
	Role   string `json:"role" binding:"required"`
	Object string `json:"object" binding:"required"`
	Action string `json:"action" binding:"required"`
}

type authzSetAdminRolesPayload struct {
	Roles []string `json:"roles"`
}

func buildAuthzPolicyAuditRecord(c *gin.Context, req authzPolicyPayload, action string) auditlogapp.AuthzRecord {
	return auditlogapp.AuthzRecord{
		OperatorAdminID:  c.GetUint("admin_id"),
		OperatorUsername: strings.TrimSpace(c.GetString("username")),
		Action:           action,
		Role:             req.Role,
		Object:           req.Object,
		Method:           req.Action,
		RequestID:        strings.TrimSpace(c.GetString("request_id")),
		Detail: jsonmap.JSON{
			"role":   req.Role,
			"object": req.Object,
			"method": strings.ToUpper(strings.TrimSpace(req.Action)),
		},
	}
}

// GetAuthzMe 获取当前管理员权限快照
func (h *AdminHandler) GetAuthzMe(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}

	roles, err := h.authz.GetAdminRoles(adminID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	policies, err := h.authz.GetAdminPolicies(adminID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}

	isSuper := false
	if value, exists := c.Get("admin_is_super"); exists {
		if flag, typeOK := value.(bool); typeOK {
			isSuper = flag
		}
	}

	response.Success(c, gin.H{
		"admin_id": adminID,
		"is_super": isSuper,
		"roles":    roles,
		"policies": policies,
	})
}

// ListAuthzRoles 获取角色列表
func (h *AdminHandler) ListAuthzRoles(c *gin.Context) {
	roles, err := h.authz.ListRoles()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	includeMetadata, err := ginutil.ParseQueryBool(c, "include_metadata")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if !includeMetadata {
		// 保持旧版管理端客户端使用的字符串数组响应。
		response.Success(c, buildAuthzRoleListPayload(roles, false))
		return
	}
	response.Success(c, buildAuthzRoleListPayload(roles, true))
}

func buildAuthzRoleListPayload(roles []string, includeMetadata bool) interface{} {
	if !includeMetadata {
		return roles
	}
	return buildAuthzRoleItems(roles)
}

func buildAuthzRoleItems(roles []string) []AuthzRoleItem {
	items := make([]AuthzRoleItem, 0, len(roles))
	for _, role := range roles {
		items = append(items, AuthzRoleItem{
			Role:      role,
			Immutable: authz.IsImmutableBuiltinRole(role),
		})
	}
	return items
}

// ListAuthzAdmins 获取管理员列表
func (h *AdminHandler) ListAuthzAdmins(c *gin.Context) {
	admins, err := h.admins.List()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}

	items := make([]gin.H, 0, len(admins))
	for _, admin := range admins {
		roles, roleErr := h.authz.GetAdminRoles(admin.ID)
		if roleErr != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", roleErr)
			return
		}
		items = append(items, gin.H{
			"id":              admin.ID,
			"username":        admin.Username,
			"is_super":        admin.IsSuper,
			"last_login_at":   admin.LastLoginAt,
			"created_at":      admin.CreatedAt,
			"roles":           roles,
			"totp_enabled":    admin.TOTPEnabledAt != nil,
			"totp_enabled_at": admin.TOTPEnabledAt,
		})
	}

	response.Success(c, items)
}

// CreateAuthzRole 创建角色
func (h *AdminHandler) CreateAuthzRole(c *gin.Context) {
	var req authzRolePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	role, err := h.authz.EnsureRole(req.Role)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	h.recordAuthzAudit(c, auditlogapp.AuthzRecord{
		OperatorAdminID:  c.GetUint("admin_id"),
		OperatorUsername: strings.TrimSpace(c.GetString("username")),
		Action:           "role_create",
		Role:             role,
		RequestID:        strings.TrimSpace(c.GetString("request_id")),
		Detail: jsonmap.JSON{
			"role": role,
		},
	})

	logger.Infow("admin_authz_role_created",
		"operator_admin_id", c.GetUint("admin_id"),
		"role", role,
	)

	response.Success(c, gin.H{"role": role})
}

// DeleteAuthzRole 删除角色
func (h *AdminHandler) DeleteAuthzRole(c *gin.Context) {
	role := decodeRoleParam(c.Param("role"))
	if strings.TrimSpace(role) == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	if err := h.authz.DeleteRole(role); err != nil {
		if errors.Is(err, authz.ErrImmutableBuiltinRole) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.authz_builtin_role_immutable", err)
			return
		}
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	h.recordAuthzAudit(c, auditlogapp.AuthzRecord{
		OperatorAdminID:  c.GetUint("admin_id"),
		OperatorUsername: strings.TrimSpace(c.GetString("username")),
		Action:           "role_delete",
		Role:             role,
		RequestID:        strings.TrimSpace(c.GetString("request_id")),
		Detail: jsonmap.JSON{
			"role": role,
		},
	})

	logger.Infow("admin_authz_role_deleted",
		"operator_admin_id", c.GetUint("admin_id"),
		"role", role,
	)

	response.Success(c, nil)
}

// GetAuthzRolePolicies 获取角色策略
func (h *AdminHandler) GetAuthzRolePolicies(c *gin.Context) {
	role := decodeRoleParam(c.Param("role"))
	if strings.TrimSpace(role) == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	policies, err := h.authz.GetRolePolicies(role)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	response.Success(c, policies)
}

// GrantAuthzPolicy 授予角色策略
func (h *AdminHandler) GrantAuthzPolicy(c *gin.Context) {
	var req authzPolicyPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.authz.GrantRolePolicy(req.Role, req.Object, req.Action); err != nil {
		if errors.Is(err, authz.ErrImmutableBuiltinRole) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.authz_builtin_role_immutable", err)
			return
		}
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	h.recordAuthzAudit(c, buildAuthzPolicyAuditRecord(c, req, "policy_grant"))

	logger.Infow("admin_authz_policy_granted",
		"operator_admin_id", c.GetUint("admin_id"),
		"role", req.Role,
		"object", req.Object,
		"action", req.Action,
	)

	response.Success(c, nil)
}

// RevokeAuthzPolicy 撤销角色策略
func (h *AdminHandler) RevokeAuthzPolicy(c *gin.Context) {
	var req authzPolicyPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.authz.RevokeRolePolicy(req.Role, req.Object, req.Action); err != nil {
		if errors.Is(err, authz.ErrImmutableBuiltinRole) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.authz_builtin_role_immutable", err)
			return
		}
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	h.recordAuthzAudit(c, buildAuthzPolicyAuditRecord(c, req, "policy_revoke"))

	logger.Infow("admin_authz_policy_revoked",
		"operator_admin_id", c.GetUint("admin_id"),
		"role", req.Role,
		"object", req.Object,
		"action", req.Action,
	)

	response.Success(c, nil)
}

// GetAuthzAdminRoles 获取管理员角色
func (h *AdminHandler) GetAuthzAdminRoles(c *gin.Context) {
	adminID, ok := parseAdminIDParam(c)
	if !ok {
		return
	}
	if _, err := h.admins.GetByID(adminID); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}

	roles, err := h.authz.GetAdminRoles(adminID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	response.Success(c, roles)
}

// SetAuthzAdminRoles 设置管理员角色
func (h *AdminHandler) SetAuthzAdminRoles(c *gin.Context) {
	adminID, ok := parseAdminIDParam(c)
	if !ok {
		return
	}
	admin, err := h.admins.GetByID(adminID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		return
	}
	if admin == nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_id_invalid", nil)
		return
	}
	if rejectNonSuperEscalation(c, admin.IsSuper) {
		return
	}

	var req authzSetAdminRolesPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.authz.SetAdminRoles(adminID, req.Roles); err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	h.recordAuthzAudit(c, auditlogapp.AuthzRecord{
		OperatorAdminID:  c.GetUint("admin_id"),
		OperatorUsername: strings.TrimSpace(c.GetString("username")),
		TargetAdminID:    &adminID,
		TargetUsername:   admin.Username,
		Action:           "admin_roles_update",
		RequestID:        strings.TrimSpace(c.GetString("request_id")),
		Detail: jsonmap.JSON{
			"target_admin_id": adminID,
			"target_username": admin.Username,
			"roles":           req.Roles,
		},
	})

	logger.Infow("admin_authz_admin_roles_updated",
		"operator_admin_id", c.GetUint("admin_id"),
		"target_admin_id", adminID,
		"roles", req.Roles,
	)

	response.Success(c, nil)
}

func (h *AdminHandler) recordAuthzAudit(c *gin.Context, input auditlogapp.AuthzRecord) {
	if h == nil || h.audit == nil {
		return
	}
	if input.OperatorAdminID == 0 || strings.TrimSpace(input.Action) == "" {
		return
	}
	if err := h.audit.Record(input); err != nil {
		logger.Warnw("admin_authz_audit_record_failed",
			"error", err,
			"action", input.Action,
			"operator_admin_id", input.OperatorAdminID,
		)
	}
}

func parseAdminIDParam(c *gin.Context) (uint, bool) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_id_invalid", nil)
		return 0, false
	}
	return id, true
}

func decodeRoleParam(value string) string {
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(decoded)
}

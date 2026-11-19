package authz

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/util"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

var ErrImmutableBuiltinRole = errors.New("immutable builtin role")

const (
	apiV1Prefix     = "/api/v1"
	casbinTableName = "casbin_rule"
	adminSubjectFmt = "admin:%d"
	rolePrefix      = "role:"
	roleAnchor      = "role:__anchor__"
)

const defaultRBACModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (g(r.sub, p.sub) || r.sub == p.sub) && keyMatch2(r.obj, p.obj) && (r.act == p.act || p.act == "*")
`

// Policy 权限策略
type Policy struct {
	Subject string `json:"subject"`
	Object  string `json:"object"`
	Action  string `json:"action"`
}

// Service Casbin 授权服务
// 统一封装策略加载、授权判定与策略管理逻辑
type Service struct {
	enforcer *casbin.SyncedEnforcer
}

// NewService 创建授权服务
func NewService(db *gorm.DB) (*Service, error) {
	if db == nil {
		return nil, fmt.Errorf("authz db is nil")
	}

	adapter, err := gormadapter.NewAdapterByDBUseTableName(db, "", casbinTableName)
	if err != nil {
		return nil, fmt.Errorf("create authz adapter failed: %w", err)
	}

	m, err := model.NewModelFromString(defaultRBACModel)
	if err != nil {
		return nil, fmt.Errorf("load authz model failed: %w", err)
	}

	enforcer, err := casbin.NewSyncedEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("init authz enforcer failed: %w", err)
	}
	enforcer.AddFunction("keyMatch2", util.KeyMatch2Func)
	enforcer.EnableAutoSave(true)

	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("load authz policy failed: %w", err)
	}

	return &Service{enforcer: enforcer}, nil
}

// Enforcer 返回底层 enforcer（供策略管理模块复用）
func (s *Service) Enforcer() *casbin.SyncedEnforcer {
	if s == nil {
		return nil
	}
	return s.enforcer
}

// Enforce 执行授权判断
func (s *Service) Enforce(sub, obj, act string) (bool, error) {
	if s == nil || s.enforcer == nil {
		return false, fmt.Errorf("authz service unavailable")
	}
	return s.enforcer.Enforce(strings.TrimSpace(sub), NormalizeObject(obj), NormalizeAction(act))
}

// EnforceAdmin 按管理员 ID 判定授权
func (s *Service) EnforceAdmin(adminID uint, obj, act string) (bool, error) {
	return s.Enforce(SubjectForAdmin(adminID), obj, act)
}

// ReloadPolicy 重新加载策略
func (s *Service) ReloadPolicy() error {
	if s == nil || s.enforcer == nil {
		return fmt.Errorf("authz service unavailable")
	}
	return s.enforcer.LoadPolicy()
}

// EnsureRole 确保角色存在
func (s *Service) EnsureRole(role string) (string, error) {
	normalized, err := NormalizeRole(role)
	if err != nil {
		return "", err
	}
	if s == nil || s.enforcer == nil {
		return "", fmt.Errorf("authz service unavailable")
	}
	if normalized == roleAnchor {
		return "", fmt.Errorf("reserved role is not allowed")
	}

	exists, err := s.enforcer.HasNamedGroupingPolicy("g", normalized, roleAnchor)
	if err != nil {
		return "", fmt.Errorf("check role failed: %w", err)
	}
	if exists {
		return normalized, nil
	}

	added, err := s.enforcer.AddNamedGroupingPolicy("g", normalized, roleAnchor)
	if err != nil {
		return "", fmt.Errorf("create role failed: %w", err)
	}
	if added {
		if err := s.saveAndReload(); err != nil {
			return "", err
		}
	}
	return normalized, nil
}

// ListRoles 列出角色
func (s *Service) ListRoles() ([]string, error) {
	if s == nil || s.enforcer == nil {
		return nil, fmt.Errorf("authz service unavailable")
	}
	rules, err := s.enforcer.GetFilteredNamedGroupingPolicy("g", 0)
	if err != nil {
		return nil, fmt.Errorf("list roles failed: %w", err)
	}
	roleSet := make(map[string]struct{})
	for _, rule := range rules {
		if len(rule) >= 1 {
			if strings.HasPrefix(rule[0], rolePrefix) && rule[0] != roleAnchor {
				roleSet[rule[0]] = struct{}{}
			}
		}
		if len(rule) >= 2 {
			if strings.HasPrefix(rule[1], rolePrefix) && rule[1] != roleAnchor {
				roleSet[rule[1]] = struct{}{}
			}
		}
	}
	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles, nil
}

// DeleteRole 删除角色及其关联策略
func (s *Service) DeleteRole(role string) error {
	normalized, err := NormalizeRole(role)
	if err != nil {
		return err
	}
	if normalized == roleAnchor {
		return fmt.Errorf("reserved role is not allowed")
	}
	if IsImmutableBuiltinRole(normalized) {
		return ErrImmutableBuiltinRole
	}
	if s == nil || s.enforcer == nil {
		return fmt.Errorf("authz service unavailable")
	}

	changed := false
	if removed, err := s.enforcer.RemoveFilteredPolicy(0, normalized); err != nil {
		return fmt.Errorf("remove role policy failed: %w", err)
	} else if removed {
		changed = true
	}
	if removed, err := s.enforcer.RemoveFilteredNamedGroupingPolicy("g", 0, normalized); err != nil {
		return fmt.Errorf("remove role link failed: %w", err)
	} else if removed {
		changed = true
	}
	if removed, err := s.enforcer.RemoveFilteredNamedGroupingPolicy("g", 1, normalized); err != nil {
		return fmt.Errorf("remove role incoming link failed: %w", err)
	} else if removed {
		changed = true
	}

	if changed {
		if err := s.saveAndReload(); err != nil {
			return err
		}
	}
	return nil
}

// GrantRolePolicy 为角色授予策略
func (s *Service) GrantRolePolicy(role, object, action string) error {
	if IsImmutableBuiltinRole(role) {
		return ErrImmutableBuiltinRole
	}
	normalizedRole, err := s.EnsureRole(role)
	if err != nil {
		return err
	}
	normalizedObject := NormalizeObject(object)
	normalizedAction := NormalizeAction(action)
	if normalizedAction == "" {
		return fmt.Errorf("action is required")
	}
	if s == nil || s.enforcer == nil {
		return fmt.Errorf("authz service unavailable")
	}

	added, err := s.enforcer.AddPolicy(normalizedRole, normalizedObject, normalizedAction)
	if err != nil {
		return fmt.Errorf("grant policy failed: %w", err)
	}
	if added {
		if err := s.saveAndReload(); err != nil {
			return err
		}
	}
	return nil
}

// RevokeRolePolicy 撤销角色策略
func (s *Service) RevokeRolePolicy(role, object, action string) error {
	normalizedRole, err := NormalizeRole(role)
	if err != nil {
		return err
	}
	if IsImmutableBuiltinRole(normalizedRole) {
		return ErrImmutableBuiltinRole
	}
	normalizedObject := NormalizeObject(object)
	normalizedAction := NormalizeAction(action)
	if normalizedAction == "" {
		return fmt.Errorf("action is required")
	}
	if s == nil || s.enforcer == nil {
		return fmt.Errorf("authz service unavailable")
	}

	removed, err := s.enforcer.RemovePolicy(normalizedRole, normalizedObject, normalizedAction)
	if err != nil {
		return fmt.Errorf("revoke policy failed: %w", err)
	}
	if removed {
		if err := s.saveAndReload(); err != nil {
			return err
		}
	}
	return nil
}

// GetRolePolicies 查询角色策略
func (s *Service) GetRolePolicies(role string) ([]Policy, error) {
	normalizedRole, err := NormalizeRole(role)
	if err != nil {
		return nil, err
	}
	if s == nil || s.enforcer == nil {
		return nil, fmt.Errorf("authz service unavailable")
	}

	rules, err := s.enforcer.GetFilteredPolicy(0, normalizedRole)
	if err != nil {
		return nil, fmt.Errorf("get role policies failed: %w", err)
	}
	return convertPolicies(rules), nil
}

// SetAdminRoles 覆盖设置管理员角色
func (s *Service) SetAdminRoles(adminID uint, roles []string) error {
	if adminID == 0 {
		return fmt.Errorf("admin id is required")
	}
	if s == nil || s.enforcer == nil {
		return fmt.Errorf("authz service unavailable")
	}
	subject := SubjectForAdmin(adminID)

	if _, err := s.enforcer.RemoveFilteredNamedGroupingPolicy("g", 0, subject); err != nil {
		return fmt.Errorf("clear admin roles failed: %w", err)
	}

	for _, role := range roles {
		normalizedRole, err := s.EnsureRole(role)
		if err != nil {
			return err
		}
		if _, err := s.enforcer.AddNamedGroupingPolicy("g", subject, normalizedRole); err != nil {
			return fmt.Errorf("assign admin role failed: %w", err)
		}
	}

	return s.saveAndReload()
}

// GetAdminRoles 查询管理员角色
func (s *Service) GetAdminRoles(adminID uint) ([]string, error) {
	if adminID == 0 {
		return nil, fmt.Errorf("admin id is required")
	}
	if s == nil || s.enforcer == nil {
		return nil, fmt.Errorf("authz service unavailable")
	}
	roles, err := s.enforcer.GetRolesForUser(SubjectForAdmin(adminID))
	if err != nil {
		return nil, fmt.Errorf("get admin roles failed: %w", err)
	}
	filtered := make([]string, 0, len(roles))
	for _, role := range roles {
		if !strings.HasPrefix(role, rolePrefix) || role == roleAnchor {
			continue
		}
		filtered = append(filtered, role)
	}
	sort.Strings(filtered)
	return filtered, nil
}

// GetAdminPolicies 查询管理员生效策略（角色 + 直连）
func (s *Service) GetAdminPolicies(adminID uint) ([]Policy, error) {
	if adminID == 0 {
		return nil, fmt.Errorf("admin id is required")
	}
	if s == nil || s.enforcer == nil {
		return nil, fmt.Errorf("authz service unavailable")
	}
	subject := SubjectForAdmin(adminID)
	policyMap := map[string]Policy{}

	appendRules := func(rules [][]string) {
		for _, item := range convertPolicies(rules) {
			key := item.Subject + "|" + item.Object + "|" + item.Action
			policyMap[key] = item
		}
	}

	directRules, err := s.enforcer.GetFilteredPolicy(0, subject)
	if err != nil {
		return nil, fmt.Errorf("get direct policies failed: %w", err)
	}
	appendRules(directRules)

	roles, err := s.GetAdminRoles(adminID)
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		rules, err := s.enforcer.GetFilteredPolicy(0, role)
		if err != nil {
			return nil, fmt.Errorf("get role policies failed: %w", err)
		}
		appendRules(rules)
	}

	result := make([]Policy, 0, len(policyMap))
	for _, item := range policyMap {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Subject == result[j].Subject {
			if result[i].Object == result[j].Object {
				return result[i].Action < result[j].Action
			}
			return result[i].Object < result[j].Object
		}
		return result[i].Subject < result[j].Subject
	})
	return result, nil
}

func (s *Service) saveAndReload() error {
	if s == nil || s.enforcer == nil {
		return fmt.Errorf("authz service unavailable")
	}
	return nil
}

func convertPolicies(rules [][]string) []Policy {
	policies := make([]Policy, 0, len(rules))
	for _, rule := range rules {
		if len(rule) < 3 {
			continue
		}
		policies = append(policies, Policy{
			Subject: strings.TrimSpace(rule[0]),
			Object:  NormalizeObject(rule[1]),
			Action:  NormalizeAction(rule[2]),
		})
	}
	return policies
}

// SubjectForAdmin 生成管理员主体标识
func SubjectForAdmin(adminID uint) string {
	return fmt.Sprintf(adminSubjectFmt, adminID)
}

// NormalizeRole 统一角色名称
func NormalizeRole(role string) (string, error) {
	normalized := strings.TrimSpace(role)
	if normalized == "" {
		return "", fmt.Errorf("role is required")
	}
	normalized = strings.ReplaceAll(normalized, " ", "_")
	if !strings.HasPrefix(normalized, rolePrefix) {
		normalized = rolePrefix + normalized
	}
	if len(normalized) <= len(rolePrefix) {
		return "", fmt.Errorf("role is required")
	}
	return normalized, nil
}

// IsImmutableBuiltinRole 判断角色是否由启动引导流程完全托管。
// 这类角色允许分配给管理员，但不允许通过通用角色 API 改写或删除。
func IsImmutableBuiltinRole(role string) bool {
	normalized, err := NormalizeRole(role)
	if err != nil {
		return false
	}
	for _, seed := range BuiltinRoleSeeds() {
		if !seed.Immutable {
			continue
		}
		seedRole, err := NormalizeRole(seed.Role)
		if err == nil && seedRole == normalized {
			return true
		}
	}
	return false
}

// NormalizeObject 统一授权资源路径
func NormalizeObject(object string) string {
	normalized := strings.TrimSpace(object)
	if normalized == "" {
		return "/"
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	if strings.HasPrefix(normalized, apiV1Prefix+"/") {
		return strings.TrimPrefix(normalized, apiV1Prefix)
	}
	if normalized == apiV1Prefix {
		return "/"
	}
	return normalized
}

// NormalizeAction 统一授权动作
func NormalizeAction(action string) string {
	return strings.ToUpper(strings.TrimSpace(action))
}

package adminauthzhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

func TestBuildAuthzPolicyAuditRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("admin_id", uint(7))
	c.Set("username", " admin ")
	c.Set("request_id", " req-1 ")

	req := authzPolicyPayload{
		Role:   "operator",
		Object: "orders",
		Action: " get ",
	}

	got := buildAuthzPolicyAuditRecord(c, req, "policy_grant")

	if got.OperatorAdminID != 7 {
		t.Fatalf("OperatorAdminID = %d, want 7", got.OperatorAdminID)
	}
	if got.OperatorUsername != "admin" {
		t.Fatalf("OperatorUsername = %q, want %q", got.OperatorUsername, "admin")
	}
	if got.Action != "policy_grant" {
		t.Fatalf("Action = %q, want %q", got.Action, "policy_grant")
	}
	if got.Role != req.Role {
		t.Fatalf("Role = %q, want %q", got.Role, req.Role)
	}
	if got.Object != req.Object {
		t.Fatalf("Object = %q, want %q", got.Object, req.Object)
	}
	if got.Method != req.Action {
		t.Fatalf("Method = %q, want %q", got.Method, req.Action)
	}
	if got.RequestID != "req-1" {
		t.Fatalf("RequestID = %q, want %q", got.RequestID, "req-1")
	}
	if got.Detail["role"] != req.Role {
		t.Fatalf("Detail role = %v, want %q", got.Detail["role"], req.Role)
	}
	if got.Detail["object"] != req.Object {
		t.Fatalf("Detail object = %v, want %q", got.Detail["object"], req.Object)
	}
	if got.Detail["method"] != "GET" {
		t.Fatalf("Detail method = %v, want %q", got.Detail["method"], "GET")
	}
}

func TestBuildAuthzRoleItemsUsesBackendImmutableSeeds(t *testing.T) {
	items := buildAuthzRoleItems([]string{"role:readonly_auditor", "role:custom_billing"})
	if len(items) != 2 {
		t.Fatalf("items length = %d, want 2", len(items))
	}
	if items[0].Role != "role:readonly_auditor" || !items[0].Immutable {
		t.Fatalf("builtin role metadata = %+v, want immutable", items[0])
	}
	if items[1].Role != "role:custom_billing" || items[1].Immutable {
		t.Fatalf("custom role metadata = %+v, want mutable", items[1])
	}
}

func TestBuildAuthzRoleListPayloadKeepsLegacyStringShape(t *testing.T) {
	roles := []string{"role:readonly_auditor", "role:custom_billing"}
	legacy, ok := buildAuthzRoleListPayload(roles, false).([]string)
	if !ok || len(legacy) != 2 || legacy[0] != roles[0] {
		t.Fatalf("legacy payload = %#v, want string role array", legacy)
	}
	metadata, ok := buildAuthzRoleListPayload(roles, true).([]AuthzRoleItem)
	if !ok || len(metadata) != 2 || !metadata[0].Immutable || metadata[1].Immutable {
		t.Fatalf("metadata payload = %#v, want role descriptors", metadata)
	}
}

type fakeRolePolicyService struct {
	RolePolicyService
	setRolesCalls int
}

func (f *fakeRolePolicyService) SetAdminRoles(uint, []string) error {
	f.setRolesCalls++
	return nil
}

type fakeAdminDirectory struct {
	AdminDirectory
	admins map[uint]*admindomain.Admin
}

func (f *fakeAdminDirectory) GetByID(id uint) (*admindomain.Admin, error) {
	if admin, ok := f.admins[id]; ok {
		copied := *admin
		return &copied, nil
	}
	return nil, nil
}

func (f *fakeAdminDirectory) GetByUsername(username string) (*admindomain.Admin, error) {
	for _, admin := range f.admins {
		if admin.Username == username {
			copied := *admin
			return &copied, nil
		}
	}
	return nil, nil
}

func (f *fakeAdminDirectory) Create(admin *admindomain.Admin) error {
	admin.ID = uint(len(f.admins) + 1)
	f.admins[admin.ID] = admin
	return nil
}

func (f *fakeAdminDirectory) Update(admin *admindomain.Admin) error {
	copied := *admin
	f.admins[admin.ID] = &copied
	return nil
}

func (f *fakeAdminDirectory) Delete(id uint) error {
	delete(f.admins, id)
	return nil
}

func (f *fakeAdminDirectory) Count() (int64, error) { return int64(len(f.admins)), nil }

type fakePasswordService struct{}

func (fakePasswordService) ValidatePassword(string) error { return nil }
func (fakePasswordService) HashPassword(p string) (string, error) {
	return "hash:" + p, nil
}

type fakeAuthStateCache struct{}

func (fakeAuthStateCache) SetAdminAuthState(context.Context, *admindomain.Admin) error { return nil }
func (fakeAuthStateCache) DelAdminAuthState(context.Context, uint) error               { return nil }

func TestAdminAccountWritesRejectNonSuperEscalation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newEngine := func(operatorID uint, operatorIsSuper bool) (*gin.Engine, *fakeAdminDirectory, *fakeRolePolicyService) {
		dir := &fakeAdminDirectory{admins: map[uint]*admindomain.Admin{
			1: {ID: 1, Username: "admin", PasswordHash: "orig", IsSuper: true},
			2: {ID: 2, Username: "sys1", PasswordHash: "orig"},
			3: {ID: 3, Username: "staff", PasswordHash: "orig"},
		}}
		roles := &fakeRolePolicyService{}
		h := NewAdminHandler(roles, dir, fakePasswordService{}, fakeAuthStateCache{}, nil)
		engine := gin.New()
		engine.Use(func(c *gin.Context) {
			c.Set("admin_id", operatorID)
			c.Set("admin_is_super", operatorIsSuper)
		})
		engine.POST("/authz/admins", h.CreateAuthzAdmin)
		engine.PUT("/authz/admins/:id", h.UpdateAuthzAdmin)
		engine.DELETE("/authz/admins/:id", h.DeleteAuthzAdmin)
		engine.PUT("/authz/admins/:id/roles", h.SetAuthzAdminRoles)
		return engine, dir, roles
	}
	call := func(engine *gin.Engine, method, path, body string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(rec, req)
		var resp struct {
			StatusCode int `json:"status_code"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s %s: decode response %q: %v", method, path, rec.Body.String(), err)
		}
		return resp.StatusCode
	}

	forbidden := []struct {
		name, method, path, body string
	}{
		{"promote self", http.MethodPut, "/authz/admins/2", `{"is_super":true}`},
		{"reset super password", http.MethodPut, "/authz/admins/1", `{"password":"Pwn-2026!"}`},
		{"rename to protected username", http.MethodPut, "/authz/admins/2", `{"username":"Admin"}`},
		{"create super", http.MethodPost, "/authz/admins", `{"username":"evil","password":"Pwn-2026!","is_super":true}`},
		{"create protected username", http.MethodPost, "/authz/admins", `{"username":"ADMIN","password":"Pwn-2026!"}`},
		{"delete super", http.MethodDelete, "/authz/admins/1", ``},
		{"set super roles", http.MethodPut, "/authz/admins/1/roles", `{"roles":[]}`},
	}
	for _, tc := range forbidden {
		t.Run(tc.name, func(t *testing.T) {
			engine, dir, roles := newEngine(2, false)
			if got := call(engine, tc.method, tc.path, tc.body); got != response.CodeForbidden {
				t.Fatalf("status_code = %d, want %d", got, response.CodeForbidden)
			}
			if super := dir.admins[1]; super == nil || super.PasswordHash != "orig" || !super.IsSuper {
				t.Fatalf("super admin changed: %+v", super)
			}
			if dir.admins[2].IsSuper || len(dir.admins) != 3 || roles.setRolesCalls != 0 {
				t.Fatalf("unexpected write: admins=%d sys1.IsSuper=%v setRoles=%d", len(dir.admins), dir.admins[2].IsSuper, roles.setRolesCalls)
			}
		})
	}

	t.Run("non-super can still edit regular admin", func(t *testing.T) {
		engine, dir, _ := newEngine(2, false)
		if got := call(engine, http.MethodPut, "/authz/admins/3", `{"username":"staff2","password":"New-2026!","is_super":false}`); got != response.CodeOK {
			t.Fatalf("status_code = %d, want success", got)
		}
		if dir.admins[3].Username != "staff2" || dir.admins[3].PasswordHash != "hash:New-2026!" {
			t.Fatalf("regular admin not updated: %+v", dir.admins[3])
		}
	})

	t.Run("super can promote", func(t *testing.T) {
		engine, dir, _ := newEngine(1, true)
		if got := call(engine, http.MethodPut, "/authz/admins/3", `{"is_super":true}`); got != response.CodeOK {
			t.Fatalf("status_code = %d, want success", got)
		}
		if !dir.admins[3].IsSuper {
			t.Fatal("super operator should be able to promote")
		}
	})
}

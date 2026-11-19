package httpserver

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/authz"
	"github.com/dujiao-next/internal/bootstrap/database/migrations"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/logger"
	tokenapp "github.com/dujiao-next/internal/modules/identity/adminapitoken/application"
	"github.com/dujiao-next/internal/platform/database/gormdb"

	"github.com/casbin/casbin/v3/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestAllAdminRoutesCoveredByBuiltinRoles 校验 admin 路由文件里的每条路由
// 都被 authz.BuiltinRoleSeeds() 中至少一条角色策略覆盖。
//
// 目的：避免新增 admin 接口时忘记同步 RBAC 预置角色，导致非超管角色无法通过
// 角色分配获得该权限（catalog UI 上能看到，但任何角色都拿不到）。
//
// 实现：静态扫描 routes_admin*.go、internal/modules/**/routes.go
// 以及 internal/platform/http/**/routes.go，
// 提取 admin/authorized/paymentProtected.METHOD("/path", ...) 调用，
// 与 builtin role seeds 用 keyMatch2 比对（与运行时 Casbin 模型一致）。
func TestAllAdminRoutesCoveredByBuiltinRoles(t *testing.T) {
	routes, err := extractAdminRoutesFromSource()
	if err != nil {
		t.Fatalf("extract admin routes: %v", err)
	}
	if len(routes) == 0 {
		t.Fatalf("no admin routes extracted; parser or source layout changed?")
	}
	t.Logf("validating RBAC coverage for %d admin routes", len(routes))

	seeds := authz.BuiltinRoleSeeds()
	if len(seeds) == 0 {
		t.Fatalf("no builtin role seeds")
	}

	type policy struct {
		object string
		action string
	}
	var policies []policy
	for _, seed := range seeds {
		for _, p := range seed.Policies {
			policies = append(policies, policy{
				object: authz.NormalizeObject(p.Object),
				action: authz.NormalizeAction(p.Action),
			})
		}
	}

	var uncovered []adminRoute
	for _, r := range routes {
		matched := false
		for _, p := range policies {
			if p.action != "*" && p.action != r.method {
				continue
			}
			if util.KeyMatch2(r.object, p.object) {
				matched = true
				break
			}
		}
		if !matched {
			uncovered = append(uncovered, r)
		}
	}

	if len(uncovered) > 0 {
		var lines []string
		for _, r := range uncovered {
			lines = append(lines, "  "+r.method+" "+r.object)
		}
		t.Fatalf("the following admin routes are not covered by any builtin role seed in authz.BuiltinRoleSeeds() — add them to the appropriate role in api/internal/authz/bootstrap.go:\n%s",
			strings.Join(lines, "\n"))
	}
}

func TestAdminAPITokenAllowlistMatchesRegisteredRoutes(t *testing.T) {
	originalDB, originalLog := gormdb.DB, logger.L
	t.Cleanup(func() { gormdb.DB, logger.L = originalDB, originalLog })
	logger.L = zap.NewNop()
	require.NoError(t, gormdb.InitDB("sqlite", ":memory:", gormdb.DBPoolConfig{MaxOpenConns: 1, MaxIdleConns: 1}, "release"))
	db, err := gormdb.DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, migrations.AutoMigrate())

	cfg := &config.Config{}
	cfg.App.SecretKey = t.Name()
	cfg.Web.AdminPath = "/admin"
	c, err := container.NewContainer(cfg)
	require.NoError(t, err)
	engine := SetupRouter(cfg, c)
	registered := make(map[string]bool)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, route := range tokenapp.AllowedRoutes() {
		if !registered[route] {
			t.Errorf("admin API token allowlist refers to unregistered route %s", route)
		}
	}
}

type adminRoute struct {
	method string
	object string // 例如 "/admin/users/:id"
}

var adminRouteMethods = map[string]struct{}{
	"GET":    {},
	"POST":   {},
	"PUT":    {},
	"PATCH":  {},
	"DELETE": {},
}

var adminRouteReceivers = map[string]struct{}{
	"admin":            {},
	"authorized":       {},
	"paymentProtected": {},
}

var publicAdminRoutes = map[string]struct{}{
	"POST /admin/login":            {},
	"POST /admin/login/verify-2fa": {},
}

// extractAdminRoutesFromSource 从应用 admin 路由、模块路由和平台 HTTP 路由文件中读取调用。
// 方法范围：GET / POST / PUT / PATCH / DELETE。HEAD/OPTIONS 不参与 RBAC。
func extractAdminRoutesFromSource() ([]adminRoute, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	routerDirectory := filepath.Dir(thisFile)

	sources, err := discoverAdminRouteSources(routerDirectory)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, os.ErrNotExist
	}

	seen := make(map[string]struct{})
	var out []adminRoute
	for _, source := range sources {
		routes, err := extractAdminRoutesFromFile(source)
		if err != nil {
			return nil, err
		}
		for _, route := range routes {
			key := route.method + " " + route.object
			if _, public := publicAdminRoutes[key]; public {
				continue
			}
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, route)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].object == out[j].object {
			return out[i].method < out[j].method
		}
		return out[i].object < out[j].object
	})
	return out, nil
}

func discoverAdminRouteSources(routerDirectory string) ([]string, error) {
	adminSources, err := filepath.Glob(filepath.Join(routerDirectory, "routes_admin*.go"))
	if err != nil {
		return nil, err
	}

	sources := make([]string, 0, len(adminSources)+32)
	for _, path := range adminSources {
		if !strings.HasSuffix(path, "_test.go") {
			sources = append(sources, path)
		}
	}

	for _, directory := range []string{
		filepath.Join(routerDirectory, "..", "..", "modules"),
		filepath.Join(routerDirectory, "..", "..", "platform", "http"),
	} {
		if err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && entry.Name() == "routes.go" {
				sources = append(sources, path)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	sort.Strings(sources)
	return sources, nil
}

func TestExtractAdminRoutesIncludesPlatformHTTPRoutes(t *testing.T) {
	routes, err := extractAdminRoutesFromSource()
	if err != nil {
		t.Fatalf("extract admin routes: %v", err)
	}
	for _, route := range routes {
		if route.method == "GET" && route.object == "/admin/system/version/check" {
			return
		}
	}
	t.Fatal("platform HTTP route GET /admin/system/version/check was not discovered")
}

func extractAdminRoutesFromFile(path string) ([]adminRoute, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return nil, err
	}

	var routes []adminRoute
	var routeErr error
	ast.Inspect(file, func(node ast.Node) bool {
		if routeErr != nil {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		if _, ok := adminRouteReceivers[receiver.Name]; !ok {
			return true
		}
		if _, ok := adminRouteMethods[selector.Sel.Name]; !ok {
			return true
		}
		pathLiteral, ok := call.Args[0].(*ast.BasicLit)
		if !ok || pathLiteral.Kind != token.STRING {
			routeErr = fmt.Errorf("%s:%d: admin route path must be a string literal", path, fileSet.Position(call.Args[0].Pos()).Line)
			return false
		}
		routePath, err := strconv.Unquote(pathLiteral.Value)
		if err != nil {
			routeErr = fmt.Errorf("%s:%d: decode admin route path: %w", path, fileSet.Position(pathLiteral.Pos()).Line, err)
			return false
		}
		routes = append(routes, adminRoute{
			method: selector.Sel.Name,
			object: authz.NormalizeObject("/admin" + routePath),
		})
		return true
	})
	if routeErr != nil {
		return nil, routeErr
	}
	return routes, nil
}

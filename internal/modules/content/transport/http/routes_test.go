package contenthttp

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestContentRouteRegistrationContract(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterPublicRoutes(router.Group("/api/v1/public"), NewPublicHandler(nil, nil, nil))
	RegisterAdminRoutes(router.Group("/api/v1/admin"), NewAdminHandler(nil, nil, nil, nil))

	want := []string{
		http.MethodGet + " /api/v1/public/posts",
		http.MethodGet + " /api/v1/public/posts/:slug",
		http.MethodGet + " /api/v1/public/banners",
		http.MethodGet + " /api/v1/public/post-categories",
		http.MethodGet + " /api/v1/admin/posts",
		http.MethodPost + " /api/v1/admin/posts",
		http.MethodPost + " /api/v1/admin/posts/batch-status",
		http.MethodPost + " /api/v1/admin/posts/batch-category",
		http.MethodPost + " /api/v1/admin/posts/batch-delete",
		http.MethodPut + " /api/v1/admin/posts/:id",
		http.MethodDelete + " /api/v1/admin/posts/:id",
		http.MethodGet + " /api/v1/admin/posts/:id/products",
		http.MethodGet + " /api/v1/admin/post-categories",
		http.MethodPost + " /api/v1/admin/post-categories",
		http.MethodPut + " /api/v1/admin/post-categories/:id",
		http.MethodDelete + " /api/v1/admin/post-categories/:id",
		http.MethodPatch + " /api/v1/admin/post-categories/:id/status",
		http.MethodGet + " /api/v1/admin/banners",
		http.MethodGet + " /api/v1/admin/banners/:id",
		http.MethodPost + " /api/v1/admin/banners",
		http.MethodPut + " /api/v1/admin/banners/:id",
		http.MethodDelete + " /api/v1/admin/banners/:id",
		http.MethodGet + " /api/v1/admin/media",
		http.MethodPost + " /api/v1/admin/media/batch-delete",
		http.MethodPut + " /api/v1/admin/media/:id",
		http.MethodDelete + " /api/v1/admin/media/:id",
	}
	got := make([]string, 0, len(router.Routes()))
	for _, route := range router.Routes() {
		got = append(got, route.Method+" "+route.Path)
	}
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("route contract mismatch\nwant:\n%s\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

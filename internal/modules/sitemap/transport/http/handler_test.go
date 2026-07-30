package sitemaphttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeGenerator struct {
	xml    string
	robots string
	err    error
	base   string
}

func (f *fakeGenerator) Generate(_ context.Context, baseURL string) (string, error) {
	f.base = baseURL
	return f.xml, f.err
}

func (f *fakeGenerator) GenerateRobots(baseURL string) string {
	f.base = baseURL
	return f.robots
}

type fakeBrand struct {
	url string
	err error
}

func (f fakeBrand) GetSiteURL() (string, error) {
	return f.url, f.err
}

func TestGetSitemapUsesConfiguredSiteURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gen := &fakeGenerator{xml: "<urlset/>"}
	handler := NewHandler(gen, fakeBrand{url: "https://shop.example/"}, false)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	c.Request.Host = "ignored.example"

	handler.GetSitemap(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}
	if gen.base != "https://shop.example" {
		t.Fatalf("base URL want https://shop.example got %q", gen.base)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "application/xml") {
		t.Fatalf("unexpected content type %q", w.Header().Get("Content-Type"))
	}
}

func TestGetSitemapFallsBackToRequestHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gen := &fakeGenerator{xml: "<urlset/>"}
	handler := NewHandler(gen, fakeBrand{url: ""}, false)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = "store.example"
	req.Header.Set("X-Forwarded-Proto", "https")
	c.Request = req

	handler.GetSitemap(c)

	if gen.base != "https://store.example" {
		t.Fatalf("base URL want https://store.example got %q", gen.base)
	}
}

func TestGetSitemapUnavailableWithoutGenerator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, false)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)

	handler.GetSitemap(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status want 503 got %d", w.Code)
	}
}

func TestGetRobotsFallsBackWhenGeneratorMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, false)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/robots.txt", nil)

	handler.GetRobots(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "User-agent: *") {
		t.Fatalf("unexpected robots body %q", w.Body.String())
	}
}

func TestGetSitemapReturns404WhenUserSPADisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&fakeGenerator{xml: "<urlset/>"}, nil, true)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)

	handler.GetSitemap(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status want 404 got %d", w.Code)
	}
}

func TestGetRobotsReturnsDisallowWhenUserSPADisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, true)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/robots.txt", nil)

	handler.GetRobots(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Disallow: /") {
		t.Fatalf("robots should disallow all, got %q", w.Body.String())
	}
}

func TestSitemapNormalWhenUserSPANotDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&fakeGenerator{xml: "<urlset/>"}, nil, false)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)

	handler.GetSitemap(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}
}

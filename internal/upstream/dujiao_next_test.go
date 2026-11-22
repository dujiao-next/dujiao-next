package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
)

// 被上游限流（429）时应等待 Retry-After 后重试，而不是把 429 直接冒泡成"拉取失败"。
func TestRequestRetriesOnRateLimit(t *testing.T) {
	disableRequestPacing(t)

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error_code":"rate_limited","error_message":"too many requests"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"categories":[]}`))
	}))
	defer server.Close()

	adapter := NewDujiaoNextAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL}, t.TempDir())
	result, err := adapter.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls)
	}
	if result == nil || !result.Supported {
		t.Fatalf("expected supported categories, got %+v", result)
	}
}

// 持续限流时重试有上限，最终把上游错误返回给调用方。
func TestRequestStopsAfterMaxAttempts(t *testing.T) {
	disableRequestPacing(t)

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error_code":"rate_limited","error_message":"too many requests"}`))
	}))
	defer server.Close()

	adapter := NewDujiaoNextAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL}, t.TempDir())
	err := adapter.CancelOrder(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if calls != maxRequestAttempts {
		t.Fatalf("expected %d attempts, got %d", maxRequestAttempts, calls)
	}
	if code := extractUpstreamErrorCode(err); code != "rate_limited" {
		t.Fatalf("expected rate_limited error code, got %q (%v)", code, err)
	}
}

// disableRequestPacing 关闭客户端节流，避免测试被令牌桶拖慢。
func disableRequestPacing(t *testing.T) {
	t.Helper()
	oldBurst, oldInterval := upstreamRequestBurst, upstreamRequestInterval
	upstreamRequestBurst, upstreamRequestInterval = 0, 0
	t.Cleanup(func() {
		upstreamRequestBurst, upstreamRequestInterval = oldBurst, oldInterval
	})
}
func TestDownloadImageRestrictsHostAndSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/huge.png" {
			_, _ = w.Write([]byte(strings.Repeat("x", maxUpstreamImageBytes+1)))
			return
		}
		_, _ = w.Write([]byte("png"))
	}))
	defer server.Close()
	adapter := NewDujiaoNextAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL}, t.TempDir())

	if local, err := adapter.DownloadImage(context.Background(), "/uploads/a.png"); err != nil || !strings.HasSuffix(local, ".png") {
		t.Fatalf("same-host image should download: local=%q err=%v", local, err)
	}
	if _, err := adapter.DownloadImage(context.Background(), "http://169.254.169.254/latest/meta-data"); err == nil {
		t.Fatal("foreign host must be rejected")
	}
	if _, err := adapter.DownloadImage(context.Background(), "/huge.png"); err == nil {
		t.Fatal("oversized image must be rejected")
	}
}

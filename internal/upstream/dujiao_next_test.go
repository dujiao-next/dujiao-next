package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// Retry-After 大于 0 时应等待相应时长再重试。旧用例全用 Retry-After: 0，
// 覆盖不到「等待把每页超时吃光」的场景，所以单独补一个。
func TestRequestRetriesOnRateLimitWithRetryAfter(t *testing.T) {
	disableRequestPacing(t)

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error_code":"rate_limited","error_message":"too many requests"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"categories":[]}`))
	}))
	defer server.Close()

	adapter := NewDujiaoNextAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL}, t.TempDir())
	start := time.Now()
	result, err := adapter.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("expected retry to succeed with Retry-After>0, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Fatalf("expected to wait ~1s for Retry-After, waited %v", elapsed)
	}
	if result == nil || !result.Supported {
		t.Fatalf("expected supported categories, got %+v", result)
	}
}

// upstreamRetryWait 应优先采用 Retry-After，并按 retryWaitMax 封顶，缺省时退化为按次数递增。
func TestUpstreamRetryWaitHonorsRetryAfter(t *testing.T) {
	mkResp := func(retryAfter string) *http.Response {
		h := http.Header{}
		if retryAfter != "" {
			h.Set("Retry-After", retryAfter)
		}
		return &http.Response{Header: h}
	}
	if got := upstreamRetryWait(mkResp("5"), 1); got != 5*time.Second {
		t.Fatalf("expected 5s from Retry-After, got %v", got)
	}
	if got := upstreamRetryWait(mkResp("999"), 1); got != retryWaitMax {
		t.Fatalf("expected clamp to %v, got %v", retryWaitMax, got)
	}
	if got := upstreamRetryWait(mkResp(""), 3); got != 3*time.Second {
		t.Fatalf("expected fallback to attempt-based wait, got %v", got)
	}
}

// disableRequestPacing 关闭客户端节流，避免测试被令牌桶拖慢。
func disableRequestPacing(t *testing.T) {
	t.Helper()
	t.Cleanup(SetRequestPacing(0, 0))
}

// 限速器必须真的按「突发额度 + 补充间隔」排队：算错要么所有请求卡住，要么完全不限速。
func TestUpstreamPacerEnforcesBurstAndInterval(t *testing.T) {
	t.Cleanup(SetRequestPacing(2, 50*time.Millisecond))
	p := &upstreamPacer{tokens: float64(upstreamRequestBurst)}
	start := time.Now()
	for i := 0; i < 4; i++ {
		if err := p.wait(context.Background()); err != nil {
			t.Fatalf("wait %d failed: %v", i+1, err)
		}
	}
	// 前 2 次用突发额度即时通过，第 3、4 次各等约 50ms。
	if elapsed := time.Since(start); elapsed < 90*time.Millisecond {
		t.Fatalf("expected pacing to delay calls beyond the burst, elapsed %v", elapsed)
	}
}

// 等不到令牌时应响应 ctx 取消，而不是继续阻塞。
func TestUpstreamPacerHonorsContextCancellation(t *testing.T) {
	t.Cleanup(SetRequestPacing(1, time.Hour))
	p := &upstreamPacer{tokens: 0}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := p.wait(ctx); err == nil {
		t.Fatal("expected context error while waiting for a token")
	}
}

// ctx 剩余期限不足以等待 Retry-After 时，应立即返回上游错误，而不是把失败推迟到超时。
func TestRequestDoesNotSleepPastContextDeadline(t *testing.T) {
	disableRequestPacing(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error_code":"rate_limited","error_message":"too many requests"}`))
	}))
	defer server.Close()

	adapter := NewDujiaoNextAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL}, t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := adapter.ListCategories(ctx); err == nil {
		t.Fatal("expected rate limited error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("expected immediate failure when deadline is shorter than Retry-After, waited %v", elapsed)
	}
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

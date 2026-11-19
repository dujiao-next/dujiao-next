package errorloghttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/errorlog/application"
	"github.com/dujiao-next/internal/modules/errorlog/contract"
	"github.com/dujiao-next/internal/modules/errorlog/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type captureRepo struct {
	mu   sync.Mutex
	logs []*domain.ErrorLog
}

func (r *captureRepo) Create(log *domain.ErrorLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, log)
	return nil
}

func (r *captureRepo) BatchCreate(logs []*domain.ErrorLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, logs...)
	return nil
}

func (r *captureRepo) ListAdmin(contract.Filter) ([]domain.ErrorLog, int64, error) {
	return nil, 0, nil
}

func (r *captureRepo) DeleteOlderThan(time.Time) (int64, error) { return 0, nil }

func (r *captureRepo) all() []*domain.ErrorLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*domain.ErrorLog, len(r.logs))
	copy(out, r.logs)
	return out
}

// testRequestIDMiddleware 等价于 httpserver 的 RequestIDMiddleware：
// 本包不能反向依赖 internal/app，测试里用最小实现写入同一个 key。
func testRequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" {
			requestID = "test-request-id"
		}
		c.Set(RequestIDKey, requestID)
		c.Next()
	}
}

// testRecoveryMiddleware 等价于 httpserver 的 RecoveryMiddleware：捕获 panic 并写出真实 HTTP 500，
// 用于验证 ErrorLogMiddleware 在它之前注册时仍能读到 panic 场景的状态码。
func testRecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				if !c.Writer.Written() {
					c.AbortWithStatus(http.StatusInternalServerError)
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}

func newTestRouter(repo *captureRepo, handler gin.HandlerFunc) (*gin.Engine, *application.Service) {
	gin.SetMode(gin.TestMode)
	svc := application.NewService(repo, nil)
	r := gin.New()
	r.Use(testRequestIDMiddleware())
	r.Use(ErrorLogMiddleware(svc))
	r.Use(testRecoveryMiddleware())
	r.GET("/boom", handler)
	r.POST("/api/v1/payment/callback", handler)
	return r, svc
}

// 业务 500：本项目约定 response.Error() 返回 HTTP 200 + body.status_code=500。
// 这条路径必须能被记录，否则整个功能等于失效。
func TestErrorLogMiddleware_RecordsBusiness500WithHTTP200(t *testing.T) {
	repo := &captureRepo{}
	r, svc := newTestRouter(repo, func(c *gin.Context) {
		ginutil.RespondError(c, response.CodeInternal, "error.internal_server_error", assertErr("db exploded"))
	})
	defer svc.Stop()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("前置假设错误：业务错误应返回 HTTP 200，实际 %d", w.Code)
	}

	svc.Stop()
	logs := repo.all()
	if len(logs) != 1 {
		t.Fatalf("业务 500 应被记录 1 条，实际 %d 条", len(logs))
	}
	if logs[0].StatusCode != 500 {
		t.Fatalf("状态码应为 500，实际 %d", logs[0].StatusCode)
	}
	if logs[0].Message == "" || logs[0].Message == "server error" {
		t.Fatalf("应带上真实错误信息，实际 %q", logs[0].Message)
	}
	if logs[0].RequestID == "" {
		t.Fatal("应带上 request_id")
	}
}

// panic 场景：RecoveryMiddleware 在下游捕获并写出真实 HTTP 500，
// ErrorLogMiddleware 在前置位置仍要读到。
func TestErrorLogMiddleware_RecordsPanic(t *testing.T) {
	repo := &captureRepo{}
	r, svc := newTestRouter(repo, func(*gin.Context) {
		panic("kaboom")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/payment/callback", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("panic 应返回 HTTP 500，实际 %d", w.Code)
	}

	svc.Stop()
	logs := repo.all()
	if len(logs) != 1 {
		t.Fatalf("panic 应被记录 1 条，实际 %d 条", len(logs))
	}
	if logs[0].Category != domain.CategoryPayment {
		t.Fatalf("支付回调路径应归类为 payment，实际 %q", logs[0].Category)
	}
}

// 4xx 是预期内的客户端错误，不应写库。
func TestErrorLogMiddleware_Skips4xx(t *testing.T) {
	repo := &captureRepo{}
	r, svc := newTestRouter(repo, func(c *gin.Context) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", assertErr("bad input"))
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))

	svc.Stop()
	if logs := repo.all(); len(logs) != 0 {
		t.Fatalf("4xx 不应被记录，实际 %d 条", len(logs))
	}
}

// 成功请求不应写库。
func TestErrorLogMiddleware_SkipsSuccess(t *testing.T) {
	repo := &captureRepo{}
	r, svc := newTestRouter(repo, func(c *gin.Context) {
		response.Success(c, gin.H{"ok": true})
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))

	svc.Stop()
	if logs := repo.all(); len(logs) != 0 {
		t.Fatalf("成功请求不应被记录，实际 %d 条", len(logs))
	}
}

// svc 为 nil 时中间件不应 panic（测试与未配置场景）。
func TestErrorLogMiddleware_NilServiceIsSafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ErrorLogMiddleware(nil))
	r.GET("/boom", func(c *gin.Context) {
		response.ErrorWithHTTPStatus(c, http.StatusInternalServerError, 500, "boom")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("响应不应受影响，实际 %d", w.Code)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

// 支付回调端点必须对网关返回 HTTP 200（否则网关无限重试），
// 失败只能由 handler 通过 ginutil.MarkErrorLog 显式标记。
// 若不支持这条路径，所有回调类失败都不会进入后台错误日志。
func TestErrorLogMiddleware_RecordsMarkedErrorWithHTTP200(t *testing.T) {
	repo := &captureRepo{}
	r, svc := newTestRouter(repo, func(c *gin.Context) {
		ginutil.MarkErrorLog(c, domain.LevelWarn, "globepay_callback_payment_not_found: foo [partner_order_id=X]")
		c.String(http.StatusOK, "fail")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/payment/callback", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("前置假设错误：回调端点应返回 HTTP 200，实际 %d", w.Code)
	}

	svc.Stop()
	logs := repo.all()
	if len(logs) != 1 {
		t.Fatalf("显式标记的回调失败应被记录 1 条，实际 %d 条", len(logs))
	}
	if logs[0].Level != domain.LevelWarn {
		t.Fatalf("级别应取 handler 显式声明（warn），实际 %q", logs[0].Level)
	}
	if logs[0].Category != domain.CategoryPayment {
		t.Fatalf("回调路径应归类为 payment，实际 %q", logs[0].Category)
	}
	if !strings.Contains(logs[0].Message, "globepay_callback_payment_not_found") {
		t.Fatalf("message 应保留 alert_type，实际 %q", logs[0].Message)
	}
}

// 显式标记的 error 级别不能被状态码推导覆盖成 warn。
func TestErrorLogMiddleware_MarkedLevelOverridesStatusDerived(t *testing.T) {
	repo := &captureRepo{}
	r, svc := newTestRouter(repo, func(c *gin.Context) {
		ginutil.MarkErrorLog(c, domain.LevelError, "epay_callback_handle_failed: sign mismatch")
		c.String(http.StatusOK, "fail")
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/payment/callback", nil))

	svc.Stop()
	logs := repo.all()
	if len(logs) != 1 {
		t.Fatalf("应记录 1 条，实际 %d 条", len(logs))
	}
	if logs[0].Level != domain.LevelError {
		t.Fatalf("级别应为 error，实际 %q", logs[0].Level)
	}
}

// collect_4xx=true 时，4xx 也应被记录为 warn 级别。
func TestErrorLogMiddleware_Records4xxWhenEnabled(t *testing.T) {
	repo := &captureRepo{}
	gin.SetMode(gin.TestMode)
	svc := application.NewService(repo, &application.Options{Collect4xx: true})
	r := gin.New()
	r.Use(testRequestIDMiddleware())
	r.Use(ErrorLogMiddleware(svc))
	r.Use(testRecoveryMiddleware())
	r.GET("/boom", func(c *gin.Context) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", assertErr("bad input"))
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))

	svc.Stop()
	logs := repo.all()
	if len(logs) != 1 {
		t.Fatalf("collect_4xx=true 时 4xx 应被记录 1 条，实际 %d 条", len(logs))
	}
	if logs[0].Level != domain.LevelWarn {
		t.Fatalf("4xx 应记录为 warn，实际 %q", logs[0].Level)
	}
	if logs[0].StatusCode != 400 {
		t.Fatalf("状态码应为 400，实际 %d", logs[0].StatusCode)
	}
	// 4xx 不会挂 c.Errors，message 必须来自 ginutil 透传的 i18n key，
	// 不能兜底成 "server error" —— 否则后台看到的 400 全长得一样，没法定位原因。
	if logs[0].Message == "" || logs[0].Message == "server error" {
		t.Fatalf("4xx 应带上具体原因，实际 %q", logs[0].Message)
	}
	if !strings.Contains(logs[0].Message, "error.bad_request") {
		t.Fatalf("message 应包含原始 i18n key 便于聚合，实际 %q", logs[0].Message)
	}
}

// RespondErrorWithMsg 传的是已翻译文案（没有 i18n key），应原样落库。
func TestErrorLogMiddleware_Records4xxCustomMessage(t *testing.T) {
	repo := &captureRepo{}
	gin.SetMode(gin.TestMode)
	svc := application.NewService(repo, &application.Options{Collect4xx: true})
	r := gin.New()
	r.Use(testRequestIDMiddleware())
	r.Use(ErrorLogMiddleware(svc))
	r.Use(testRecoveryMiddleware())
	r.GET("/boom", func(c *gin.Context) {
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, "邮箱格式不正确", nil)
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))

	svc.Stop()
	logs := repo.all()
	if len(logs) != 1 {
		t.Fatalf("应记录 1 条，实际 %d 条", len(logs))
	}
	if logs[0].Message != "邮箱格式不正确" {
		t.Fatalf("自定义文案应原样保留，实际 %q", logs[0].Message)
	}
}

// collect_4xx=false 时，4xx 依然不记录（默认行为回归）。
func TestErrorLogMiddleware_Skips4xxWhenDisabled(t *testing.T) {
	repo := &captureRepo{}
	gin.SetMode(gin.TestMode)
	svc := application.NewService(repo, &application.Options{Collect4xx: false})
	r := gin.New()
	r.Use(testRequestIDMiddleware())
	r.Use(ErrorLogMiddleware(svc))
	r.Use(testRecoveryMiddleware())
	r.GET("/boom", func(c *gin.Context) {
		ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))

	svc.Stop()
	if logs := repo.all(); len(logs) != 0 {
		t.Fatalf("collect_4xx=false 时 4xx 不应被记录，实际 %d 条", len(logs))
	}
}

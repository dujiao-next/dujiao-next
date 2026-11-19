package errorloghttp

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/modules/errorlog/application"
	"github.com/dujiao-next/internal/modules/errorlog/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/gin-gonic/gin"
)

// ErrorLogMiddleware 在每个请求完成后，把错误响应记录进 error_logs 表。
//
// 设计约束：
//  1. 必须在 RecoveryMiddleware 之前注册：panic 会穿过 c.Next() 帧向上抛，
//     放在 Recovery 之后会导致本中间件 c.Next() 后续代码不执行，漏掉全部 panic。
//  2. 判断 5xx 有两个来源，缺一不可：
//     - c.Writer.Status()：真实 HTTP 500（recovery / auth 中间件走 ErrorWithHTTPStatus）
//     - ginutil.ServerErrorStatusKey：业务 500（本项目约定 HTTP 200 + body.status_code）
//  3. 4xx 默认不记录；svc.Collect4xx() 为 true 时一并收集。
//  4. 调用 svc.Record() 是非阻塞的，永不影响请求延迟。
//  5. svc 为 nil 时直接跳过，保证测试与未配置场景不 panic。
// RequestIDKey 是 RequestIDMiddleware 写入 gin.Context 的键，与 httpserver 中间件包保持一致。
const RequestIDKey = "request_id"

func ErrorLogMiddleware(svc *application.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if svc == nil {
			return
		}

		status := resolveErrorStatus(c, svc.Collect4xx())
		if status == 0 {
			return
		}

		svc.Record(buildEntry(c, status))
	}
}

// resolveErrorStatus 返回该请求应记录的状态码，0 表示无需记录。
//
// HTTP 状态码优先（真实 5xx）；否则取 handler 通过 ginutil 透传的业务状态码，
// 因为 response.Error() 一律返回 HTTP 200。
// collect4xx 为 true 时，4xx 同样返回实际状态码。
func resolveErrorStatus(c *gin.Context, collect4xx bool) int {
	// 先看真实 HTTP 状态
	httpCode := c.Writer.Status()
	if httpCode >= 500 {
		return httpCode
	}

	// 再看业务状态码（5xx 优先，4xx 按配置）
	if v, exists := c.Get(ginutil.ServerErrorStatusKey); exists {
		if code, ok := v.(int); ok {
			if code >= 500 {
				return code
			}
			if collect4xx && code >= 400 {
				return code
			}
		}
	}

	// 真实 HTTP 4xx（如 auth 中间件直接返回 401）
	if collect4xx && httpCode >= 400 {
		return httpCode
	}

	return 0
}

func buildEntry(c *gin.Context, status int) *domain.ErrorLog {
	path := c.Request.URL.Path
	method := strings.ToUpper(c.Request.Method)

	// 收集 c.Errors（handler 通过 c.Error() 附加的错误消息）
	var message, detail string
	if len(c.Errors) > 0 {
		message = c.Errors.Last().Error()
		if len(c.Errors) > 1 {
			detail = c.Errors.String()
		}
	}
	// 4xx 不挂 c.Errors（避免被 LoggerMiddleware 刷成 error 级别），
	// 错误描述改由 ginutil 通过独立 key 透传：可能是 i18n key，也可能是直接文案。
	// 5xx 已有 c.Errors 的真实错误，保留它，这里只做兜底。
	if message == "" {
		if v, exists := c.Get(ginutil.ServerErrorMessageKey); exists {
			if raw, ok := v.(string); ok && raw != "" {
				message = resolveErrorMessage(raw)
			}
		}
	}
	if message == "" {
		message = "server error"
	}

	// 读取由 RecoveryMiddleware 写入的 gin error（panic 场景）
	// gin 的 c.Errors 列表本身已经够用，这里不另行传递 stack，
	// 因为 stack 会由 RecoveryMiddleware 单独写入 zap 日志，
	// 数据库里只需保留能定位问题的摘要。

	var userID *uint
	if v, exists := c.Get("user_id"); exists {
		if id, ok := v.(uint); ok && id > 0 {
			uid := id
			userID = &uid
		}
	}
	var adminID *uint
	if v, exists := c.Get("admin_id"); exists {
		if id, ok := v.(uint); ok && id > 0 {
			aid := id
			adminID = &aid
		}
	}

	requestID := ""
	if v, exists := c.Get(RequestIDKey); exists {
		if id, ok := v.(string); ok {
			requestID = id
		}
	}

	level := domain.LevelError
	if status < 500 {
		level = domain.LevelWarn
	}
	// handler 可通过 ginutil.MarkErrorLog 显式声明级别：
	// 对外返回 200 但业务失败的回调类端点，靠状态码推导不出真实严重程度。
	if v, exists := c.Get(ginutil.ServerErrorLevelKey); exists {
		if l, ok := v.(string); ok && (l == domain.LevelError || l == domain.LevelWarn) {
			level = l
		}
	}

	return &domain.ErrorLog{
		Level:      level,
		Category:   domain.CategoryFromPath(path),
		Message:    truncate(message, 500),
		Detail:     truncate(detail, 2000),
		Method:     method,
		Path:       truncate(path, 255),
		StatusCode: status,
		ClientIP:   c.ClientIP(),
		UserAgent:  truncate(c.Request.UserAgent(), 300),
		RequestID:  requestID,
		UserID:     userID,
		AdminID:    adminID,
		CreatedAt:  time.Now(),
	}
}

// resolveErrorMessage 把 ginutil 透传的错误描述还原成可读文案。
//
// 传进来的是 i18n key（如 error.guest_email_required）时用中文翻译，
// 后台日志统一中文，方便按原因聚合；查不到（自定义文案，如 RespondErrorWithMsg
// 传入的校验详情）就原样使用。
func resolveErrorMessage(raw string) string {
	msg := i18n.T(i18n.LocaleZH, raw)
	if msg == raw {
		return raw
	}
	return msg + " (" + raw + ")"
}

// truncate 截断超长字符串，防止写入超出列长度限制。
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

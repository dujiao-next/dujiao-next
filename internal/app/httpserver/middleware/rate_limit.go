package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimitKeyFunc 生成限流 key 的函数
type RateLimitKeyFunc func(*gin.Context) string

// RateLimitRule 限流规则
type RateLimitRule struct {
	Prefix        string
	WindowSeconds int
	MaxRequests   int
	BlockSeconds  int
	MessageKey    string
}

var rateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
	redis.call("EXPIRE", KEYS[1], ARGV[1])
end
if tonumber(ARGV[2]) > 0 and tonumber(ARGV[3]) > 0 and current == tonumber(ARGV[2]) + 1 then
	redis.call("EXPIRE", KEYS[1], ARGV[3])
end
local ttl = redis.call("TTL", KEYS[1])
return {current, ttl}
`)

type localRateLimitEntry struct {
	count     int64
	expiresAt time.Time
}

type localRateLimiter struct {
	mu                      sync.Mutex
	entries                 map[string]localRateLimitEntry
	lastCapacityWarningAt   time.Time
	lastRedisFallbackWarnAt time.Time
}

const localRateLimitMaxEntries = 10000
const localRateLimitWarningInterval = time.Minute

func (l *localRateLimiter) increment(key string, rule RateLimitRule, now time.Time) (int64, int64, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = make(map[string]localRateLimitEntry)
	}

	entry, exists := l.entries[key]
	if exists && (entry.expiresAt.IsZero() || !now.Before(entry.expiresAt)) {
		delete(l.entries, key)
		exists = false
	}
	if !exists {
		if len(l.entries) >= localRateLimitMaxEntries {
			for existingKey, existing := range l.entries {
				if existing.expiresAt.IsZero() || !now.Before(existing.expiresAt) {
					delete(l.entries, existingKey)
				}
			}
		}
		if len(l.entries) >= localRateLimitMaxEntries {
			// 不淘汰仍在窗口内的计数器；容量耗尽时仅拒绝新的 key，
			// 防止攻击者用高基数 IP/标识绕过已有 key 的频率限制。
			ttl := int64(rule.WindowSeconds)
			if ttl < 1 {
				ttl = 1
			}
			shouldWarn := l.lastCapacityWarningAt.IsZero() ||
				now.Sub(l.lastCapacityWarningAt) >= localRateLimitWarningInterval
			if shouldWarn {
				l.lastCapacityWarningAt = now
			}
			return int64(rule.MaxRequests) + 1, ttl, shouldWarn
		}
		entry = localRateLimitEntry{expiresAt: now.Add(time.Duration(rule.WindowSeconds) * time.Second)}
	}
	entry.count++
	if entry.count == int64(rule.MaxRequests)+1 && rule.BlockSeconds > 0 {
		entry.expiresAt = now.Add(time.Duration(rule.BlockSeconds) * time.Second)
	}
	l.entries[key] = entry
	ttl := int64(entry.expiresAt.Sub(now).Seconds())
	if ttl < 1 {
		ttl = 1
	}
	return entry.count, ttl, false
}

func (l *localRateLimiter) shouldWarnRedisFallback(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.lastRedisFallbackWarnAt.IsZero() &&
		now.Sub(l.lastRedisFallbackWarnAt) < localRateLimitWarningInterval {
		return false
	}
	l.lastRedisFallbackWarnAt = now
	return true
}

// RateLimitMiddleware Redis 频率限制中间件。
// Redis 未配置或运行时暂时不可用时使用进程内兜底；该兜底仅对当前实例生效，
// 多实例部署必须配置共享 Redis 才能获得全局一致的计数。Redis 故障降级和
// 本地容量耗尽都会输出节流后的 warning，避免故障期间产生日志洪泛。
func RateLimitMiddleware(client *redis.Client, rule RateLimitRule, keyFunc RateLimitKeyFunc) gin.HandlerFunc {
	local := &localRateLimiter{}
	return func(c *gin.Context) {
		if rule.WindowSeconds <= 0 || rule.MaxRequests <= 0 {
			c.Next()
			return
		}

		key := ""
		if keyFunc != nil {
			key = strings.TrimSpace(keyFunc(c))
		}
		if key == "" {
			key = c.ClientIP()
		}
		if rule.Prefix != "" {
			key = fmt.Sprintf("%s:%s", rule.Prefix, key)
		}

		var count, ttlSeconds int64
		now := time.Now()
		incrementLocal := func() {
			var capacityWarning bool
			count, ttlSeconds, capacityWarning = local.increment(key, rule, now)
			if capacityWarning {
				logger.Warnw(
					"rate_limit_local_capacity_exhausted",
					"prefix", rule.Prefix,
					"max_entries", localRateLimitMaxEntries,
					"window_seconds", rule.WindowSeconds,
				)
			}
		}
		if client == nil {
			incrementLocal()
		} else {
			result, err := rateLimitScript.Run(
				c.Request.Context(),
				client,
				[]string{key},
				rule.WindowSeconds,
				rule.MaxRequests,
				rule.BlockSeconds,
			).Result()
			if err != nil {
				if local.shouldWarnRedisFallback(now) {
					logger.Warnw("rate_limit_redis_fallback", "prefix", rule.Prefix, "error", err)
				}
				incrementLocal()
			} else {
				values, ok := result.([]interface{})
				if !ok || len(values) < 2 {
					if local.shouldWarnRedisFallback(now) {
						logger.Warnw(
							"rate_limit_redis_fallback",
							"prefix", rule.Prefix,
							"error", fmt.Sprintf("unexpected result shape %T", result),
						)
					}
					incrementLocal()
				} else {
					count, ok = toInt64(values[0])
					if !ok {
						if local.shouldWarnRedisFallback(now) {
							logger.Warnw(
								"rate_limit_redis_fallback",
								"prefix", rule.Prefix,
								"error", fmt.Sprintf("invalid count type %T", values[0]),
							)
						}
						incrementLocal()
					} else {
						ttlSeconds, _ = toInt64(values[1])
					}
				}
			}
		}
		if count > int64(rule.MaxRequests) {
			waitSeconds := int(ttlSeconds)
			if waitSeconds < 1 {
				waitSeconds = rule.WindowSeconds
			}
			if waitSeconds < 1 {
				waitSeconds = 1
			}
			msgKey := strings.TrimSpace(rule.MessageKey)
			if msgKey == "" {
				msgKey = "error.rate_limited"
			}
			msg := i18n.Sprintf(i18n.ResolveLocale(c), msgKey, waitSeconds)
			if isChannelAPIRequest(c) {
				response.ChannelError(c, http.StatusTooManyRequests, response.CodeTooManyRequests, msg, "rate_limit_exceeded")
			} else {
				response.ErrorWithHTTPStatus(c, http.StatusTooManyRequests, response.CodeTooManyRequests, msg)
			}
			c.Abort()
			return
		}

		c.Next()
	}
}

func isChannelAPIRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	return strings.HasPrefix(c.Request.URL.Path, "/api/v1/channel")
}

// KeyByIP 使用 IP 作为限流 key
func KeyByIP(c *gin.Context) string {
	return c.ClientIP()
}

// KeyByUserIDAndIP isolates authenticated mutation limits by both account and
// source IP. It falls back to IP when authentication context is unavailable.
func KeyByUserIDAndIP(c *gin.Context) string {
	if c == nil {
		return ""
	}
	userID, exists := c.Get("user_id")
	if !exists {
		return c.ClientIP()
	}
	return fmt.Sprintf("%v|%s", userID, c.ClientIP())
}

// KeyByIPAndHeader 以 "IP|header 值" 作为限流 key。
// 限流位于鉴权之前，header 未经校验，因此计数必须同时绑定来源 IP。
func KeyByIPAndHeader(header string) RateLimitKeyFunc {
	return func(c *gin.Context) string {
		value := c.GetHeader(header)
		if len(value) > 128 {
			value = value[:128]
		}
		if value == "" {
			return c.ClientIP()
		}
		return c.ClientIP() + "|" + value
	}
}

// KeyByUpstreamApiKey 使用 "IP|上游 API Key" 作为限流 key
func KeyByUpstreamApiKey(c *gin.Context) string {
	return KeyByIPAndHeader("Dujiao-Next-Api-Key")(c)
}

// KeyByIPAndJSONField 使用 IP + JSON 字段作为限流 key
func KeyByIPAndJSONField(field string) RateLimitKeyFunc {
	return func(c *gin.Context) string {
		value := strings.ToLower(strings.TrimSpace(readJSONField(c, field)))
		if value == "" {
			return c.ClientIP()
		}
		return fmt.Sprintf("%s|%s", value, c.ClientIP())
	}
}

func readJSONField(c *gin.Context, field string) string {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return ""
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	if len(body) == 0 {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	value, ok := payload[field]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func toInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case int16:
		return int64(v), true
	case int8:
		return int64(v), true
	case uint64:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint8:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	default:
		return 0, false
	}
}

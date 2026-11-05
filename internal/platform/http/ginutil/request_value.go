package ginutil

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ParseParamUint 解析路径参数中的正整数 ID。
func ParseParamUint(c *gin.Context, key string) (uint, error) {
	if c == nil {
		return 0, errors.New("context is nil")
	}
	raw := strings.TrimSpace(c.Param(key))
	if raw == "" {
		return 0, errors.New("param is empty")
	}
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New("param is invalid")
	}
	return uint(parsed), nil
}

// ParseQueryUint 解析可选查询参数中的正整数 ID。
func ParseQueryUint(raw string, zeroInvalid bool) (uint, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		return 0, err
	}
	if zeroInvalid && parsed == 0 {
		return 0, errors.New("query value is invalid")
	}
	return uint(parsed), nil
}

// ParseQueryBool 解析可选查询参数中的布尔值，未传或空白返回 false。
func ParseQueryBool(c *gin.Context, key string) (bool, error) {
	value, err := ParseQueryBoolPtr(c, key)
	if err != nil {
		return false, err
	}
	if value == nil {
		return false, nil
	}
	return *value, nil
}

// ParseQueryBoolPtr 解析可选查询参数中的布尔值，未传或空白返回 nil。
func ParseQueryBoolPtr(c *gin.Context, key string) (*bool, error) {
	if c == nil {
		return nil, errors.New("context is nil")
	}
	raw, ok := c.GetQuery(key)
	if !ok {
		return nil, nil
	}
	return ParseOptionalBoolValue(raw)
}

// ParseOptionalBoolValue 解析可选布尔字符串，空白返回 nil。
func ParseOptionalBoolValue(raw string) (*bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// RequestScheme 返回当前请求的 scheme（优先 X-Forwarded-Proto）。
func RequestScheme(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		proto = strings.ToLower(strings.TrimSpace(strings.Split(proto, ",")[0]))
		if proto == "http" || proto == "https" {
			return proto
		}
	}
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}

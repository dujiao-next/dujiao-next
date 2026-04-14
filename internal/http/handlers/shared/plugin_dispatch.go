package shared

import (
	"io"
	"strings"

	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"
	"github.com/dujiao-next/pluginhost"

	"github.com/gin-gonic/gin"
)

// HandleMountedPluginRoute 优先将请求分发给挂载到原始路由的插件；若无插件接管则返回 false。
func HandleMountedPluginRoute(c *gin.Context, pluginService *service.PluginCenterService, scope, routePattern string) bool {
	if c == nil || pluginService == nil {
		return false
	}
	if !pluginService.HasMountedRoute(scope, routePattern, c.Request.Method) {
		return false
	}
	req, err := buildMountedPluginRequest(c, scope, routePattern)
	if err != nil {
		RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return true
	}
	result, err := pluginService.DispatchMounted(scope, routePattern, req)
	if err != nil {
		RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return true
	}
	writePluginResponse(c, result)
	return true
}

func buildMountedPluginRequest(c *gin.Context, scope, routePattern string) (*pluginhost.HTTPRequest, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	req := &pluginhost.HTTPRequest{
		Scope:         strings.TrimSpace(scope),
		Method:        strings.ToUpper(strings.TrimSpace(c.Request.Method)),
		RoutePattern:  strings.TrimSpace(routePattern),
		Path:          scopedRequestPath(c, scope),
		Query:         c.Request.URL.Query(),
		Headers:       flattenHeaders(c),
		Body:          body,
		ContextValues: map[string]interface{}{},
	}
	if value, ok := c.Get("user_id"); ok {
		switch typed := value.(type) {
		case uint:
			req.UserID = typed
		case int:
			if typed > 0 {
				req.UserID = uint(typed)
			}
		}
	}
	if value, ok := c.Get("admin_id"); ok {
		switch typed := value.(type) {
		case uint:
			req.AdminID = typed
		case int:
			if typed > 0 {
				req.AdminID = uint(typed)
			}
		}
	}
	for _, key := range []string{"request_id", "channel_client_id", "channel_key", "channel_type", "username"} {
		if value, ok := c.Get(key); ok {
			req.ContextValues[key] = value
		}
	}
	return req, nil
}

func scopedRequestPath(c *gin.Context, scope string) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return "/"
	}
	path := strings.TrimSpace(c.Request.URL.Path)
	prefix := scopePrefix(scope)
	if prefix != "" && strings.HasPrefix(path, prefix) {
		path = strings.TrimPrefix(path, prefix)
	}
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func scopePrefix(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case pluginhost.ScopeAdmin:
		return "/api/v1/admin"
	case pluginhost.ScopeAuth:
		return "/api/v1/auth"
	case pluginhost.ScopeUser:
		return "/api/v1"
	case pluginhost.ScopeChannel:
		return "/api/v1/channel"
	default:
		return "/api/v1/public"
	}
}

func flattenHeaders(c *gin.Context) map[string]string {
	result := make(map[string]string)
	if c == nil || c.Request == nil {
		return result
	}
	for key, values := range c.Request.Header {
		if len(values) == 0 {
			continue
		}
		result[key] = values[0]
	}
	return result
}

func writePluginResponse(c *gin.Context, result *pluginhost.HTTPResponse) {
	if c == nil {
		return
	}
	if result == nil {
		c.JSON(200, gin.H{})
		return
	}
	for key, value := range result.Headers {
		c.Header(key, value)
	}
	statusCode := result.StatusCode
	if statusCode <= 0 {
		statusCode = 200
	}
	if len(result.RawBody) > 0 {
		contentType := strings.TrimSpace(result.Headers["Content-Type"])
		if contentType == "" {
			contentType = c.ContentType()
		}
		c.Data(statusCode, contentType, result.RawBody)
		return
	}
	c.JSON(statusCode, result.Data)
}

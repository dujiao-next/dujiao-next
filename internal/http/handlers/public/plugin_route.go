package public

import (
	"errors"
	"io"
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/pluginhost"

	"github.com/gin-gonic/gin"
)

// HandlePluginPublicRoute 处理插件公共路由。
func (h *Handler) HandlePluginPublicRoute(c *gin.Context) {
	h.dispatchPlugin(pluginhost.ScopePublic, c)
}

func (h *Handler) dispatchPlugin(scope string, c *gin.Context) {
	if h.PluginCenterService == nil {
		shared.RespondError(c, response.CodeNotFound, "error.not_found", errors.New("plugin service unavailable"))
		return
	}
	body, _ := io.ReadAll(c.Request.Body)
	req := &pluginhost.HTTPRequest{
		PluginID: strings.TrimSpace(c.Param("plugin_id")),
		Scope:    scope,
		Method:   strings.ToUpper(strings.TrimSpace(c.Request.Method)),
		Path:     strings.TrimSpace(c.Param("path")),
		Query:    c.Request.URL.Query(),
		Headers:  flattenHeaders(c),
		Body:     body,
	}
	if userID, ok := c.Get("user_id"); ok {
		if value, ok := userID.(uint); ok {
			req.UserID = value
		}
	}
	result, err := h.PluginCenterService.Dispatch(scope, req.PluginID, req)
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	writePluginHTTPResponse(c, result)
}

func flattenHeaders(c *gin.Context) map[string]string {
	result := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) == 0 {
			continue
		}
		result[key] = values[0]
	}
	return result
}

func writePluginHTTPResponse(c *gin.Context, result *pluginhost.HTTPResponse) {
	if result == nil {
		response.Success(c, gin.H{})
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
		c.Data(statusCode, c.ContentType(), result.RawBody)
		return
	}
	c.JSON(statusCode, result.Data)
}

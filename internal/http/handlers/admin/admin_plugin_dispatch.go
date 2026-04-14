package admin

import (
	"errors"
	"io"
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/pluginhost"

	"github.com/gin-gonic/gin"
)

// HandlePluginAdminRoute 处理插件管理路由。
func (h *Handler) HandlePluginAdminRoute(c *gin.Context) {
	if h.PluginCenterService == nil {
		shared.RespondError(c, response.CodeNotFound, "error.not_found", errors.New("plugin service unavailable"))
		return
	}
	body, _ := io.ReadAll(c.Request.Body)
	req := &pluginhost.HTTPRequest{
		PluginID: strings.TrimSpace(c.Param("plugin_id")),
		Scope:    pluginhost.ScopeAdmin,
		Method:   strings.ToUpper(strings.TrimSpace(c.Request.Method)),
		Path:     strings.TrimSpace(c.Param("path")),
		Query:    c.Request.URL.Query(),
		Headers:  map[string]string{},
		Body:     body,
	}
	if req.PluginID == "" {
		req.PluginID = strings.TrimSpace(c.Param("id"))
	}
	if adminID, ok := c.Get("admin_id"); ok {
		if value, ok := adminID.(uint); ok {
			req.AdminID = value
		}
	}
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			req.Headers[key] = values[0]
		}
	}
	result, err := h.PluginCenterService.Dispatch(pluginhost.ScopeAdmin, req.PluginID, req)
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	writeAdminPluginHTTPResponse(c, result)
}

func writeAdminPluginHTTPResponse(c *gin.Context, result *pluginhost.HTTPResponse) {
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

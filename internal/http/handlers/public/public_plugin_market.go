package public

import (
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

// GetPluginMarketFeed 获取在线插件市场 feed。
func (h *Handler) GetPluginMarketFeed(c *gin.Context) {
	feed, err := h.OnlinePluginCenterService.BuildPublicFeed(strings.TrimSpace(c.DefaultQuery("scope", "official")))
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "获取在线插件 feed 失败", err)
		return
	}
	response.Success(c, feed)
}

// GetPluginMarketPublicPlugins 获取公开插件列表。
func (h *Handler) GetPluginMarketPublicPlugins(c *gin.Context) {
	page := parsePublicIntDefault(c.Query("page"), 1)
	pageSize := parsePublicIntDefault(c.Query("page_size"), 20)
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	items, total, err := h.OnlinePluginCenterService.ListPublicPlugins(service.OnlinePluginCenterPublicListFilter{
		Page:        page,
		PageSize:    pageSize,
		Keyword:     strings.TrimSpace(c.Query("q")),
		PluginType:  strings.TrimSpace(c.Query("plugin_type")),
		BillingMode: strings.TrimSpace(c.Query("billing_mode")),
		Scope:       strings.TrimSpace(c.Query("scope")),
	})
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "获取在线插件列表失败", err)
		return
	}
	response.SuccessWithPage(c, items, response.BuildPagination(page, pageSize, total))
}

// GetPluginMarketPublicPlugin 获取公开插件详情。
func (h *Handler) GetPluginMarketPublicPlugin(c *gin.Context) {
	item, err := h.OnlinePluginCenterService.GetPublicPluginDetail(strings.TrimSpace(c.Param("plugin_id")))
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "获取在线插件详情失败", err)
		return
	}
	if item == nil {
		shared.RespondErrorWithMsg(c, response.CodeNotFound, "在线插件不存在", nil)
		return
	}
	response.Success(c, item)
}

// GetPluginMarketPublicVersions 获取公开插件版本列表。
func (h *Handler) GetPluginMarketPublicVersions(c *gin.Context) {
	items, err := h.OnlinePluginCenterService.GetPublicVersions(strings.TrimSpace(c.Param("plugin_id")))
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "获取在线插件版本失败", err)
		return
	}
	response.Success(c, items)
}

// GetPluginMarketPublicPlans 获取公开插件套餐列表。
func (h *Handler) GetPluginMarketPublicPlans(c *gin.Context) {
	items, err := h.OnlinePluginCenterService.GetPublicPlans(strings.TrimSpace(c.Param("plugin_id")))
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "获取在线插件套餐失败", err)
		return
	}
	response.Success(c, items)
}

// GetPluginMarketPublicDownload 获取公开插件下载信息。
func (h *Handler) GetPluginMarketPublicDownload(c *gin.Context) {
	item, err := h.OnlinePluginCenterService.GetPublicDownloadInfo(strings.TrimSpace(c.Param("plugin_id")), strings.TrimSpace(c.Query("version")))
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "获取在线插件下载信息失败", err)
		return
	}
	if item == nil {
		shared.RespondErrorWithMsg(c, response.CodeNotFound, "在线插件下载信息不存在", nil)
		return
	}
	response.Success(c, item)
}

func parsePublicIntDefault(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

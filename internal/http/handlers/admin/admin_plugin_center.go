package admin

import (
	"errors"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/repository"

	"github.com/gin-gonic/gin"
)

// ListPlugins 获取插件列表。
func (h *Handler) ListPlugins(c *gin.Context) {
	if h.PluginCenterService == nil {
		response.SuccessWithPage(c, []interface{}{}, response.BuildPagination(1, 20, 0))
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = shared.NormalizePagination(page, pageSize)
	var enabledPtr *bool
	if raw := strings.TrimSpace(c.Query("is_enabled")); raw != "" {
		value := raw == "1" || strings.EqualFold(raw, "true")
		enabledPtr = &value
	}
	items, total, err := h.PluginCenterService.ListPlugins(repository.PluginListFilter{
		Page:      page,
		PageSize:  pageSize,
		Keyword:   strings.TrimSpace(c.Query("keyword")),
		Type:      strings.TrimSpace(c.Query("type")),
		Status:    strings.TrimSpace(c.Query("status")),
		Source:    strings.TrimSpace(c.Query("source")),
		IsEnabled: enabledPtr,
	})
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, items, response.BuildPagination(page, pageSize, total))
}

// ListPluginRuntimePages 获取当前运行时已挂载的插件页面。
func (h *Handler) ListPluginRuntimePages(c *gin.Context) {
	if h.PluginCenterService == nil {
		response.Success(c, []interface{}{})
		return
	}
	items := h.PluginCenterService.ListMountedPages(strings.TrimSpace(c.Query("scope")))
	response.Success(c, items)
}

// UploadPlugin 上传插件包。
func (h *Handler) UploadPlugin(c *gin.Context) {
	if h.PluginCenterService == nil {
		shared.RespondError(c, response.CodeInternal, "error.upload_failed", errors.New("plugin service unavailable"))
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.file_missing", err)
		return
	}
	result, err := h.PluginCenterService.UploadArchive(file)
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	response.Success(c, result)
}

// GetPlugin 获取插件详情。
func (h *Handler) GetPlugin(c *gin.Context) {
	if h.PluginCenterService == nil {
		shared.RespondError(c, response.CodeNotFound, "error.not_found", errors.New("plugin service unavailable"))
		return
	}
	item, err := h.PluginCenterService.GetPluginDetail(strings.TrimSpace(c.Param("id")))
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	if item == nil {
		shared.RespondError(c, response.CodeNotFound, "error.not_found", errors.New("plugin not found"))
		return
	}
	response.Success(c, item)
}

type pluginVersionRequest struct {
	Version string `json:"version"`
}

// InstallPlugin 安装插件。
func (h *Handler) InstallPlugin(c *gin.Context) {
	var req pluginVersionRequest
	_ = c.ShouldBindJSON(&req)
	item, err := h.PluginCenterService.InstallUploaded(strings.TrimSpace(c.Param("id")), strings.TrimSpace(req.Version))
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	response.Success(c, item)
}

// EnablePlugin 启用插件。
func (h *Handler) EnablePlugin(c *gin.Context) {
	var req pluginVersionRequest
	_ = c.ShouldBindJSON(&req)
	item, err := h.PluginCenterService.EnablePlugin(strings.TrimSpace(c.Param("id")), strings.TrimSpace(req.Version))
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	response.Success(c, item)
}

// DisablePlugin 禁用插件。
func (h *Handler) DisablePlugin(c *gin.Context) {
	item, err := h.PluginCenterService.DisablePlugin(strings.TrimSpace(c.Param("id")))
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	response.Success(c, item)
}

// ApplyPluginReload 重载插件运行时。
func (h *Handler) ApplyPluginReload(c *gin.Context) {
	if err := h.PluginCenterService.ReloadRuntime(); err != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	response.Success(c, gin.H{"reloaded": true})
}

// RollbackPlugin 回滚插件。
func (h *Handler) RollbackPlugin(c *gin.Context) {
	item, err := h.PluginCenterService.RollbackPlugin(strings.TrimSpace(c.Param("id")))
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	response.Success(c, item)
}

// DeletePlugin 删除插件。
func (h *Handler) DeletePlugin(c *gin.Context) {
	purge := c.Query("purge") == "1" || strings.EqualFold(c.Query("purge"), "true")
	if err := h.PluginCenterService.RemovePlugin(strings.TrimSpace(c.Param("id")), purge); err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	response.Success(c, gin.H{"deleted": true, "purge": purge})
}

// GetPluginLogs 获取插件日志。
func (h *Handler) GetPluginLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = shared.NormalizePagination(page, pageSize)
	items, total, err := h.PluginCenterService.ListPluginLogs(repository.PluginLogListFilter{
		Page:      page,
		PageSize:  pageSize,
		PluginID:  strings.TrimSpace(c.Param("id")),
		Level:     strings.TrimSpace(c.Query("level")),
		EventType: strings.TrimSpace(c.Query("event_type")),
	})
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, items, response.BuildPagination(page, pageSize, total))
}

// GetPluginConfig 获取插件配置。
func (h *Handler) GetPluginConfig(c *gin.Context) {
	value, err := h.PluginCenterService.GetPluginConfig(strings.TrimSpace(c.Param("id")))
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	response.Success(c, value)
}

// UpdatePluginConfig 更新插件配置。
func (h *Handler) UpdatePluginConfig(c *gin.Context) {
	payload := map[string]interface{}{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	value, err := h.PluginCenterService.UpdatePluginConfig(strings.TrimSpace(c.Param("id")), payload)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		return
	}
	response.Success(c, value)
}

// ListPluginMarketRegistries 获取在线插件库注册表。
func (h *Handler) ListPluginMarketRegistries(c *gin.Context) {
	items, err := h.PluginCenterService.ListRegistries()
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	response.Success(c, items)
}

// RefreshPluginMarket 刷新在线插件库。
func (h *Handler) RefreshPluginMarket(c *gin.Context) {
	if err := h.PluginCenterService.RefreshMarket(); err != nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, err.Error(), err)
		return
	}
	response.Success(c, gin.H{"refreshed": true})
}

// ListPluginMarketItems 获取在线插件列表。
func (h *Handler) ListPluginMarketItems(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = shared.NormalizePagination(page, pageSize)
	items, total, err := h.PluginCenterService.ListMarketItems(repository.PluginMarketListFilter{
		Page:       page,
		PageSize:   pageSize,
		RegistryID: strings.TrimSpace(c.Query("registry_id")),
		Keyword:    strings.TrimSpace(c.Query("keyword")),
		Type:       strings.TrimSpace(c.Query("type")),
		PluginID:   strings.TrimSpace(c.Query("plugin_id")),
	})
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, items, response.BuildPagination(page, pageSize, total))
}

// GetPluginMarketItem 获取在线插件详情。
func (h *Handler) GetPluginMarketItem(c *gin.Context) {
	registryID := strings.TrimSpace(c.Query("registry_id"))
	item, versions, err := h.PluginCenterService.GetMarketItem(registryID, strings.TrimSpace(c.Param("plugin_id")))
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	if item == nil {
		shared.RespondError(c, response.CodeNotFound, "error.not_found", errors.New("market item not found"))
		return
	}
	response.Success(c, gin.H{"item": item, "versions": versions})
}

// GetPluginMarketVersions 获取在线插件版本列表。
func (h *Handler) GetPluginMarketVersions(c *gin.Context) {
	registryID := strings.TrimSpace(c.Query("registry_id"))
	_, versions, err := h.PluginCenterService.GetMarketItem(registryID, strings.TrimSpace(c.Param("plugin_id")))
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	response.Success(c, versions)
}

type marketInstallRequest struct {
	RegistryID string `json:"registry_id"`
	Version    string `json:"version"`
}

// InstallPluginMarketItem 从在线插件库安装插件。
func (h *Handler) InstallPluginMarketItem(c *gin.Context) {
	var req marketInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	item, err := h.PluginCenterService.InstallFromMarket(strings.TrimSpace(req.RegistryID), strings.TrimSpace(c.Param("plugin_id")), strings.TrimSpace(req.Version))
	if err != nil {
		shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	response.Success(c, item)
}

package public

import (
	"strings"

	"github.com/dujiao-next/internal/logger"

	"github.com/gin-gonic/gin"
)

// GetSitemap GET /sitemap.xml
func (h *Handler) GetSitemap(c *gin.Context) {
	if h.SitemapService == nil {
		c.String(503, "sitemap service unavailable")
		return
	}

	baseURL := h.resolveSitemapBaseURL(c)
	localeURLMode := "none"
	if h.SettingService != nil {
		if cfg, err := h.SettingService.GetConfig(nil); err == nil {
			if mode, ok := cfg["locale_url_mode"].(string); ok {
				localeURLMode = mode
			}
		}
	}
	xmlStr, err := h.SitemapService.Generate(c.Request.Context(), baseURL, localeURLMode)
	if err != nil {
		logger.Errorw("sitemap_generate_failed", "error", err)
		c.String(500, "internal error")
		return
	}

	c.Header("Cache-Control", "public, max-age=300")
	c.Data(200, "application/xml; charset=utf-8", []byte(xmlStr))
}

// GetRobots GET /robots.txt
func (h *Handler) GetRobots(c *gin.Context) {
	baseURL := ""
	if h.SitemapService != nil {
		baseURL = h.resolveSitemapBaseURL(c)
	}
	body := ""
	if h.SitemapService != nil {
		body = h.SitemapService.GenerateRobots(baseURL)
	} else {
		body = "User-agent: *\nDisallow:\n"
	}

	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(200, "text/plain; charset=utf-8", []byte(body))
}

// resolveSitemapBaseURL 站点 URL 优先取后台 brand.site_url，否则回落到当前请求 Host
func (h *Handler) resolveSitemapBaseURL(c *gin.Context) string {
	if h.SettingService != nil {
		if brand, err := h.SettingService.GetSiteBrand(); err == nil {
			if u := strings.TrimRight(strings.TrimSpace(brand.SiteURL), "/"); u != "" {
				return u
			}
		}
	}

	scheme := "https"
	if c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") == "" {
		scheme = "http"
	}
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host := c.Request.Host
	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}
	return scheme + "://" + host
}

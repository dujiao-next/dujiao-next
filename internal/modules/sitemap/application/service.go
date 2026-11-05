package application

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/sitemap/contract"
	"github.com/dujiao-next/internal/modules/sitemap/domain"
)

// Service 生成 sitemap.xml / robots.txt 内容。
type Service struct {
	catalog contract.CatalogReader
	posts   contract.PublishedPostReader
	cache   contract.Cache
}

// NewService 创建 sitemap 服务。
func NewService(
	catalog contract.CatalogReader,
	posts contract.PublishedPostReader,
	cacheStore contract.Cache,
) (*Service, error) {
	if catalog == nil || posts == nil || cacheStore == nil {
		return nil, fmt.Errorf("sitemap: required dependency is nil")
	}
	return &Service{
		catalog: catalog,
		posts:   posts,
		cache:   cacheStore,
	}, nil
}

const (
	sitemapCacheTTL    = 5 * time.Minute
	sitemapCachePrefix = "sitemap:xml:"
	sitemapMaxFetch    = 50000 // 单次拉取上限，避免极端数据量打爆内存
)

// Generate 生成 sitemap.xml 内容；baseURL 必须是不带尾斜杠的站点根（如 https://example.com）
func (s *Service) Generate(ctx context.Context, baseURL string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("sitemap: empty base url")
	}

	cacheKey := sitemapCachePrefix + baseURL
	if cached, err := s.cache.GetString(ctx, cacheKey); err == nil && cached != "" {
		return cached, nil
	}

	entries, err := s.collectURLs(ctx, baseURL)
	if err != nil {
		return "", err
	}

	xmlStr, err := renderSitemapXML(entries)
	if err != nil {
		return "", err
	}

	_ = s.cache.SetString(ctx, cacheKey, xmlStr, sitemapCacheTTL)
	return xmlStr, nil
}

// GenerateRobots 生成 robots.txt 内容
func (s *Service) GenerateRobots(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	// 前台是 CSR，爬虫渲染时要能请求公开接口，否则商品页会被判为 Soft 404
	b.WriteString("Allow: /api/v1/public/\n")
	b.WriteString("Disallow: /api/\n")
	b.WriteString("Disallow: /admin/\n")
	b.WriteString("Disallow: /me/\n")
	b.WriteString("Disallow: /cart\n")
	b.WriteString("Disallow: /checkout\n")
	b.WriteString("Disallow: /pay\n")
	b.WriteString("Disallow: /orders/\n")
	b.WriteString("Disallow: /recharge-orders/\n")
	b.WriteString("Disallow: /guest/\n")
	b.WriteString("Disallow: /auth/\n")
	if baseURL != "" {
		b.WriteString("\n")
		b.WriteString("Sitemap: ")
		b.WriteString(baseURL)
		b.WriteString("/sitemap.xml\n")
	}
	return b.String()
}

// xmlEntry 是 sitemap.xml 的传输结构。
type xmlEntry struct {
	XMLName    xml.Name `xml:"url"`
	Loc        string   `xml:"loc"`
	LastMod    string   `xml:"lastmod,omitempty"`
	ChangeFreq string   `xml:"changefreq,omitempty"`
	Priority   string   `xml:"priority,omitempty"`
}

type urlSet struct {
	XMLName xml.Name   `xml:"urlset"`
	Xmlns   string     `xml:"xmlns,attr"`
	URLs    []xmlEntry `xml:"url"`
}

func (s *Service) collectURLs(ctx context.Context, baseURL string) ([]domain.URL, error) {
	now := time.Now().UTC().Format("2006-01-02")
	entries := make([]domain.URL, 0, 64)

	// 1. 静态页面
	staticPages := []struct {
		Path       string
		ChangeFreq string
		Priority   string
	}{
		{"/", "daily", "1.0"},
		{"/products", "daily", "0.9"},
		{"/blog", "weekly", "0.6"},
		{"/notice", "weekly", "0.5"},
		{"/about", "monthly", "0.3"},
		{"/terms", "yearly", "0.2"},
		{"/privacy", "yearly", "0.2"},
	}
	for _, p := range staticPages {
		entries = append(entries, domain.URL{
			Location:        baseURL + p.Path,
			LastModified:    now,
			ChangeFrequency: p.ChangeFreq,
			Priority:        p.Priority,
		})
	}

	// 2. 启用的分类
	categories, err := s.catalog.ListActiveCategories()
	if err != nil {
		return nil, fmt.Errorf("sitemap: list categories: %w", err)
	}
	for _, cat := range categories {
		entries = append(entries, domain.URL{
			Location:        baseURL + "/categories/" + url.PathEscape(cat.Slug),
			LastModified:    cat.CreatedAt.UTC().Format("2006-01-02"),
			ChangeFrequency: "weekly",
			Priority:        "0.7",
		})
	}

	// 3. 上架的商品（OnlyActive 已含分类启用过滤）
	products, err := s.catalog.ListActiveProducts(sitemapMaxFetch)
	if err != nil {
		return nil, fmt.Errorf("sitemap: list products: %w", err)
	}
	for _, p := range products {
		entries = append(entries, domain.URL{
			Location:        baseURL + "/products/" + url.PathEscape(p.Slug),
			LastModified:    p.UpdatedAt.UTC().Format("2006-01-02"),
			ChangeFrequency: "daily",
			Priority:        "0.8",
		})
	}

	// 4. 已发布的博客 / 公告
	posts, err := s.posts.ListPublishedPosts(ctx, sitemapMaxFetch)
	if err != nil {
		return nil, fmt.Errorf("sitemap: list posts: %w", err)
	}
	for _, post := range posts {
		lastmod := post.CreatedAt
		if post.PublishedAt != nil {
			lastmod = *post.PublishedAt
		}
		// blog 与 notice 共用 /blog/:slug 详情页（user 前台 Notice.vue 跳转到 /blog/{slug}）
		entries = append(entries, domain.URL{
			Location:        baseURL + "/blog/" + url.PathEscape(post.Slug),
			LastModified:    lastmod.UTC().Format("2006-01-02"),
			ChangeFrequency: "monthly",
			Priority:        "0.5",
		})
	}

	return entries, nil
}

func renderSitemapXML(entries []domain.URL) (string, error) {
	xmlEntries := make([]xmlEntry, 0, len(entries))
	for _, entry := range entries {
		xmlEntries = append(xmlEntries, xmlEntry{
			Loc:        entry.Location,
			LastMod:    entry.LastModified,
			ChangeFreq: entry.ChangeFrequency,
			Priority:   entry.Priority,
		})
	}
	set := urlSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  xmlEntries,
	}
	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return "", err
	}
	return xml.Header + string(body) + "\n", nil
}

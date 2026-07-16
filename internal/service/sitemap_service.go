package service

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/repository"
)

// localeHreflangMap 前缀模式下的 locale 段 → hreflang 值映射
var localeHreflangMap = []struct {
	Short string
	Full  string
}{
	{"zh", "zh-CN"},
	{"en", "en-US"},
	{"tw", "zh-TW"},
}

// SitemapService 生成 sitemap.xml / robots.txt 内容
type SitemapService struct {
	productRepo  repository.ProductRepository
	categoryRepo repository.CategoryRepository
	postRepo     repository.PostRepository
}

// NewSitemapService 创建 sitemap 服务
func NewSitemapService(
	productRepo repository.ProductRepository,
	categoryRepo repository.CategoryRepository,
	postRepo repository.PostRepository,
) *SitemapService {
	return &SitemapService{
		productRepo:  productRepo,
		categoryRepo: categoryRepo,
		postRepo:     postRepo,
	}
}

const (
	sitemapCacheTTL    = 5 * time.Minute
	sitemapCachePrefix = "sitemap:xml:"
	sitemapMaxFetch    = 50000 // 单次拉取上限，避免极端数据量打爆内存
)

// Generate 生成 sitemap.xml（带缓存）
func (s *SitemapService) Generate(ctx context.Context, baseURL, localeURLMode string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("sitemap: baseURL is empty")
	}

	cacheKey := sitemapCachePrefix + baseURL + ":" + localeURLMode
	if cached, err := cache.GetString(ctx, cacheKey); err == nil && cached != "" {
		return cached, nil
	}

	entries, err := s.collectURLs(baseURL, localeURLMode)
	if err != nil {
		return "", err
	}

	xmlStr, err := renderSitemapXML(entries, localeURLMode)
	if err != nil {
		return "", err
	}

	_ = cache.SetString(ctx, cacheKey, xmlStr, sitemapCacheTTL)
	return xmlStr, nil
}

// GenerateRobots 生成 robots.txt 内容
func (s *SitemapService) GenerateRobots(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	var b strings.Builder
	b.WriteString("User-agent: *\n")
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

// urlEntry sitemap.xml 中的单条 URL
type urlEntry struct {
	XMLName        xml.Name    `xml:"url"`
	Loc            string      `xml:"loc"`
	LastMod        string      `xml:"lastmod,omitempty"`
	ChangeFreq     string      `xml:"changefreq,omitempty"`
	Priority       string      `xml:"priority,omitempty"`
	AlternateLinks []xhtmlLink `xml:"http://www.w3.org/1999/xhtml link,omitempty"`
}

type xhtmlLink struct {
	XMLName  xml.Name `xml:"http://www.w3.org/1999/xhtml link"`
	Rel      string   `xml:"rel,attr"`
	Hreflang string   `xml:"hreflang,attr"`
	Href     string   `xml:"href,attr"`
}

type urlSet struct {
	XMLName    xml.Name   `xml:"urlset"`
	Xmlns      string     `xml:"xmlns,attr"`
	XmlnsXhtml string     `xml:"xmlns:xhtml,attr,omitempty"`
	URLs       []urlEntry `xml:"url"`
}

func (s *SitemapService) collectURLs(baseURL, localeURLMode string) ([]urlEntry, error) {
	now := time.Now().UTC().Format("2006-01-02")
	entries := make([]urlEntry, 0, 64)

	isPrefix := localeURLMode == "prefix"

	// 辅助：创建带可选 hreflang 的条目
	makeEntry := func(path, lastMod, changeFreq, priority string) urlEntry {
		loc := baseURL + path
		if isPrefix {
			loc = baseURL + "/zh" + path
		}
		entry := urlEntry{
			Loc:        loc,
			LastMod:    lastMod,
			ChangeFreq: changeFreq,
			Priority:   priority,
		}
		if isPrefix {
			for _, lh := range localeHreflangMap {
				entry.AlternateLinks = append(entry.AlternateLinks, xhtmlLink{
					Rel:      "alternate",
					Hreflang: lh.Full,
					Href:     baseURL + "/" + lh.Short + path,
				})
			}
			entry.AlternateLinks = append(entry.AlternateLinks, xhtmlLink{
				Rel:      "alternate",
				Hreflang: "x-default",
				Href:     baseURL + "/zh" + path,
			})
		}
		return entry
	}

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
		entries = append(entries, makeEntry(p.Path, now, p.ChangeFreq, p.Priority))
	}

	// 2. 启用的分类
	categories, err := s.categoryRepo.ListActive()
	if err != nil {
		return nil, fmt.Errorf("sitemap: list categories: %w", err)
	}
	for _, cat := range categories {
		entries = append(entries, makeEntry(
			"/categories/"+url.PathEscape(cat.Slug),
			cat.CreatedAt.UTC().Format("2006-01-02"),
			"weekly",
			"0.7",
		))
	}

	// 3. 上架的商品
	products, _, err := s.productRepo.List(repository.ProductListFilter{
		Page:       1,
		PageSize:   sitemapMaxFetch,
		OnlyActive: true,
	})
	if err != nil {
		return nil, fmt.Errorf("sitemap: list products: %w", err)
	}
	for _, p := range products {
		entries = append(entries, makeEntry(
			"/products/"+url.PathEscape(p.Slug),
			p.UpdatedAt.UTC().Format("2006-01-02"),
			"daily",
			"0.8",
		))
	}

	// 4. 已发布的博客 / 公告
	posts, _, err := s.postRepo.List(repository.PostListFilter{
		Page:          1,
		PageSize:      sitemapMaxFetch,
		OnlyPublished: true,
		OrderBy:       "published_at DESC, created_at DESC",
	})
	if err != nil {
		return nil, fmt.Errorf("sitemap: list posts: %w", err)
	}
	for _, post := range posts {
		lastmod := post.CreatedAt
		if post.PublishedAt != nil {
			lastmod = *post.PublishedAt
		}
		entries = append(entries, makeEntry(
			"/blog/"+url.PathEscape(post.Slug),
			lastmod.UTC().Format("2006-01-02"),
			"monthly",
			"0.5",
		))
	}

	// 5. Tag 页面
	tags, err := s.productRepo.ListUniqueTags()
	if err != nil {
		return nil, fmt.Errorf("sitemap: list unique tags: %w", err)
	}
	for _, tag := range tags {
		entries = append(entries, makeEntry(
			"/tag/"+url.PathEscape(tag),
			now,
			"daily",
			"0.5",
		))
	}

	return entries, nil
}

func renderSitemapXML(entries []urlEntry, localeURLMode string) (string, error) {
	set := urlSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  entries,
	}
	if localeURLMode == "prefix" {
		set.XmlnsXhtml = "http://www.w3.org/1999/xhtml"
	}
	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return "", err
	}
	return xml.Header + string(body) + "\n", nil
}

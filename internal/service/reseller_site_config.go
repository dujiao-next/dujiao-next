package service

import (
	"context"
	"encoding/json"
	"net/mail"
	"net/url"
	"strings"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
)

type LocalizedTextInput map[string]string

type ResellerAnnouncementInput struct {
	Enabled bool               `json:"enabled"`
	Type    string             `json:"type"`
	Title   LocalizedTextInput `json:"title"`
	Content LocalizedTextInput `json:"content"`
}

type ResellerSupportInput struct {
	Telegram   string `json:"telegram"`
	WhatsApp   string `json:"whatsapp"`
	Email      string `json:"email"`
	SupportURL string `json:"support_url"`
}

type ResellerSEOInput struct {
	Title          LocalizedTextInput `json:"title"`
	Keywords       LocalizedTextInput `json:"keywords"`
	Description    LocalizedTextInput `json:"description"`
	DefaultOGImage string             `json:"default_og_image"`
}

type ResellerFooterLinkInput struct {
	Name LocalizedTextInput `json:"name"`
	URL  string             `json:"url"`
}

type ResellerNavConfigInput struct {
	Builtin     map[string]bool           `json:"builtin"`
	CustomItems []ResellerFooterLinkInput `json:"custom_items"`
}

type ResellerSiteConfigInput struct {
	SiteName     string                    `json:"site_name"`
	Logo         string                    `json:"logo"`
	Favicon      string                    `json:"favicon"`
	Announcement ResellerAnnouncementInput `json:"announcement"`
	Support      ResellerSupportInput      `json:"support"`
	SEO          ResellerSEOInput          `json:"seo"`
	FooterLinks  []ResellerFooterLinkInput `json:"footer_links"`
	NavConfig    ResellerNavConfigInput    `json:"nav_config"`
}

type ResellerSiteConfigService struct {
	repo repository.ResellerRepository
}

func NewResellerSiteConfigService(repo repository.ResellerRepository) *ResellerSiteConfigService {
	return &ResellerSiteConfigService{repo: repo}
}

func trimLimit(raw string, max int) string {
	value := strings.TrimSpace(raw)
	if max <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) > max {
		return string(runes[:max])
	}
	return value
}

// ResellerSiteConfigFieldError 在站点配置校验失败时携带具体字段，便于前端给出精确提示。
// 通过 Unwrap 兼容既有的 errors.Is(err, ErrResellerSiteConfigInvalid) 判断。
type ResellerSiteConfigFieldError struct {
	Field string
}

func (e *ResellerSiteConfigFieldError) Error() string {
	return "reseller site config field invalid: " + e.Field
}

func (e *ResellerSiteConfigFieldError) Unwrap() error { return ErrResellerSiteConfigInvalid }

func newResellerFieldError(field string) error {
	return &ResellerSiteConfigFieldError{Field: field}
}

func normalizeResellerLocalizedText(raw LocalizedTextInput, max int) models.JSON {
	out := models.JSON{}
	for _, lang := range []string{"zh-CN", "zh-TW", "en-US"} {
		out[lang] = trimLimit(raw[lang], max)
	}
	return out
}

func validateHTTPOrUploadPath(raw string) (string, error) {
	value := trimLimit(raw, 500)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "/uploads/") {
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", ErrResellerSiteConfigInvalid
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		return value, nil
	}
	return "", ErrResellerSiteConfigInvalid
}

func validateSupportURL(raw string) (string, error) {
	value := trimLimit(raw, 500)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "tg://") {
		return value, nil
	}
	if strings.HasPrefix(value, "mailto:") {
		addr := strings.TrimPrefix(value, "mailto:")
		if _, err := mail.ParseAddress(addr); err != nil {
			return "", ErrResellerSiteConfigInvalid
		}
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" {
		return "", ErrResellerSiteConfigInvalid
	}
	return value, nil
}

func normalizeResellerSupport(input ResellerSupportInput) (models.JSON, error) {
	telegram := trimLimit(input.Telegram, 500)
	if telegram != "" && !strings.HasPrefix(telegram, "https://telegram.me/") && !strings.HasPrefix(telegram, "https://t.me/") && !strings.HasPrefix(telegram, "tg://") {
		return nil, newResellerFieldError("support_telegram")
	}
	whatsApp := trimLimit(input.WhatsApp, 500)
	if whatsApp != "" && !strings.HasPrefix(whatsApp, "https://wa.me/") && !strings.HasPrefix(whatsApp, "https://api.whatsapp.com/") {
		return nil, newResellerFieldError("support_whatsapp")
	}
	email := trimLimit(input.Email, 320)
	if strings.HasPrefix(email, "mailto:") {
		email = strings.TrimPrefix(email, "mailto:")
	}
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return nil, newResellerFieldError("support_email")
		}
	}
	supportURL, err := validateSupportURL(input.SupportURL)
	if err != nil {
		return nil, newResellerFieldError("support_url")
	}
	return models.JSON{"telegram": telegram, "whatsapp": whatsApp, "email": email, "support_url": supportURL}, nil
}

func normalizeResellerAnnouncement(input ResellerAnnouncementInput) models.JSON {
	typ := trimLimit(input.Type, 32)
	if typ != "info" && typ != "success" && typ != "warning" {
		typ = "info"
	}
	return models.JSON{
		"enabled": input.Enabled,
		"type":    typ,
		"title":   normalizeResellerLocalizedText(input.Title, 120),
		// 公告内容为富文本 HTML（前端 TipTap），含标签开销，限额需高于纯文本；
		// 按 rune 截断仍可能切断标签，但前端渲染统一经 DOMPurify 解析补全，不会破坏页面。
		"content": normalizeResellerLocalizedText(input.Content, 4000),
	}
}

func normalizeResellerSEO(input ResellerSEOInput) (models.JSON, error) {
	image, err := validateHTTPOrUploadPath(input.DefaultOGImage)
	if err != nil {
		return nil, newResellerFieldError("image")
	}
	return models.JSON{
		"title":            normalizeResellerLocalizedText(input.Title, 120),
		"keywords":         normalizeResellerLocalizedText(input.Keywords, 200),
		"description":      normalizeResellerLocalizedText(input.Description, 300),
		"default_og_image": image,
	}, nil
}

func normalizeResellerFooterLinks(input []ResellerFooterLinkInput) (models.JSON, error) {
	items := make([]models.JSON, 0, min(len(input), 10))
	for i, item := range input {
		if i >= 10 {
			break
		}
		urlValue, err := validateSupportURL(item.URL)
		if err != nil {
			return nil, newResellerFieldError("link")
		}
		if urlValue == "" {
			continue
		}
		items = append(items, models.JSON{
			"name": normalizeResellerLocalizedText(item.Name, 80),
			"url":  urlValue,
		})
	}
	return models.JSON{"items": items}, nil
}

func normalizeResellerNavConfig(input ResellerNavConfigInput) (models.JSON, error) {
	builtin := models.JSON{"blog": true, "notice": true, "about": true}
	for _, key := range []string{"blog", "notice", "about"} {
		if value, ok := input.Builtin[key]; ok {
			builtin[key] = value
		}
	}
	custom, err := normalizeResellerFooterLinks(input.CustomItems)
	if err != nil {
		return nil, err
	}
	return models.JSON{"builtin": builtin, "custom_items": custom["items"]}, nil
}

func (s *ResellerSiteConfigService) buildModel(resellerID uint, input ResellerSiteConfigInput) (*models.ResellerSiteConfig, error) {
	logo, err := validateHTTPOrUploadPath(input.Logo)
	if err != nil {
		return nil, newResellerFieldError("image")
	}
	favicon, err := validateHTTPOrUploadPath(input.Favicon)
	if err != nil {
		return nil, newResellerFieldError("image")
	}
	support, err := normalizeResellerSupport(input.Support)
	if err != nil {
		return nil, err
	}
	seo, err := normalizeResellerSEO(input.SEO)
	if err != nil {
		return nil, err
	}
	footerLinks, err := normalizeResellerFooterLinks(input.FooterLinks)
	if err != nil {
		return nil, err
	}
	navConfig, err := normalizeResellerNavConfig(input.NavConfig)
	if err != nil {
		return nil, err
	}
	return &models.ResellerSiteConfig{
		ResellerID:       resellerID,
		SiteName:         trimLimit(input.SiteName, 120),
		Logo:             logo,
		Favicon:          favicon,
		AnnouncementJSON: normalizeResellerAnnouncement(input.Announcement),
		SupportJSON:      support,
		SEOJSON:          seo,
		FooterLinksJSON:  footerLinks,
		NavConfigJSON:    navConfig,
		ThemeJSON:        models.JSON{},
	}, nil
}

func (s *ResellerSiteConfigService) GetUserSiteConfig(userID uint) (*models.ResellerProfile, *models.ResellerSiteConfig, bool, error) {
	if s == nil || s.repo == nil || userID == 0 {
		return nil, nil, false, ErrNotFound
	}
	profile, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return nil, nil, false, err
	}
	if profile == nil {
		return nil, nil, false, ErrResellerNotOpened
	}
	canEdit := profile.Status == models.ResellerProfileStatusActive
	row, err := s.repo.GetSiteConfigByResellerID(profile.ID)
	if err != nil {
		return nil, nil, canEdit, err
	}
	return profile, row, canEdit, nil
}

func (s *ResellerSiteConfigService) UpdateUserSiteConfig(ctx context.Context, userID uint, input ResellerSiteConfigInput) (*models.ResellerSiteConfig, error) {
	profile, _, canEdit, err := s.GetUserSiteConfig(userID)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, ErrResellerProfileInactive
	}
	row, err := s.buildModel(profile.ID, input)
	if err != nil {
		return nil, err
	}
	saved, err := s.repo.UpsertSiteConfig(*row)
	if err != nil {
		return nil, err
	}
	_ = cache.Del(ctx, cache.PublicConfigCacheKey(&profile.ID))
	return saved, nil
}

func (s *ResellerSiteConfigService) UpdateAdminSiteConfig(ctx context.Context, resellerID uint, input ResellerSiteConfigInput) (*models.ResellerSiteConfig, error) {
	if resellerID == 0 {
		return nil, ErrNotFound
	}
	profile, err := s.repo.GetProfileByID(resellerID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, ErrNotFound
	}
	row, err := s.buildModel(resellerID, input)
	if err != nil {
		return nil, err
	}
	saved, err := s.repo.UpsertSiteConfig(*row)
	if err != nil {
		return nil, err
	}
	_ = cache.Del(ctx, cache.PublicConfigCacheKey(&resellerID))
	return saved, nil
}

func (s *ResellerSiteConfigService) ResetAdminSiteConfig(ctx context.Context, resellerID uint) error {
	if resellerID == 0 {
		return ErrNotFound
	}
	profile, err := s.repo.GetProfileByID(resellerID)
	if err != nil {
		return err
	}
	if profile == nil {
		return ErrNotFound
	}
	if err := s.repo.DeleteSiteConfigByResellerID(resellerID); err != nil {
		return err
	}
	_ = cache.Del(ctx, cache.PublicConfigCacheKey(&resellerID))
	return nil
}

func (s *ResellerSiteConfigService) ApplyPublicConfigOverlay(ctx context.Context, tenant TenantContext, base map[string]interface{}) (map[string]interface{}, error) {
	out := clonePublicConfigMap(base)
	if tenant.ResellerID == nil || tenant.IsMain || tenant.Unavailable {
		out["tenant"] = map[string]interface{}{"mode": "main", "host": tenant.Host}
		return out, nil
	}
	resellerID := *tenant.ResellerID
	out["tenant"] = map[string]interface{}{
		"mode":           "reseller",
		"host":           tenant.Host,
		"primary_domain": tenant.PrimaryDomain,
	}
	if s == nil || s.repo == nil {
		return out, nil
	}
	profile, err := s.repo.GetProfileByID(resellerID)
	if err != nil {
		return nil, err
	}
	if profile == nil || profile.Status != models.ResellerProfileStatusActive {
		return out, nil
	}
	cfg, err := s.repo.GetSiteConfigByResellerID(resellerID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return out, nil
	}
	applyResellerSiteConfigToPublicConfig(out, cfg)
	return out, nil
}

func clonePublicConfigMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return map[string]interface{}{}
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return map[string]interface{}{}
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]interface{}{}
	}
	return out
}

func footerItemsFromEnvelope(raw models.JSON) []interface{} {
	if raw == nil {
		return make([]interface{}, 0)
	}
	if items, ok := raw["items"].([]interface{}); ok {
		return items
	}
	if typed, ok := raw["items"].([]models.JSON); ok {
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	}
	return make([]interface{}, 0)
}

// resellerAnnouncementLocalizedMap 兼容两种载体：DB 反序列化后的 map[string]interface{}
// 与 Upsert 后内存对象里的 models.JSON。
func resellerAnnouncementLocalizedMap(raw interface{}) map[string]interface{} {
	switch typed := raw.(type) {
	case map[string]interface{}:
		return typed
	case models.JSON:
		return map[string]interface{}(typed)
	default:
		return map[string]interface{}{}
	}
}

// applyResellerAnnouncementToPublicConfig 将分销站公告以主站同构的
// {type,title,content,version} 写入 public config；未启用或内容为空时移除该字段。
func applyResellerAnnouncementToPublicConfig(out map[string]interface{}, raw models.JSON) {
	delete(out, "announcement")
	if !parseSettingBool(raw["enabled"]) {
		return
	}
	content := resellerAnnouncementLocalizedMap(raw["content"])
	if !hasHomeAnnouncementContent(content) {
		return
	}
	title := resellerAnnouncementLocalizedMap(raw["title"])
	annType, _ := raw["type"].(string)
	if annType == "" {
		annType = "info"
	}
	out["announcement"] = map[string]interface{}{
		"type":    annType,
		"title":   title,
		"content": content,
		"version": homeAnnouncementVersion(annType, title, content),
	}
}

func applyResellerSiteConfigToPublicConfig(out map[string]interface{}, cfg *models.ResellerSiteConfig) {
	if cfg == nil {
		return
	}
	brand, _ := out["brand"].(map[string]interface{})
	if brand == nil {
		brand = map[string]interface{}{}
	}
	if cfg.SiteName != "" {
		brand["site_name"] = cfg.SiteName
	}
	if cfg.Logo != "" {
		brand["site_logo"] = cfg.Logo
	}
	if cfg.Favicon != "" {
		brand["site_icon"] = cfg.Favicon
	}
	out["brand"] = brand

	contact, _ := out["contact"].(map[string]interface{})
	if contact == nil {
		contact = map[string]interface{}{}
	}
	for _, key := range []string{"telegram", "whatsapp", "email", "support_url"} {
		if value, ok := cfg.SupportJSON[key].(string); ok && strings.TrimSpace(value) != "" {
			contact[key] = value
		}
	}
	out["contact"] = contact

	if len(cfg.SEOJSON) > 0 {
		out["seo"] = cfg.SEOJSON
	}
	// A saved reseller config intentionally owns announcement and navigation:
	// a disabled/empty announcement removes the field so main-site content never
	// leaks into a white-label site; an enabled one is emitted in the same
	// {type,title,content,version} shape the storefront expects (see GetActiveHomeAnnouncement).
	applyResellerAnnouncementToPublicConfig(out, cfg.AnnouncementJSON)
	if len(cfg.FooterLinksJSON) > 0 {
		out["footer_links"] = footerItemsFromEnvelope(cfg.FooterLinksJSON)
	}
	if len(cfg.NavConfigJSON) > 0 {
		out["nav_config"] = cfg.NavConfigJSON
	}
}

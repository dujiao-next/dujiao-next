package application

import (
	"context"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/content/contract"
	"github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// BannerInput 描述 Banner 创建和更新所需字段。
type BannerInput struct {
	Name         string
	Position     string
	TitleJSON    map[string]interface{}
	SubtitleJSON map[string]interface{}
	CTATextJSON  map[string]interface{}
	BuyTextJSON  map[string]interface{}
	BuyLink      string
	Tone         string
	Image        string
	MobileImage  string
	LinkType     string
	LinkValue    string
	OpenInNewTab *bool
	IsActive     *bool
	StartAt      *time.Time
	EndAt        *time.Time
	SortOrder    int
}

// AdminBannerQuery 描述后台 Banner 列表查询。
type AdminBannerQuery struct {
	Position string
	Search   string
	IsActive *bool
	Page     int
	PageSize int
}

// PublicBannerQuery 描述公开 Banner 列表查询。
type PublicBannerQuery struct {
	Position string
	Limit    int
}

// BannerService 实现 Banner 用例。
type BannerService struct {
	store contract.BannerStore
	clock contract.Clock
}

// NewBannerService 创建 Banner 用例服务。
func NewBannerService(store contract.BannerStore, clock contract.Clock) *BannerService {
	if clock == nil {
		clock = SystemClock{}
	}
	return &BannerService{store: store, clock: clock}
}

// ListAdmin 获取后台 Banner 列表。
func (s *BannerService) ListAdmin(ctx context.Context, query AdminBannerQuery) ([]domain.Banner, int64, error) {
	return s.store.List(ctx, contract.BannerQuery{
		Page:     query.Page,
		PageSize: query.PageSize,
		Position: strings.TrimSpace(query.Position),
		Search:   strings.TrimSpace(query.Search),
		IsActive: query.IsActive,
	})
}

// ListPublic 获取公开 Banner 列表。
func (s *BannerService) ListPublic(ctx context.Context, query PublicBannerQuery) ([]domain.Banner, error) {
	return s.store.ListValidByPosition(ctx, normalizeBannerPosition(query.Position), query.Limit, s.clock.Now())
}

// GetByID 根据 ID 获取 Banner。
func (s *BannerService) GetByID(ctx context.Context, id string) (*domain.Banner, error) {
	banner, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if banner == nil {
		return nil, contract.ErrNotFound
	}
	return banner, nil
}

// Create 创建 Banner。
func (s *BannerService) Create(ctx context.Context, input BannerInput) (*domain.Banner, error) {
	banner, err := buildBannerEntity(input, nil)
	if err != nil {
		return nil, err
	}
	if err := s.store.Create(ctx, banner); err != nil {
		return nil, err
	}
	return banner, nil
}

// Update 更新 Banner。
func (s *BannerService) Update(ctx context.Context, id string, input BannerInput) (*domain.Banner, error) {
	existing, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, contract.ErrNotFound
	}

	banner, err := buildBannerEntity(input, existing)
	if err != nil {
		return nil, err
	}
	if err := s.store.Update(ctx, banner); err != nil {
		return nil, err
	}
	return banner, nil
}

// Delete 删除 Banner。
func (s *BannerService) Delete(ctx context.Context, id string) error {
	banner, err := s.store.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if banner == nil {
		return contract.ErrNotFound
	}
	return s.store.Delete(ctx, id)
}

func buildBannerEntity(input BannerInput, existing *domain.Banner) (*domain.Banner, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, contract.ErrInvalidBanner
	}
	image := strings.TrimSpace(input.Image)
	if image == "" {
		return nil, contract.ErrInvalidBanner
	}

	position := normalizeBannerPosition(input.Position)
	linkType := normalizeBannerLinkType(input.LinkType)
	if linkType == "" {
		return nil, contract.ErrInvalidBanner
	}
	if input.StartAt != nil && input.EndAt != nil && input.EndAt.Before(*input.StartAt) {
		return nil, contract.ErrInvalidBanner
	}

	linkValue := strings.TrimSpace(input.LinkValue)
	if linkType == constants.BannerLinkTypeNone {
		linkValue = ""
	}
	if linkType != constants.BannerLinkTypeNone && linkValue == "" {
		return nil, contract.ErrInvalidBanner
	}

	if existing == nil {
		banner := &domain.Banner{
			Name:         name,
			Position:     position,
			TitleJSON:    normalizeMultiLangJSON(input.TitleJSON),
			SubtitleJSON: normalizeMultiLangJSON(input.SubtitleJSON),
			CTATextJSON:  normalizeMultiLangJSON(input.CTATextJSON),
			BuyTextJSON:  normalizeMultiLangJSON(input.BuyTextJSON),
			BuyLink:      strings.TrimSpace(input.BuyLink),
			Tone:         normalizeBannerTone(input.Tone),
			Image:        image,
			MobileImage:  strings.TrimSpace(input.MobileImage),
			LinkType:     linkType,
			LinkValue:    linkValue,
			StartAt:      input.StartAt,
			EndAt:        input.EndAt,
			SortOrder:    input.SortOrder,
		}
		if input.OpenInNewTab != nil {
			banner.OpenInNewTab = *input.OpenInNewTab
		}
		if input.IsActive != nil {
			banner.IsActive = *input.IsActive
		} else {
			banner.IsActive = true
		}
		return banner, nil
	}

	existing.Name = name
	existing.Position = position
	existing.TitleJSON = normalizeMultiLangJSON(input.TitleJSON)
	existing.SubtitleJSON = normalizeMultiLangJSON(input.SubtitleJSON)
	existing.CTATextJSON = normalizeMultiLangJSON(input.CTATextJSON)
	existing.BuyTextJSON = normalizeMultiLangJSON(input.BuyTextJSON)
	existing.BuyLink = strings.TrimSpace(input.BuyLink)
	existing.Tone = normalizeBannerTone(input.Tone)
	existing.Image = image
	existing.MobileImage = strings.TrimSpace(input.MobileImage)
	existing.LinkType = linkType
	existing.LinkValue = linkValue
	existing.StartAt = input.StartAt
	existing.EndAt = input.EndAt
	existing.SortOrder = input.SortOrder
	if input.OpenInNewTab != nil {
		existing.OpenInNewTab = *input.OpenInNewTab
	}
	if input.IsActive != nil {
		existing.IsActive = *input.IsActive
	}
	return existing, nil
}

func normalizeBannerPosition(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" || value != constants.BannerPositionHomeHero {
		return constants.BannerPositionHomeHero
	}
	return value
}

func normalizeBannerLinkType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", constants.BannerLinkTypeNone:
		return constants.BannerLinkTypeNone
	case constants.BannerLinkTypeInternal:
		return constants.BannerLinkTypeInternal
	case constants.BannerLinkTypeExternal:
		return constants.BannerLinkTypeExternal
	default:
		return ""
	}
}

func normalizeMultiLangJSON(raw map[string]interface{}) jsonmap.JSON {
	result := jsonmap.JSON{}
	for _, key := range constants.SupportedLocales {
		value, exists := raw[key]
		if !exists {
			result[key] = ""
			continue
		}
		if text, ok := value.(string); ok {
			result[key] = strings.TrimSpace(text)
			continue
		}
		result[key] = ""
	}
	return result
}

// normalizeBannerTone 只允许 dark / light，其余视为自动（空）。
func normalizeBannerTone(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "dark":
		return "dark"
	case "light":
		return "light"
	default:
		return ""
	}
}

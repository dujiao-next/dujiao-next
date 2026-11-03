package presenter

import (
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// BannerResp 前台 Banner 响应
type BannerResp struct {
	ID           uint         `json:"id"`
	Position     string       `json:"position"`
	Title        jsonmap.JSON `json:"title"`
	Subtitle     jsonmap.JSON `json:"subtitle"`
	Image        string       `json:"image"`
	MobileImage  string       `json:"mobile_image,omitempty"`
	LinkType     string       `json:"link_type"`
	LinkValue    string       `json:"link_value,omitempty"`
	OpenInNewTab bool         `json:"open_in_new_tab"`
	CTAText      jsonmap.JSON `json:"cta_text"`
	BuyText      jsonmap.JSON `json:"buy_text"`
	BuyLink      string       `json:"buy_link"`
	Tone         string       `json:"tone"`
}

// NewBannerResp 从 Content 横幅领域对象构造响应。
func NewBannerResp(b *contentdomain.Banner) BannerResp {
	return BannerResp{
		ID:           b.ID,
		Position:     b.Position,
		Title:        b.TitleJSON,
		Subtitle:     b.SubtitleJSON,
		CTAText:      b.CTATextJSON,
		BuyText:      b.BuyTextJSON,
		BuyLink:      b.BuyLink,
		Tone:         b.Tone,
		Image:        b.Image,
		MobileImage:  b.MobileImage,
		LinkType:     b.LinkType,
		LinkValue:    b.LinkValue,
		OpenInNewTab: b.OpenInNewTab,
	}
	// 排除：Name(管理标识)、IsActive、StartAt、EndAt、SortOrder、CreatedAt、UpdatedAt
}

// NewBannerRespList 批量转换 Banner 列表
func NewBannerRespList(banners []contentdomain.Banner) []BannerResp {
	result := make([]BannerResp, 0, len(banners))
	for i := range banners {
		result = append(result, NewBannerResp(&banners[i]))
	}
	return result
}

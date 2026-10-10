package presenter

import (
	"time"

	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

// PostResp 文章/公告公共响应
type PostResp struct {
	ID              uint                 `json:"id"`
	Slug            string               `json:"slug"`
	Type            string               `json:"type"`
	Title           jsonmap.JSON         `json:"title"`
	Summary         jsonmap.JSON         `json:"summary"`
	Content         jsonmap.JSON         `json:"content"`
	Thumbnail       string               `json:"thumbnail,omitempty"`
	PublishedAt     *time.Time           `json:"published_at"`
	RelatedProducts []RelatedProductCard `json:"related_products,omitempty"`
	// CategorySlug / CategoryName 是文章所属分类的标识与名称，由调用方按需填充；
	// 分类已被删除或文章未挂分类时为空。名称随详情一起返回，前端无需再拉一次分类列表；
	// 对外以 slug 标识分类，因此不返回分类自增 id。
	CategorySlug string       `json:"category_slug,omitempty"`
	CategoryName jsonmap.JSON `json:"category_name,omitempty"`
}

// RelatedProductCard 文章详情底部展示的关联商品轻量卡片
type RelatedProductCard struct {
	ID          uint         `json:"id"`
	Slug        string       `json:"slug"`
	Title       jsonmap.JSON `json:"title"`
	PriceAmount money.Amount `json:"price_amount"`
	Image       string       `json:"image,omitempty"`
}

// NewRelatedProductCardList 将 Product 列表转为关联卡片列表
func NewRelatedProductCardList(products []contentcontract.RelatedProduct) []RelatedProductCard {
	cards := make([]RelatedProductCard, 0, len(products))
	for i := range products {
		p := &products[i]
		if !p.IsActive {
			continue
		}
		card := RelatedProductCard{
			ID:          p.ID,
			Slug:        p.Slug,
			Title:       p.Title,
			PriceAmount: p.PriceAmount,
		}
		if len(p.Images) > 0 {
			card.Image = p.Images[0]
		}
		cards = append(cards, card)
	}
	return cards
}

// NewPostResp 从 Content 文章领域对象构造响应。
func NewPostResp(p *contentdomain.Post) PostResp {
	return PostResp{
		ID:          p.ID,
		Slug:        p.Slug,
		Type:        p.Type,
		Title:       p.TitleJSON,
		Summary:     p.SummaryJSON,
		Content:     p.ContentJSON,
		Thumbnail:   p.Thumbnail,
		PublishedAt: p.PublishedAt,
	}
	// 排除：IsPublished(内部状态)、CreatedAt
}

// NewPostRespList 批量转换文章列表
func NewPostRespList(posts []contentdomain.Post) []PostResp {
	result := make([]PostResp, 0, len(posts))
	for i := range posts {
		result = append(result, NewPostResp(&posts[i]))
	}
	return result
}

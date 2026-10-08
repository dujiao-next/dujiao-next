package contenthttp

import (
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// CreatePostRequest 创建或更新文章的 HTTP 请求。
type CreatePostRequest struct {
	Slug        string                 `json:"slug" binding:"required"`
	Type        string                 `json:"type" binding:"required"`
	TitleJSON   map[string]interface{} `json:"title" binding:"required"`
	SummaryJSON map[string]interface{} `json:"summary"`
	ContentJSON map[string]interface{} `json:"content"`
	Thumbnail   string                 `json:"thumbnail"`
	IsPublished *bool                  `json:"is_published"`
	ProductIDs  *[]uint                `json:"product_ids"`
	CategoryID  *uint                  `json:"category_id"`
}

func (request CreatePostRequest) toInput() contentapp.CreatePostInput {
	return contentapp.CreatePostInput{
		Slug:        request.Slug,
		Type:        request.Type,
		TitleJSON:   request.TitleJSON,
		SummaryJSON: request.SummaryJSON,
		ContentJSON: request.ContentJSON,
		Thumbnail:   request.Thumbnail,
		IsPublished: request.IsPublished,
		ProductIDs:  request.ProductIDs,
		CategoryID:  request.CategoryID,
	}
}

// CreatePostCategoryRequest 创建文章分类的 HTTP 请求。
type CreatePostCategoryRequest struct {
	NameJSON  jsonmap.JSON `json:"name" binding:"required"`
	Slug      string       `json:"slug" binding:"required"`
	ParentID  *uint        `json:"parent_id"`
	SortOrder int          `json:"sort_order"`
	Icon      string       `json:"icon"`
}

// UpdatePostCategoryRequest 更新文章分类的 HTTP 请求。
type UpdatePostCategoryRequest struct {
	NameJSON  jsonmap.JSON `json:"name"`
	Slug      string       `json:"slug"`
	ParentID  *uint        `json:"parent_id"`
	SortOrder int          `json:"sort_order"`
	Icon      string       `json:"icon"`
}

func postCategoryInput(name jsonmap.JSON, slug string, parentID *uint, sortOrder int, icon string) contentapp.CreatePostCategoryInput {
	return contentapp.CreatePostCategoryInput{
		NameJSON:  name,
		Slug:      slug,
		ParentID:  parentID,
		SortOrder: sortOrder,
		Icon:      icon,
	}
}

// PatchPostCategoryStatusRequest 切换文章分类状态的 HTTP 请求。
type PatchPostCategoryStatusRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

// BatchPostStatusRequest 批量设置文章发布状态的 HTTP 请求。
type BatchPostStatusRequest struct {
	IDs         []uint `json:"ids" binding:"required,min=1"`
	IsPublished bool   `json:"is_published"`
}

// BatchPostCategoryRequest 批量调整文章分类的 HTTP 请求。
type BatchPostCategoryRequest struct {
	IDs        []uint `json:"ids" binding:"required,min=1"`
	CategoryID uint   `json:"category_id"`
}

// BatchDeletePostsRequest 批量删除文章的 HTTP 请求。
type BatchDeletePostsRequest struct {
	IDs []uint `json:"ids" binding:"required,min=1"`
}

// BannerUpsertRequest 创建或更新 Banner 的 HTTP 请求。
type BannerUpsertRequest struct {
	Name         string                 `json:"name" binding:"required"`
	Position     string                 `json:"position"`
	TitleJSON    map[string]interface{} `json:"title"`
	SubtitleJSON map[string]interface{} `json:"subtitle"`
	Image        string                 `json:"image" binding:"required"`
	MobileImage  string                 `json:"mobile_image"`
	LinkType     string                 `json:"link_type"`
	LinkValue    string                 `json:"link_value"`
	OpenInNewTab *bool                  `json:"open_in_new_tab"`
	IsActive     *bool                  `json:"is_active"`
	StartAt      string                 `json:"start_at"`
	EndAt        string                 `json:"end_at"`
	SortOrder    int                    `json:"sort_order"`
}

// buildBannerInputFromRequest 将 Banner HTTP 请求转换为 Content 输入。
func buildBannerInputFromRequest(request BannerUpsertRequest) (contentapp.BannerInput, error) {
	startAt, err := ginutil.ParseTimeNullable(request.StartAt)
	if err != nil {
		return contentapp.BannerInput{}, err
	}
	endAt, err := ginutil.ParseTimeNullable(request.EndAt)
	if err != nil {
		return contentapp.BannerInput{}, err
	}
	return contentapp.BannerInput{
		Name:         request.Name,
		Position:     request.Position,
		TitleJSON:    request.TitleJSON,
		SubtitleJSON: request.SubtitleJSON,
		Image:        request.Image,
		MobileImage:  request.MobileImage,
		LinkType:     request.LinkType,
		LinkValue:    request.LinkValue,
		OpenInNewTab: request.OpenInNewTab,
		IsActive:     request.IsActive,
		StartAt:      startAt,
		EndAt:        endAt,
		SortOrder:    request.SortOrder,
	}, nil
}

// BatchDeleteMediaRequest 批量删除素材的 HTTP 请求。
type BatchDeleteMediaRequest struct {
	IDs []uint `json:"ids" binding:"required,min=1"`
}

// UpdateMediaRequest 重命名素材的 HTTP 请求。
type UpdateMediaRequest struct {
	Name string `json:"name" binding:"required"`
}

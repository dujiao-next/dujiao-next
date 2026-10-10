package presenter

import (
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// PostCategoryResp 是文章分类的公开响应，同时用于分类列表接口与文章详情内嵌的分类。
// 不暴露软删除标记与内部时间字段。
type PostCategoryResp struct {
	ID        uint         `json:"id"`
	ParentID  uint         `json:"parent_id"`
	Slug      string       `json:"slug"`
	Name      jsonmap.JSON `json:"name"`
	Icon      string       `json:"icon"`
	SortOrder int          `json:"sort_order"`
}

// NewPostCategoryResp 从 Content 文章分类领域对象构造响应。
func NewPostCategoryResp(category *contentdomain.PostCategory) PostCategoryResp {
	return PostCategoryResp{
		ID:        category.ID,
		ParentID:  optionalUintOrZero(category.ParentID),
		Slug:      category.Slug,
		Name:      category.NameJSON,
		Icon:      category.Icon,
		SortOrder: category.SortOrder,
	}
}

// NewPostCategoryRespList 批量转换文章分类列表。
func NewPostCategoryRespList(categories []contentdomain.PostCategory) []PostCategoryResp {
	result := make([]PostCategoryResp, 0, len(categories))
	for index := range categories {
		result = append(result, NewPostCategoryResp(&categories[index]))
	}
	return result
}

func optionalUintOrZero(value *uint) uint {
	if value == nil {
		return 0
	}
	return *value
}

package public

import (
	"github.com/dujiao-next/internal/http/response"
	"github.com/gin-gonic/gin"
)

// PostCategoryDTO 公开接口返回的文章分类（对齐商品分类格式）
type PostCategoryDTO struct {
	ID        uint                   `json:"id"`
	ParentID  uint                   `json:"parent_id"`
	Slug      string                 `json:"slug"`
	Name      map[string]interface{} `json:"name"`
	Icon      string                 `json:"icon"`
	SortOrder int                    `json:"sort_order"`
}

// GetPostCategories 获取文章分类（公开，平铺列表，与 GET /categories 格式一致）
func (h *Handler) GetPostCategories(c *gin.Context) {
	cats, err := h.PostCategoryService.ListActive()
	if err != nil {
		response.Success(c, []PostCategoryDTO{})
		return
	}
	result := make([]PostCategoryDTO, 0, len(cats))
	for _, c := range cats {
		result = append(result, PostCategoryDTO{
			ID:        c.ID,
			ParentID:  orZero(c.ParentID),
			Slug:      c.Slug,
			Name:      c.NameJSON,
			Icon:      c.Icon,
			SortOrder: c.SortOrder,
		})
	}
	response.Success(c, result)
}

func orZero(p *uint) uint {
	if p == nil {
		return 0
	}
	return *p
}

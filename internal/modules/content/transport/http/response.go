package contenthttp

import (
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// AdminPostProductRef 是后台文章编辑回填使用的关联商品精简结构。
type AdminPostProductRef struct {
	ID    uint         `json:"id"`
	Slug  string       `json:"slug"`
	Title jsonmap.JSON `json:"title"`
	Image string       `json:"image,omitempty"`
}

func newAdminPostProductRefs(products []contentcontract.RelatedProduct) []AdminPostProductRef {
	refs := make([]AdminPostProductRef, 0, len(products))
	for index := range products {
		product := &products[index]
		ref := AdminPostProductRef{
			ID:    product.ID,
			Slug:  product.Slug,
			Title: product.Title,
		}
		if len(product.Images) > 0 {
			ref.Image = product.Images[0]
		}
		refs = append(refs, ref)
	}
	return refs
}

package productadapter

import (
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
)

// Adapter 把商品/SKU 仓储适配为专属价展示所需的摘要解析端口。
type Adapter struct {
	products productcontract.Repository
	skus     productcontract.SKURepository
}

// New 创建商品摘要解析适配器。
func New(products productcontract.Repository, skus productcontract.SKURepository) *Adapter {
	return &Adapter{products: products, skus: skus}
}

// ResolveProducts 按商品 ID 批量解析 slug、多语言标题与上架状态。
// 已删除的商品不会出现在结果中。
func (a *Adapter) ResolveProducts(ids []uint) (map[uint]memberlevelcontract.ProductSummary, error) {
	if a.products == nil || len(ids) == 0 {
		return nil, nil
	}
	items, err := a.products.ListByIDs(ids)
	if err != nil {
		return nil, err
	}
	result := make(map[uint]memberlevelcontract.ProductSummary, len(items))
	for _, item := range items {
		result[item.ID] = memberlevelcontract.ProductSummary{
			Slug:     item.Slug,
			Title:    item.TitleJSON,
			IsActive: item.IsActive,
		}
	}
	return result, nil
}

// ResolveSKUs 按 SKU ID 批量解析编码与规格值。
// 已删除的 SKU 不会出现在结果中。
func (a *Adapter) ResolveSKUs(ids []uint) (map[uint]memberlevelcontract.SKUSummary, error) {
	if a.skus == nil || len(ids) == 0 {
		return nil, nil
	}
	items, err := a.skus.ListByIDs(ids)
	if err != nil {
		return nil, err
	}
	result := make(map[uint]memberlevelcontract.SKUSummary, len(items))
	for _, item := range items {
		result[item.ID] = memberlevelcontract.SKUSummary{
			SKUCode:    item.SKUCode,
			SpecValues: item.SpecValuesJSON,
		}
	}
	return result, nil
}

var _ memberlevelcontract.ProductDirectory = (*Adapter)(nil)

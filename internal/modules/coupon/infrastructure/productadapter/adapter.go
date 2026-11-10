package productadapter

import (
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
)

// Adapter 把规格仓储适配为优惠券规格目录端口。
type Adapter struct {
	skus productcontract.SKURepository
}

// New 创建规格目录适配器。
func New(skus productcontract.SKURepository) *Adapter {
	return &Adapter{skus: skus}
}

// ResolveSKUProducts 按规格 ID 返回所属商品 ID；已删除的规格不会出现在结果中。
func (a *Adapter) ResolveSKUProducts(ids []uint) (map[uint]uint, error) {
	if a.skus == nil || len(ids) == 0 {
		return map[uint]uint{}, nil
	}
	items, err := a.skus.ListByIDs(ids)
	if err != nil {
		return nil, err
	}
	result := make(map[uint]uint, len(items))
	for _, item := range items {
		result[item.ID] = item.ProductID
	}
	return result, nil
}

var _ couponcontract.SKUDirectory = (*Adapter)(nil)

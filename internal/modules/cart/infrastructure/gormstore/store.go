package gormstore

import (
	"errors"

	"github.com/dujiao-next/internal/modules/cart/contract"
	"github.com/dujiao-next/internal/modules/cart/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"gorm.io/gorm"
)

// Store 是 Cart 的 GORM 持久化适配器。
type Store struct {
	db *gorm.DB
}

var _ contract.Repository = (*Store)(nil)

type itemRecord struct {
	domain.Item `gorm:"embedded"`
	Product     *productdomain.Product    `gorm:"foreignKey:ProductID"`
	SKU         *productdomain.ProductSKU `gorm:"foreignKey:SKUID"`
}

func (itemRecord) TableName() string { return "cart_items" }

// New 创建购物车持久化适配器。
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// WithTx 绑定事务
func (r *Store) WithTx(tx *gorm.DB) *Store {
	if tx == nil {
		return r
	}
	return &Store{db: tx}
}

// ListByUser 获取用户购物车项
func (r *Store) ListByUser(userID uint) ([]contract.StoredItem, error) {
	var records []itemRecord
	if err := r.db.
		Preload("Product", "deleted_at IS NULL").
		Preload("SKU", "deleted_at IS NULL").
		Where("cart_items.user_id = ? AND cart_items.deleted_at IS NULL", userID).
		Order("updated_at desc").
		Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]contract.StoredItem, 0, len(records))
	for index := range records {
		record := &records[index]
		items = append(items, contract.StoredItem{
			Item:    record.Item,
			Product: record.Product,
			SKU:     record.SKU,
		})
	}
	return items, nil
}

// Upsert 添加或更新购物车项
func (r *Store) Upsert(item *domain.Item) error {
	if item == nil {
		return nil
	}
	var existing domain.Item
	// 唯一索引不含 deleted_at，软删除的行仍占着键：必须连它一起查出来复活，否则移除后再加入会撞唯一约束
	err := r.db.Where("user_id = ? AND product_id = ? AND sku_id = ?", item.UserID, item.ProductID, item.SKUID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.Create(item).Error
	}
	if err != nil {
		return err
	}
	updates := map[string]interface{}{
		"sku_id":           item.SKUID,
		"quantity":         item.Quantity,
		"fulfillment_type": item.FulfillmentType,
		"updated_at":       item.UpdatedAt,
		"deleted_at":       nil,
	}
	return r.db.Model(&existing).Updates(updates).Error
}

// DeleteByProduct 删除指定商品的所有购物车项
func (r *Store) DeleteByProduct(productID uint) error {
	return r.softDelete(r.db.Where("product_id = ? AND deleted_at IS NULL", productID))
}

// DeleteByUserAndProduct 删除购物车项
func (r *Store) DeleteByUserAndProduct(userID, productID uint) error {
	return r.softDelete(r.db.Where("user_id = ? AND product_id = ? AND deleted_at IS NULL", userID, productID))
}

// DeleteByUserProductSKU 按用户+商品+SKU删除购物车项
func (r *Store) DeleteByUserProductSKU(userID, productID, skuID uint) error {
	if skuID == 0 {
		return r.DeleteByUserAndProduct(userID, productID)
	}
	return r.softDelete(r.db.Where("user_id = ? AND product_id = ? AND sku_id = ? AND deleted_at IS NULL", userID, productID, skuID))
}

// ClearByUser 清空购物车
func (r *Store) ClearByUser(userID uint) error {
	return r.softDelete(r.db.Where("user_id = ? AND deleted_at IS NULL", userID))
}

func (r *Store) softDelete(query *gorm.DB) error {
	return query.Model(&domain.Item{}).Update("deleted_at", r.db.NowFunc()).Error
}

package gormstore

import (
	"errors"
	"time"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"

	"gorm.io/gorm"
)

// UserPriceStore 持久化用户专属商品价。
type UserPriceStore struct {
	db *gorm.DB
}

func NewUserPriceStore(db *gorm.DB) *UserPriceStore {
	return &UserPriceStore{db: db}
}

func (r *UserPriceStore) GetByUserAndProductAndSKU(userID, productID, skuID uint) (*memberleveldomain.UserProductPrice, error) {
	var price memberleveldomain.UserProductPrice
	if err := r.db.Where("deleted_at IS NULL AND user_id = ? AND product_id = ? AND sku_id = ?", userID, productID, skuID).First(&price).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &price, nil
}

// ListByUser 分页获取用户的专属价。
func (r *UserPriceStore) ListByUser(userID uint, page, pageSize int) ([]memberleveldomain.UserProductPrice, int64, error) {
	query := r.db.Model(&memberleveldomain.UserProductPrice{}).Where("deleted_at IS NULL AND user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page > 0 && pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	var prices []memberleveldomain.UserProductPrice
	if err := query.Order("product_id asc, sku_id asc").Find(&prices).Error; err != nil {
		return nil, 0, err
	}
	return prices, total, nil
}

// BatchUpsert 在同一事务内创建或更新专属价；唯一键命中软删除记录时复活该记录。
func (r *UserPriceStore) BatchUpsert(prices []memberleveldomain.UserProductPrice) error {
	if len(prices) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range prices {
			var existing memberleveldomain.UserProductPrice
			err := tx.Where("user_id = ? AND product_id = ? AND sku_id = ?", item.UserID, item.ProductID, item.SKUID).
				First(&existing).Error
			switch {
			case err == nil:
				if err := tx.Model(&memberleveldomain.UserProductPrice{}).
					Where("id = ?", existing.ID).
					Updates(map[string]interface{}{
						"price_amount": item.PriceAmount,
						"deleted_at":   nil,
						"updated_at":   time.Now(),
					}).Error; err != nil {
					return err
				}
			case errors.Is(err, gorm.ErrRecordNotFound):
				item.ID = 0
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
			default:
				return err
			}
		}
		return nil
	})
}

func (r *UserPriceStore) Delete(id uint) error {
	return r.db.Model(&memberleveldomain.UserProductPrice{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now()).Error
}

var _ memberlevelcontract.UserPriceRepository = (*UserPriceStore)(nil)

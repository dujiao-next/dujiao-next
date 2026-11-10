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

// ListByUser 分页获取用户的专属价。不分页时不再单独跑一次 COUNT。
func (r *UserPriceStore) ListByUser(userID uint, page, pageSize int) ([]memberleveldomain.UserProductPrice, int64, error) {
	paginated := page > 0 && pageSize > 0
	query := r.db.Model(&memberleveldomain.UserProductPrice{}).Where("deleted_at IS NULL AND user_id = ?", userID)
	var total int64
	if paginated {
		if err := query.Count(&total).Error; err != nil {
			return nil, 0, err
		}
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	var prices []memberleveldomain.UserProductPrice
	if err := query.Order("product_id asc, sku_id asc").Find(&prices).Error; err != nil {
		return nil, 0, err
	}
	if !paginated {
		total = int64(len(prices))
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

func (r *UserPriceStore) DeleteByProduct(productID uint) error {
	return r.db.Model(&memberleveldomain.UserProductPrice{}).
		Where("product_id = ? AND deleted_at IS NULL", productID).
		Update("deleted_at", time.Now()).Error
}

// DeleteByProductInTx 是商品级联删除时的事务内适配入口，与会员等级价同款。
func (r *UserPriceStore) DeleteByProductInTx(tx *gorm.DB, productID uint) error {
	if tx == nil {
		return r.DeleteByProduct(productID)
	}
	return NewUserPriceStore(tx).DeleteByProduct(productID)
}

var _ memberlevelcontract.UserPriceRepository = (*UserPriceStore)(nil)

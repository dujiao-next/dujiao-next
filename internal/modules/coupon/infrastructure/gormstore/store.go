package gormstore

import (
	"errors"
	"fmt"
	"github.com/dujiao-next/internal/persistence/gormutil"
	"time"

	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store GORM 优惠券存储。
type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// WithTx 绑定事务
func (r *Store) WithTx(tx *gorm.DB) couponcontract.Repository {
	if tx == nil {
		return r
	}
	return &Store{db: tx}
}

// GetByID 根据ID获取优惠券
func (r *Store) GetByID(id uint) (*coupondomain.Coupon, error) {
	var coupon coupondomain.Coupon
	if err := r.db.Where("deleted_at IS NULL").First(&coupon, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &coupon, nil
}

// GetByIDForUpdate 在当前事务中锁定优惠券，用于原子校验并占用使用次数。
func (r *Store) GetByIDForUpdate(id uint) (*coupondomain.Coupon, error) {
	var coupon coupondomain.Coupon
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("deleted_at IS NULL").
		First(&coupon, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &coupon, nil
}

// GetByCode 根据优惠码获取优惠券
func (r *Store) GetByCode(code string) (*coupondomain.Coupon, error) {
	var coupon coupondomain.Coupon
	if err := r.db.Where("deleted_at IS NULL AND code = ?", code).First(&coupon).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &coupon, nil
}

// ListByIDs 批量获取优惠券
func (r *Store) ListByIDs(ids []uint) ([]coupondomain.Coupon, error) {
	if len(ids) == 0 {
		return []coupondomain.Coupon{}, nil
	}
	var coupons []coupondomain.Coupon
	if err := r.db.Where("deleted_at IS NULL AND id IN ?", ids).Find(&coupons).Error; err != nil {
		return nil, err
	}
	return coupons, nil
}

// Create 创建优惠券
func (r *Store) Create(coupon *coupondomain.Coupon) error {
	return r.db.Create(coupon).Error
}

// Update 更新优惠券
func (r *Store) Update(coupon *coupondomain.Coupon) error {
	return r.db.Save(coupon).Error
}

// Delete 删除优惠券
func (r *Store) Delete(id uint) error {
	return r.db.Model(&coupondomain.Coupon{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now()).Error
}

// List 获取优惠券列表
func (r *Store) List(filter couponcontract.ListFilter) ([]coupondomain.Coupon, int64, error) {
	var coupons []coupondomain.Coupon
	query := r.db.Model(&coupondomain.Coupon{}).Where("deleted_at IS NULL")

	if filter.ID > 0 {
		query = query.Where("id = ?", filter.ID)
	}
	if filter.Code != "" {
		query = query.Where("code = ?", filter.Code)
	}
	if filter.ScopeRefID > 0 {
		// scope_ref_ids 存储格式为 JSON 数组（例如 [1,2,3]），按边界匹配避免误命中（如 1 命中 11）。
		exact := fmt.Sprintf("[%d]", filter.ScopeRefID)
		prefix := fmt.Sprintf("[%d,%%", filter.ScopeRefID)
		middle := fmt.Sprintf("%%,%d,%%", filter.ScopeRefID)
		suffix := fmt.Sprintf("%%,%d]", filter.ScopeRefID)
		query = query.Where(
			"(scope_ref_ids = ? OR scope_ref_ids LIKE ? OR scope_ref_ids LIKE ? OR scope_ref_ids LIKE ?)",
			exact,
			prefix,
			middle,
			suffix,
		)
	}
	if filter.IsActive != nil {
		query = query.Where("is_active = ?", *filter.IsActive)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = gormutil.ApplyPagination(query, filter.Page, filter.PageSize)

	if err := query.Order("id desc").Find(&coupons).Error; err != nil {
		return nil, 0, err
	}
	return coupons, total, nil
}

// IncrementUsedCount 增加优惠券使用次数
func (r *Store) IncrementUsedCount(id uint, delta int) error {
	if delta == 0 {
		delta = 1
	}
	return r.db.Model(&coupondomain.Coupon{}).
		Where("id = ? AND deleted_at IS NULL", id).
		UpdateColumn("used_count", gorm.Expr("used_count + ?", delta)).Error
}

// DecrementUsedCount 减少优惠券使用次数
func (r *Store) DecrementUsedCount(id uint, delta int) error {
	if delta == 0 {
		delta = 1
	}
	if delta < 0 {
		delta = -delta
	}
	return r.db.Model(&coupondomain.Coupon{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Where("used_count >= ?", delta).
		UpdateColumn("used_count", gorm.Expr("used_count - ?", delta)).Error
}

var _ couponcontract.Repository = (*Store)(nil)

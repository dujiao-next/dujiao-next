package gormstore

import (
	"errors"
	"github.com/dujiao-next/internal/persistence/gormutil"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	promotioncontract "github.com/dujiao-next/internal/modules/promotion/contract"
	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"

	"gorm.io/gorm"
)

// Store 是 Promotion 领域的 GORM 存储实现。
type Store struct {
	db *gorm.DB
}

// New 创建 GORM 存储。
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// WithTx 返回绑定到指定事务的存储。
func (r *Store) WithTx(tx *gorm.DB) *Store {
	if tx == nil {
		return r
	}
	return &Store{db: tx}
}

// GetByID 根据ID获取活动价
func (r *Store) GetByID(id uint) (*promotiondomain.Promotion, error) {
	var promotion promotiondomain.Promotion
	if err := r.db.Where("deleted_at IS NULL").First(&promotion, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &promotion, nil
}

// GetActiveByProduct 获取商品有效活动价
func (r *Store) GetActiveByProduct(productID uint, now time.Time) (*promotiondomain.Promotion, error) {
	var promotion promotiondomain.Promotion
	query := r.db.Where("deleted_at IS NULL AND scope_type = ? AND scope_ref_id = ? AND is_active = ?", constants.ScopeTypeProduct, productID, true)
	query = query.Where("(starts_at IS NULL OR starts_at <= ?)", now)
	query = query.Where("(ends_at IS NULL OR ends_at >= ?)", now)
	if err := query.Order("id desc").First(&promotion).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &promotion, nil
}

// GetAllActiveByProduct 获取商品所有有效活动价（按 MinAmount 升序）
func (r *Store) GetAllActiveByProduct(productID uint, now time.Time) ([]promotiondomain.Promotion, error) {
	var promotions []promotiondomain.Promotion
	query := r.db.Where("deleted_at IS NULL AND scope_type = ? AND scope_ref_id = ? AND is_active = ?", constants.ScopeTypeProduct, productID, true)
	query = query.Where("(starts_at IS NULL OR starts_at <= ?)", now)
	query = query.Where("(ends_at IS NULL OR ends_at >= ?)", now)
	if err := query.Order("min_amount asc").Find(&promotions).Error; err != nil {
		return nil, err
	}
	return promotions, nil
}

// Create 创建活动价
func (r *Store) Create(promotion *promotiondomain.Promotion) error {
	return r.db.Create(promotion).Error
}

// Update 更新活动价
func (r *Store) Update(promotion *promotiondomain.Promotion) error {
	return r.db.Save(promotion).Error
}

// Delete 删除活动价
func (r *Store) Delete(id uint) error {
	return r.db.Model(&promotiondomain.Promotion{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now()).Error
}

// List 获取活动价列表
func (r *Store) List(filter promotioncontract.ListFilter) ([]promotiondomain.Promotion, int64, error) {
	var promotions []promotiondomain.Promotion
	query := r.db.Model(&promotiondomain.Promotion{}).Where("deleted_at IS NULL")

	if filter.ID != 0 {
		query = query.Where("id = ?", filter.ID)
	}
	if name := strings.TrimSpace(filter.Name); name != "" {
		// LOWER(...) LIKE LOWER(?) 保证 SQLite 与 PostgreSQL 大小写不敏感行为一致
		query = query.Where("LOWER(name) LIKE LOWER(?)", "%"+name+"%")
	}
	if filter.ScopeRefID != 0 {
		query = query.Where("scope_ref_id = ?", filter.ScopeRefID)
	}
	if filter.IsActive != nil {
		query = query.Where("is_active = ?", *filter.IsActive)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = gormutil.ApplyPagination(query, filter.Page, filter.PageSize)

	if err := query.Order("id desc").Find(&promotions).Error; err != nil {
		return nil, 0, err
	}
	return promotions, total, nil
}

var _ promotioncontract.Repository = (*Store)(nil)

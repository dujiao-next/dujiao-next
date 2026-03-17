package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProductRepository 商品数据访问接口
type ProductRepository interface {
	List(filter ProductListFilter) ([]models.Product, int64, error)
	GetBySlug(slug string, onlyActive bool) (*models.Product, error)
	GetByID(id string) (*models.Product, error)
	ListByIDs(ids []uint) ([]models.Product, error)
	Create(product *models.Product) error
	Update(product *models.Product) error
	Delete(id string) error
	CountBySlug(slug string, excludeID *string) (int64, error)
	ReserveManualStock(productID uint, quantity int) (int64, error)
	ReleaseManualStock(productID uint, quantity int) (int64, error)
	ConsumeManualStock(productID uint, quantity int) (int64, error)
	Transaction(fn func(tx *gorm.DB) error) error
	WithTx(tx *gorm.DB) ProductRepository
}

// GormProductRepository GORM 实现
type GormProductRepository struct {
	BaseRepository
}

// NewProductRepository 创建商品仓库
func NewProductRepository(db *gorm.DB) *GormProductRepository {
	return &GormProductRepository{BaseRepository: BaseRepository{db: db}}
}

// WithTx 绑定事务
func (r *GormProductRepository) WithTx(tx *gorm.DB) ProductRepository {
	if tx == nil {
		return r
	}
	return &GormProductRepository{BaseRepository: BaseRepository{db: tx}}
}

// List 商品列表
func (r *GormProductRepository) List(filter ProductListFilter) ([]models.Product, int64, error) {
	var products []models.Product

	query := r.db.Model(&models.Product{})
	if filter.WithCategory {
		query = query.Preload("Category")
	}
	if filter.OnlyActive {
		query = query.Where("is_active = ?", true)
		query = query.Preload("SKUs", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("sort_order DESC, id ASC")
		})
	} else {
		query = query.Preload("SKUs", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order DESC, id ASC")
		})
	}
	if len(filter.CategoryIDs) > 0 {
		query = query.Where("category_id IN ?", filter.CategoryIDs)
	} else if filter.CategoryID != "" {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if fulfillmentType := strings.TrimSpace(filter.FulfillmentType); fulfillmentType != "" {
		query = query.Where("fulfillment_type = ?", fulfillmentType)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + search + "%"
		condition, argCount := buildLocalizedLikeCondition(r.db, []string{"slug"}, []string{"title_json", "description_json"})
		query = query.Where(condition, repeatLikeArgs(like, argCount)...)
	}

	manualStockStatus := strings.ToLower(strings.TrimSpace(filter.ManualStockStatus))
	query = applyManualStockStatusFilter(query, manualStockStatus)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = applyPagination(query, filter.Page, filter.PageSize)

	if err := query.Order("sort_order DESC, created_at DESC").Find(&products).Error; err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

func applyManualStockStatusFilter(query *gorm.DB, status string) *gorm.DB {
	if query == nil {
		return query
	}

	const activeSKUExistsSQL = "EXISTS (SELECT 1 FROM product_skus ps WHERE ps.product_id = products.id AND ps.is_active = ? AND ps.deleted_at IS NULL)"
	const activeUnlimitedSKUExistsSQL = "EXISTS (SELECT 1 FROM product_skus ps WHERE ps.product_id = products.id AND ps.is_active = ? AND ps.deleted_at IS NULL AND ps.manual_stock_total = ?)"
	const activeSKURemainingSQL = "COALESCE((SELECT SUM(CASE WHEN ps.manual_stock_total > 0 THEN ps.manual_stock_total ELSE 0 END) FROM product_skus ps WHERE ps.product_id = products.id AND ps.is_active = ? AND ps.deleted_at IS NULL), 0)"

	switch status {
	case "low":
		condition := fmt.Sprintf(
			"fulfillment_type = ? AND (((%s) AND NOT (%s) AND (%s) <= 0) OR (NOT (%s) AND manual_stock_total = 0))",
			activeSKUExistsSQL,
			activeUnlimitedSKUExistsSQL,
			activeSKURemainingSQL,
			activeSKUExistsSQL,
		)
		return query.Where(
			condition,
			constants.FulfillmentTypeManual,
			true,
			true, constants.ManualStockUnlimited,
			true,
			true,
		)
	case "normal":
		condition := fmt.Sprintf(
			"fulfillment_type = ? AND (((%s) AND NOT (%s) AND (%s) > 0) OR (NOT (%s) AND manual_stock_total > 0))",
			activeSKUExistsSQL,
			activeUnlimitedSKUExistsSQL,
			activeSKURemainingSQL,
			activeSKUExistsSQL,
		)
		return query.Where(
			condition,
			constants.FulfillmentTypeManual,
			true,
			true, constants.ManualStockUnlimited,
			true,
			true,
		)
	case "unlimited":
		condition := fmt.Sprintf(
			"fulfillment_type = ? AND ((%s) OR (NOT (%s) AND manual_stock_total = ?))",
			activeUnlimitedSKUExistsSQL,
			activeSKUExistsSQL,
		)
		return query.Where(
			condition,
			constants.FulfillmentTypeManual,
			true, constants.ManualStockUnlimited,
			true,
			constants.ManualStockUnlimited,
		)
	default:
		return query
	}
}

// GetBySlug 根据 slug 获取商品
func (r *GormProductRepository) GetBySlug(slug string, onlyActive bool) (*models.Product, error) {
	query := r.db.Preload("Category").Where("slug = ?", slug)
	if onlyActive {
		query = query.Where("is_active = ?", true)
		query = query.Preload("SKUs", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("sort_order DESC, id ASC")
		})
	} else {
		query = query.Preload("SKUs", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order DESC, id ASC")
		})
	}

	var product models.Product
	if err := query.First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &product, nil
}

// GetByID 根据 ID 获取商品
func (r *GormProductRepository) GetByID(id string) (*models.Product, error) {
	var product models.Product
	if err := r.db.Preload("Category").
		Preload("SKUs", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("sort_order DESC, id ASC")
		}).
		First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &product, nil
}

// ListByIDs 批量获取商品
func (r *GormProductRepository) ListByIDs(ids []uint) ([]models.Product, error) {
	if len(ids) == 0 {
		return []models.Product{}, nil
	}
	var products []models.Product
	if err := r.db.Where("id IN ?", ids).Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

// Create 创建商品
func (r *GormProductRepository) Create(product *models.Product) error {
	return r.db.Create(product).Error
}

// Update 更新商品
func (r *GormProductRepository) Update(product *models.Product) error {
	return r.db.Omit(clause.Associations).Save(product).Error
}

// Delete 删除商品
func (r *GormProductRepository) Delete(id string) error {
	return r.db.Delete(&models.Product{}, id).Error
}

// CountBySlug 统计 slug 数量
func (r *GormProductRepository) CountBySlug(slug string, excludeID *string) (int64, error) {
	var count int64
	query := r.db.Model(&models.Product{}).Where("slug = ?", slug)
	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ReserveManualStock 预占手动库存
func (r *GormProductRepository) ReserveManualStock(productID uint, quantity int) (int64, error) {
	if productID == 0 || quantity <= 0 {
		return 0, errors.New("invalid manual stock reserve params")
	}
	result := r.db.Model(&models.Product{}).
		Where("id = ? AND manual_stock_total >= 0 AND manual_stock_total >= ?", productID, quantity).
		Updates(map[string]interface{}{
			"manual_stock_total":  gorm.Expr("manual_stock_total - ?", quantity),
			"manual_stock_locked": gorm.Expr("manual_stock_locked + ?", quantity),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// ReleaseManualStock 释放手动库存占用
func (r *GormProductRepository) ReleaseManualStock(productID uint, quantity int) (int64, error) {
	if productID == 0 || quantity <= 0 {
		return 0, errors.New("invalid manual stock release params")
	}
	result := r.db.Model(&models.Product{}).
		Where("id = ? AND manual_stock_total >= 0 AND manual_stock_locked >= ?", productID, quantity).
		Updates(map[string]interface{}{
			"manual_stock_total":  gorm.Expr("manual_stock_total + ?", quantity),
			"manual_stock_locked": gorm.Expr("manual_stock_locked - ?", quantity),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// ConsumeManualStock 消耗手动库存（支付成功后占用转已售）
func (r *GormProductRepository) ConsumeManualStock(productID uint, quantity int) (int64, error) {
	if productID == 0 || quantity <= 0 {
		return 0, errors.New("invalid manual stock consume params")
	}
	result := r.db.Model(&models.Product{}).
		Where("id = ? AND manual_stock_total >= ? AND (manual_stock_locked >= ? OR (manual_stock_locked < ? AND manual_stock_total >= (? - manual_stock_locked)))",
			productID, constants.ManualStockUnlimited+1, quantity, quantity, quantity).
		Updates(map[string]interface{}{
			// 兼容历史未预占订单：锁定不足时按短缺量扣减剩余库存。
			"manual_stock_total":  gorm.Expr("manual_stock_total - CASE WHEN manual_stock_locked >= ? THEN 0 ELSE ? - manual_stock_locked END", quantity, quantity),
			"manual_stock_locked": gorm.Expr("CASE WHEN manual_stock_locked >= ? THEN manual_stock_locked - ? ELSE 0 END", quantity, quantity),
			"manual_stock_sold":   gorm.Expr("manual_stock_sold + ?", quantity),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

package repository

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProductSKURepository 商品 SKU 数据访问接口
type ProductSKURepository interface {
	ListByProduct(productID uint, onlyActive bool) ([]models.ProductSKU, error)
	GetByID(id uint) (*models.ProductSKU, error)
	GetByIDForUpdate(id uint) (*models.ProductSKU, error)
	GetByProductAndCode(productID uint, skuCode string) (*models.ProductSKU, error)
	ListByIDs(ids []uint) ([]models.ProductSKU, error)
	Create(item *models.ProductSKU) error
	CreateBatch(items []models.ProductSKU) error
	Update(item *models.ProductSKU) error
	Delete(id uint) error
	DeleteByProduct(productID uint) error
	PurgeSoftDeletedByProductAndCode(productID uint, skuCode string) error
	ReserveManualStock(skuID uint, quantity int) (int64, error)
	ReleaseManualStock(skuID uint, quantity int) (int64, error)
	ConsumeManualStock(skuID uint, quantity int) (int64, error)
	WithTx(tx *gorm.DB) ProductSKURepository
}

// GormProductSKURepository GORM 实现
type GormProductSKURepository struct {
	db *gorm.DB
}

// NewProductSKURepository 创建 SKU 仓库
func NewProductSKURepository(db *gorm.DB) *GormProductSKURepository {
	return &GormProductSKURepository{db: db}
}

// WithTx 绑定事务
func (r *GormProductSKURepository) WithTx(tx *gorm.DB) ProductSKURepository {
	if tx == nil {
		return r
	}
	return &GormProductSKURepository{db: tx}
}

// ListByProduct 根据商品获取 SKU 列表
func (r *GormProductSKURepository) ListByProduct(productID uint, onlyActive bool) ([]models.ProductSKU, error) {
	if productID == 0 {
		return nil, errors.New("invalid product id")
	}
	query := r.db.Model(&models.ProductSKU{}).Where("product_id = ?", productID)
	if onlyActive {
		query = query.Where("is_active = ?", true)
	}
	var items []models.ProductSKU
	if err := query.Order("sort_order DESC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 根据 ID 获取 SKU
func (r *GormProductSKURepository) GetByID(id uint) (*models.ProductSKU, error) {
	if id == 0 {
		return nil, errors.New("invalid sku id")
	}
	var item models.ProductSKU
	if err := r.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// GetByIDForUpdate 在事务中锁定 SKU，供同一库存范围的写操作串行执行。
func (r *GormProductSKURepository) GetByIDForUpdate(id uint) (*models.ProductSKU, error) {
	if id == 0 {
		return nil, errors.New("invalid sku id")
	}
	var item models.ProductSKU
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// GetByProductAndCode 按商品和编码获取 SKU
func (r *GormProductSKURepository) GetByProductAndCode(productID uint, skuCode string) (*models.ProductSKU, error) {
	if productID == 0 {
		return nil, errors.New("invalid product id")
	}
	code := strings.TrimSpace(skuCode)
	if code == "" {
		return nil, errors.New("invalid sku code")
	}

	var item models.ProductSKU
	if err := r.db.Where("product_id = ? AND sku_code = ?", productID, code).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// ListByIDs 批量获取 SKU
func (r *GormProductSKURepository) ListByIDs(ids []uint) ([]models.ProductSKU, error) {
	if len(ids) == 0 {
		return []models.ProductSKU{}, nil
	}
	var items []models.ProductSKU
	if err := r.db.Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Create 创建 SKU
func (r *GormProductSKURepository) Create(item *models.ProductSKU) error {
	if item == nil {
		return errors.New("sku is nil")
	}
	return r.db.Create(item).Error
}

// CreateBatch 批量创建 SKU
func (r *GormProductSKURepository) CreateBatch(items []models.ProductSKU) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.Create(&items).Error
}

// Update 更新 SKU
func (r *GormProductSKURepository) Update(item *models.ProductSKU) error {
	if item == nil {
		return errors.New("sku is nil")
	}
	return r.db.Save(item).Error
}

// Delete 硬删除单个 SKU（绕过软删除，避免唯一索引冲突）
func (r *GormProductSKURepository) Delete(id uint) error {
	if id == 0 {
		return errors.New("invalid sku id")
	}
	return r.db.Unscoped().Delete(&models.ProductSKU{}, id).Error
}

// PurgeSoftDeletedByProductAndCode 清理指定商品下同 sku_code 的软删除残留记录
func (r *GormProductSKURepository) PurgeSoftDeletedByProductAndCode(productID uint, skuCode string) error {
	return r.db.Unscoped().
		Where("product_id = ? AND sku_code = ? AND deleted_at IS NOT NULL", productID, skuCode).
		Delete(&models.ProductSKU{}).Error
}

// DeleteByProduct 删除指定商品下的 SKU
func (r *GormProductSKURepository) DeleteByProduct(productID uint) error {
	if productID == 0 {
		return errors.New("invalid product id")
	}
	return r.db.Where("product_id = ?", productID).Delete(&models.ProductSKU{}).Error
}

// ReserveManualStock 预占手动库存
func (r *GormProductSKURepository) ReserveManualStock(skuID uint, quantity int) (int64, error) {
	if skuID == 0 || quantity <= 0 {
		return 0, errors.New("invalid manual stock reserve params")
	}
	result := r.db.Model(&models.ProductSKU{}).
		Where("id = ? AND manual_stock_total >= 0 AND manual_stock_total >= ?", skuID, quantity).
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
func (r *GormProductSKURepository) ReleaseManualStock(skuID uint, quantity int) (int64, error) {
	if skuID == 0 || quantity <= 0 {
		return 0, errors.New("invalid manual stock release params")
	}
	result := r.db.Model(&models.ProductSKU{}).
		Where("id = ? AND manual_stock_total >= 0 AND manual_stock_locked >= ?", skuID, quantity).
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
func (r *GormProductSKURepository) ConsumeManualStock(skuID uint, quantity int) (int64, error) {
	if skuID == 0 || quantity <= 0 {
		return 0, errors.New("invalid manual stock consume params")
	}
	result := r.db.Model(&models.ProductSKU{}).
		Where("id = ? AND manual_stock_total >= ? AND (manual_stock_locked >= ? OR (manual_stock_locked < ? AND manual_stock_total >= (? - manual_stock_locked)))",
			skuID, constants.ManualStockUnlimited+1, quantity, quantity, quantity).
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

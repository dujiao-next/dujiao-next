package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CardSecretListFilter 卡密列表筛选条件
type CardSecretListFilter struct {
	ProductID uint
	SKUID     uint
	BatchID   uint
	Status    string
	Secret    string
	BatchNo   string
	Page      int
	PageSize  int
}

// CardSecretBatchStatusCount 批次状态统计结果
type CardSecretBatchStatusCount struct {
	BatchID uint   `gorm:"column:batch_id"`
	Status  string `gorm:"column:status"`
	Total   int64  `gorm:"column:total"`
}

type CardSecretExportListFilter struct {
	ID        uint
	ProductID uint
	SKUID     uint
	BatchID   uint
	Page      int
	PageSize  int
}

// CardSecretRepository 卡密库存数据访问接口
type CardSecretRepository interface {
	CreateBatch(items []models.CardSecret) error
	List(filter CardSecretListFilter) ([]models.CardSecret, int64, error)
	ListIDs(filter CardSecretListFilter) ([]uint, error)
	ListByIDs(ids []uint) ([]models.CardSecret, error)
	ListIDsByBatchID(batchID uint) ([]uint, error)
	CountByBatchIDs(batchIDs []uint) ([]CardSecretBatchStatusCount, error)
	ListByOrderAndStatus(orderID uint, status string) ([]models.CardSecret, error)
	// ListAvailableByProduct 在事务中按 product_id + (可选)sku_id + status=available 列出
	// 最多 limit 条卡密,按 id 升序。skuID=0 表示不限制 SKU。
	ListAvailableByProduct(productID, skuID uint, limit int) ([]models.CardSecret, error)
	// ListAvailableByProductForUpdate 同 ListAvailableByProduct,但加 SELECT ... FOR UPDATE 行锁,
	// 用于卡密扣库存事务避免并发出库重复。
	ListAvailableByProductForUpdate(productID, skuID uint, limit int) ([]models.CardSecret, error)
	// ListAvailableByProductBatchForUpdate 按商品/SKU/批次锁定可用卡密。
	ListAvailableByProductBatchForUpdate(productID, skuID, batchID uint, limit int) ([]models.CardSecret, error)
	GetByID(id uint) (*models.CardSecret, error)
	Update(secret *models.CardSecret) error
	BatchUpdateStatus(ids []uint, status string, updatedAt time.Time) (int64, error)
	MarkExported(ids []uint, exportID uint, usedAt time.Time) (int64, error)
	BatchDeleteByIDs(ids []uint) (int64, error)
	CreateExport(record *models.CardSecretExport) error
	ListExports(filter CardSecretExportListFilter) ([]models.CardSecretExport, int64, error)
	CountByProduct(productID, skuID uint) (int64, int64, int64, error)
	CountAvailable(productID, skuID uint) (int64, error)
	CountAvailableByProductIDs(productIDs []uint) (map[uint]int64, error)
	CountReserved(productID, skuID uint) (int64, error)
	CountStockByProductIDs(productIDs []uint) ([]SKUStockCount, error)
	Reserve(ids []uint, orderID uint, reservedAt time.Time) (int64, error)
	ReleaseByOrder(orderID uint) (int64, error)
	MarkUsed(ids []uint, orderID uint, usedAt time.Time) (int64, error)
	DeleteByProduct(productID uint) error
	Transaction(fn func(tx *gorm.DB) error) error
	WithTx(tx *gorm.DB) *GormCardSecretRepository
}

// SKUStockCount 卡密库存统计结果
type SKUStockCount struct {
	ProductID uint   `gorm:"column:product_id"`
	SKUID     uint   `gorm:"column:sku_id"`
	Status    string `gorm:"column:status"`
	Total     int64  `gorm:"column:total"`
}

// GormCardSecretRepository GORM 实现
type GormCardSecretRepository struct {
	BaseRepository
}

// NewCardSecretRepository 创建卡密仓库
func NewCardSecretRepository(db *gorm.DB) *GormCardSecretRepository {
	return &GormCardSecretRepository{BaseRepository: BaseRepository{db: db}}
}

// WithTx 绑定事务
func (r *GormCardSecretRepository) WithTx(tx *gorm.DB) *GormCardSecretRepository {
	if tx == nil {
		return r
	}
	return &GormCardSecretRepository{BaseRepository: BaseRepository{db: tx}}
}

// CreateBatch 批量创建卡密
func (r *GormCardSecretRepository) CreateBatch(items []models.CardSecret) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.CreateInBatches(&items, 200).Error
}

func (r *GormCardSecretRepository) buildListQuery(filter CardSecretListFilter) *gorm.DB {
	query := r.db.Model(&models.CardSecret{}).Preload("Batch")
	if filter.ProductID > 0 {
		query = query.Where("card_secrets.product_id = ?", filter.ProductID)
	}
	if filter.SKUID > 0 {
		query = query.Where("card_secrets.sku_id = ?", filter.SKUID)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("card_secrets.status = ?", status)
	}
	if filter.BatchID > 0 {
		query = query.Where("card_secrets.batch_id = ?", filter.BatchID)
	}
	if secret := strings.TrimSpace(filter.Secret); secret != "" {
		query = query.Where("LOWER(card_secrets.secret) LIKE LOWER(?)", "%"+secret+"%")
	}
	if batchNo := strings.TrimSpace(filter.BatchNo); batchNo != "" {
		query = query.Joins("LEFT JOIN card_secret_batches ON card_secret_batches.id = card_secrets.batch_id").
			Where("LOWER(card_secret_batches.batch_no) LIKE LOWER(?)", "%"+batchNo+"%")
	}
	return query
}

// List 查询卡密列表
func (r *GormCardSecretRepository) List(filter CardSecretListFilter) ([]models.CardSecret, int64, error) {
	if filter.ProductID == 0 && filter.SKUID > 0 {
		return nil, 0, errors.New("invalid product id")
	}
	query := r.buildListQuery(filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = applyPagination(query, filter.Page, filter.PageSize)

	var items []models.CardSecret
	if err := query.Order("card_secrets.id asc").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListIDs 按筛选条件查询卡密 ID 列表
func (r *GormCardSecretRepository) ListIDs(filter CardSecretListFilter) ([]uint, error) {
	query := r.buildListQuery(filter)
	var ids []uint
	if err := query.Order("card_secrets.id asc").Pluck("card_secrets.id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// ListByIDs 按 ID 列表查询卡密
func (r *GormCardSecretRepository) ListByIDs(ids []uint) ([]models.CardSecret, error) {
	if len(ids) == 0 {
		return []models.CardSecret{}, nil
	}
	var items []models.CardSecret
	if err := r.db.Where("id IN ?", ids).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListIDsByBatchID 按批次查询卡密 ID
func (r *GormCardSecretRepository) ListIDsByBatchID(batchID uint) ([]uint, error) {
	if batchID == 0 {
		return []uint{}, nil
	}
	var ids []uint
	if err := r.db.Model(&models.CardSecret{}).Where("batch_id = ?", batchID).Order("id asc").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// CountByBatchIDs 统计多个批次下各状态的实时数量
func (r *GormCardSecretRepository) CountByBatchIDs(batchIDs []uint) ([]CardSecretBatchStatusCount, error) {
	if len(batchIDs) == 0 {
		return []CardSecretBatchStatusCount{}, nil
	}
	var rows []CardSecretBatchStatusCount
	if err := r.db.Model(&models.CardSecret{}).
		Select("batch_id, status, COUNT(*) as total").
		Where("batch_id IN ?", batchIDs).
		Group("batch_id, status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByOrderAndStatus 按订单与状态获取卡密
func (r *GormCardSecretRepository) ListByOrderAndStatus(orderID uint, status string) ([]models.CardSecret, error) {
	if orderID == 0 {
		return nil, errors.New("invalid order id")
	}
	query := r.db.Model(&models.CardSecret{}).Where("order_id = ?", orderID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var items []models.CardSecret
	if err := query.Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListAvailableByProduct 在事务中按 product_id + (可选)sku_id + status=available 列出
// 最多 limit 条卡密,按 id 升序。
func (r *GormCardSecretRepository) ListAvailableByProduct(productID, skuID uint, limit int) ([]models.CardSecret, error) {
	if productID == 0 || limit <= 0 {
		return nil, nil
	}
	query := r.db.Where("product_id = ? AND status = ?", productID, models.CardSecretStatusAvailable)
	if skuID > 0 {
		query = query.Where("sku_id = ?", skuID)
	}
	var rows []models.CardSecret
	if err := query.Order("id asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListAvailableByProductForUpdate 加 FOR UPDATE 行锁的版本(用于事务内扣库存)。
func (r *GormCardSecretRepository) ListAvailableByProductForUpdate(productID, skuID uint, limit int) ([]models.CardSecret, error) {
	if productID == 0 || limit <= 0 {
		return nil, nil
	}
	query := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("product_id = ? AND status = ?", productID, models.CardSecretStatusAvailable)
	if skuID > 0 {
		query = query.Where("sku_id = ?", skuID)
	}
	var rows []models.CardSecret
	if err := query.Order("id asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListAvailableByProductBatchForUpdate 按商品/SKU/批次锁定可用卡密。
func (r *GormCardSecretRepository) ListAvailableByProductBatchForUpdate(productID, skuID, batchID uint, limit int) ([]models.CardSecret, error) {
	if productID == 0 || limit <= 0 {
		return nil, nil
	}
	query := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("product_id = ? AND status = ?", productID, models.CardSecretStatusAvailable)
	if skuID > 0 {
		query = query.Where("sku_id = ?", skuID)
	}
	if batchID > 0 {
		query = query.Where("batch_id = ?", batchID)
	}
	var rows []models.CardSecret
	if err := query.Order("id asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetByID 根据 ID 获取卡密
func (r *GormCardSecretRepository) GetByID(id uint) (*models.CardSecret, error) {
	var secret models.CardSecret
	if err := r.db.First(&secret, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &secret, nil
}

// Update 更新卡密
func (r *GormCardSecretRepository) Update(secret *models.CardSecret) error {
	return r.db.Save(secret).Error
}

// BatchUpdateStatus 批量更新卡密状态
func (r *GormCardSecretRepository) BatchUpdateStatus(ids []uint, status string, updatedAt time.Time) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	result := r.db.Model(&models.CardSecret{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": updatedAt,
		})
	return result.RowsAffected, result.Error
}

// MarkExported 将卡密标记为已使用并关联导出记录。
func (r *GormCardSecretRepository) MarkExported(ids []uint, exportID uint, usedAt time.Time) (int64, error) {
	if len(ids) == 0 || exportID == 0 {
		return 0, nil
	}
	if usedAt.IsZero() {
		usedAt = time.Now()
	}
	result := r.db.Model(&models.CardSecret{}).
		Where("id IN ? AND status = ?", ids, models.CardSecretStatusAvailable).
		Updates(map[string]interface{}{
			"status":      models.CardSecretStatusUsed,
			"order_id":    nil,
			"export_id":   exportID,
			"reserved_at": nil,
			"used_at":     usedAt,
			"updated_at":  usedAt,
		})
	return result.RowsAffected, result.Error
}

func (r *GormCardSecretRepository) CreateExport(record *models.CardSecretExport) error {
	if record == nil {
		return errors.New("card secret export is nil")
	}
	return r.db.Create(record).Error
}

func (r *GormCardSecretRepository) ListExports(filter CardSecretExportListFilter) ([]models.CardSecretExport, int64, error) {
	query := r.db.Model(&models.CardSecretExport{})
	if filter.ID > 0 {
		query = query.Where("id = ?", filter.ID)
	}
	if filter.ProductID > 0 {
		query = query.Where("product_id = ?", filter.ProductID)
	}
	if filter.SKUID > 0 {
		query = query.Where("sku_id = ?", filter.SKUID)
	}
	if filter.BatchID > 0 {
		query = query.Where("batch_id = ?", filter.BatchID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = applyPagination(query, filter.Page, filter.PageSize)
	var rows []models.CardSecretExport
	if err := query.Order("id DESC").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// BatchDeleteByIDs 批量删除卡密
func (r *GormCardSecretRepository) BatchDeleteByIDs(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Where("id IN ?", ids).Delete(&models.CardSecret{})
	return result.RowsAffected, result.Error
}

// DeleteByProduct 删除指定商品下的所有卡密
func (r *GormCardSecretRepository) DeleteByProduct(productID uint) error {
	if productID == 0 {
		return errors.New("invalid product id")
	}
	return r.db.Where("product_id = ?", productID).Delete(&models.CardSecret{}).Error
}

// CountByProduct 统计库存数量（总/可用/已用）
func (r *GormCardSecretRepository) CountByProduct(productID, skuID uint) (int64, int64, int64, error) {
	if productID == 0 {
		return 0, 0, 0, errors.New("invalid product id")
	}

	buildQuery := func() *gorm.DB {
		query := r.db.Model(&models.CardSecret{}).Where("product_id = ?", productID)
		if skuID > 0 {
			query = query.Where("sku_id = ?", skuID)
		}
		return query
	}

	var total int64
	if err := buildQuery().Count(&total).Error; err != nil {
		return 0, 0, 0, err
	}

	var available int64
	if err := buildQuery().Where("status = ?", models.CardSecretStatusAvailable).
		Count(&available).Error; err != nil {
		return 0, 0, 0, err
	}

	var used int64
	if err := buildQuery().Where("status = ?", models.CardSecretStatusUsed).
		Count(&used).Error; err != nil {
		return 0, 0, 0, err
	}
	return total, available, used, nil
}

// CountAvailable 统计可用库存
func (r *GormCardSecretRepository) CountAvailable(productID, skuID uint) (int64, error) {
	if productID == 0 {
		return 0, errors.New("invalid product id")
	}
	query := r.db.Model(&models.CardSecret{}).
		Where("product_id = ? AND status = ?", productID, models.CardSecretStatusAvailable)
	if skuID > 0 {
		query = query.Where("sku_id = ?", skuID)
	}
	var count int64
	if err := query.
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountAvailableByProductIDs 批量统计可用库存
func (r *GormCardSecretRepository) CountAvailableByProductIDs(productIDs []uint) (map[uint]int64, error) {
	result := make(map[uint]int64)
	if len(productIDs) == 0 {
		return result, nil
	}

	type countRow struct {
		ProductID uint
		Total     int64
	}

	var rows []countRow
	if err := r.db.Model(&models.CardSecret{}).
		Select("product_id, COUNT(*) as total").
		Where("product_id IN ? AND status = ?", productIDs, models.CardSecretStatusAvailable).
		Group("product_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		result[row.ProductID] = row.Total
	}

	return result, nil
}

// CountStockByProductIDs 批量获取商品的 SKUs 的各状态卡密数量
func (r *GormCardSecretRepository) CountStockByProductIDs(productIDs []uint) ([]SKUStockCount, error) {
	if len(productIDs) == 0 {
		return []SKUStockCount{}, nil
	}

	var rows []SKUStockCount
	if err := r.db.Model(&models.CardSecret{}).
		Select("product_id, sku_id, status, COUNT(*) as total").
		Where("product_id IN ?", productIDs).
		Group("product_id, sku_id, status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

// CountReserved 统计占用库存
func (r *GormCardSecretRepository) CountReserved(productID, skuID uint) (int64, error) {
	if productID == 0 {
		return 0, errors.New("invalid product id")
	}
	query := r.db.Model(&models.CardSecret{}).
		Where("product_id = ? AND status = ?", productID, models.CardSecretStatusReserved)
	if skuID > 0 {
		query = query.Where("sku_id = ?", skuID)
	}
	var count int64
	if err := query.
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Reserve 占用卡密库存
func (r *GormCardSecretRepository) Reserve(ids []uint, orderID uint, reservedAt time.Time) (int64, error) {
	if len(ids) == 0 || orderID == 0 {
		return 0, nil
	}
	result := r.db.Model(&models.CardSecret{}).
		Where("id IN ? AND status = ?", ids, models.CardSecretStatusAvailable).
		Updates(map[string]interface{}{
			"status":      models.CardSecretStatusReserved,
			"order_id":    orderID,
			"reserved_at": reservedAt,
			"updated_at":  reservedAt,
		})
	return result.RowsAffected, result.Error
}

// ReleaseByOrder 释放占用库存
func (r *GormCardSecretRepository) ReleaseByOrder(orderID uint) (int64, error) {
	if orderID == 0 {
		return 0, nil
	}
	now := time.Now()
	result := r.db.Model(&models.CardSecret{}).
		Where("order_id = ? AND status = ?", orderID, models.CardSecretStatusReserved).
		Updates(map[string]interface{}{
			"status":      models.CardSecretStatusAvailable,
			"order_id":    nil,
			"reserved_at": nil,
			"updated_at":  now,
		})
	return result.RowsAffected, result.Error
}

// MarkUsed 标记卡密已使用
func (r *GormCardSecretRepository) MarkUsed(ids []uint, orderID uint, usedAt time.Time) (int64, error) {
	if len(ids) == 0 || orderID == 0 {
		return 0, nil
	}
	result := r.db.Model(&models.CardSecret{}).
		Where("id IN ? AND status IN ? AND (order_id IS NULL OR order_id = ?)", ids, []string{models.CardSecretStatusAvailable, models.CardSecretStatusReserved}, orderID).
		Updates(map[string]interface{}{
			"status":      models.CardSecretStatusUsed,
			"order_id":    orderID,
			"used_at":     usedAt,
			"reserved_at": nil,
			"updated_at":  usedAt,
		})
	return result.RowsAffected, result.Error
}

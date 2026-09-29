package gormstore

import (
	"errors"
	"strings"
	"time"

	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store 是卡密库存端口的 GORM 实现。
type Store struct {
	db *gorm.DB
}

var (
	_ cardsecretcontract.Repository = (*Store)(nil)
	_ cardsecretcontract.UnitOfWork = (*Store)(nil)
)

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// BindTx 将库存端口绑定到调用方事务，不暴露具体 Store 类型。
func (r *Store) BindTx(tx *gorm.DB) cardsecretcontract.Repository {
	if tx == nil {
		return r
	}
	return &Store{db: tx}
}

// Transaction 为卡密与批次写入提供同一事务中的模块端口。
func (r *Store) Transaction(fn func(cardsecretcontract.Repository, cardsecretcontract.BatchRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(New(tx), NewBatch(tx))
	})
}

// CreateBatch 批量创建卡密
func (r *Store) CreateBatch(items []cardsecretdomain.Secret) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.CreateInBatches(&items, 200).Error
}

func (r *Store) buildListQuery(filter cardsecretcontract.ListFilter) *gorm.DB {
	query := r.db.Model(&cardsecretdomain.Secret{}).
		Where("card_secrets.deleted_at IS NULL").
		Preload("Batch", "deleted_at IS NULL")
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
			Where("card_secret_batches.deleted_at IS NULL AND LOWER(card_secret_batches.batch_no) LIKE LOWER(?)", "%"+batchNo+"%")
	}
	return query
}

// List 查询卡密列表
func (r *Store) List(filter cardsecretcontract.ListFilter) ([]cardsecretdomain.Secret, int64, error) {
	if filter.ProductID == 0 && filter.SKUID > 0 {
		return nil, 0, errors.New("invalid product id")
	}
	query := r.buildListQuery(filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.PageSize > 0 {
		page := filter.Page
		if page < 1 {
			page = 1
		}
		query = query.Offset((page - 1) * filter.PageSize).Limit(filter.PageSize)
	}

	var items []cardsecretdomain.Secret
	if err := query.Order("card_secrets.id asc").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListIDs 按筛选条件查询卡密 ID 列表
func (r *Store) ListIDs(filter cardsecretcontract.ListFilter) ([]uint, error) {
	query := r.buildListQuery(filter)
	var ids []uint
	if err := query.Order("card_secrets.id asc").Pluck("card_secrets.id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// ListByIDs 按 ID 列表查询卡密
func (r *Store) ListByIDs(ids []uint) ([]cardsecretdomain.Secret, error) {
	if len(ids) == 0 {
		return []cardsecretdomain.Secret{}, nil
	}
	var items []cardsecretdomain.Secret
	if err := r.db.Where("id IN ? AND deleted_at IS NULL", ids).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListIDsByBatchID 按批次查询卡密 ID
func (r *Store) ListIDsByBatchID(batchID uint) ([]uint, error) {
	if batchID == 0 {
		return []uint{}, nil
	}
	var ids []uint
	if err := r.db.Model(&cardsecretdomain.Secret{}).Where("batch_id = ? AND deleted_at IS NULL", batchID).Order("id asc").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// CountByBatchIDs 统计多个批次下各状态的实时数量
func (r *Store) CountByBatchIDs(batchIDs []uint) ([]cardsecretcontract.BatchStatusCount, error) {
	if len(batchIDs) == 0 {
		return []cardsecretcontract.BatchStatusCount{}, nil
	}
	var rows []cardsecretcontract.BatchStatusCount
	if err := r.db.Model(&cardsecretdomain.Secret{}).
		Select("batch_id, status, COUNT(*) as total").
		Where("batch_id IN ? AND deleted_at IS NULL", batchIDs).
		Group("batch_id, status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByOrderAndStatus 按订单与状态获取卡密
func (r *Store) ListByOrderAndStatus(orderID uint, status string) ([]cardsecretdomain.Secret, error) {
	if orderID == 0 {
		return nil, errors.New("invalid order id")
	}
	query := r.db.Model(&cardsecretdomain.Secret{}).Where("order_id = ? AND deleted_at IS NULL", orderID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var items []cardsecretdomain.Secret
	if err := query.Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListAvailableByProduct 在事务中按 product_id + (可选)sku_id + status=available 列出
// 最多 limit 条卡密,按 id 升序。
func (r *Store) ListAvailableByProduct(productID, skuID uint, limit int) ([]cardsecretdomain.Secret, error) {
	if productID == 0 || limit <= 0 {
		return nil, nil
	}
	query := r.db.Where("product_id = ? AND status = ? AND deleted_at IS NULL", productID, cardsecretdomain.StatusAvailable)
	if skuID > 0 {
		query = query.Where("sku_id = ?", skuID)
	}
	var rows []cardsecretdomain.Secret
	if err := query.Order("id asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListAvailableByProductForUpdate 加 FOR UPDATE 行锁的版本(用于事务内扣库存)。
func (r *Store) ListAvailableByProductForUpdate(productID, skuID uint, limit int) ([]cardsecretdomain.Secret, error) {
	if productID == 0 || limit <= 0 {
		return nil, nil
	}
	query := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("product_id = ? AND status = ? AND deleted_at IS NULL", productID, cardsecretdomain.StatusAvailable)
	if skuID > 0 {
		query = query.Where("sku_id = ?", skuID)
	}
	var rows []cardsecretdomain.Secret
	if err := query.Order("id asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// FindAvailableLoop 返回商品/SKU 下可用的循环卡密，没有时返回 nil。
// 循环卡密可重复发货，不参与占用和状态流转，因此无需加锁。
func (r *Store) FindAvailableLoop(productID, skuID uint) (*cardsecretdomain.Secret, error) {
	if productID == 0 {
		return nil, nil
	}
	query := r.db.Where("product_id = ? AND status = ? AND is_loop = ? AND deleted_at IS NULL", productID, cardsecretdomain.StatusAvailable, true)
	if skuID > 0 {
		query = query.Where("sku_id = ?", skuID)
	}
	var rows []cardsecretdomain.Secret
	if err := query.Order("id asc").Limit(1).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// ListAvailableByProductBatchForUpdate 按商品/SKU/批次锁定可用卡密。
func (r *Store) ListAvailableByProductBatchForUpdate(productID, skuID, batchID uint, limit int) ([]cardsecretdomain.Secret, error) {
	if productID == 0 || limit <= 0 {
		return nil, nil
	}
	query := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("product_id = ? AND status = ? AND deleted_at IS NULL", productID, cardsecretdomain.StatusAvailable)
	if skuID > 0 {
		query = query.Where("sku_id = ?", skuID)
	}
	if batchID > 0 {
		query = query.Where("batch_id = ?", batchID)
	}
	var rows []cardsecretdomain.Secret
	if err := query.Order("id asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetByID 根据 ID 获取卡密
func (r *Store) GetByID(id uint) (*cardsecretdomain.Secret, error) {
	var secret cardsecretdomain.Secret
	if err := r.db.Where("deleted_at IS NULL").First(&secret, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &secret, nil
}

// Update 更新卡密
func (r *Store) Update(secret *cardsecretdomain.Secret) error {
	return r.db.Save(secret).Error
}

// BatchUpdateStatus 批量更新卡密状态
func (r *Store) BatchUpdateStatus(ids []uint, status string, updatedAt time.Time) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	result := r.db.Model(&cardsecretdomain.Secret{}).
		Where("id IN ? AND deleted_at IS NULL", ids).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": updatedAt,
		})
	return result.RowsAffected, result.Error
}

// BatchDeleteByIDs 批量删除卡密
func (r *Store) BatchDeleteByIDs(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	now := time.Now()
	result := r.db.Model(&cardsecretdomain.Secret{}).
		Where("id IN ? AND deleted_at IS NULL", ids).
		Updates(map[string]interface{}{"deleted_at": now, "updated_at": now})
	return result.RowsAffected, result.Error
}

// DeleteByProduct 删除指定商品下的所有卡密
func (r *Store) DeleteByProduct(productID uint) error {
	if productID == 0 {
		return errors.New("invalid product id")
	}
	now := time.Now()
	return r.db.Model(&cardsecretdomain.Secret{}).
		Where("product_id = ? AND deleted_at IS NULL", productID).
		Updates(map[string]interface{}{"deleted_at": now, "updated_at": now}).Error
}

// CountByProduct 统计库存数量（总/可用/已用）
func (r *Store) CountByProduct(productID, skuID uint) (int64, int64, int64, error) {
	if productID == 0 {
		return 0, 0, 0, errors.New("invalid product id")
	}

	buildQuery := func() *gorm.DB {
		query := r.db.Model(&cardsecretdomain.Secret{}).Where("product_id = ? AND deleted_at IS NULL", productID)
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
	if err := buildQuery().Where("status = ?", cardsecretdomain.StatusAvailable).
		Count(&available).Error; err != nil {
		return 0, 0, 0, err
	}

	var used int64
	if err := buildQuery().Where("status = ?", cardsecretdomain.StatusUsed).
		Count(&used).Error; err != nil {
		return 0, 0, 0, err
	}
	return total, available, used, nil
}

// CountAvailable 统计可用库存
func (r *Store) CountAvailable(productID, skuID uint) (int64, error) {
	if productID == 0 {
		return 0, errors.New("invalid product id")
	}
	query := r.db.Model(&cardsecretdomain.Secret{}).
		Where("product_id = ? AND status = ? AND deleted_at IS NULL", productID, cardsecretdomain.StatusAvailable)
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
func (r *Store) CountAvailableByProductIDs(productIDs []uint) (map[uint]int64, error) {
	result := make(map[uint]int64)
	if len(productIDs) == 0 {
		return result, nil
	}

	type countRow struct {
		ProductID uint
		Total     int64
	}

	var rows []countRow
	if err := r.db.Model(&cardsecretdomain.Secret{}).
		Select("product_id, COUNT(*) as total").
		Where("product_id IN ? AND status = ? AND deleted_at IS NULL", productIDs, cardsecretdomain.StatusAvailable).
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
func (r *Store) CountStockByProductIDs(productIDs []uint) ([]cardsecretcontract.SKUStockCount, error) {
	if len(productIDs) == 0 {
		return []cardsecretcontract.SKUStockCount{}, nil
	}

	var rows []cardsecretcontract.SKUStockCount
	if err := r.db.Model(&cardsecretdomain.Secret{}).
		Select("product_id, sku_id, status, is_loop, COUNT(*) as total").
		Where("product_id IN ? AND deleted_at IS NULL", productIDs).
		Group("product_id, sku_id, status, is_loop").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

// CountReserved 统计占用库存
func (r *Store) CountReserved(productID, skuID uint) (int64, error) {
	if productID == 0 {
		return 0, errors.New("invalid product id")
	}
	query := r.db.Model(&cardsecretdomain.Secret{}).
		Where("product_id = ? AND status = ? AND deleted_at IS NULL", productID, cardsecretdomain.StatusReserved)
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
func (r *Store) Reserve(ids []uint, orderID uint, reservedAt time.Time) (int64, error) {
	if len(ids) == 0 || orderID == 0 {
		return 0, nil
	}
	result := r.db.Model(&cardsecretdomain.Secret{}).
		Where("id IN ? AND status = ? AND deleted_at IS NULL", ids, cardsecretdomain.StatusAvailable).
		Updates(map[string]interface{}{
			"status":      cardsecretdomain.StatusReserved,
			"order_id":    orderID,
			"reserved_at": reservedAt,
			"updated_at":  reservedAt,
		})
	return result.RowsAffected, result.Error
}

// ReleaseByOrder 释放占用库存
func (r *Store) ReleaseByOrder(orderID uint) (int64, error) {
	if orderID == 0 {
		return 0, nil
	}
	now := time.Now()
	result := r.db.Model(&cardsecretdomain.Secret{}).
		Where("order_id = ? AND status = ? AND deleted_at IS NULL", orderID, cardsecretdomain.StatusReserved).
		Updates(map[string]interface{}{
			"status":      cardsecretdomain.StatusAvailable,
			"order_id":    nil,
			"reserved_at": nil,
			"updated_at":  now,
		})
	return result.RowsAffected, result.Error
}

// MarkUsed 标记卡密已使用
func (r *Store) MarkUsed(ids []uint, orderID uint, usedAt time.Time) (int64, error) {
	if len(ids) == 0 || orderID == 0 {
		return 0, nil
	}
	result := r.db.Model(&cardsecretdomain.Secret{}).
		Where("id IN ? AND status IN ? AND (order_id IS NULL OR order_id = ?) AND deleted_at IS NULL", ids, []string{cardsecretdomain.StatusAvailable, cardsecretdomain.StatusReserved}, orderID).
		Updates(map[string]interface{}{
			"status":      cardsecretdomain.StatusUsed,
			"order_id":    orderID,
			"used_at":     usedAt,
			"reserved_at": nil,
			"updated_at":  usedAt,
		})
	return result.RowsAffected, result.Error
}

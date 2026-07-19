package repository

import (
	"errors"
	"time"

	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
)

// ProcurementOrderRepository 采购单数据访问接口
type ProcurementOrderRepository interface {
	GetByID(id uint) (*models.ProcurementOrder, error)
	GetByLocalOrderID(localOrderID uint) (*models.ProcurementOrder, error)
	GetByLocalOrderNo(localOrderNo string) (*models.ProcurementOrder, error)
	GetByUpstreamOrderID(connectionID, upstreamOrderID uint) (*models.ProcurementOrder, error)
	Create(order *models.ProcurementOrder) error
	Update(order *models.ProcurementOrder) error
	UpdateFields(id uint, updates map[string]interface{}) error
	UpdateStatus(id uint, status string, updates map[string]interface{}) error
	UpdateStatusIfCurrent(id uint, currentStatuses []string, status string, updates map[string]interface{}) (bool, error)
	List(filter ProcurementOrderListFilter) ([]models.ProcurementOrder, int64, error)
	StatsByStatus(filter ProcurementOrderListFilter) (map[string]int64, error)
	ListRetriable(now time.Time, limit int) ([]models.ProcurementOrder, error)
	ListByLocalOrderIDs(localOrderIDs []uint) ([]models.ProcurementOrder, error)
	ListByConnectionAndTimeRange(connectionID uint, start, end time.Time) ([]models.ProcurementOrder, error)
	Transaction(fn func(tx *gorm.DB) error) error
	WithTx(tx *gorm.DB) *GormProcurementOrderRepository
}

// ProcurementOrderListFilter 采购单列表筛选
type ProcurementOrderListFilter struct {
	ConnectionID    uint
	Status          string
	LocalOrderNo    string
	UpstreamOrderNo string
	CreatedFrom     *time.Time
	CreatedTo       *time.Time
	Pagination
}

// GormProcurementOrderRepository GORM 实现
type GormProcurementOrderRepository struct {
	db *gorm.DB
}

// NewProcurementOrderRepository 创建采购单仓库
func NewProcurementOrderRepository(db *gorm.DB) *GormProcurementOrderRepository {
	return &GormProcurementOrderRepository{db: db}
}

// WithTx 绑定事务
func (r *GormProcurementOrderRepository) WithTx(tx *gorm.DB) *GormProcurementOrderRepository {
	if tx == nil {
		return r
	}
	return &GormProcurementOrderRepository{db: tx}
}

// Transaction 执行事务
func (r *GormProcurementOrderRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// GetByID 根据 ID 获取
func (r *GormProcurementOrderRepository) GetByID(id uint) (*models.ProcurementOrder, error) {
	var order models.ProcurementOrder
	if err := r.db.Preload("Connection").Preload("LocalOrder").Preload("LocalOrder.Items").First(&order, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

// GetByLocalOrderID 根据本地订单 ID 获取
func (r *GormProcurementOrderRepository) GetByLocalOrderID(localOrderID uint) (*models.ProcurementOrder, error) {
	var order models.ProcurementOrder
	if err := r.db.Where("local_order_id = ?", localOrderID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

// GetByLocalOrderNo 根据本地订单号获取
func (r *GormProcurementOrderRepository) GetByLocalOrderNo(localOrderNo string) (*models.ProcurementOrder, error) {
	var order models.ProcurementOrder
	if err := r.db.Where("local_order_no = ?", localOrderNo).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

// GetByUpstreamOrderID 根据连接+上游订单 ID 获取
func (r *GormProcurementOrderRepository) GetByUpstreamOrderID(connectionID, upstreamOrderID uint) (*models.ProcurementOrder, error) {
	var order models.ProcurementOrder
	if err := r.db.Where("connection_id = ? AND upstream_order_id = ?", connectionID, upstreamOrderID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

// Create 创建采购单
func (r *GormProcurementOrderRepository) Create(order *models.ProcurementOrder) error {
	return r.db.Create(order).Error
}

// Update 更新采购单
func (r *GormProcurementOrderRepository) Update(order *models.ProcurementOrder) error {
	return r.db.Save(order).Error
}

// UpdateFields 更新采购单指定字段，不改变未显式传入的状态。
func (r *GormProcurementOrderRepository) UpdateFields(id uint, updates map[string]interface{}) error {
	if id == 0 || len(updates) == 0 {
		return nil
	}
	return r.db.Model(&models.ProcurementOrder{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateStatus 更新采购单状态
func (r *GormProcurementOrderRepository) UpdateStatus(id uint, status string, updates map[string]interface{}) error {
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["status"] = status
	return r.db.Model(&models.ProcurementOrder{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateStatusIfCurrent 仅当采购单仍处于指定状态时更新，防止异步回调后的状态被旧流程覆盖。
func (r *GormProcurementOrderRepository) UpdateStatusIfCurrent(id uint, currentStatuses []string, status string, updates map[string]interface{}) (bool, error) {
	if id == 0 || len(currentStatuses) == 0 {
		return false, nil
	}
	fields := make(map[string]interface{}, len(updates)+1)
	for key, value := range updates {
		fields[key] = value
	}
	fields["status"] = status
	result := r.db.Model(&models.ProcurementOrder{}).
		Where("id = ? AND status IN ?", id, currentStatuses).
		Updates(fields)
	return result.RowsAffected > 0, result.Error
}

// List 列表查询
func (r *GormProcurementOrderRepository) List(filter ProcurementOrderListFilter) ([]models.ProcurementOrder, int64, error) {
	var orders []models.ProcurementOrder
	var total int64

	q := r.db.Model(&models.ProcurementOrder{})
	if filter.ConnectionID > 0 {
		q = q.Where("connection_id = ?", filter.ConnectionID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.LocalOrderNo != "" {
		q = q.Where("local_order_no = ?", filter.LocalOrderNo)
	}
	if filter.UpstreamOrderNo != "" {
		q = q.Where("upstream_order_no = ?", filter.UpstreamOrderNo)
	}
	if filter.CreatedFrom != nil {
		q = q.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		q = q.Where("created_at <= ?", *filter.CreatedTo)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	q = q.Order("created_at DESC").Preload("Connection").Preload("LocalOrder").Preload("LocalOrder.Items")
	if filter.Page > 0 && filter.PageSize > 0 {
		q = q.Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize)
	}

	if err := q.Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// StatsByStatus 按状态聚合采购单数量（忽略分页，复用其他筛选条件）
func (r *GormProcurementOrderRepository) StatsByStatus(filter ProcurementOrderListFilter) (map[string]int64, error) {
	q := r.db.Model(&models.ProcurementOrder{})
	if filter.ConnectionID > 0 {
		q = q.Where("connection_id = ?", filter.ConnectionID)
	}
	// 注意：StatsByStatus 不应用 filter.Status，因为聚合的目的就是看各状态分布
	if filter.LocalOrderNo != "" {
		q = q.Where("local_order_no = ?", filter.LocalOrderNo)
	}
	if filter.UpstreamOrderNo != "" {
		q = q.Where("upstream_order_no = ?", filter.UpstreamOrderNo)
	}
	if filter.CreatedFrom != nil {
		q = q.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		q = q.Where("created_at <= ?", *filter.CreatedTo)
	}

	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	if err := q.Select("status, COUNT(*) as count").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[string]int64, len(rows))
	for _, item := range rows {
		result[item.Status] = item.Count
	}
	return result, nil
}

// ListRetriable 获取需要重试的采购单
func (r *GormProcurementOrderRepository) ListRetriable(now time.Time, limit int) ([]models.ProcurementOrder, error) {
	var orders []models.ProcurementOrder
	q := r.db.Where("status IN (?, ?) AND next_retry_at IS NOT NULL AND next_retry_at <= ?",
		"pending", "failed", now).
		Order("next_retry_at ASC").
		Limit(limit)
	if err := q.Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

// ListByLocalOrderIDs 根据本地订单 ID 批量查询
func (r *GormProcurementOrderRepository) ListByLocalOrderIDs(localOrderIDs []uint) ([]models.ProcurementOrder, error) {
	if len(localOrderIDs) == 0 {
		return nil, nil
	}
	var orders []models.ProcurementOrder
	if err := r.db.Where("local_order_id IN ?", localOrderIDs).Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

// ListByConnectionAndTimeRange 按连接和时间范围查询采购单
func (r *GormProcurementOrderRepository) ListByConnectionAndTimeRange(connectionID uint, start, end time.Time) ([]models.ProcurementOrder, error) {
	var orders []models.ProcurementOrder
	q := r.db.Preload("LocalOrder").Where("connection_id = ? AND created_at >= ? AND created_at <= ?", connectionID, start, end)
	if err := q.Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

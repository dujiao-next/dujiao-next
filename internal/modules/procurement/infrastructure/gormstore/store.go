package gormstore

import (
	"errors"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	"github.com/dujiao-next/internal/modules/procurement/infrastructure/orderreader"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	db *gorm.DB
}

var _ procurementcontract.Repository = (*Store)(nil)

func New(db *gorm.DB) *Store { return &Store{db: db} }

func (s *Store) GetByID(id uint) (*procurementdomain.Order, error) {
	var order procurementdomain.Order
	if err := preloadProcurement(s.active()).First(&order, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if err := s.attachLocalOrder(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *Store) GetByLocalOrderID(localOrderID uint) (*procurementdomain.Order, error) {
	return s.first("local_order_id = ?", localOrderID)
}

func (s *Store) GetByLocalOrderNo(localOrderNo string) (*procurementdomain.Order, error) {
	return s.first("local_order_no = ?", localOrderNo)
}

func (s *Store) first(query string, argument any) (*procurementdomain.Order, error) {
	var order procurementdomain.Order
	if err := preloadProcurement(s.active().Where(query, argument)).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if err := s.attachLocalOrder(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *Store) Create(order *procurementdomain.Order) error {
	return s.db.Omit(clause.Associations).Create(order).Error
}

func (s *Store) UpdateStatus(id uint, fromStatus, toStatus string, updates map[string]interface{}) (bool, error) {
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["status"] = toStatus
	result := s.db.Model(&procurementdomain.Order{}).
		Where("id = ? AND deleted_at IS NULL AND status = ?", id, fromStatus).
		Updates(updates)
	return result.RowsAffected == 1, result.Error
}

func (s *Store) List(filter procurementcontract.ListFilter) ([]procurementdomain.Order, int64, error) {
	query := applyFilter(s.active().Model(&procurementdomain.Order{}), filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = preloadProcurement(query.Order("created_at DESC"))
	if filter.Page > 0 && filter.PageSize > 0 {
		query = query.Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize)
	}
	var orders []procurementdomain.Order
	if err := query.Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	if err := s.attachLocalOrders(orders); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (s *Store) StatsByStatus(filter procurementcontract.ListFilter) (map[string]int64, error) {
	filter.Status = ""
	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	query := applyFilter(s.active().Model(&procurementdomain.Order{}), filter)
	if err := query.Select("status, COUNT(*) as count").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, item := range rows {
		result[item.Status] = item.Count
	}
	return result, nil
}

func (s *Store) ListByConnectionAndTimeRange(connectionID uint, start, end time.Time) ([]procurementdomain.Order, error) {
	var orders []procurementdomain.Order
	query := preloadProcurement(s.active().Where(
		"connection_id = ? AND created_at >= ? AND created_at <= ?",
		connectionID, start, end,
	))
	if err := query.Find(&orders).Error; err != nil {
		return nil, err
	}
	if err := s.attachLocalOrders(orders); err != nil {
		return nil, err
	}
	return orders, nil
}

func (s *Store) active() *gorm.DB {
	return s.db.Where("procurement_orders.deleted_at IS NULL")
}

func preloadProcurement(query *gorm.DB) *gorm.DB {
	return query.Preload("Connection", "deleted_at IS NULL")
}

func applyFilter(query *gorm.DB, filter procurementcontract.ListFilter) *gorm.DB {
	if filter.ConnectionID > 0 {
		query = query.Where("connection_id = ?", filter.ConnectionID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.LocalOrderNo != "" {
		query = query.Where("local_order_no = ?", filter.LocalOrderNo)
	}
	if filter.UpstreamOrderNo != "" {
		query = query.Where("upstream_order_no = ?", filter.UpstreamOrderNo)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}
	return query
}

func (s *Store) attachLocalOrder(order *procurementdomain.Order) error {
	if order == nil || order.LocalOrderID == 0 {
		return nil
	}
	var local orderdomain.Order
	if err := s.db.
		Where("orders.deleted_at IS NULL").
		Preload("Items", "deleted_at IS NULL").
		First(&local, order.LocalOrderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	snapshot := orderreader.MapOrder(local)
	order.LocalOrder = &snapshot
	return nil
}

func (s *Store) attachLocalOrders(orders []procurementdomain.Order) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(orders))
	for i := range orders {
		if orders[i].LocalOrderID > 0 {
			ids = append(ids, orders[i].LocalOrderID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var locals []orderdomain.Order
	if err := s.db.
		Where("orders.deleted_at IS NULL AND id IN ?", ids).
		Preload("Items", "deleted_at IS NULL").
		Find(&locals).Error; err != nil {
		return err
	}
	byID := make(map[uint]procurementdomain.LocalOrder, len(locals))
	for i := range locals {
		byID[locals[i].ID] = orderreader.MapOrder(locals[i])
	}
	for i := range orders {
		if snapshot, ok := byID[orders[i].LocalOrderID]; ok {
			snapshot := snapshot
			orders[i].LocalOrder = &snapshot
		}
	}
	return nil
}

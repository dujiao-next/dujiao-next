package gormstore

import (
	"github.com/dujiao-next/internal/modules/notification/contract"
	"github.com/dujiao-next/internal/modules/notification/domain"
	"github.com/dujiao-next/internal/persistence/gormutil"

	"gorm.io/gorm"
)

// LogStore 是通知日志的 GORM 存储实现。
type LogStore struct {
	db *gorm.DB
}

func NewLogStore(db *gorm.DB) *LogStore {
	return &LogStore{db: db}
}

// Create 创建通知日志
func (r *LogStore) Create(log *domain.NotificationLog) error {
	if log == nil {
		return nil
	}
	return r.db.Create(log).Error
}

// ListAdmin 管理端查询通知日志
func (r *LogStore) ListAdmin(filter contract.LogListFilter) ([]domain.NotificationLog, int64, error) {
	query := r.db.Model(&domain.NotificationLog{})
	if filter.Channel != "" {
		query = query.Where("channel = ?", filter.Channel)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.EventType != "" {
		query = query.Where("event_type = ?", filter.EventType)
	}
	if filter.IsTest != nil {
		query = query.Where("is_test = ?", *filter.IsTest)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = gormutil.ApplyPagination(query, filter.Page, filter.PageSize)

	logs := make([]domain.NotificationLog, 0)
	if err := query.Order("id DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

var _ contract.LogRepository = (*LogStore)(nil)

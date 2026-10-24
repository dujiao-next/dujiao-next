package gormstore

import (
	"github.com/dujiao-next/internal/modules/auditlog/contract"
	"github.com/dujiao-next/internal/modules/auditlog/domain"
	"github.com/dujiao-next/internal/persistence/gormutil"

	"gorm.io/gorm"
)

// UserLoginStore GORM 用户登录日志存储。
type UserLoginStore struct {
	db *gorm.DB
}

func NewUserLoginStore(db *gorm.DB) *UserLoginStore {
	return &UserLoginStore{db: db}
}

// Create 创建登录日志
func (r *UserLoginStore) Create(log *domain.UserLoginLog) error {
	if log == nil {
		return nil
	}
	return r.db.Create(log).Error
}

// ListAdmin 管理端查询登录日志
func (r *UserLoginStore) ListAdmin(filter contract.UserLoginFilter) ([]domain.UserLoginLog, int64, error) {
	query := r.db.Model(&domain.UserLoginLog{})
	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Email != "" {
		query = query.Where("email = ?", filter.Email)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.FailReason != "" {
		query = query.Where("fail_reason = ?", filter.FailReason)
	}
	if filter.ClientIP != "" {
		query = query.Where("client_ip = ?", filter.ClientIP)
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

	logs := make([]domain.UserLoginLog, 0)
	if err := query.Order("id desc").Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

// ListByUser 用户侧查询自己的登录日志
func (r *UserLoginStore) ListByUser(userID uint, page, pageSize int) ([]domain.UserLoginLog, int64, error) {
	query := r.db.Model(&domain.UserLoginLog{}).Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	logs := make([]domain.UserLoginLog, 0)
	if err := query.Order("id desc").Limit(pageSize).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

var _ contract.UserLoginRepository = (*UserLoginStore)(nil)

package gormstore

import (
	"github.com/dujiao-next/internal/modules/auditlog/contract"
	"github.com/dujiao-next/internal/modules/auditlog/domain"
	"github.com/dujiao-next/internal/persistence/gormutil"

	"gorm.io/gorm"
)

// AuthzStore GORM 权限审计日志存储。
type AuthzStore struct {
	db *gorm.DB
}

func NewAuthzStore(db *gorm.DB) *AuthzStore {
	return &AuthzStore{db: db}
}

// Create 创建权限审计日志
func (r *AuthzStore) Create(log *domain.AuthzAuditLog) error {
	if log == nil {
		return nil
	}
	return r.db.Create(log).Error
}

// ListAdmin 管理端查询权限审计日志
func (r *AuthzStore) ListAdmin(filter contract.AuthzFilter) ([]domain.AuthzAuditLog, int64, error) {
	query := r.db.Model(&domain.AuthzAuditLog{})
	if filter.OperatorAdminID != 0 {
		query = query.Where("operator_admin_id = ?", filter.OperatorAdminID)
	}
	if filter.TargetAdminID != 0 {
		query = query.Where("target_admin_id = ?", filter.TargetAdminID)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.Role != "" {
		query = query.Where("role = ?", filter.Role)
	}
	if filter.Object != "" {
		query = query.Where("object = ?", filter.Object)
	}
	if filter.Method != "" {
		query = query.Where("method = ?", filter.Method)
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

	logs := make([]domain.AuthzAuditLog, 0)
	if err := query.Order("id DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

var _ contract.AuthzRepository = (*AuthzStore)(nil)

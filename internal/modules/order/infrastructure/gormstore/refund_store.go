package gormstore

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/persistence/gormutil"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	memberlevelgormstore "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
)

// Create 创建退款记录
func (r *Store) CreateRefundRecord(record *orderdomain.OrderRefundRecord) error {
	if record == nil {
		return nil
	}
	return r.db.Create(record).Error
}

// DecrementTotalSpent 使用当前仓储的事务连接扣回用户累计消费。
func (r *Store) DecrementTotalSpent(userID uint, amount decimal.Decimal) error {
	return memberlevelgormstore.DecrementTotalSpent(r.db, userID, amount)
}

// GetByID 根据 ID 获取退款记录
func (r *Store) GetRefundRecordByID(id uint) (*orderdomain.OrderRefundRecord, error) {
	if id == 0 {
		return nil, nil
	}
	var record orderdomain.OrderRefundRecord
	if err := r.db.Where("deleted_at IS NULL").First(&record, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *Store) GetRefundRecordByIDForUpdate(id uint) (*orderdomain.OrderRefundRecord, error) {
	if id == 0 {
		return nil, nil
	}
	var record orderdomain.OrderRefundRecord
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("deleted_at IS NULL").
		First(&record, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *Store) UpdateRefundRecordPaymentFee(id uint, refunded bool, amount money.Amount, updatedAt time.Time) error {
	return r.db.Model(&orderdomain.OrderRefundRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"payment_fee_refunded":        refunded,
			"payment_fee_refunded_amount": amount,
			"updated_at":                  updatedAt,
		}).Error
}

// ListByOrderIDs 按订单ID列表获取退款记录（按创建时间倒序）
func (r *Store) ListRefundRecordsByOrderIDs(orderIDs []uint) ([]orderdomain.OrderRefundRecord, error) {
	records := make([]orderdomain.OrderRefundRecord, 0)
	if len(orderIDs) == 0 {
		return records, nil
	}
	if err := r.db.
		Where("deleted_at IS NULL AND order_id IN ?", orderIDs).
		Order("created_at DESC, id DESC").
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// ListAdmin 管理端退款记录列表
func (r *Store) ListRefundRecordsAdmin(filter ordercontract.RefundRecordListFilter) ([]orderdomain.OrderRefundRecord, int64, error) {
	records := make([]orderdomain.OrderRefundRecord, 0)
	query := r.db.Model(&orderdomain.OrderRefundRecord{}).
		Joins("LEFT JOIN orders ON orders.id = order_refund_records.order_id AND orders.deleted_at IS NULL").
		Where("order_refund_records.deleted_at IS NULL")

	if filter.UserID != 0 {
		query = query.Where("orders.user_id = ?", filter.UserID)
	}
	if keyword := strings.TrimSpace(filter.UserKeyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"orders.user_id IN ("+
				"SELECT users.id FROM users "+
				"WHERE users.deleted_at IS NULL AND ("+
				"users.email LIKE ? OR "+
				"users.display_name LIKE ? OR "+
				"EXISTS ("+
				"SELECT 1 FROM user_oauth_identities "+
				"WHERE user_oauth_identities.user_id = users.id AND ("+
				"user_oauth_identities.provider LIKE ? OR "+
				"user_oauth_identities.provider_user_id LIKE ? OR "+
				"user_oauth_identities.username LIKE ?"+
				")"+
				")"+
				")"+
				")",
			like, like, like, like, like,
		)
	}
	if orderNo := strings.TrimSpace(filter.OrderNo); orderNo != "" {
		query = query.Where("orders.order_no = ?", orderNo)
	}
	if guestEmail := strings.TrimSpace(filter.GuestEmail); guestEmail != "" {
		query = query.Where("(order_refund_records.guest_email = ? OR orders.guest_email = ?)", guestEmail, guestEmail)
	}
	if keyword := strings.TrimSpace(filter.ProductKeyword); keyword != "" {
		like := "%" + keyword + "%"
		cond, argCount := gormutil.BuildLocalizedLikeCondition(r.db, nil, []string{"oi.title_json"})
		if cond != "" {
			args := gormutil.RepeatLikeArgs(like, argCount)
			query = query.Where(
				"EXISTS (SELECT 1 FROM order_items oi WHERE oi.deleted_at IS NULL AND oi.order_id = order_refund_records.order_id AND ("+cond+"))",
				args...,
			)
		}
	}
	if filter.CreatedFrom != nil {
		query = query.Where("order_refund_records.created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("order_refund_records.created_at <= ?", *filter.CreatedTo)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	dataQuery := gormutil.ApplyPagination(query.Session(&gorm.Session{}), filter.Page, filter.PageSize)
	if err := dataQuery.
		Order("order_refund_records.id DESC").
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

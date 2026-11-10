package gormstore

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/telegramidentity"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Lifecycle owns the procurement-side persistence operations that span
// fulfillments and parent/child order status. It deliberately uses private
// records through the procurement-owned store.
type Lifecycle struct {
	db                 *gorm.DB
	queue              StatusEmailQueue
	settings           *settingsapp.Service
	defaultEmailConfig config.EmailConfig
}

var _ procurementcontract.OrderLifecycle = (*Lifecycle)(nil)

type StatusEmailQueue interface {
	EnqueueOrderStatusEmail(payload queue.OrderStatusEmailPayload, opts ...asynq.Option) error
}

type lifecycleOrderRecord struct {
	ID         uint                   `gorm:"primarykey"`
	ParentID   *uint                  `gorm:"index"`
	UserID     uint                   `gorm:"index;not null"`
	GuestEmail string                 `gorm:"index"`
	Status     string                 `gorm:"index;not null"`
	DeletedAt  gorm.DeletedAt         `gorm:"index"`
	Children   []lifecycleOrderRecord `gorm:"foreignKey:ParentID"`
}

func (lifecycleOrderRecord) TableName() string { return "orders" }

type lifecycleUserRecord struct {
	ID        uint `gorm:"primarykey"`
	Email     string
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (lifecycleUserRecord) TableName() string { return "users" }

type lifecycleFulfillmentRecord struct {
	ID            uint           `gorm:"primarykey"`
	OrderID       uint           `gorm:"uniqueIndex;not null"`
	Type          string         `gorm:"not null"`
	Status        string         `gorm:"not null"`
	Payload       string         `gorm:"type:text"`
	LogisticsJSON jsonmap.JSON   `gorm:"type:json"`
	DeliveredAt   *time.Time     `gorm:"index"`
	CreatedAt     time.Time      `gorm:"index"`
	UpdatedAt     time.Time      `gorm:"index"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (lifecycleFulfillmentRecord) TableName() string { return "fulfillments" }

func NewLifecycle(
	db *gorm.DB,
	queueClient StatusEmailQueue,
	settings *settingsapp.Service,
	defaultEmailConfig config.EmailConfig,
) *Lifecycle {
	return &Lifecycle{
		db: db, queue: queueClient, settings: settings, defaultEmailConfig: defaultEmailConfig,
	}
}

func (s *Store) NewLifecycle(
	queueClient StatusEmailQueue,
	settings *settingsapp.Service,
	defaultEmailConfig config.EmailConfig,
) *Lifecycle {
	return NewLifecycle(s.db, queueClient, settings, defaultEmailConfig)
}

func (l *Lifecycle) DeliverUpstreamOrder(orderID uint, fulfillment *procurementcontract.Fulfillment, now time.Time) (bool, error) {
	delivered := false
	err := l.db.Transaction(func(tx *gorm.DB) error {
		// 先锁订单再复核状态（与退款的加锁顺序一致）：退款先提交时不能再交付，也不能覆盖退款状态。
		var order lifecycleOrderRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, orderID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if order.Status == constants.OrderStatusRefunded || order.Status == constants.OrderStatusCanceled {
			return nil
		}
		if fulfillment != nil {
			var existing lifecycleFulfillmentRecord
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("order_id = ?", orderID).
				First(&existing).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err != nil {
				deliveredAt := fulfillment.DeliveredAt
				if deliveredAt == nil {
					deliveredAt = &now
				}
				if err := tx.Create(&lifecycleFulfillmentRecord{
					OrderID: orderID, Type: constants.FulfillmentTypeUpstream,
					Status: constants.FulfillmentStatusDelivered, Payload: fulfillment.Payload,
					LogisticsJSON: fulfillment.DeliveryData, DeliveredAt: deliveredAt,
					CreatedAt: now, UpdatedAt: now,
				}).Error; err != nil {
					return err
				}
			}
		}
		delivered = true
		return tx.Table(order.TableName()).Where("id = ?", orderID).Updates(map[string]interface{}{
			"status": constants.OrderStatusDelivered, "updated_at": now,
		}).Error
	})
	return delivered, err
}

func (l *Lifecycle) SyncParentStatus(parentID uint, now time.Time) (string, error) {
	if parentID == 0 {
		return "", nil
	}
	var parent lifecycleOrderRecord
	if err := l.db.Preload("Children").First(&parent, parentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	if parent.ParentID != nil {
		return "", nil
	}
	if parent.Status == constants.OrderStatusCanceled {
		return parent.Status, nil
	}
	newStatus := calculateParentStatus(parent.Children, parent.Status)
	if newStatus == "" || newStatus == parent.Status {
		return parent.Status, nil
	}
	if err := l.db.Table(parent.TableName()).Where("id = ? AND deleted_at IS NULL", parent.ID).Updates(map[string]interface{}{
		"status": newStatus, "updated_at": now,
	}).Error; err != nil {
		return "", err
	}
	return newStatus, nil
}

func (l *Lifecycle) EnqueueStatusEmail(orderID uint, status string) (bool, error) {
	if l.queue == nil || orderID == 0 {
		return true, nil
	}
	if l.settings != nil {
		smtpSetting, err := l.settings.GetSMTPSetting(l.defaultEmailConfig)
		if err != nil {
			return false, err
		}
		if !smtpSetting.Enabled || !smtpSetting.OrderNotificationEnabled {
			return true, nil
		}
	}
	receiverEmail, err := l.resolveReceiverEmail(orderID)
	if err == nil {
		receiverEmail = strings.TrimSpace(receiverEmail)
		if receiverEmail == "" || telegramidentity.IsPlaceholderEmail(receiverEmail) {
			return true, nil
		}
	}
	if err := l.queue.EnqueueOrderStatusEmail(queue.OrderStatusEmailPayload{
		OrderID: orderID, Status: strings.TrimSpace(status),
	}); err != nil {
		return false, err
	}
	return false, nil
}

func (l *Lifecycle) resolveReceiverEmail(orderID uint) (string, error) {
	var order lifecycleOrderRecord
	if err := l.db.Select("user_id", "guest_email").First(&order, orderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	if order.UserID == 0 {
		return strings.TrimSpace(order.GuestEmail), nil
	}
	var user lifecycleUserRecord
	if err := l.db.Select("email").First(&user, order.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(user.Email), nil
}

func calculateParentStatus(children []lifecycleOrderRecord, currentStatus string) string {
	if len(children) == 0 {
		return currentStatus
	}
	counts := make(map[string]int, 8)
	for _, child := range children {
		counts[strings.ToLower(strings.TrimSpace(child.Status))]++
	}
	size := len(children)
	if counts[constants.OrderStatusCanceled] == size {
		return constants.OrderStatusCanceled
	}
	if counts[constants.OrderStatusRefunded] == size {
		return constants.OrderStatusRefunded
	}
	if counts[constants.OrderStatusRefunded] > 0 || counts[constants.OrderStatusPartiallyRefunded] > 0 {
		return constants.OrderStatusPartiallyRefunded
	}
	if counts[constants.OrderStatusCompleted] == size {
		return constants.OrderStatusCompleted
	}
	delivered := counts[constants.OrderStatusDelivered] + counts[constants.OrderStatusCompleted]
	if delivered == size {
		return constants.OrderStatusDelivered
	}
	if delivered > 0 {
		return constants.OrderStatusPartiallyDelivered
	}
	if counts[constants.OrderStatusFulfilling] > 0 {
		return constants.OrderStatusFulfilling
	}
	if counts[constants.OrderStatusPaid] > 0 {
		return constants.OrderStatusPaid
	}
	if counts[constants.OrderStatusPendingPayment] > 0 {
		return constants.OrderStatusPendingPayment
	}
	return currentStatus
}

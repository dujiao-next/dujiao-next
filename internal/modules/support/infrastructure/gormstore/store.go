package gormstore

import (
	"context"
	"errors"
	"time"

	"github.com/dujiao-next/internal/modules/support/application"
	"github.com/dujiao-next/internal/modules/support/contract"
	"github.com/dujiao-next/internal/modules/support/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct{ db *gorm.DB }

func New(db *gorm.DB) *Store { return &Store{db} }

var _ contract.Store = (*Store)(nil)

func (s *Store) Create(ctx context.Context, c *domain.Conversation, m *domain.Message) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing domain.Conversation
		if err := tx.Where("creation_key = ?", c.CreationKey).First(&existing).Error; err == nil {
			*c = existing
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(c).Error; err != nil {
			return err
		}
		m.ConversationID = c.ID
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		return tx.Create(&domain.Reminder{ConversationID: c.ID, MessageID: m.ID, DueAt: time.Now().Add(time.Minute)}).Error
	})
}
func (s *Store) Get(ctx context.Context, id uint) (*domain.Conversation, error) {
	var c domain.Conversation
	err := s.db.WithContext(ctx).First(&c, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, contract.ErrNotFound
	}
	return &c, err
}
func (s *Store) List(ctx context.Context, a contract.Actor, status string, page, limit int) ([]domain.Conversation, int64, error) {
	q := s.db.WithContext(ctx).Model(&domain.Conversation{})
	if a.AdminID > 0 {
		if !a.Supervisor {
			q = q.Where("assignee_id = 0 OR assignee_id = ?", a.AdminID)
		}
	} else {
		q = q.Where("tenant_id = ?", a.TenantID)
		if a.UserID > 0 {
			q = q.Where("user_id = ?", a.UserID)
		} else {
			q = q.Where("user_id = 0 AND guest_hash = ? AND guest_expires_at > ?", application.HashToken(a.GuestToken), time.Now())
		}
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := []domain.Conversation{}
	err := q.Order("updated_at DESC, id DESC").Offset((page - 1) * limit).Limit(limit).Find(&items).Error
	return items, total, err
}
func (s *Store) Messages(ctx context.Context, id, after uint, limit int, admin bool) ([]domain.Message, error) {
	q := s.db.WithContext(ctx).Where("conversation_id = ? AND id > ?", id, after)
	if !admin {
		q = q.Where("internal = ?", false)
	}
	items := []domain.Message{}
	err := q.Order("id ASC").Limit(limit).Find(&items).Error
	return items, err
}
func (s *Store) Append(ctx context.Context, c *domain.Conversation, m *domain.Message, attachmentIDs []uint) (*domain.Message, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize per-conversation writes and recheck assignment after locking.
		var current domain.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, c.ID).Error; err != nil {
			return err
		}
		if current.AssigneeID != c.AssigneeID {
			return contract.ErrConflict
		}
		var old domain.Message
		err := tx.Where("conversation_id = ? AND client_id = ?", c.ID, m.ClientID).First(&old).Error
		if err == nil {
			if old.SenderType != m.SenderType || old.SenderID != m.SenderID {
				return contract.ErrConflict
			}
			*m = old
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		for _, id := range attachmentIDs {
			r := tx.Model(&domain.Attachment{}).Where("id = ? AND conversation_id = ? AND message_id = 0 AND sender_type = ? AND sender_id = ?", id, c.ID, m.SenderType, m.SenderID).Update("message_id", m.ID)
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return contract.ErrInvalid
			}
		}
		updates := map[string]interface{}{"updated_at": time.Now()}
		if m.SenderType != "admin" {
			updates["status"] = "open"
			if err := tx.Create(&domain.Reminder{ConversationID: c.ID, MessageID: m.ID, DueAt: time.Now().Add(time.Minute)}).Error; err != nil {
				return err
			}
		}
		if m.SenderType == "admin" {
			if err := tx.Create(&domain.Audit{ConversationID: c.ID, AdminID: m.SenderID, Action: "message", TargetID: m.ID}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&domain.Conversation{}).Where("id = ?", c.ID).Updates(updates).Error
	})
	return m, err
}
func (s *Store) Update(ctx context.Context, c *domain.Conversation, a contract.Actor, action string, target uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&domain.Conversation{}).Where("id = ?", c.ID)
		updates := map[string]interface{}{"updated_at": time.Now()}
		switch action {
		case "claim":
			q = q.Where("assignee_id = 0 OR assignee_id = ?", a.AdminID)
			updates["assignee_id"] = a.AdminID
			updates["status"] = "active"
		case "assign":
			if !a.Supervisor {
				q = q.Where("assignee_id = ?", a.AdminID)
			}
			updates["assignee_id"] = target
		case "status":
			if !a.Supervisor {
				q = q.Where("assignee_id = ?", a.AdminID)
			}
			updates["status"] = c.Status
		case "order":
			updates["order_summary"] = c.OrderSummary
		default:
			return contract.ErrInvalid
		}
		r := q.Updates(updates)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return contract.ErrConflict
		}
		if a.AdminID > 0 {
			return tx.Create(&domain.Audit{ConversationID: c.ID, AdminID: a.AdminID, Action: action, TargetID: target}).Error
		}
		return nil
	})
}
func (s *Store) Read(ctx context.Context, id, msg uint, admin bool) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m domain.Message
		q := tx.Where("id = ? AND conversation_id = ?", msg, id)
		if !admin {
			q = q.Where("internal = ?", false)
		}
		if err := q.First(&m).Error; err != nil {
			return contract.ErrInvalid
		}
		column := "customer_read_id"
		if admin {
			column = "admin_read_id"
		}
		return tx.Model(&domain.Conversation{}).Where("id = ? AND "+column+" < ?", id, msg).Update(column, msg).Error
	})
}
func (s *Store) Unread(ctx context.Context, c *domain.Conversation, admin bool) (int64, error) {
	q := s.db.WithContext(ctx).Model(&domain.Message{}).Where("conversation_id = ? AND internal = ?", c.ID, false)
	if admin {
		q = q.Where("sender_type <> 'admin' AND id > ?", c.AdminReadID)
	} else {
		q = q.Where("sender_type = 'admin' AND id > ?", c.CustomerReadID)
	}
	var n int64
	err := q.Count(&n).Error
	return n, err
}
func (s *Store) PutAttachment(ctx context.Context, a *domain.Attachment) error {
	return s.db.WithContext(ctx).Create(a).Error
}
func (s *Store) Attachments(ctx context.Context, id uint, msgs []uint) ([]domain.Attachment, error) {
	items := []domain.Attachment{}
	if len(msgs) == 0 {
		return items, nil
	}
	err := s.db.WithContext(ctx).Select("id, message_id, mime, size").Where("conversation_id = ? AND message_id IN ?", id, msgs).Find(&items).Error
	return items, err
}
func (s *Store) GetAttachment(ctx context.Context, id, aid uint) (*domain.Attachment, error) {
	var a domain.Attachment
	err := s.db.WithContext(ctx).Where("id = ? AND conversation_id = ?", aid, id).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, contract.ErrNotFound
	}
	return &a, err
}
func (s *Store) DueReminders(ctx context.Context, now time.Time) ([]domain.Reminder, error) {
	items := []domain.Reminder{}
	err := s.db.WithContext(ctx).Where("due_at <= ?", now).Order("id ASC").Limit(100).Find(&items).Error
	return items, err
}
func (s *Store) DeleteReminder(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&domain.Reminder{}, id).Error
}

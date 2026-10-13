package contract

import (
	"context"
	"errors"
	"time"

	"github.com/dujiao-next/internal/modules/support/domain"
)

var (
	ErrNotFound                  = errors.New("support not found")
	ErrInvalid                   = errors.New("invalid support request")
	ErrConflict                  = errors.New("support conflict")
	ErrForbidden                 = errors.New("support forbidden")
	ErrNotificationNotConfigured = errors.New("support notification not configured")
)

type Actor struct {
	UserID, AdminID, TenantID uint
	GuestToken                string
	Supervisor                bool
}
type Agent struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}
type OrderItem struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}
type OrderSummary struct {
	OrderNo     string      `json:"order_no"`
	Status      string      `json:"status"`
	TotalAmount string      `json:"total_amount"`
	Currency    string      `json:"currency"`
	Items       []OrderItem `json:"items"`
}
type Orders interface {
	Verify(context.Context, Actor, string, string, string) (*OrderSummary, error)
}
type Agents interface {
	Eligible(context.Context, uint) bool
	List(context.Context) ([]Agent, error)
}
type Store interface {
	Create(context.Context, *domain.Conversation, *domain.Message) error
	Get(context.Context, uint) (*domain.Conversation, error)
	List(context.Context, Actor, string, int, int) ([]domain.Conversation, int64, error)
	Messages(context.Context, uint, uint, int, bool) ([]domain.Message, error)
	Append(context.Context, *domain.Conversation, *domain.Message, []uint) (*domain.Message, error)
	Update(context.Context, *domain.Conversation, Actor, string, uint) error
	Read(context.Context, uint, uint, bool) error
	Unread(context.Context, *domain.Conversation, bool) (int64, error)
	PutAttachment(context.Context, *domain.Attachment) error
	Attachments(context.Context, uint, []uint) ([]domain.Attachment, error)
	GetAttachment(context.Context, uint, uint) (*domain.Attachment, error)
	DueReminders(context.Context, time.Time) ([]domain.Reminder, error)
	DeleteReminder(context.Context, uint) error
	ClaimReminder(context.Context, uint, time.Time) (bool, error)
	RetryReminder(context.Context, uint, time.Time, string) error
	Config(context.Context) (*domain.NotificationConfig, error)
	SaveConfig(context.Context, *domain.NotificationConfig) error
	Binding(context.Context, uint) (*domain.Binding, error)
	SaveBinding(context.Context, *domain.Binding) error
	Bindings(context.Context) ([]domain.Binding, error)
	Delivery(context.Context, uint, uint) (*domain.Delivery, error)
	SaveDelivery(context.Context, *domain.Delivery) error
	Deliveries(context.Context) ([]domain.Delivery, error)
	CreateScan(context.Context, *domain.ScanSession) error
	Scan(context.Context, uint, string) (*domain.ScanSession, error)
	ClaimScanPoll(context.Context, uint, string, time.Time) (bool, error)
	SetScanCandidate(context.Context, uint, string, string, string) error
	ConfirmScan(context.Context, uint, string, string) error
	CancelScan(context.Context, uint, string) error
	CancelAdminScans(context.Context, uint) error
}

type ParameterQRCode struct {
	Code string
	URL  string
}
type QRSubscriber interface {
	CreateQRCode(context.Context, string, string, int) (ParameterQRCode, error)
	ScanUID(context.Context, string) (string, error)
}

type PushResult struct{ SendRecordID int64 }
type Pusher interface {
	Send(context.Context, string, string, string, string, string) (PushResult, error)
}
type SecretBox interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

type SimplePusher interface {
	SendSimple(context.Context, string, string, string, string) (PushResult, error)
}

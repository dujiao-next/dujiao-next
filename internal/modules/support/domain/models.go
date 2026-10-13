package domain

import "time"

type Conversation struct {
	CreationKey    string    `gorm:"size:64;uniqueIndex" json:"-"`
	ID             uint      `gorm:"primaryKey" json:"id"`
	TenantID       uint      `gorm:"index" json:"tenant_id"`
	UserID         uint      `gorm:"index" json:"user_id"`
	GuestHash      string    `gorm:"size:64;index" json:"-"`
	GuestExpiresAt time.Time `json:"-"`
	Subject        string    `gorm:"size:160" json:"subject"`
	Category       string    `gorm:"size:32" json:"category"`
	ProductID      uint      `json:"product_id"`
	OrderSummary   string    `gorm:"type:text" json:"-"`
	Status         string    `gorm:"size:20;index" json:"status"`
	AssigneeID     uint      `gorm:"index" json:"assignee_id"`
	CustomerReadID uint      `json:"-"`
	AdminReadID    uint      `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `gorm:"index" json:"updated_at"`
}

func (Conversation) TableName() string { return "support_conversations" }

type Message struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ConversationID uint      `gorm:"uniqueIndex:idx_support_client;index;not null" json:"conversation_id"`
	ClientID       string    `gorm:"size:64;uniqueIndex:idx_support_client;not null" json:"client_id"`
	SenderType     string    `gorm:"size:16" json:"sender_type"`
	SenderID       uint      `json:"sender_id"`
	Content        string    `gorm:"type:text" json:"content"`
	Internal       bool      `json:"internal"`
	CreatedAt      time.Time `json:"created_at"`
}

func (Message) TableName() string { return "support_messages" }

// Audit deliberately records identifiers, not message bodies or credentials.
type Audit struct {
	ID             uint `gorm:"primaryKey"`
	ConversationID uint `gorm:"index"`
	AdminID        uint
	Action         string `gorm:"size:32"`
	TargetID       uint
	CreatedAt      time.Time
}

func (Audit) TableName() string { return "support_audits" }

type Attachment struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ConversationID uint      `gorm:"index" json:"-"`
	MessageID      uint      `gorm:"index" json:"-"`
	SenderType     string    `json:"-"`
	SenderID       uint      `json:"-"`
	MIME           string    `json:"mime"`
	Size           int       `json:"size"`
	Data           []byte    `json:"-"`
	CreatedAt      time.Time `json:"-"`
}

func (Attachment) TableName() string { return "support_attachments" }

// Reminder is a durable per-message event; a delivery adapter must recheck state.
type Reminder struct {
	ID             uint      `gorm:"primaryKey"`
	ConversationID uint      `gorm:"index"`
	MessageID      uint      `gorm:"uniqueIndex"`
	DueAt          time.Time `gorm:"index"`
	LeaseUntil     time.Time `gorm:"index"`
	Attempts       int
	LastError      string `gorm:"size:200"`
	CreatedAt      time.Time
}

func (Reminder) TableName() string { return "support_reminders" }

type NotificationConfig struct {
	ID          uint   `gorm:"primaryKey" json:"-"`
	Enabled     bool   `json:"enabled"`
	TokenCipher string `json:"-"`
	BaseURL     string `json:"base_url"`
}

func (NotificationConfig) TableName() string { return "support_notification_config" }

type Binding struct {
	Mode             string    `gorm:"size:16" json:"mode"`
	SPTCipher        string    `json:"-"`
	PendingMode      string    `json:"-"`
	PendingSPTCipher string    `json:"-"`
	AdminID          uint      `gorm:"primaryKey" json:"-"`
	UID              string    `gorm:"size:80" json:"uid"`
	Enabled          bool      `json:"enabled"`
	PendingUID       string    `json:"-"`
	PendingHash      string    `json:"-"`
	PendingExpires   time.Time `json:"-"`
	LastSentAt       time.Time `json:"-"`
	LastAttemptAt    time.Time `json:"-"`
}

func (Binding) TableName() string { return "support_notification_bindings" }

type Delivery struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ReminderID   uint      `gorm:"uniqueIndex:idx_support_delivery" json:"reminder_id"`
	AdminID      uint      `gorm:"uniqueIndex:idx_support_delivery" json:"admin_id"`
	Status       string    `json:"status"`
	SendRecordID int64     `json:"send_record_id"`
	Error        string    `gorm:"size:200" json:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (Delivery) TableName() string { return "support_notification_deliveries" }

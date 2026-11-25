package domain

import (
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

// LedgerEntry 分销商资金流水。
type LedgerEntry struct {
	ID                uint         `gorm:"primarykey" json:"id"`
	ResellerID        uint         `gorm:"not null;index" json:"reseller_id"`
	OrderID           *uint        `gorm:"index" json:"order_id,omitempty"`
	Type              string       `gorm:"type:varchar(32);not null;index" json:"type"`
	Amount            money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"amount"`
	Currency          string       `gorm:"type:varchar(16);not null;index" json:"currency"`
	IdempotencyKey    string       `gorm:"type:varchar(160);not null;uniqueIndex" json:"idempotency_key"`
	MetadataJSON      jsonmap.JSON `gorm:"type:json" json:"metadata_json"`
	Status            string       `gorm:"type:varchar(32);not null;index" json:"status"`
	AvailableAt       *time.Time   `gorm:"index" json:"available_at,omitempty"`
	WithdrawRequestID *uint        `gorm:"index" json:"withdraw_request_id,omitempty"`
	Remark            string       `gorm:"type:text" json:"remark,omitempty"`
	CreatedAt         time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt         time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt         *time.Time   `gorm:"index" json:"-"`

	Profile *Profile           `gorm:"foreignKey:ResellerID" json:"profile,omitempty"`
	Order   *orderdomain.Order `gorm:"foreignKey:OrderID" json:"order,omitempty"`
}

func (LedgerEntry) TableName() string { return "reseller_ledger_entries" }

// WithdrawRequest 分销商提现申请。
type WithdrawRequest struct {
	ID           uint         `gorm:"primarykey" json:"id"`
	ResellerID   uint         `gorm:"not null;index" json:"reseller_id"`
	Amount       money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"amount"`
	Currency     string       `gorm:"type:varchar(16);not null;index" json:"currency"`
	Channel      string       `gorm:"type:varchar(64);not null" json:"channel"`
	Account      string       `gorm:"type:varchar(255);not null" json:"account"`
	Status       string       `gorm:"type:varchar(32);not null;index" json:"status"`
	RejectReason string       `gorm:"type:text" json:"reject_reason,omitempty"`
	ProcessedBy  *uint        `gorm:"index" json:"processed_by,omitempty"`
	ProcessedAt  *time.Time   `gorm:"index" json:"processed_at,omitempty"`
	CreatedAt    time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt    *time.Time   `gorm:"index" json:"-"`

	Profile   *Profile           `gorm:"foreignKey:ResellerID" json:"profile,omitempty"`
	Processor *admindomain.Admin `gorm:"foreignKey:ProcessedBy" json:"processor,omitempty"`
}

func (WithdrawRequest) TableName() string { return "reseller_withdraw_requests" }

// BalanceAccount 分销商按币种余额账户。
type BalanceAccount struct {
	ID                   uint         `gorm:"primarykey" json:"id"`
	ResellerID           uint         `gorm:"not null;index" json:"reseller_id"`
	Currency             string       `gorm:"type:varchar(16);not null;index" json:"currency"`
	Status               string       `gorm:"type:varchar(32);not null;index;default:'normal'" json:"status"`
	AvailableAmountCache money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"available_amount_cache"`
	LockedAmountCache    money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"locked_amount_cache"`
	NegativeAmountCache  money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"negative_amount_cache"`
	// ChannelFeeCarryover 通道费分摊的待抵扣余额：当单利润不足以覆盖应分摊金额时，
	// 扣满利润后剩余的差额记在这里，由后续订单利润优先冲抵。不会计入负余额。
	ChannelFeeCarryover money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"channel_fee_carryover"`
	LastLedgerEntryID   uint         `gorm:"not null;default:0" json:"last_ledger_entry_id"`
	RiskNote            string       `gorm:"type:text" json:"risk_note,omitempty"`
	CreatedAt           time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt           time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt           *time.Time   `gorm:"index" json:"-"`

	Profile *Profile `gorm:"foreignKey:ResellerID" json:"profile,omitempty"`
}

func (BalanceAccount) TableName() string { return "reseller_balance_accounts" }

package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// Transaction 钱包流水明细。
type Transaction struct {
	ID                       uint         `gorm:"primarykey" json:"id"`
	UserID                   uint         `gorm:"index;not null" json:"user_id"`
	OperatorAdminID          *uint        `gorm:"index" json:"operator_admin_id,omitempty"`
	OrderID                  *uint        `gorm:"index" json:"order_id,omitempty"`
	RechargeOrderID          *uint        `gorm:"index" json:"recharge_order_id,omitempty"`
	Type                     string       `gorm:"type:varchar(40);index;not null" json:"type"`
	Direction                string       `gorm:"type:varchar(16);index;not null" json:"direction"`
	Amount                   money.Amount `gorm:"type:decimal(20,2);not null" json:"amount"`
	BalanceBefore            money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"balance_before"`
	BalanceAfter             money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"balance_after"`
	PaymentFeeRefunded       bool         `gorm:"not null;default:false" json:"payment_fee_refunded"`
	PaymentFeeRefundedAmount money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"payment_fee_refunded_amount"`
	Currency                 string       `gorm:"type:varchar(16);not null;default:'CNY'" json:"currency"`
	Reference                string       `gorm:"type:varchar(120);uniqueIndex" json:"reference"`
	Remark                   string       `gorm:"type:varchar(255)" json:"remark"`
	CreatedAt                time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt                time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt                *time.Time   `gorm:"index" json:"-"`
}

func (Transaction) TableName() string {
	return "wallet_transactions"
}

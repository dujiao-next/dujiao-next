package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

// OrderRefundRecord 退款记录
type OrderRefundRecord struct {
	ID                       uint         `gorm:"primarykey" json:"id"`
	UserID                   uint         `gorm:"index;not null;default:0" json:"user_id"`
	GuestEmail               string       `gorm:"index;type:varchar(255)" json:"guest_email,omitempty"`
	OrderID                  uint         `gorm:"index;not null" json:"order_id"`
	Type                     string       `gorm:"index;type:varchar(32);not null" json:"type"`
	Amount                   money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"amount"`
	PaymentFeeRefunded       bool         `gorm:"not null;default:false" json:"payment_fee_refunded"`
	PaymentFeeRefundedAmount money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"payment_fee_refunded_amount"`
	Currency                 string       `gorm:"type:varchar(16);not null;default:''" json:"currency"`
	Remark                   string       `gorm:"type:text" json:"remark,omitempty"`
	CreatedAt                time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt                time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt                *time.Time   `gorm:"index" json:"-"`
}

// TableName 指定表名
func (OrderRefundRecord) TableName() string {
	return "order_refund_records"
}

// CalculatePaymentFeeRefundAmount 按退款比例分摊原支付手续费，并处理累计退款的舍入尾差。
func CalculatePaymentFeeRefundAmount(
	paymentAmount, paymentFee, refundedPrincipalBefore, refundedFeeBefore, refundAmount decimal.Decimal,
) decimal.Decimal {
	return money.CalculatePaymentFeeRefundAmount(paymentAmount, paymentFee, refundedPrincipalBefore, refundedFeeBefore, refundAmount)
}

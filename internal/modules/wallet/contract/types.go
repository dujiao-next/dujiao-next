package contract

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

type AccountListFilter struct {
	Page     int
	PageSize int
	UserID   uint
}

type TransactionListFilter struct {
	Page        int
	PageSize    int
	UserID      uint
	OrderID     uint
	Type        string
	Direction   string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

type RechargeListFilter struct {
	Page         int
	PageSize     int
	RechargeNo   string
	UserID       uint
	UserKeyword  string
	PaymentID    uint
	ChannelID    uint
	ProviderType string
	ChannelType  string
	Status       string
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
	PaidFrom     *time.Time
	PaidTo       *time.Time
}

type RechargeInput struct {
	UserID   uint
	Amount   money.Amount
	Currency string
	Remark   string
}

type AdjustBalanceInput struct {
	UserID          uint
	OperatorAdminID uint
	Delta           money.Amount
	Currency        string
	Remark          string
}

type CreditInput struct {
	UserID    uint
	Amount    money.Amount
	Currency  string
	Type      string
	Reference string
	Remark    string
	OrderID   *uint
}

type OrderBalanceInput struct {
	OrderID          uint
	UserID           uint
	TotalAmount      money.Amount
	WalletPaidAmount money.Amount
	Currency         string
	UseBalance       bool
}

type OrderReleaseInput struct {
	OrderID          uint
	UserID           uint
	WalletPaidAmount money.Amount
	TotalAmount      money.Amount
	Currency         string
	TransactionType  string
	Remark           string
}

// OrderBalanceRecoveryInput identifies an order allocation that was released
// before a late successful online-payment callback arrived.
type OrderBalanceRecoveryInput struct {
	OrderID                uint
	UserID                 uint
	TotalAmount            money.Amount
	ExpectedAmount         money.Amount
	SnapshotKnown          bool
	Currency               string
	ReleaseTransactionType string
	Remark                 string
}

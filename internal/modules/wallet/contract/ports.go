package contract

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
)

// Repository owns all wallet aggregate persistence. Transactional callers
// receive the same port bound to their transaction through Transaction.
type Repository interface {
	GetAccountByUserID(userID uint) (*walletdomain.Account, error)
	GetAccountByUserIDForUpdate(userID uint) (*walletdomain.Account, error)
	GetAccountsByUserIDs(userIDs []uint) ([]walletdomain.Account, error)
	CreateAccount(account *walletdomain.Account) error
	UpdateAccount(account *walletdomain.Account) error
	ListAccounts(filter AccountListFilter) ([]walletdomain.Account, int64, error)

	CreateTransaction(transaction *walletdomain.Transaction) error
	GetTransactionByReference(reference string) (*walletdomain.Transaction, error)
	CountOrderTransactionsByType(orderID uint, transactionType string) (int64, error)
	ListTransactions(filter TransactionListFilter) ([]walletdomain.Transaction, int64, error)

	ListRechargeRefunds(id uint) ([]walletdomain.Transaction, error)
	GetRechargeOrderByID(id uint) (*walletdomain.RechargeOrder, error)
	CreateRechargeOrder(order *walletdomain.RechargeOrder) error
	UpdateRechargeOrder(order *walletdomain.RechargeOrder) error
	ApplyRechargeRefund(userID uint, amount decimal.Decimal) error
	GetRechargeOrderByRechargeNo(userID uint, rechargeNo string) (*walletdomain.RechargeOrder, error)
	GetRechargeOrderByPaymentID(paymentID uint) (*walletdomain.RechargeOrder, error)
	GetRechargeOrderByPaymentIDAndUser(paymentID, userID uint) (*walletdomain.RechargeOrder, error)
	GetRechargeOrderByPaymentIDForUpdate(paymentID uint) (*walletdomain.RechargeOrder, error)
	GetRechargeOrdersByPaymentIDs(paymentIDs []uint) ([]walletdomain.RechargeOrder, error)
	ListRechargeOrdersAdmin(filter RechargeListFilter) ([]walletdomain.RechargeOrder, int64, error)
	StatsRechargeOrders(filter RechargeListFilter) (map[string]int64, error)
}

// Transaction is the wallet-owned view of an already-open database
// transaction. It intentionally exposes no ORM primitive.
type Transaction interface {
	Wallets() Repository
}

type UnitOfWork interface {
	WithinTransaction(fn func(Transaction) error) error
}

// RechargeRefundEmailQueue 将已完成的充值退款邮件加入异步队列。
type RechargeRefundEmailQueue interface {
	EnqueueRechargeRefundEmail(rechargeID, refundID uint, status string) error
}

type UseCase interface {
	GetAccount(userID uint) (*walletdomain.Account, error)
	ListTransactions(filter TransactionListFilter) ([]walletdomain.Transaction, int64, error)
	ListRechargeOrdersAdmin(filter RechargeListFilter) ([]walletdomain.RechargeOrder, int64, error)
	ListUserRechargeOrders(userID uint, page, pageSize int, status, rechargeNo string) ([]walletdomain.RechargeOrder, int64, error)
	StatsUserRechargeOrders(userID uint, rechargeNo string) (map[string]int64, error)
	GetRechargeOrderByRechargeNo(userID uint, rechargeNo string) (*walletdomain.RechargeOrder, error)
	GetRechargeOrderByPaymentIDAndUser(paymentID, userID uint) (*walletdomain.RechargeOrder, error)
	GetBalancesByUserIDs(userIDs []uint) (map[uint]money.Amount, error)

	Recharge(input RechargeInput) (*walletdomain.Account, *walletdomain.Transaction, error)
	AdminAdjustBalance(input AdjustBalanceInput) (*walletdomain.Account, *walletdomain.Transaction, error)
	CreditInTransaction(tx Transaction, input CreditInput) (*walletdomain.Account, *walletdomain.Transaction, error)
	ApplyRechargePayment(tx Transaction, recharge *walletdomain.RechargeOrder) (*walletdomain.Transaction, error)
	ApplyOrderBalance(tx Transaction, input OrderBalanceInput) (money.Amount, error)
	ReleaseOrderBalance(tx Transaction, input OrderReleaseInput, claim ReleaseClaim) (money.Amount, error)
}

// ReleaseClaim atomically clears the order-side wallet allocation before the
// wallet credit is written. Returning false means another attempt already won.
type ReleaseClaim func(now time.Time) (bool, error)

package contract

import (
	"time"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"
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
	ListTransactions(filter TransactionListFilter) ([]walletdomain.Transaction, int64, error)

	CreateRechargeOrder(order *walletdomain.RechargeOrder) error
	UpdateRechargeOrder(order *walletdomain.RechargeOrder) error
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
	RecoverReleasedOrderBalance(tx Transaction, input OrderBalanceRecoveryInput) (money.Amount, error)
}

// ReleaseClaim atomically clears the order-side wallet allocation before the
// wallet credit is written. Returning false means another attempt already won.
type ReleaseClaim func(now time.Time) (bool, error)

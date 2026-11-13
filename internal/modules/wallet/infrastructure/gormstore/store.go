package gormstore

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/persistence/gormutil"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	memberlevelgormstore "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
)

type Store struct{ db *gorm.DB }

var (
	_ walletcontract.Repository = (*Store)(nil)
	_ walletcontract.UnitOfWork = (*Store)(nil)
)

func New(db *gorm.DB) *Store { return &Store{db: db} }

func (s *Store) Bind(tx *gorm.DB) *Store {
	if tx == nil {
		return s
	}
	return New(tx)
}

type transaction struct{ wallets *Store }

func (tx transaction) Wallets() walletcontract.Repository { return tx.wallets }

func UseTransaction(tx *gorm.DB) walletcontract.Transaction {
	if tx == nil {
		return nil
	}
	return transaction{wallets: New(tx)}
}

func (s *Store) WithinTransaction(fn func(walletcontract.Transaction) error) error {
	if s == nil || s.db == nil {
		return walletcontract.ErrTransactionRequired
	}
	if fn == nil {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return fn(UseTransaction(tx))
	})
}

func (s *Store) GetAccountByUserID(userID uint) (*walletdomain.Account, error) {
	if userID == 0 {
		return nil, nil
	}
	var account walletdomain.Account
	if err := s.db.Where("user_id = ? AND deleted_at IS NULL", userID).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}

func (s *Store) GetAccountByUserIDForUpdate(userID uint) (*walletdomain.Account, error) {
	if userID == 0 {
		return nil, nil
	}
	var account walletdomain.Account
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}

func (s *Store) GetAccountsByUserIDs(userIDs []uint) ([]walletdomain.Account, error) {
	if len(userIDs) == 0 {
		return []walletdomain.Account{}, nil
	}
	var accounts []walletdomain.Account
	if err := s.db.Where("user_id IN ? AND deleted_at IS NULL", userIDs).Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

func (s *Store) CreateAccount(account *walletdomain.Account) error {
	return s.db.Create(account).Error
}

func (s *Store) UpdateAccount(account *walletdomain.Account) error {
	return s.db.Save(account).Error
}

func (s *Store) ListAccounts(filter walletcontract.AccountListFilter) ([]walletdomain.Account, int64, error) {
	query := s.db.Model(&walletdomain.Account{}).Where("deleted_at IS NULL")
	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var accounts []walletdomain.Account
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).Order("id desc").Find(&accounts).Error; err != nil {
		return nil, 0, err
	}
	return accounts, total, nil
}

func (s *Store) CreateTransaction(transaction *walletdomain.Transaction) error {
	return s.db.Create(transaction).Error
}

// CountOrderTransactionsByType 统计订单在某一流水类型下已经发生的次数，
// 用于给订单余额分配生成轮次幂等键。
func (s *Store) CountOrderTransactionsByType(orderID uint, transactionType string) (int64, error) {
	transactionType = strings.TrimSpace(transactionType)
	if orderID == 0 || transactionType == "" {
		return 0, nil
	}
	var count int64
	if err := s.db.Model(&walletdomain.Transaction{}).
		Where("order_id = ? AND type = ? AND deleted_at IS NULL", orderID, transactionType).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) GetTransactionByReference(reference string) (*walletdomain.Transaction, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return nil, nil
	}
	var transaction walletdomain.Transaction
	if err := s.db.Where("reference = ? AND deleted_at IS NULL", reference).First(&transaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &transaction, nil
}

func (s *Store) ListTransactions(filter walletcontract.TransactionListFilter) ([]walletdomain.Transaction, int64, error) {
	query := s.db.Model(&walletdomain.Transaction{}).Where("deleted_at IS NULL")
	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.OrderID != 0 {
		query = query.Where("order_id = ?", filter.OrderID)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Direction != "" {
		query = query.Where("direction = ?", filter.Direction)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var transactions []walletdomain.Transaction
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).Order("id desc").Find(&transactions).Error; err != nil {
		return nil, 0, err
	}
	return transactions, total, nil
}

func (s *Store) CreateRechargeOrder(order *walletdomain.RechargeOrder) error {
	return s.db.Create(order).Error
}

func (s *Store) UpdateRechargeOrder(order *walletdomain.RechargeOrder) error {
	return s.db.Save(order).Error
}

// DecrementTotalRecharged 使用当前仓储的事务连接扣回用户累计充值。
func (s *Store) DecrementTotalRecharged(userID uint, amount decimal.Decimal) error {
	return memberlevelgormstore.DecrementTotalRecharged(s.db, userID, amount)
}

func (s *Store) GetRechargeOrderByRechargeNo(userID uint, rechargeNo string) (*walletdomain.RechargeOrder, error) {
	if userID == 0 || strings.TrimSpace(rechargeNo) == "" {
		return nil, nil
	}
	var order walletdomain.RechargeOrder
	if err := s.db.Where(
		"user_id = ? AND recharge_no = ? AND deleted_at IS NULL",
		userID, strings.TrimSpace(rechargeNo),
	).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (s *Store) GetRechargeOrderByPaymentID(paymentID uint) (*walletdomain.RechargeOrder, error) {
	if paymentID == 0 {
		return nil, nil
	}
	var order walletdomain.RechargeOrder
	if err := s.db.Where("payment_id = ? AND deleted_at IS NULL", paymentID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (s *Store) GetRechargeOrderByPaymentIDAndUser(paymentID, userID uint) (*walletdomain.RechargeOrder, error) {
	if paymentID == 0 || userID == 0 {
		return nil, nil
	}
	var order walletdomain.RechargeOrder
	if err := s.db.Where(
		"payment_id = ? AND user_id = ? AND deleted_at IS NULL", paymentID, userID,
	).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (s *Store) GetRechargeOrderByPaymentIDForUpdate(paymentID uint) (*walletdomain.RechargeOrder, error) {
	if paymentID == 0 {
		return nil, nil
	}
	var order walletdomain.RechargeOrder
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("payment_id = ? AND deleted_at IS NULL", paymentID).
		First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func applyRechargeFilters(query *gorm.DB, filter walletcontract.RechargeListFilter, includeStatus bool) *gorm.DB {
	query = query.Where("wallet_recharge_orders.deleted_at IS NULL")
	if filter.RechargeNo != "" {
		query = query.Where("wallet_recharge_orders.recharge_no LIKE ?", "%"+filter.RechargeNo+"%")
	}
	if filter.UserID != 0 {
		query = query.Where("wallet_recharge_orders.user_id = ?", filter.UserID)
	}
	if filter.UserKeyword != "" {
		like := "%" + filter.UserKeyword + "%"
		query = query.Joins("LEFT JOIN users ON users.id = wallet_recharge_orders.user_id").
			Where("(users.email LIKE ? OR users.display_name LIKE ?)", like, like)
	}
	if filter.PaymentID != 0 {
		query = query.Where("wallet_recharge_orders.payment_id = ?", filter.PaymentID)
	}
	if filter.ChannelID != 0 {
		query = query.Where("wallet_recharge_orders.channel_id = ?", filter.ChannelID)
	}
	if filter.ProviderType != "" {
		query = query.Where("wallet_recharge_orders.provider_type = ?", filter.ProviderType)
	}
	if filter.ChannelType != "" {
		query = query.Where("wallet_recharge_orders.channel_type = ?", filter.ChannelType)
	}
	if includeStatus && filter.Status != "" {
		query = query.Where("wallet_recharge_orders.status = ?", filter.Status)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("wallet_recharge_orders.created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("wallet_recharge_orders.created_at <= ?", *filter.CreatedTo)
	}
	if filter.PaidFrom != nil {
		query = query.Where("wallet_recharge_orders.paid_at >= ?", *filter.PaidFrom)
	}
	if filter.PaidTo != nil {
		query = query.Where("wallet_recharge_orders.paid_at <= ?", *filter.PaidTo)
	}
	return query
}

func (s *Store) ListRechargeOrdersAdmin(filter walletcontract.RechargeListFilter) ([]walletdomain.RechargeOrder, int64, error) {
	query := applyRechargeFilters(s.db.Model(&walletdomain.RechargeOrder{}), filter, true)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var orders []walletdomain.RechargeOrder
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).
		Order("wallet_recharge_orders.id DESC").
		Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (s *Store) StatsRechargeOrders(filter walletcontract.RechargeListFilter) (map[string]int64, error) {
	query := s.db.Model(&walletdomain.RechargeOrder{}).
		Where("wallet_recharge_orders.deleted_at IS NULL")
	if filter.RechargeNo != "" {
		query = query.Where("wallet_recharge_orders.recharge_no LIKE ?", "%"+filter.RechargeNo+"%")
	}
	if filter.UserID != 0 {
		query = query.Where("wallet_recharge_orders.user_id = ?", filter.UserID)
	}
	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	if err := query.Select("wallet_recharge_orders.status as status, COUNT(*) as count").
		Group("wallet_recharge_orders.status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, item := range rows {
		result[item.Status] = item.Count
	}
	return result, nil
}

func (s *Store) GetRechargeOrdersByPaymentIDs(paymentIDs []uint) ([]walletdomain.RechargeOrder, error) {
	if len(paymentIDs) == 0 {
		return []walletdomain.RechargeOrder{}, nil
	}
	var orders []walletdomain.RechargeOrder
	if err := s.db.Where("payment_id IN ? AND deleted_at IS NULL", paymentIDs).Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

func (s *Store) GetRechargeOrderByID(id uint) (*walletdomain.RechargeOrder, error) {
	if id == 0 {
		return nil, nil
	}
	var order walletdomain.RechargeOrder
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (s *Store) ListRechargeRefunds(id uint) ([]walletdomain.Transaction, error) {
	rows := []walletdomain.Transaction{}
	err := s.db.Where("recharge_order_id = ? AND type = ? AND deleted_at IS NULL", id, "admin_refund").Order("id DESC").Find(&rows).Error
	return rows, err
}

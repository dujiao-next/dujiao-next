package application

import (
	"time"

	"github.com/dujiao-next/internal/constants"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// ApplyWalletBalance coordinates the order allocation update with the
// wallet debit in the caller's transaction. Wallet owns account movements;
// Order owns wallet_paid_amount and online_paid_amount.
func ApplyWalletBalance(
	wallets *walletapp.Service,
	tx ordercontract.Transaction,
	order *orderdomain.Order,
	useBalance bool,
) (decimal.Decimal, error) {
	if tx == nil {
		return decimal.Zero, ErrOrderUpdateFailed
	}
	if order == nil {
		return decimal.Zero, ErrOrderNotFound
	}
	if wallets == nil {
		return decimal.Zero, walletcontract.ErrAccountNotFound
	}

	amount, err := wallets.ApplyOrderBalance(tx.Wallets(), walletcontract.OrderBalanceInput{
		OrderID:          order.ID,
		UserID:           order.UserID,
		TotalAmount:      order.TotalAmount,
		WalletPaidAmount: order.WalletPaidAmount,
		Currency:         order.Currency,
		UseBalance:       useBalance,
	})
	if err != nil {
		return decimal.Zero, err
	}
	deducted := amount.Decimal.Round(2)
	if !useBalance || order.WalletPaidAmount.Decimal.GreaterThan(decimal.Zero) || deducted.LessThanOrEqual(decimal.Zero) {
		return deducted, nil
	}

	now := time.Now()
	onlineAmount := normalizeOrderAmount(order.TotalAmount.Decimal.Sub(deducted))
	if err := tx.Orders().UpdateFields(order.ID, map[string]interface{}{
		"wallet_paid_amount": money.FromDecimal(deducted),
		"online_paid_amount": money.FromDecimal(onlineAmount),
		"updated_at":         now,
	}); err != nil {
		return decimal.Zero, ErrOrderUpdateFailed
	}
	order.WalletPaidAmount = money.FromDecimal(deducted)
	order.OnlinePaidAmount = money.FromDecimal(onlineAmount)
	order.UpdatedAt = now
	return deducted, nil
}

// ReleaseWalletBalance atomically claims the order allocation before
// asking Wallet to credit it back. This prevents duplicate credits while
// keeping the aggregate write in the owning context.
func ReleaseWalletBalance(
	wallets *walletapp.Service,
	tx ordercontract.Transaction,
	order *orderdomain.Order,
	transactionType string,
	remark string,
) (decimal.Decimal, error) {
	if tx == nil {
		return decimal.Zero, ErrOrderUpdateFailed
	}
	if order == nil || order.UserID == 0 || order.WalletPaidAmount.Decimal.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, nil
	}
	if wallets == nil {
		return decimal.Zero, walletcontract.ErrAccountNotFound
	}

	claimed := false
	amount, err := wallets.ReleaseOrderBalance(
		tx.Wallets(),
		walletcontract.OrderReleaseInput{
			OrderID:          order.ID,
			UserID:           order.UserID,
			WalletPaidAmount: order.WalletPaidAmount,
			TotalAmount:      order.TotalAmount,
			Currency:         order.Currency,
			TransactionType:  transactionType,
			Remark:           remark,
		},
		func(now time.Time) (bool, error) {
			affected, updateErr := tx.Orders().UpdateFieldsWhereWalletPaid(order.ID, map[string]interface{}{
				"wallet_paid_amount": money.FromDecimal(decimal.Zero),
				"online_paid_amount": money.FromDecimal(order.TotalAmount.Decimal.Round(2)),
				"updated_at":         now,
			})
			if updateErr != nil {
				return false, ErrOrderUpdateFailed
			}
			claimed = affected > 0
			return claimed, nil
		},
	)
	if err != nil {
		return decimal.Zero, err
	}
	if claimed {
		order.WalletPaidAmount = money.FromDecimal(decimal.Zero)
		order.OnlinePaidAmount = money.FromDecimal(order.TotalAmount.Decimal.Round(2))
		order.UpdatedAt = time.Now()
	}
	return amount.Decimal.Round(2), nil
}

// RecoverReleasedWalletBalance restores a mixed-payment allocation that was
// returned by failed/expired processing before a late success callback won.
// The wallet debit and order allocation update share the caller's transaction.
func RecoverReleasedWalletBalance(
	wallets *walletapp.Service,
	tx ordercontract.Transaction,
	order *orderdomain.Order,
	expectedAmount money.Amount,
	snapshotKnown bool,
) (decimal.Decimal, error) {
	if tx == nil {
		return decimal.Zero, ErrOrderUpdateFailed
	}
	if order == nil {
		return decimal.Zero, ErrOrderNotFound
	}
	if order.UserID == 0 || order.WalletPaidAmount.Decimal.GreaterThan(decimal.Zero) {
		return order.WalletPaidAmount.Decimal.Round(2), nil
	}
	if wallets == nil {
		return decimal.Zero, walletcontract.ErrAccountNotFound
	}

	amount, err := wallets.RecoverReleasedOrderBalance(tx.Wallets(), walletcontract.OrderBalanceRecoveryInput{
		OrderID:                order.ID,
		UserID:                 order.UserID,
		TotalAmount:            order.TotalAmount,
		ExpectedAmount:         expectedAmount,
		SnapshotKnown:          snapshotKnown,
		Currency:               order.Currency,
		ReleaseTransactionType: constants.WalletTxnTypeOrderRefund,
		Remark:                 "支付成功后重新扣回已退订单余额",
	})
	if err != nil {
		return decimal.Zero, err
	}
	recovered := amount.Decimal.Round(2)
	if recovered.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, nil
	}

	now := time.Now()
	onlineAmount := normalizeOrderAmount(order.TotalAmount.Decimal.Sub(recovered))
	if err := tx.Orders().UpdateFields(order.ID, map[string]interface{}{
		"wallet_paid_amount": money.FromDecimal(recovered),
		"online_paid_amount": money.FromDecimal(onlineAmount),
		"updated_at":         now,
	}); err != nil {
		return decimal.Zero, ErrOrderUpdateFailed
	}
	order.WalletPaidAmount = money.FromDecimal(recovered)
	order.OnlinePaidAmount = money.FromDecimal(onlineAmount)
	order.UpdatedAt = now
	return recovered, nil
}

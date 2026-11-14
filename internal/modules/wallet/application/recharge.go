package application

import (
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"

	settingscontract "github.com/dujiao-next/internal/modules/settings/contract"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
)

func (s *Service) ApplyRechargePayment(
	tx walletcontract.Transaction,
	recharge *walletdomain.RechargeOrder,
) (*walletdomain.Transaction, error) {
	if tx == nil {
		return nil, walletcontract.ErrTransactionRequired
	}
	if recharge == nil || recharge.ID == 0 || recharge.UserID == 0 {
		return nil, walletcontract.ErrRechargeNotFound
	}
	if recharge.Amount.Decimal.Round(2).LessThanOrEqual(decimal.Zero) {
		return nil, walletcontract.ErrInvalidAmount
	}
	_, transaction, err := s.CreditInTransaction(tx, walletcontract.CreditInput{
		UserID: recharge.UserID, Amount: recharge.Amount, Currency: recharge.Currency,
		Type:      constants.WalletTxnTypeRecharge,
		Reference: fmt.Sprintf("recharge:%d:success", recharge.ID),
		Remark:    cleanRemark(recharge.Remark, "在线充值到账"),
	})
	return transaction, err
}

// RefundRecharge 在同一事务中扣回用户钱包余额并记录手动退款。
func (s *Service) RefundRecharge(input walletcontract.RechargeRefundInput) error {
	amount := input.Amount.Decimal
	requestID := strings.TrimSpace(input.RequestID)
	if input.RechargeID == 0 || input.AdminID == 0 || len(requestID) < 16 || len(requestID) > 64 ||
		!amount.GreaterThan(decimal.Zero) || !amount.Equal(amount.Round(2)) || len([]rune(input.Remark)) > 255 {
		return walletcontract.ErrInvalidAmount
	}
	if s.transactions == nil {
		return walletcontract.ErrTransactionRequired
	}
	recharge, err := s.repository.GetRechargeOrderByID(input.RechargeID)
	if err != nil {
		return err
	}
	if recharge == nil {
		return walletcontract.ErrRechargeNotFound
	}
	refundConfig, err := s.getRechargeRefundConfig()
	if err != nil {
		return err
	}
	var refundID uint
	var refundStatus string
	err = s.transactions.WithinTransaction(func(tx walletcontract.Transaction) error {
		repo := tx.Wallets()
		locked, err := repo.GetRechargeOrderByPaymentIDForUpdate(recharge.PaymentID)
		if err != nil {
			return err
		}
		if locked == nil {
			return walletcontract.ErrRechargeNotFound
		}
		reference := fmt.Sprintf("recharge:%d:refund:%s", locked.ID, requestID)
		existing, err := repo.GetTransactionByReference(reference)
		if err != nil {
			return err
		}
		if existing != nil {
			if !existing.Amount.Decimal.Equal(amount) || existing.Remark != strings.TrimSpace(input.Remark) || existing.PaymentFeeRefunded != input.PaymentFeeRefunded {
				return walletcontract.ErrInvalidAmount
			}
			return nil
		}
		if locked.Status != constants.WalletRechargeStatusSuccess &&
			locked.Status != constants.WalletRechargeStatusPartiallyRefunded && locked.Status != constants.WalletRechargeStatusRefunded {
			return walletcontract.ErrRechargeStatusInvalid
		}
		now := time.Now()
		if settingscontract.IsOrderRefundWindowExpired(locked.CreatedAt, locked.PaidAt, refundConfig.MaxRefundDays, now) {
			return walletcontract.ErrRechargeRefundExpired
		}
		refunded := locked.RefundedAmount.Decimal.Add(amount)
		if refunded.GreaterThan(locked.Amount.Decimal) {
			return walletcontract.ErrRefundExceeded
		}
		feeRefunded := decimal.Zero
		if input.PaymentFeeRefunded && input.FeeSnapshot.PaymentAmount.Decimal.IsPositive() && input.FeeSnapshot.PaymentFee.Decimal.IsPositive() {
			records, err := repo.ListRechargeRefunds(locked.ID)
			if err != nil {
				return err
			}
			principalBefore, feeBefore := decimal.Zero, decimal.Zero
			for _, record := range records {
				if record.PaymentFeeRefunded {
					principalBefore = principalBefore.Add(record.Amount.Decimal)
					feeBefore = feeBefore.Add(record.PaymentFeeRefundedAmount.Decimal)
				}
			}
			feeRefunded = money.CalculatePaymentFeeRefundAmount(input.FeeSnapshot.PaymentAmount.Decimal, input.FeeSnapshot.PaymentFee.Decimal, principalBefore, feeBefore, amount)
		}
		account, err := repo.GetAccountByUserIDForUpdate(locked.UserID)
		if err != nil {
			return err
		}
		if account == nil || account.Balance.Decimal.LessThan(amount) {
			return walletcontract.ErrInsufficientBalance
		}
		before := account.Balance
		account.Balance = money.FromDecimal(before.Decimal.Sub(amount))
		account.UpdatedAt = now
		if err := repo.UpdateAccount(account); err != nil {
			return err
		}
		transaction := &walletdomain.Transaction{
			PaymentFeeRefunded:       input.PaymentFeeRefunded,
			PaymentFeeRefundedAmount: money.FromDecimal(feeRefunded),
			UserID:                   locked.UserID, RechargeOrderID: &locked.ID, OperatorAdminID: &input.AdminID,
			Type: constants.WalletTxnTypeAdminRefund, Direction: constants.WalletTxnDirectionOut,
			Amount: input.Amount, BalanceBefore: before, BalanceAfter: account.Balance,
			Currency: locked.Currency, Reference: reference, Remark: strings.TrimSpace(input.Remark), CreatedAt: now, UpdatedAt: now,
		}
		if err := repo.CreateTransaction(transaction); err != nil {
			return err
		}
		locked.RefundedAmount = money.FromDecimal(refunded)
		locked.Status = constants.WalletRechargeStatusPartiallyRefunded
		if refunded.Equal(locked.Amount.Decimal) {
			locked.Status = constants.WalletRechargeStatusRefunded
		}
		locked.UpdatedAt = now
		if err := repo.UpdateRechargeOrder(locked); err != nil {
			return err
		}
		if err := repo.ApplyRechargeRefund(locked.UserID, amount); err != nil {
			return err
		}
		refundID, refundStatus = transaction.ID, locked.Status
		return nil
	})
	if err != nil {
		return err
	}
	// 仅在新退款事务提交后通知；幂等重试不重复发送，队列失败不回滚已完成的退款。
	if refundID > 0 && s.refundEmails != nil {
		if err := s.refundEmails.EnqueueRechargeRefundEmail(recharge.ID, refundID, refundStatus); err != nil {
			logger.Warnw("wallet_recharge_refund_enqueue_email_failed", "recharge_id", recharge.ID, "refund_id", refundID, "error", err)
		}
	}
	return nil
}

func (s *Service) ListRechargeRefunds(id uint) ([]walletdomain.Transaction, error) {
	return s.repository.ListRechargeRefunds(id)
}

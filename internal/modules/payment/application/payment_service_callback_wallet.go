package application

import (
	"strings"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	"github.com/dujiao-next/internal/constants"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
)

func (s *PaymentService) handleWalletRechargeCallback(payment *paymentdomain.Payment, status string, input PaymentCallbackInput) (*paymentdomain.Payment, error) {
	log := paymentLogger(
		"payment_id", payment.ID,
		"recharge_no", strings.TrimSpace(input.OrderNo),
		"target_status", status,
		"callback_channel_id", input.ChannelID,
		"callback_provider_ref", strings.TrimSpace(input.ProviderRef),
		"callback_currency", strings.ToUpper(strings.TrimSpace(input.Currency)),
		"callback_amount", input.Amount.String(),
	)
	if s.walletRepo == nil {
		log.Errorw("wallet_recharge_callback_wallet_repo_nil")
		return nil, ErrPaymentUpdateFailed
	}
	recharge, err := s.walletRepo.GetRechargeOrderByPaymentID(payment.ID)
	if err != nil {
		log.Errorw("wallet_recharge_callback_recharge_fetch_failed", "error", err)
		return nil, ErrPaymentUpdateFailed
	}
	if recharge == nil {
		log.Warnw("wallet_recharge_callback_recharge_not_found")
		return nil, walletcontract.ErrRechargeNotFound
	}

	if err := validateWalletCallbackPaymentFacts(payment, recharge.RechargeNo, status, input); err != nil {
		return nil, err
	}

	now := time.Now()
	updated, newlySucceeded, err := s.applyWalletRechargePaymentUpdate(payment, status, input, now)
	if err != nil {
		log.Errorw("wallet_recharge_callback_apply_failed", "error", err)
		return nil, err
	}
	log.Infow("wallet_recharge_callback_processed",
		"new_status", updated.Status,
	)
	if newlySucceeded {
		reloadedRecharge, reloadErr := s.walletRepo.GetRechargeOrderByPaymentID(payment.ID)
		if reloadErr != nil {
			log.Warnw("wallet_recharge_callback_reload_failed", "error", reloadErr)
		} else if reloadedRecharge != nil {
			recharge = reloadedRecharge
		}
		if recharge != nil {
			recharge.Status = constants.WalletRechargeStatusSuccess
			recharge.PaidAt = updated.PaidAt
			recharge.UpdatedAt = updated.UpdatedAt
		}
		if s.memberLevelSvc != nil && recharge != nil && recharge.UserID > 0 {
			if err := s.memberLevelSvc.OnRechargeCompleted(recharge.UserID, recharge.Amount.Decimal); err != nil {
				paymentLogger().Warnw("member_level_recharge_completed_failed",
					"payment_id", payment.ID,
					"user_id", recharge.UserID,
					"amount", recharge.Amount.Decimal.String(),
					"error", err,
				)
			}
		}
		s.enqueueWalletRechargeSuccessAsync(recharge, updated, log)
		s.enqueueWalletRechargeBotNotifyAsync(recharge, log)
	}
	return updated, nil
}

func validateWalletCallbackPaymentFacts(payment *paymentdomain.Payment, rechargeNo, status string, input PaymentCallbackInput) error {
	return validateCallbackPaymentFacts(payment, rechargeNo, status, input)
}

func (s *PaymentService) applyWalletRechargePaymentUpdate(payment *paymentdomain.Payment, status string, input PaymentCallbackInput, now time.Time) (*paymentdomain.Payment, bool, error) {
	paymentVal := payment
	newlySucceeded := false

	err := s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
		paymentRepo := tx.Payments()
		walletTx := tx.Wallets()
		rechargeRepo := walletTx.Wallets()

		lockedPayment, err := paymentRepo.GetByIDForUpdate(payment.ID)
		if err != nil {
			return ErrPaymentUpdateFailed
		}
		recharge, err := rechargeRepo.GetRechargeOrderByPaymentIDForUpdate(payment.ID)
		if err != nil {
			return ErrPaymentUpdateFailed
		}
		if recharge == nil {
			return walletcontract.ErrRechargeNotFound
		}
		if err := validateWalletCallbackPaymentFacts(lockedPayment, recharge.RechargeNo, status, input); err != nil {
			return err
		}
		if input.manualConfirm && recharge.Status != constants.WalletRechargeStatusPending &&
			recharge.Status != constants.WalletRechargeStatusExpired && recharge.Status != constants.WalletRechargeStatusFailed &&
			recharge.Status != constants.WalletRechargeStatusSuccess {
			return walletcontract.ErrRechargeStatusInvalid
		}
		adoptVerifiedLegacyDujiaoPayCurrency(lockedPayment, status, input)
		paymentVal = lockedPayment
		if lockedPayment.Status == constants.PaymentStatusSuccess && (!input.manualConfirm || recharge.Status == constants.WalletRechargeStatusSuccess) {
			_, err := updateCallbackMetaWithRepo(paymentRepo, lockedPayment, constants.PaymentStatusSuccess, input)
			return err
		}
		if lockedPayment.Status == status && !input.manualConfirm {
			_, err := updateCallbackMetaWithRepo(paymentRepo, lockedPayment, status, input)
			return err
		}
		if !canApplyWalletRechargeCallback(lockedPayment.Status, recharge.Status, status) {
			_, err := updateCallbackMetaWithRepo(paymentRepo, lockedPayment, lockedPayment.Status, input)
			return err
		}

		switch status {
		case constants.PaymentStatusSuccess:
			paidAt := now
			if input.PaidAt != nil {
				paidAt = *input.PaidAt
			}
			if !input.manualConfirm || lockedPayment.PaidAt == nil {
				lockedPayment.PaidAt = &paidAt
			}
		case constants.PaymentStatusExpired:
			lockedPayment.ExpiredAt = &now
		}
		canCredit := status == constants.PaymentStatusSuccess && (input.manualConfirm ||
			(recharge.Status == constants.WalletRechargeStatusPending && lockedPayment.ExpiredAt == nil &&
				(lockedPayment.Status == constants.PaymentStatusPending || lockedPayment.Status == constants.PaymentStatusInitiated)))
		if status == constants.PaymentStatusSuccess {
			if canCredit {
				lockedPayment.ExceptionCode = ""
			} else {
				lockedPayment.ExceptionCode = constants.PaymentExceptionClosedOrderSucceeded
			}
		}
		lockedPayment.Status = status
		lockedPayment.CallbackAt = &now
		lockedPayment.UpdatedAt = now
		if input.ProviderRef != "" {
			lockedPayment.ProviderRef = input.ProviderRef
		}
		if input.Payload != nil {
			lockedPayment.ProviderPayload = mergeProviderPayload(lockedPayment.ProviderPayload, input.Payload)
		}
		if err := paymentRepo.Update(lockedPayment); err != nil {
			return ErrPaymentUpdateFailed
		}
		if recharge.Status == constants.WalletRechargeStatusSuccess {
			return nil
		}

		switch status {
		case constants.PaymentStatusSuccess:
			// 渠道到账与钱包入账分开：关闭的充值单等待管理员确认。
			if !canCredit {
				return nil
			}
			if s.walletSvc == nil {
				return walletcontract.ErrAccountNotFound
			}
			if _, err := s.walletSvc.ApplyRechargePayment(walletTx, recharge); err != nil {
				return err
			}
			newlySucceeded = true
			recharge.Status = constants.WalletRechargeStatusSuccess
			paidAt := now
			if lockedPayment.PaidAt != nil {
				paidAt = *lockedPayment.PaidAt
			}
			recharge.PaidAt = &paidAt
		case constants.PaymentStatusFailed:
			recharge.Status = constants.WalletRechargeStatusFailed
		case constants.PaymentStatusExpired:
			recharge.Status = constants.WalletRechargeStatusExpired
		default:
			recharge.Status = constants.WalletRechargeStatusPending
		}
		recharge.UpdatedAt = now
		if err := rechargeRepo.UpdateRechargeOrder(recharge); err != nil {
			return ErrPaymentUpdateFailed
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return paymentVal, newlySucceeded, nil
}

func canApplyWalletRechargeCallback(paymentStatus string, rechargeStatus string, targetStatus string) bool {
	// 成功回调记录渠道实际到账；是否给钱包入账由事务内的充值状态决定。
	if targetStatus == constants.PaymentStatusSuccess {
		return true
	}
	// 非成功回调不允许改变任何终态，避免 expired/failed/success 被回调串扰重开。
	if paymentStatus == constants.PaymentStatusSuccess || rechargeStatus == constants.WalletRechargeStatusSuccess {
		return false
	}
	if paymentStatus == constants.PaymentStatusFailed || rechargeStatus == constants.WalletRechargeStatusFailed {
		return false
	}
	if paymentStatus == constants.PaymentStatusExpired || rechargeStatus == constants.WalletRechargeStatusExpired {
		return false
	}
	return true
}

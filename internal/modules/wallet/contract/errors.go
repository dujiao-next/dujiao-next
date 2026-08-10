package contract

import "errors"

var (
	ErrInvalidAmount           = errors.New("wallet invalid amount")
	ErrInsufficientBalance     = errors.New("wallet insufficient balance")
	ErrAccountNotFound         = errors.New("wallet account not found")
	ErrAccountCreateFailed     = errors.New("wallet account create failed")
	ErrAccountUpdateFailed     = errors.New("wallet account update failed")
	ErrTransactionCreateFailed = errors.New("wallet transaction create failed")
	ErrRefundExceeded          = errors.New("wallet refund exceeded")
	ErrNotSupportedForGuest    = errors.New("wallet not supported for guest")
	ErrRechargeNotFound        = errors.New("wallet recharge not found")
	ErrRechargeStatusInvalid   = errors.New("wallet recharge status invalid")
	ErrOnlyPaymentRequired     = errors.New("wallet only payment required")
	ErrTransactionRequired     = errors.New("wallet transaction required")
	ErrBalanceRecoveryRequired = errors.New("wallet balance recovery requires manual reconciliation")
)

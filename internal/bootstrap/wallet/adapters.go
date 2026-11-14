package walletbootstrap

import (
	"errors"
	"fmt"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	wallettransport "github.com/dujiao-next/internal/modules/wallet/transport/http"
)

// walletTransportAdapter composes wallet use cases with payment capabilities
// needed by the wallet HTTP surface.
type walletTransportAdapter struct {
	wallets  *walletapp.Service
	payments *paymentapp.PaymentService
}

func (a walletTransportAdapter) GetAccount(userID uint) (*walletdomain.Account, error) {
	account, err := a.wallets.GetAccount(userID)
	return account, mapWalletTransportError(err)
}

func (a walletTransportAdapter) ListTransactions(userID uint, page, pageSize int) ([]walletdomain.Transaction, int64, error) {
	transactions, total, err := a.wallets.ListTransactions(walletcontract.TransactionListFilter{
		Page: page, PageSize: pageSize, UserID: userID,
	})
	return transactions, total, mapWalletTransportError(err)
}

func (a walletTransportAdapter) ListAdminTransactions(userID uint, page, pageSize int, typ, direction string) ([]walletdomain.Transaction, int64, error) {
	transactions, total, err := a.wallets.ListTransactions(walletcontract.TransactionListFilter{
		Page: page, PageSize: pageSize, UserID: userID, Type: typ, Direction: direction,
	})
	return transactions, total, mapWalletTransportError(err)
}

func (a walletTransportAdapter) ListRechargeOrdersAdmin(filter wallettransport.AdminRechargeListFilter) ([]walletdomain.RechargeOrder, int64, error) {
	orders, total, err := a.wallets.ListRechargeOrdersAdmin(walletcontract.RechargeListFilter{
		Page:         filter.Page,
		PageSize:     filter.PageSize,
		RechargeNo:   filter.RechargeNo,
		UserID:       filter.UserID,
		UserKeyword:  filter.UserKeyword,
		PaymentID:    filter.PaymentID,
		ChannelID:    filter.ChannelID,
		ProviderType: filter.ProviderType,
		ChannelType:  filter.ChannelType,
		Status:       filter.Status,
		CreatedFrom:  filter.CreatedFrom,
		CreatedTo:    filter.CreatedTo,
		PaidFrom:     filter.PaidFrom,
		PaidTo:       filter.PaidTo,
	})
	return orders, total, mapWalletTransportError(err)
}

func (a walletTransportAdapter) AdminAdjustBalance(input wallettransport.AdjustBalanceInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	account, txn, err := a.wallets.AdminAdjustBalance(walletcontract.AdjustBalanceInput{
		UserID:          input.UserID,
		OperatorAdminID: input.OperatorAdminID,
		Delta:           input.Delta,
		Currency:        input.Currency,
		Remark:          input.Remark,
	})
	return account, txn, mapWalletTransportError(err)
}

func (a walletTransportAdapter) ListUserRechargeOrders(userID uint, page, pageSize int, status, rechargeNo string) ([]walletdomain.RechargeOrder, int64, error) {
	orders, total, err := a.wallets.ListUserRechargeOrders(userID, page, pageSize, status, rechargeNo)
	return orders, total, mapWalletTransportError(err)
}

func (a walletTransportAdapter) StatsUserRechargeOrders(userID uint, rechargeNo string) (map[string]int64, error) {
	stats, err := a.wallets.StatsUserRechargeOrders(userID, rechargeNo)
	return stats, mapWalletTransportError(err)
}

func (a walletTransportAdapter) GetRechargeOrderByRechargeNo(userID uint, rechargeNo string) (*walletdomain.RechargeOrder, error) {
	order, err := a.wallets.GetRechargeOrderByRechargeNo(userID, rechargeNo)
	return order, mapWalletTransportError(err)
}

func (a walletTransportAdapter) GetRechargeOrderByPaymentIDAndUser(paymentID uint, userID uint) (*walletdomain.RechargeOrder, error) {
	order, err := a.wallets.GetRechargeOrderByPaymentIDAndUser(paymentID, userID)
	return order, mapWalletTransportError(err)
}

func (a walletTransportAdapter) GetAvailableWalletRechargeChannels(amount money.Amount, user *userdomain.User) ([]map[string]interface{}, error) {
	channels, err := a.payments.GetAvailableChannels(paymentapp.AvailablePaymentChannelFilter{
		TargetAmount: &amount, User: user, PaymentType: constants.PaymentTypeWallet,
	})
	return channels, mapWalletTransportError(err)
}

func (a walletTransportAdapter) CreateWalletRechargePayment(input wallettransport.CreateRechargePaymentInput) (*wallettransport.CreateRechargePaymentResult, error) {
	result, err := a.payments.CreateWalletRechargePayment(paymentapp.CreateWalletRechargePaymentInput{
		UserID: input.UserID, ChannelID: input.ChannelID, Amount: input.Amount, Currency: input.Currency,
		Remark: input.Remark, ClientIP: input.ClientIP, Context: input.Context, RequestScheme: input.RequestScheme,
	})
	if err != nil {
		return nil, mapWalletTransportError(err)
	}
	return &wallettransport.CreateRechargePaymentResult{Recharge: result.Recharge, Payment: result.Payment}, nil
}

func (a walletTransportAdapter) GetPayment(id uint) (*paymentdomain.Payment, error) {
	payment, err := a.payments.GetPayment(id)
	return payment, mapWalletTransportError(err)
}

func (a walletTransportAdapter) CapturePayment(input wallettransport.CapturePaymentInput) (*paymentdomain.Payment, error) {
	payment, err := a.payments.CapturePayment(paymentapp.CapturePaymentInput{PaymentID: input.PaymentID, Context: input.Context})
	return payment, mapWalletTransportError(err)
}

func mapWalletTransportError(err error) error {
	if err == nil {
		return nil
	}
	for _, mapping := range []struct {
		source error
		target error
	}{
		{walletcontract.ErrInvalidAmount, wallettransport.ErrInvalidAmount},
		{walletcontract.ErrRefundExceeded, wallettransport.ErrRechargeRefundExceeded},
		{walletcontract.ErrRechargeStatusInvalid, wallettransport.ErrRechargeStatusInvalid},
		{walletcontract.ErrRechargeRefundExpired, wallettransport.ErrRechargeRefundExpired},
		{walletcontract.ErrInsufficientBalance, wallettransport.ErrInsufficientBalance},
		{walletcontract.ErrNotSupportedForGuest, wallettransport.ErrNotSupportedForGuest},
		{walletcontract.ErrRechargeNotFound, wallettransport.ErrRechargeNotFound},
		{paymentapp.ErrPaymentInvalid, wallettransport.ErrPaymentInvalid},
		{paymentapp.ErrPaymentNotFound, wallettransport.ErrPaymentNotFound},
		{orderapp.ErrOrderNotFound, wallettransport.ErrOrderNotFound},
		{orderapp.ErrOrderStatusInvalid, wallettransport.ErrOrderStatusInvalid},
		{paymentapp.ErrPaymentChannelNotFound, wallettransport.ErrPaymentChannelNotFound},
		{paymentapp.ErrPaymentChannelInactive, wallettransport.ErrPaymentChannelInactive},
		{paymentapp.ErrPaymentProviderNotSupported, wallettransport.ErrPaymentProviderNotSupported},
		{paymentapp.ErrPaymentChannelConfigInvalid, wallettransport.ErrPaymentChannelConfigInvalid},
		{paymentapp.ErrPaymentGatewayRequestFailed, wallettransport.ErrPaymentGatewayRequestFailed},
		{paymentapp.ErrPaymentGatewayResponseInvalid, wallettransport.ErrPaymentGatewayResponseInvalid},
		{paymentapp.ErrPaymentCurrencyMismatch, wallettransport.ErrPaymentCurrencyMismatch},
		{paymentapp.ErrPaymentChannelNotAllowedForProduct, wallettransport.ErrPaymentChannelNotAllowedProduct},
		{paymentapp.ErrPaymentChannelNotAllowedForRecharge, wallettransport.ErrPaymentChannelNotAllowedRecharge},
		{walletcontract.ErrOnlyPaymentRequired, wallettransport.ErrWalletOnlyPaymentRequired},
		{paymentapp.ErrPaymentStatusInvalid, wallettransport.ErrPaymentStatusInvalid},
		{paymentapp.ErrPaymentAmountMismatch, wallettransport.ErrPaymentAmountMismatch},
	} {
		if errors.Is(err, mapping.source) {
			return fmt.Errorf("%w: %v", mapping.target, err)
		}
	}
	return err
}

// GetRechargeDetail 将后台充值详情查询转交支付服务，返回充值单及关联支付记录。
func (a walletTransportAdapter) GetRechargeDetail(id uint) (*walletdomain.RechargeOrder, *paymentdomain.Payment, error) {
	return a.payments.GetWalletRechargeDetail(id)
}

// IsRechargeRefundWindowExpired 使用订单退款配置判断充值单的退款期限。
func (a walletTransportAdapter) IsRechargeRefundWindowExpired(recharge *walletdomain.RechargeOrder) (bool, error) {
	return a.wallets.IsRechargeRefundWindowExpired(recharge)
}

func (a walletTransportAdapter) ConfirmRecharge(input wallettransport.ConfirmRechargeInput) error {
	_, err := a.payments.ManualConfirmWalletRecharge(paymentapp.ManualConfirmWalletRechargeInput{RechargeID: input.RechargeID, PaymentID: input.PaymentID, AdminID: input.AdminID, ProviderRef: input.ProviderRef, Remark: input.Remark})
	return err
}

// RefundRecharge 读取支付手续费快照后登记充值退款，所有快照查询均在钱包事务开始前完成。
func (a walletTransportAdapter) RefundRecharge(input wallettransport.RechargeRefundInput) error {
	refund := walletcontract.RechargeRefundInput{
		PaymentFeeRefunded: input.PaymentFeeRefunded, RechargeID: input.RechargeID, AdminID: input.AdminID, Amount: input.Amount, Remark: input.Remark, RequestID: input.RequestID,
	}
	if input.PaymentFeeRefunded {
		snapshot, err := a.payments.GetWalletRechargeRefundFeeSnapshot(input.RechargeID)
		if err != nil {
			return mapWalletTransportError(err)
		}
		refund.FeeSnapshot = snapshot
	}
	return mapWalletTransportError(a.wallets.RefundRecharge(refund))
}
func (a walletTransportAdapter) ListRechargeRefunds(id uint) ([]walletdomain.Transaction, error) {
	return a.wallets.ListRechargeRefunds(id)
}

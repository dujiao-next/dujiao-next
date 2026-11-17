package presenter

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	paymentpresenter "github.com/dujiao-next/internal/modules/payment/transport/presenter"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
)

// WalletAccountResp 钱包账户响应
type WalletAccountResp struct {
	Balance money.Amount `json:"balance"`
}

// NewWalletAccountResp 从 walletdomain.Account 构造响应
func NewWalletAccountResp(a *walletdomain.Account) WalletAccountResp {
	return WalletAccountResp{
		Balance: a.Balance,
	}
}

// WalletTransactionResp 钱包流水响应
type WalletTransactionResp struct {
	ID           uint         `json:"id"`
	Type         string       `json:"type"`
	Direction    string       `json:"direction"`
	Amount       money.Amount `json:"amount"`
	BalanceAfter money.Amount `json:"balance_after"`
	Remark       string       `json:"remark"`
	CreatedAt    time.Time    `json:"created_at"`
}

// NewWalletTransactionResp 从 walletdomain.Transaction 构造响应
func NewWalletTransactionResp(t *walletdomain.Transaction) WalletTransactionResp {
	return WalletTransactionResp{
		ID:           t.ID,
		Type:         t.Type,
		Direction:    t.Direction,
		Amount:       t.Amount,
		BalanceAfter: t.BalanceAfter,
		Remark:       t.Remark,
		CreatedAt:    t.CreatedAt,
	}
	// 排除：UserID、Currency、BalanceBefore、Reference、UpdatedAt
}

// NewWalletTransactionRespList 批量转换钱包流水
func NewWalletTransactionRespList(txns []walletdomain.Transaction) []WalletTransactionResp {
	result := make([]WalletTransactionResp, 0, len(txns))
	for i := range txns {
		result = append(result, NewWalletTransactionResp(&txns[i]))
	}
	return result
}

// WalletRechargeResp 钱包充值单响应
type WalletRechargeResp struct {
	ID             uint         `json:"id"`
	RechargeNo     string       `json:"recharge_no"`
	Amount         money.Amount `json:"amount"`
	PayableAmount  money.Amount `json:"payable_amount"`
	FeeRate        money.Amount `json:"fee_rate"`
	FeeAmount      money.Amount `json:"fee_amount"`
	RefundedAmount money.Amount `json:"refunded_amount"`
	Currency       string       `json:"currency"`
	Status         string       `json:"status"`
	Remark         string       `json:"remark"`
	PaidAt         *time.Time   `json:"paid_at"`
	CreatedAt      time.Time    `json:"created_at"`
}

// NewWalletRechargeResp 从 walletdomain.RechargeOrder 构造响应
func NewWalletRechargeResp(r *walletdomain.RechargeOrder) WalletRechargeResp {
	return WalletRechargeResp{
		ID:             r.ID,
		RechargeNo:     r.RechargeNo,
		Amount:         r.Amount,
		PayableAmount:  r.PayableAmount,
		FeeRate:        r.FeeRate,
		FeeAmount:      r.FeeAmount,
		RefundedAmount: r.RefundedAmount,
		Currency:       r.Currency,
		Status:         r.Status,
		Remark:         r.Remark,
		PaidAt:         r.PaidAt,
		CreatedAt:      r.CreatedAt,
	}
	// 排除：UserID、PaymentID、ChannelID、ProviderType、ChannelType、InteractionMode、UpdatedAt
}

// NewWalletRechargeRespList 批量转换钱包充值单
func NewWalletRechargeRespList(orders []walletdomain.RechargeOrder) []WalletRechargeResp {
	result := make([]WalletRechargeResp, 0, len(orders))
	for i := range orders {
		result = append(result, NewWalletRechargeResp(&orders[i]))
	}
	return result
}

// WalletRechargePaymentPayload 钱包充值支付响应载荷
type WalletRechargePaymentPayload struct {
	Recharge        *WalletRechargeResp `json:"recharge,omitempty"`
	RechargeNo      string              `json:"recharge_no,omitempty"`
	RechargeStatus  string              `json:"recharge_status,omitempty"`
	Account         *WalletAccountResp  `json:"account,omitempty"`
	PaymentID       *uint               `json:"payment_id,omitempty"`
	ProviderType    string              `json:"provider_type,omitempty"`
	ChannelType     string              `json:"channel_type,omitempty"`
	InteractionMode string              `json:"interaction_mode,omitempty"`
	PayURL          string              `json:"pay_url,omitempty"`
	QRCode          string              `json:"qr_code,omitempty"`
	WalletAddress   string              `json:"wallet_address,omitempty"`
	ChainAmount     string              `json:"chain_amount,omitempty"`
	Chain           string              `json:"chain,omitempty"`
	TokenID         string              `json:"token_id,omitempty"`
	ExpiresAt       *time.Time          `json:"expires_at,omitempty"`
	Status          string              `json:"status,omitempty"`
	FeePolicy       string              `json:"fee_policy,omitempty"`
}

// NewWalletRechargePaymentPayload 构造钱包充值支付响应
func NewWalletRechargePaymentPayload(recharge *walletdomain.RechargeOrder, payment *paymentdomain.Payment, account *walletdomain.Account) WalletRechargePaymentPayload {
	p := WalletRechargePaymentPayload{}
	if recharge != nil {
		r := NewWalletRechargeResp(recharge)
		p.Recharge = &r
		p.RechargeNo = recharge.RechargeNo
		p.RechargeStatus = recharge.Status
	}
	if account != nil {
		a := NewWalletAccountResp(account)
		p.Account = &a
	}
	if payment != nil {
		p.PaymentID = &payment.ID
		p.ProviderType = payment.ProviderType
		p.ChannelType = payment.ChannelType
		p.InteractionMode = payment.InteractionMode
		p.PayURL = payment.PayURL
		p.QRCode = payment.QRCode
		p.ExpiresAt = payment.ExpiredAt
		p.Status = payment.Status
		p.FeePolicy = payment.FeePolicy
		info := paymentpresenter.ExtractCryptoWalletInfo(
			payment.ProviderType,
			payment.InteractionMode,
			payment.ProviderPayload,
		)
		p.WalletAddress = info.Address
		p.ChainAmount = info.ChainAmount
		p.Chain = info.Chain
		p.TokenID = info.TokenID
	}
	return p
	// 排除 Payment 的：OrderID、ChannelID、Amount、FeeRate、FixedFee、FeeAmount、Currency、
	// ProviderRef、GatewayOrderNo、ProviderPayload、CreatedAt、UpdatedAt、PaidAt、CallbackAt
}

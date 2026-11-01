package presenter

import (
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	"github.com/dujiao-next/internal/shared/money"
)

// CreatePaymentResp 创建支付响应
type CreatePaymentResp struct {
	FixedPayment     bool         `json:"fixed_payment"`
	OrderPaid        bool         `json:"order_paid"`
	WalletPaidAmount money.Amount `json:"wallet_paid_amount"`
	OnlinePayAmount  money.Amount `json:"online_pay_amount"`
	PayableAmount    money.Amount `json:"payable_amount"`
	// Currency 是 PayableAmount 的实际币种。换汇渠道（如 Stripe 配置了
	// target_currency+exchange_rate）下 PayableAmount 是结算币种数值，与订单币种不同，
	// 前端必须用这个字段而不是订单币种来展示 PayableAmount，否则会把结算币种数值
	// 误标成订单币种（例如把 0.11 GBP 显示成 "0.11 CNY"）。
	Currency        string       `json:"currency"`
	FeeAmount       money.Amount `json:"fee_amount"`
	FeePolicy       string       `json:"fee_policy,omitempty"`
	PaymentID       *uint        `json:"payment_id,omitempty"`
	ChannelID       *uint        `json:"channel_id,omitempty"`
	ProviderType    string       `json:"provider_type,omitempty"`
	ChannelType     string       `json:"channel_type,omitempty"`
	InteractionMode string       `json:"interaction_mode,omitempty"`
	PayURL          string       `json:"pay_url,omitempty"`
	QRCode          string       `json:"qr_code,omitempty"`
	WalletAddress   string       `json:"wallet_address,omitempty"`
	ChainAmount     string       `json:"chain_amount,omitempty"`
	Chain           string       `json:"chain,omitempty"`
	TokenID         string       `json:"token_id,omitempty"`
	ExpiresAt       *time.Time   `json:"expires_at,omitempty"`
	ChannelName     string       `json:"channel_name,omitempty"`
}

// CreatePaymentResultView 创建支付结果视图（供 HTTP 层构造响应，避免依赖 service）。
type CreatePaymentResultView struct {
	Payment          *paymentdomain.Payment
	Channel          *paymentdomain.PaymentChannel
	OrderPaid        bool
	WalletPaidAmount money.Amount
	OnlinePayAmount  money.Amount
}

// NewCreatePaymentResp 从创建支付结果视图构造响应
func NewCreatePaymentResp(result *CreatePaymentResultView) CreatePaymentResp {
	resp := CreatePaymentResp{
		OrderPaid:        result.OrderPaid,
		WalletPaidAmount: result.WalletPaidAmount,
		OnlinePayAmount:  result.OnlinePayAmount,
	}
	if result.Payment != nil {
		resp.FixedPayment = result.Payment.RequiresChainVerification
		resp.PaymentID = &result.Payment.ID
		resp.ChannelID = &result.Payment.ChannelID
		resp.ProviderType = result.Payment.ProviderType
		resp.ChannelType = result.Payment.ChannelType
		resp.InteractionMode = result.Payment.InteractionMode
		resp.PayURL = result.Payment.PayURL
		resp.QRCode = result.Payment.QRCode
		resp.PayableAmount = result.Payment.Amount
		resp.Currency = result.Payment.Currency
		resp.FeeAmount = result.Payment.FeeAmount
		resp.FeePolicy = result.Payment.FeePolicy
		resp.ExpiresAt = result.Payment.ExpiredAt
		info := ExtractCryptoWalletInfo(
			result.Payment.ProviderType,
			result.Payment.InteractionMode,
			result.Payment.ProviderPayload,
		)
		resp.WalletAddress = info.Address
		resp.ChainAmount = info.ChainAmount
		resp.Chain = info.Chain
		resp.TokenID = info.TokenID
	}
	if result.Channel != nil {
		resp.ChannelName = result.Channel.Name
	}
	if resp.FixedPayment {
		resp.PayURL, resp.QRCode, resp.WalletAddress, resp.ChainAmount, resp.Chain, resp.TokenID = "", "", "", "", "", ""
	}
	return resp
}

// LatestPaymentResp 最新待支付记录响应
type LatestPaymentResp struct {
	FixedPayment    bool         `json:"fixed_payment"`
	PaymentID       uint         `json:"payment_id"`
	OrderNo         string       `json:"order_no"`
	ChannelID       uint         `json:"channel_id"`
	ChannelName     string       `json:"channel_name,omitempty"`
	ProviderType    string       `json:"provider_type"`
	ChannelType     string       `json:"channel_type"`
	InteractionMode string       `json:"interaction_mode"`
	PayURL          string       `json:"pay_url"`
	QRCode          string       `json:"qr_code"`
	WalletAddress   string       `json:"wallet_address,omitempty"`
	ChainAmount     string       `json:"chain_amount,omitempty"`
	Chain           string       `json:"chain,omitempty"`
	TokenID         string       `json:"token_id,omitempty"`
	ExpiresAt       *time.Time   `json:"expires_at"`
	PayableAmount   money.Amount `json:"payable_amount"`
	// Currency 是 PayableAmount 的实际币种，见 CreatePaymentResp.Currency 注释。
	Currency  string       `json:"currency"`
	FeeAmount money.Amount `json:"fee_amount"`
	FeePolicy string       `json:"fee_policy,omitempty"`
}

// NewLatestPaymentResp 从 Payment + Order 构造响应
func NewLatestPaymentResp(payment *paymentdomain.Payment, orderNo string) LatestPaymentResp {
	info := ExtractCryptoWalletInfo(payment.ProviderType, payment.InteractionMode, payment.ProviderPayload)
	resp := LatestPaymentResp{
		FixedPayment:    payment.RequiresChainVerification,
		PaymentID:       payment.ID,
		OrderNo:         orderNo,
		ChannelID:       payment.ChannelID,
		ChannelName:     payment.ChannelName,
		ProviderType:    payment.ProviderType,
		ChannelType:     payment.ChannelType,
		InteractionMode: payment.InteractionMode,
		PayURL:          payment.PayURL,
		QRCode:          payment.QRCode,
		WalletAddress:   info.Address,
		ChainAmount:     info.ChainAmount,
		Chain:           info.Chain,
		TokenID:         info.TokenID,
		ExpiresAt:       payment.ExpiredAt,
		PayableAmount:   payment.Amount,
		Currency:        payment.Currency,
		FeeAmount:       payment.FeeAmount,
		FeePolicy:       payment.FeePolicy,
	}
	if resp.FixedPayment {
		resp.PayURL, resp.QRCode, resp.WalletAddress, resp.ChainAmount, resp.Chain, resp.TokenID = "", "", "", "", "", ""
	}
	return resp
	// 排除：OrderID、FeeRate、FixedFee、Currency、Status、
	// ProviderRef、GatewayOrderNo、ProviderPayload、CreatedAt、UpdatedAt、PaidAt、CallbackAt
}

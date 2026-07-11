package dto

import (
	"time"

	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/service"
)

// CreatePaymentResp 创建支付响应
type CreatePaymentResp struct {
	OrderPaid        bool                  `json:"order_paid"`
	WalletPaidAmount models.Money          `json:"wallet_paid_amount"`
	OnlinePayAmount  models.Money          `json:"online_pay_amount"`
	PaymentID        *uint                 `json:"payment_id,omitempty"`
	ChannelID        *uint                 `json:"channel_id,omitempty"`
	ProviderType     string                `json:"provider_type,omitempty"`
	ChannelType      string                `json:"channel_type,omitempty"`
	InteractionMode  string                `json:"interaction_mode,omitempty"`
	PayURL           string                `json:"pay_url,omitempty"`
	QRCode           string                `json:"qr_code,omitempty"`
	WalletAddress    string                `json:"wallet_address,omitempty"`
	ChainAmount      string                `json:"chain_amount,omitempty"`
	Chain            string                `json:"chain,omitempty"`
	TokenID          string                `json:"token_id,omitempty"`
	PaymentMethods   []CryptoPaymentMethod `json:"payment_methods,omitempty"`
	SelectedCurrency string                `json:"selected_currency,omitempty"`
	SelectedNetwork  string                `json:"selected_network,omitempty"`
	ExpiresAt        *time.Time            `json:"expires_at,omitempty"`
	ChannelName      string                `json:"channel_name,omitempty"`
}

// NewCreatePaymentResp 从 service.CreatePaymentResult 构造响应
func NewCreatePaymentResp(result *service.CreatePaymentResult) CreatePaymentResp {
	resp := CreatePaymentResp{
		OrderPaid:        result.OrderPaid,
		WalletPaidAmount: result.WalletPaidAmount,
		OnlinePayAmount:  result.OnlinePayAmount,
	}
	if result.Payment != nil {
		resp.PaymentID = &result.Payment.ID
		resp.ChannelID = &result.Payment.ChannelID
		resp.ProviderType = result.Payment.ProviderType
		resp.ChannelType = result.Payment.ChannelType
		resp.InteractionMode = result.Payment.InteractionMode
		resp.PayURL = result.Payment.PayURL
		resp.QRCode = result.Payment.QRCode
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
		resp.PaymentMethods = ExtractCryptoPaymentMethods(
			result.Payment.ProviderType,
			result.Payment.InteractionMode,
			result.Payment.ProviderPayload,
		)
		resp.SelectedCurrency, resp.SelectedNetwork = ExtractSelectedCryptoPaymentMethod(result.Payment.ProviderPayload)
	}
	if result.Channel != nil {
		resp.ChannelName = result.Channel.Name
	}
	return resp
}

// LatestPaymentResp 最新待支付记录响应
type LatestPaymentResp struct {
	PaymentID        uint                  `json:"payment_id"`
	OrderNo          string                `json:"order_no"`
	ChannelID        uint                  `json:"channel_id"`
	ChannelName      string                `json:"channel_name,omitempty"`
	ProviderType     string                `json:"provider_type"`
	ChannelType      string                `json:"channel_type"`
	InteractionMode  string                `json:"interaction_mode"`
	PayURL           string                `json:"pay_url"`
	QRCode           string                `json:"qr_code"`
	WalletAddress    string                `json:"wallet_address,omitempty"`
	ChainAmount      string                `json:"chain_amount,omitempty"`
	Chain            string                `json:"chain,omitempty"`
	TokenID          string                `json:"token_id,omitempty"`
	PaymentMethods   []CryptoPaymentMethod `json:"payment_methods,omitempty"`
	SelectedCurrency string                `json:"selected_currency,omitempty"`
	SelectedNetwork  string                `json:"selected_network,omitempty"`
	ExpiresAt        *time.Time            `json:"expires_at"`
}

// NewLatestPaymentResp 从 Payment + Order 构造响应
func NewLatestPaymentResp(payment *models.Payment, orderNo string) LatestPaymentResp {
	info := ExtractCryptoWalletInfo(payment.ProviderType, payment.InteractionMode, payment.ProviderPayload)
	currency, network := ExtractSelectedCryptoPaymentMethod(payment.ProviderPayload)
	return LatestPaymentResp{
		PaymentID:        payment.ID,
		OrderNo:          orderNo,
		ChannelID:        payment.ChannelID,
		ChannelName:      payment.ChannelName,
		ProviderType:     payment.ProviderType,
		ChannelType:      payment.ChannelType,
		InteractionMode:  payment.InteractionMode,
		PayURL:           payment.PayURL,
		QRCode:           payment.QRCode,
		WalletAddress:    info.Address,
		ChainAmount:      info.ChainAmount,
		Chain:            info.Chain,
		TokenID:          info.TokenID,
		PaymentMethods:   ExtractCryptoPaymentMethods(payment.ProviderType, payment.InteractionMode, payment.ProviderPayload),
		SelectedCurrency: currency,
		SelectedNetwork:  network,
		ExpiresAt:        payment.ExpiredAt,
	}
	// 排除：OrderID、Amount、FeeRate、FixedFee、FeeAmount、Currency、Status、
	// ProviderRef、GatewayOrderNo、ProviderPayload、CreatedAt、UpdatedAt、PaidAt、CallbackAt
}

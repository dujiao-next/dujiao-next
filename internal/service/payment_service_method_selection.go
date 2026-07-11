package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/payment/provider"

	"gorm.io/gorm"
)

type SelectPaymentMethodInput struct {
	PaymentID uint
	Currency  string
	Network   string
	Context   context.Context
}

// SelectPaymentMethod 为支持动态付款方式的二维码网关选择币种和网络。
func (s *PaymentService) SelectPaymentMethod(input SelectPaymentMethodInput) (*models.Payment, error) {
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	network := strings.ToLower(strings.TrimSpace(input.Network))
	if input.PaymentID == 0 || currency == "" || network == "" {
		return nil, ErrPaymentInvalid
	}
	payment, err := s.paymentRepo.GetByID(input.PaymentID)
	if err != nil {
		return nil, ErrPaymentUpdateFailed
	}
	if payment == nil {
		return nil, ErrPaymentNotFound
	}
	if payment.Status != constants.PaymentStatusPending ||
		strings.ToLower(strings.TrimSpace(payment.ProviderType)) != constants.PaymentProviderBepusdt ||
		strings.ToLower(strings.TrimSpace(payment.InteractionMode)) != constants.PaymentInteractionQR ||
		strings.TrimSpace(payment.ProviderRef) == "" ||
		(payment.ExpiredAt != nil && !payment.ExpiredAt.After(time.Now())) {
		return nil, ErrPaymentStatusInvalid
	}
	if !paymentMethodAllowed(payment.ProviderPayload, currency, network) {
		return nil, ErrPaymentInvalid
	}
	channel, err := s.channelRepo.GetByID(payment.ChannelID)
	if err != nil || channel == nil {
		return nil, ErrPaymentChannelNotFound
	}
	if s.paymentProviderRegistry == nil {
		return nil, ErrPaymentProviderNotSupported
	}
	p, ok := s.paymentProviderRegistry.Lookup(channel.ProviderType, channel.ChannelType)
	if !ok {
		return nil, ErrPaymentProviderNotSupported
	}
	selector, ok := p.(provider.PaymentMethodSelector)
	if !ok {
		return nil, ErrPaymentProviderNotSupported
	}

	ctx, cancel := detachOutboundRequestContext(input.Context)
	defer cancel()
	selection, err := selector.SelectPaymentMethod(ctx, channel.ConfigJSON, provider.SelectPaymentMethodInput{
		ProviderRef: payment.ProviderRef,
		Currency:    currency,
		Network:     network,
	})
	if err != nil {
		return nil, mapProviderErrorToService(err)
	}
	if selection == nil || strings.TrimSpace(selection.QRCodeURL) == "" {
		return nil, ErrPaymentGatewayResponseInvalid
	}

	var updated *models.Payment
	err = s.paymentRepo.Transaction(func(tx *gorm.DB) error {
		repo := s.paymentRepo.WithTx(tx)
		locked, err := repo.GetByIDForUpdate(payment.ID)
		if err != nil {
			return err
		}
		if locked == nil {
			return ErrPaymentNotFound
		}
		if locked.Status != constants.PaymentStatusPending ||
			(locked.ExpiredAt != nil && !locked.ExpiredAt.After(time.Now())) {
			return ErrPaymentStatusInvalid
		}
		locked.PayURL = ""
		locked.QRCode = strings.TrimSpace(selection.QRCodeURL)
		locked.ProviderPayload = mergePaymentProviderPayload(locked.ProviderPayload, selection.Payload)
		locked.UpdatedAt = time.Now()
		if err := repo.Update(locked); err != nil {
			return ErrPaymentUpdateFailed
		}
		updated = locked
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func paymentMethodAllowed(payload models.JSON, currency, network string) bool {
	data := paymentPayloadMap(payload["data"])
	raw, exists := data["payment_methods"]
	if !exists || raw == nil {
		return false
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return false
	}
	var methods []struct {
		Currency string `json:"currency"`
		Network  string `json:"network"`
	}
	if err := json.Unmarshal(encoded, &methods); err != nil {
		return false
	}
	for _, method := range methods {
		if strings.EqualFold(strings.TrimSpace(method.Currency), currency) &&
			strings.EqualFold(strings.TrimSpace(method.Network), network) {
			return true
		}
	}
	return false
}

func mergePaymentProviderPayload(current, next models.JSON) models.JSON {
	merged := models.JSON{}
	for key, value := range current {
		merged[key] = value
	}
	for key, value := range next {
		if key != "data" {
			merged[key] = value
		}
	}
	currentData := paymentPayloadMap(current["data"])
	nextData := paymentPayloadMap(next["data"])
	data := map[string]interface{}{}
	for key, value := range currentData {
		data[key] = value
	}
	for key, value := range nextData {
		data[key] = value
	}
	if len(data) > 0 {
		merged["data"] = data
	}
	return merged
}

func paymentPayloadMap(value interface{}) map[string]interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return typed
	case models.JSON:
		return map[string]interface{}(typed)
	default:
		return map[string]interface{}{}
	}
}

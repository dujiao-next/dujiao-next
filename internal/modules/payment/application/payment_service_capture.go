package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	orderapp "github.com/dujiao-next/internal/modules/order/application"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// CapturePaymentInput 捕获支付输入。
type CapturePaymentInput struct {
	PaymentID uint
	Context   context.Context
}

func (s *PaymentService) CapturePayment(input CapturePaymentInput) (*paymentdomain.Payment, error) {
	if input.PaymentID == 0 {
		return nil, ErrPaymentInvalid
	}
	payment, err := s.paymentRepo.GetByID(input.PaymentID)
	if err != nil {
		return nil, ErrPaymentUpdateFailed
	}
	if payment == nil {
		return nil, ErrPaymentNotFound
	}
	if payment.Status == constants.PaymentStatusSuccess {
		return payment, nil
	}

	channel, err := s.channelRepo.GetByID(payment.ChannelID)
	if err != nil {
		return nil, ErrPaymentUpdateFailed
	}
	if channel == nil {
		return nil, ErrPaymentChannelNotFound
	}

	providerType := strings.ToLower(strings.TrimSpace(channel.ProviderType))
	if providerType != constants.PaymentProviderOfficial {
		return nil, ErrPaymentProviderNotSupported
	}
	if strings.TrimSpace(payment.ProviderRef) == "" {
		return nil, ErrPaymentInvalid
	}

	// 统一通过 Registry 路由。Registry.Lookup 会返回 channel 对应的 adapter,
	// 如果 adapter 不实现 Capturer,type assertion 失败,返回 ErrPaymentProviderNotSupported。
	// 因此无需在此显式检查 channel 是否支持 capture。
	return s.captureViaRegistry(input, payment, channel)
}

// captureViaRegistry 通过 PaymentProviderRegistry 路由调用 QueryPayment。
// stripe + paypal + wechat 实现了 paymentcontract.GatewayCapturer 接口,其它 channel
// (alipay / epay / epusdt / bepusdt / tokenpay / okpay) 仅实现 webhook 回调,
// type assertion 失败时返回 ErrPaymentProviderNotSupported。
func (s *PaymentService) captureViaRegistry(input CapturePaymentInput, payment *paymentdomain.Payment, channel *paymentdomain.PaymentChannel) (*paymentdomain.Payment, error) {
	logger.Infow("payment_capture_via_registry",
		"payment_id", payment.ID,
		"provider_type", channel.ProviderType,
		"channel_type", channel.ChannelType,
	)
	if s.paymentProviderRegistry == nil {
		return nil, ErrPaymentProviderNotSupported
	}
	p, ok := s.paymentProviderRegistry.Lookup(channel.ProviderType, channel.ChannelType)
	if !ok {
		return nil, ErrPaymentProviderNotSupported
	}
	capturer, ok := p.(paymentcontract.GatewayCapturer)
	if !ok {
		logger.Warnw("payment_provider_capture_not_implemented",
			"provider_type", channel.ProviderType,
			"channel_type", channel.ChannelType,
		)
		return nil, ErrPaymentProviderNotSupported
	}

	// 第二参数是 interactionMode,不是 channelType。stripe/wechat adapter
	// 会拒绝任何非法 mode,传 channelType 会导致  一律 ErrConfigInvalid。
	if err := capturer.ValidateConfig(channel.ConfigJSON, channel.InteractionMode); err != nil {
		return nil, mapProviderErrorToService(err)
	}

	ctx, cancel := detachOutboundRequestContext(input.Context)
	defer cancel()

	queryResult, err := capturer.QueryPayment(ctx, channel.ConfigJSON, payment.ProviderRef)
	if err != nil {
		return nil, mapProviderErrorToService(err)
	}

	payload := jsonmap.JSON{}
	if queryResult.Payload != nil {
		payload = queryResult.Payload
	}
	status := strings.TrimSpace(queryResult.Status)
	if status == "" {
		status = constants.PaymentStatusPending
	}

	callbackInput := PaymentCallbackInput{
		PaymentID:   payment.ID,
		ChannelID:   channel.ID,
		Status:      status,
		ProviderRef: pickFirstNonEmpty(queryResult.ProviderRef, payment.ProviderRef),
		Amount:      queryResult.Amount,
		Currency:    strings.ToUpper(strings.TrimSpace(queryResult.Currency)),
		PaidAt:      queryResult.PaidAt,
		Payload:     payload,
	}
	return s.HandleCallback(callbackInput)
}

// mapProviderErrorToService 把 provider.ErrXxx 转换为 service 层的 ErrPaymentXxx。
func mapProviderErrorToService(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, paymentcontract.ErrGatewayConfigInvalid):
		return fmt.Errorf("%w: %v", ErrPaymentChannelConfigInvalid, err)
	case errors.Is(err, paymentcontract.ErrGatewayRequestFailed), errors.Is(err, paymentcontract.ErrGatewayAuthFailed):
		return fmt.Errorf("%w: %v", ErrPaymentGatewayRequestFailed, err)
	case errors.Is(err, paymentcontract.ErrGatewayResponseInvalid), errors.Is(err, paymentcontract.ErrGatewaySignatureInvalid):
		return fmt.Errorf("%w: %v", ErrPaymentGatewayResponseInvalid, err)
	case errors.Is(err, paymentcontract.ErrGatewayUnsupportedChannel), errors.Is(err, paymentcontract.ErrGatewayProviderNotFound):
		return ErrPaymentProviderNotSupported
	default:
		return fmt.Errorf("%w: %v", ErrPaymentGatewayRequestFailed, err)
	}
}

// ManualConfirmPaymentInput 后台人工确认支付输入。
type ManualConfirmPaymentInput struct {
	OrderID     uint
	PaymentID   uint // 必填：明确要确认的那一条支付记录
	AdminID     uint
	ProviderRef string // 第三方支付流水号 / 链上交易 Hash（可选）
	Remark      string
}

// ManualConfirmPayment 管理员在支付渠道侧核实到账后，手动把订单恢复为已支付并继续履约。
//
// 它复用支付回调的事务路径（金额/币种校验、库存消耗、分销利润入账），
// 提交后同样触发自动发货、邮件、通知、采购等后续流程，不是简单改订单状态。
// 订单已被超时取消时，会在同一事务内重开订单并重新占用自动发货卡密，库存不足则整体回滚。
func (s *PaymentService) ManualConfirmPayment(input ManualConfirmPaymentInput) (*paymentdomain.Payment, error) {
	if input.OrderID == 0 || input.AdminID == 0 || input.PaymentID == 0 {
		return nil, ErrPaymentInvalid
	}
	order, err := s.orderRepo.GetByID(input.OrderID)
	if err != nil {
		return nil, orderapp.ErrOrderFetchFailed
	}
	if order == nil {
		return nil, orderapp.ErrOrderNotFound
	}
	if order.ParentID != nil || order.PaidAt != nil ||
		(order.Status != constants.OrderStatusPendingPayment && order.Status != constants.OrderStatusCanceled) {
		return nil, orderapp.ErrOrderStatusInvalid
	}

	payment, err := s.loadManualConfirmPayment(order.ID, input.PaymentID)
	if err != nil {
		return nil, err
	}

	ref := strings.TrimSpace(input.ProviderRef)
	callbackRef := ""
	if strings.TrimSpace(payment.ProviderRef) == "" {
		callbackRef = ref
	}
	now := time.Now()
	return s.HandleCallback(PaymentCallbackInput{
		PaymentID:   payment.ID,
		OrderNo:     order.OrderNo,
		ChannelID:   payment.ChannelID,
		Status:      constants.PaymentStatusSuccess,
		ProviderRef: callbackRef,
		Amount:      payment.Amount,
		Currency:    payment.Currency,
		Payload: jsonmap.JSON{
			"manual_confirm": jsonmap.JSON{
				"admin_id":                input.AdminID,
				"remark":                  strings.TrimSpace(input.Remark),
				"provider_ref":            ref,
				"previous_order_status":   order.Status,
				"previous_payment_status": payment.Status,
				"confirmed_at":            now.Format(time.RFC3339),
			},
		},
		manualConfirm: true,
	})
}

// loadManualConfirmPayment 取出管理员明确指定要确认的支付记录：必须属于该订单。
// 异常码为 underpaid_payment_succeeded 的记录，款项已转入用户余额，不能再拿来履约，否则会重复给付。
func (s *PaymentService) loadManualConfirmPayment(orderID, paymentID uint) (*paymentdomain.Payment, error) {
	payment, err := s.paymentRepo.GetByID(paymentID)
	if err != nil {
		return nil, ErrPaymentUpdateFailed
	}
	if payment == nil {
		return nil, ErrPaymentNotFound
	}
	if payment.OrderID != orderID {
		return nil, ErrPaymentInvalid
	}
	if payment.ExceptionCode == constants.PaymentExceptionUnderpaidSucceeded {
		return nil, ErrPaymentUnderpaidCredited
	}
	return payment, nil
}

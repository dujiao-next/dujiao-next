package application

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/shared/outboundctx"
	"github.com/dujiao-next/internal/shared/serial"

	"github.com/shopspring/decimal"
)

// detachOutboundRequestContext 将出站请求从上游 HTTP 连接生命周期中解耦。
func detachOutboundRequestContext(parent context.Context) (context.Context, context.CancelFunc) {
	return outboundctx.Detach(parent, outboundctx.DefaultTimeout)
}

func shouldUseGatewayOrderNo(channel *paymentdomain.PaymentChannel) bool {
	return channel != nil
}

func buildGatewayOrderNo() string {
	return serial.Generate("DJP")
}

func resolveGatewayOrderNo(channel *paymentdomain.PaymentChannel, payment *paymentdomain.Payment) string {
	if !shouldUseGatewayOrderNo(channel) {
		return ""
	}
	if payment != nil {
		if gatewayOrderNo := strings.TrimSpace(payment.GatewayOrderNo); gatewayOrderNo != "" {
			return gatewayOrderNo
		}
	}
	return buildGatewayOrderNo()
}

func resolveProviderOrderNo(businessOrderNo string, payment *paymentdomain.Payment) string {
	if payment != nil {
		if gatewayOrderNo := strings.TrimSpace(payment.GatewayOrderNo); gatewayOrderNo != "" {
			return gatewayOrderNo
		}
	}
	return strings.TrimSpace(businessOrderNo)
}

func matchesBusinessOrderNo(callbackOrderNo string, businessOrderNo string, payment *paymentdomain.Payment) bool {
	callbackOrderNo = strings.TrimSpace(callbackOrderNo)
	if callbackOrderNo == "" || callbackOrderNo == strings.TrimSpace(businessOrderNo) {
		return true
	}
	return payment != nil && callbackOrderNo == strings.TrimSpace(payment.GatewayOrderNo)
}

func buildPaymentReturnQuery(input CreatePaymentInput, order *orderdomain.Order, marker string, sessionID string) map[string]string {
	params := map[string]string{}
	bizType := strings.ToLower(strings.TrimSpace(input.ReturnBizType))
	businessNo := strings.TrimSpace(input.ReturnBusinessNo)
	isGuest := input.ReturnGuest
	if bizType == "" {
		bizType = "order"
	}
	if order != nil {
		if businessNo == "" {
			businessNo = strings.TrimSpace(order.OrderNo)
		}
		if !isGuest && order.UserID == 0 && bizType == "order" {
			isGuest = true
		}
	}
	params["biz_type"] = bizType
	switch bizType {
	case "recharge":
		if businessNo != "" {
			params["recharge_no"] = businessNo
		}
	default:
		if businessNo != "" {
			params["order_no"] = businessNo
		}
		if isGuest {
			params["guest"] = "1"
		}
	}
	if marker = strings.TrimSpace(marker); marker != "" {
		params[marker] = "1"
	}
	if sessionID = strings.TrimSpace(sessionID); sessionID != "" {
		params["session_id"] = sessionID
	}
	return params
}

func (s *PaymentService) applyProviderPayment(input CreatePaymentInput, order *orderdomain.Order, channel *paymentdomain.PaymentChannel, payment *paymentdomain.Payment) (err error) {
	providerType := strings.ToLower(strings.TrimSpace(channel.ProviderType))
	channelType := strings.ToLower(strings.TrimSpace(channel.ChannelType))
	gatewayCtx, cancel := detachOutboundRequestContext(input.Context)
	defer cancel()
	payment.GatewayOrderNo = resolveGatewayOrderNo(channel, payment)
	providerOrderNo := resolveProviderOrderNo(order.OrderNo, payment)
	log := paymentLogger(
		"order_id", order.ID,
		"order_no", order.OrderNo,
		"gateway_order_no", payment.GatewayOrderNo,
		"payment_id", payment.ID,
		"channel_id", channel.ID,
		"provider_type", providerType,
		"channel_type", channelType,
		"interaction_mode", channel.InteractionMode,
	)
	defer func() {
		if err != nil {
			log.Errorw("payment_provider_apply_failed", "error", err)
			return
		}
		log.Infow("payment_provider_apply_success")
	}()
	if s.paymentProviderRegistry == nil {
		return ErrPaymentProviderNotSupported
	}

	p, ok := s.paymentProviderRegistry.Lookup(channel.ProviderType, channel.ChannelType)
	if !ok {
		return ErrPaymentProviderNotSupported
	}

	// 构造 paymentcontract.GatewayCreateInput。
	// NotifyURL / ReturnURL 留空：各 adapter/native 包均实现 "input值 || cfg值" fallback，
	// 空值时自动读取 channel.ConfigJSON 里配置的 notify_url / return_url。
	// P1.2c Task 3: returnURLQuery 携带 biz_type/order_no/marker 等，由 wrapper append 到 ReturnURL。
	extra := jsonmap.JSON{}
	if interactionMode := strings.TrimSpace(channel.InteractionMode); interactionMode != "" {
		extra["interaction_mode"] = interactionMode
	}
	// order_user_key 是 tokenpay 必须的稳定用户标识符；其他 adapter 忽略此字段。
	extra["order_user_key"] = resolveTokenPayOrderUserKey(order)

	// P1.2c Task 3: 构造 return URL tracking marker。
	// official provider 用 channelType 区分网关(paypal/alipay/wechat/stripe)，其他 provider 用 providerType。
	returnMarker := providerType + "_return"
	if providerType == constants.PaymentProviderOfficial {
		returnMarker = channelType + "_return"
	}
	returnURLQuery := buildPaymentReturnQuery(input, order, returnMarker, "")

	createInput := paymentcontract.GatewayCreateInput{
		PaymentID:      payment.ID,
		OrderID:        order.ID,
		OrderNo:        providerOrderNo,
		Subject:        buildOrderSubject(order),
		Amount:         payment.Amount,
		Currency:       payment.Currency,
		ClientIP:       strings.TrimSpace(input.ClientIP),
		ChannelType:    channel.ChannelType,
		Extra:          extra,
		ReturnURLQuery: returnURLQuery,
		// 分销站/自定义域名下单时按当前 tenant 域名生成 ReturnURL（主站为空，
		// 由各 adapter fallback 到 cfg 的 return_url/success_url），再 append ReturnURLQuery。
		// NotifyURL 留空，始终由各 adapter 从 cfg 读取。
		ReturnURL: resolveTenantReturnURL(input.Context, input.RequestScheme, channel),
	}

	result, err := p.CreatePayment(gatewayCtx, channel.ConfigJSON, createInput)
	if err != nil {
		return mapProviderErrorToService(err)
	}

	// 把 result 写回 payment 字段
	payment.PayURL = strings.TrimSpace(result.RedirectURL)
	payment.QRCode = strings.TrimSpace(result.QRCodeURL)
	if result.ProviderRef != "" {
		payment.ProviderRef = result.ProviderRef
	}
	// 确保 ProviderRef 始终有值（各 adapter 可能返回空，如 wechat CreatePayment 阶段）。
	// 主动查询必须使用实际提交到网关的订单号，而不是业务订单号。
	if payment.ProviderRef == "" {
		payment.ProviderRef = providerOrderNo
	}
	if result.Payload != nil {
		payment.ProviderPayload = mergeProviderPayload(payment.ProviderPayload, result.Payload)
	}
	// DisplayChannelType 是 adapter 返回的“展示用渠道类型”。
	// 例如 BEpusdt 新格式的 payment.channel_type 固定为 bepusdt，
	// 但交易模式实际展示应使用 config_json.trade_type（如 usdt.arbitrum）。
	// 这里统一写入 provider_payload.display_channel_type，供通知、后台列表等展示层读取，
	// 避免修改 payment.channel_type 的数据库语义。
	if displayChannelType := strings.TrimSpace(result.DisplayChannelType); displayChannelType != "" {
		if payment.ProviderPayload == nil {
			payment.ProviderPayload = jsonmap.JSON{}
		}
		payment.ProviderPayload["display_channel_type"] = displayChannelType
	}
	// P1.2c: 用 wrapper 转换后的 amount/currency 更新 payment 记录，
	// 保持 DB 状态与实际发给网关的金额/币种一致（P1.2c Task 1 把 conversion 下沉到 wrapper）。
	if strings.TrimSpace(result.CurrencySent) != "" {
		payment.Currency = result.CurrencySent
	}
	if strings.TrimSpace(result.AmountSent) != "" {
		if d, parseErr := decimal.NewFromString(result.AmountSent); parseErr == nil {
			payment.Amount = money.Amount{Decimal: d}
		}
	}
	payment.Status = constants.PaymentStatusPending
	payment.UpdatedAt = time.Now()

	if err := s.paymentRepo.Update(payment); err != nil {
		return ErrPaymentUpdateFailed
	}
	return nil
}

// TestChannelSecurity 对已保存的微信支付渠道执行官方非交易安全诊断。
func (s *PaymentService) TestChannelSecurity(ctx context.Context, channelID uint) (*paymentcontract.GatewaySecurityTestResult, error) {
	channel, err := s.GetChannel(channelID)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(channel.ProviderType), constants.PaymentProviderOfficial) ||
		!strings.EqualFold(strings.TrimSpace(channel.ChannelType), constants.PaymentChannelTypeWechat) {
		return nil, ErrPaymentProviderNotSupported
	}
	if s.paymentProviderRegistry == nil {
		return nil, ErrPaymentProviderNotSupported
	}
	provider, ok := s.paymentProviderRegistry.Lookup(channel.ProviderType, channel.ChannelType)
	if !ok {
		return nil, ErrPaymentProviderNotSupported
	}
	tester, ok := provider.(paymentcontract.GatewaySecurityTester)
	if !ok {
		return nil, ErrPaymentProviderNotSupported
	}

	result, err := tester.TestSecurity(ctx, channel.ConfigJSON)
	if err != nil {
		return nil, mapProviderErrorToService(err)
	}
	if result == nil {
		return nil, fmt.Errorf("%w: empty security test result", ErrPaymentGatewayResponseInvalid)
	}
	paymentLogger(
		"channel_id", channel.ID,
		"provider_type", channel.ProviderType,
		"channel_type", channel.ChannelType,
		"verification_mode", result.VerificationMode,
		"response_serial", result.ResponseSerial,
	).Infow("payment_channel_security_test_success")
	return result, nil
}

// ValidateChannel 校验支付渠道配置（admin 端 channel 创建/更新时调用）。
//
// P1.2c Task 10: 160 行 switch 退化为 Registry.Lookup + Provider.ValidateConfig 单点。
// 基础字段校验（nil / fee / amount / providerType / wallet 跳过）保留在 service 层；
// 配置格式验证通过 Registry 委托给各 adapter wrapper。
func (s *PaymentService) ValidateChannel(channel *paymentdomain.PaymentChannel) error {
	if channel == nil {
		return ErrPaymentChannelConfigInvalid
	}
	feeRate := channel.FeeRate.Decimal.Round(2)
	if feeRate.LessThan(decimal.Zero) || feeRate.GreaterThan(decimal.NewFromInt(100)) {
		return ErrPaymentChannelConfigInvalid
	}
	fixedFee := channel.FixedFee.Decimal.Round(2)
	// decimal(6,2) max value is 9999.99
	if fixedFee.LessThan(decimal.Zero) || fixedFee.GreaterThanOrEqual(decimal.NewFromInt(10000)) {
		return ErrPaymentChannelConfigInvalid
	}
	minAmount := channel.MinAmount.Decimal.Round(2)
	maxAmount := channel.MaxAmount.Decimal.Round(2)
	amountOverflow20_2 := decimal.NewFromInt(1000000000000000000)
	// min/max amount are stored as decimal(20,2), max allowed is 999999999999999999.99.
	if minAmount.LessThan(decimal.Zero) || minAmount.GreaterThanOrEqual(amountOverflow20_2) || maxAmount.LessThan(decimal.Zero) || maxAmount.GreaterThanOrEqual(amountOverflow20_2) {
		return ErrPaymentChannelConfigInvalid
	}
	if maxAmount.GreaterThan(decimal.Zero) && minAmount.GreaterThan(maxAmount) {
		return ErrPaymentChannelConfigInvalid
	}

	providerType := strings.ToLower(strings.TrimSpace(channel.ProviderType))
	channelType := strings.ToLower(strings.TrimSpace(channel.ChannelType))
	if providerType == "" {
		return ErrPaymentChannelConfigInvalid
	}

	// wallet 是内部余额通道，无 native adapter，直接通过。
	if providerType == constants.PaymentProviderWallet {
		return nil
	}

	// 非 official provider（epay/bepusdt/epusdt/okpay/tokenpay）只支持 qr/redirect。
	// official provider 的 interaction_mode 验证由各 adapter 的 ValidateConfig 负责。
	if providerType != constants.PaymentProviderOfficial {
		mode := strings.ToLower(strings.TrimSpace(channel.InteractionMode))
		if mode != constants.PaymentInteractionQR && mode != constants.PaymentInteractionRedirect {
			return ErrPaymentChannelConfigInvalid
		}
		if providerType == constants.PaymentProviderBepusdt && mode == constants.PaymentInteractionQR {
			orderMode := strings.ToLower(strings.TrimSpace(fmt.Sprint(channel.ConfigJSON["order_mode"])))
			if orderMode == constants.PaymentBepusdtOrderModeCashier {
				return ErrPaymentChannelConfigInvalid
			}
		}
	}

	if s.paymentProviderRegistry == nil {
		return ErrPaymentProviderNotSupported
	}
	p, ok := s.paymentProviderRegistry.Lookup(channel.ProviderType, channel.ChannelType)
	if !ok {
		return fmt.Errorf("%w: unsupported provider_type=%s channel_type=%s",
			ErrPaymentChannelConfigInvalid, channel.ProviderType, channel.ChannelType)
	}

	// official provider：第二参数传 interactionMode，供 wechatpay/alipay adapter 验证。
	// 非 official provider：第二参数传 channelType，供 epay/bepusdt/okpay adapter 验证 channel 类型。
	var validateParam string
	if providerType == constants.PaymentProviderOfficial {
		validateParam = strings.ToLower(strings.TrimSpace(channel.InteractionMode))
	} else {
		validateParam = channelType
	}
	if err := p.ValidateConfig(channel.ConfigJSON, validateParam); err != nil {
		return mapProviderErrorToService(err)
	}
	return nil
}

// resolveTenantReturnURL 在分销站/自定义域名场景下按当前 tenant 域名生成同步回跳地址。
// 主站 tenant 返回空串，保持渠道配置 return_url/success_url 的固定值兜底行为；
// 分销 tenant 若回跳到主站域名，游客订单会因 tenant 隔离查不到（见 order_reseller_snapshot_test.go）。
func resolveTenantReturnURL(ctx context.Context, requestScheme string, channel *paymentdomain.PaymentChannel) string {
	tenant, ok := resellercontract.TenantFromContext(ctx)
	if !ok || tenant.IsMain || tenant.Unavailable {
		return ""
	}
	host := strings.TrimSpace(tenant.Host)
	if host == "" {
		host = strings.TrimSpace(tenant.PrimaryDomain)
	}
	if host == "" {
		return ""
	}
	scheme := strings.ToLower(strings.TrimSpace(requestScheme))
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	return scheme + "://" + host + tenantReturnPath(channel)
}

// tenantReturnPath 沿用渠道配置回跳地址中的路径与 query（站长可能自定义了路径），
// 配置缺失或无路径时默认前台支付结果页 /pay。
func tenantReturnPath(channel *paymentdomain.PaymentChannel) string {
	if channel != nil {
		for _, key := range []string{"return_url", "success_url"} {
			raw, _ := channel.ConfigJSON[key].(string)
			if raw = strings.TrimSpace(raw); raw == "" {
				continue
			}
			u, err := url.Parse(raw)
			if err != nil {
				continue
			}
			path := u.EscapedPath()
			if path == "" || path == "/" {
				continue
			}
			if u.RawQuery != "" {
				path += "?" + u.RawQuery
			}
			return path
		}
	}
	return "/pay"
}

func resolveTokenPayOrderUserKey(order *orderdomain.Order) string {
	if order == nil {
		return ""
	}
	if order.UserID > 0 {
		return strconv.FormatUint(uint64(order.UserID), 10)
	}
	if guestEmail := strings.TrimSpace(order.GuestEmail); guestEmail != "" {
		return guestEmail
	}
	return strings.TrimSpace(order.OrderNo)
}

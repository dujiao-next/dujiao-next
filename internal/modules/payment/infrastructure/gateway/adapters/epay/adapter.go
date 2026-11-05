package epayadapter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	gatewaycommon "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/epay"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// epayAdapter 是 epay 网关的 paymentcontract.GatewayProvider + paymentcontract.GatewayCallbackVerifier 实现。
// epay 没有主动查询 API，callback 是同步 form POST（不是 JSON webhook），
// 所以**不**实现 paymentcontract.GatewayCapturer 和 paymentcontract.GatewayWebhooker。
type epayAdapter struct{}

// NewEpayAdapter 实例化 epay adapter。
func NewEpayAdapter() paymentcontract.GatewayProvider { return &epayAdapter{} }

// 编译期断言 epayAdapter 实现了 paymentcontract.GatewayProvider 和 paymentcontract.GatewayCallbackVerifier。
var (
	_ paymentcontract.GatewayProvider         = (*epayAdapter)(nil)
	_ paymentcontract.GatewayCallbackVerifier = (*epayAdapter)(nil)
)

// Type 返回 provider 标识。
func (a *epayAdapter) Type() string {
	return constants.PaymentProviderEpay + ":"
}

// parseConfig 解析并验证 epay Config。epay 不需要 interactionMode，
// 通过 dispatch 时按 mode 调用不同的函数（BuildRedirectURL vs CreatePayment）。
func (a *epayAdapter) parseConfig(raw jsonmap.JSON) (*epay.Config, error) {
	cfg, err := epay.ParseConfig(raw)
	if err != nil {
		return nil, mapEpayError(err)
	}
	if err := epay.ValidateConfig(cfg); err != nil {
		return nil, mapEpayError(err)
	}
	return cfg, nil
}

// ValidateConfig 验证 channel.ConfigJSON。
// 入口先校验 channelType（如果非空）是否被 epay 支持，
// 然后调用 parseConfig 验证配置完整性。
func (a *epayAdapter) ValidateConfig(raw jsonmap.JSON, channelType string) error {
	if channelType != "" && !epay.IsSupportedChannelType(channelType) {
		return fmt.Errorf("%w: epay channel_type %s", paymentcontract.ErrGatewayUnsupportedChannel, channelType)
	}
	_, err := a.parseConfig(raw)
	return err
}

// CreatePayment 创建支付。双 mode dispatch：
//   - mode == "redirect" → BuildRedirectURL（不发 HTTP，仅构造跳转 URL）
//   - mode == "" 或 mode == "qr" → CreatePayment（发 HTTP）
//   - 其他 mode → 返回 paymentcontract.ErrGatewayConfigInvalid
func (a *epayAdapter) CreatePayment(ctx context.Context, raw jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	// 先校验 channelType
	if !epay.IsSupportedChannelType(input.ChannelType) {
		return nil, fmt.Errorf("%w: epay channel_type %s", paymentcontract.ErrGatewayUnsupportedChannel, input.ChannelType)
	}

	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}

	// NotifyURL / ReturnURL: 优先用 input 传入值，为空则 fallback 到 cfg 里的值。
	// epay native 硬校验这两个字段非空，所以必须做 fallback，否则 caller 若没传就会报错。
	notifyURL := strings.TrimSpace(input.NotifyURL)
	if notifyURL == "" {
		notifyURL = strings.TrimSpace(cfg.NotifyURL)
	}
	returnURL := strings.TrimSpace(input.ReturnURL)
	if returnURL == "" {
		returnURL = strings.TrimSpace(cfg.ReturnURL)
	}
	// P1.2c Task 3: fallback 完成后 append tracking marker。
	returnURL = gatewaycommon.AppendQueryParams(returnURL, input.ReturnURLQuery)

	// P1.2c: wrapper 内做 currency conversion + audit 字段写入，在 mode dispatch 之前完成，
	// 使两条 path（BuildRedirectURL / CreatePayment）都使用转换后金额。
	// exchange_rate / original_amount / original_currency 保留到 result.Payload，
	// 供运营/财务跨币种对账追溯实际收费 vs 原始金额。
	// result.AmountSent/CurrencySent 反映实际发给网关的金额/币种，
	// 让 service 层据此更新 payment.Amount/Currency，保持记录与实际收费一致。
	originalAmount := input.Amount.Decimal.String()
	originalCurrency := input.Currency
	payAmount := originalAmount
	payCurrency := originalCurrency
	converted := false
	if cfg.NeedsCurrencyConversion() {
		convAmount, convCurrency, convErr := cfg.ConvertAmount(payAmount, payCurrency, 2)
		if convErr != nil {
			return nil, fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, convErr)
		}
		payAmount = convAmount
		payCurrency = convCurrency
		converted = true
	}

	native := epay.CreateInput{
		OrderNo:     input.OrderNo,
		Amount:      payAmount,
		Subject:     input.Subject,
		ChannelType: input.ChannelType,
		ClientIP:    input.ClientIP,
		NotifyURL:   notifyURL,
		ReturnURL:   returnURL,
	}

	// 从 input.Extra 取 interaction_mode，按模式 dispatch
	mode, _ := input.Extra["interaction_mode"].(string)
	mode = strings.ToLower(strings.TrimSpace(mode))

	var result *epay.CreateResult
	switch mode {
	case constants.PaymentInteractionRedirect:
		result, err = epay.BuildRedirectURL(cfg, native)
	case "", constants.PaymentInteractionQR:
		result, err = epay.CreatePayment(ctx, cfg, native)
	default:
		return nil, fmt.Errorf("%w: epay interaction_mode %s", paymentcontract.ErrGatewayConfigInvalid, mode)
	}

	if err != nil {
		return nil, mapEpayError(err)
	}

	payload := jsonmap.JSON{}
	if result.Raw != nil {
		payload = jsonmap.JSON(result.Raw)
	}
	if converted {
		payload["exchange_rate"] = strings.TrimSpace(cfg.ExchangeRate)
		payload["original_amount"] = originalAmount
		payload["original_currency"] = originalCurrency
	}

	redirectURL := result.PayURL
	if mode == constants.PaymentInteractionRedirect && epaySubmitPOST(payload) {
		formURL, token, formErr := epayRedirectFormURL(input.PaymentID, returnURL, notifyURL)
		if formErr != nil {
			return nil, formErr
		}
		payload["token"] = token
		redirectURL = formURL
	}

	return &paymentcontract.GatewayCreateResult{
		ProviderRef:  result.TradeNo,
		RedirectURL:  redirectURL,
		QRCodeURL:    result.QRCode,
		Payload:      payload,
		AmountSent:   payAmount,
		CurrencySent: payCurrency,
	}, nil
}

func epaySubmitPOST(payload jsonmap.JSON) bool {
	method, _ := payload["submit_method"].(string)
	return strings.EqualFold(strings.TrimSpace(method), http.MethodPost)
}

// epayRedirectFormURL 把 v2 跳转收成站内表单地址。域名优先取 return_url，为空再退回 notify_url。
func epayRedirectFormURL(paymentID uint, returnURL, notifyURL string) (string, string, error) {
	if paymentID == 0 {
		return "", "", fmt.Errorf("%w: epay redirect payment id is required", paymentcontract.ErrGatewayConfigInvalid)
	}
	originSource := strings.TrimSpace(returnURL)
	if originSource == "" {
		originSource = strings.TrimSpace(notifyURL)
	}
	origin, err := httpOrigin(originSource)
	if err != nil {
		return "", "", fmt.Errorf("%w: epay redirect url origin", paymentcontract.ErrGatewayConfigInvalid)
	}
	token, err := newEpayRedirectToken()
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	}
	formURL := origin + "/api/v1/payments/epay-redirect?payment_id=" + strconv.FormatUint(uint64(paymentID), 10) + "&token=" + token
	return formURL, token, nil
}

func httpOrigin(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("url scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("url host is empty")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func newEpayRedirectToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// VerifyCallback 实现 paymentcontract.GatewayCallbackVerifier。epay 用 form POST，body 参数忽略。
// epay 同步 callback form 字段：
//   - out_trade_no = 商户单号(OrderNo)
//   - trade_no     = epay 网关单号(ProviderRef)
//   - trade_status = TRADE_SUCCESS / TRADE_FINISHED 等
//   - money        = 金额
//   - sign / sign_type / type / pid / param 等元数据
func (a *epayAdapter) VerifyCallback(raw jsonmap.JSON, form map[string][]string, _ []byte) (*paymentcontract.GatewayCallbackResult, error) {
	cfg, err := epay.ParseConfig(raw)
	if err != nil {
		return nil, mapEpayError(err)
	}

	if err := epay.VerifyCallback(cfg, form); err != nil {
		return nil, mapEpayError(err)
	}
	if err := epay.VerifyCallbackOwnership(cfg, form); err != nil {
		return nil, mapEpayError(err)
	}

	orderNo := gatewaycommon.FormValue(form, "out_trade_no")
	providerRef := gatewaycommon.FormValue(form, "trade_no")
	tradeStatus := gatewaycommon.FormValue(form, "trade_status")
	amountStr := gatewaycommon.FormValue(form, "money")

	status := constants.PaymentStatusPending
	if tradeStatus == "TRADE_SUCCESS" || tradeStatus == "TRADE_FINISHED" {
		status = constants.PaymentStatusSuccess
	}

	// amount 解析失败时返回零值：wrapper 仅做适配，金额异常由业务层判定。
	amount := money.Amount{}
	if s := strings.TrimSpace(amountStr); s != "" {
		if d, parseErr := decimal.NewFromString(s); parseErr == nil {
			amount = money.FromDecimal(d)
		}
	}

	return &paymentcontract.GatewayCallbackResult{
		OrderNo:     orderNo,
		ProviderRef: providerRef,
		Status:      status,
		Amount:      amount,
		Currency:    "CNY",
		PaidAt:      nil, // epay callback 不带付款时间字段
		Payload:     gatewaycommon.FormToJSON(form),
	}, nil
}

// mapEpayError 把 epay 包的 sentinel error 映射为 provider 统一错误。
func mapEpayError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, epay.ErrConfigInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, epay.ErrChannelTypeNotOK):
		// P1.2a Task 1 加的 paymentcontract.ErrGatewayUnsupportedChannel 就是给这个用的
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayUnsupportedChannel, err)
	case errors.Is(err, epay.ErrSignatureGenerate):
		// 签名生成失败 = 配置层面错误(私钥不可用)
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, epay.ErrSignatureInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewaySignatureInvalid, err)
	case errors.Is(err, epay.ErrRequestFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	case errors.Is(err, epay.ErrResponseInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	default:
		return err
	}
}

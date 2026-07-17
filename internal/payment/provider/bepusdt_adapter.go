package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/payment/bepusdt"

	"github.com/shopspring/decimal"
)

// bepusdtAdapter 是 bepusdt 网关的 Provider + CallbackVerifier 实现。
// 与 epusdt 相似，但多 channel type 支持（usdt-trc20 / usdc-trc20 / trx 等），
// 需要根据 channelType 动态设置 cfg.TradeType。
// callback 是同步 JSON POST（不是 form），所以**不**实现 Capturer 和 Webhooker。
type bepusdtAdapter struct{}

// NewBepusdtAdapter 实例化 bepusdt adapter。
func NewBepusdtAdapter() Provider { return &bepusdtAdapter{} }

// 编译期断言 bepusdtAdapter 实现了 Provider 和 CallbackVerifier。
var (
	_ Provider         = (*bepusdtAdapter)(nil)
	_ CallbackVerifier = (*bepusdtAdapter)(nil)
)

// Type 返回 provider 标识。bepusdt 是多 channel type provider，返回值中 channelType 部分为空。
func (a *bepusdtAdapter) Type() string {
	return constants.PaymentProviderBepusdt + ":"
}

// parseConfig 解析并验证 bepusdt Config。
// 关键：如果 cfg.TradeType 未显式配置且 channelType 非空，
// 则从 channelType 自动 resolve trade_type（沿用 payment_service_provider.go 的逻辑）。
func (a *bepusdtAdapter) parseConfig(raw models.JSON, channelType string) (*bepusdt.Config, error) {
	cfg, err := bepusdt.ParseConfig(raw)
	if err != nil {
		return nil, mapBepusdtError(err)
	}
	// 如果配置中没有指定 trade_type，根据 channel_type 自动设置
	if strings.TrimSpace(cfg.TradeType) == "" && channelType != "" {
		cfg.TradeType = bepusdt.ResolveTradeType(channelType)
	}
	if err := bepusdt.ValidateConfig(cfg); err != nil {
		return nil, mapBepusdtError(err)
	}
	return cfg, nil
}

// ValidateConfig 验证 channel.ConfigJSON。
// 入口先校验 channelType（如果非空）是否被 bepusdt 支持，
// 然后调用 parseConfig 验证配置完整性。
func (a *bepusdtAdapter) ValidateConfig(raw models.JSON, channelType string) error {
	if channelType != "" && !bepusdt.IsSupportedChannelType(channelType) {
		return fmt.Errorf("%w: bepusdt channel_type %s", ErrUnsupportedChannel, channelType)
	}
	_, err := a.parseConfig(raw, channelType)
	return err
}

// CreatePayment 创建支付。bepusdt 多 channel type，需要先校验 channelType。
func (a *bepusdtAdapter) CreatePayment(ctx context.Context, raw models.JSON, input CreateInput) (*CreateResult, error) {
	// 先校验 channelType
	if input.ChannelType != "" && !bepusdt.IsSupportedChannelType(input.ChannelType) {
		return nil, fmt.Errorf("%w: bepusdt channel_type %s", ErrUnsupportedChannel, input.ChannelType)
	}

	cfg, err := a.parseConfig(raw, input.ChannelType)
	if err != nil {
		return nil, err
	}

	returnURL := strings.TrimSpace(input.ReturnURL)
	if returnURL == "" {
		returnURL = strings.TrimSpace(cfg.ReturnURL)
	}
	returnURL = appendQueryParams(returnURL, input.ReturnURLQuery)

	native := bepusdt.CreateInput{
		OrderNo:   input.OrderNo,
		Amount:    input.Amount.Decimal.String(),
		Name:      input.Subject,
		NotifyURL: input.NotifyURL,
		ReturnURL: returnURL,
	}
	result, err := bepusdt.CreatePayment(ctx, cfg, native)
	if err != nil {
		return nil, mapBepusdtError(err)
	}

	mode, _ := input.Extra["interaction_mode"].(string)
	mode = strings.ToLower(strings.TrimSpace(mode))
	redirectURL := strings.TrimSpace(result.PaymentURL)
	qrCodeURL := redirectURL
	switch mode {
	case constants.PaymentInteractionQR:
		qrCodeURL = strings.TrimSpace(result.Token)
		redirectURL = ""
		if qrCodeURL == "" {
			return nil, fmt.Errorf("%w: bepusdt token is empty", ErrResponseInvalid)
		}
	case "", constants.PaymentInteractionRedirect:
	default:
		return nil, fmt.Errorf("%w: bepusdt interaction_mode %s", ErrConfigInvalid, mode)
	}

	return &CreateResult{
		ProviderRef: result.TradeID,
		RedirectURL: redirectURL,
		QRCodeURL:   qrCodeURL,
		Payload:     buildBepusdtCreatePayload(result, cfg.TradeType),
	}, nil
}

func buildBepusdtCreatePayload(result *bepusdt.CreateResult, tradeType string) models.JSON {
	payload := models.JSON{}
	if result == nil {
		return payload
	}
	if result.Raw != nil {
		for key, value := range result.Raw {
			payload[key] = value
		}
	}

	data := ensureBepusdtPayloadData(payload)
	setBepusdtPayloadString(data, "trade_type", tradeType)
	setBepusdtPayloadString(data, "token", result.Token)
	setBepusdtPayloadString(data, "actual_amount", result.ActualAmount)
	setBepusdtPayloadString(data, "payment_url", result.PaymentURL)

	chain, tokenID := resolveBepusdtTradeLabels(tradeType)
	setBepusdtPayloadString(data, "chain", chain)
	setBepusdtPayloadString(data, "token_id", tokenID)
	return payload
}

func ensureBepusdtPayloadData(payload models.JSON) map[string]interface{} {
	if raw, ok := payload["data"].(map[string]interface{}); ok {
		return raw
	}
	if raw, ok := payload["data"].(models.JSON); ok {
		data := map[string]interface{}(raw)
		payload["data"] = data
		return data
	}
	data := map[string]interface{}{}
	payload["data"] = data
	return data
}

func setBepusdtPayloadString(payload map[string]interface{}, key string, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		payload[key] = value
	}
}

func resolveBepusdtTradeLabels(tradeType string) (chain string, tokenID string) {
	switch strings.ToLower(strings.TrimSpace(tradeType)) {
	case "usdt.trc20":
		return "tron", "tron-usdt"
	case "usdt.erc20":
		return "ethereum", "ethereum-usdt"
	case "usdt.bep20":
		return "bsc", "bsc-usdt"
	case "usdt.polygon":
		return "polygon", "polygon-usdt"
	case "usdc.trc20":
		return "tron", "tron-usdc"
	case "usdc.erc20":
		return "ethereum", "ethereum-usdc"
	case "usdc.polygon":
		return "polygon", "polygon-usdc"
	case "usdc.bep20":
		return "bsc", "bsc-usdc"
	case "tron.trx":
		return "tron", "tron-trx"
	case "eth.eth":
		return "ethereum", "ethereum-eth"
	case "bsc.bnb":
		return "bsc", "bsc-bnb"
	default:
		return "", ""
	}
}

// VerifyCallback 实现 CallbackVerifier。bepusdt 用 JSON POST body，form 参数忽略。
// 注意：callback 阶段不调 ValidateConfig——配置错误由签名校验兜底，
// 与 alipay/epay/epusdt adapter 行为一致。
func (a *bepusdtAdapter) VerifyCallback(raw models.JSON, _ map[string][]string, body []byte) (*CallbackResult, error) {
	cfg, err := bepusdt.ParseConfig(raw)
	if err != nil {
		return nil, mapBepusdtError(err)
	}

	data, err := bepusdt.ParseCallback(body)
	if err != nil {
		return nil, mapBepusdtError(err)
	}

	if err := bepusdt.VerifyCallback(cfg, data); err != nil {
		return nil, mapBepusdtError(err)
	}

	// bepusdt 用 status int → PaymentStatusXxx string 映射
	status := bepusdt.ToPaymentStatus(data.Status)

	// amount 解析失败时返回零值：wrapper 仅做适配，金额异常由业务层判定。
	amount := models.Money{}
	if data.Amount != nil {
		amountFloat := data.GetAmount()
		if amountFloat > 0 {
			amount = models.NewMoneyFromDecimal(decimal.NewFromFloat(amountFloat))
		}
	}

	// 把 callback 关键字段塞进 Payload
	payload := models.JSON{}
	if pb, marshalErr := json.Marshal(data); marshalErr == nil {
		var m map[string]interface{}
		if jsonErr := json.Unmarshal(pb, &m); jsonErr == nil {
			payload = models.JSON(m)
		}
	}

	// bepusdt callback 不带 currency，从 cfg.Fiat 取（默认 CNY）
	currency := strings.ToUpper(strings.TrimSpace(cfg.Fiat))
	if currency == "" {
		currency = "CNY"
	}

	return &CallbackResult{
		OrderNo:     data.OrderID,
		ProviderRef: data.TradeID,
		Status:      status,
		Amount:      amount,
		Currency:    currency,
		PaidAt:      nil, // bepusdt callback 不带付款时间
		Payload:     payload,
	}, nil
}

// mapBepusdtError 把 bepusdt 包的 sentinel error 映射为 provider 统一错误。
// 比 epusdt 多一个 ErrTradeTypeNotSupport → ErrUnsupportedChannel 映射。
func mapBepusdtError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, bepusdt.ErrConfigInvalid):
		return fmt.Errorf("%w: %v", ErrConfigInvalid, err)
	case errors.Is(err, bepusdt.ErrTradeTypeNotSupport):
		// P1.2a Task 1 加的 ErrUnsupportedChannel 就是给这种场景用的
		return fmt.Errorf("%w: %v", ErrUnsupportedChannel, err)
	case errors.Is(err, bepusdt.ErrRequestFailed):
		return fmt.Errorf("%w: %v", ErrRequestFailed, err)
	case errors.Is(err, bepusdt.ErrResponseInvalid):
		return fmt.Errorf("%w: %v", ErrResponseInvalid, err)
	case errors.Is(err, bepusdt.ErrSignatureInvalid):
		return fmt.Errorf("%w: %v", ErrSignatureInvalid, err)
	default:
		return err
	}
}

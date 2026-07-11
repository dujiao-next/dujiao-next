package dto

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
)

// CryptoWalletInfo 是二维码加密货币支付页需要直接展示的链上付款信息。
type CryptoWalletInfo struct {
	Address     string
	ChainAmount string
	Chain       string
	TokenID     string
}

// CryptoPaymentMethod 是 BEpusdt 收银台订单可选择的链上付款方式。
type CryptoPaymentMethod struct {
	Amount          string `json:"amount,omitempty"`
	ActualAmount    string `json:"actual_amount,omitempty"`
	Fiat            string `json:"fiat,omitempty"`
	ExchangeRate    string `json:"exchange_rate,omitempty"`
	Currency        string `json:"currency"`
	Network         string `json:"network"`
	TokenNetName    string `json:"token_net_name,omitempty"`
	TokenCustomName string `json:"token_custom_name,omitempty"`
	IsPopular       bool   `json:"is_popular,omitempty"`
}

// HasAny 返回是否存在任意可展示的链上付款字段。
func (info CryptoWalletInfo) HasAny() bool {
	return info.Address != "" || info.ChainAmount != "" || info.Chain != "" || info.TokenID != ""
}

// ExtractCryptoWalletInfo 从 Payment.ProviderPayload 中提取链上付款信息。
// 只在 interactionMode == "qr" 且 providerType 为加密货币网关时返回非空值；
// 其他情况返回空结构体，由调用方决定是否输出 omitempty 字段。
//
// 字段名按各 provider 原生响应：
//   - bepusdt:   data.token            (地址)  / data.actual_amount
//   - epusdt:    data.receive_address  (地址)  / data.actual_amount
//   - dujiaopay: pay_address           (地址)  / payable_amount / chain / token_id
//   - tokenpay:  暂不支持（包未解析地址），保留扩展位
func ExtractCryptoWalletInfo(providerType, interactionMode string, payload models.JSON) CryptoWalletInfo {
	if strings.ToLower(strings.TrimSpace(interactionMode)) != constants.PaymentInteractionQR {
		return CryptoWalletInfo{}
	}
	pt := strings.ToLower(strings.TrimSpace(providerType))
	switch pt {
	case constants.PaymentProviderBepusdt:
		return CryptoWalletInfo{
			Address:     readPayloadString(payload, "data", "token"),
			ChainAmount: readPayloadString(payload, "data", "actual_amount"),
			Chain:       readPayloadString(payload, "data", "chain"),
			TokenID:     readPayloadString(payload, "data", "token_id"),
		}
	case constants.PaymentProviderEpusdt:
		return CryptoWalletInfo{
			Address:     readPayloadString(payload, "data", "receive_address"),
			ChainAmount: readPayloadString(payload, "data", "actual_amount"),
		}
	case constants.PaymentProviderDujiaoPay:
		return CryptoWalletInfo{
			Address: firstPayloadString(
				readPayloadString(payload, "pay_address"),
				readPayloadString(payload, "data", "pay_address"),
			),
			ChainAmount: firstPayloadString(
				readPayloadString(payload, "payable_amount"),
				readPayloadString(payload, "data", "payable_amount"),
			),
			Chain: firstPayloadString(
				readPayloadString(payload, "chain"),
				readPayloadString(payload, "data", "chain"),
			),
			TokenID: firstPayloadString(
				readPayloadString(payload, "token_id"),
				readPayloadString(payload, "data", "token_id"),
			),
		}
	default:
		return CryptoWalletInfo{}
	}
}

// ExtractUSDTWalletInfo 从 Payment.ProviderPayload 中提取 USDT 收款钱包地址和链上实付金额。
func ExtractUSDTWalletInfo(providerType, interactionMode string, payload models.JSON) (address, chainAmount string) {
	info := ExtractCryptoWalletInfo(providerType, interactionMode, payload)
	return info.Address, info.ChainAmount
}

// ExtractCryptoPaymentMethods 从 BEpusdt 创建订单响应中提取可选币种和网络。
func ExtractCryptoPaymentMethods(providerType, interactionMode string, payload models.JSON) []CryptoPaymentMethod {
	if !strings.EqualFold(strings.TrimSpace(providerType), constants.PaymentProviderBepusdt) ||
		!strings.EqualFold(strings.TrimSpace(interactionMode), constants.PaymentInteractionQR) {
		return nil
	}
	data, ok := payloadMap(payload["data"])
	if !ok {
		return nil
	}
	raw, ok := data["payment_methods"]
	if !ok || raw == nil {
		return nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var methods []CryptoPaymentMethod
	if err := json.Unmarshal(encoded, &methods); err != nil {
		return nil
	}
	result := make([]CryptoPaymentMethod, 0, len(methods))
	seen := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		method.Currency = strings.ToUpper(strings.TrimSpace(method.Currency))
		method.Network = strings.ToLower(strings.TrimSpace(method.Network))
		if method.Currency == "" || method.Network == "" {
			continue
		}
		key := method.Currency + ":" + method.Network
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, method)
	}
	return result
}

// ExtractSelectedCryptoPaymentMethod 返回当前已选币种和网络。
func ExtractSelectedCryptoPaymentMethod(payload models.JSON) (currency, network string) {
	return strings.ToUpper(readPayloadString(payload, "data", "selected_currency")),
		strings.ToLower(readPayloadString(payload, "data", "selected_network"))
}

func firstPayloadString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// readPayloadString 沿 keys 路径在 payload 内取字符串值，支持 string / json.Number / 数值（fmt 转换）。
func readPayloadString(payload models.JSON, keys ...string) string {
	if payload == nil || len(keys) == 0 {
		return ""
	}
	var cur any = map[string]any(payload)
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			if mj, ok2 := cur.(models.JSON); ok2 {
				m = map[string]any(mj)
			} else {
				return ""
			}
		}
		v, ok := m[k]
		if !ok {
			return ""
		}
		cur = v
	}
	switch v := cur.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.8f", v), "0"), ".")
	case int, int32, int64:
		return strings.TrimSpace(fmt.Sprintf("%d", v))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func payloadMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case models.JSON:
		return map[string]any(typed), true
	default:
		return nil, false
	}
}

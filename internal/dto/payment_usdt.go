package dto

import (
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

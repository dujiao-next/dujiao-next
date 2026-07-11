// Package provider 定义统一的支付网关 Provider interface 与共享类型,
// 所有 adapter wrapper(stripe / paypal / alipay / wechatpay / epay / epusdt /
// bepusdt / tokenpay / okpay)都实现这些 interface,把不一致的包级函数
// 收敛到统一抽象。
//
// 三层 interface 设计:
//   - Provider: 核心能力(所有 adapter 必须实现:Type / ValidateConfig / CreatePayment)
//   - Capturer / Webhooker / CallbackVerifier: 三个可选能力
//
// 可选能力 interface 均嵌入 Provider,以便 Registry 仅持有 Provider,
// caller 通过类型断言 (p.(Capturer))检测能力升级,避免胖接口或 stub 方法。
package provider

import (
	"context"
	"time"

	"github.com/dujiao-next/internal/models"
)

// CreateInput 统一支付创建输入。各 adapter wrapper 把它转成自己的 native 输入。
type CreateInput struct {
	PaymentID      uint
	OrderID        uint
	OrderNo        string
	Subject        string
	Amount         models.Money
	Currency       string
	NotifyURL      string
	ReturnURL      string
	ReturnURLQuery map[string]string // P1.2c Task 3: append 到 ReturnURL 的 query 参数(biz_type/order_no/marker 等)
	ClientIP       string
	ChannelType    string
	Extra          models.JSON
}

// CreateResult 统一支付创建结果。
type CreateResult struct {
	ProviderRef  string
	RedirectURL  string
	QRCodeURL    string
	Payload      models.JSON
	AmountSent   string
	CurrencySent string
}

type SelectPaymentMethodInput struct {
	ProviderRef string
	Currency    string
	Network     string
}

type SelectPaymentMethodResult struct {
	QRCodeURL string
	Payload   models.JSON
}

// QueryResult 主动查询订单状态返回。
type QueryResult struct {
	ProviderRef string
	Status      string
	Amount      models.Money
	Currency    string
	PaidAt      *time.Time
	Payload     models.JSON
}

// CallbackResult 同步回调验签后的结构化结果。
type CallbackResult struct {
	OrderNo     string
	ProviderRef string
	Status      string
	Amount      models.Money
	Currency    string
	PaidAt      *time.Time
	Payload     models.JSON
}

// WebhookResult 是 CallbackResult 的别名:异步 webhook 与同步 callback
// 解析后语义一致,共享同一份结构定义。
type WebhookResult = CallbackResult

// Provider 是所有支付网关 adapter 的核心 interface。
type Provider interface {
	Type() string
	ValidateConfig(cfg models.JSON, channelType string) error
	CreatePayment(ctx context.Context, cfg models.JSON, input CreateInput) (*CreateResult, error)
}

// PaymentMethodSelector 是支持在订单创建后选择链上付款方式的可选能力。
type PaymentMethodSelector interface {
	Provider
	SelectPaymentMethod(ctx context.Context, cfg models.JSON, input SelectPaymentMethodInput) (*SelectPaymentMethodResult, error)
}

// Capturer 可选能力:主动查询订单状态(stripe/paypal/wechat 实现)。
type Capturer interface {
	Provider
	QueryPayment(ctx context.Context, cfg models.JSON, providerRef string) (*QueryResult, error)
}

// Webhooker 可选能力:解析异步 webhook(stripe/paypal/wechat 实现)。
type Webhooker interface {
	Provider
	ParseWebhook(ctx context.Context, cfg models.JSON, headers map[string]string, body []byte, now time.Time) (*WebhookResult, error)
}

// CallbackVerifier 可选能力:验证同步回调表单(alipay/epay/epusdt/bepusdt/tokenpay/okpay 实现)。
type CallbackVerifier interface {
	Provider
	VerifyCallback(cfg models.JSON, form map[string][]string, body []byte) (*CallbackResult, error)
}

package epusdt

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

var (
	ErrConfigInvalid    = errors.New("epusdt config invalid")
	ErrRequestFailed    = errors.New("epusdt request failed")
	ErrResponseInvalid  = errors.New("epusdt response invalid")
	ErrSignatureInvalid = errors.New("epusdt signature invalid")
)

// 订单状态常量（epusdt 文档未明示具体取值，按通用约定实现并对未知值兜底）
const (
	StatusWaiting = 1
	StatusSuccess = 2
	StatusExpired = 3

	gmpayCreateTransactionPath = "/payments/gmpay/v1/order/create-transaction"
	checkoutCounterPathPrefix  = "/pay/checkout-counter/"
)

// Config epusdt（GMPay）配置
type Config struct {
	FixedPaymentIntent bool         `json:"fixed_payment_intent"`
	QuotePolicy        *QuotePolicy `json:"quote_policy,omitempty"`
	GatewayURL         string       `json:"gateway_url"`
	PID                string       `json:"pid"`
	ServiceSecret      string       `json:"service_secret"`
	SecretKey          string       `json:"secret_key"`
	OrderMode          string       `json:"order_mode"` // 订单接口模式：transaction/cashier
	Token              string       `json:"token"`
	Network            string       `json:"network"`
	Currency           string       `json:"currency,omitempty"`
	NotifyURL          string       `json:"notify_url"`
	ReturnURL          string       `json:"return_url"`
	PaymentType        string       `json:"payment_type,omitempty"`
}

// ParseConfig 把 channel.ConfigJSON 反序列化为 Config
func ParseConfig(raw map[string]interface{}) (*Config, error) {
	return common.ParseConfig[Config](raw, ErrConfigInvalid)
}

// Normalize 统一字段格式
func (c *Config) Normalize() {
	if c == nil {
		return
	}
	c.GatewayURL = strings.TrimRight(strings.TrimSpace(c.GatewayURL), "/")
	c.PID = strings.TrimSpace(c.PID)
	c.SecretKey = strings.TrimSpace(c.SecretKey)
	c.OrderMode = strings.ToLower(strings.TrimSpace(c.OrderMode))
	if c.OrderMode == "" {
		c.OrderMode = constants.PaymentEpusdtOrderModeTransaction
	}
	c.Token = strings.ToLower(strings.TrimSpace(c.Token))
	c.Network = strings.ToLower(strings.TrimSpace(c.Network))
	if c.OrderMode == constants.PaymentEpusdtOrderModeCashier {
		c.Token = ""
		c.Network = ""
	}
	c.Currency = strings.ToLower(strings.TrimSpace(c.Currency))
	c.NotifyURL = strings.TrimSpace(c.NotifyURL)
	c.ReturnURL = strings.TrimSpace(c.ReturnURL)
	c.PaymentType = strings.TrimSpace(c.PaymentType)
	if c.OrderMode == constants.PaymentEpusdtOrderModeCashier {
		c.PaymentType = ""
	}
	if c.Currency == "" {
		c.Currency = strings.ToLower(constants.SiteCurrencyDefault)
	}
}

// ValidateConfig 校验必填字段
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("%w: config is nil", ErrConfigInvalid)
	}
	checks := []struct {
		field string
		val   string
	}{
		{"gateway_url", cfg.GatewayURL},
		{"pid", cfg.PID},
		{"secret_key", cfg.SecretKey},
		{"notify_url", cfg.NotifyURL},
		{"return_url", cfg.ReturnURL},
	}
	for _, c := range checks {
		if strings.TrimSpace(c.val) == "" {
			return fmt.Errorf("%w: %s is required", ErrConfigInvalid, c.field)
		}
	}
	orderMode := strings.ToLower(strings.TrimSpace(cfg.OrderMode))
	if orderMode == "" {
		orderMode = constants.PaymentEpusdtOrderModeTransaction
	}
	if orderMode != constants.PaymentEpusdtOrderModeTransaction && orderMode != constants.PaymentEpusdtOrderModeCashier {
		return fmt.Errorf("%w: order_mode is invalid", ErrConfigInvalid)
	}
	if orderMode == constants.PaymentEpusdtOrderModeTransaction {
		if strings.TrimSpace(cfg.Token) == "" {
			return fmt.Errorf("%w: token is required", ErrConfigInvalid)
		}
		if strings.TrimSpace(cfg.Network) == "" {
			return fmt.Errorf("%w: network is required", ErrConfigInvalid)
		}
	}
	return validateQuotePolicy(cfg)
}

// Sign 计算签名。算法：剔除 signature 与空值，key 升序，按 key=value 用 & 连接，
// 用 secret_key 作为 HMAC key 对拼接串做 HMAC-SHA256，小写 hex。
// epusdt 服务端已从旧版 MD5（拼接 secret_key 后缀）迁移到 HMAC-SHA256，且不再接受 MD5 签名，
// 数值格式化必须与 epusdt util/sign.MapToParams 保持一致（float 去除多余尾零）。
func Sign(params map[string]interface{}, secretKey string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "signature" {
			continue
		}
		if isEmptyValue(v) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, formatSignValue(params[k])))
	}

	content := strings.Join(pairs, "&")
	mac := hmac.New(sha256.New, []byte(secretKey))
	_, _ = mac.Write([]byte(content))
	return strings.ToLower(hex.EncodeToString(mac.Sum(nil)))
}

// formatSignValue 按 epusdt util/sign.MapToParams 的规则格式化签名参数值：
// float64 用 strconv.FormatFloat(v, 'f', -1, 64) 去除多余尾零，其余按字符串原样拼接。
func formatSignValue(v interface{}) string {
	switch val := v.(type) {
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

func isEmptyValue(v interface{}) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}

// ToPaymentStatus 把 epusdt 状态码映射为内部 payment 状态
func ToPaymentStatus(status int) string {
	switch status {
	case StatusSuccess:
		return constants.PaymentStatusSuccess
	case StatusExpired:
		return constants.PaymentStatusExpired
	default:
		return constants.PaymentStatusPending
	}
}

// CreateInput 创建订单输入
type CreateInput struct {
	OrderNo   string
	Amount    string
	Name      string
	NotifyURL string
	ReturnURL string
}

// CreateResult 创建订单结果
type CreateResult struct {
	TradeID       string
	PaymentURL    string // {GatewayURL}/pay/checkout-counter/{TradeID}
	Raw           map[string]interface{}
	PaymentIntent jsonmap.JSON
	ExpiresAt     time.Time
}

// CallbackData 回调数据
type CallbackData struct {
	PID                string      `json:"pid"`
	TradeID            string      `json:"trade_id"`
	OrderID            string      `json:"order_id"`
	Amount             interface{} `json:"amount"`
	ActualAmount       interface{} `json:"actual_amount"`
	ReceiveAddress     string      `json:"receive_address"`
	Token              string      `json:"token"`
	BlockTransactionID string      `json:"block_transaction_id"`
	Status             int         `json:"status"`
	Signature          string      `json:"signature"`
}

// GetAmount 兼容 float64 / string 两种 JSON 类型
func (c *CallbackData) GetAmount() float64       { return toFloat(c.Amount) }
func (c *CallbackData) GetActualAmount() float64 { return toFloat(c.ActualAmount) }

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			return f
		}
	}
	return 0
}

// CreatePayment 调用 GMPay 创建交易接口并返回 trade_id 和收银台 URL。
// epusdt 的收银台模式仍使用 create-transaction，只是同时省略 token/network，
// 由服务端创建等待用户选择支付网络和代币的占位订单。
func CreatePayment(ctx context.Context, cfg *Config, input CreateInput) (*CreateResult, error) {
	if cfg == nil {
		return nil, ErrConfigInvalid
	}
	if cfg.FixedPaymentIntent || cfg.QuotePolicy != nil {
		return nil, fmt.Errorf("%w: fixed payments require persisted authorization", ErrConfigInvalid)
	}
	if strings.TrimSpace(input.OrderNo) == "" || strings.TrimSpace(input.Amount) == "" {
		return nil, fmt.Errorf("%w: order_no and amount are required", ErrConfigInvalid)
	}
	if len(input.OrderNo) > 32 {
		return nil, fmt.Errorf("%w: order_no exceeds 32 chars", ErrConfigInvalid)
	}

	amount, err := strconv.ParseFloat(input.Amount, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid amount %q", ErrConfigInvalid, input.Amount)
	}
	if amount <= 0.01 {
		return nil, fmt.Errorf("%w: amount must be greater than 0.01", ErrConfigInvalid)
	}

	notifyURL := input.NotifyURL
	if strings.TrimSpace(notifyURL) == "" {
		notifyURL = cfg.NotifyURL
	}
	returnURL := input.ReturnURL
	if strings.TrimSpace(returnURL) == "" {
		returnURL = cfg.ReturnURL
	}

	params := map[string]interface{}{
		"pid":          cfg.PID,
		"order_id":     input.OrderNo,
		"currency":     cfg.Currency,
		"amount":       amount,
		"notify_url":   notifyURL,
		"redirect_url": returnURL,
	}
	if cfg.OrderMode != constants.PaymentEpusdtOrderModeCashier {
		params["token"] = cfg.Token
		params["network"] = cfg.Network
	}
	if strings.TrimSpace(input.Name) != "" {
		params["name"] = input.Name
	}
	if strings.TrimSpace(cfg.PaymentType) != "" {
		params["payment_type"] = cfg.PaymentType
	}
	params["signature"] = Sign(params, cfg.SecretKey)

	endpoint := cfg.GatewayURL + gmpayCreateTransactionPath
	respBody, err := postJSON(ctx, endpoint, params)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrResponseInvalid, err)
	}

	tradeID := extractTradeID(raw)
	if tradeID == "" {
		return nil, fmt.Errorf("%w: trade_id missing in response", ErrResponseInvalid)
	}

	return &CreateResult{
		TradeID:    tradeID,
		PaymentURL: cfg.GatewayURL + checkoutCounterPathPrefix + tradeID,
		Raw:        raw,
	}, nil
}

// extractTradeID 宽松解析：顶层 / data.trade_id / data.id 任一命中
func extractTradeID(raw map[string]interface{}) string {
	if v, ok := raw["trade_id"].(string); ok && v != "" {
		return v
	}
	if data, ok := raw["data"].(map[string]interface{}); ok {
		if v, ok := data["trade_id"].(string); ok && v != "" {
			return v
		}
		if v, ok := data["id"].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func postJSON(ctx context.Context, endpoint string, params map[string]interface{}) ([]byte, error) {
	return postJSONLimit(ctx, endpoint, params, 64<<10)
}

func postJSONLimit(ctx context.Context, endpoint string, params map[string]interface{}, maxResponseBytes int64) ([]byte, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(responseBody)) > maxResponseBytes {
		return nil, fmt.Errorf("response exceeds limit")
	}
	return responseBody, nil
}

// ParseCallback 解析 epusdt 回调 JSON
func ParseCallback(body []byte) (*CallbackData, error) {
	if len(body) == 0 {
		return nil, ErrResponseInvalid
	}
	var data CallbackData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrResponseInvalid, err)
	}
	return &data, nil
}

// VerifyCallback 验签 + 状态校验。仅 status==StatusSuccess 视为合法成功通知。
func VerifyCallback(cfg *Config, data *CallbackData) error {
	if cfg == nil || data == nil {
		return ErrConfigInvalid
	}
	if data.Status != StatusSuccess {
		return fmt.Errorf("%w: status=%d", ErrResponseInvalid, data.Status)
	}

	params := map[string]interface{}{
		"pid":                  data.PID,
		"trade_id":             data.TradeID,
		"order_id":             data.OrderID,
		"amount":               data.GetAmount(),
		"actual_amount":        data.GetActualAmount(),
		"receive_address":      data.ReceiveAddress,
		"token":                data.Token,
		"block_transaction_id": data.BlockTransactionID,
		"status":               data.Status,
	}
	if strings.TrimSpace(cfg.SecretKey) == "" {
		return ErrConfigInvalid
	}
	expected := Sign(params, cfg.SecretKey)
	if !hmac.Equal([]byte(strings.ToLower(expected)), []byte(strings.ToLower(data.Signature))) {
		return ErrSignatureInvalid
	}
	return nil
}

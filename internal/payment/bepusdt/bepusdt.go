package bepusdt

import (
	"context"
	"crypto/md5"
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
	"github.com/dujiao-next/internal/payment/common"
)

var (
	ErrConfigInvalid       = errors.New("bepusdt config invalid")
	ErrRequestFailed       = errors.New("bepusdt request failed")
	ErrResponseInvalid     = errors.New("bepusdt response invalid")
	ErrSignatureInvalid    = errors.New("bepusdt signature invalid")
	ErrTradeTypeNotSupport = errors.New("bepusdt trade type not supported")
)

// 订单状态常量
const (
	StatusWaiting = 1 // 等待支付
	StatusSuccess = 2 // 支付成功
	StatusExpired = 3 // 支付超时

	bepusdtTradeTypeUSDTTRC20 = "usdt.trc20"
	bepusdtTradeTypeUSDTERC20 = "usdt.erc20"
	bepusdtTradeTypeUSDTBEP20 = "usdt.bep20"
	bepusdtTradeTypeUSDTPOLY  = "usdt.polygon"
	bepusdtTradeTypeUSDCTRC20 = "usdc.trc20"
	bepusdtTradeTypeUSDCERC20 = "usdc.erc20"
	bepusdtTradeTypeUSDCPOLY  = "usdc.polygon"
	bepusdtTradeTypeUSDCBEP20 = "usdc.bep20"
	bepusdtTradeTypeTRX       = "tron.trx"
	bepusdtTradeTypeETH       = "eth.eth"
	bepusdtTradeTypeBNB       = "bsc.bnb"

	bepusdtChannelTypeUSDT      = "usdt"
	bepusdtChannelTypeUSDTTRC20 = "usdt-trc20"
	bepusdtChannelTypeUSDCTRC20 = "usdc-trc20"
	bepusdtChannelTypeTRX       = "trx"

	bepusdtCreateTransactionPath = "/api/v1/order/create-transaction"
	bepusdtCreateOrderPath       = "/api/v1/order/create-order"
	bepusdtUpdateOrderPath       = "/api/v1/pay/update-order"
	bepusdtStatusSuccessMsg      = "status is not success"
)

// Config BEpusdt 配置
type Config struct {
	GatewayURL string `json:"gateway_url"` // 网关地址，如 https://usdt.example.com
	AuthToken  string `json:"auth_token"`  // API Token
	TradeType  string `json:"trade_type"`  // 交易类型，如 usdt.trc20
	Currencies string `json:"currencies"`  // 收银台模式允许的币种，逗号分隔
	Fiat       string `json:"fiat"`        // 法币类型，默认 CNY
	NotifyURL  string `json:"notify_url"`  // 异步通知地址
	ReturnURL  string `json:"return_url"`  // 同步跳转地址
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
	TradeID      string                 // 系统交易 ID
	OrderID      string                 // 商户订单编号
	Amount       string                 // 请求支付金额（法币）
	ActualAmount string                 // 实际支付金额（加密货币）
	Token        string                 // 收款地址
	PaymentURL   string                 // 收银台地址
	TradeType    string                 // 实际交易类型
	Methods      []PaymentMethod        // 可选付款方式
	Raw          map[string]interface{} // 原始响应
}

type PaymentMethod struct {
	Amount          string `json:"amount"`
	ActualAmount    string `json:"actual_amount"`
	Fiat            string `json:"fiat"`
	ExchangeRate    string `json:"exchange_rate"`
	Currency        string `json:"currency"`
	Network         string `json:"network"`
	TokenNetName    string `json:"token_net_name"`
	TokenCustomName string `json:"token_custom_name"`
	IsPopular       bool   `json:"is_popular"`
}

// CallbackData 回调数据
type CallbackData struct {
	TradeID            string      `json:"trade_id"`
	OrderID            string      `json:"order_id"`
	Amount             interface{} `json:"amount"`        // 可能是 float64 或 string
	ActualAmount       interface{} `json:"actual_amount"` // 可能是 float64 或 string
	Token              string      `json:"token"`
	BlockTransactionID string      `json:"block_transaction_id"`
	Signature          string      `json:"signature"`
	Status             int         `json:"status"`
}

// GetAmount 获取金额（float64）
func (c *CallbackData) GetAmount() float64 {
	switch v := c.Amount.(type) {
	case float64:
		return v
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

// GetActualAmount 获取实际金额（float64）
func (c *CallbackData) GetActualAmount() float64 {
	switch v := c.ActualAmount.(type) {
	case float64:
		return v
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

// ParseConfig 解析配置
func ParseConfig(raw map[string]interface{}) (*Config, error) {
	return common.ParseConfig[Config](raw, ErrConfigInvalid)
}

// ValidateConfig 校验配置
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("%w: config is nil", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.GatewayURL) == "" {
		return fmt.Errorf("%w: gateway_url is required", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.AuthToken) == "" {
		return fmt.Errorf("%w: auth_token is required", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.NotifyURL) == "" {
		return fmt.Errorf("%w: notify_url is required", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.ReturnURL) == "" {
		return fmt.Errorf("%w: return_url is required", ErrConfigInvalid)
	}
	return nil
}

func (c *Config) Normalize() {
	c.GatewayURL = strings.TrimRight(strings.TrimSpace(c.GatewayURL), "/")
	c.AuthToken = strings.TrimSpace(c.AuthToken)
	c.TradeType = strings.TrimSpace(c.TradeType)
	c.Currencies = strings.TrimSpace(c.Currencies)
	c.Fiat = strings.TrimSpace(c.Fiat)
	c.NotifyURL = strings.TrimSpace(c.NotifyURL)
	c.ReturnURL = strings.TrimSpace(c.ReturnURL)
	if c.TradeType == "" {
		c.TradeType = bepusdtTradeTypeUSDTTRC20
	}
	if c.Fiat == "" {
		c.Fiat = constants.SiteCurrencyDefault
	}
}

// CreateSelectableOrder 创建由商户前台选择付款币种/网络的订单。
func CreateSelectableOrder(ctx context.Context, cfg *Config, input CreateInput) (*CreateResult, error) {
	if cfg == nil || input.OrderNo == "" || input.Amount == "" {
		return nil, ErrConfigInvalid
	}
	amount, err := strconv.ParseFloat(input.Amount, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid amount", ErrConfigInvalid)
	}
	notifyURL := pickCreateURL(input.NotifyURL, cfg.NotifyURL)
	returnURL := pickCreateURL(input.ReturnURL, cfg.ReturnURL)
	params := map[string]interface{}{
		"order_id":     input.OrderNo,
		"amount":       amount,
		"notify_url":   notifyURL,
		"redirect_url": returnURL,
		"fiat":         cfg.Fiat,
		"reselect":     true,
	}
	if input.Name != "" {
		params["name"] = input.Name
	}
	if cfg.Currencies != "" {
		params["currencies"] = cfg.Currencies
	}
	params["signature"] = Sign(params, cfg.AuthToken)

	respBytes, err := postJSON(ctx, cfg.GatewayURL+bepusdtCreateOrderPath, params)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	var resp struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
		Data       struct {
			Fiat           string          `json:"fiat"`
			TradeID        string          `json:"trade_id"`
			OrderID        string          `json:"order_id"`
			Amount         string          `json:"amount"`
			ExpirationTime int             `json:"expiration_time"`
			PaymentURL     string          `json:"payment_url"`
			Methods        []PaymentMethod `json:"network"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrResponseInvalid, err)
	}
	if resp.StatusCode != 200 || strings.TrimSpace(resp.Data.TradeID) == "" {
		return nil, fmt.Errorf("%w: %s", ErrResponseInvalid, resp.Message)
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(respBytes, &raw)
	return &CreateResult{
		TradeID:    resp.Data.TradeID,
		OrderID:    resp.Data.OrderID,
		Amount:     resp.Data.Amount,
		PaymentURL: resp.Data.PaymentURL,
		Methods:    resp.Data.Methods,
		Raw:        raw,
	}, nil
}

// UpdateSelectableOrder 为待支付订单选定币种和网络，返回对应收款地址。
func UpdateSelectableOrder(ctx context.Context, cfg *Config, tradeID, currency, network string) (*CreateResult, error) {
	if cfg == nil || strings.TrimSpace(tradeID) == "" || strings.TrimSpace(currency) == "" || strings.TrimSpace(network) == "" {
		return nil, ErrConfigInvalid
	}
	params := map[string]interface{}{
		"trade_id": strings.TrimSpace(tradeID),
		"currency": strings.ToUpper(strings.TrimSpace(currency)),
		"network":  strings.ToLower(strings.TrimSpace(network)),
	}
	respBytes, err := postJSON(ctx, cfg.GatewayURL+bepusdtUpdateOrderPath, params)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	var resp struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
		Data       struct {
			Fiat           string `json:"fiat"`
			TradeType      string `json:"trade_type"`
			TradeID        string `json:"trade_id"`
			OrderID        string `json:"order_id"`
			Amount         string `json:"amount"`
			ActualAmount   string `json:"actual_amount"`
			Token          string `json:"token"`
			ExpirationTime int    `json:"expiration_time"`
			PaymentURL     string `json:"payment_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrResponseInvalid, err)
	}
	if resp.StatusCode != 200 || strings.TrimSpace(resp.Data.Token) == "" {
		return nil, fmt.Errorf("%w: %s", ErrResponseInvalid, resp.Message)
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(respBytes, &raw)
	return &CreateResult{
		TradeID:      resp.Data.TradeID,
		OrderID:      resp.Data.OrderID,
		Amount:       resp.Data.Amount,
		ActualAmount: resp.Data.ActualAmount,
		Token:        resp.Data.Token,
		PaymentURL:   resp.Data.PaymentURL,
		TradeType:    resp.Data.TradeType,
		Raw:          raw,
	}, nil
}

func pickCreateURL(primary, fallback string) string {
	if value := strings.TrimSpace(primary); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

// CreatePayment 创建支付订单
func CreatePayment(ctx context.Context, cfg *Config, input CreateInput) (*CreateResult, error) {
	if cfg == nil {
		return nil, ErrConfigInvalid
	}
	if input.OrderNo == "" || input.Amount == "" {
		return nil, ErrConfigInvalid
	}

	notifyURL := input.NotifyURL
	if notifyURL == "" {
		notifyURL = cfg.NotifyURL
	}
	returnURL := input.ReturnURL
	if returnURL == "" {
		returnURL = cfg.ReturnURL
	}

	// 将 amount 从字符串转换为 float64
	amountFloat, err := strconv.ParseFloat(input.Amount, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid amount", ErrConfigInvalid)
	}

	params := map[string]interface{}{
		"order_id":     input.OrderNo,
		"amount":       amountFloat,
		"notify_url":   notifyURL,
		"redirect_url": returnURL,
		"trade_type":   cfg.TradeType,
		"fiat":         cfg.Fiat,
	}
	if input.Name != "" {
		params["name"] = input.Name
	}

	// 生成签名
	signature := Sign(params, cfg.AuthToken)
	params["signature"] = signature

	endpoint := cfg.GatewayURL + bepusdtCreateTransactionPath
	respBytes, err := postJSON(ctx, endpoint, params)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}

	var resp struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
		Data       struct {
			Fiat           string `json:"fiat"`
			TradeID        string `json:"trade_id"`
			OrderID        string `json:"order_id"`
			Amount         string `json:"amount"`
			ActualAmount   string `json:"actual_amount"`
			Token          string `json:"token"`
			ExpirationTime int    `json:"expiration_time"`
			PaymentURL     string `json:"payment_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrResponseInvalid, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%w: %s", ErrResponseInvalid, resp.Message)
	}

	var raw map[string]interface{}
	_ = json.Unmarshal(respBytes, &raw)

	return &CreateResult{
		TradeID:      resp.Data.TradeID,
		OrderID:      resp.Data.OrderID,
		Amount:       resp.Data.Amount,
		ActualAmount: resp.Data.ActualAmount,
		Token:        resp.Data.Token,
		PaymentURL:   resp.Data.PaymentURL,
		Raw:          raw,
	}, nil
}

// VerifyCallback 验证回调签名
func VerifyCallback(cfg *Config, data *CallbackData) error {
	if cfg == nil || data == nil {
		return ErrConfigInvalid
	}

	if data.Status != StatusSuccess {
		return fmt.Errorf("%w: %s", ErrResponseInvalid, bepusdtStatusSuccessMsg)
	}

	params := map[string]interface{}{
		"trade_id":             data.TradeID,
		"order_id":             data.OrderID,
		"amount":               data.GetAmount(),
		"actual_amount":        data.GetActualAmount(),
		"token":                data.Token,
		"block_transaction_id": data.BlockTransactionID,
		"status":               data.Status,
	}

	expected := Sign(params, cfg.AuthToken)
	if !strings.EqualFold(expected, data.Signature) {
		return ErrSignatureInvalid
	}
	return nil
}

// ParseCallback 解析回调数据
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

// Sign 生成签名
// 签名规则：
// 1. 筛选所有非空且非 signature 的参数
// 2. 按参数名 ASCII 码从小到大排序
// 3. 按 key=value 格式拼接，使用 & 连接
// 4. 在末尾追加 AuthToken（无 & 符号）
// 5. MD5 加密并转小写
func Sign(params map[string]interface{}, authToken string) string {
	var keys []string
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

	var pairs []string
	for _, k := range keys {
		v := params[k]
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
	}

	content := strings.Join(pairs, "&") + authToken
	sum := md5.Sum([]byte(content))
	return strings.ToLower(hex.EncodeToString(sum[:]))
}

func isEmptyValue(v interface{}) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val) == ""
	case int, int8, int16, int32, int64:
		return false
	case uint, uint8, uint16, uint32, uint64:
		return false
	case float32, float64:
		return false
	case bool:
		return false
	default:
		return false
	}
}

func postJSON(ctx context.Context, endpoint string, params map[string]interface{}) ([]byte, error) {
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

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// IsSupportedChannelType 判断是否支持的渠道类型
func IsSupportedChannelType(channelType string) bool {
	return ResolveTradeType(channelType) != ""
}

// ResolveTradeType 根据 channel_type 解析 trade_type
func ResolveTradeType(channelType string) string {
	switch strings.ToLower(strings.TrimSpace(channelType)) {
	case bepusdtChannelTypeUSDT, bepusdtChannelTypeUSDTTRC20:
		return bepusdtTradeTypeUSDTTRC20
	case bepusdtChannelTypeUSDCTRC20:
		return bepusdtTradeTypeUSDCTRC20
	case bepusdtChannelTypeTRX:
		return bepusdtTradeTypeTRX
	default:
		return ""
	}
}

// ToPaymentStatus 将 BEpusdt 状态转换为支付状态
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

package epay

import (
	"bytes"
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"
)

const (
	VersionV1 = "v1"
	VersionV2 = "v2"

	epaySignTypeRSA = "RSA"
	epaySignTypeMD5 = "MD5"

	epayAPIPathV2    = "/api/pay/create"
	epayAPIPathV1    = "/mapi.php"
	epaySubmitPathV2 = "/api/pay/submit"
	epaySubmitPathV1 = "/submit.php"

	epayMethodWeb = "web"
	epayDevicePC  = "pc"

	epayHeaderAcceptLanguage = "zh-CN,zh;q=0.9,en;q=0.8"
)

var (
	ErrConfigInvalid     = errors.New("epay config invalid")
	ErrRequestFailed     = errors.New("epay request failed")
	ErrResponseInvalid   = errors.New("epay response invalid")
	ErrChannelTypeNotOK  = errors.New("epay channel type invalid")
	ErrSignatureGenerate = errors.New("epay signature generate failed")
	ErrSignatureInvalid  = errors.New("epay signature invalid")
)

// Config 易支付配置
type Config struct {
	common.ExchangeRateConfig
	GatewayURL  string `json:"gateway_url"`         // 网关地址
	EpayVersion string `json:"epay_version"`        // 版本（v1/v2）
	MerchantID  string `json:"merchant_id"`         // 商户号
	MerchantKey string `json:"merchant_key"`        // 商户密钥（v1）
	PrivateKey  string `json:"private_key"`         // 商户私钥（v2）
	PublicKey   string `json:"platform_public_key"` // 平台公钥（v2）
	SignType    string `json:"sign_type"`           // 签名类型
	APIPath     string `json:"api_path"`            // 接口路径
	NotifyURL   string `json:"notify_url"`          // 异步通知地址
	ReturnURL   string `json:"return_url"`          // 同步跳转地址
	Method      string `json:"method"`              // 支付方式（v2 method）
	Device      string `json:"device"`              // 设备类型（v1 device）
}

// CreateInput 易支付下单输入
type CreateInput struct {
	OrderNo     string
	Amount      string
	Subject     string
	ChannelType string
	ClientIP    string
	NotifyURL   string
	ReturnURL   string
}

// CreateResult 易支付下单结果
type CreateResult struct {
	PayURL  string
	QRCode  string
	TradeNo string
	PayType string
	Raw     map[string]interface{}
}

// ParseConfig 解析配置
func ParseConfig(raw map[string]interface{}) (*Config, error) {
	return common.ParseConfig[Config](raw, ErrConfigInvalid)
}

// ValidateConfig 校验易支付配置完整性
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("%w: config is nil", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.GatewayURL) == "" {
		return fmt.Errorf("%w: gateway_url is required", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.MerchantID) == "" {
		return fmt.Errorf("%w: merchant_id is required", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.NotifyURL) == "" {
		return fmt.Errorf("%w: notify_url is required", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.ReturnURL) == "" {
		return fmt.Errorf("%w: return_url is required", ErrConfigInvalid)
	}
	switch cfg.EpayVersion {
	case VersionV2:
		if strings.TrimSpace(cfg.PrivateKey) == "" {
			return fmt.Errorf("%w: private_key is required for v2", ErrConfigInvalid)
		}
		if strings.TrimSpace(cfg.PublicKey) == "" {
			return fmt.Errorf("%w: platform_public_key is required for v2", ErrConfigInvalid)
		}
	default:
		if strings.TrimSpace(cfg.MerchantKey) == "" {
			return fmt.Errorf("%w: merchant_key is required for v1", ErrConfigInvalid)
		}
	}
	return nil
}

// CreatePayment 发起易支付
func CreatePayment(ctx context.Context, cfg *Config, input CreateInput) (*CreateResult, error) {
	if cfg == nil {
		return nil, ErrConfigInvalid
	}
	if input.OrderNo == "" || input.Amount == "" || input.ClientIP == "" {
		return nil, ErrConfigInvalid
	}
	if input.NotifyURL == "" || input.ReturnURL == "" {
		return nil, ErrConfigInvalid
	}
	if cfg.GatewayURL == "" || cfg.MerchantID == "" {
		return nil, ErrConfigInvalid
	}
	if input.Subject == "" {
		input.Subject = input.OrderNo
	}
	payType := resolvePayType(input.ChannelType)
	if payType == "" {
		return nil, ErrChannelTypeNotOK
	}
	switch cfg.EpayVersion {
	case VersionV2:
		return createV2(ctx, cfg, input, payType)
	default:
		return createV1(ctx, cfg, input, payType)
	}
}

// BuildRedirectURL 构造易支付页面跳转链接。
func BuildRedirectURL(cfg *Config, input CreateInput) (*CreateResult, error) {
	if cfg == nil {
		return nil, ErrConfigInvalid
	}
	if input.OrderNo == "" || input.Amount == "" {
		return nil, ErrConfigInvalid
	}
	if input.NotifyURL == "" || input.ReturnURL == "" {
		return nil, ErrConfigInvalid
	}
	if cfg.GatewayURL == "" || cfg.MerchantID == "" {
		return nil, ErrConfigInvalid
	}
	if input.Subject == "" {
		input.Subject = input.OrderNo
	}
	payType := resolvePayType(input.ChannelType)
	if payType == "" {
		return nil, ErrChannelTypeNotOK
	}
	switch cfg.EpayVersion {
	case VersionV2:
		return buildRedirectV2(cfg, input, payType)
	default:
		return buildRedirectV1(cfg, input, payType)
	}
}

func (c *Config) Normalize() {
	c.EpayVersion = strings.ToLower(strings.TrimSpace(c.EpayVersion))
	c.SignType = strings.TrimSpace(c.SignType)
	if c.SignType == "" {
		if c.EpayVersion == VersionV2 {
			c.SignType = epaySignTypeRSA
		} else {
			c.SignType = epaySignTypeMD5
		}
	}
	if c.APIPath == "" {
		if c.EpayVersion == VersionV2 {
			c.APIPath = epayAPIPathV2
		} else {
			c.APIPath = epayAPIPathV1
		}
	}
	if c.Method == "" {
		c.Method = epayMethodWeb
	}
	if c.Device == "" {
		c.Device = epayDevicePC
	}
	c.ExchangeRateConfig.NormalizeExchangeRate()
}

func createV1(ctx context.Context, cfg *Config, input CreateInput, payType string) (*CreateResult, error) {
	if cfg.MerchantKey == "" {
		return nil, ErrConfigInvalid
	}
	params := map[string]string{
		"pid":          cfg.MerchantID,
		"type":         payType,
		"out_trade_no": input.OrderNo,
		"notify_url":   input.NotifyURL,
		"return_url":   input.ReturnURL,
		"name":         input.Subject,
		"money":        input.Amount,
		"clientip":     input.ClientIP,
		"device":       cfg.Device,
	}
	signContent := buildSignContent(params)
	sign := signMD5(signContent + cfg.MerchantKey)
	params["sign"] = sign
	params["sign_type"] = cfg.SignType

	endpoint := buildEndpoint(cfg.GatewayURL, cfg.APIPath)
	respBytes, err := postForm(ctx, endpoint, params)
	if err != nil {
		return nil, ErrRequestFailed
	}
	respBytes, err = normalizeResponseBody(respBytes)
	if err != nil {
		return nil, ErrResponseInvalid
	}

	var raw map[string]interface{}
	_ = json.Unmarshal(respBytes, &raw)
	var resp struct {
		Code      int    `json:"code"`
		Msg       string `json:"msg"`
		TradeNo   string `json:"trade_no"`
		PayURL    string `json:"payurl"`
		QRCode    string `json:"qrcode"`
		URLScheme string `json:"urlscheme"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, ErrResponseInvalid
	}
	if resp.Code != 1 {
		return nil, fmt.Errorf("%w: %s", ErrResponseInvalid, resp.Msg)
	}
	result := &CreateResult{
		PayURL:  strings.TrimSpace(resp.PayURL),
		QRCode:  strings.TrimSpace(resp.QRCode),
		TradeNo: strings.TrimSpace(resp.TradeNo),
		Raw:     raw,
	}
	if result.PayURL == "" && resp.URLScheme != "" {
		result.PayURL = strings.TrimSpace(resp.URLScheme)
	}
	return result, nil
}

func buildRedirectV1(cfg *Config, input CreateInput, payType string) (*CreateResult, error) {
	if cfg.MerchantKey == "" {
		return nil, ErrConfigInvalid
	}
	params := map[string]string{
		"pid":          cfg.MerchantID,
		"type":         payType,
		"out_trade_no": input.OrderNo,
		"notify_url":   input.NotifyURL,
		"return_url":   input.ReturnURL,
		"name":         input.Subject,
		"money":        input.Amount,
	}
	signContent := buildSignContent(params)
	params["sign"] = signMD5(signContent + cfg.MerchantKey)
	params["sign_type"] = cfg.SignType
	endpoint := buildEndpoint(cfg.GatewayURL, epaySubmitPathV1)
	return buildRedirectResult(endpoint, payType, params), nil
}

// VerifyCallback 验证易支付回调签名
func VerifyCallback(cfg *Config, form map[string][]string) error {
	if cfg == nil {
		return ErrConfigInvalid
	}
	sign := strings.TrimSpace(firstValue(form, "sign"))
	if sign == "" {
		return ErrSignatureInvalid
	}
	params := make(map[string]string, len(form))
	for key, values := range form {
		if len(values) == 0 {
			continue
		}
		params[key] = values[0]
	}
	content := buildSignContent(params)
	switch cfg.EpayVersion {
	case VersionV2:
		return verifyRSA(content, sign, cfg.PublicKey)
	default:
		if strings.TrimSpace(cfg.MerchantKey) == "" {
			return ErrConfigInvalid
		}
		expected := signMD5(content + cfg.MerchantKey)
		if !hmac.Equal([]byte(strings.ToLower(expected)), []byte(strings.ToLower(sign))) {
			return ErrSignatureInvalid
		}
	}
	return nil
}

// VerifyCallbackOwnership 校验易支付回调归属，防止跨商户回调注入。
func VerifyCallbackOwnership(cfg *Config, form map[string][]string) error {
	if cfg == nil {
		return ErrConfigInvalid
	}
	callbackMerchantID := strings.TrimSpace(firstValue(form, "pid"))
	if callbackMerchantID == "" {
		return ErrSignatureInvalid
	}
	if callbackMerchantID != strings.TrimSpace(cfg.MerchantID) {
		return ErrSignatureInvalid
	}
	return nil
}

func createV2(ctx context.Context, cfg *Config, input CreateInput, payType string) (*CreateResult, error) {
	if cfg.PrivateKey == "" {
		return nil, ErrConfigInvalid
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	params := map[string]string{
		"pid":          cfg.MerchantID,
		"method":       cfg.Method,
		"type":         payType,
		"out_trade_no": input.OrderNo,
		"notify_url":   input.NotifyURL,
		"return_url":   input.ReturnURL,
		"name":         input.Subject,
		"money":        input.Amount,
		"clientip":     input.ClientIP,
		"timestamp":    timestamp,
	}
	signContent := buildSignContent(params)
	sign, err := signRSA(signContent, cfg.PrivateKey)
	if err != nil {
		return nil, ErrSignatureGenerate
	}
	params["sign"] = sign
	params["sign_type"] = cfg.SignType

	endpoint := buildEndpoint(cfg.GatewayURL, cfg.APIPath)
	respBytes, err := postForm(ctx, endpoint, params)
	if err != nil {
		return nil, ErrRequestFailed
	}
	respBytes, err = normalizeResponseBody(respBytes)
	if err != nil {
		return nil, ErrResponseInvalid
	}

	var raw map[string]interface{}
	_ = json.Unmarshal(respBytes, &raw)
	var resp struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		TradeNo string `json:"trade_no"`
		PayType string `json:"pay_type"`
		PayInfo string `json:"pay_info"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, ErrResponseInvalid
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("%w: %s", ErrResponseInvalid, resp.Msg)
	}
	result := &CreateResult{
		TradeNo: strings.TrimSpace(resp.TradeNo),
		PayType: strings.TrimSpace(resp.PayType),
		Raw:     raw,
	}
	if strings.EqualFold(result.PayType, constants.EpayPayTypeQRCode) {
		result.QRCode = strings.TrimSpace(resp.PayInfo)
	} else {
		result.PayURL = strings.TrimSpace(resp.PayInfo)
	}
	return result, nil
}

func buildRedirectV2(cfg *Config, input CreateInput, payType string) (*CreateResult, error) {
	if cfg.PrivateKey == "" {
		return nil, ErrConfigInvalid
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	params := map[string]string{
		"pid":          cfg.MerchantID,
		"type":         payType,
		"out_trade_no": input.OrderNo,
		"notify_url":   input.NotifyURL,
		"return_url":   input.ReturnURL,
		"name":         input.Subject,
		"money":        input.Amount,
		"timestamp":    timestamp,
	}
	signContent := buildSignContent(params)
	sign, err := signRSA(signContent, cfg.PrivateKey)
	if err != nil {
		return nil, ErrSignatureGenerate
	}
	params["sign"] = sign
	params["sign_type"] = cfg.SignType
	endpoint := buildEndpoint(cfg.GatewayURL, epaySubmitPathV2)
	// v2 的 RSA sign 含 + / =，放进网关 query 会被伪静态重写破坏。只把已签名字段交给 POST 表单。
	result := newRedirectResult(endpoint, payType, params)
	result.Raw["submit_method"] = http.MethodPost
	return result, nil
}

func resolvePayType(channelType string) string {
	switch strings.ToLower(strings.TrimSpace(channelType)) {
	case constants.PaymentChannelTypeWechat, constants.PaymentChannelTypeWxpay:
		return constants.PaymentChannelTypeWxpay
	case constants.PaymentChannelTypeAlipay:
		return constants.PaymentChannelTypeAlipay
	case constants.PaymentChannelTypeQqpay:
		return constants.PaymentChannelTypeQqpay
	default:
		return ""
	}
}

// IsSupportedChannelType 判断易支付支持的支付方式
func IsSupportedChannelType(channelType string) bool {
	return resolvePayType(channelType) != ""
}

func buildEndpoint(gatewayURL, apiPath string) string {
	base := strings.TrimRight(strings.TrimSpace(gatewayURL), "/")
	path := strings.TrimSpace(apiPath)
	if path == "" {
		return base
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func buildRedirectResult(endpoint, payType string, params map[string]string) *CreateResult {
	values := url.Values{}
	for key, value := range params {
		if strings.TrimSpace(value) == "" {
			continue
		}
		values.Set(key, value)
	}
	redirectURL := endpoint
	encoded := values.Encode()
	if encoded != "" {
		redirectURL += "?" + encoded
	}
	result := newRedirectResult(endpoint, payType, params)
	result.PayURL = redirectURL
	return result
}

func newRedirectResult(endpoint, payType string, params map[string]string) *CreateResult {
	rawParams := make(map[string]interface{}, len(params))
	for key, value := range params {
		rawParams[key] = value
	}
	return &CreateResult{
		PayType: payType,
		Raw: map[string]interface{}{
			"mode":     constants.PaymentInteractionRedirect,
			"endpoint": endpoint,
			"params":   rawParams,
		},
	}
}

func postForm(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	values := url.Values{}
	for k, v := range params {
		if v == "" {
			continue
		}
		values.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept-Language", epayHeaderAcceptLanguage)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrRequestFailed
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// normalizeResponseBody 兼容部分网关返回顶层 string 的双重编码 JSON。
func normalizeResponseBody(body []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return trimmed, nil
	}
	if trimmed[0] != '"' {
		return trimmed, nil
	}

	var inner string
	if err := json.Unmarshal(trimmed, &inner); err != nil {
		return nil, err
	}
	return bytes.TrimSpace([]byte(inner)), nil
}

func buildSignContent(params map[string]string) string {
	var keys []string
	for k, v := range params {
		if v == "" {
			continue
		}
		if k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, params[k]))
	}
	return strings.Join(pairs, "&")
}

func signMD5(content string) string {
	sum := md5.Sum([]byte(content))
	return strings.ToLower(hex.EncodeToString(sum[:]))
}

func signRSA(content, privateKey string) (string, error) {
	key, err := parseRSAPrivateKey(privateKey)
	if err != nil {
		return "", err
	}
	hashed := sha256.Sum256([]byte(content))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hashed[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func verifyRSA(content, signature, publicKey string) error {
	key, err := parseRSAPublicKey(publicKey)
	if err != nil {
		return ErrSignatureInvalid
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return ErrSignatureInvalid
	}
	hashed := sha256.Sum256([]byte(content))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, hashed[:], raw); err != nil {
		return ErrSignatureInvalid
	}
	return nil
}

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "\\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	block, _ := pem.Decode([]byte(normalized))
	if block != nil {
		if strings.Contains(block.Type, "PRIVATE KEY") {
			if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
				if rsaKey, ok := key.(*rsa.PrivateKey); ok {
					return rsaKey, nil
				}
			}
		}
		if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
			return key, nil
		}
	}

	decoded, err := decodeKeyBody(normalized)
	if err != nil {
		return nil, ErrConfigInvalid
	}
	if key, err := x509.ParsePKCS8PrivateKey(decoded); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(decoded); err == nil {
		return key, nil
	}
	return nil, ErrConfigInvalid
}

func parseRSAPublicKey(raw string) (*rsa.PublicKey, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "\\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	block, _ := pem.Decode([]byte(normalized))
	if block != nil {
		if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
			if rsaKey, ok := key.(*rsa.PublicKey); ok {
				return rsaKey, nil
			}
		}
		if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
			return key, nil
		}
	}

	decoded, err := decodeKeyBody(normalized)
	if err != nil {
		return nil, ErrConfigInvalid
	}
	if key, err := x509.ParsePKIXPublicKey(decoded); err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
	}
	if key, err := x509.ParsePKCS1PublicKey(decoded); err == nil {
		return key, nil
	}
	return nil, ErrConfigInvalid
}

func decodeKeyBody(raw string) ([]byte, error) {
	lines := strings.Split(raw, "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "-----BEGIN ") || strings.HasPrefix(trimmed, "-----END ") {
			continue
		}
		parts = append(parts, trimmed)
	}
	if len(parts) == 0 {
		return nil, ErrConfigInvalid
	}
	body := strings.Join(parts, "")
	decoded, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, ErrConfigInvalid
	}
	return decoded, nil
}

func firstValue(form map[string][]string, key string) string {
	if values, ok := form[key]; ok && len(values) > 0 {
		return values[0]
	}
	return ""
}

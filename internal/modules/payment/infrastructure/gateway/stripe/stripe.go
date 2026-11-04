package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"

	"github.com/shopspring/decimal"
)

var (
	ErrConfigInvalid    = errors.New("stripe config invalid")
	ErrRequestFailed    = errors.New("stripe request failed")
	ErrResponseInvalid  = errors.New("stripe response invalid")
	ErrSignatureInvalid = errors.New("stripe signature invalid")
)

const (
	defaultAPIBaseURL        = "https://api.stripe.com"
	defaultTimeout           = 12 * time.Second
	defaultWebhookToleranceS = 300

	stripeObjectCheckoutSession = "checkout.session"
	stripeObjectPaymentIntent   = "payment_intent"

	stripeEventCheckoutSessionCompleted           = "checkout.session.completed"
	stripeEventCheckoutSessionAsyncPaymentSuccess = "checkout.session.async_payment_succeeded"
	stripeEventCheckoutSessionExpired             = "checkout.session.expired"
	stripeEventCheckoutSessionAsyncPaymentFailed  = "checkout.session.async_payment_failed"
	stripeEventPaymentIntentSucceeded             = "payment_intent.succeeded"
	stripeEventPaymentIntentFailed                = "payment_intent.payment_failed"
	stripeEventPaymentIntentCanceled              = "payment_intent.canceled"
	stripeEventPaymentIntentProcessing            = "payment_intent.processing"

	stripePaymentStatusPaid  = "paid"
	stripeSessionExpired     = "expired"
	stripeSessionComplete    = "complete"
	stripePaymentNoRequired  = "no_payment_required"
	stripePIStatusSucceeded  = "succeeded"
	stripePIStatusCanceled   = "canceled"
	stripePIStatusReqPayMeth = "requires_payment_method"
	stripePIStatusProcessing = "processing"
	stripePIStatusReqCapture = "requires_capture"
	stripePIStatusReqAction  = "requires_action"
	stripePIStatusReqConfirm = "requires_confirmation"
)

var zeroDecimalCurrencies = map[string]struct{}{
	"BIF": {},
	"CLP": {},
	"DJF": {},
	"GNF": {},
	"JPY": {},
	"KMF": {},
	"KRW": {},
	"MGA": {},
	"PYG": {},
	"RWF": {},
	"UGX": {},
	"VND": {},
	"VUV": {},
	"XAF": {},
	"XOF": {},
	"XPF": {},
}

// Config Stripe 渠道配置。
type Config struct {
	common.ExchangeRateConfig
	SecretKey               string   `json:"secret_key"`
	PublishableKey          string   `json:"publishable_key"`
	WebhookSecret           string   `json:"webhook_secret"`
	SuccessURL              string   `json:"success_url"`
	CancelURL               string   `json:"cancel_url"`
	APIBaseURL              string   `json:"api_base_url"`
	WebhookToleranceSeconds int      `json:"webhook_tolerance_seconds"`
	PaymentMethodTypes      []string `json:"payment_method_types"`
}

// CreateInput 创建 Stripe 支付输入。
type CreateInput struct {
	OrderNo     string
	Amount      string
	Currency    string
	Description string
	Email       string
	SuccessURL  string
	CancelURL   string
}

// CreateResult 创建 Stripe 支付返回。
type CreateResult struct {
	SessionID       string
	PaymentIntentID string
	URL             string
	Status          string
	Raw             map[string]interface{}
}

// QueryResult 查询 Stripe 支付返回。
type QueryResult struct {
	SessionID       string
	PaymentIntentID string
	Status          string
	Amount          string
	Currency        string
	PaidAt          *time.Time
	Raw             map[string]interface{}
}

// WebhookResult Stripe Webhook 解析结果。
type WebhookResult struct {
	EventID         string
	EventType       string
	OrderNo         string
	ProviderRef     string
	SessionID       string
	PaymentIntentID string
	Status          string
	Amount          string
	Currency        string
	PaidAt          *time.Time
	Raw             map[string]interface{}
}

// ParseConfig 解析配置。
func ParseConfig(raw map[string]interface{}) (*Config, error) {
	return common.ParseConfig[Config](raw, ErrConfigInvalid)
}

// ValidateConfig 校验配置。
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("%w: config is nil", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.SecretKey) == "" {
		return fmt.Errorf("%w: secret_key is required", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.WebhookSecret) == "" {
		return fmt.Errorf("%w: webhook_secret is required", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.SuccessURL) == "" {
		return fmt.Errorf("%w: success_url is required", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.CancelURL) == "" {
		return fmt.Errorf("%w: cancel_url is required", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.APIBaseURL) == "" {
		return fmt.Errorf("%w: api_base_url is required", ErrConfigInvalid)
	}
	if _, err := url.ParseRequestURI(strings.TrimSpace(cfg.APIBaseURL)); err != nil {
		return fmt.Errorf("%w: api_base_url is invalid", ErrConfigInvalid)
	}
	if _, err := url.ParseRequestURI(sanitizeURLForValidation(cfg.SuccessURL)); err != nil {
		return fmt.Errorf("%w: success_url is invalid", ErrConfigInvalid)
	}
	if _, err := url.ParseRequestURI(sanitizeURLForValidation(cfg.CancelURL)); err != nil {
		return fmt.Errorf("%w: cancel_url is invalid", ErrConfigInvalid)
	}
	// payment_method_types 允许为空：空值表示使用 Stripe Dashboard 动态支付方式（新版 API 推荐做法）。
	// 非空时各项由 Normalize() 已 trim/lowercase，此处无需再校验单项格式。
	return nil
}

// CreatePayment 创建 Stripe Checkout Session。
func CreatePayment(ctx context.Context, cfg *Config, input CreateInput) (*CreateResult, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	orderNo := strings.TrimSpace(input.OrderNo)
	if orderNo == "" {
		return nil, fmt.Errorf("%w: order_no is required", ErrConfigInvalid)
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		return nil, fmt.Errorf("%w: currency is required", ErrConfigInvalid)
	}
	minorAmount, err := toMinorAmount(input.Amount, currency)
	if err != nil {
		return nil, err
	}

	successURL := strings.TrimSpace(input.SuccessURL)
	if successURL == "" {
		successURL = cfg.SuccessURL
	}
	cancelURL := strings.TrimSpace(input.CancelURL)
	if cancelURL == "" {
		cancelURL = cfg.CancelURL
	}
	subject := strings.TrimSpace(input.Description)
	if subject == "" {
		subject = orderNo
	}

	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)
	form.Set("client_reference_id", orderNo)
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", strings.ToLower(currency))
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(minorAmount, 10))
	form.Set("line_items[0][price_data][product_data][name]", subject)
	form.Set("metadata[order_no]", orderNo)
	form.Set("payment_intent_data[metadata][order_no]", orderNo)
	if email := strings.TrimSpace(input.Email); email != "" {
		form.Set("customer_email", email)
	}
	for _, pmType := range cfg.PaymentMethodTypes {
		form.Add("payment_method_types[]", pmType)
		// Stripe 要求 Web Checkout 场景下 WeChat Pay 必须显式声明 client，否则返回 400。
		if pmType == "wechat_pay" {
			form.Set("payment_method_options[wechat_pay][client]", "web")
		}
	}

	respBody, statusCode, err := doFormRequest(ctx, cfg, http.MethodPost, "/v1/checkout/sessions", form)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("%w: create checkout session status %d", ErrResponseInvalid, statusCode)
	}

	raw, err := decodeRawMap(respBody)
	if err != nil {
		return nil, err
	}
	result := &CreateResult{
		Raw: raw,
	}
	result.SessionID = strings.TrimSpace(readString(raw, "id"))
	result.URL = strings.TrimSpace(readString(raw, "url"))
	result.Status = strings.TrimSpace(readString(raw, "status"))
	result.PaymentIntentID = strings.TrimSpace(readPaymentIntentID(raw))
	if result.SessionID == "" || result.URL == "" {
		return nil, fmt.Errorf("%w: missing session id or url", ErrResponseInvalid)
	}
	return result, nil
}

// QueryPayment 按 provider_ref 查询 Stripe 支付状态。
func QueryPayment(ctx context.Context, cfg *Config, providerRef string) (*QueryResult, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	providerRef = strings.TrimSpace(providerRef)
	if providerRef == "" {
		return nil, fmt.Errorf("%w: provider_ref is required", ErrConfigInvalid)
	}

	if strings.HasPrefix(providerRef, "cs_") {
		return queryCheckoutSession(ctx, cfg, providerRef)
	}
	if strings.HasPrefix(providerRef, "pi_") {
		return queryPaymentIntent(ctx, cfg, providerRef)
	}

	result, err := queryCheckoutSession(ctx, cfg, providerRef)
	if err == nil {
		return result, nil
	}
	return queryPaymentIntent(ctx, cfg, providerRef)
}

// VerifyAndParseWebhook 校验并解析 Stripe webhook。
func VerifyAndParseWebhook(cfg *Config, headers map[string]string, body []byte, now time.Time) (*WebhookResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: config is nil", ErrConfigInvalid)
	}
	if strings.TrimSpace(cfg.WebhookSecret) == "" {
		return nil, fmt.Errorf("%w: webhook_secret is required", ErrConfigInvalid)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("%w: body is empty", ErrResponseInvalid)
	}
	if now.IsZero() {
		now = time.Now()
	}

	signatureHeader := getHeaderValue(headers, "Stripe-Signature")
	if strings.TrimSpace(signatureHeader) == "" {
		return nil, fmt.Errorf("%w: Stripe-Signature is required", ErrSignatureInvalid)
	}
	timestamp, signatures, err := parseSignatureHeader(signatureHeader)
	if err != nil {
		return nil, err
	}
	if cfg.WebhookToleranceSeconds > 0 {
		delta := math.Abs(float64(now.Unix() - timestamp))
		if delta > float64(cfg.WebhookToleranceSeconds) {
			return nil, fmt.Errorf("%w: timestamp outside tolerance", ErrSignatureInvalid)
		}
	}

	expected := computeSignature(cfg.WebhookSecret, timestamp, body)
	matched := false
	for _, sig := range signatures {
		if hmac.Equal([]byte(strings.ToLower(sig)), []byte(expected)) {
			matched = true
			break
		}
	}
	if !matched {
		return nil, fmt.Errorf("%w: verify failed", ErrSignatureInvalid)
	}

	eventRaw, err := decodeRawMap(body)
	if err != nil {
		return nil, err
	}
	eventType := strings.TrimSpace(readString(eventRaw, "type"))
	if eventType == "" {
		return nil, fmt.Errorf("%w: missing event type", ErrResponseInvalid)
	}
	dataRaw, ok := eventRaw["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: missing data object", ErrResponseInvalid)
	}
	objectRaw, ok := dataRaw["object"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: missing event object", ErrResponseInvalid)
	}

	result := &WebhookResult{
		EventID:   strings.TrimSpace(readString(eventRaw, "id")),
		EventType: eventType,
		Raw:       eventRaw,
	}
	if err := fillWebhookResult(result, eventType, objectRaw); err != nil {
		return nil, err
	}
	return result, nil
}

func queryCheckoutSession(ctx context.Context, cfg *Config, sessionID string) (*QueryResult, error) {
	path := fmt.Sprintf("/v1/checkout/sessions/%s?expand[]=payment_intent", url.PathEscape(strings.TrimSpace(sessionID)))
	respBody, statusCode, err := doJSONRequest(ctx, cfg, http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("%w: query checkout session status %d", ErrResponseInvalid, statusCode)
	}
	raw, err := decodeRawMap(respBody)
	if err != nil {
		return nil, err
	}
	result := &QueryResult{Raw: raw}
	result.SessionID = strings.TrimSpace(readString(raw, "id"))
	result.PaymentIntentID = strings.TrimSpace(readPaymentIntentID(raw))
	result.Currency = strings.ToUpper(strings.TrimSpace(readString(raw, "currency")))
	amountMinor := readInt64(raw, "amount_total")
	if amountMinor > 0 && result.Currency != "" {
		result.Amount = fromMinorAmount(amountMinor, result.Currency)
	}
	result.Status = mapCheckoutSessionStatus(strings.TrimSpace(readString(raw, "payment_status")), strings.TrimSpace(readString(raw, "status")))
	if created := readInt64(raw, "created"); created > 0 {
		paidAt := time.Unix(created, 0)
		result.PaidAt = &paidAt
	}
	if result.SessionID == "" {
		return nil, fmt.Errorf("%w: missing checkout session id", ErrResponseInvalid)
	}
	return result, nil
}

func queryPaymentIntent(ctx context.Context, cfg *Config, paymentIntentID string) (*QueryResult, error) {
	path := fmt.Sprintf("/v1/payment_intents/%s", url.PathEscape(strings.TrimSpace(paymentIntentID)))
	respBody, statusCode, err := doJSONRequest(ctx, cfg, http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("%w: query payment intent status %d", ErrResponseInvalid, statusCode)
	}
	raw, err := decodeRawMap(respBody)
	if err != nil {
		return nil, err
	}
	result := &QueryResult{Raw: raw}
	result.PaymentIntentID = strings.TrimSpace(readString(raw, "id"))
	result.Currency = strings.ToUpper(strings.TrimSpace(readString(raw, "currency")))
	amountMinor := readInt64(raw, "amount_received")
	if amountMinor <= 0 {
		amountMinor = readInt64(raw, "amount")
	}
	if amountMinor > 0 && result.Currency != "" {
		result.Amount = fromMinorAmount(amountMinor, result.Currency)
	}
	result.Status = mapPaymentIntentStatus(strings.TrimSpace(readString(raw, "status")))
	if created := readInt64(raw, "created"); created > 0 {
		paidAt := time.Unix(created, 0)
		result.PaidAt = &paidAt
	}
	if result.PaymentIntentID == "" {
		return nil, fmt.Errorf("%w: missing payment intent id", ErrResponseInvalid)
	}
	return result, nil
}

func fillWebhookResult(result *WebhookResult, eventType string, objectRaw map[string]interface{}) error {
	if result == nil {
		return fmt.Errorf("%w: webhook result is nil", ErrResponseInvalid)
	}
	objectType := strings.TrimSpace(readString(objectRaw, "object"))
	metadata := readMap(objectRaw, "metadata")
	result.OrderNo = strings.TrimSpace(readString(metadata, "order_no"))

	switch objectType {
	case stripeObjectCheckoutSession:
		result.SessionID = strings.TrimSpace(readString(objectRaw, "id"))
		result.PaymentIntentID = strings.TrimSpace(readPaymentIntentID(objectRaw))
		result.ProviderRef = result.SessionID
		result.Currency = strings.ToUpper(strings.TrimSpace(readString(objectRaw, "currency")))
		amountMinor := readInt64(objectRaw, "amount_total")
		if amountMinor > 0 && result.Currency != "" {
			result.Amount = fromMinorAmount(amountMinor, result.Currency)
		}
		if created := readInt64(objectRaw, "created"); created > 0 {
			paidAt := time.Unix(created, 0)
			result.PaidAt = &paidAt
		}
		// checkout.session.completed 对延迟到账支付方式会在 payment_status=unpaid 时触发，
		// 该事件以 session 自身的 payment_status 为准。
		if status, ok := mapEventTypeStatus(eventType); ok && strings.ToLower(strings.TrimSpace(eventType)) != stripeEventCheckoutSessionCompleted {
			result.Status = status
		} else {
			result.Status = mapCheckoutSessionStatus(strings.TrimSpace(readString(objectRaw, "payment_status")), strings.TrimSpace(readString(objectRaw, "status")))
		}
	case stripeObjectPaymentIntent:
		result.PaymentIntentID = strings.TrimSpace(readString(objectRaw, "id"))
		result.ProviderRef = result.PaymentIntentID
		result.Currency = strings.ToUpper(strings.TrimSpace(readString(objectRaw, "currency")))
		amountMinor := readInt64(objectRaw, "amount_received")
		if amountMinor <= 0 {
			amountMinor = readInt64(objectRaw, "amount")
		}
		if amountMinor > 0 && result.Currency != "" {
			result.Amount = fromMinorAmount(amountMinor, result.Currency)
		}
		if created := readInt64(objectRaw, "created"); created > 0 {
			paidAt := time.Unix(created, 0)
			result.PaidAt = &paidAt
		}
		if status, ok := mapEventTypeStatus(eventType); ok {
			result.Status = status
		} else {
			result.Status = mapPaymentIntentStatus(strings.TrimSpace(readString(objectRaw, "status")))
		}
	default:
		if status, ok := mapEventTypeStatus(eventType); ok {
			result.Status = status
		}
	}

	if result.ProviderRef == "" {
		result.ProviderRef = strings.TrimSpace(readString(objectRaw, "id"))
	}
	return nil
}

func mapEventTypeStatus(eventType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case stripeEventCheckoutSessionCompleted, stripeEventCheckoutSessionAsyncPaymentSuccess, stripeEventPaymentIntentSucceeded:
		return constants.PaymentStatusSuccess, true
	case stripeEventCheckoutSessionExpired:
		return constants.PaymentStatusExpired, true
	case stripeEventCheckoutSessionAsyncPaymentFailed, stripeEventPaymentIntentFailed, stripeEventPaymentIntentCanceled:
		return constants.PaymentStatusFailed, true
	case stripeEventPaymentIntentProcessing:
		return constants.PaymentStatusPending, true
	default:
		return "", false
	}
}

func mapCheckoutSessionStatus(paymentStatus string, sessionStatus string) string {
	paymentStatus = strings.ToLower(strings.TrimSpace(paymentStatus))
	sessionStatus = strings.ToLower(strings.TrimSpace(sessionStatus))
	if paymentStatus == stripePaymentStatusPaid {
		return constants.PaymentStatusSuccess
	}
	if sessionStatus == stripeSessionExpired {
		return constants.PaymentStatusExpired
	}
	if sessionStatus == stripeSessionComplete && paymentStatus == stripePaymentNoRequired {
		return constants.PaymentStatusSuccess
	}
	return constants.PaymentStatusPending
}

func mapPaymentIntentStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case stripePIStatusSucceeded:
		return constants.PaymentStatusSuccess
	case stripePIStatusCanceled, stripePIStatusReqPayMeth:
		return constants.PaymentStatusFailed
	case stripePIStatusProcessing, stripePIStatusReqCapture, stripePIStatusReqAction, stripePIStatusReqConfirm:
		return constants.PaymentStatusPending
	default:
		return constants.PaymentStatusPending
	}
}

func sanitizeURLForValidation(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return trimmed
	}
	return strings.ReplaceAll(trimmed, "{CHECKOUT_SESSION_ID}", "cs_test_placeholder")
}

func (c *Config) Normalize() {
	c.SecretKey = strings.TrimSpace(c.SecretKey)
	c.PublishableKey = strings.TrimSpace(c.PublishableKey)
	c.WebhookSecret = strings.TrimSpace(c.WebhookSecret)
	c.SuccessURL = strings.TrimSpace(c.SuccessURL)
	c.CancelURL = strings.TrimSpace(c.CancelURL)
	c.APIBaseURL = strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	if c.APIBaseURL == "" {
		c.APIBaseURL = defaultAPIBaseURL
	}
	if c.WebhookToleranceSeconds <= 0 {
		c.WebhookToleranceSeconds = defaultWebhookToleranceS
	}
	// payment_method_types 为空时保持空列表，表示使用 Stripe Dashboard 动态支付方式（新版 API 推荐）。
	// 非空时做 trim/lowercase/去重，不再 fallback 到 ["card"]。
	normalized := make([]string, 0, len(c.PaymentMethodTypes))
	for _, item := range c.PaymentMethodTypes {
		trimmed := strings.ToLower(strings.TrimSpace(item))
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	c.PaymentMethodTypes = normalized
	c.ExchangeRateConfig.NormalizeExchangeRate()
}

func toMinorAmount(amount string, currency string) (int64, error) {
	parsed, err := decimal.NewFromString(strings.TrimSpace(amount))
	if err != nil {
		return 0, fmt.Errorf("%w: amount is invalid", ErrConfigInvalid)
	}
	if parsed.LessThanOrEqual(decimal.Zero) {
		return 0, fmt.Errorf("%w: amount must be greater than zero", ErrConfigInvalid)
	}
	scale := currencyScale(currency)
	minor := parsed.Shift(int32(scale)).Round(0)
	if !minor.Equal(minor.Truncate(0)) {
		return 0, fmt.Errorf("%w: amount precision is invalid", ErrConfigInvalid)
	}
	return minor.IntPart(), nil
}

func fromMinorAmount(minor int64, currency string) string {
	scale := currencyScale(currency)
	return decimal.NewFromInt(minor).Shift(int32(-scale)).StringFixed(int32(scale))
}

func currencyScale(currency string) int {
	upper := strings.ToUpper(strings.TrimSpace(currency))
	if _, ok := zeroDecimalCurrencies[upper]; ok {
		return 0
	}
	return 2
}

func doFormRequest(ctx context.Context, cfg *Config, method, path string, form url.Values) ([]byte, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.APIBaseURL), "/") + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: build request failed", ErrRequestFailed)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.SecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := (&http.Client{Timeout: defaultTimeout}).Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("%w: read response failed", ErrResponseInvalid)
	}
	return body, resp.StatusCode, nil
}

func doJSONRequest(ctx context.Context, cfg *Config, method, path string) ([]byte, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.APIBaseURL), "/") + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: build request failed", ErrRequestFailed)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.SecretKey)

	resp, err := (&http.Client{Timeout: defaultTimeout}).Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("%w: read response failed", ErrResponseInvalid)
	}
	return body, resp.StatusCode, nil
}

func decodeRawMap(body []byte) (map[string]interface{}, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("%w: decode response failed", ErrResponseInvalid)
	}
	return raw, nil
}

func readPaymentIntentID(raw map[string]interface{}) string {
	if raw == nil {
		return ""
	}
	value, ok := raw["payment_intent"]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]interface{}:
		return strings.TrimSpace(readString(typed, "id"))
	default:
		return ""
	}
}

func computeSignature(secret string, timestamp int64, body []byte) string {
	payload := strconv.FormatInt(timestamp, 10) + "." + string(body)
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(payload))
	return strings.ToLower(hex.EncodeToString(h.Sum(nil)))
}

func parseSignatureHeader(signatureHeader string) (int64, []string, error) {
	timestamp := int64(0)
	signatures := make([]string, 0)
	parts := strings.Split(signatureHeader, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		switch key {
		case "t":
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil || parsed <= 0 {
				return 0, nil, fmt.Errorf("%w: invalid timestamp", ErrSignatureInvalid)
			}
			timestamp = parsed
		case "v1":
			if value != "" {
				signatures = append(signatures, strings.ToLower(value))
			}
		}
	}
	if timestamp <= 0 {
		return 0, nil, fmt.Errorf("%w: timestamp is missing", ErrSignatureInvalid)
	}
	if len(signatures) == 0 {
		return 0, nil, fmt.Errorf("%w: v1 signature is missing", ErrSignatureInvalid)
	}
	return timestamp, signatures, nil
}

func getHeaderValue(headers map[string]string, key string) string {
	if len(headers) == 0 || strings.TrimSpace(key) == "" {
		return ""
	}
	for h, value := range headers {
		if strings.EqualFold(strings.TrimSpace(h), key) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func readString(raw map[string]interface{}, key string) string {
	if raw == nil || strings.TrimSpace(key) == "" {
		return ""
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	case float64:
		return strings.TrimSpace(strconv.FormatInt(int64(typed), 10))
	case int64:
		return strings.TrimSpace(strconv.FormatInt(typed, 10))
	case int:
		return strings.TrimSpace(strconv.Itoa(typed))
	default:
		return ""
	}
}

func readMap(raw map[string]interface{}, key string) map[string]interface{} {
	if raw == nil || strings.TrimSpace(key) == "" {
		return nil
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return nil
	}
	mapped, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	return mapped
}

func readInt64(raw map[string]interface{}, key string) int64 {
	if raw == nil || strings.TrimSpace(key) == "" {
		return 0
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed
		}
		floatVal, err := typed.Float64()
		if err != nil {
			return 0
		}
		return int64(floatVal)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

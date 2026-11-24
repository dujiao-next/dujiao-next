package upstream

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/shopspring/decimal"
)

// YiciyuanAdapter implements the authenticated /shared API. Credentials stay server-side.
type YiciyuanAdapter struct {
	baseURL, appID, appKey string
	client                 *http.Client
	images                 *DujiaoNextAdapter
}

var _ Adapter = (*YiciyuanAdapter)(nil)
var _ OrderReferenceAdapter = (*YiciyuanAdapter)(nil)

func NewYiciyuanAdapter(conn *siteconnectiondomain.Connection, uploadsDir string) *YiciyuanAdapter {
	return &YiciyuanAdapter{
		baseURL: strings.TrimRight(conn.BaseURL, "/"), appID: conn.ApiKey, appKey: conn.ApiSecret,
		client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		images: NewDujiaoNextAdapter(conn, uploadsDir),
	}
}

// signYiciyuan mirrors ksort + http_build_query + urldecode. Empty strings are omitted,
// but zero is retained. Flattened sku[dimension] fields are signed in transmitted order.
func signYiciyuan(values url.Values, key string) string {
	data := url.Values{}
	for k, v := range values {
		if k != "sign" && len(v) > 0 && v[0] != "" {
			data[k] = v
		}
	}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, _, _ := strings.Cut(keys[i], "[")
		right, _, _ := strings.Cut(keys[j], "[")
		if left == right {
			return keys[i] < keys[j]
		}
		return left < right
	})
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, url.Values{k: data[k]}.Encode())
	}
	decoded := ycyPHPURLDecode(strings.Join(parts, "&") + "&key=" + key)
	sum := md5.Sum([]byte(decoded))
	return hex.EncodeToString(sum[:])
}

// PHP urldecode preserves malformed percent escapes, including those in app_key.
func ycyPHPURLDecode(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '+' {
			out.WriteByte(' ')
			continue
		}
		if value[i] == '%' && i+2 < len(value) {
			if decoded, err := hex.DecodeString(value[i+1 : i+3]); err == nil {
				out.WriteByte(decoded[0])
				i += 2
				continue
			}
		}
		out.WriteByte(value[i])
	}
	return out.String()
}

type yiciyuanError struct{ message string }

func (e *yiciyuanError) Error() string { return e.message }

func (a *YiciyuanAdapter) request(ctx context.Context, endpoint string, values url.Values, out any) error {
	base, err := url.Parse(a.baseURL)
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || (base.Path != "" && base.Path != "/") || base.RawQuery != "" || base.Fragment != "" {
		return fmt.Errorf("yiciyuan: base URL must be an HTTP(S) origin")
	}
	if a.appID == "" || a.appKey == "" {
		return fmt.Errorf("yiciyuan: merchant credentials are required")
	}
	form := url.Values{}
	for k, v := range values {
		form[k] = append([]string(nil), v...)
	}
	form.Set("app_id", a.appID)
	form.Set("sign", signYiciyuan(form, a.appKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("yiciyuan: cannot build request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("yiciyuan: %s request failed: %w", endpoint, ctxOrTransportError(ctx, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("yiciyuan: %s HTTP %d", endpoint, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20+1))
	if err != nil || len(body) > 8<<20 {
		return fmt.Errorf("yiciyuan: invalid or oversized response")
	}
	var result struct {
		Code json.Number     `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("yiciyuan: invalid JSON response")
	}
	if result.Code.String() != "200" {
		message := strings.ReplaceAll(result.Msg, a.appKey, "[redacted]")
		if len(message) > 500 {
			message = message[:500]
		}
		if message == "" {
			message = "request rejected"
		}
		return &yiciyuanError{message: "yiciyuan: " + message}
	}
	if out != nil && len(result.Data) > 0 && string(result.Data) != "null" {
		decoder := json.NewDecoder(strings.NewReader(string(result.Data)))
		decoder.UseNumber()
		if err := decoder.Decode(out); err != nil {
			return fmt.Errorf("yiciyuan: invalid business response")
		}
	}
	return nil
}

// Avoid emitting a signed POST body or credentials in transport error messages.
func ctxOrTransportError(ctx context.Context, _ error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return errors.New("transport unavailable")
}

func (a *YiciyuanAdapter) Ping(ctx context.Context) (*PingResult, error) {
	var raw json.RawMessage
	if err := a.request(ctx, "/shared/authentication/connect", nil, &raw); err != nil {
		return nil, err
	}
	var data map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	_ = decoder.Decode(&data)
	host, _ := url.Parse(a.baseURL)
	name := ycyText(data["shopName"])
	if name == "" {
		name = host.Hostname()
	}
	id, _ := strconv.ParseUint(a.appID, 10, 64)
	return &PingResult{SiteName: name, ProtocolVersion: "yiciyuan", UserID: uint(id), Balance: ycyText(data["balance"]), Currency: "CNY"}, nil
}

func (a *YiciyuanAdapter) DownloadImage(ctx context.Context, imageURL string) (string, error) {
	return a.images.DownloadImage(ctx, imageURL)
}

func (a *YiciyuanAdapter) GetOrder(ctx context.Context, id uint) (*UpstreamOrderDetail, error) {
	if id == 0 {
		return nil, fmt.Errorf("yiciyuan: order reference is required")
	}
	return a.GetOrderByReference(ctx, strconv.FormatUint(uint64(id), 10))
}

func (a *YiciyuanAdapter) GetOrderByReference(ctx context.Context, orderNo string) (*UpstreamOrderDetail, error) {
	if strings.TrimSpace(orderNo) == "" {
		return nil, fmt.Errorf("yiciyuan: order reference is required")
	}
	var data map[string]any
	if err := a.request(ctx, "/shared/commodity/query", url.Values{"tradeNo": {orderNo}}, &data); err != nil {
		return nil, err
	}
	result := &UpstreamOrderDetail{OrderNo: orderNo, Status: "pending_payment"}
	if ycyText(data["status"]) == "1" {
		result.Status = "fulfilling"
		result.Fulfillment = ycyFulfillment(data, false)
		if result.Fulfillment != nil {
			result.Status = "completed"
		}
	}
	return result, nil
}

func (a *YiciyuanAdapter) CancelOrder(context.Context, uint) error {
	return fmt.Errorf("%w: yiciyuan API does not provide cancellation", ErrUpstreamCancelNotAllowed)
}

func (a *YiciyuanAdapter) CreateOrder(ctx context.Context, input CreateUpstreamOrderReq) (*CreateUpstreamOrderResp, error) {
	code := ycyText(input.Metadata["code"])
	if code == "" || input.Quantity <= 0 || input.DownstreamOrderNo == "" {
		return nil, fmt.Errorf("yiciyuan: product metadata, quantity and downstream order number are required")
	}
	requestNo := ycyRequestNo(input.DownstreamOrderNo)
	// Recover before stock/price checks: a previous attempt may have bought the last card.
	if recovered := a.recoverPurchase(ctx, requestNo); recovered != nil {
		return recovered, nil
	}
	if min := ycyInt(input.Metadata["minimum"]); min > 0 && input.Quantity < min {
		return ycyRejected("invalid_request", "quantity below upstream minimum"), nil
	}
	if max := ycyInt(input.Metadata["maximum"]); max > 0 && input.Quantity > max {
		return ycyRejected("invalid_request", "quantity exceeds upstream maximum"), nil
	}
	form := ycySelection(code, ycyText(input.Metadata["race"]), ycyMap(input.Metadata["sku"]))
	form.Set("num", strconv.Itoa(input.Quantity))
	var quote map[string]any
	if err := a.request(ctx, "/shared/commodity/valuation", form, &quote); err != nil {
		return a.purchaseFailure(err)
	}
	amount, err := decimal.NewFromString(ycyText(quote["price"]))
	if err != nil || amount.IsNegative() {
		return nil, fmt.Errorf("yiciyuan: invalid quoted amount")
	}
	expected, err := decimal.NewFromString(ycyText(input.Metadata["unit_price"]))
	if err != nil {
		return nil, fmt.Errorf("yiciyuan: missing imported purchase price; synchronize this product first")
	}
	limit := expected.Mul(decimal.NewFromInt(int64(input.Quantity))).Round(2)
	if reference, err := decimal.NewFromString(ycyText(input.Metadata["reference_amount"])); err == nil && ycyInt(input.Metadata["reference_quantity"]) > 0 {
		limit = reference.Mul(decimal.NewFromInt(int64(input.Quantity))).Div(decimal.NewFromInt(int64(ycyInt(input.Metadata["reference_quantity"])))).Round(2)
	}
	if amount.GreaterThan(limit) {
		return ycyRejected("invalid_request", "upstream price increased; synchronize product pricing before retrying"), nil
	}
	quotedCurrency := strings.ToUpper(ycyText(quote["currency_code"]))
	currency := ycyText(input.Metadata["currency"])
	if currency == "" {
		currency = "CNY"
	}
	if quotedCurrency != "" && quotedCurrency != currency {
		return ycyRejected("invalid_request", "upstream currency changed; synchronize product first"), nil
	}
	inventory := url.Values{"shared_code": {code}, "num": {strconv.Itoa(input.Quantity)}, "race": {ycyText(input.Metadata["race"])}, "card_id": {"0"}}
	if err := a.request(ctx, "/shared/commodity/inventoryState", inventory, nil); err != nil {
		return a.purchaseFailure(err)
	}
	form.Del("code")
	form.Set("shared_code", code)
	// Keep valid existing order numbers; hash only incompatible values to the upstream's 32-char alphanumeric limit.
	form.Set("request_no", requestNo)
	form.Set("card_id", "0")
	form.Set("device", "0")
	contact := ycyText(input.ManualFormData["ycy_contact"])
	if contact == "" {
		contact = "api-" + a.appID
	}
	form.Set("contact", contact)
	password := sha256.Sum256([]byte(a.appKey + ":" + requestNo))
	form.Set("password", hex.EncodeToString(password[:8]))
	for local, raw := range ycyMap(input.Metadata["widget_fields"]) {
		remote := ycyText(raw)
		if !ycyWidgetNameAllowed(remote) {
			return nil, fmt.Errorf("yiciyuan: unsafe widget parameter name")
		}
		value := ycyText(input.ManualFormData[local])
		if value == "" {
			return ycyRejected("invalid_request", "required upstream widget is missing"), nil
		}
		form.Set(remote, value)
	}
	var data map[string]any
	if err := a.request(ctx, "/shared/commodity/trade", form, &data); err != nil {
		// A lost response may already have debited the supplier wallet. Recover the
		// reference for manual resolution; queryByRequest does not return the debit
		// amount, so never invent a cost or treat the order as an unpaid rejection.
		recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if recovered := a.recoverPurchase(recoveryCtx, requestNo); recovered != nil {
			return recovered, nil
		}
		var business *yiciyuanError
		if errors.As(err, &business) && (strings.Contains(strings.ToLower(business.message), "request id already exists") || strings.Contains(business.message, "请求号已存在")) {
			return ycyRejected("invalid_request", "upstream request already exists; inspect supplier orders using request_no "+requestNo+"; do not repurchase"), nil
		}
		return a.purchaseFailure(err)
	}
	orderNo := ycyText(data["tradeNo"])
	if orderNo == "" {
		return nil, fmt.Errorf("yiciyuan: trade succeeded without an order reference; inspect supplier orders")
	}
	actual, err := decimal.NewFromString(ycyText(data["amount"]))
	if err != nil || actual.IsNegative() {
		return nil, fmt.Errorf("yiciyuan: trade returned an invalid amount; inspect supplier orders")
	}
	result := &CreateUpstreamOrderResp{OK: true, OrderNo: orderNo, Status: "fulfilling", Amount: actual.StringFixed(2), Currency: currency}
	result.Fulfillment = ycyFulfillment(data, ycyInt(input.Metadata["delivery_way"]) == 0)
	if result.Fulfillment != nil {
		result.Status = "completed"
	}
	return result, nil
}

func (a *YiciyuanAdapter) recoverPurchase(ctx context.Context, requestNo string) *CreateUpstreamOrderResp {
	var recovery map[string]any
	if err := a.request(ctx, "/shared/commodity/queryByRequest", url.Values{"request_no": {requestNo}}, &recovery); err != nil {
		// Older suppliers do not expose this endpoint; their request_no still prevents duplicate debit.
		return nil
	}
	if found, _ := recovery["found"].(bool); found && ycyText(recovery["trade_no"]) != "" {
		return ycyRejected("invalid_request", "upstream order already exists: "+ycyText(recovery["trade_no"])+"; verify supplier debit and fulfill manually; do not repurchase")
	}
	return nil
}

func (a *YiciyuanAdapter) purchaseFailure(err error) (*CreateUpstreamOrderResp, error) {
	var business *yiciyuanError
	if errors.As(err, &business) {
		return ycyRejected("invalid_request", business.message), nil
	}
	return nil, err
}
func ycyRejected(code, message string) *CreateUpstreamOrderResp {
	return &CreateUpstreamOrderResp{OK: false, ErrorCode: code, ErrorMessage: message}
}

func ycyRequestNo(value string) string {
	valid := len(value) > 0 && len(value) <= 32
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			valid = false
		}
	}
	if valid {
		return value
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:16])
}

func ycyFulfillment(data map[string]any, auto bool) *UpstreamFulfillment {
	secret := ycyText(data["secret"])
	if secret == "" {
		return nil
	}
	if raw, ok := data["delivery_status"]; ok {
		if ycyText(raw) != "1" {
			return nil
		}
	} else {
		if !auto {
			return nil
		}
		// Older upstreams omit delivery_status. Never ship a known waiting/review placeholder as a card.
		for _, marker := range []string{"正在发货", "等待发货", "等待处理", "耐心等待", "人工审核", "being prepared", "pending delivery", "waiting for"} {
			if strings.Contains(strings.ToLower(secret), marker) {
				return nil
			}
		}
	}
	return &UpstreamFulfillment{Type: "auto", Status: "delivered", Payload: secret}
}

func ycySelection(code, race string, sku map[string]any) url.Values {
	values := url.Values{"code": {code}, "race": {race}, "card_id": {"0"}}
	for key, value := range sku {
		values.Set("sku["+key+"]", ycyText(value))
	}
	return values
}
func ycyMap(raw any) map[string]any {
	switch v := raw.(type) {
	case map[string]any:
		return v
	case jsonmap.JSON:
		return map[string]any(v)
	}
	return map[string]any{}
}
func ycyText(raw any) string {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}
func ycyInt(raw any) int { n, _ := strconv.Atoi(ycyText(raw)); return n }

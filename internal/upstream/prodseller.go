package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

const (
	prodSellerDefaultBaseURL = "https://prodseller.com/v1"
	prodSellerHTTPTimeout    = 10 * time.Second
	prodSellerMaxBodyBytes   = 4 << 20
)

var (
	// ErrProdSellerAPIKeyMissing 表示连接引用的环境变量不存在或为空。
	ErrProdSellerAPIKeyMissing = errors.New("prodseller api key environment variable is missing")
	// ErrProdSellerAPIKeyRefInvalid 表示秘密引用不是合法的环境变量名。
	ErrProdSellerAPIKeyRefInvalid = errors.New("prodseller api key environment variable name is invalid")
	// ErrProdSellerAuthentication 表示 API Key 无效或已失效。
	ErrProdSellerAuthentication = errors.New("prodseller authentication failed")
	// ErrProdSellerInsufficientBalance 表示上游账户余额不足。
	ErrProdSellerInsufficientBalance = errors.New("prodseller balance is insufficient")
	// ErrProdSellerConflict 表示商品缺货或请求冲突。
	ErrProdSellerConflict = errors.New("prodseller request conflicted")
	// ErrProdSellerRateLimited 表示被上游限流。
	ErrProdSellerRateLimited = errors.New("prodseller rate limited")
	// ErrProdSellerRetryable 表示网络、超时或上游 5xx 等可重试错误。
	ErrProdSellerRetryable = errors.New("prodseller request is retryable")
	// ErrProdSellerInvalidRequest 表示参数错误或不支持的请求。
	ErrProdSellerInvalidRequest = errors.New("prodseller request is invalid")
	// ErrProdSellerWriteDisabled 防止只读阶段误发真实订单。
	ErrProdSellerWriteDisabled = errors.New("prodseller write operation is disabled")
	// ErrProdSellerExternalIDRequired 表示调用方必须提供完整字符串 ID。
	ErrProdSellerExternalIDRequired = errors.New("prodseller external id is required")
)

// ProdSellerAPIError 是 ProdSeller HTTP 错误的脱敏结构。
// Error() 只包含状态、错误码和分类，不包含响应原文，避免把账户信息或密钥相关内容带入日志。
type ProdSellerAPIError struct {
	StatusCode    int
	Code          string
	Kind          error
	RetryAfter    time.Duration
	RateLimit     string
	RateRemaining string
	RateReset     string
}

func (e *ProdSellerAPIError) Error() string {
	if e == nil {
		return "prodseller api error"
	}
	if e.Code != "" {
		return fmt.Sprintf("prodseller api error: status %d (%s)", e.StatusCode, e.Code)
	}
	return fmt.Sprintf("prodseller api error: status %d", e.StatusCode)
}

func (e *ProdSellerAPIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Kind
}

// ProdSellerBalance 是 GET /v1/balance 的脱敏结果。
type ProdSellerBalance struct {
	TelegramID string                 `json:"telegram_id,omitempty"`
	Username   string                 `json:"username,omitempty"`
	Balance    string                 `json:"balance"`
	Currency   string                 `json:"currency,omitempty"`
	Membership map[string]interface{} `json:"membership,omitempty"`
}

// ProdSellerAdapter 实现 ProdSeller v1 的只读客户端。
// apiKey 只存在内存中，来源是 SecretRef 指向的环境变量；连接模型和数据库不保存它。
type ProdSellerAdapter struct {
	baseURL    string
	secretRef  string
	apiKey     string
	uploadsDir string
	client     *http.Client
}

var _ Adapter = (*ProdSellerAdapter)(nil)
var _ ExternalIDAdapter = (*ProdSellerAdapter)(nil)

// NewProdSellerAdapter 从连接引用的环境变量读取 API Key。
func NewProdSellerAdapter(conn *siteconnectiondomain.Connection, uploadsDir string) (*ProdSellerAdapter, error) {
	if conn == nil {
		return nil, ErrProdSellerAPIKeyRefInvalid
	}
	secretRef := strings.TrimSpace(conn.SecretRef)
	if secretRef == "" {
		secretRef = "PRODSELLER_API_KEY"
	}
	if !isEnvironmentVariableName(secretRef) {
		return nil, fmt.Errorf("%w: %q", ErrProdSellerAPIKeyRefInvalid, secretRef)
	}
	apiKey, ok := os.LookupEnv(secretRef)
	if !ok || strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("%w: %s", ErrProdSellerAPIKeyMissing, secretRef)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(conn.BaseURL), "/")
	if baseURL == "" {
		baseURL = prodSellerDefaultBaseURL
	} else if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}

	return &ProdSellerAdapter{
		baseURL:    baseURL,
		secretRef:  secretRef,
		apiKey:     apiKey,
		uploadsDir: uploadsDir,
		client:     &http.Client{Timeout: prodSellerHTTPTimeout},
	}, nil
}

// SecretRef 返回环境变量名，不返回 API Key。
func (a *ProdSellerAdapter) SecretRef() string {
	if a == nil {
		return ""
	}
	return a.secretRef
}

// Ping 通过余额接口测试连接。
func (a *ProdSellerAdapter) Ping(ctx context.Context) (*PingResult, error) {
	balance, err := a.GetBalance(ctx)
	if err != nil {
		return nil, err
	}
	return &PingResult{
		SiteName:        balance.Username,
		ProtocolVersion: "prodseller-v1",
		Balance:         balance.Balance,
		Currency:        balance.Currency,
		MemberLevel:     balance.Membership,
	}, nil
}

// GetBalance 读取 ProdSeller 账户余额。
func (a *ProdSellerAdapter) GetBalance(ctx context.Context) (*ProdSellerBalance, error) {
	var raw json.RawMessage
	if err := a.doJSON(ctx, http.MethodGet, "/balance", nil, &raw); err != nil {
		return nil, err
	}
	return decodeBalance(raw)
}

// ListProducts 读取 ProdSeller 商品列表。ProdSeller 当前商品 ID 保留在 ExternalID。
func (a *ProdSellerAdapter) ListProducts(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error) {
	values := url.Values{}
	if opts.Page > 0 {
		values.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.PageSize > 0 {
		values.Set("page_size", strconv.Itoa(opts.PageSize))
	}
	path := "/products"
	if query := values.Encode(); query != "" {
		path += "?" + query
	}

	var raw json.RawMessage
	if err := a.doJSON(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	return decodeProductList(raw)
}

// GetProduct 在旧的数字 ID 接口上明确拒绝调用，避免字符串 ID 被截断。
func (a *ProdSellerAdapter) GetProduct(context.Context, uint) (*UpstreamProduct, error) {
	return nil, ErrProdSellerExternalIDRequired
}

// GetProductByExternalID 获取完整的 ProdSeller 字符串商品 ID。
func (a *ProdSellerAdapter) GetProductByExternalID(ctx context.Context, productID string) (*UpstreamProduct, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("%w: product id", ErrProdSellerExternalIDRequired)
	}

	var raw json.RawMessage
	path := "/products/" + url.PathEscape(productID)
	if err := a.doJSON(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	return decodeProduct(raw)
}

// ListCategories：ProdSeller 商品 API 没有 Dujiao-Next 分类接口，返回不支持。
func (a *ProdSellerAdapter) ListCategories(context.Context) (*CategoryListResult, error) {
	return &CategoryListResult{Supported: false, Categories: []UpstreamCategory{}}, nil
}

// CreateOrder 在只读阶段永久禁用，避免误扣 ProdSeller 余额。
func (a *ProdSellerAdapter) CreateOrder(context.Context, CreateUpstreamOrderReq) (*CreateUpstreamOrderResp, error) {
	return nil, ErrProdSellerWriteDisabled
}

// GetOrder 在旧的数字 ID接口上明确拒绝，履约阶段使用 GetOrderByExternalID。
func (a *ProdSellerAdapter) GetOrder(context.Context, uint) (*UpstreamOrderDetail, error) {
	return nil, ErrProdSellerExternalIDRequired
}

// GetOrderByExternalID 获取完整的 ProdSeller 字符串订单 ID。
func (a *ProdSellerAdapter) GetOrderByExternalID(ctx context.Context, orderID string) (*UpstreamOrderDetail, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("%w: order id", ErrProdSellerExternalIDRequired)
	}
	var raw json.RawMessage
	if err := a.doJSON(ctx, http.MethodGet, "/orders/"+url.PathEscape(orderID), nil, &raw); err != nil {
		return nil, err
	}
	return decodeOrder(raw)
}

// ListOrdersByExternalID 读取订单列表。ExternalID 由调用方在返回结果中按完整字符串匹配。
func (a *ProdSellerAdapter) ListOrdersByExternalID(ctx context.Context, opts ListOrdersOpts) ([]UpstreamOrderDetail, error) {
	values := url.Values{}
	if opts.Page > 0 {
		values.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.PageSize > 0 {
		values.Set("page_size", strconv.Itoa(opts.PageSize))
	}
	var raw json.RawMessage
	path := "/orders"
	if query := values.Encode(); query != "" {
		path += "?" + query
	}
	if err := a.doJSON(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	orders, err := decodeOrderList(raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.ExternalID) == "" {
		return orders, nil
	}
	filtered := make([]UpstreamOrderDetail, 0, len(orders))
	for _, order := range orders {
		if order.ExternalID == opts.ExternalID || order.OrderNo == opts.ExternalID {
			filtered = append(filtered, order)
		}
	}
	return filtered, nil
}

// CancelOrder 在只读阶段和当前 ProdSeller API 只读边界内不可用。
func (a *ProdSellerAdapter) CancelOrder(context.Context, uint) error {
	return ErrProdSellerWriteDisabled
}

// DownloadImage 当前只读阶段不主动下载第三方图片，导入层会保留原始 URL。
func (a *ProdSellerAdapter) DownloadImage(context.Context, string) (string, error) {
	return "", errors.New("prodseller image download is not enabled")
}

func (a *ProdSellerAdapter) doJSON(ctx context.Context, method, requestPath string, body interface{}, result interface{}) error {
	if a == nil || a.client == nil {
		return fmt.Errorf("%w: adapter is nil", ErrProdSellerInvalidRequest)
	}
	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%w: marshal request", ErrProdSellerInvalidRequest)
		}
		bodyReader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+requestPath, bodyReader)
	if err != nil {
		return fmt.Errorf("%w: create request", ErrProdSellerInvalidRequest)
	}
	req.Header.Set("Accept", "application/json")
	// API Key 只写入内存中的请求头，不进入日志、错误或持久化记录。
	req.Header.Set("X-API-Key", a.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProdSellerRetryable, err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, prodSellerMaxBodyBytes+1))
	if err != nil {
		return fmt.Errorf("%w: read response", ErrProdSellerRetryable)
	}
	if len(responseBody) > prodSellerMaxBodyBytes {
		return fmt.Errorf("%w: response too large", ErrProdSellerRetryable)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return prodSellerHTTPError(resp, responseBody)
	}
	if result == nil || len(bytes.TrimSpace(responseBody)) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, result); err != nil {
		return fmt.Errorf("%w: decode response", ErrProdSellerRetryable)
	}
	return nil
}

func prodSellerHTTPError(resp *http.Response, body []byte) error {
	var payload struct {
		Code    string `json:"code"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &payload)
	code := strings.TrimSpace(payload.Code)
	if code == "" {
		code = strings.TrimSpace(payload.Error)
	}
	kind := ErrProdSellerInvalidRequest
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		kind = ErrProdSellerAuthentication
	case http.StatusPaymentRequired:
		kind = ErrProdSellerInsufficientBalance
	case http.StatusConflict:
		kind = ErrProdSellerConflict
	case http.StatusTooManyRequests:
		kind = ErrProdSellerRateLimited
	case http.StatusRequestTimeout, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		kind = ErrProdSellerRetryable
	}

	err := &ProdSellerAPIError{
		StatusCode:    resp.StatusCode,
		Code:          code,
		Kind:          kind,
		RateLimit:     resp.Header.Get("X-RateLimit-Limit"),
		RateRemaining: resp.Header.Get("X-RateLimit-Remaining"),
		RateReset:     resp.Header.Get("X-RateLimit-Reset"),
	}
	if retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After")); retryAfter != "" {
		if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil && seconds >= 0 {
			err.RetryAfter = time.Duration(seconds) * time.Second
		} else if parsed, parseErr := http.ParseTime(retryAfter); parseErr == nil {
			err.RetryAfter = time.Until(parsed)
			if err.RetryAfter < 0 {
				err.RetryAfter = 0
			}
		}
	}
	return err
}

func isEnvironmentVariableName(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// flexibleString 接受官方响应中常见的 string/number 两种表示，但始终返回完整文本。
type flexibleString string

func (s *flexibleString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*s = ""
		return nil
	}
	var value string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	} else {
		value = string(data)
	}
	*s = flexibleString(strings.TrimSpace(value))
	return nil
}

type prodSellerProduct struct {
	ID               flexibleString  `json:"id"`
	Name             string          `json:"name"`
	Title            json.RawMessage `json:"title"`
	Description      json.RawMessage `json:"description"`
	Price            flexibleString  `json:"price"`
	PublicPrice      flexibleString  `json:"publicPrice"`
	PublicPriceSnake flexibleString  `json:"public_price"`
	ImageURL         string          `json:"imageUrl"`
	ImageURLSnake    string          `json:"image_url"`
	Images           []string        `json:"images"`
	Delivery         struct {
		Type string `json:"type"`
	} `json:"delivery"`
	Stock                   *int            `json:"stock"`
	InStock                 *bool           `json:"inStock"`
	InStockSnake            *bool           `json:"in_stock"`
	RequiresEmailActivation bool            `json:"requiresEmailActivation"`
	RequiresEmailSnake      bool            `json:"requires_email_activation"`
	Sold                    int             `json:"sold"`
	SKUs                    []prodSellerSKU `json:"skus"`
}

type prodSellerSKU struct {
	ID         flexibleString `json:"id"`
	SKUCode    string         `json:"sku"`
	SKUCodeAlt string         `json:"skuCode"`
	Price      flexibleString `json:"price"`
	Stock      int            `json:"stock"`
	InStock    *bool          `json:"inStock"`
	IsActive   *bool          `json:"isActive"`
	SpecValues jsonmap.JSON   `json:"specValues"`
}

func (p prodSellerProduct) toUpstreamProduct() UpstreamProduct {
	publicPrice := string(p.PublicPrice)
	if publicPrice == "" {
		publicPrice = string(p.PublicPriceSnake)
	}
	imageList := append([]string(nil), p.Images...)
	imageURL := strings.TrimSpace(p.ImageURL)
	if imageURL == "" {
		imageURL = strings.TrimSpace(p.ImageURLSnake)
	}
	if imageURL != "" && len(imageList) == 0 {
		imageList = []string{imageURL}
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = localizedRawString(p.Title)
	}
	stock := 0
	inStock := false
	if p.Stock != nil {
		stock = *p.Stock
		inStock = stock > 0
	}
	if p.InStock != nil {
		inStock = *p.InStock
	} else if p.InStockSnake != nil {
		inStock = *p.InStockSnake
	}
	// ProdSeller uses null stock for products whose availability is expressed
	// only by inStock. Preserve that distinction as unlimited stock for the
	// downstream upstream-stock policy; inStock=false remains sold out.
	if p.Stock == nil && inStock {
		stock = -1
	}
	fulfillment := strings.ToLower(strings.TrimSpace(p.Delivery.Type))
	if fulfillment == "instant" || fulfillment == "auto" {
		fulfillment = "auto"
	} else if fulfillment == "" {
		fulfillment = "manual"
	}
	requiresEmail := p.RequiresEmailActivation || p.RequiresEmailSnake

	up := UpstreamProduct{
		ExternalID:        string(p.ID),
		Title:             localizedJSON(name),
		Description:       localizedJSON(localizedRawString(p.Description)),
		Images:            imageList,
		PriceAmount:       string(p.Price),
		PublicPriceAmount: publicPrice,
		Currency:          "USDT",
		FulfillmentType:   fulfillment,
		// /products 已经是活动商品列表；库存单独由 InStock/Stock 表示。
		IsActive:                strings.TrimSpace(string(p.ID)) != "",
		RequiresEmailActivation: requiresEmail,
		InStock:                 inStock,
		Stock:                   stock,
	}
	if p.ID == "" {
		up.IsActive = false
	}
	for _, rawSKU := range p.SKUs {
		skuCode := strings.TrimSpace(rawSKU.SKUCode)
		if skuCode == "" {
			skuCode = strings.TrimSpace(rawSKU.SKUCodeAlt)
		}
		skuActive := rawSKU.IsActive == nil || *rawSKU.IsActive
		skuInStock := rawSKU.Stock > 0
		if rawSKU.InStock != nil {
			skuInStock = *rawSKU.InStock
		}
		if rawSKU.InStock != nil && *rawSKU.InStock && rawSKU.Stock == 0 {
			rawSKU.Stock = -1
		}
		up.SKUs = append(up.SKUs, UpstreamSKU{
			ExternalID:    string(rawSKU.ID),
			SKUCode:       skuCode,
			SpecValues:    rawSKU.SpecValues,
			PriceAmount:   string(rawSKU.Price),
			StockQuantity: rawSKU.Stock,
			// IsActive describes whether the upstream SKU is listed, while
			// StockQuantity carries sold-out state separately.
			IsActive:    skuActive,
			StockStatus: stockStatus(rawSKU.Stock, skuInStock),
		})
	}
	if len(up.SKUs) == 0 {
		up.SKUs = []UpstreamSKU{{
			ExternalID:    string(p.ID),
			SKUCode:       "DEFAULT",
			PriceAmount:   string(p.Price),
			StockQuantity: stock,
			// The /products response already lists this product. Keep its
			// default SKU listed when sold out; stock is checked at order time.
			IsActive:    strings.TrimSpace(string(p.ID)) != "",
			StockStatus: stockStatus(stock, inStock),
		}}
	}
	return up
}

func stockStatus(stock int, inStock bool) string {
	if !inStock || stock == 0 {
		return "out_of_stock"
	}
	if stock < 0 {
		return "unlimited"
	}
	return "in_stock"
}

func localizedJSON(value string) jsonmap.JSON {
	value = strings.TrimSpace(value)
	if value == "" {
		return jsonmap.JSON{}
	}
	return jsonmap.JSON{"zh-CN": value, "zh-TW": value, "en-US": value}
}

func localizedRawString(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var values map[string]interface{}
	if json.Unmarshal(raw, &values) != nil {
		return ""
	}
	for _, key := range []string{"en-US", "zh-CN", "zh-TW", "en", "zh"} {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type prodSellerBalanceEnvelope struct {
	TelegramID  flexibleString             `json:"telegramId"`
	TelegramIDS flexibleString             `json:"telegram_id"`
	Username    string                     `json:"username"`
	Balance     flexibleString             `json:"balance"`
	Currency    string                     `json:"currency"`
	Membership  flexibleMembership         `json:"membership"`
	Data        *prodSellerBalanceEnvelope `json:"data"`
}

// flexibleMembership 兼容 ProdSeller 当前返回的字符串会员等级和旧的对象形式。
// 例如 membership="bronze" 会被归一化为 {"level":"bronze"}，避免 Ping 因字段类型变化失败。
type flexibleMembership map[string]interface{}

func (m *flexibleMembership) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) || len(data) == 0 {
		*m = nil
		return nil
	}
	if data[0] == '{' {
		var object map[string]interface{}
		if err := json.Unmarshal(data, &object); err != nil {
			return err
		}
		*m = flexibleMembership(object)
		return nil
	}
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*m = flexibleMembership{"level": value}
	return nil
}

func decodeBalance(raw json.RawMessage) (*ProdSellerBalance, error) {
	var envelope prodSellerBalanceEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("%w: balance response", ErrProdSellerRetryable)
	}
	if envelope.Data != nil && string(envelope.Balance) == "" {
		envelope = *envelope.Data
	}
	telegramID := string(envelope.TelegramID)
	if telegramID == "" {
		telegramID = string(envelope.TelegramIDS)
	}
	currency := strings.TrimSpace(envelope.Currency)
	if currency == "" {
		currency = "USDT"
	}
	return &ProdSellerBalance{
		TelegramID: telegramID,
		Username:   envelope.Username,
		Balance:    string(envelope.Balance),
		Currency:   currency,
		Membership: map[string]interface{}(envelope.Membership),
	}, nil
}

func decodeProductList(raw json.RawMessage) (*ProductListResult, error) {
	products, total, err := decodeProductArrayOrEnvelope(raw)
	if err != nil {
		return nil, err
	}
	items := make([]UpstreamProduct, 0, len(products))
	for _, product := range products {
		items = append(items, product.toUpstreamProduct())
	}
	if total == 0 {
		total = len(items)
	}
	// ProdSeller /products 返回活动商品集合，不承诺包含已下架商品。
	return &ProductListResult{Total: total, Items: items, IncludesInactive: false}, nil
}

func decodeProduct(raw json.RawMessage) (*UpstreamProduct, error) {
	var envelope struct {
		Product *prodSellerProduct `json:"product"`
		Data    *prodSellerProduct `json:"data"`
	}
	if len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '{' {
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, fmt.Errorf("%w: product response", ErrProdSellerRetryable)
		}
		if envelope.Product != nil {
			result := envelope.Product.toUpstreamProduct()
			return &result, nil
		}
		if envelope.Data != nil {
			result := envelope.Data.toUpstreamProduct()
			return &result, nil
		}
	}
	var product prodSellerProduct
	if err := json.Unmarshal(raw, &product); err != nil {
		return nil, fmt.Errorf("%w: product response", ErrProdSellerRetryable)
	}
	result := product.toUpstreamProduct()
	return &result, nil
}

func decodeProductArrayOrEnvelope(raw json.RawMessage) ([]prodSellerProduct, int, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, 0, fmt.Errorf("%w: empty products response", ErrProdSellerRetryable)
	}
	if trimmed[0] == '[' {
		var products []prodSellerProduct
		if err := json.Unmarshal(trimmed, &products); err != nil {
			return nil, 0, fmt.Errorf("%w: products response", ErrProdSellerRetryable)
		}
		return products, len(products), nil
	}
	var envelope struct {
		Products []prodSellerProduct `json:"products"`
		Items    []prodSellerProduct `json:"items"`
		Data     json.RawMessage     `json:"data"`
		Total    int                 `json:"total"`
		Count    int                 `json:"count"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, 0, fmt.Errorf("%w: products response", ErrProdSellerRetryable)
	}
	products := envelope.Products
	if len(products) == 0 {
		products = envelope.Items
	}
	if len(products) == 0 && len(bytes.TrimSpace(envelope.Data)) > 0 {
		var dataProducts []prodSellerProduct
		if json.Unmarshal(envelope.Data, &dataProducts) == nil {
			products = dataProducts
		} else {
			return decodeProductArrayOrEnvelope(envelope.Data)
		}
	}
	total := envelope.Total
	if total == 0 {
		total = envelope.Count
	}
	return products, total, nil
}

type prodSellerOrder struct {
	ID            flexibleString `json:"id"`
	OrderID       flexibleString `json:"orderId"`
	OrderIDS      flexibleString `json:"order_id"`
	OrderNo       string         `json:"orderNo"`
	Status        string         `json:"status"`
	Amount        flexibleString `json:"amount"`
	Currency      string         `json:"currency"`
	DeliveredKey  string         `json:"deliveredKey"`
	DeliveredKeys []string       `json:"deliveredKeys"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	Data          jsonmap.JSON   `json:"deliveryData"`
}

func (o prodSellerOrder) toUpstreamOrder() UpstreamOrderDetail {
	id := string(o.ID)
	if id == "" {
		id = string(o.OrderID)
	}
	if id == "" {
		id = string(o.OrderIDS)
	}
	payload := o.DeliveredKey
	if len(o.DeliveredKeys) > 0 {
		payload = strings.Join(o.DeliveredKeys, "\n")
	}
	return UpstreamOrderDetail{
		ExternalID: id,
		OrderNo:    o.OrderNo,
		Status:     o.Status,
		Amount:     string(o.Amount),
		Currency:   o.Currency,
		Fulfillment: &UpstreamFulfillment{
			Type:         "instant",
			Status:       o.Status,
			Payload:      payload,
			DeliveryData: o.Data,
			DeliveredAt:  deliveredAt(o.UpdatedAt, o.CreatedAt),
		},
	}
}

func deliveredAt(updated, created time.Time) *time.Time {
	if !updated.IsZero() {
		return &updated
	}
	if !created.IsZero() {
		return &created
	}
	return nil
}

func decodeOrder(raw json.RawMessage) (*UpstreamOrderDetail, error) {
	var envelope struct {
		Order *prodSellerOrder `json:"order"`
		Data  *prodSellerOrder `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("%w: order response", ErrProdSellerRetryable)
	}
	order := envelope.Order
	if order == nil {
		order = envelope.Data
	}
	if order == nil {
		var direct prodSellerOrder
		if err := json.Unmarshal(raw, &direct); err != nil {
			return nil, fmt.Errorf("%w: order response", ErrProdSellerRetryable)
		}
		order = &direct
	}
	result := order.toUpstreamOrder()
	return &result, nil
}

func decodeOrderList(raw json.RawMessage) ([]UpstreamOrderDetail, error) {
	trimmed := bytes.TrimSpace(raw)
	var orders []prodSellerOrder
	if len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &orders); err != nil {
			return nil, fmt.Errorf("%w: orders response", ErrProdSellerRetryable)
		}
	} else {
		var envelope struct {
			Orders []prodSellerOrder `json:"orders"`
			Items  []prodSellerOrder `json:"items"`
			Data   []prodSellerOrder `json:"data"`
		}
		if err := json.Unmarshal(trimmed, &envelope); err != nil {
			return nil, fmt.Errorf("%w: orders response", ErrProdSellerRetryable)
		}
		orders = envelope.Orders
		if len(orders) == 0 {
			orders = envelope.Items
		}
		if len(orders) == 0 {
			orders = envelope.Data
		}
	}
	result := make([]UpstreamOrderDetail, 0, len(orders))
	for _, order := range orders {
		result = append(result, order.toUpstreamOrder())
	}
	return result, nil
}

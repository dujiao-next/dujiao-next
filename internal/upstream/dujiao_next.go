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
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	"github.com/dujiao-next/internal/logger"

	"github.com/google/uuid"
)

const (
	// maxRequestAttempts 单次上游请求的最大尝试次数（含首次）。
	maxRequestAttempts = 3
	// retryWaitMax 单次重试的最长等待，防止上游返回异常大的 Retry-After 卡住调用方。
	retryWaitMax = 30 * time.Second
)

// 客户端节流参数（包级变量，测试可覆盖）。
// 上游默认限流是每个 Key 60 次/分钟。补充间隔 1250ms 即约 48 次/分钟，
// 加上 10 次突发额度，第一分钟最多发出 58 次，压在 60 次/分钟以下；
// 突发额度用来覆盖「打开导入弹窗同时请求列表 + 分类 + 统计」这类正常操作，
// 又不会在批量翻页导入时先把额度打满、触发上游封禁。
var (
	upstreamRequestBurst    = 10
	upstreamRequestInterval = 1250 * time.Millisecond
)

// SetRequestPacing 覆盖客户端节流参数并返回还原函数。
// 供测试使用：其它包遍历大量请求时关闭节流，避免测试被令牌桶拖慢数分钟。
// burst 或 interval 小于等于 0 表示关闭节流。
func SetRequestPacing(burst int, interval time.Duration) (restore func()) {
	oldBurst, oldInterval := upstreamRequestBurst, upstreamRequestInterval
	upstreamRequestBurst, upstreamRequestInterval = burst, interval
	return func() {
		upstreamRequestBurst, upstreamRequestInterval = oldBurst, oldInterval
	}
}

// upstreamPacer 按 baseURL|apiKey 共享的令牌桶。
// GetAdapter 每次调用都会新建适配器实例，节流状态挂在实例上会随每次调用重置，
// 批量导入时就起不到限流作用，所以按远端身份共享。
type upstreamPacer struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

var (
	upstreamPacerMu sync.Mutex
	upstreamPacers  = map[string]*upstreamPacer{}
)

func pacerFor(key string) *upstreamPacer {
	upstreamPacerMu.Lock()
	defer upstreamPacerMu.Unlock()
	p, ok := upstreamPacers[key]
	if !ok {
		p = &upstreamPacer{tokens: float64(upstreamRequestBurst)}
		upstreamPacers[key] = p
	}
	return p
}

// wait 取一个令牌，必要时阻塞到令牌可用。上游已经限流时由重试逻辑兜底。
func (p *upstreamPacer) wait(ctx context.Context) error {
	if upstreamRequestInterval <= 0 || upstreamRequestBurst <= 0 {
		return nil
	}
	for {
		p.mu.Lock()
		now := time.Now()
		if p.last.IsZero() {
			p.last = now
		}
		p.tokens += float64(now.Sub(p.last)) / float64(upstreamRequestInterval)
		if p.tokens > float64(upstreamRequestBurst) {
			p.tokens = float64(upstreamRequestBurst)
		}
		p.last = now
		if p.tokens >= 1 {
			p.tokens--
			p.mu.Unlock()
			return nil
		}
		wait := time.Duration((1 - p.tokens) * float64(upstreamRequestInterval))
		p.mu.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}

// upstreamHTTPError 上游返回非 200 时的结构化错误
type upstreamHTTPError struct {
	Status  int
	Code    string
	Message string
	Body    string
}

func (e *upstreamHTTPError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("upstream responded with status %d (%s): %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("upstream responded with status %d: %s", e.Status, e.Body)
}

// extractUpstreamErrorCode 从错误链中提取 upstreamHTTPError.Code
func extractUpstreamErrorCode(err error) string {
	var ue *upstreamHTTPError
	if errors.As(err, &ue) {
		return ue.Code
	}
	return ""
}

// DujiaoNextAdapter Dujiao-Next 协议适配器
type DujiaoNextAdapter struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	uploadsDir string
	client     *http.Client
}

// NewDujiaoNextAdapter 创建 Dujiao-Next 适配器
func NewDujiaoNextAdapter(conn *siteconnectiondomain.Connection, uploadsDir string) *DujiaoNextAdapter {
	return &DujiaoNextAdapter{
		baseURL:    strings.TrimRight(conn.BaseURL, "/"),
		apiKey:     conn.ApiKey,
		apiSecret:  conn.ApiSecret,
		uploadsDir: uploadsDir,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Ping 连接测试
func (a *DujiaoNextAdapter) Ping(ctx context.Context) (*PingResult, error) {
	var result struct {
		OK bool `json:"ok"`
		PingResult
	}
	if err := a.doRequest(ctx, http.MethodPost, "/api/v1/upstream/ping", nil, &result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("ping failed")
	}
	return &result.PingResult, nil
}

// ListCategories 拉取上游分类列表
func (a *DujiaoNextAdapter) ListCategories(ctx context.Context) (*CategoryListResult, error) {
	var result struct {
		OK         bool               `json:"ok"`
		Categories []UpstreamCategory `json:"categories"`
	}
	if err := a.doRequest(ctx, http.MethodGet, "/api/v1/upstream/categories", nil, &result); err != nil {
		// 旧版上游不支持分类 API，返回空列表
		var ue *upstreamHTTPError
		if errors.As(err, &ue) && ue.Status == http.StatusNotFound {
			return &CategoryListResult{Supported: false, Categories: []UpstreamCategory{}}, nil
		}
		return nil, err
	}
	return &CategoryListResult{Supported: true, Categories: result.Categories}, nil
}

// ListProducts 拉取上游商品列表
func (a *DujiaoNextAdapter) ListProducts(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error) {
	path := fmt.Sprintf("/api/v1/upstream/products?page=%d&page_size=%d", opts.Page, opts.PageSize)
	if opts.UpdatedAfter != nil {
		path += "&updated_after=" + opts.UpdatedAfter.Format(time.RFC3339)
	}
	if opts.IncludeInactive {
		path += "&include_inactive=true"
	}
	var result ProductListResult
	if err := a.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetProduct 获取单个商品详情
// 上游已删除（软删）→ 返回 ErrUpstreamProductDeleted
// 旧版上游对下架商品也返回 404 product_unavailable → 返回 ErrUpstreamProductUnavailable
// 新版上游下架商品改为 200 + is_active=false，调用方应根据 IsActive 字段判断
func (a *DujiaoNextAdapter) GetProduct(ctx context.Context, productID uint) (*UpstreamProduct, error) {
	path := fmt.Sprintf("/api/v1/upstream/products/%d", productID)
	var result struct {
		OK      bool            `json:"ok"`
		Product UpstreamProduct `json:"product"`
	}
	if err := a.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		// 解析上游返回的 error_code 归一化为哨兵错误
		switch extractUpstreamErrorCode(err) {
		case "product_deleted", "product_not_found":
			return nil, ErrUpstreamProductDeleted
		case "product_unavailable":
			return nil, ErrUpstreamProductUnavailable
		}
		return nil, err
	}
	return &result.Product, nil
}

// CreateOrder 发起采购单
func (a *DujiaoNextAdapter) CreateOrder(ctx context.Context, req CreateUpstreamOrderReq) (*CreateUpstreamOrderResp, error) {
	var result CreateUpstreamOrderResp
	if err := a.doRequest(ctx, http.MethodPost, "/api/v1/upstream/orders", req, &result); err != nil {
		// 上游明确拒绝（非 200 且带错误码，如 insufficient_balance）按业务失败返回，
		// 由调用方按错误码决定是否重试；网络/超时等无错误码的错误仍按错误返回。
		var ue *upstreamHTTPError
		if errors.As(err, &ue) && ue.Code != "" {
			return &CreateUpstreamOrderResp{OK: false, ErrorCode: ue.Code, ErrorMessage: ue.Message}, nil
		}
		return nil, err
	}
	return &result, nil
}

// GetOrder 查询上游订单状态
func (a *DujiaoNextAdapter) GetOrder(ctx context.Context, orderID uint) (*UpstreamOrderDetail, error) {
	path := fmt.Sprintf("/api/v1/upstream/orders/%d", orderID)
	var result UpstreamOrderDetail
	if err := a.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CancelOrder 取消采购单
func (a *DujiaoNextAdapter) CancelOrder(ctx context.Context, orderID uint) error {
	path := fmt.Sprintf("/api/v1/upstream/orders/%d/cancel", orderID)
	var result struct {
		OK bool `json:"ok"`
	}
	if err := a.doRequest(ctx, http.MethodPost, path, nil, &result); err != nil {
		var ue *upstreamHTTPError
		if errors.As(err, &ue) && ue.Code == "cancel_not_allowed" {
			return fmt.Errorf("%w: %s", ErrUpstreamCancelNotAllowed, ue.Message)
		}
		return err
	}
	if !result.OK {
		return fmt.Errorf("cancel order failed")
	}
	return nil
}

// maxUpstreamImageBytes 单张上游图片的下载上限，防止异常响应写满磁盘。
const maxUpstreamImageBytes = 10 << 20

var allowedUpstreamImageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}

// DownloadImage 下载图片到本地。
// 只下载与上游站点同主机的图片（含重定向），防止上游返回内网/元数据地址把内部响应落到公开的 /uploads；
// 其它主机的图片由调用方保留原始 URL。
func (a *DujiaoNextAdapter) DownloadImage(ctx context.Context, imageURL string) (string, error) {
	// 相对路径转绝对 URL
	fullURL := imageURL
	if strings.HasPrefix(imageURL, "/") {
		fullURL = a.baseURL + imageURL
	}
	base, err := url.Parse(a.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}
	target, err := url.Parse(fullURL)
	if err != nil || !strings.EqualFold(target.Host, base.Host) {
		return "", fmt.Errorf("download image: host not allowed: %s", fullURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create download request: %w", err)
	}

	client := *a.client
	client.CheckRedirect = func(next *http.Request, _ []*http.Request) error {
		if !strings.EqualFold(next.URL.Host, base.Host) {
			return fmt.Errorf("download image: redirect host not allowed: %s", next.URL.Host)
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download image: status %d", resp.StatusCode)
	}

	ext := strings.ToLower(path.Ext(target.Path))
	if !allowedUpstreamImageExts[ext] {
		ext = ".jpg"
	}

	filename := uuid.New().String() + ext
	dir := filepath.Join(a.uploadsDir, "upstream")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create uploads dir: %w", err)
	}

	filePath := filepath.Join(dir, filename)
	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	written, err := io.Copy(f, io.LimitReader(resp.Body, maxUpstreamImageBytes+1))
	closeErr := f.Close()
	if err == nil && written > maxUpstreamImageBytes {
		err = fmt.Errorf("image exceeds %d bytes", maxUpstreamImageBytes)
	}
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(filePath)
		return "", fmt.Errorf("write file: %w", err)
	}

	// 返回相对路径
	return "/uploads/upstream/" + filename, nil
}

// doRequest 发送签名请求。被限流（429）时按 Retry-After 或退避等待后重试，
// 并对同一远端的所有请求做客户端节流，避免批量导入把上游打满后只导入一部分。
func (a *DujiaoNextAdapter) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
	}

	// 签名用的 path 不含 query string
	signPath := path
	if idx := strings.Index(path, "?"); idx > 0 {
		signPath = path[:idx]
	}

	pacer := pacerFor(a.baseURL + "|" + a.apiKey)

	for attempt := 1; ; attempt++ {
		if err := pacer.wait(ctx); err != nil {
			return err
		}

		timestamp := time.Now().Unix()
		signature := Sign(a.apiSecret, method, signPath, timestamp, bodyBytes)

		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}

		req.Header.Set(HeaderApiKey, a.apiKey)
		req.Header.Set(HeaderTimestamp, fmt.Sprintf("%d", timestamp))
		req.Header.Set(HeaderSignature, signature)
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := a.client.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		if resp.StatusCode == http.StatusOK {
			if result != nil {
				if err := json.Unmarshal(respBody, result); err != nil {
					return fmt.Errorf("unmarshal response: %w", err)
				}
			}
			return nil
		}

		logger.Warnw("upstream_request_error",
			"method", method, "path", path,
			"status", resp.StatusCode, "attempt", attempt, "body", string(respBody))

		if attempt < maxRequestAttempts && shouldRetryUpstream(resp.StatusCode, method) {
			wait := upstreamRetryWait(resp, attempt)
			// 剩余期限不够等待时，这次重试注定等不到，直接返回上游错误：
			// 否则下单库存校验、采购单巡检这类短期限调用会被无谓地拖到超时。
			if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= wait {
				return newUpstreamHTTPError(resp.StatusCode, respBody)
			}
			if waitErr := sleepCtx(ctx, wait); waitErr != nil {
				// 上下文先结束：按上游错误返回，调用方才看得到 429/5xx 的真实原因。
				return newUpstreamHTTPError(resp.StatusCode, respBody)
			}
			continue
		}

		return newUpstreamHTTPError(resp.StatusCode, respBody)
	}
}

func newUpstreamHTTPError(status int, respBody []byte) error {
	var errPayload struct {
		ErrorCode    string `json:"error_code"`
		ErrorMessage string `json:"error_message"`
	}
	_ = json.Unmarshal(respBody, &errPayload)
	return &upstreamHTTPError{
		Status:  status,
		Code:    errPayload.ErrorCode,
		Message: errPayload.ErrorMessage,
		Body:    string(respBody),
	}
}

// shouldRetryUpstream 429 表示请求被限流拒绝、上游没有处理，任何方法重试都安全；
// 网关类 5xx 只对幂等请求重试，避免重复创建采购单。
func shouldRetryUpstream(status int, method string) bool {
	if status == http.StatusTooManyRequests {
		return true
	}
	if method == http.MethodGet || method == http.MethodHead {
		switch status {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
	}
	return false
}

func upstreamRetryWait(resp *http.Response, attempt int) time.Duration {
	wait := time.Duration(attempt) * time.Second
	if header := strings.TrimSpace(resp.Header.Get("Retry-After")); header != "" {
		if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
			wait = time.Duration(seconds) * time.Second
		}
	}
	if wait > retryWaitMax {
		wait = retryWaitMax
	}
	return wait
}

func sleepCtx(ctx context.Context, wait time.Duration) error {
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

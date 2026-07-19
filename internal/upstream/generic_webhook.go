package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dujiao-next/internal/models"
)

const genericWebhookResponseLimit = 1 << 20

// RequestError describes an outbound protocol error and whether retrying may succeed.
type RequestError struct {
	StatusCode int
	Code       string
	Message    string
	Retryable  bool
}

func (e *RequestError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("webhook responded with status %d: %s", e.StatusCode, e.Message)
	}
	return e.Message
}

// IsRetryableRequestError defaults unknown transport errors to retryable, preserving
// the existing Dujiao-Next procurement behavior.
func IsRetryableRequestError(err error) bool {
	var requestErr *RequestError
	if errors.As(err, &requestErr) {
		return requestErr.Retryable
	}
	return true
}

type GenericWebhookAdapter struct {
	endpointURL string
	token       string
	client      *http.Client
}

func NewGenericWebhookAdapter(conn *models.SiteConnection) *GenericWebhookAdapter {
	return &GenericWebhookAdapter{
		endpointURL: strings.TrimSpace(conn.BaseURL),
		token:       conn.ApiKey,
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *GenericWebhookAdapter) Ping(ctx context.Context) (*PingResult, error) {
	payload := map[string]interface{}{
		"event":     "connection.test",
		"timestamp": time.Now().Unix(),
	}
	responseBody, err := a.post(ctx, payload, "")
	if err != nil {
		return nil, err
	}
	if len(responseBody) > 0 {
		var response struct {
			OK      *bool  `json:"ok"`
			Message string `json:"message"`
		}
		if json.Unmarshal(responseBody, &response) == nil && response.OK != nil && !*response.OK {
			message := strings.TrimSpace(response.Message)
			if message == "" {
				message = "connection test rejected"
			}
			return nil, &RequestError{Code: "connection_rejected", Message: message, Retryable: false}
		}
	}
	return &PingResult{
		SiteName:        "Generic Webhook",
		ProtocolVersion: "1.0",
	}, nil
}

func (a *GenericWebhookAdapter) CreateOrder(ctx context.Context, req CreateUpstreamOrderReq) (*CreateUpstreamOrderResp, error) {
	payload := struct {
		Event          string      `json:"event"`
		OrderNo        string      `json:"order_no"`
		TraceID        string      `json:"trace_id"`
		CallbackURL    string      `json:"callback_url"`
		ProductID      uint        `json:"product_id"`
		SKUID          uint        `json:"sku_id"`
		SKUCode        string      `json:"sku_code"`
		Quantity       int         `json:"quantity"`
		Amount         string      `json:"amount"`
		Currency       string      `json:"currency"`
		ManualFormData models.JSON `json:"manual_form_data,omitempty"`
		Timestamp      int64       `json:"timestamp"`
	}{
		Event:          "order.created",
		OrderNo:        req.DownstreamOrderNo,
		TraceID:        req.TraceID,
		CallbackURL:    req.CallbackURL,
		ProductID:      req.LocalProductID,
		SKUID:          req.LocalSKUID,
		SKUCode:        req.LocalSKUCode,
		Quantity:       req.Quantity,
		Amount:         req.Amount,
		Currency:       req.Currency,
		ManualFormData: req.ManualFormData,
		Timestamp:      time.Now().Unix(),
	}

	responseBody, err := a.post(ctx, payload, req.DownstreamOrderNo)
	if err != nil {
		return nil, err
	}

	result := &CreateUpstreamOrderResp{
		OK:       true,
		Status:   "accepted",
		Amount:   req.Amount,
		Currency: req.Currency,
	}
	if len(responseBody) == 0 {
		return result, nil
	}

	var response struct {
		OK           *bool  `json:"ok"`
		OrderNo      string `json:"order_no"`
		ErrorCode    string `json:"error_code"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		// Any 2xx response is accepted; response JSON is optional.
		return result, nil
	}
	if response.OK != nil && !*response.OK {
		result.OK = false
		result.ErrorCode = strings.TrimSpace(response.ErrorCode)
		if result.ErrorCode == "" {
			result.ErrorCode = "webhook_rejected"
		}
		result.ErrorMessage = strings.TrimSpace(response.ErrorMessage)
		return result, nil
	}
	result.OrderNo = strings.TrimSpace(response.OrderNo)
	return result, nil
}

func (a *GenericWebhookAdapter) post(ctx context.Context, payload interface{}, idempotencyKey string) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal webhook request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, &RequestError{Code: "invalid_endpoint", Message: err.Error(), Retryable: false}
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, &RequestError{Code: "transport_error", Message: err.Error(), Retryable: true}
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, genericWebhookResponseLimit))
	if readErr != nil {
		return nil, &RequestError{StatusCode: resp.StatusCode, Code: "read_error", Message: readErr.Error(), Retryable: true}
	}
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return responseBody, nil
	}

	retryable := resp.StatusCode == http.StatusRequestTimeout ||
		resp.StatusCode == http.StatusTooManyRequests ||
		resp.StatusCode >= http.StatusInternalServerError
	message := strings.TrimSpace(string(responseBody))
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return nil, &RequestError{
		StatusCode: resp.StatusCode,
		Code:       fmt.Sprintf("http_%d", resp.StatusCode),
		Message:    message,
		Retryable:  retryable,
	}
}

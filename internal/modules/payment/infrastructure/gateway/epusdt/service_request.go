package epusdt

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/epusdt/securehttp"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/epusdt/servicemac"
	"io"
	"net/http"
)

// Legacy non-fixed deployments retain their existing protocol. The dedicated
// secure gateway requires this envelope in addition to the original signature.
func servicePost(ctx context.Context, cfg *Config, endpoint string, params map[string]interface{}, maximum int64) ([]byte, error) {
	if cfg.ServiceSecret == "" {
		return postJSONLimit(ctx, endpoint, params, maximum)
	}
	if len(cfg.ServiceSecret) < 32 {
		return nil, ErrConfigInvalid
	}
	client, err := securehttp.Client(endpoint)
	if err != nil {
		return nil, ErrConfigInvalid
	}
	body, err := json.Marshal(params)
	if err != nil {
		return nil, ErrConfigInvalid
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, ErrConfigInvalid
	}
	req.Header.Set("Content-Type", "application/json")
	if err = servicemac.Sign(req, body, cfg.ServiceSecret); err != nil {
		return nil, ErrResponseInvalid
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrResponseInvalid
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maximum+1))
	if err != nil || int64(len(b)) > maximum || resp.StatusCode != 200 {
		return nil, ErrResponseInvalid
	}
	return b, nil
}

package epusdt

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
)

const candidatePath = "/payments/gmpay/v1/order/transfer-candidates"

func CandidateSource(cfg *Config) (paymentdomain.CandidateSource, bool, error) {
	if cfg == nil {
		return paymentdomain.CandidateSource{}, false, ErrConfigInvalid
	}
	if !cfg.FixedPaymentIntent {
		return paymentdomain.CandidateSource{}, false, nil
	}
	if err := ValidateConfig(cfg); err != nil {
		return paymentdomain.CandidateSource{}, false, err
	}
	return paymentdomain.CandidateSource{GatewayURL: cfg.GatewayURL, PID: cfg.PID}, true, nil
}

func validCandidateID(id string) bool {
	n, err := strconv.ParseUint(id, 10, 64)
	return err == nil && n > 0 && strconv.FormatUint(n, 10) == id
}

// Only structural validation belongs here. Even valid fields are untrusted;
// malformed economic facts within these bounds remain available for review.
func boundedCandidate(c paymentdomain.TransferCandidate) bool {
	if !validCandidateID(c.ID) || len(c.ObservationKey) > 64 || c.OrderID == "" || len(c.OrderID) > 64 || c.TradeID == "" || len(c.TradeID) > 64 {
		return false
	}
	return len(c.Network) <= 32 && len(c.Kind) <= 32 && len(c.TransactionHash) <= 66 && len(c.BlockHash) <= 66 &&
		len(c.Contract) <= 64 && len(c.Recipient) <= 64 && len(c.RawAmount) <= 80 && len(c.ReviewReason) <= 32
}

func ListCandidates(ctx context.Context, cfg *Config) ([]paymentdomain.TransferCandidate, error) {
	if _, enabled, err := CandidateSource(cfg); err != nil || !enabled {
		return nil, ErrConfigInvalid
	}
	params := map[string]interface{}{"pid": cfg.PID, "action": "transfer_candidates.list.v1"}
	params["signature"] = Sign(params, cfg.SecretKey)
	// A 100-row gateway batch can exceed the create endpoint's 64 KiB limit.
	b, err := servicePost(ctx, cfg, cfg.GatewayURL+candidatePath, params, 512<<10)
	if err != nil {
		return nil, ErrRequestFailed // Never leak the credential-bearing endpoint.
	}
	var response struct {
		StatusCode int                               `json:"status_code"`
		Data       []paymentdomain.TransferCandidate `json:"data"`
	}
	if json.Unmarshal(b, &response) != nil || response.StatusCode != 200 || response.Data == nil || len(response.Data) > 100 {
		return nil, ErrResponseInvalid
	}
	seen := map[string]bool{}
	for _, c := range response.Data {
		if !boundedCandidate(c) || seen[c.ID] {
			return nil, ErrResponseInvalid
		}
		seen[c.ID] = true
	}
	return response.Data, nil
}

func AcknowledgeCandidates(ctx context.Context, cfg *Config, ids []string) error {
	if _, enabled, err := CandidateSource(cfg); err != nil || !enabled || len(ids) == 0 || len(ids) > 100 {
		return ErrConfigInvalid
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !validCandidateID(id) || seen[id] {
			return ErrResponseInvalid
		}
		seen[id] = true
	}
	params := map[string]interface{}{"pid": cfg.PID, "action": "transfer_candidates.ack.v1", "ids": strings.Join(ids, ",")}
	params["signature"] = Sign(params, cfg.SecretKey)
	b, err := servicePost(ctx, cfg, cfg.GatewayURL+candidatePath+"/ack", params, 64<<10)
	if err != nil {
		return ErrRequestFailed
	}
	var response struct {
		StatusCode int `json:"status_code"`
		Data       struct {
			Received bool `json:"received"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &response) != nil || response.StatusCode != 200 || !response.Data.Received {
		return ErrResponseInvalid
	}
	return nil
}

func ScanHealthy(ctx context.Context, cfg *Config) (bool, error) {
	if cfg.ServiceSecret == "" {
		return false, nil
	}
	params := map[string]interface{}{"pid": cfg.PID, "action": "scan_health.v1"}
	params["signature"] = Sign(params, cfg.SecretKey)
	b, err := servicePost(ctx, cfg, cfg.GatewayURL+"/payments/gmpay/v1/order/scan-health", params, 64<<10)
	if err != nil {
		return false, err
	}
	var result struct {
		StatusCode int  `json:"status_code"`
		Healthy    bool `json:"healthy"`
	}
	if json.Unmarshal(b, &result) != nil || result.StatusCode != 200 {
		return false, ErrResponseInvalid
	}
	return result.Healthy, nil
}

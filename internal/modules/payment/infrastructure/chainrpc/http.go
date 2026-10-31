package chainrpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
)

func publicRPCAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	for _, cidr := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/32", "2002::/16", "64:ff9b::/96"} {
		if netip.MustParsePrefix(cidr).Contains(ip) {
			return false
		}
	}
	return true
}

func rpcHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	transport.DialTLSContext = nil
	transport.Proxy = nil // Ambient proxies must not redirect this trust boundary.
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, paymentcontract.ErrChainUnavailable
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil || len(ips) == 0 {
			return nil, paymentcontract.ErrChainUnavailable
		}
		for _, ip := range ips {
			if !publicRPCAddress(ip) {
				return nil, paymentcontract.ErrChainUnavailable
			}
		}
		for _, ip := range ips {
			conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
		}
		return nil, paymentcontract.ErrChainUnavailable
	}
	transport.MaxResponseHeaderBytes = 32 << 10
	transport.ResponseHeaderTimeout = 7 * time.Second
	return &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (v *Verifier) post(ctx context.Context, source Source, path string, input, output interface{}) error {
	if limiter := v.limits[strings.ToLower(strings.TrimSpace(source.Provider))]; limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			return paymentcontract.ErrChainUnavailable
		}
	}
	body, err := json.Marshal(input)
	if err != nil {
		return paymentcontract.ErrChainUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(source.URL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return paymentcontract.ErrChainUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	if source.APIKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", source.APIKey)
	}
	response, err := v.client.Do(req)
	if err != nil {
		return paymentcontract.ErrChainUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return paymentcontract.ErrChainUnavailable
	}
	const maximum = 4 << 20 // Full block transaction lists are bounded too.
	b, err := io.ReadAll(io.LimitReader(response.Body, maximum+1))
	if err != nil || len(b) > maximum || json.Unmarshal(b, output) != nil {
		return paymentcontract.ErrChainUnavailable
	}
	return nil
}

func (v *Verifier) evmCall(ctx context.Context, source Source, method string, params interface{}, out interface{}) error {
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if err := v.post(ctx, source, "", map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}, &response); err != nil {
		return err
	}
	if response.JSONRPC != "2.0" || string(response.ID) != "1" || (len(response.Error) > 0 && string(response.Error) != "null") {
		return paymentcontract.ErrChainUnavailable
	}
	if len(response.Result) == 0 || string(response.Result) == "null" {
		return paymentcontract.ErrChainPending
	}
	if json.Unmarshal(response.Result, out) != nil {
		return paymentcontract.ErrChainUnavailable
	}
	return nil
}

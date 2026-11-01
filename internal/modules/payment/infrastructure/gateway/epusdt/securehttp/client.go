// Package securehttp pins outbound connections to validated public addresses.
package securehttp

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"
)

var ErrDenied = errors.New("outbound request unavailable or denied")

func Public(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, cidr := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/32", "2002::/16", "64:ff9b::/96"} {
		if netip.MustParsePrefix(cidr).Contains(ip) {
			return false
		}
	}
	return true
}
func Endpoint(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || (u.Port() != "" && u.Port() != "443") {
		return nil, ErrDenied
	}
	return u, nil
}
func Transport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	t.DialTLSContext = nil
	t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	t.MaxResponseHeaderBytes = 32 << 10
	t.ResponseHeaderTimeout = 10 * time.Second
	t.MaxIdleConnsPerHost = 2
	t.MaxConnsPerHost = 4
	t.DialContext = Dial
	return t
}
func Dial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" {
		return nil, ErrDenied
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, ErrDenied
	}
	for _, ip := range ips {
		if !Public(ip) {
			return nil, ErrDenied
		}
	}
	for _, ip := range ips {
		c, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return c, nil
		}
	}
	return nil, ErrDenied
}

type restricted struct {
	transport *http.Transport
	allowed   map[string]bool
}

func (t restricted) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		return nil, ErrDenied
	}
	if !t.allowed[r.URL.String()] || r.URL.User != nil {
		return nil, ErrDenied
	}
	response, err := t.transport.RoundTrip(r)
	if err != nil {
		return nil, ErrDenied
	}
	return response, nil
}
func Client(endpoints ...string) (*http.Client, error) {
	allowed := map[string]bool{}
	for _, raw := range endpoints {
		if _, err := Endpoint(raw); err != nil {
			return nil, err
		}
		allowed[raw] = true
	}
	return &http.Client{Transport: restricted{Transport(), allowed}, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}

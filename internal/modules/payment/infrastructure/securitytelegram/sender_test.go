package securitytelegram

import (
	"context"
	"encoding/json"
	"errors"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTelegramRequiresActualAcknowledgmentAndNeverReturnsSecrets(t *testing.T) {
	token := "123456:synthetic_token_for_isolated_tests"
	for _, tc := range []struct {
		name   string
		status int
		body   string
		fail   bool
		ok     bool
	}{
		{"valid", 200, `{"ok":true,"result":{"message_id":7,"chat":{"id":-12345}}}`, false, true},
		{"wrong_chat", 200, `{"ok":true,"result":{"message_id":7,"chat":{"id":8}}}`, false, false},
		{"no_message", 200, `{"ok":true}`, false, false},
		{"refused", 200, `{"ok":false,"description":"private message"}`, false, false},
		{"rate_limit", 429, `private response`, false, false},
		{"redirect", 302, ``, false, false},
		{"network", 0, ``, true, false},
		{"oversized", 200, strings.Repeat("x", 65537), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := New(token, "-12345")
			if err != nil {
				t.Fatal(err)
			}
			if s.client.Transport.(*http.Transport).Proxy != nil {
				t.Fatal("environment proxy enabled")
			}
			calls := 0
			s.client.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.URL.Scheme != "https" || req.URL.Host != "api.telegram.org" || !strings.HasSuffix(req.URL.Path, "/sendMessage") {
					t.Fatal("unexpected destination")
				}
				raw, _ := io.ReadAll(req.Body)
				var body map[string]interface{}
				if json.Unmarshal(raw, &body) != nil {
					t.Fatal("bad body")
				}
				if body["parse_mode"] != nil || strings.Contains(string(raw), token) || !strings.Contains(body["text"].(string), "数量：2") {
					t.Fatal("unsafe message")
				}
				if tc.fail {
					return nil, errors.New(req.URL.String() + " private")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header), Request: req}, nil
			})
			err = s.Send(context.Background(), pd.SecurityAlert{Kind: "rpc_retry", Active: true, Count: 2, Version: 1})
			if (err == nil) != tc.ok {
				t.Fatalf("unexpected result %v", err)
			}
			if err != nil && (strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "private")) {
				t.Fatal("secret/error body leaked")
			}
			if calls != 1 {
				t.Fatal("unexpected retry amplification")
			}
			if err := s.Send(context.Background(), pd.SecurityAlert{Kind: "attacker supplied text"}); err == nil || calls != 1 {
				t.Fatal("unknown condition sent")
			}
		})
	}
	if _, err := New("https://attacker.example", "-12345"); err == nil {
		t.Fatal("invalid token accepted")
	}
}

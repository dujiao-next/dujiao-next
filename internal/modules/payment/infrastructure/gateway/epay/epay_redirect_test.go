package epay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func TestBuildRedirectURLV1(t *testing.T) {
	cfg := &Config{
		GatewayURL:  "https://gateway.example.com/",
		EpayVersion: VersionV1,
		MerchantID:  "1001",
		MerchantKey: "key-001",
		NotifyURL:   "https://api.example.com/api/v1/payments/callback",
		ReturnURL:   "https://shop.example.com/pay",
		SignType:    epaySignTypeMD5,
	}
	cfg.Normalize()

	result, err := BuildRedirectURL(cfg, CreateInput{
		OrderNo:     "DJP10001",
		Amount:      "88.00",
		Subject:     "测试订单",
		ChannelType: constants.PaymentChannelTypeAlipay,
		NotifyURL:   cfg.NotifyURL,
		ReturnURL:   cfg.ReturnURL,
	})
	if err != nil {
		t.Fatalf("BuildRedirectURL v1 failed: %v", err)
	}

	assertRedirectURLPath(t, result.PayURL, "https://gateway.example.com/submit.php")
	query := parseRedirectQuery(t, result.PayURL)
	if got := query.Get("pid"); got != "1001" {
		t.Fatalf("pid = %s, want 1001", got)
	}
	if got := query.Get("type"); got != constants.PaymentChannelTypeAlipay {
		t.Fatalf("type = %s, want %s", got, constants.PaymentChannelTypeAlipay)
	}
	if got := query.Get("out_trade_no"); got != "DJP10001" {
		t.Fatalf("out_trade_no = %s", got)
	}
	if got := query.Get("money"); got != "88.00" {
		t.Fatalf("money = %s", got)
	}
	if got := query.Get("notify_url"); got != cfg.NotifyURL {
		t.Fatalf("notify_url = %s", got)
	}
	if got := query.Get("return_url"); got != cfg.ReturnURL {
		t.Fatalf("return_url = %s", got)
	}
	if got := query.Get("sign_type"); got != epaySignTypeMD5 {
		t.Fatalf("sign_type = %s, want %s", got, epaySignTypeMD5)
	}
	expectedSign := signMD5(buildSignContent(map[string]string{
		"pid":          "1001",
		"type":         constants.PaymentChannelTypeAlipay,
		"out_trade_no": "DJP10001",
		"notify_url":   cfg.NotifyURL,
		"return_url":   cfg.ReturnURL,
		"name":         "测试订单",
		"money":        "88.00",
	}) + cfg.MerchantKey)
	if got := query.Get("sign"); got != expectedSign {
		t.Fatalf("sign = %s, want %s", got, expectedSign)
	}
}

func TestBuildRedirectURLV2(t *testing.T) {
	privateKeyPEM, publicKeyPEM := generateEpayRSAKeyPair(t)
	cfg := &Config{
		GatewayURL:  "https://gateway.example.com",
		EpayVersion: VersionV2,
		MerchantID:  "1002",
		PrivateKey:  privateKeyPEM,
		PublicKey:   publicKeyPEM,
		NotifyURL:   "https://api.example.com/api/v1/payments/callback",
		ReturnURL:   "https://shop.example.com/pay",
		SignType:    epaySignTypeRSA,
	}
	cfg.Normalize()

	result, err := BuildRedirectURL(cfg, CreateInput{
		OrderNo:     "DJP20002",
		Amount:      "66.00",
		Subject:     "测试订单2",
		ChannelType: constants.PaymentChannelTypeWechat,
		NotifyURL:   cfg.NotifyURL,
		ReturnURL:   cfg.ReturnURL,
	})
	if err != nil {
		t.Fatalf("BuildRedirectURL v2 failed: %v", err)
	}

	if result.PayURL != "" {
		t.Fatalf("v2 redirect pay url = %q, want empty so the RSA sign is not on a query string", result.PayURL)
	}
	if result.Raw == nil {
		t.Fatalf("v2 redirect raw payload is nil")
	}
	if got := rawRedirectString(t, result.Raw, "endpoint"); got != "https://gateway.example.com/api/pay/submit" {
		t.Fatalf("endpoint = %s", got)
	}
	if got := rawRedirectString(t, result.Raw, "submit_method"); got != http.MethodPost {
		t.Fatalf("submit_method = %s, want POST", got)
	}
	params := rawRedirectParams(t, result.Raw)
	if sign := params["sign"]; sign != "" && strings.Contains(result.PayURL, sign) {
		t.Fatalf("RSA sign must not be placed on a query string")
	}
	if got := params["pid"]; got != "1002" {
		t.Fatalf("pid = %s, want 1002", got)
	}
	if got := params["type"]; got != constants.PaymentChannelTypeWxpay {
		t.Fatalf("type = %s, want %s", got, constants.PaymentChannelTypeWxpay)
	}
	if got := params["out_trade_no"]; got != "DJP20002" {
		t.Fatalf("out_trade_no = %s", got)
	}
	if got := params["notify_url"]; got != cfg.NotifyURL {
		t.Fatalf("notify_url = %s", got)
	}
	if got := params["return_url"]; got != cfg.ReturnURL {
		t.Fatalf("return_url = %s", got)
	}
	if got := params["name"]; got != "测试订单2" {
		t.Fatalf("name = %s", got)
	}
	if got := params["money"]; got != "66.00" {
		t.Fatalf("money = %s", got)
	}
	if got := params["timestamp"]; got == "" {
		t.Fatalf("timestamp should not be empty")
	}
	if got := params["sign_type"]; got != epaySignTypeRSA {
		t.Fatalf("sign_type = %s, want %s", got, epaySignTypeRSA)
	}
	if _, ok := params["clientip"]; ok {
		t.Fatalf("clientip should be omitted for redirect mode")
	}
	if _, ok := params["method"]; ok {
		t.Fatalf("method should be omitted for redirect mode")
	}
	signContent := buildSignContent(map[string]string{
		"pid":          "1002",
		"type":         constants.PaymentChannelTypeWxpay,
		"out_trade_no": "DJP20002",
		"notify_url":   cfg.NotifyURL,
		"return_url":   cfg.ReturnURL,
		"name":         "测试订单2",
		"money":        "66.00",
		"timestamp":    params["timestamp"],
	})
	if err := verifyRSA(signContent, params["sign"], publicKeyPEM); err != nil {
		t.Fatalf("verify redirect sign failed: %v", err)
	}
}

func rawRedirectString(t *testing.T, raw map[string]interface{}, key string) string {
	t.Helper()
	value, ok := raw[key].(string)
	if !ok {
		t.Fatalf("raw[%s] = %#v, want string", key, raw[key])
	}
	return value
}

func rawRedirectParams(t *testing.T, raw map[string]interface{}) map[string]string {
	t.Helper()
	params, ok := raw["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("params = %#v, want map", raw["params"])
	}
	out := make(map[string]string, len(params))
	for key, value := range params {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("param %s = %#v, want string", key, value)
		}
		out[key] = text
	}
	return out
}

func parseRedirectQuery(t *testing.T, rawURL string) url.Values {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse redirect url failed: %v", err)
	}
	return parsed.Query()
}

func assertRedirectURLPath(t *testing.T, rawURL, want string) {
	t.Helper()
	if !strings.HasPrefix(rawURL, want) {
		t.Fatalf("redirect url = %s, want prefix %s", rawURL, want)
	}
}

func generateEpayRSAKeyPair(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key failed: %v", err)
	}
	privateDER := x509.MarshalPKCS1PrivateKey(key)
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateDER})
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key failed: %v", err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	return string(privatePEM), string(publicPEM)
}

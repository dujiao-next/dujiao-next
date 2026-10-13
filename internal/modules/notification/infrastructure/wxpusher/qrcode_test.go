package wxpusher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParameterQRCodeAndScanUID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/fun/create/qrcode":
			var v map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&v)
			if r.Method != "POST" || v["appToken"] != "AT_test" || v["extra"] != "nonce" || v["validTime"] != float64(300) {
				t.Error("invalid QR request")
			}
			_, _ = w.Write([]byte(`{"code":1000,"data":{"code":"opaque-provider-code","url":"https://wxpusher.zjiecode.com/api/qrcode/test.jpg"}}`))
		case "/api/fun/scan-qrcode-uid":
			if r.URL.Query().Get("code") != "opaque-provider-code" {
				t.Error("incorrect provider code")
			}
			_, _ = w.Write([]byte(`{"code":1000,"data":"UID_test"}`))
		default:
			t.Error("unexpected request")
		}
	}))
	defer server.Close()
	s := &Sender{Client: server.Client(), Endpoint: server.URL + "/api/send/message"}
	qr, err := s.CreateQRCode(context.Background(), "AT_test", "nonce", 300)
	if err != nil || qr.Code != "opaque-provider-code" {
		t.Fatalf("QR failed: %+v %v", qr, err)
	}
	uid, err := s.ScanUID(context.Background(), qr.Code)
	if err != nil || uid != "UID_test" {
		t.Fatalf("scan failed: %q %v", uid, err)
	}
}
func TestRejectUntrustedQRCodeURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":1000,"data":{"code":"test","url":"https://evil.example/qr"}}`))
	}))
	defer server.Close()
	s := &Sender{Client: server.Client(), Endpoint: server.URL + "/api/send/message"}
	if _, err := s.CreateQRCode(context.Background(), "AT_test", "nonce", 300); err == nil {
		t.Fatal("untrusted URL accepted")
	}
}

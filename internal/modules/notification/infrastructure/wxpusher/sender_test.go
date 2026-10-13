package wxpusher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStandardPushChecksRecipientResult(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantError  bool
	}{
		{"accepted", `{"code":1000,"data":[{"code":1000,"sendRecordId":42}]}`, false},
		{"recipient failed", `{"code":1000,"data":[{"code":1001}]}`, true},
		{"rate limited", `{"code":1001,"data":[]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.RawQuery != "" {
					t.Error("credentials should be in POST body")
				}
				var body map[string]interface{}
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					t.Error("invalid body")
				}
				if body["contentType"] != float64(1) || body["appToken"] != "AT_test" {
					t.Error("unexpected push fields")
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			sender := &Sender{Client: server.Client(), Endpoint: server.URL}
			result, err := sender.Send(context.Background(), "AT_test", "UID_test", "summary", "content", "https://shop.example/admin/support")
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantError && result.SendRecordID != 42 {
				t.Fatal("missing send record")
			}
		})
	}
}

package wxpusher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSimplePushUsesPOSTWithoutAppToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/send/message/simple-push" || r.Method != "POST" || r.URL.RawQuery != "" {
			t.Error("unsafe simple endpoint")
		}
		var v map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&v)
		if v["spt"] != "SPT_test" || v["contentType"] != float64(1) {
			t.Error("wrong simple payload")
		}
		if _, exists := v["appToken"]; exists {
			t.Error("simple push must not need app token")
		}
		_, _ = w.Write([]byte(`{"code":1000,"data":null}`))
	}))
	defer server.Close()
	s := &Sender{Client: server.Client(), Endpoint: server.URL + "/api/send/message"}
	if _, err := s.SendSimple(context.Background(), "SPT_test", "summary", "content", "https://shop.example/admin/support"); err != nil {
		t.Fatal(err)
	}
}
func TestSimpleRecipientFailureIsNotAccepted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":1000,"data":[{"code":1001}]}`))
	}))
	defer server.Close()
	s := &Sender{Client: server.Client(), Endpoint: server.URL + "/api/send/message"}
	if _, err := s.SendSimple(context.Background(), "SPT_test", "summary", "content", ""); err == nil {
		t.Fatal("recipient failure accepted")
	}
}

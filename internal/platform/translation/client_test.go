package translation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
)

func TestClientTranslatesProductWithoutPersistingSecret(t *testing.T) {
	t.Setenv(APIKeyEnv, "test-only-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-only-key" {
			t.Fatalf("authorization = %q", got)
		}
		var request struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Model != "test-model" || len(request.Messages) != 2 {
			t.Fatalf("request = %#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]string{
						"content": "```json\n{\"title\":\"测试商品\",\"description\":\"中文描述\",\"content\":\"中文详情\"}\n```",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(config.TranslationConfig{
		Enabled:        true,
		BaseURL:        server.URL,
		Model:          "test-model",
		TimeoutSeconds: 2,
		MaxChars:       1000,
	})
	got, err := client.TranslateProduct(context.Background(), ProductText{
		Title:       "Test product",
		Description: "A description",
		Content:     "Details",
	})
	if err != nil {
		t.Fatalf("TranslateProduct() error = %v", err)
	}
	if got.Title != "测试商品" || got.Description != "中文描述" || got.Content != "中文详情" {
		t.Fatalf("translated = %#v", got)
	}
}

func TestClientRequiresEnvironmentKey(t *testing.T) {
	t.Setenv(APIKeyEnv, "")
	client := NewClient(config.TranslationConfig{Enabled: true, BaseURL: "http://127.0.0.1:1"})
	_, err := client.TranslateProduct(context.Background(), ProductText{Title: "source"})
	if !errors.Is(err, ErrAPIKeyMissing) {
		t.Fatalf("error = %v, want ErrAPIKeyMissing", err)
	}
}

func TestClientDoesNotExposeProviderBodyOnHTTPError(t *testing.T) {
	t.Setenv(APIKeyEnv, "test-only-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("sensitive provider detail"))
	}))
	defer server.Close()

	client := NewClient(config.TranslationConfig{Enabled: true, BaseURL: server.URL})
	_, err := client.TranslateProduct(context.Background(), ProductText{Title: "source"})
	if err == nil || !strings.Contains(err.Error(), "status 401") || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientHonorsContextTimeout(t *testing.T) {
	t.Setenv(APIKeyEnv, "test-only-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(config.TranslationConfig{Enabled: true, BaseURL: server.URL, TimeoutSeconds: 30})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := client.TranslateProduct(ctx, ProductText{Title: "source"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

package application

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/dujiao-next/internal/config"
)

func TestExchangeCodeFetchesVerifiedPrimaryEmail(t *testing.T) {
	var redirectURI string
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		redirectURI = r.Form.Get("redirect_uri")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "token", "token_type": "bearer"})
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 123, "login": "octocat", "name": "Octo"})
	})
	mux.HandleFunc("/emails", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{{"email": "unverified@example.com", "verified": false, "primary": true}, {"email": "verified@example.com", "verified": true, "primary": false}})
	})
	s := NewService(config.GitHubAuthConfig{Enabled: true, ClientID: "id", ClientSecret: "secret"}, WithEndpoints(server.URL+"/token", server.URL+"/user", server.URL+"/emails"), WithHTTPClient(server.Client()))
	identity, err := s.ExchangeCode(context.Background(), "code", "https://outlooksell.uno/api/v1/auth/github/callback")
	if err != nil {
		t.Fatal(err)
	}
	if identity.ID != "123" || identity.Email != "verified@example.com" || identity.Login != "octocat" {
		t.Fatalf("identity=%+v", identity)
	}
	if redirectURI != "https://outlooksell.uno/api/v1/auth/github/callback" {
		t.Fatalf("redirect=%q", redirectURI)
	}
}

func TestExchangeCodeRejectsHTTPAndJSONErrorsAndUnverifiedEmails(t *testing.T) {
	for _, tc := range []struct {
		name        string
		tokenStatus int
		tokenBody   string
		emailBody   string
		target      error
	}{
		{"token status", 500, `{}`, `[]`, ErrGitHubServiceUnavailable},
		{"token json", 200, `not-json`, `[]`, ErrGitHubServiceUnavailable},
		{"unverified", 200, `{"access_token":"token"}`, `[{"email":"bad@example.com","verified":false}]`, ErrGitHubEmailUnverified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			server := httptest.NewServer(mux)
			defer server.Close()
			mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.tokenStatus)
				_, _ = w.Write([]byte(tc.tokenBody))
			})
			mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"id":1,"login":"x"}`)) })
			mux.HandleFunc("/emails", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(tc.emailBody)) })
			s := NewService(config.GitHubAuthConfig{Enabled: true, ClientID: "id", ClientSecret: "secret"}, WithEndpoints(server.URL+"/token", server.URL+"/user", server.URL+"/emails"), WithHTTPClient(server.Client()))
			_, err := s.ExchangeCode(context.Background(), "code", "https://site.test/api/v1/auth/github/callback")
			if !errors.Is(err, tc.target) {
				t.Fatalf("error=%v want=%v", err, tc.target)
			}
		})
	}
}

func TestAuthorizationURLUsesUserEmailScopeAndState(t *testing.T) {
	s := NewService(config.GitHubAuthConfig{Enabled: true, ClientID: "id", ClientSecret: "secret"})
	raw, err := s.AuthorizationURL("state-value", "https://site.test/api/v1/auth/github/callback")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	if u.Query().Get("scope") != "user:email" || u.Query().Get("state") != "state-value" {
		t.Fatalf("url=%s", raw)
	}
}

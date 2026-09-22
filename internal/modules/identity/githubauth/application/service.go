package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dujiao-next/internal/config"
)

var (
	ErrGitHubAuthDisabled       = errors.New("github auth disabled")
	ErrGitHubAuthConfigInvalid  = errors.New("github auth config invalid")
	ErrGitHubCodeInvalid        = errors.New("github oauth code invalid")
	ErrGitHubEmailUnverified    = errors.New("github email unverified")
	ErrGitHubServiceUnavailable = errors.New("github service unavailable")
)

const (
	defaultTokenURL     = "https://github.com/login/oauth/access_token"
	defaultUserURL      = "https://api.github.com/user"
	defaultEmailsURL    = "https://api.github.com/user/emails"
	defaultAuthorizeURL = "https://github.com/login/oauth/authorize"
	maxResponseBytes    = 1 << 20
)

type VerifiedIdentity struct {
	ID, Email, Login, Name, AvatarURL string
	AuthAt                            time.Time
}
type Option func(*Service)

func WithHTTPClient(client *http.Client) Option {
	return func(s *Service) {
		if client != nil {
			s.client = client
		}
	}
}
func WithEndpoints(token, user, emails string) Option {
	return func(s *Service) { s.tokenURL = token; s.userURL = user; s.emailsURL = emails }
}

type Service struct {
	mu                           sync.RWMutex
	cfg                          config.GitHubAuthConfig
	client                       *http.Client
	tokenURL, userURL, emailsURL string
}

func NewService(cfg config.GitHubAuthConfig, opts ...Option) *Service {
	s := &Service{client: &http.Client{Timeout: 5 * time.Second}, tokenURL: defaultTokenURL, userURL: defaultUserURL, emailsURL: defaultEmailsURL}
	s.SetConfig(cfg)
	for _, o := range opts {
		o(s)
	}
	return s
}
func (s *Service) SetConfig(cfg config.GitHubAuthConfig) {
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.ClientSecret = strings.TrimSpace(cfg.ClientSecret)
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
}
func (s *Service) snapshot() config.GitHubAuthConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}
func (s *Service) PublicConfig() map[string]interface{} {
	c := s.snapshot()
	return map[string]interface{}{"enabled": c.Enabled && c.ClientID != "" && c.ClientSecret != "", "client_id": c.ClientID}
}
func (s *Service) ValidateRuntimeConfig() error {
	c := s.snapshot()
	if !c.Enabled {
		return ErrGitHubAuthDisabled
	}
	if c.ClientID == "" || c.ClientSecret == "" || s.client == nil {
		return ErrGitHubAuthConfigInvalid
	}
	return nil
}
func (s *Service) AuthorizationURL(state, redirect string) (string, error) {
	if err := s.ValidateRuntimeConfig(); err != nil {
		return "", err
	}
	if strings.TrimSpace(state) == "" {
		return "", ErrGitHubCodeInvalid
	}
	u, _ := url.Parse(defaultAuthorizeURL)
	q := u.Query()
	q.Set("client_id", s.snapshot().ClientID)
	q.Set("redirect_uri", redirect)
	q.Set("scope", "user:email")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *Service) ExchangeCode(ctx context.Context, code, redirect string) (*VerifiedIdentity, error) {
	if err := s.ValidateRuntimeConfig(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(code) == "" {
		return nil, ErrGitHubCodeInvalid
	}
	cfg := s.snapshot()
	form := url.Values{"client_id": {cfg.ClientID}, "client_secret": {cfg.ClientSecret}, "code": {code}, "redirect_uri": {redirect}}
	var token struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := s.request(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", "", &token); err != nil {
		return nil, err
	}
	if token.Error != "" || token.AccessToken == "" {
		return nil, ErrGitHubCodeInvalid
	}
	var user struct {
		ID        json.Number `json:"id"`
		Login     string      `json:"login"`
		Name      string      `json:"name"`
		AvatarURL string      `json:"avatar_url"`
	}
	if err := s.request(ctx, http.MethodGet, s.userURL, nil, "", token.AccessToken, &user); err != nil {
		return nil, err
	}
	if user.ID.String() == "" || strings.TrimSpace(user.Login) == "" {
		return nil, ErrGitHubCodeInvalid
	}
	var emails []struct {
		Email    string `json:"email"`
		Verified bool   `json:"verified"`
		Primary  bool   `json:"primary"`
	}
	if err := s.request(ctx, http.MethodGet, s.emailsURL, nil, "", token.AccessToken, &emails); err != nil {
		return nil, err
	}
	email := ""
	for _, e := range emails {
		if !e.Verified {
			continue
		}
		normalized := strings.ToLower(strings.TrimSpace(e.Email))
		address, err := mail.ParseAddress(normalized)
		if err != nil || address.Address != normalized {
			continue
		}
		if email == "" || e.Primary {
			email = normalized
		}
		if e.Primary {
			break
		}
	}
	if email == "" {
		return nil, ErrGitHubEmailUnverified
	}
	id, err := strconv.ParseUint(user.ID.String(), 10, 64)
	if err != nil || id == 0 {
		return nil, ErrGitHubCodeInvalid
	}
	return &VerifiedIdentity{ID: strconv.FormatUint(id, 10), Email: email, Login: strings.TrimSpace(user.Login), Name: strings.TrimSpace(user.Name), AvatarURL: safeURL(user.AvatarURL), AuthAt: time.Now()}, nil
}
func (s *Service) request(ctx context.Context, method, endpoint string, body io.Reader, contentType, token string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGitHubServiceUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "dujiao-next")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGitHubServiceUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%w: status %d", ErrGitHubServiceUnavailable, resp.StatusCode)
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes))
	dec.UseNumber()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("%w: %v", ErrGitHubServiceUnavailable, err)
	}
	return nil
}
func safeURL(v string) string {
	u, err := url.Parse(strings.TrimSpace(v))
	if err != nil || u.User != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	return u.String()
}

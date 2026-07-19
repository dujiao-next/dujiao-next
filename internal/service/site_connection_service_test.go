package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/crypto"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/shopspring/decimal"
)

func TestSiteConnectionServiceGenericWebhookTokenLifecycle(t *testing.T) {
	repo := &siteConnectionRepoStub{}
	svc := NewSiteConnectionService(repo, "test-app-secret", t.TempDir())
	conn, err := svc.Create(CreateConnectionInput{
		Name:        "fulfillment webhook",
		BaseURL:     "https://provider.example.com/orders",
		ApiKey:      "first-token",
		Protocol:    constants.ConnectionProtocolGenericWebhook,
		CallbackURL: "https://shop.example.com/api/v1/upstream/generic-webhook/callback",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if conn.ApiKey != "first-token" {
		t.Fatalf("unexpected generic webhook connection: %#v", conn)
	}
	if conn.CallbackURL != "https://shop.example.com/api/v1/upstream/generic-webhook/callback" {
		t.Fatalf("unexpected generic webhook callback URL: %q", conn.CallbackURL)
	}
	verified, err := svc.VerifyGenericWebhookToken("first-token")
	if err != nil || verified.ID != conn.ID {
		t.Fatalf("VerifyGenericWebhookToken: conn=%#v err=%v", verified, err)
	}

	updated, err := svc.Update(conn.ID, UpdateConnectionInput{ApiKey: "second-token"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.ApiKey != "second-token" {
		t.Fatal("expected api key rotation")
	}
	if _, err := svc.VerifyGenericWebhookToken("first-token"); !errors.Is(err, ErrConnectionTokenInvalid) {
		t.Fatalf("expected old token rejection, got %v", err)
	}
	if _, err := svc.VerifyGenericWebhookToken("second-token"); err != nil {
		t.Fatalf("expected new token verification, got %v", err)
	}
}

func TestSiteConnectionServiceGenericWebhookValidation(t *testing.T) {
	svc := NewSiteConnectionService(&siteConnectionRepoStub{}, "test-app-secret", t.TempDir())
	base := CreateConnectionInput{
		Name:        "webhook",
		BaseURL:     "https://provider.example.com/orders",
		ApiKey:      "token",
		Protocol:    constants.ConnectionProtocolGenericWebhook,
		CallbackURL: "https://shop.example.com/api/v1/upstream/generic-webhook/callback",
	}
	if _, err := svc.Create(base); err != nil {
		t.Fatalf("generic webhook should not require api_secret: %v", err)
	}
	missingAPIKey := base
	missingAPIKey.ApiKey = ""
	if _, err := NewSiteConnectionService(&siteConnectionRepoStub{}, "test-app-secret", t.TempDir()).Create(missingAPIKey); !errors.Is(err, ErrConnectionInvalid) {
		t.Fatalf("expected missing api_key error, got %v", err)
	}

	invalidProtocol := base
	invalidProtocol.Protocol = "unknown"
	if _, err := NewSiteConnectionService(&siteConnectionRepoStub{}, "test-app-secret", t.TempDir()).Create(invalidProtocol); !errors.Is(err, ErrConnectionInvalid) {
		t.Fatalf("expected invalid protocol error, got %v", err)
	}
	missingCallback := base
	missingCallback.CallbackURL = ""
	if _, err := NewSiteConnectionService(&siteConnectionRepoStub{}, "test-app-secret", t.TempDir()).Create(missingCallback); !errors.Is(err, ErrConnectionInvalid) {
		t.Fatalf("expected invalid callback error, got %v", err)
	}
}

func TestSiteConnectionServiceGenericWebhookRejectsDuplicateAPIKey(t *testing.T) {
	repo := &siteConnectionRepoStub{conn: &models.SiteConnection{
		ID:       1,
		ApiKey:   "shared-token",
		Protocol: constants.ConnectionProtocolDujiaoNext,
	}}
	svc := NewSiteConnectionService(repo, "test-app-secret", t.TempDir())
	_, err := svc.Create(CreateConnectionInput{
		Name:        "webhook",
		BaseURL:     "https://provider.example.com/orders",
		ApiKey:      "shared-token",
		Protocol:    constants.ConnectionProtocolGenericWebhook,
		CallbackURL: "https://shop.example.com/api/v1/upstream/generic-webhook/callback",
	})
	if !errors.Is(err, ErrConnectionInvalid) {
		t.Fatalf("expected duplicate api_key rejection, got %v", err)
	}
}

func TestSupportedConnectionProtocols(t *testing.T) {
	protocols := SupportedConnectionProtocols()
	if len(protocols) != 2 {
		t.Fatalf("expected 2 protocols, got %d", len(protocols))
	}
	if protocols[1].Value != constants.ConnectionProtocolGenericWebhook || protocols[1].Capabilities.CatalogSync || !protocols[1].Capabilities.AsyncCallback {
		t.Fatalf("unexpected generic webhook capabilities: %#v", protocols[1])
	}
}

func TestSiteConnectionServicePingReturnsAdapterCreationError(t *testing.T) {
	appSecretKey := "test-secret-key"
	encrypted, err := crypto.Encrypt(crypto.DeriveKey(appSecretKey), "upstream-secret")
	if err != nil {
		t.Fatalf("encrypt secret failed: %v", err)
	}
	repo := &siteConnectionRepoStub{
		conn: &models.SiteConnection{
			ID:        1,
			Name:      "unsupported upstream",
			BaseURL:   "https://upstream.example.com",
			ApiKey:    "upstream-key",
			ApiSecret: encrypted,
			Protocol:  "unsupported-protocol",
			Status:    constants.ConnectionStatusPending,
		},
	}
	svc := NewSiteConnectionService(repo, appSecretKey, t.TempDir())

	result, err := svc.Ping(1)

	if err == nil {
		t.Fatalf("expected adapter creation error")
	}
	if result != nil {
		t.Fatalf("expected nil ping result, got %#v", result)
	}
	if !strings.Contains(err.Error(), "unsupported protocol") {
		t.Fatalf("expected unsupported protocol error, got %v", err)
	}
	if repo.updated {
		t.Fatalf("connection should not be updated when adapter creation fails")
	}
}

type fakeMarkupReapplier struct {
	calls []uint
}

func (f *fakeMarkupReapplier) ReapplyMarkup(connectionID uint) (int, error) {
	f.calls = append(f.calls, connectionID)
	return 0, nil
}

func newReapplyTestConn() *models.SiteConnection {
	encrypted, _ := crypto.Encrypt(crypto.DeriveKey("test-secret-key"), "stored-secret")
	return &models.SiteConnection{
		ID:                 7,
		Name:               "conn",
		BaseURL:            "https://up.example.com",
		ApiKey:             "key",
		ApiSecret:          encrypted,
		Protocol:           "dujiao-next",
		Status:             constants.ConnectionStatusActive,
		ExchangeRate:       decimal.NewFromInt(1),
		PriceMarkupPercent: decimal.Zero,
		PriceRoundingMode:  "none",
	}
}

func TestSiteConnectionServiceUpdateTriggersReapplyWhenExchangeRateChanges(t *testing.T) {
	repo := &siteConnectionRepoStub{conn: newReapplyTestConn()}
	svc := NewSiteConnectionService(repo, "test-secret-key", t.TempDir())
	reapplier := &fakeMarkupReapplier{}
	svc.SetMarkupReapplier(reapplier)

	rate := 6.9
	if _, err := svc.Update(7, UpdateConnectionInput{ExchangeRate: &rate}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if len(reapplier.calls) != 1 || reapplier.calls[0] != 7 {
		t.Fatalf("expected reapply called once for conn 7, got %#v", reapplier.calls)
	}
}

func TestSiteConnectionServiceUpdateSkipsReapplyWhenPriceConfigUnchanged(t *testing.T) {
	repo := &siteConnectionRepoStub{conn: newReapplyTestConn()}
	svc := NewSiteConnectionService(repo, "test-secret-key", t.TempDir())
	reapplier := &fakeMarkupReapplier{}
	svc.SetMarkupReapplier(reapplier)

	// 只改名字，汇率传入与现值相同的 1 → 定价配置未变，不应触发重算。
	name := "renamed"
	rate := 1.0
	if _, err := svc.Update(7, UpdateConnectionInput{Name: name, ExchangeRate: &rate}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if len(reapplier.calls) != 0 {
		t.Fatalf("expected no reapply when price config unchanged, got %#v", reapplier.calls)
	}
}

type siteConnectionRepoStub struct {
	conn    *models.SiteConnection
	updated bool
}

func (r *siteConnectionRepoStub) GetByID(id uint) (*models.SiteConnection, error) {
	if r.conn != nil && r.conn.ID == id {
		copy := *r.conn
		return &copy, nil
	}
	return nil, nil
}

func (r *siteConnectionRepoStub) GetByApiKey(apiKey string) (*models.SiteConnection, error) {
	if r.conn != nil && r.conn.ApiKey == apiKey {
		copy := *r.conn
		return &copy, nil
	}
	return nil, nil
}

func (r *siteConnectionRepoStub) Create(conn *models.SiteConnection) error {
	copy := *conn
	r.conn = &copy
	return nil
}

func (r *siteConnectionRepoStub) Update(conn *models.SiteConnection) error {
	r.updated = true
	copy := *conn
	r.conn = &copy
	return nil
}

func (r *siteConnectionRepoStub) Delete(id uint) error {
	if r.conn != nil && r.conn.ID == id {
		r.conn = nil
	}
	return nil
}

func (r *siteConnectionRepoStub) List(repository.SiteConnectionListFilter) ([]models.SiteConnection, int64, error) {
	if r.conn == nil {
		return nil, 0, nil
	}
	return []models.SiteConnection{*r.conn}, 1, nil
}

func (r *siteConnectionRepoStub) ListActive() ([]models.SiteConnection, error) {
	if r.conn == nil || r.conn.Status != constants.ConnectionStatusActive {
		return nil, nil
	}
	return []models.SiteConnection{*r.conn}, nil
}

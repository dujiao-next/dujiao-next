package siteconnection_test

import (
	"strings"
	"testing"

	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/crypto"

	"github.com/shopspring/decimal"
)

func TestSiteConnectionServicePingReturnsAdapterCreationError(t *testing.T) {
	appSecretKey := "test-secret-key"
	encrypted, err := crypto.Encrypt(crypto.DeriveKey(appSecretKey), "upstream-secret")
	if err != nil {
		t.Fatalf("encrypt secret failed: %v", err)
	}
	repo := &siteConnectionRepoStub{
		conn: &siteconnectiondomain.Connection{
			ID:        1,
			Name:      "unsupported upstream",
			BaseURL:   "https://upstream.example.com",
			ApiKey:    "upstream-key",
			ApiSecret: encrypted,
			Protocol:  "unsupported-protocol",
			Status:    constants.ConnectionStatusPending,
		},
	}
	svc := siteconnectionapp.NewService(repo, appSecretKey, t.TempDir())

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

func TestSiteConnectionServiceProdSellerStoresOnlySecretReference(t *testing.T) {
	repo := &siteConnectionRepoStub{}
	svc := siteconnectionapp.NewService(repo, "test-secret-key", t.TempDir())

	conn, err := svc.Create(siteconnectionapp.CreateInput{
		Name:      "ProdSeller",
		Protocol:  constants.ConnectionProtocolProdSeller,
		SecretRef: "PRODSELLER_API_KEY",
	})
	if err != nil {
		t.Fatalf("create ProdSeller connection failed: %v", err)
	}
	if conn.ApiKey != "" || conn.ApiSecret != "" {
		t.Fatalf("ProdSeller connection must not persist API credentials: %#v", conn)
	}
	if conn.SecretRef != "PRODSELLER_API_KEY" {
		t.Fatalf("secret ref = %q", conn.SecretRef)
	}
	if conn.BaseURL != "https://prodseller.com/v1" {
		t.Fatalf("default base URL = %q", conn.BaseURL)
	}
	if !conn.PriceMarkupPercent.Equal(decimal.NewFromInt(400)) || conn.PriceRoundingMode != "ceil_cent" || !conn.AutoSyncPrice {
		t.Fatalf("ProdSeller price defaults = markup %s rounding %q auto_sync=%v", conn.PriceMarkupPercent, conn.PriceRoundingMode, conn.AutoSyncPrice)
	}
}

type fakeMarkupReapplier struct {
	calls []uint
}

func (f *fakeMarkupReapplier) ReapplyMarkup(connectionID uint) (int, error) {
	f.calls = append(f.calls, connectionID)
	return 0, nil
}

func newReapplyTestConn() *siteconnectiondomain.Connection {
	return &siteconnectiondomain.Connection{
		ID:                 7,
		Name:               "conn",
		BaseURL:            "https://up.example.com",
		ApiKey:             "key",
		Protocol:           "dujiao-next",
		Status:             constants.ConnectionStatusActive,
		ExchangeRate:       decimal.NewFromInt(1),
		PriceMarkupPercent: decimal.Zero,
		PriceRoundingMode:  "none",
	}
}

func TestSiteConnectionServiceUpdateTriggersReapplyWhenExchangeRateChanges(t *testing.T) {
	repo := &siteConnectionRepoStub{conn: newReapplyTestConn()}
	svc := siteconnectionapp.NewService(repo, "test-secret-key", t.TempDir())
	reapplier := &fakeMarkupReapplier{}
	svc.SetMarkupReapplier(reapplier)

	rate := 6.9
	if _, err := svc.Update(7, siteconnectionapp.UpdateInput{ExchangeRate: &rate}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if len(reapplier.calls) != 1 || reapplier.calls[0] != 7 {
		t.Fatalf("expected reapply called once for conn 7, got %#v", reapplier.calls)
	}
}

func TestSiteConnectionServiceUpdateSkipsReapplyWhenPriceConfigUnchanged(t *testing.T) {
	repo := &siteConnectionRepoStub{conn: newReapplyTestConn()}
	svc := siteconnectionapp.NewService(repo, "test-secret-key", t.TempDir())
	reapplier := &fakeMarkupReapplier{}
	svc.SetMarkupReapplier(reapplier)

	// 只改名字，汇率传入与现值相同的 1 → 定价配置未变，不应触发重算。
	name := "renamed"
	rate := 1.0
	if _, err := svc.Update(7, siteconnectionapp.UpdateInput{Name: name, ExchangeRate: &rate}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if len(reapplier.calls) != 0 {
		t.Fatalf("expected no reapply when price config unchanged, got %#v", reapplier.calls)
	}
}

func TestSiteConnectionServiceNormalizesLegacyProdSellerPricing(t *testing.T) {
	repo := &siteConnectionRepoStub{conn: &siteconnectiondomain.Connection{
		ID:                 9,
		Protocol:           constants.ConnectionProtocolProdSeller,
		PriceMarkupPercent: decimal.Zero,
		PriceRoundingMode:  "none",
		AutoSyncPrice:      false,
	}}
	svc := siteconnectionapp.NewService(repo, "test-secret-key", t.TempDir())
	reapplier := &fakeMarkupReapplier{}
	svc.SetMarkupReapplier(reapplier)

	if err := svc.NormalizeProdSellerDefaults(); err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if !repo.conn.PriceMarkupPercent.Equal(decimal.NewFromInt(400)) || repo.conn.PriceRoundingMode != "ceil_cent" || !repo.conn.AutoSyncPrice {
		t.Fatalf("legacy defaults were not normalized: %#v", repo.conn)
	}
	if len(reapplier.calls) != 1 || reapplier.calls[0] != 9 {
		t.Fatalf("expected one markup reapply, got %#v", reapplier.calls)
	}
}

func TestSiteConnectionServiceNormalizesConfiguredProdSellerPricingToFivefold(t *testing.T) {
	repo := &siteConnectionRepoStub{conn: &siteconnectiondomain.Connection{
		ID:                 10,
		Protocol:           constants.ConnectionProtocolProdSeller,
		PriceMarkupPercent: decimal.NewFromInt(100),
		PriceRoundingMode:  "ceil_cent",
		AutoSyncPrice:      true,
	}}
	svc := siteconnectionapp.NewService(repo, "test-secret-key", t.TempDir())
	reapplier := &fakeMarkupReapplier{}
	svc.SetMarkupReapplier(reapplier)

	if err := svc.NormalizeProdSellerDefaults(); err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if !repo.conn.PriceMarkupPercent.Equal(decimal.NewFromInt(400)) || repo.conn.PriceRoundingMode != "ceil_cent" || !repo.conn.AutoSyncPrice {
		t.Fatalf("configured pricing was not normalized: %#v", repo.conn)
	}
	if len(reapplier.calls) != 1 || reapplier.calls[0] != 10 {
		t.Fatalf("expected one markup reapply, got %#v", reapplier.calls)
	}
}

type siteConnectionRepoStub struct {
	conn    *siteconnectiondomain.Connection
	updated bool
}

func (r *siteConnectionRepoStub) GetByID(id uint) (*siteconnectiondomain.Connection, error) {
	if r.conn != nil && r.conn.ID == id {
		copy := *r.conn
		return &copy, nil
	}
	return nil, nil
}

func (r *siteConnectionRepoStub) GetByApiKey(apiKey string) (*siteconnectiondomain.Connection, error) {
	if r.conn != nil && r.conn.ApiKey == apiKey {
		copy := *r.conn
		return &copy, nil
	}
	return nil, nil
}

func (r *siteConnectionRepoStub) Create(conn *siteconnectiondomain.Connection) error {
	copy := *conn
	r.conn = &copy
	return nil
}

func (r *siteConnectionRepoStub) Update(conn *siteconnectiondomain.Connection) error {
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

func (r *siteConnectionRepoStub) List(siteconnectioncontract.ListFilter) ([]siteconnectiondomain.Connection, int64, error) {
	if r.conn == nil {
		return nil, 0, nil
	}
	return []siteconnectiondomain.Connection{*r.conn}, 1, nil
}

func (r *siteConnectionRepoStub) ListActive() ([]siteconnectiondomain.Connection, error) {
	if r.conn == nil || r.conn.Status != constants.ConnectionStatusActive {
		return nil, nil
	}
	return []siteconnectiondomain.Connection{*r.conn}, nil
}

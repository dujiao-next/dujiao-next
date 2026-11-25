package userhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	resellermodule "github.com/dujiao-next/internal/modules/reseller/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	uploadcontract "github.com/dujiao-next/internal/modules/upload/contract"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type managementStub struct {
	profile  *resellerdomain.Profile
	domains  []resellerdomain.Domain
	canApply bool
	err      error
}

func (s managementStub) GetUserManagementSnapshot(userID uint) (*resellerdomain.Profile, []resellerdomain.Domain, bool, error) {
	return s.profile, s.domains, s.canApply, s.err
}

func (s managementStub) UpdateUserDefaultMarkup(userID uint, value string) (*resellerdomain.Profile, error) {
	return s.profile, s.err
}

func (s managementStub) ApplyUserReseller(userID uint, input resellermodule.ResellerApplyInput) (*resellerdomain.Profile, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.profile != nil {
		return s.profile, nil
	}
	return &resellerdomain.Profile{UserID: userID, Status: resellerdomain.ProfileStatusPendingReview, ApplyReason: input.Reason}, nil
}

func (s managementStub) SubmitUserCustomDomain(userID uint, rawDomain string) (*resellerdomain.Domain, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &resellerdomain.Domain{Domain: rawDomain}, nil
}

type siteConfigStub struct {
	profile *resellerdomain.Profile
	row     *resellerdomain.SiteConfig
	canEdit bool
	err     error
}

func (s siteConfigStub) GetUserSiteConfig(userID uint) (*resellerdomain.Profile, *resellerdomain.SiteConfig, bool, error) {
	return s.profile, s.row, s.canEdit, s.err
}

func (s siteConfigStub) UpdateUserSiteConfig(ctx context.Context, userID uint, input resellermodule.ResellerSiteConfigInput) (*resellerdomain.SiteConfig, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &resellerdomain.SiteConfig{SiteName: input.SiteName, Logo: input.Logo}, nil
}

type uploadStub struct{}

func (uploadStub) SaveFileWithMeta(file *multipart.FileHeader, category string) (*uploadcontract.Result, error) {
	return &uploadcontract.Result{URL: "/uploads/reseller/x.png", Filename: "x.png", Size: 3}, nil
}

func newUserHandlerTestContext(method, path string, body []byte, userID uint) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("user_id", userID)
	return c, recorder
}

func TestUserHandlerApplyAndSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	profile := &resellerdomain.Profile{ID: 9, UserID: 7, Status: resellerdomain.ProfileStatusPendingReview}
	h := NewUserHandler(managementStub{profile: profile}, siteConfigStub{}, uploadStub{})

	c, recorder := newUserHandlerTestContext(http.MethodPost, "/api/v1/user/reseller/apply", []byte(`{"reason":"operate"}`), 7)
	h.ApplyProfile(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	c, recorder = newUserHandlerTestContext(http.MethodGet, "/api/v1/user/reseller/profile", nil, 7)
	h.GetManagementSnapshot(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"pending_review"`) {
		t.Fatalf("unexpected snapshot body: %s", recorder.Body.String())
	}
}

func TestUserHandlerSubmitCustomDomainMapsInactiveProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUserHandler(managementStub{err: resellercontract.ErrProfileInactive}, siteConfigStub{}, uploadStub{})
	c, recorder := newUserHandlerTestContext(http.MethodPost, "/api/v1/user/reseller/domains", []byte(`{"domain":"shop.customer.example"}`), 7)
	h.SubmitCustomDomain(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected http 200 envelope, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if resp.StatusCode != response.CodeBadRequest {
		t.Fatalf("expected status_code=400, got %+v body=%s", resp, recorder.Body.String())
	}
}

func TestUserHandlerSiteConfigUpdateAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	profile := &resellerdomain.Profile{ID: 3, UserID: 8, Status: resellerdomain.ProfileStatusActive}
	h := NewUserHandler(
		managementStub{profile: profile},
		siteConfigStub{profile: profile, canEdit: true, row: &resellerdomain.SiteConfig{SiteName: "Alice Store"}},
		uploadStub{},
	)

	c, recorder := newUserHandlerTestContext(http.MethodPut, "/api/v1/user/reseller/site-config", []byte(`{"site_name":"Alice Store"}`), 8)
	h.UpdateSiteConfig(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"site_name":"Alice Store"`) {
		t.Fatalf("unexpected update body: %s", recorder.Body.String())
	}

	c, recorder = newUserHandlerTestContext(http.MethodGet, "/api/v1/user/reseller/site-config", nil, 8)
	h.GetSiteConfig(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"site_name":"Alice Store"`) {
		t.Fatalf("unexpected get body: %s", recorder.Body.String())
	}
}

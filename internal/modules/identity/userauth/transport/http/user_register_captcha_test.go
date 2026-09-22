package userauthhttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	captcha "github.com/dujiao-next/internal/modules/captcha/contract"
	captchahttp "github.com/dujiao-next/internal/modules/captcha/transport/http"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/gin-gonic/gin"
)

type registerCaptchaSettingsStub struct {
	registrationEnabled      bool
	emailVerificationEnabled bool
}

func (s registerCaptchaSettingsStub) GetRegistrationEnabled(bool) (bool, error) {
	return s.registrationEnabled, nil
}

func (s registerCaptchaSettingsStub) GetEmailVerificationEnabled(bool) (bool, error) {
	return s.emailVerificationEnabled, nil
}

type registerCaptchaAuthStub struct {
	registerCalls int
}

func (s *registerCaptchaAuthStub) Register(string, string, string, bool, bool) (*userdomain.User, string, time.Time, error) {
	s.registerCalls++
	return &userdomain.User{ID: 1, Email: "buyer@example.com"}, "token", time.Now().Add(time.Hour), nil
}

func (s *registerCaptchaAuthStub) LoginStep1(string, string, bool) (*AuthLoginResult, error) {
	return nil, nil
}

type registerCaptchaVerifierStub struct {
	scene    string
	payload  captchahttp.CaptchaPayloadRequest
	clientIP string
	err      error
}

func (s *registerCaptchaVerifierStub) Verify(scene string, payload captchahttp.CaptchaPayloadRequest, clientIP string) error {
	s.scene = scene
	s.payload = payload
	s.clientIP = clientIP
	return s.err
}

func TestUserRegisterVerifiesDirectRegistrationCaptchaWhenEmailVerificationDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &registerCaptchaAuthStub{}
	verifier := &registerCaptchaVerifierStub{}
	handler := NewUserLoginHandler(registerCaptchaSettingsStub{
		registrationEnabled:      true,
		emailVerificationEnabled: false,
	}, auth, verifier, nil)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{
		"email":"buyer@example.com",
		"password":"Password1",
		"agreement_accepted":true,
		"captcha_payload":{"turnstile_token":"registration-token"}
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.RemoteAddr = "203.0.113.9:1234"

	handler.UserRegister(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if verifier.scene != constants.CaptchaSceneRegister {
		t.Fatalf("captcha scene = %q, want %q", verifier.scene, constants.CaptchaSceneRegister)
	}
	if verifier.payload.TurnstileToken != "registration-token" {
		t.Fatalf("turnstile token = %q", verifier.payload.TurnstileToken)
	}
	if verifier.clientIP != "203.0.113.9" {
		t.Fatalf("client IP = %q", verifier.clientIP)
	}
	if auth.registerCalls != 1 {
		t.Fatalf("register calls = %d, want 1", auth.registerCalls)
	}
}

func TestUserRegisterRejectsMissingDirectRegistrationCaptchaBeforeCreatingUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &registerCaptchaAuthStub{}
	verifier := &registerCaptchaVerifierStub{err: captcha.ErrRequired}
	handler := NewUserLoginHandler(registerCaptchaSettingsStub{
		registrationEnabled:      true,
		emailVerificationEnabled: false,
	}, auth, verifier, nil)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{
		"email":"buyer@example.com",
		"password":"Password1",
		"agreement_accepted":true
	}`))
	context.Request.Header.Set("Content-Type", "application/json")

	handler.UserRegister(context)

	var responseBody struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if responseBody.StatusCode != http.StatusBadRequest {
		t.Fatalf("status_code = %d, body = %s", responseBody.StatusCode, recorder.Body.String())
	}
	if verifier.scene != constants.CaptchaSceneRegister {
		t.Fatalf("captcha scene = %q, want %q", verifier.scene, constants.CaptchaSceneRegister)
	}
	if auth.registerCalls != 0 {
		t.Fatalf("register calls = %d, want 0", auth.registerCalls)
	}
}

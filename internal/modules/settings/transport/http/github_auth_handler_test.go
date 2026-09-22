package settingshttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	"github.com/gin-gonic/gin"
)

type gitHubAuthAdminStub struct {
	setting settingssecurity.GitHubAuthSetting
	patch   settingssecurity.GitHubAuthSettingPatch
	applied settingssecurity.GitHubAuthSetting
}

func (s *gitHubAuthAdminStub) GetGitHubAuthSetting() (settingssecurity.GitHubAuthSetting, error) {
	return s.setting, nil
}
func (s *gitHubAuthAdminStub) PatchGitHubAuthSetting(p settingssecurity.GitHubAuthSettingPatch) (settingssecurity.GitHubAuthSetting, error) {
	s.patch = p
	return s.setting, nil
}
func (s *gitHubAuthAdminStub) ApplyRuntime(v settingssecurity.GitHubAuthSetting) { s.applied = v }

func TestGitHubAuthHandlerNeverReturnsSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &gitHubAuthAdminStub{setting: settingssecurity.GitHubAuthSetting{Enabled: true, ClientID: "id", ClientSecret: "top-secret"}}
	h := NewGitHubAuthHandler(stub)
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := bytes.NewBufferString(`{"enabled":true,"client_id":"id","client_secret":""}`)
		c.Request = httptest.NewRequest(method, "/", body)
		c.Request.Header.Set("Content-Type", "application/json")
		if method == http.MethodGet {
			h.GetGitHubAuth(c)
		} else {
			h.UpdateGitHubAuth(c)
		}
		if bytes.Contains(w.Body.Bytes(), []byte("top-secret")) {
			t.Fatalf("secret leaked: %s", w.Body.String())
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
}

package settingshttp

import (
	"errors"

	"github.com/dujiao-next/internal/cache"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

type GitHubAuthAdminService interface {
	GetGitHubAuthSetting() (settingssecurity.GitHubAuthSetting, error)
	PatchGitHubAuthSetting(settingssecurity.GitHubAuthSettingPatch) (settingssecurity.GitHubAuthSetting, error)
	ApplyRuntime(settingssecurity.GitHubAuthSetting)
}

type GitHubAuthHandler struct{ gitHubAuth GitHubAuthAdminService }

func NewGitHubAuthHandler(service GitHubAuthAdminService) *GitHubAuthHandler {
	if service == nil {
		panic("settings github auth handler: service is nil")
	}
	return &GitHubAuthHandler{gitHubAuth: service}
}

func (h *GitHubAuthHandler) GetGitHubAuth(c *gin.Context) {
	setting, err := h.gitHubAuth.GetGitHubAuthSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, settingssecurity.MaskGitHubAuthSettingForAdmin(setting))
}

func (h *GitHubAuthHandler) UpdateGitHubAuth(c *gin.Context) {
	var patch settingssecurity.GitHubAuthSettingPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	setting, err := h.gitHubAuth.PatchGitHubAuthSetting(patch)
	if err != nil {
		if errors.Is(err, settingssecurity.ErrGitHubAuthConfigInvalid) {
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
		} else {
			ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		}
		return
	}
	h.gitHubAuth.ApplyRuntime(setting)
	_ = cache.DelAllPublicConfig(c.Request.Context())
	response.Success(c, settingssecurity.MaskGitHubAuthSettingForAdmin(setting))
}

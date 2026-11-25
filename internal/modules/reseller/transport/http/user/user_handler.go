package userhttp

import (
	"context"
	"errors"
	"mime/multipart"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	resellermodule "github.com/dujiao-next/internal/modules/reseller/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	dto "github.com/dujiao-next/internal/modules/reseller/transport/http/presenter"
	uploadcontract "github.com/dujiao-next/internal/modules/upload/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type ManagementService interface {
	GetUserManagementSnapshot(userID uint) (*resellerdomain.Profile, []resellerdomain.Domain, bool, error)
	UpdateUserDefaultMarkup(userID uint, value string) (*resellerdomain.Profile, error)
	ApplyUserReseller(userID uint, input resellermodule.ResellerApplyInput) (*resellerdomain.Profile, error)
	SubmitUserCustomDomain(userID uint, rawDomain string) (*resellerdomain.Domain, error)
}

type SiteConfigService interface {
	GetUserSiteConfig(userID uint) (*resellerdomain.Profile, *resellerdomain.SiteConfig, bool, error)
	UpdateUserSiteConfig(ctx context.Context, userID uint, input resellermodule.ResellerSiteConfigInput) (*resellerdomain.SiteConfig, error)
}

type UploadService interface {
	SaveFileWithMeta(file *multipart.FileHeader, category string) (*uploadcontract.Result, error)
}

type UserHandler struct {
	management ManagementService
	siteConfig SiteConfigService
	uploads    UploadService
}

func NewUserHandler(management ManagementService, siteConfig SiteConfigService, uploads UploadService) *UserHandler {
	if management == nil || siteConfig == nil || uploads == nil {
		panic("reseller user handler: required dependency is nil")
	}
	return &UserHandler{management: management, siteConfig: siteConfig, uploads: uploads}
}

type applyRequest struct {
	Reason string `json:"reason"`
}

type customDomainRequest struct {
	Domain string `json:"domain" binding:"required"`
}

type siteConfigRequest struct {
	SiteName     string                                   `json:"site_name"`
	Logo         string                                   `json:"logo"`
	Favicon      string                                   `json:"favicon"`
	Announcement resellermodule.ResellerAnnouncementInput `json:"announcement"`
	Support      resellermodule.ResellerSupportInput      `json:"support"`
	SEO          resellermodule.ResellerSEOInput          `json:"seo"`
	FooterLinks  []resellermodule.ResellerFooterLinkInput `json:"footer_links"`
	NavConfig    resellermodule.ResellerNavConfigInput    `json:"nav_config"`
}

func (req siteConfigRequest) toInput() resellermodule.ResellerSiteConfigInput {
	return resellermodule.ResellerSiteConfigInput{
		SiteName:     req.SiteName,
		Logo:         req.Logo,
		Favicon:      req.Favicon,
		Announcement: req.Announcement,
		Support:      req.Support,
		SEO:          req.SEO,
		FooterLinks:  req.FooterLinks,
		NavConfig:    req.NavConfig,
	}
}

// GetManagementSnapshot 获取当前用户的分销商准入与域名状态。
func (h *UserHandler) GetManagementSnapshot(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	profile, domains, canApply, err := h.management.GetUserManagementSnapshot(uid)
	if err != nil {
		respondUserManagementError(c, err, "error.user_fetch_failed")
		return
	}
	response.Success(c, dto.NewResellerManagementSnapshotResp(profile, domains, canApply))
}

// UpdateDefaultMarkup 保存当前分销商的默认加价。
func (h *UserHandler) UpdateDefaultMarkup(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req struct {
		DefaultMarkupPercent string `json:"default_markup_percent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	profile, err := h.management.UpdateUserDefaultMarkup(uid, req.DefaultMarkupPercent)
	if err != nil {
		respondUserManagementError(c, err, "error.save_failed")
		return
	}
	response.Success(c, dto.NewResellerManagementProfileResp(profile))
}

// ApplyProfile 提交当前用户的分销商申请。
func (h *UserHandler) ApplyProfile(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req applyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	profile, err := h.management.ApplyUserReseller(uid, resellermodule.ResellerApplyInput{Reason: req.Reason})
	if err != nil {
		respondUserManagementError(c, err, "error.save_failed")
		return
	}
	response.Success(c, dto.NewResellerManagementProfileResp(profile))
}

// ListDomains 查询当前用户的分销域名。
func (h *UserHandler) ListDomains(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	profile, domains, _, err := h.management.GetUserManagementSnapshot(uid)
	if err != nil {
		respondUserManagementError(c, err, "error.user_fetch_failed")
		return
	}
	if profile == nil {
		respondUserManagementError(c, resellercontract.ErrNotOpened, "error.user_fetch_failed")
		return
	}
	response.Success(c, dto.NewResellerDomainRespList(domains))
}

// SubmitCustomDomain 提交当前用户的自定义分销域名。
func (h *UserHandler) SubmitCustomDomain(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req customDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	row, err := h.management.SubmitUserCustomDomain(uid, req.Domain)
	if err != nil {
		respondUserManagementError(c, err, "error.save_failed")
		return
	}
	response.Success(c, dto.NewResellerDomainResp(row))
}

// GetSiteConfig 获取当前用户的分销站点配置。
func (h *UserHandler) GetSiteConfig(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	profile, row, canEdit, err := h.siteConfig.GetUserSiteConfig(uid)
	if err != nil {
		respondUserManagementError(c, err, "error.user_fetch_failed")
		return
	}
	response.Success(c, dto.NewResellerSiteConfigSnapshotResp(profile, row, canEdit))
}

// UpdateSiteConfig 更新当前用户的分销站点配置。
func (h *UserHandler) UpdateSiteConfig(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req siteConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	row, err := h.siteConfig.UpdateUserSiteConfig(c.Request.Context(), uid, req.toInput())
	if err != nil {
		var fieldErr *resellermodule.ResellerSiteConfigFieldError
		if errors.As(err, &fieldErr) {
			ginutil.RespondError(c, response.CodeBadRequest, siteConfigFieldErrorKey(fieldErr.Field), nil)
			return
		}
		respondUserManagementError(c, err, "error.save_failed")
		return
	}
	response.Success(c, dto.NewResellerSiteConfigResp(row))
}

// UploadImage 分销商上传站点图片。
func (h *UserHandler) UploadImage(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	_, _, canEdit, err := h.siteConfig.GetUserSiteConfig(uid)
	if err != nil {
		respondUserManagementError(c, err, "error.forbidden")
		return
	}
	if !canEdit {
		ginutil.RespondError(c, response.CodeForbidden, "error.forbidden", nil)
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.file_missing", nil)
		return
	}
	result, err := h.uploads.SaveFileWithMeta(file, "reseller")
	if err != nil {
		if isUploadValidationError(err) {
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.upload_failed", err)
		return
	}
	response.Success(c, gin.H{
		"url":      result.URL,
		"filename": result.Filename,
		"size":     result.Size,
	})
}
